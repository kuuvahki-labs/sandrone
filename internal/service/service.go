// Package service composes parser, renderer, processor chain and file
// resolution into the high-level operations exposed through pkg/sandrone and
// the entrypoint packages.
//
// It owns the processor registry and is the only place allowed to compose
// adapter + processor dependencies. Entry points (CLI, HTTP, MCP) call
// into Service and never reach adapter or processor packages directly.
package service

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/spf13/afero"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/jsonnodes"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shadowrocket"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/fetcher"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	fileproc "github.com/kuuvahki-labs/sandrone/internal/processor/file"
	nodeproc "github.com/kuuvahki-labs/sandrone/internal/processor/node"
	scriptproc "github.com/kuuvahki-labs/sandrone/internal/processor/script"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

// Parser is the interface adapters implement to convert raw bytes into NodeIR.
type Parser interface {
	Name() string
	Parse(ctx context.Context, in []byte) ([]domain.NodeIR, *domain.SourceInfo, error)
}

// Renderer is the interface adapters implement to convert NodeIR back into a
// target-specific representation.
type Renderer interface {
	Name() string
	Render(ctx context.Context, nodes []domain.NodeIR, opt domain.RenderOptions) ([]byte, error)
}

type reportingRenderer interface {
	Renderer
	RenderWithReport(ctx context.Context, nodes []domain.NodeIR, opt domain.RenderOptions) ([]byte, domain.RenderReport, error)
}

type parseCapabilityProvider interface {
	ParseCapabilities() []shared.Capability
}

type renderCapabilityProvider interface {
	RenderCapabilities() []shared.Capability
}

type ProbeEngine interface {
	Probe(ctx context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, payloads ...probe.Payload) (*domain.ProbeResult, error)
}

type probeCoreSelector interface {
	SelectCore(req domain.ProbeRequest, nodes []domain.NodeIR) (string, bool)
}

// Service is the central orchestrator. It composes adapters and the
// processor registry; consumers receive a value via New().
type Service struct {
	parsers           map[string]Parser
	renderers         map[string]Renderer
	uriParser         *uri.Parser
	registry          *processor.Registry
	typedFiles        *typedFileRegistry
	prober            ProbeEngine
	cache             cachepkg.Cache
	store             store.Store
	storeCoordinator  store.Coordinator
	metaStore         *store.MetaStore
	settingsStore     *store.SettingsStore
	fetcher           *fetcher.Fetcher
	logger            *slog.Logger
	now               func() time.Time
	catalog           func() (*ruleSetCatalogSnapshot, error)
	settingsMu        sync.RWMutex
	storedSettings    domain.Settings
	effectiveSettings domain.Settings
	settingsOverrides map[string]string
}

// Option lets callers customise Service construction.
type Option func(*Service)

// WithProcessor adds an extra ProcessorSpec hook to the registry.
// (Currently a thin wrapper around the registry; kept here so callers do not
// need to import the processor package directly.)
func WithProcessor(register func(*processor.Registry)) Option {
	return func(s *Service) {
		if register != nil {
			register(s.registry)
		}
	}
}

// WithStore injects the resource store used for named specs, metadata and
// local file inputs.
func WithStore(resourceStore store.Store) Option {
	return func(s *Service) {
		if resourceStore == nil {
			return
		}
		coordinator := store.Coordinate(resourceStore)
		s.store = coordinator
		s.storeCoordinator = coordinator
		s.metaStore = store.NewMetaStore(coordinator)
		s.settingsStore = store.NewSettingsStore(coordinator)
	}
}

func WithProjectSettings(repository *store.SettingsStore, stored, effective domain.Settings, overrides map[string]string) Option {
	return func(s *Service) {
		s.settingsStore = repository
		s.storedSettings = stored
		s.effectiveSettings = effective
		s.settingsOverrides = cloneStringMap(overrides)
	}
}

func WithCache(resultCache cachepkg.Cache) Option {
	return func(s *Service) {
		if resultCache != nil {
			s.cache = resultCache
		}
	}
}

func WithProbeEngine(prober ProbeEngine) Option {
	return func(s *Service) {
		if prober != nil {
			s.prober = prober
		}
	}
}

func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

func WithFetcher(remoteFetcher *fetcher.Fetcher) Option {
	return func(s *Service) {
		if remoteFetcher != nil {
			s.fetcher = remoteFetcher
		}
	}
}

// WithFS wraps an afero filesystem in the default store implementation.
func WithFS(fs afero.Fs) Option {
	return WithStore(store.NewFSStore(fs))
}

