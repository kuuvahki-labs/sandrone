// Package probe implements runtime node reachability checks. It intentionally
// depends only on domain types; service owns parsing, storage and entrypoint
// policy before calling into this package.
package probe

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const (
	defaultTimeout     = 5 * time.Second
	defaultAttempts    = 1
	defaultConcurrency = 10
	defaultURLTestURL  = "http://www.gstatic.com/generate_204"
	defaultNTPServer   = "time.apple.com"
)

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type Payload struct {
	Core         string
	Format       string
	Body         []byte
	RenderReport domain.RenderReport
}

type BackendRequest struct {
	Probe   domain.ProbeRequest
	Payload *Payload
}

type Backend interface {
	Method() domain.ProbeMethod
	Name() string
	Version() string
	Probe(ctx context.Context, req BackendRequest, nodes []domain.NodeIR) (*domain.ProbeResult, error)
}

type CoreBackend interface {
	Backend
	Core() string
}

type backendFactory func(*Engine) Backend

var builtinBackendFactories []backendFactory

type Engine struct {
	dialer   Dialer
	now      func() time.Time
	backends map[domain.ProbeMethod][]Backend
}

type Option func(*Engine)

func WithDialer(dialer Dialer) Option {
	return func(e *Engine) {
		if dialer != nil {
			e.dialer = dialer
		}
	}
}

func WithBackend(backend Backend) Option {
	return func(e *Engine) {
		e.RegisterBackend(backend)
	}
}

func New(opts ...Option) *Engine {
	e := &Engine{
		dialer:   &net.Dialer{},
		now:      time.Now,
		backends: map[domain.ProbeMethod][]Backend{},
	}
	for _, opt := range opts {
		opt(e)
	}
	e.RegisterBackend(NewTCPBackend(e.dialer, e.now))
	for _, factory := range builtinBackendFactories {
		e.RegisterBackend(factory(e))
	}
	return e
}

func (e *Engine) RegisterBackend(backend Backend) {
	if backend == nil {
		return
	}
	if e.backends == nil {
		e.backends = map[domain.ProbeMethod][]Backend{}
	}
	method := NormalizeMethod(backend.Method())
	e.backends[method] = append(e.backends[method], backend)
}

func (e *Engine) BackendSummary() []map[string]string {
	out := []map[string]string{}
	methods := make([]string, 0, len(e.backends))
	for method := range e.backends {
		methods = append(methods, string(method))
	}
	sort.Strings(methods)
	for _, method := range methods {
		for _, backend := range e.backends[domain.ProbeMethod(method)] {
			item := map[string]string{
				"method": string(backend.Method()),
				"name":   backend.Name(),
			}
			if backend.Version() != "" {
				item["version"] = backend.Version()
			}
			if coreBackend, ok := backend.(CoreBackend); ok {
				item["core"] = coreBackend.Core()
			}
			out = append(out, item)
		}
	}
	return out
}

func (e *Engine) Probe(ctx context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, payloads ...Payload) (*domain.ProbeResult, error) {
	req = normalizeRequest(req)
	groups := probeGroups(req, nodes)
	if len(groups) == 1 {
		group := groups[0]
		backend, err := e.selectBackend(group.req)
		if err != nil {
			return nil, err
		}
		group.req = requestForBackend(group.req, backend)
		payload := payloadForBackend(backend, payloads)
		return backend.Probe(ctx, BackendRequest{Probe: group.req, Payload: payload}, group.nodes)
	}

	results := make([]domain.NodeProbeResult, len(nodes))
	runs := make([]probeGroupRun, len(groups))
	for i, group := range groups {
		backend, err := e.selectBackend(group.req)
		if err != nil {
			return nil, err
		}
		group.req = requestForBackend(group.req, backend)
		payload := payloadForBackend(backend, payloads)
		runs[i] = probeGroupRun{group: group, backend: backend, payload: payload}
	}

	reports := make([]domain.Report, len(runs))
	groupCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for i, run := range runs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			groupResult, err := run.backend.Probe(groupCtx, BackendRequest{Probe: run.group.req, Payload: run.payload}, run.group.nodes)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				mu.Unlock()
				return
			}
			if groupResult == nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = domain.NewError(domain.CodeNotImplemented, "probe backend returned nil result")
					cancel()
				}
				mu.Unlock()
				return
			}
			for resultIndex, originalIndex := range run.group.indexes {
				if resultIndex < len(groupResult.Results) {
					results[originalIndex] = groupResult.Results[resultIndex]
				}
			}
			reports[i] = groupResult.Report
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return &domain.ProbeResult{Results: results, Report: combineReports(reports, nodes, results)}, nil
}

