//go:build probe_singbox

package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/experimental/clashapi"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

var errSingBoxProxyNotFound = errors.New("sing-box proxy not found")

func init() {
	experimental.RegisterClashServerConstructor(clashapi.NewServer)
	builtinBackendFactories = append(builtinBackendFactories, func(e *Engine) Backend {
		return &SingBoxBackend{now: e.now}
	})
	builtinBackendFactories = append(builtinBackendFactories, func(e *Engine) Backend {
		return &SingBoxNTPBackend{now: e.now}
	})
}

type SingBoxBackend struct {
	now func() time.Time
}

func (b *SingBoxBackend) Method() domain.ProbeMethod { return domain.ProbeURLTest }

func (b *SingBoxBackend) Core() string { return "sing-box" }

func (b *SingBoxBackend) Name() string { return "singbox_url_test" }

func (b *SingBoxBackend) Version() string { return constant.Version }

func (b *SingBoxBackend) Probe(ctx context.Context, backendReq BackendRequest, nodes []domain.NodeIR) (*domain.ProbeResult, error) {
	req := backendReq.Probe
	testURL := urlFromRequest(req)
	if err := validateURLTestURL(testURL); err != nil {
		return nil, domain.WrapError(domain.CodeProbeInvalidTarget, "invalid url_test url", err)
	}
	if backendReq.Payload == nil || len(backendReq.Payload.Body) == 0 {
		return nil, domain.NewError(domain.CodeProbeInvalidTarget, "sing-box probe payload is missing")
	}
	controller, err := localControllerAddress()
	if err != nil {
		return nil, domain.WrapError(domain.CodeProbeCoreStartFailed, "allocate sing-box clash controller", err)
	}
	boxCtx, cancel := context.WithCancel(include.Context(ctx))
	defer cancel()
	options, err := singBoxOptions(boxCtx, backendReq.Payload, controller)
	if err != nil {
		return nil, err
	}
	instance, err := box.New(box.Options{
		Context: boxCtx,
		Options: options,
	})
	if err != nil {
		return nil, domain.WrapError(domain.CodeProbeCoreStartFailed, "create sing-box instance", err)
	}
	defer instance.Close()
	if err := instance.Start(); err != nil {
		return nil, domain.WrapError(domain.CodeProbeCoreStartFailed, "start sing-box instance", err)
	}

	timeout := timeoutFromRequest(req)
	attempts := attemptsFromRequest(req)
	concurrency := concurrencyFromRequest(req)
	client := &http.Client{Timeout: timeout + time.Second}
	results := make([]domain.NodeProbeResult, len(nodes))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, node := range nodes {
		i, node := i, node
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = resultForError(req, node, "probe_context_canceled", ctx.Err(), b.now())
				return
			}
			results[i] = b.probeNode(ctx, req, node, client, controller, testURL, timeout, attempts)
		}()
	}
	wg.Wait()

	report := reportForResults(b.Name(), b.Version(), string(req.Layer), string(req.Method), req.Core, nodes, results)
	for i := range results {
		results[i].Backend = b.Name()
	}
	if req.ExpectedStatus != "" {
		report.Warnings = append(report.Warnings, domain.Warning{
			Code:    "probe_expected_status_unsupported",
			Message: "sing-box urltest does not expose expected_status matching; expected_status was ignored",
		})
	}
	return &domain.ProbeResult{Results: results, Report: report}, nil
}