// New constructs a Service with all built-in adapters and processors
// registered. Pass Options to extend behaviour.
func New(opts ...Option) *Service {
	uriParser := uri.NewParser()
	mihomoParser := mihomo.NewParser()
	singBoxParser := singbox.NewParser()
	jsonParser := jsonnodes.NewParser()
	uriRenderer := uri.NewRenderer()
	base64Renderer := uri.NewBase64Renderer(uriRenderer)
	mihomoRenderer := mihomo.NewRenderer()
	singBoxRenderer := singbox.NewRenderer()
	jsonRenderer := jsonnodes.NewRenderer()
	shadowrocketRenderer := shadowrocket.NewRenderer()

	registry := processor.NewRegistry()
	typedFiles := newTypedFileRegistry()
	for _, driver := range []typedFileDriver{mihomoFileDriver{}, singBoxFileDriver{}, shadowrocketFileDriver{}} {
		if err := typedFiles.Register(driver); err != nil {
			panic(err)
		}
	}
	s := &Service{
		parsers: map[string]Parser{
			"uri":        uriParser,
			"uri-list":   uriParser,
			"base64":     uriParser,
			"mihomo":     mihomoParser,
			"sing-box":   singBoxParser,
			"json-nodes": jsonParser,
		},
		renderers: map[string]Renderer{
			"base64":               base64Renderer,
			"mihomo-proxies":       mihomoRenderer,
			"shadowrocket-proxies": shadowrocketRenderer,
			"sing-box-outbounds":   singBoxRenderer,
			"json-nodes":           jsonRenderer,
			"uri-list":             uriRenderer,
		},
		uriParser:  uriParser,
		registry:   registry,
		typedFiles: typedFiles,
		prober:     probe.New(),
		fetcher:    fetcher.New(),
		now:        time.Now,
	}
	nodeproc.Register(registry, s)
	fileproc.Register(registry)
	scriptproc.Register(registry, scriptproc.WithProbeRunner(s), scriptproc.WithResourceResolver(s), scriptproc.WithLoader(s.loadScriptSource))
	for _, opt := range opts {
		opt(s)
	}
	if s.storedSettings.SchemaVersion == 0 {
		s.storedSettings = projectsettings.Default()
	}
	if s.effectiveSettings.SchemaVersion == 0 {
		s.effectiveSettings = s.storedSettings
	}
	if s.settingsOverrides == nil {
		s.settingsOverrides = map[string]string{}
	}
	if s.cache == nil && s.store != nil {
		s.cache = cachepkg.New(s.store, s.now)
	}
	return s
}

// Registry returns the processor registry. Exposed so callers needing fine
// control over which processors are available can mutate it before issuing
// requests; for the typical user this is unnecessary.
func (s *Service) Registry() *processor.Registry { return s.registry }

func (s *Service) CapabilitySummary() map[string]any {
	summary := map[string]any{
		"parse_formats": []string{
			"uri", "uri-list", "base64", "mihomo", "sing-box", "json-nodes",
		},
		"render_formats": []string{
			"base64", "mihomo-proxies", "shadowrocket-proxies", "sing-box-outbounds", "json-nodes", "uri-list",
		},
		"node_processors": s.registry.NodeTypes(),
		"file_processors": s.registry.FileTypes(),
		"probe_methods":   []string{string(domain.ProbeTCPConnect), string(domain.ProbeUDPNTP), string(domain.ProbeURLTest)},
		"capabilities":    s.adapterCapabilities(),
	}
	if inspector, ok := s.prober.(interface{ BackendSummary() []map[string]string }); ok {
		summary["probe_backends"] = inspector.BackendSummary()
	}
	return summary
}

func (s *Service) adapterCapabilities() []shared.Capability {
	capabilities := []shared.Capability{}
	seen := map[string]bool{}
	for _, parser := range s.parsers {
		if provider, ok := parser.(parseCapabilityProvider); ok {
			capabilities = appendUniqueCapabilities(capabilities, seen, provider.ParseCapabilities())
		}
	}
	for _, renderer := range s.renderers {
		if provider, ok := renderer.(renderCapabilityProvider); ok {
			capabilities = appendUniqueCapabilities(capabilities, seen, provider.RenderCapabilities())
		}
	}
	sort.Slice(capabilities, func(i, j int) bool {
		if capabilities[i].Direction != capabilities[j].Direction {
			return capabilities[i].Direction < capabilities[j].Direction
		}
		return capabilities[i].Format < capabilities[j].Format
	})
	return capabilities
}

func appendUniqueCapabilities(dst []shared.Capability, seen map[string]bool, capabilities []shared.Capability) []shared.Capability {
	for _, capability := range capabilities {
		key := capability.Format + "\x00" + string(capability.Direction)
		if capability.Format == "" || seen[key] {
			continue
		}
		seen[key] = true
		dst = append(dst, capability)
	}
	return dst
}
