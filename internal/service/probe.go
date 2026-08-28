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
	ConnectionKey string
	Node          domain.NodeIR
	Indexes       []int
	Cached        *cachedNodeProbe
	MissIndex     int
}

type probeCacheSelector struct {
	Method         string `json:"method"`
	Core           string `json:"core,omitempty"`
	Backend        string `json:"backend,omitempty"`
	BackendVersion string `json:"backend_version,omitempty"`
	URL            string `json:"url,omitempty"`
	NTPServer      string `json:"ntp_server,omitempty"`
	ExpectedStatus string `json:"expected_status,omitempty"`
	TimeoutMS      int    `json:"timeout_ms,omitempty"`
	Attempts       int    `json:"attempts,omitempty"`
}

type probeCacheGroup struct {
	Selector probeCacheSelector         `json:"selector"`
	Nodes    map[string]cachedNodeProbe `json:"nodes"`
}

type probeCacheValue struct {
	Groups map[string]probeCacheGroup `json:"groups"`
}

type probeExecution struct {
	service         *Service
	ctx             context.Context
	req             domain.ProbeRequest
	start           time.Time
	nodeSet         *domain.NodeSet
	backend         domain.ProbeBackendSummary
	groups          []probeNodeGroup
	misses          []domain.NodeIR
	cacheSelector   probeCacheSelector
	results         []domain.NodeProbeResult
	renderWarnings  []domain.Warning
	backendWarnings []domain.Warning
	hasCachedGroup  bool
}

type freshProbeBatch struct {
	result         *domain.ProbeResult
	renderWarnings []domain.Warning
	cacheable      bool
}

func (s *Service) Probe(ctx context.Context, req domain.ProbeRequest) (out *domain.ProbeResult, err error) {
	ctx = withProbeInputCacheOwner(ctx, req.Input.Type, req.Input.Ref.Kind, req.Input.Ref.Name)
	execution := &probeExecution{service: s, ctx: ctx, req: req, start: time.Now()}
	defer func() {
		execution.logFailure(err)
	}()
	if err := execution.prepareRequest(); err != nil {
		return nil, err
	}
	if err := execution.resolveInput(); err != nil {
		return nil, err
	}
	if err := execution.resolveBackendAndCache(); err != nil {
		return nil, err
	}
	execution.bindCachedGroups()
	if err := execution.executeMisses(); err != nil {
		return nil, err
	}
	return execution.finish(), nil
}

func (e *probeExecution) prepareRequest() error {
	if e.service.prober == nil {
		return domain.NewError(domain.CodeNotImplemented, "probe engine is not configured")
	}
	if availability, ok := e.service.prober.(probeAvailability); ok && !availability.ProbeAvailable() {
		return domain.NewError(domain.CodeProbeBackendUnavailable, "probe backend is not available")
	}
	e.req = e.service.probeRequestWithDefaults(e.req)
	e.req.Method = probe.NormalizeMethod(e.req.Method)
	switch e.req.Method {
	case domain.ProbeTCPConnect:
		e.req.Core = ""
	case domain.ProbeUDPNTP, domain.ProbeURLTest:
	default:
		return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported probe method %q", e.req.Method))
	}
	if availability, ok := e.service.prober.(probeRequestAvailability); ok {
		return availability.CheckAvailability(e.req)
	}
	return nil
}

func (e *probeExecution) resolveInput() error {
	nodeSet, err := e.service.resolveNodeInput(e.ctx, e.req.Input, domain.FileRequest{
		Request: domain.RequestInfo{Meta: e.req.Meta},
		Meta:    e.req.Meta,
	})
	if err != nil {
		return err
	}
	validated, validationWarnings, err := validateNodeBatch(nodeSet.Nodes, nodevalidation.StageProbe, e.req.Core)
	if err != nil {
		return err
	}
	nodeSet.Nodes = validated.Nodes
	nodeSet.Warnings = append(nodeSet.Warnings, validationWarnings...)
	e.nodeSet = nodeSet
	e.results = make([]domain.NodeProbeResult, len(nodeSet.Nodes))
	return nil
}

