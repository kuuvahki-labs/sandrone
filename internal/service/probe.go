package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
)

type probeAvailability interface {
	ProbeAvailable() bool
}

func (s *Service) Probe(ctx context.Context, req domain.ProbeRequest) (out *domain.ProbeResult, err error) {
	start := time.Now()
	nodeCount := 0
	defer func() {
		if err == nil {
			return
		}
		s.log(ctx, slog.LevelError, "service probe failed",
			"operation", "probe",
			"method", string(probe.NormalizeMethod(req.Method)),
			"core", probe.NormalizeCore(req.Core),
			"node_count", nodeCount,
			"cache_ttl_seconds", req.CacheTTLSeconds,
			"duration_ms", elapsedMillis(start),
			"error", err.Error(),
		)
	}()
	if s.prober == nil {
		return nil, domain.NewError(domain.CodeNotImplemented, "probe engine is not configured")
	}
	if availability, ok := s.prober.(probeAvailability); ok && !availability.ProbeAvailable() {
		return nil, domain.NewError(domain.CodeProbeBackendUnavailable, "probe backend is not available")
	}
	req = s.probeRequestWithDefaults(req)
	req.Method = probe.NormalizeMethod(req.Method)
	switch req.Method {
	case domain.ProbeTCPConnect:
		req.Core = ""
	case domain.ProbeUDPNTP, domain.ProbeURLTest:
	default:
		return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported probe method %q", req.Method))
	}
	nodeSet, err := s.resolveNodeInput(ctx, req.Input, domain.FileRequest{
		Request: domain.RequestInfo{Meta: req.Meta},
		Meta:    req.Meta,
	})
	if err != nil {
		return nil, err
	}
	validated, validationWarnings, err := validateNodeBatch(nodeSet.Nodes, nodevalidation.StageProbe, req.Core)
	if err != nil {
		return nil, err
	}
	nodeSet.Nodes = validated.Nodes
	nodeSet.Warnings = append(nodeSet.Warnings, validationWarnings...)
	nodeCount = len(nodeSet.Nodes)
	if cached := s.readProbeCache(ctx, req, nodeSet.Nodes); cached != nil {
		cached.Report.Dependencies = append(cached.Report.Dependencies, nodeSet.Dependencies...)
		for _, source := range nodeSet.Sources {
			cached.Report.SourceRefs = append(cached.Report.SourceRefs, source.SourceRefs...)
		}
		cached.Report = s.prepareReport("probe", cached.Report)
		success, failure, cacheHits := probeCounts(cached)
		s.log(ctx, slog.LevelInfo, "service probe completed",
			"operation", "probe",
			"method", string(probe.NormalizeMethod(req.Method)),
			"core", probe.NormalizeCore(req.Core),
			"node_count", nodeCount,
			"success_count", success,
			"failure_count", failure,
			"cache_hit_count", cacheHits,
			"cache_hit", true,
			"cache_layer", cacheLayerProbe,
			"cache_ttl_seconds", req.CacheTTLSeconds,
			"warning_count", len(cached.Report.Warnings),
			"duration_ms", elapsedMillis(start),
		)
		return cached, nil
	}
	payloads, renderWarnings, err := s.renderProbePayloads(ctx, &req, nodeSet.Nodes)
	if err != nil {
		return nil, err
	}
	result, err := s.prober.Probe(ctx, req, nodeSet.Nodes, payloads...)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, domain.NewError(domain.CodeNotImplemented, "probe engine returned nil result")
	}
	result.Report.Dependencies = append(result.Report.Dependencies, nodeSet.Dependencies...)
	for _, source := range nodeSet.Sources {
		result.Report.SourceRefs = append(result.Report.SourceRefs, source.SourceRefs...)
	}
	warnings := append([]domain.Warning{}, nodeSet.Warnings...)
	warnings = append(warnings, renderWarnings...)
	warnings = append(warnings, result.Report.Warnings...)
	result.Report.Warnings = warnings
	if req.CacheTTLSeconds > 0 {
		if err := s.writeProbeCache(ctx, req, nodeSet.Nodes, result); err != nil {
			result.Report.Warnings = append(result.Report.Warnings, domain.Warning{
				Code:    "probe_cache_write_failed",
				Message: err.Error(),
			})
			s.log(ctx, slog.LevelWarn, "service probe cache write failed",
				"operation", "probe",
				"method", string(probe.NormalizeMethod(req.Method)),
				"core", probe.NormalizeCore(req.Core),
				"node_count", nodeCount,
				"cache_layer", cacheLayerProbe,
				"cache_ttl_seconds", req.CacheTTLSeconds,
				"duration_ms", elapsedMillis(start),
				"error", err.Error(),
			)
		}
	}
	result.Report = s.prepareReport("probe", result.Report)
	success, failure, cacheHits := probeCounts(result)
	s.log(ctx, slog.LevelInfo, "service probe completed",
		"operation", "probe",
		"method", string(probe.NormalizeMethod(req.Method)),
		"core", probe.NormalizeCore(req.Core),
		"node_count", nodeCount,
		"success_count", success,
		"failure_count", failure,
		"cache_hit_count", cacheHits,
		"cache_hit", false,
		"cache_layer", cacheLayerProbe,
		"cache_ttl_seconds", req.CacheTTLSeconds,
		"warning_count", len(result.Report.Warnings),
		"duration_ms", elapsedMillis(start),
	)
	return result, nil
}

