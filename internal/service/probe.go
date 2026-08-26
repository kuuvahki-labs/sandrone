package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

type probeAvailability interface {
	ProbeAvailable() bool
}

type probeRequestAvailability interface {
	CheckAvailability(domain.ProbeRequest) error
}

type cachedNodeProbe struct {
	Result         domain.NodeProbeResult `json:"result"`
	Backend        string                 `json:"backend,omitempty"`
	BackendVersion string                 `json:"backend_version,omitempty"`
	RenderWarnings []domain.Warning       `json:"render_warnings,omitempty"`
}

type probeNodeGroup struct {
	EntryID   string
	Node      domain.NodeIR
	Indexes   []int
	Cached    *cachedNodeProbe
	MissIndex int
}

func (s *Service) Probe(ctx context.Context, req domain.ProbeRequest) (out *domain.ProbeResult, err error) {
	ctx = withProbeInputCacheScope(ctx, req.Input.Type, req.Input.Ref.Kind, req.Input.Ref.Name)
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
	if availability, ok := s.prober.(probeRequestAvailability); ok {
		if err := availability.CheckAvailability(req); err != nil {
			return nil, err
		}
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

	backend, err := s.resolveProbeBackend(&req, nodeSet.Nodes)
	if err != nil {
		return nil, err
	}
	groups, misses, err := s.resolveProbeNodeGroups(ctx, req, backend, nodeSet.Nodes)
	if err != nil {
		return nil, err
	}

	results := make([]domain.NodeProbeResult, len(nodeSet.Nodes))
	renderWarnings := make([]domain.Warning, 0)
	backendWarnings := make([]domain.Warning, 0)
	hasCachedGroup := false
	for groupIndex := range groups {
		group := &groups[groupIndex]
		if group.Cached == nil {
			continue
		}
		hasCachedGroup = true
		if backend.Name == "" {
			backend.Name = group.Cached.Backend
			backend.Version = group.Cached.BackendVersion
		}
		for _, nodeIndex := range group.Indexes {
			results[nodeIndex] = bindProbeResult(group.Cached.Result, nodeSet.Nodes[nodeIndex], true)
			renderWarnings = append(renderWarnings, bindProbeWarnings(group.Cached.RenderWarnings, nodeSet.Nodes[nodeIndex], nodeIndex)...)
		}
	}

	if len(misses) > 0 {
		payloads, freshRenderWarnings, renderErr := s.renderProbePayloads(ctx, &req, misses)
		cacheFreshResults := renderErr == nil
		var fresh *domain.ProbeResult
		if renderErr != nil {
			if !hasCachedGroup || !allProbeNodesSkipped(freshRenderWarnings, len(misses)) {
				return nil, renderErr
			}
			fresh = s.unrenderableProbeResult(req, backend, misses, freshRenderWarnings)
		} else {
			var probeErr error
			fresh, probeErr = s.prober.Probe(ctx, req, misses, payloads...)
			if probeErr != nil {
				return nil, probeErr
			}
		}
		if fresh == nil {
			return nil, domain.NewError(domain.CodeNotImplemented, "probe engine returned nil result")
		}
		if len(fresh.Results) != len(misses) {
			return nil, domain.NewError(domain.CodeProbeInvalidTarget, fmt.Sprintf("probe result count %d does not match miss count %d", len(fresh.Results), len(misses)))
		}
		freshByRuntimeID, resultErr := probeResultsByRuntimeID(fresh.Results)
		if resultErr != nil {
			return nil, resultErr
		}
		if fresh.Report.Probe != nil {
			if backend.Name == "" {
				backend.Name = fresh.Report.Probe.Backend
			}
			if backend.Version == "" {
				backend.Version = fresh.Report.Probe.BackendVersion
			}
		}
		for _, warning := range fresh.Report.Warnings {
			if warning.NodeIndex == nil {
				backendWarnings = append(backendWarnings, warning)
			}
		}

		warningTemplates, unscopedWarnings := probeRenderWarningTemplates(freshRenderWarnings, len(misses))
		renderWarnings = append(renderWarnings, unscopedWarnings...)
		for groupIndex := range groups {
			group := &groups[groupIndex]
			if group.Cached != nil {
				continue
			}
			freshResult, exists := freshByRuntimeID[domain.NodeRuntimeID(group.Node)]
			if !exists {
				return nil, domain.NewError(domain.CodeProbeInvalidTarget, fmt.Sprintf("probe result missing for runtime_id %q", domain.NodeRuntimeID(group.Node)))
			}
			freshResult = bindProbeResult(freshResult, group.Node, false)
			templates := warningTemplates[group.MissIndex]
			for _, nodeIndex := range group.Indexes {
				results[nodeIndex] = bindProbeResult(freshResult, nodeSet.Nodes[nodeIndex], false)
				renderWarnings = append(renderWarnings, bindProbeWarnings(templates, nodeSet.Nodes[nodeIndex], nodeIndex)...)
			}
			group.Cached = &cachedNodeProbe{
				Result:         cacheableProbeResult(freshResult),
				Backend:        backend.Name,
				BackendVersion: backend.Version,
				RenderWarnings: templates,
			}
		}
		if cacheFreshResults {
			if writeErr := s.writeProbeNodeGroups(ctx, req, groups); writeErr != nil {
				renderWarnings = append(renderWarnings, domain.Warning{Code: "probe_cache_write_failed", Message: writeErr.Error()})
				s.log(ctx, slog.LevelWarn, "service probe cache write failed",
					"operation", "probe",
					"method", string(req.Method),
					"core", req.Core,
					"node_count", nodeCount,
					"cache_key_prefix", cacheKeyPrefixProbe,
					"cache_ttl_seconds", req.CacheTTLSeconds,
					"duration_ms", elapsedMillis(start),
					"error", writeErr.Error(),
				)
			}
		}
	}

	report := probe.ReportForResults(backend.Name, backend.Version, string(req.Method), req.Core, nodeSet.Nodes, results)
	report.Dependencies = append(report.Dependencies, nodeSet.Dependencies...)
	for _, source := range nodeSet.Sources {
		report.SourceRefs = append(report.SourceRefs, source.SourceRefs...)
	}
	warnings := append([]domain.Warning{}, nodeSet.Warnings...)
	warnings = append(warnings, renderWarnings...)
	warnings = append(warnings, backendWarnings...)
	warnings = append(warnings, report.Warnings...)
	report.Warnings = warnings
	report = s.prepareReport("probe", report)
	result := &domain.ProbeResult{Results: results, Report: report}
	processor.RecordProbe(ctx, result)

	success, failure, cacheHits := probeCounts(result)
	s.log(ctx, slog.LevelInfo, "service probe completed",
		"operation", "probe",
		"method", string(req.Method),
		"core", req.Core,
		"node_count", nodeCount,
		"success_count", success,
		"failure_count", failure,
		"cache_hit_count", cacheHits,
		"cache_hit", nodeCount > 0 && cacheHits == nodeCount,
		"cache_key_prefix", cacheKeyPrefixProbe,
		"cache_ttl_seconds", req.CacheTTLSeconds,
		"warning_count", len(result.Report.Warnings),
		"duration_ms", elapsedMillis(start),
	)
	return result, nil
}

func probeResultsByRuntimeID(results []domain.NodeProbeResult) (map[string]domain.NodeProbeResult, error) {
	byRuntimeID := make(map[string]domain.NodeProbeResult, len(results))
	for _, result := range results {
		if result.RuntimeID == "" {
			return nil, domain.NewError(domain.CodeProbeInvalidTarget, "probe result is missing runtime_id")
		}
		if _, exists := byRuntimeID[result.RuntimeID]; exists {
			return nil, domain.NewError(domain.CodeProbeInvalidTarget, fmt.Sprintf("duplicate probe result runtime_id %q", result.RuntimeID))
		}
		byRuntimeID[result.RuntimeID] = result
	}
	return byRuntimeID, nil
}

func allProbeNodesSkipped(warnings []domain.Warning, nodeCount int) bool {
	if nodeCount == 0 {
		return false
	}
	skipped := make(map[int]struct{}, nodeCount)
	for _, warning := range warnings {
		if warning.Code != "render_node_skipped" || warning.NodeIndex == nil {
			continue
		}
		if index := *warning.NodeIndex; index >= 0 && index < nodeCount {
			skipped[index] = struct{}{}
		}
	}
	return len(skipped) == nodeCount
}

func (s *Service) unrenderableProbeResult(req domain.ProbeRequest, backend domain.ProbeBackendSummary, nodes []domain.NodeIR, warnings []domain.Warning) *domain.ProbeResult {
	messages := make(map[int]string, len(nodes))
	for _, warning := range warnings {
		if warning.Code == "render_node_skipped" && warning.NodeIndex != nil {
			messages[*warning.NodeIndex] = warning.Message
		}
	}
	target := ""
	switch req.Method {
	case domain.ProbeURLTest:
		target = probe.URLTestTarget(req)
	case domain.ProbeUDPNTP:
		target = probe.NTPServerFromRequest(req)
	}
	results := make([]domain.NodeProbeResult, len(nodes))
	for index, node := range nodes {
		message := messages[index]
		if message == "" {
			message = "node was skipped by probe renderer"
		}
		results[index] = domain.NodeProbeResult{
			RuntimeID: domain.NodeRuntimeID(node), NodeName: node.Name,
			Method: string(req.Method), Target: target, Core: req.Core,
			Backend: backend.Name, Alive: false, CheckedAt: s.now().UTC(),
			ErrorCode: string(domain.CodeProbeInvalidTarget), Error: message,
		}
	}
	return &domain.ProbeResult{Results: results, Report: probe.ReportForResults(backend.Name, backend.Version, string(req.Method), req.Core, nodes, results)}
}

func (s *Service) resolveProbeBackend(req *domain.ProbeRequest, nodes []domain.NodeIR) (domain.ProbeBackendSummary, error) {
	if req.Method == domain.ProbeTCPConnect {
		req.Core = ""
	} else if selector, ok := s.prober.(probeCoreSelector); ok {
		if core, selected := selector.SelectCore(*req, nodes); selected {
			req.Core = core
		}
	}
	backend := domain.ProbeBackendSummary{Method: req.Method, Core: probe.NormalizeCore(req.Core)}
	if resolver, ok := s.prober.(probeBackendResolver); ok {
		resolved, err := resolver.ResolveBackend(*req)
		if err != nil {
			return domain.ProbeBackendSummary{}, err
		}
		backend = resolved
		if req.Method == domain.ProbeTCPConnect {
			req.Core = ""
		} else {
			req.Core = resolved.Core
		}
	}
	return backend, nil
}

func (s *Service) renderProbePayloads(ctx context.Context, req *domain.ProbeRequest, nodes []domain.NodeIR) ([]probe.Payload, []domain.Warning, error) {
	req.Method = probe.NormalizeMethod(req.Method)
	if req.Method == domain.ProbeTCPConnect {
		req.Core = ""
	}
	if req.NTPServer == "" {
		req.NTPServer = probe.NTPServerFromRequest(*req)
	}
	if req.Method != domain.ProbeURLTest && req.Method != domain.ProbeUDPNTP {
		return nil, nil, nil
	}
	req.Core = probe.NormalizeCore(req.Core)
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
		return nil, append([]domain.Warning{}, renderReport.Warnings...), err
	}
	payload := probe.Payload{Core: req.Core, Format: target, Body: body, RenderReport: renderReport}
	return []probe.Payload{payload}, append([]domain.Warning{}, renderReport.Warnings...), nil
}

