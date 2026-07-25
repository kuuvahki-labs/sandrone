//go:build probe_mihomo

package probe

import (
	"context"
	"errors"
	"sync"
	"time"

	mihomoadapter "github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/common/utils"
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

func (b *MihomoBackend) Method() domain.ProbeMethod { return domain.ProbeURLTest }

func (b *MihomoBackend) Core() string { return "mihomo" }

func (b *MihomoBackend) Name() string { return "mihomo_url_test" }

func (b *MihomoBackend) Version() string { return mihomoconstant.Version }

func (b *MihomoBackend) Probe(ctx context.Context, backendReq BackendRequest, nodes []domain.NodeIR) (*domain.ProbeResult, error) {
	req := backendReq.Probe
	testURL := urlFromRequest(req)
	if err := validateURLTestURL(testURL); err != nil {
		return nil, domain.WrapError(domain.CodeProbeInvalidTarget, "invalid url_test url", err)
	}
	expectedStatus, err := utils.NewUnsignedRanges[uint16](req.ExpectedStatus)
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
			results[i] = b.probeNode(ctx, req, node, proxies[i], testURL, expectedStatus, timeout, attempts)
		}()
	}
	wg.Wait()

	report := reportForResults(b.Name(), b.Version(), string(req.Method), req.Core, nodes, results)
	for i := range results {
		results[i].Backend = b.Name()
	}
	return &domain.ProbeResult{Results: results, Report: report}, nil
}

func (b *MihomoBackend) probeNode(ctx context.Context, req domain.ProbeRequest, node domain.NodeIR, mapping map[string]any, testURL string, expectedStatus utils.IntRanges[uint16], timeout time.Duration, attempts int) domain.NodeProbeResult {
	proxy, err := mihomoadapter.ParseProxy(mapping)
	if err != nil {
		return resultForError(req, node, string(domain.CodeProbeInvalidTarget), err, b.now())
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		delay, err := proxy.URLTest(attemptCtx, testURL, expectedStatus)
		cancel()
		if err == nil {
			if req.ExpectedStatus != "" && !proxy.AliveForTestUrl(testURL) {
				lastErr = errors.New("response status did not match expected_status")
			} else {
				return successResult(req, node, int(delay), b.now())
			}
		} else {
			lastErr = err
		}
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