func (e *Engine) SelectCore(req domain.ProbeRequest, nodes []domain.NodeIR) (string, bool) {
	req = normalizeRequest(req)
	method := methodRequiringCore(req, nodes)
	if method == "" {
		return "", false
	}
	req.Method = method
	backend, err := e.selectBackend(req)
	if err != nil {
		return "", false
	}
	coreBackend, ok := backend.(CoreBackend)
	if !ok {
		return "", false
	}
	return coreBackend.Core(), true
}

func (e *Engine) selectBackend(req domain.ProbeRequest) (Backend, error) {
	method := NormalizeMethod(req.Method)
	candidates := e.backends[method]
	switch method {
	case domain.ProbeURLTest:
		return selectCoreBackend(candidates, req.Core, "url_test", false)
	case domain.ProbeUDPNTP:
		return selectCoreBackend(candidates, req.Core, "udp_ntp", true)
	}
	if len(candidates) == 0 {
		return nil, domain.NewError(domain.CodeProbeBackendUnavailable, fmt.Sprintf("%s probe backend is not registered", method))
	}
	return candidates[0], nil
}

func selectCoreBackend(candidates []Backend, core, method string, preferSingBox bool) (Backend, error) {
	wantCore := NormalizeCore(core)
	coreBackends := make([]CoreBackend, 0, len(candidates))
	for _, candidate := range candidates {
		coreBackend, ok := candidate.(CoreBackend)
		if !ok {
			continue
		}
		if wantCore != "" && coreBackend.Core() == wantCore {
			return coreBackend, nil
		}
		coreBackends = append(coreBackends, coreBackend)
	}
	if wantCore != "" {
		return nil, domain.NewError(domain.CodeProbeCoreUnavailable, fmt.Sprintf("%s backend for %q is not registered", method, wantCore))
	}
	switch len(coreBackends) {
	case 0:
		return nil, domain.NewError(domain.CodeProbeCoreUnavailable, fmt.Sprintf("%s backend is not registered", method))
	case 1:
		return coreBackends[0], nil
	default:
		if preferSingBox {
			for _, backend := range coreBackends {
				if backend.Core() == "sing-box" {
					return backend, nil
				}
			}
		}
		return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("core is required when multiple %s backends are registered", method))
	}
}

func payloadForBackend(backend Backend, payloads []Payload) *Payload {
	if len(payloads) == 0 {
		return nil
	}
	coreBackend, isCore := backend.(CoreBackend)
	if !isCore {
		return nil
	}
	core := coreBackend.Core()
	for i := range payloads {
		if NormalizeCore(payloads[i].Core) == core {
			return &payloads[i]
		}
	}
	if len(payloads) == 1 {
		return &payloads[0]
	}
	return nil
}

func NormalizeMethod(method domain.ProbeMethod) domain.ProbeMethod {
	raw := strings.ToLower(strings.TrimSpace(string(method)))
	raw = strings.ReplaceAll(raw, "-", "_")
	switch raw {
	case "", string(domain.ProbeAuto):
		return domain.ProbeAuto
	case string(domain.ProbeTCPConnect):
		return domain.ProbeTCPConnect
	case string(domain.ProbeUDPNTP):
		return domain.ProbeUDPNTP
	case string(domain.ProbeURLTest), "urltest":
		return domain.ProbeURLTest
	default:
		return domain.ProbeMethod(raw)
	}
}

func NormalizeLayer(layer domain.ProbeLayer) domain.ProbeLayer {
	switch strings.ToLower(strings.TrimSpace(string(layer))) {
	case "", string(domain.ProbeLayerProtocol):
		return domain.ProbeLayerProtocol
	case string(domain.ProbeLayerProxy):
		return domain.ProbeLayerProxy
	default:
		return domain.ProbeLayer(strings.ToLower(strings.TrimSpace(string(layer))))
	}
}

func NTPServerFromRequest(req domain.ProbeRequest) string {
	return ntpServerFromRequest(req)
}

func URLTestTarget(req domain.ProbeRequest) string {
	return urlTestTarget(req)
}

func NormalizeCore(core string) string {
	raw := strings.ToLower(strings.TrimSpace(core))
	raw = strings.ReplaceAll(raw, "_", "-")
	switch raw {
	case "singbox":
		return "sing-box"
	default:
		return raw
	}
}

type probeGroup struct {
	req     domain.ProbeRequest
	indexes []int
	nodes   []domain.NodeIR
}

type probeGroupRun struct {
	group   probeGroup
	backend Backend
	payload *Payload
}