func (s *Service) resolveProbeNodeGroups(ctx context.Context, req domain.ProbeRequest, backend domain.ProbeBackendSummary, nodes []domain.NodeIR) ([]probeNodeGroup, []domain.NodeIR, error) {
	groups := make([]probeNodeGroup, 0, len(nodes))
	byKey := make(map[string]int, len(nodes))
	for nodeIndex, node := range nodes {
		entryID, err := probeNodeCacheEntryID(req, backend, node)
		if err != nil {
			return nil, nil, err
		}
		if groupIndex, exists := byKey[entryID]; exists {
			groups[groupIndex].Indexes = append(groups[groupIndex].Indexes, nodeIndex)
			continue
		}
		byKey[entryID] = len(groups)
		groups = append(groups, probeNodeGroup{EntryID: entryID, Node: node, Indexes: []int{nodeIndex}, MissIndex: -1})
	}

	misses := make([]domain.NodeIR, 0, len(groups))
	key, scoped := persistentCacheKey(ctx, cacheKeyPrefixProbe)
	canRead := req.CacheTTLSeconds > 0 && s.cache != nil && scoped && !cacheReadBypass(ctx)
	cachedValues := map[string]json.RawMessage{}
	if canRead {
		entryIDs := make([]string, len(groups))
		for index := range groups {
			entryIDs[index] = groups[index].EntryID
		}
		cachedValues = s.readCacheEntries(ctx, key, entryIDs, time.Duration(req.CacheTTLSeconds)*time.Second)
	}
	for groupIndex := range groups {
		group := &groups[groupIndex]
		if canRead {
			var cached cachedNodeProbe
			if body, ok := cachedValues[group.EntryID]; ok && json.Unmarshal(body, &cached) == nil {
				group.Cached = &cached
				continue
			}
		}
		group.MissIndex = len(misses)
		misses = append(misses, group.Node)
	}
	return groups, misses, nil
}

