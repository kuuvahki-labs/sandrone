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
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const (
	defaultTimeout     = 5 * time.Second
	defaultAttempts    = 1
	defaultConcurrency = 10
	// DefaultURLTestURL is the target used when a URL-test request omits its URL.
	DefaultURLTestURL = "https://cp.cloudflare.com"
	defaultNTPServer  = "time.apple.com"
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

func (e *Engine) BackendSummary() []domain.ProbeBackendSummary {
	out := []domain.ProbeBackendSummary{}
	methods := make([]string, 0, len(e.backends))
	for method := range e.backends {
		methods = append(methods, string(method))
	}
	sort.Strings(methods)
	for _, method := range methods {
		for _, backend := range e.backends[domain.ProbeMethod(method)] {
			item := domain.ProbeBackendSummary{
				Method: backend.Method(),
				Name:   backend.Name(),
			}
			item.Version = backend.Version()
			if coreBackend, ok := backend.(CoreBackend); ok {
				item.Core = coreBackend.Core()
			}
			out = append(out, item)
		}
	}
	return out
}

func (e *Engine) Probe(ctx context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, payloads ...Payload) (*domain.ProbeResult, error) {
	req = normalizeRequest(req)
	backend, err := e.selectBackend(req)
	if err != nil {
		return nil, err
	}
	req = requestForBackend(req, backend)
	payload := payloadForBackend(backend, payloads)
	return backend.Probe(ctx, BackendRequest{Probe: req, Payload: payload}, nodes)
}

// CheckAvailability resolves the requested backend without starting it. The
// service uses this before node rendering so unavailable cores fail with their
// canonical runtime error.
func (e *Engine) CheckAvailability(req domain.ProbeRequest) error {
	_, err := e.selectBackend(normalizeRequest(req))
	return err
}

// ResolveBackend returns the concrete backend selected for a request without
// starting it. Service uses this identity to scope per-node cache entries.
func (e *Engine) ResolveBackend(req domain.ProbeRequest) (domain.ProbeBackendSummary, error) {
	req = normalizeRequest(req)
	backend, err := e.selectBackend(req)
	if err != nil {
		return domain.ProbeBackendSummary{}, err
	}
	resolved := requestForBackend(req, backend)
	return domain.ProbeBackendSummary{
		Method:  backend.Method(),
		Name:    backend.Name(),
		Version: backend.Version(),
		Core:    resolved.Core,
	}, nil
}

func (e *Engine) SelectCore(req domain.ProbeRequest, nodes []domain.NodeIR) (string, bool) {
	req = normalizeRequest(req)
	if req.Method != domain.ProbeURLTest && req.Method != domain.ProbeUDPNTP {
		return "", false
	}
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
	case domain.ProbeTCPConnect:
		if len(candidates) == 0 {
			return nil, domain.NewError(domain.CodeProbeBackendUnavailable, fmt.Sprintf("%s probe backend is not registered", method))
		}
		return candidates[0], nil
	default:
		return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported probe method %q", method))
	}
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
	case "":
		return ""
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

func normalizeRequest(req domain.ProbeRequest) domain.ProbeRequest {
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

func targetFromRequest(req domain.ProbeRequest, node domain.NodeIR) string {
	switch req.Method {
	case domain.ProbeTCPConnect:
		if node.Server == "" || node.Port == 0 {
			return ""
		}
		return net.JoinHostPort(node.Server, strconv.Itoa(int(node.Port)))
	case domain.ProbeUDPNTP:
		return net.JoinHostPort(NTPServerFromRequest(req), "123")
	case domain.ProbeURLTest:
		return URLTestTarget(req)
	default:
		return ""
	}
}

func NTPServerFromRequest(req domain.ProbeRequest) string {
	server := strings.TrimSpace(req.NTPServer)
	if server == "" {
		return defaultNTPServer
	}
	return server
}

func URLTestTarget(req domain.ProbeRequest) string {
	if strings.TrimSpace(req.URL) != "" {
		return strings.TrimSpace(req.URL)
	}
	return DefaultURLTestURL
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