func (e *probeExecution) resolveBackendAndCache() error {
	backend, err := e.service.resolveProbeBackend(&e.req, e.nodeSet.Nodes)
	if err != nil {
		return err
	}
	groups, misses, selector, err := e.service.resolveProbeNodeGroups(e.ctx, e.req, backend, e.nodeSet.Nodes)
	if err != nil {
		return err
	}
	e.backend = backend
	e.groups = groups
	e.misses = misses
	e.cacheSelector = selector
	return nil
}

func (e *probeExecution) bindCachedGroups() {
	for groupIndex := range e.groups {
		group := &e.groups[groupIndex]
		if group.Cached == nil {
			continue
		}
		e.hasCachedGroup = true
		if e.backend.Name == "" {
			e.backend.Name = group.Cached.Backend
			e.backend.Version = group.Cached.BackendVersion
		}
		for _, nodeIndex := range group.Indexes {
			e.results[nodeIndex] = bindProbeResult(group.Cached.Result, e.nodeSet.Nodes[nodeIndex], true)
			e.renderWarnings = append(e.renderWarnings, bindProbeWarnings(group.Cached.RenderWarnings, e.nodeSet.Nodes[nodeIndex], nodeIndex)...)
		}
	}
}

func (e *probeExecution) executeMisses() error {
	if len(e.misses) == 0 {
		return nil
	}
	batch, err := e.runFreshBatch()
	if err != nil {
		return err
	}
	freshByRuntimeID, err := probeResultsByRuntimeID(batch.result.Results)
	if err != nil {
		return err
	}
	e.absorbFreshReport(batch.result.Report)
	warningTemplates, unscopedWarnings := probeRenderWarningTemplates(batch.renderWarnings, len(e.misses))
	e.renderWarnings = append(e.renderWarnings, unscopedWarnings...)
	if err := e.bindFreshGroups(freshByRuntimeID, warningTemplates); err != nil {
		return err
	}
	if batch.cacheable {
		e.writeFreshCache()
	}
	return nil
}

func (e *probeExecution) runFreshBatch() (*freshProbeBatch, error) {
	payloads, renderWarnings, renderErr := e.service.renderProbePayloads(e.ctx, &e.req, e.misses)
	batch := &freshProbeBatch{renderWarnings: renderWarnings, cacheable: renderErr == nil}
	if renderErr != nil {
		if !e.hasCachedGroup || !allProbeNodesSkipped(renderWarnings, len(e.misses)) {
			return nil, renderErr
		}
		batch.result = e.service.unrenderableProbeResult(e.req, e.backend, e.misses, renderWarnings)
	} else {
		fresh, err := e.service.prober.Probe(e.ctx, e.req, e.misses, payloads...)
		if err != nil {
			return nil, err
		}
		batch.result = fresh
	}
	if batch.result == nil {
		return nil, domain.NewError(domain.CodeNotImplemented, "probe engine returned nil result")
	}
	if len(batch.result.Results) != len(e.misses) {
		return nil, domain.NewError(domain.CodeProbeInvalidTarget, fmt.Sprintf("probe result count %d does not match miss count %d", len(batch.result.Results), len(e.misses)))
	}
	return batch, nil
}

func (e *probeExecution) absorbFreshReport(report domain.Report) {
	if report.Probe != nil {
		if e.backend.Name == "" {
			e.backend.Name = report.Probe.Backend
		}
		if e.backend.Version == "" {
			e.backend.Version = report.Probe.BackendVersion
		}
	}
	for _, warning := range report.Warnings {
		if warning.NodeIndex == nil {
			e.backendWarnings = append(e.backendWarnings, warning)
		}
	}
}