func normalizeRequest(req domain.ProbeRequest) domain.ProbeRequest {
	req.Layer = NormalizeLayer(req.Layer)
	req.Method = NormalizeMethod(req.Method)
	req.Core = NormalizeCore(req.Core)
	req.NTPServer = strings.TrimSpace(req.NTPServer)
	if req.NTPServer == "" {
		req.NTPServer = defaultNTPServer
	}
	req.URL = strings.TrimSpace(req.URL)
	return req
}

func requestForBackend(req domain.ProbeRequest, backend Backend) domain.ProbeRequest {
	if req.Method != domain.ProbeURLTest && req.Method != domain.ProbeUDPNTP {
		req.Core = ""
	} else if coreBackend, ok := backend.(CoreBackend); ok {
		req.Core = coreBackend.Core()
	}
	return req
}

func probeGroups(req domain.ProbeRequest, nodes []domain.NodeIR) []probeGroup {
	if req.Method != domain.ProbeAuto {
		return []probeGroup{{req: req, indexes: indexesForNodes(nodes), nodes: nodes}}
	}
	groupsByMethod := map[domain.ProbeMethod]int{}
	groups := []probeGroup{}
	for i, node := range nodes {
		groupReq := req
		groupReq.Method = resolveAutoMethod(req, node)
		pos, ok := groupsByMethod[groupReq.Method]
		if !ok {
			pos = len(groups)
			groupsByMethod[groupReq.Method] = pos
			groups = append(groups, probeGroup{req: groupReq})
		}
		groups[pos].indexes = append(groups[pos].indexes, i)
		groups[pos].nodes = append(groups[pos].nodes, node)
	}
	if len(nodes) == 0 {
		req.Method = resolveAutoMethod(req, domain.NodeIR{})
		return []probeGroup{{req: req}}
	}
	return groups
}

func indexesForNodes(nodes []domain.NodeIR) []int {
	indexes := make([]int, len(nodes))
	for i := range nodes {
		indexes[i] = i
	}
	return indexes
}

func resolveAutoMethod(req domain.ProbeRequest, node domain.NodeIR) domain.ProbeMethod {
	if req.Method != domain.ProbeAuto {
		return req.Method
	}
	if req.Layer == domain.ProbeLayerProxy {
		return domain.ProbeURLTest
	}
	if isUDPNTPNode(node.Type) {
		return domain.ProbeUDPNTP
	}
	return domain.ProbeTCPConnect
}

func methodRequiringCore(req domain.ProbeRequest, nodes []domain.NodeIR) domain.ProbeMethod {
	for _, group := range probeGroups(req, nodes) {
		switch group.req.Method {
		case domain.ProbeURLTest, domain.ProbeUDPNTP:
			return group.req.Method
		}
	}
	return ""
}

func isUDPNTPNode(nodeType domain.NodeType) bool {
	switch nodeType {
	case domain.NodeTypeHysteria, domain.NodeTypeHysteria2, domain.NodeTypeTUIC, domain.NodeTypeWireGuard:
		return true
	default:
		return false
	}
}

func targetFromRequest(req domain.ProbeRequest, node domain.NodeIR) string {
	switch req.Method {
	case domain.ProbeTCPConnect:
		if node.Server == "" || node.Port == 0 {
			return ""
		}
		return net.JoinHostPort(node.Server, strconv.Itoa(int(node.Port)))
	case domain.ProbeUDPNTP:
		return net.JoinHostPort(ntpServerFromRequest(req), "123")
	case domain.ProbeURLTest:
		return urlTestTarget(req)
	default:
		return ""
	}
}

func ntpServerFromRequest(req domain.ProbeRequest) string {
	server := strings.TrimSpace(req.NTPServer)
	if server == "" {
		return defaultNTPServer
	}
	return server
}

func urlTestTarget(req domain.ProbeRequest) string {
	if strings.TrimSpace(req.URL) != "" {
		return strings.TrimSpace(req.URL)
	}
	return defaultURLTestURL
}

func timeoutFromRequest(req domain.ProbeRequest) time.Duration {
	if req.TimeoutMS > 0 {
		return time.Duration(req.TimeoutMS) * time.Millisecond
	}
	return defaultTimeout
}

func attemptsFromRequest(req domain.ProbeRequest) int {
	if req.Attempts > 0 {
		return req.Attempts
	}
	return defaultAttempts
}

func concurrencyFromRequest(req domain.ProbeRequest) int {
	if req.Concurrency > 0 {
		return req.Concurrency
	}
	return defaultConcurrency
}