func (b *SingBoxBackend) probeNode(ctx context.Context, req domain.ProbeRequest, node domain.NodeIR, client *http.Client, controller, testURL string, timeout time.Duration, attempts int) domain.NodeProbeResult {
	if node.Name == "" {
		return resultForError(req, node, string(domain.CodeProbeInvalidTarget), errors.New("node name is required for sing-box delay API"), b.now())
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		delay, err := singBoxDelay(attemptCtx, client, controller, node.Name, testURL, timeout)
		cancel()
		if err == nil {
			return successResult(req, node, delay, b.now())
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return resultForError(req, node, errorCodeForSingBoxURLTest(lastErr), lastErr, b.now())
}

func singBoxOptions(ctx context.Context, payload *Payload, controller string) (option.Options, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(payload.Body, &doc); err != nil {
		return option.Options{}, domain.WrapError(domain.CodeProbeInvalidTarget, "decode sing-box probe payload", err)
	}
	logConfig, _ := json.Marshal(map[string]any{"disabled": true})
	experimentalConfig, _ := json.Marshal(map[string]any{
		"clash_api": map[string]any{
			"external_controller": controller,
		},
	})
	doc["log"] = logConfig
	doc["experimental"] = experimentalConfig
	body, err := json.Marshal(doc)
	if err != nil {
		return option.Options{}, domain.WrapError(domain.CodeProbeInvalidTarget, "encode sing-box probe config", err)
	}
	var options option.Options
	if err := options.UnmarshalJSONContext(ctx, body); err != nil {
		return option.Options{}, domain.WrapError(domain.CodeProbeInvalidTarget, "decode sing-box probe config", err)
	}
	return options, nil
}

func singBoxDelay(ctx context.Context, client *http.Client, controller, tag, testURL string, timeout time.Duration) (int, error) {
	endpoint := "http://" + controller + "/proxies/" + url.PathEscape(tag) + "/delay"
	query := url.Values{}
	query.Set("url", testURL)
	query.Set("timeout", strconv.Itoa(timeoutMillisecondsForAPI(timeout)))
	endpoint += "?" + query.Encode()

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return 0, err
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		defer resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK:
			var body struct {
				Delay int `json:"delay"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				return 0, err
			}
			return body.Delay, nil
		case http.StatusNotFound, http.StatusBadRequest:
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			if len(body) > 0 {
				return 0, fmt.Errorf("%w: %s", errSingBoxProxyNotFound, string(body))
			}
			return 0, errSingBoxProxyNotFound
		case http.StatusGatewayTimeout:
			return 0, context.DeadlineExceeded
		default:
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			if len(body) > 0 {
				return 0, fmt.Errorf("sing-box delay api returned %s: %s", resp.Status, string(body))
			}
			return 0, fmt.Errorf("sing-box delay api returned %s", resp.Status)
		}
	}
	if lastErr != nil {
		return 0, lastErr
	}
	return 0, domain.NewError(domain.CodeProbeCoreAPIFailed, "sing-box delay api did not respond")
}

func localControllerAddress() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		return "", err
	}
	return addr, nil
}

func timeoutMillisecondsForAPI(timeout time.Duration) int {
	ms := int(timeout / time.Millisecond)
	if ms < 1 {
		return 1
	}
	if ms > 32767 {
		return 32767
	}
	return ms
}

func errorCodeForSingBoxURLTest(err error) string {
	if errors.Is(err, errSingBoxProxyNotFound) {
		return string(domain.CodeProbeInvalidTarget)
	}
	return errorCodeForURLTest(err)
}

type SingBoxNTPBackend struct {
	now func() time.Time
}

func (b *SingBoxNTPBackend) Method() domain.ProbeMethod { return domain.ProbeUDPNTP }

func (b *SingBoxNTPBackend) Core() string { return "sing-box" }

func (b *SingBoxNTPBackend) Name() string { return "singbox_udp_ntp" }

func (b *SingBoxNTPBackend) Version() string { return constant.Version }

func (b *SingBoxNTPBackend) Probe(ctx context.Context, backendReq BackendRequest, nodes []domain.NodeIR) (*domain.ProbeResult, error) {
	req := backendReq.Probe
	if backendReq.Payload == nil || len(backendReq.Payload.Body) == 0 {
		return nil, domain.NewError(domain.CodeProbeInvalidTarget, "sing-box probe payload is missing")
	}
	boxCtx, cancel := context.WithCancel(include.Context(ctx))
	defer cancel()
	options, err := singBoxNTPOptions(boxCtx, backendReq.Payload)
	if err != nil {
		return nil, err
	}
	instance, err := box.New(box.Options{
		Context: boxCtx,
		Options: options,
	})
	if err != nil {
		return nil, domain.WrapError(domain.CodeProbeCoreStartFailed, "create sing-box instance", err)
	}
	defer instance.Close()
	if err := instance.Start(); err != nil {
		return nil, domain.WrapError(domain.CodeProbeCoreStartFailed, "start sing-box instance", err)
	}

	timeout := timeoutFromRequest(req)
	attempts := attemptsFromRequest(req)
	concurrency := concurrencyFromRequest(req)
	results := make([]domain.NodeProbeResult, len(nodes))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, node := range nodes {
		i, node := i, node
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = resultForError(req, node, "probe_context_canceled", ctx.Err(), b.now())
				return
			}
			results[i] = b.probeNode(ctx, req, node, instance, timeout, attempts)
		}()
	}
	wg.Wait()

	report := reportForResults(b.Name(), b.Version(), string(req.Layer), string(req.Method), req.Core, nodes, results)
	for i := range results {
		results[i].Backend = b.Name()
	}
	return &domain.ProbeResult{Results: results, Report: report}, nil
}

func (b *SingBoxNTPBackend) probeNode(ctx context.Context, req domain.ProbeRequest, node domain.NodeIR, instance *box.Box, timeout time.Duration, attempts int) domain.NodeProbeResult {
	if node.Name == "" {
		return resultForError(req, node, string(domain.CodeProbeInvalidTarget), errors.New("node name is required for sing-box outbound lookup"), b.now())
	}
	outbound, ok := instance.Outbound().Outbound(node.Name)
	if !ok {
		return resultForError(req, node, string(domain.CodeProbeInvalidTarget), fmt.Errorf("sing-box outbound %q not found", node.Name), b.now())
	}
	destination := M.ParseSocksaddrHostPort(ntpServerFromRequest(req), 123)
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		duration, err := b.ntpRoundTrip(attemptCtx, outbound, destination)
		cancel()
		if err == nil {
			return successResult(req, node, int(duration/time.Millisecond), b.now())
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return resultForError(req, node, errorCodeForUDPNTP(lastErr), lastErr, b.now())
}

func (b *SingBoxNTPBackend) ntpRoundTrip(ctx context.Context, outbound interface {
	ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error)
}, destination M.Socksaddr) (time.Duration, error) {
	start := b.now()
	conn, err := outbound.ListenPacket(ctx, destination)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	packet := make([]byte, 48)
	packet[0] = 0x1B
	if writer, ok := conn.(interface{ Write([]byte) (int, error) }); ok {
		if _, err := writer.Write(packet); err != nil {
			return 0, err
		}
	} else if _, err := conn.WriteTo(packet, destination); err != nil {
		return 0, err
	}
	buf := make([]byte, 512)
	if _, _, err := conn.ReadFrom(buf); err != nil {
		return 0, err
	}
	return b.now().Sub(start), nil
}

func singBoxNTPOptions(ctx context.Context, payload *Payload) (option.Options, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(payload.Body, &doc); err != nil {
		return option.Options{}, domain.WrapError(domain.CodeProbeInvalidTarget, "decode sing-box probe payload", err)
	}
	logConfig, _ := json.Marshal(map[string]any{"disabled": true})
	doc["log"] = logConfig
	body, err := json.Marshal(doc)
	if err != nil {
		return option.Options{}, domain.WrapError(domain.CodeProbeInvalidTarget, "encode sing-box probe config", err)
	}
	var options option.Options
	if err := options.UnmarshalJSONContext(ctx, body); err != nil {
		return option.Options{}, domain.WrapError(domain.CodeProbeInvalidTarget, "decode sing-box probe config", err)
	}
	return options, nil
}

func errorCodeForUDPNTP(err error) string {
	if err == nil {
		return string(domain.CodeProbeUDPNTPFailed)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return string(domain.CodeProbeTimeout)
	}
	if errors.Is(err, context.Canceled) {
		return "probe_context_canceled"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return string(domain.CodeProbeTimeout)
	}
	return string(domain.CodeProbeUDPNTPFailed)
}
