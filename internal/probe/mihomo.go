//go:build probe_mihomo

package probe

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	mihomoadapter "github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/component/ca"
	mihomoconstant "github.com/metacubex/mihomo/constant"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func init() {
	builtinBackendFactories = append(builtinBackendFactories, func(e *Engine) Backend {
		return &MihomoBackend{now: e.now}
	})
}

type MihomoBackend struct {
	now func() time.Time
}

type mihomoURLTestDialer struct {
	proxy mihomoconstant.Proxy
}

func (d mihomoURLTestDialer) DialContext(ctx context.Context, _ string, address string) (net.Conn, error) {
	var metadata mihomoconstant.Metadata
	if err := metadata.SetRemoteAddress(address); err != nil {
		return nil, err
	}
	return d.proxy.DialContext(ctx, &metadata)
}

func (b *MihomoBackend) Method() domain.ProbeMethod { return domain.ProbeURLTest }

func (b *MihomoBackend) Core() string { return "mihomo" }

func (b *MihomoBackend) Name() string { return "mihomo_url_test" }

func (b *MihomoBackend) Version() string { return mihomoconstant.Version }

func (b *MihomoBackend) Probe(ctx context.Context, backendReq BackendRequest, nodes []domain.NodeIR) (*domain.ProbeResult, error) {
	req := backendReq.Probe
	testURL := urlFromRequest(req)
	target, err := parseURLTestTarget(testURL)
	if err != nil {
		return nil, domain.WrapError(domain.CodeProbeInvalidTarget, "invalid url_test url", err)
	}
	expectedStatus, err := parseExpectedStatus(req.ExpectedStatus)
	if err != nil {
		return nil, domain.WrapError(domain.CodeProbeInvalidTarget, "invalid expected_status", err)
	}
	proxies, err := mihomoProxyMaps(backendReq.Payload)
	if err != nil {
		return nil, err
	}
	if len(proxies) != len(nodes) {
		return nil, domain.NewError(domain.CodeProbeInvalidTarget, "mihomo probe payload count does not match nodes")
	}

	attempts := attemptsFromRequest(req)
	timeout := timeoutFromRequest(req)
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
			results[i] = b.probeNode(ctx, req, node, proxies[i], target, expectedStatus, timeout, attempts)
		}()
	}
	wg.Wait()

	report := reportForResults(b.Name(), b.Version(), string(req.Method), req.Core, nodes, results)
	for i := range results {
		results[i].Backend = b.Name()
	}
	return &domain.ProbeResult{Results: results, Report: report}, nil
}

func (b *MihomoBackend) probeNode(ctx context.Context, req domain.ProbeRequest, node domain.NodeIR, mapping map[string]any, target urlTestTarget, expectedStatus expectedStatusMatcher, timeout time.Duration, attempts int) domain.NodeProbeResult {
	proxy, err := mihomoadapter.ParseProxy(mapping)
	if err != nil {
		return resultForError(req, node, string(domain.CodeProbeInvalidTarget), err, b.now())
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		mihomoTLSClientConfig, err := ca.GetTLSConfig(ca.Option{})
		if err == nil {
			// Mihomo's TLS fork is not assignable to crypto/tls. An empty
			// ca.Option configures only these two fields.
			tlsClientConfig := &tls.Config{
				Time:    mihomoTLSClientConfig.Time,
				RootCAs: mihomoTLSClientConfig.RootCAs,
			}
			var delay time.Duration
			delay, err = runURLTest(attemptCtx, target, urlTestOptions{
				dialer:          mihomoURLTestDialer{proxy: proxy},
				expectedStatus:  expectedStatus,
				tlsClientConfig: tlsClientConfig,
			})
			if err == nil {
				cancel()
				return successResult(req, node, int(delay/time.Millisecond), b.now())
			}
		}
		cancel()
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return resultForError(req, node, errorCodeForURLTest(lastErr), lastErr, b.now())
}

func mihomoProxyMaps(payload *Payload) ([]map[string]any, error) {
	if payload == nil || len(payload.Body) == 0 {
		return nil, domain.NewError(domain.CodeProbeInvalidTarget, "mihomo probe payload is missing")
	}
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(payload.Body, &doc); err != nil {
		return nil, domain.WrapError(domain.CodeProbeInvalidTarget, "decode mihomo probe payload", err)
	}
	if doc.Proxies == nil {
		return nil, domain.NewError(domain.CodeProbeInvalidTarget, "mihomo probe payload has no proxies")
	}
	return doc.Proxies, nil
}