func (s *Service) writeProbeNodeGroups(ctx context.Context, req domain.ProbeRequest, groups []probeNodeGroup) error {
	if req.CacheTTLSeconds <= 0 || s.cache == nil {
		return nil
	}
	key, scoped := persistentCacheKey(ctx, cacheKeyPrefixProbe)
	if !scoped {
		return nil
	}
	entries := make([]cacheDocumentUpdate, 0, len(groups))
	for _, group := range groups {
		if group.MissIndex < 0 || group.Cached == nil {
			continue
		}
		entries = append(entries, cacheDocumentUpdate{ID: group.EntryID, Value: *group.Cached})
	}
	return s.writeCacheEntries(ctx, key, time.Duration(req.CacheTTLSeconds)*time.Second, entries)
}

func probeNodeCacheEntryID(req domain.ProbeRequest, backend domain.ProbeBackendSummary, node domain.NodeIR) (string, error) {
	req.Method = probe.NormalizeMethod(req.Method)
	req.Core = probe.NormalizeCore(req.Core)
	switch req.Method {
	case domain.ProbeURLTest:
		req.URL = probe.URLTestTarget(req)
		req.NTPServer = ""
	case domain.ProbeUDPNTP:
		req.URL = ""
		req.ExpectedStatus = ""
		req.NTPServer = probe.NTPServerFromRequest(req)
	default:
		req.URL = ""
		req.NTPServer = ""
		req.ExpectedStatus = ""
	}
	connectionKey, err := domain.NodeConnectionKey(node)
	if err != nil {
		return "", err
	}
	value := struct {
		ConnectionKey  string `json:"connection_key"`
		Method         string `json:"method"`
		Core           string `json:"core,omitempty"`
		Backend        string `json:"backend,omitempty"`
		BackendVersion string `json:"backend_version,omitempty"`
		URL            string `json:"url,omitempty"`
		NTPServer      string `json:"ntp_server,omitempty"`
		ExpectedStatus string `json:"expected_status,omitempty"`
		TimeoutMS      int    `json:"timeout_ms,omitempty"`
		Attempts       int    `json:"attempts,omitempty"`
	}{
		ConnectionKey: connectionKey,
		Method:        string(req.Method), Core: req.Core, Backend: backend.Name, BackendVersion: backend.Version,
		URL: req.URL, NTPServer: req.NTPServer, ExpectedStatus: req.ExpectedStatus,
		TimeoutMS: req.TimeoutMS, Attempts: req.Attempts,
	}
	return cacheIdentity(value)
}