func (s *Service) renderProbePayloads(ctx context.Context, req *domain.ProbeRequest, nodes []domain.NodeIR) ([]probe.Payload, []domain.Warning, error) {
	req.Method = probe.NormalizeMethod(req.Method)
	if req.Method == domain.ProbeTCPConnect {
		req.Core = ""
	}
	if req.NTPServer == "" {
		req.NTPServer = probe.NTPServerFromRequest(*req)
	}
	needsPayload := req.Method == domain.ProbeURLTest || req.Method == domain.ProbeUDPNTP
	if !needsPayload {
		return nil, nil, nil
	}
	req.Core = probe.NormalizeCore(req.Core)
	if selector, ok := s.prober.(probeCoreSelector); ok {
		core, ok := selector.SelectCore(*req, nodes)
		if !ok {
			return nil, nil, nil
		}
		req.Core = core
	}
	target := ""
	switch req.Core {
	case "mihomo":
		target = "mihomo-proxies"
	case "sing-box":
		target = "sing-box-outbounds"
	default:
		return nil, nil, nil
	}
	renderer, ok := s.renderers[target]
	if !ok {
		return nil, nil, domain.NewError(domain.CodeRenderFailed, fmt.Sprintf("probe renderer %q is not registered", target))
	}
	body, renderReport, err := s.renderWithReport(ctx, renderer, nodes, domain.RenderOptions{})
	if err != nil {
		return nil, nil, err
	}
	payload := probe.Payload{
		Core:         req.Core,
		Format:       target,
		Body:         body,
		RenderReport: renderReport,
	}
	return []probe.Payload{payload}, append([]domain.Warning{}, renderReport.Warnings...), nil
}

func (s *Service) readProbeCache(ctx context.Context, req domain.ProbeRequest, nodes []domain.NodeIR) *domain.ProbeResult {
	if req.CacheTTLSeconds <= 0 || s.cache == nil || cacheReadBypass(ctx) {
		return nil
	}
	key, err := probeCacheKey(req, nodes)
	if err != nil {
		return nil
	}
	c := s.cache
	if c == nil {
		return nil
	}
	var cached domain.ProbeResult
	if !c.GetJSON(ctx, key, &cached) {
		return nil
	}
	result := cloneProbeResult(cached)
	for i := range result.Results {
		result.Results[i].CacheHit = true
		if result.Results[i].CheckedAt.IsZero() {
			result.Results[i].CheckedAt = s.now().UTC()
		}
	}
	if result.Report.Probe == nil {
		result.Report.Probe = &domain.ProbeReport{Method: string(probe.NormalizeMethod(req.Method)), Core: probe.NormalizeCore(req.Core)}
	}
	result.Report.Probe.CacheHitCount = len(result.Results)
	result.Report.Warnings = append(result.Report.Warnings, domain.Warning{
		Code:    "probe_cache_hit",
		Message: fmt.Sprintf("%d probe result(s) loaded from cache", len(result.Results)),
	})
	return &result
}

func (s *Service) writeProbeCache(ctx context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, result *domain.ProbeResult) error {
	if req.CacheTTLSeconds <= 0 || s.cache == nil || result == nil {
		return nil
	}
	key, err := probeCacheKey(req, nodes)
	if err != nil {
		return err
	}
	c := s.cache
	if c == nil {
		return nil
	}
	return c.PutJSON(ctx, key, time.Duration(req.CacheTTLSeconds)*time.Second, *result)
}

func probeCacheKey(req domain.ProbeRequest, nodes []domain.NodeIR) (string, error) {
	req.Method = probe.NormalizeMethod(req.Method)
	req.Core = probe.NormalizeCore(req.Core)
	if req.NTPServer == "" {
		req.NTPServer = probe.NTPServerFromRequest(req)
	}
	if req.URL == "" {
		req.URL = probe.URLTestTarget(req)
	}
	value := struct {
		Nodes          []domain.NodeIR `json:"nodes"`
		Method         string          `json:"method"`
		Core           string          `json:"core,omitempty"`
		URL            string          `json:"url,omitempty"`
		NTPServer      string          `json:"ntp_server,omitempty"`
		ExpectedStatus string          `json:"expected_status,omitempty"`
		TimeoutMS      int             `json:"timeout_ms,omitempty"`
		Attempts       int             `json:"attempts,omitempty"`
	}{
		Nodes:          nodes,
		Method:         string(req.Method),
		Core:           req.Core,
		URL:            req.URL,
		NTPServer:      req.NTPServer,
		ExpectedStatus: req.ExpectedStatus,
		TimeoutMS:      req.TimeoutMS,
		Attempts:       req.Attempts,
	}
	return cachepkg.HashKey(cacheLayerProbe, value)
}

func cloneProbeResult(result domain.ProbeResult) domain.ProbeResult {
	out := result
	out.Results = append([]domain.NodeProbeResult{}, result.Results...)
	out.Report = cloneReport(result.Report)
	return out
}