func (e *probeExecution) bindFreshGroups(freshByRuntimeID map[string]domain.NodeProbeResult, warningTemplates map[int][]domain.Warning) error {
	for groupIndex := range e.groups {
		group := &e.groups[groupIndex]
		if group.Cached != nil {
			continue
		}
		freshResult, exists := freshByRuntimeID[domain.NodeRuntimeID(group.Node)]
		if !exists {
			return domain.NewError(domain.CodeProbeInvalidTarget, fmt.Sprintf("probe result missing for runtime_id %q", domain.NodeRuntimeID(group.Node)))
		}
		freshResult = bindProbeResult(freshResult, group.Node, false)
		templates := warningTemplates[group.MissIndex]
		for _, nodeIndex := range group.Indexes {
			e.results[nodeIndex] = bindProbeResult(freshResult, e.nodeSet.Nodes[nodeIndex], false)
			e.renderWarnings = append(e.renderWarnings, bindProbeWarnings(templates, e.nodeSet.Nodes[nodeIndex], nodeIndex)...)
		}
		group.Cached = &cachedNodeProbe{
			Result:         cacheableProbeResult(freshResult),
			Backend:        e.backend.Name,
			BackendVersion: e.backend.Version,
			RenderWarnings: templates,
		}
	}
	return nil
}

func (e *probeExecution) writeFreshCache() {
	writeErr := e.service.writeProbeNodeGroups(e.ctx, e.req, e.cacheSelector, e.groups)
	if writeErr == nil {
		return
	}
	e.renderWarnings = append(e.renderWarnings, domain.Warning{Code: "probe_cache_write_failed", Message: writeErr.Error()})
	e.service.log(e.ctx, slog.LevelWarn, "service probe cache write failed",
		"operation", "probe",
		"method", string(e.req.Method),
		"core", e.req.Core,
		"node_count", len(e.nodeSet.Nodes),
		"cache_key_prefix", cacheKeyPrefixProbe,
		"cache_ttl_seconds", e.req.CacheTTLSeconds,
		"duration_ms", elapsedMillis(e.start),
		"error", writeErr.Error(),
	)
}

func (e *probeExecution) finish() *domain.ProbeResult {
	report := probe.ReportForResults(e.backend.Name, e.backend.Version, string(e.req.Method), e.req.Core, e.nodeSet.Nodes, e.results)
	report.Dependencies = append(report.Dependencies, e.nodeSet.Dependencies...)
	for _, source := range e.nodeSet.Sources {
		report.SourceRefs = append(report.SourceRefs, source.SourceRefs...)
	}
	warnings := append([]domain.Warning{}, e.nodeSet.Warnings...)
	warnings = append(warnings, e.renderWarnings...)
	warnings = append(warnings, e.backendWarnings...)
	warnings = append(warnings, report.Warnings...)
	report.Warnings = warnings
	report = e.service.prepareReport("probe", report)
	result := &domain.ProbeResult{Results: e.results, Report: report}
	processor.RecordProbe(e.ctx, result)

	success, failure, cacheHits := probeCounts(result)
	nodeCount := len(e.nodeSet.Nodes)
	e.service.log(e.ctx, slog.LevelInfo, "service probe completed",
		"operation", "probe",
		"method", string(e.req.Method),
		"core", e.req.Core,
		"node_count", nodeCount,
		"success_count", success,
		"failure_count", failure,
		"cache_hit_count", cacheHits,
		"cache_hit", nodeCount > 0 && cacheHits == nodeCount,
		"cache_key_prefix", cacheKeyPrefixProbe,
		"cache_ttl_seconds", e.req.CacheTTLSeconds,
		"warning_count", len(result.Report.Warnings),
		"duration_ms", elapsedMillis(e.start),
	)
	return result
}