func bindProbeResult(result domain.NodeProbeResult, node domain.NodeIR, cacheHit bool) domain.NodeProbeResult {
	result.RuntimeID = domain.NodeRuntimeID(node)
	result.NodeName = node.Name
	result.CacheHit = cacheHit
	return result
}

func cacheableProbeResult(result domain.NodeProbeResult) domain.NodeProbeResult {
	result.RuntimeID = ""
	result.NodeName = ""
	result.CacheHit = false
	return result
}

func probeRenderWarningTemplates(warnings []domain.Warning, nodeCount int) (map[int][]domain.Warning, []domain.Warning) {
	byNode := make(map[int][]domain.Warning)
	unscoped := make([]domain.Warning, 0)
	for _, warning := range warnings {
		if warning.NodeIndex == nil || *warning.NodeIndex < 0 || *warning.NodeIndex >= nodeCount {
			unscoped = append(unscoped, warning)
			continue
		}
		index := *warning.NodeIndex
		warning.Node = ""
		warning.NodeIndex = nil
		warning.NodeContext = nil
		byNode[index] = append(byNode[index], warning)
	}
	return byNode, unscoped
}

func bindProbeWarnings(templates []domain.Warning, node domain.NodeIR, nodeIndex int) []domain.Warning {
	if len(templates) == 0 {
		return nil
	}
	out := make([]domain.Warning, len(templates))
	for index, template := range templates {
		bound := template
		bound.Node = node.Name
		currentIndex := nodeIndex
		bound.NodeIndex = &currentIndex
		format := template.Target
		if format == "" {
			format = node.SourceFormat
		}
		bound.NodeContext = &domain.WarningNodeContext{
			Format: format,
			Name:   node.Name,
			Type:   node.Type,
			Server: node.Server,
			Port:   node.Port,
		}
		out[index] = bound
	}
	return out
}