func (e *probeExecution) logFailure(err error) {
	if err == nil {
		return
	}
	nodeCount := 0
	if e.nodeSet != nil {
		nodeCount = len(e.nodeSet.Nodes)
	}
	e.service.log(e.ctx, slog.LevelError, "service probe failed",
		"operation", "probe",
		"method", string(probe.NormalizeMethod(e.req.Method)),
		"core", probe.NormalizeCore(e.req.Core),
		"node_count", nodeCount,
		"cache_ttl_seconds", e.req.CacheTTLSeconds,
		"duration_ms", elapsedMillis(e.start),
		"error", err.Error(),
	)
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

func (s *Service) resolveProbeNodeGroups(ctx context.Context, req domain.ProbeRequest, backend domain.ProbeBackendSummary, nodes []domain.NodeIR) ([]probeNodeGroup, []domain.NodeIR, probeCacheSelector, error) {
	selector := newProbeCacheSelector(req, backend)
	selectorID, err := cacheIdentity(selector)
	if err != nil {
		return nil, nil, probeCacheSelector{}, err
	}
	groups := make([]probeNodeGroup, 0, len(nodes))
	byKey := make(map[string]int, len(nodes))
	for nodeIndex, node := range nodes {
		connectionKey, err := domain.NodeConnectionKey(node)
		if err != nil {
			return nil, nil, probeCacheSelector{}, err
		}
		if groupIndex, exists := byKey[connectionKey]; exists {
			groups[groupIndex].Indexes = append(groups[groupIndex].Indexes, nodeIndex)
			continue
		}
		byKey[connectionKey] = len(groups)
		groups = append(groups, probeNodeGroup{ConnectionKey: connectionKey, Node: node, Indexes: []int{nodeIndex}, MissIndex: -1})
	}

	misses := make([]domain.NodeIR, 0, len(groups))
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixProbe)
	canRead := req.CacheTTLSeconds > 0 && s.cache != nil && owned && !cacheReadBypass(ctx)
	cachedNodes := map[string]cachedNodeProbe{}
	if canRead {
		item, found := s.readCacheValue[probeCacheValue](ctx, key, time.Duration(req.CacheTTLSeconds)*time.Second)
		if found {
			if cachedGroup, exists := item.Value.Groups[selectorID]; exists && cachedGroup.Selector == selector {
				cachedNodes = cachedGroup.Nodes
			}
		}
	}
	for groupIndex := range groups {
		group := &groups[groupIndex]
		if canRead {
			if cached, ok := cachedNodes[group.ConnectionKey]; ok {
				group.Cached = &cached
				continue
			}
		}
		group.MissIndex = len(misses)
		misses = append(misses, group.Node)
	}
	return groups, misses, selector, nil
}

func (s *Service) writeProbeNodeGroups(ctx context.Context, req domain.ProbeRequest, selector probeCacheSelector, groups []probeNodeGroup) error {
	if req.CacheTTLSeconds <= 0 || s.cache == nil {
		return nil
	}
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixProbe)
	if !owned {
		return nil
	}
	selectorID, err := cacheIdentity(selector)
	if err != nil {
		return err
	}
	value, remaining, ok := s.prepareCacheValueWrite[probeCacheValue](ctx, key, time.Duration(req.CacheTTLSeconds)*time.Second)
	if !ok {
		return nil
	}
	if value.Groups == nil {
		value.Groups = map[string]probeCacheGroup{}
	}
	nodes := make(map[string]cachedNodeProbe, len(groups))
	for _, group := range groups {
		if group.Cached == nil {
			continue
		}
		nodes[group.ConnectionKey] = *group.Cached
	}
	value.Groups[selectorID] = probeCacheGroup{Selector: selector, Nodes: nodes}
	return cachepkg.SetJSON(ctx, s.cache, key, value, remaining)
}

func newProbeCacheSelector(req domain.ProbeRequest, backend domain.ProbeBackendSummary) probeCacheSelector {
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
	return probeCacheSelector{
		Method: string(req.Method), Core: req.Core, Backend: backend.Name, BackendVersion: backend.Version,
		URL: req.URL, NTPServer: req.NTPServer, ExpectedStatus: req.ExpectedStatus,
		TimeoutMS: req.TimeoutMS, Attempts: req.Attempts,
	}
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
