package sandrone

import (
	"context"
	"time"

	"github.com/spf13/afero"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
	internalstore "github.com/kuuvahki-labs/sandrone/internal/store"
)

// Engine is the public façade around service.Service. Construct via New.
type Engine struct {
	service *service.Service
}

// Type aliases let external callers build requests and inspect results
// without importing internal packages directly.
type (
	NodeIR                      = domain.NodeIR
	NodeType                    = domain.NodeType
	Warning                     = domain.Warning
	WarningNodeContext          = domain.WarningNodeContext
	SourceRef                   = domain.SourceRef
	SourceInfo                  = domain.SourceInfo
	Subscription                = domain.Subscription
	SubscriptionType            = domain.SubscriptionType
	SubscriptionPreviewResult   = domain.SubscriptionPreviewResult
	SubscriptionPreviewNodeDiff = domain.SubscriptionPreviewNodeDiff
	SubscriptionTrafficRequest  = domain.SubscriptionTrafficRequest
	SubscriptionTrafficResult   = domain.SubscriptionTrafficResult
	SubscriptionRenderRequest   = domain.SubscriptionRenderRequest
	ResourceRef                 = domain.ResourceRef
	Report                      = domain.Report
	RenderReport                = domain.RenderReport
	ProbeReport                 = domain.ProbeReport
	RenderOptions               = domain.RenderOptions
	FileDocument                = domain.FileDocument
	FilePart                    = domain.FilePart
	FileSpec                    = domain.FileSpec
	FileKind                    = domain.FileKind
	FileConfig                  = domain.FileConfig
	FileSource                  = domain.FileSource
	RuntimeSettings             = domain.RuntimeSettings
	RemoteDefaults              = domain.RemoteDefaults
	ProbeDefaults               = domain.ProbeDefaults
	CacheDefaults               = domain.CacheDefaults
	NodeInput                   = domain.NodeInput
	NodeSet                     = domain.NodeSet
	NodeContext                 = domain.NodeContext
	ResponseInfo                = domain.ResponseInfo
	RequestInfo                 = domain.RequestInfo
	ProcessorSpec               = domain.ProcessorSpec
	Stage                       = domain.Stage
	ResourceSummary             = domain.ResourceSummary
	ResourceListResult          = domain.ResourceListResult
	Share                       = domain.Share
	ShareCreateRequest          = domain.ShareCreateRequest
	ShareListResult             = domain.ShareListResult
	ShareRenderRequest          = domain.ShareRenderRequest
	ShareRenderResult           = domain.ShareRenderResult
	ValidateRequest             = domain.ValidateRequest
	ValidateResult              = domain.ValidateResult
	ValidationCounts            = domain.ValidationCounts
	ValidationIssue             = domain.ValidationIssue
	InspectRequest              = domain.InspectRequest
	InspectResult               = domain.InspectResult
	MieruOptions                = domain.MieruOptions
	TLSOptions                  = domain.TLSOptions
	ECHOptions                  = domain.ECHOptions
	RealityOptions              = domain.RealityOptions
	TransportOptions            = domain.TransportOptions
	XHTTPTransportOptions       = domain.XHTTPTransportOptions
	XHTTPReuseSettings          = domain.XHTTPReuseSettings
	XHTTPDownloadSettings       = domain.XHTTPDownloadSettings
	SnellOptions                = domain.SnellOptions
	ShadowTLSOptions            = domain.ShadowTLSOptions
	AnyTLSOptions               = domain.AnyTLSOptions

	RemoteInput     = domain.RemoteInput
	ParseRequest    = domain.ParseRequest
	ParseResult     = domain.ParseResult
	RenderRequest   = domain.RenderRequest
	RenderResult    = domain.RenderResult
	ConvertRequest  = domain.ConvertRequest
	ProbeMethod     = domain.ProbeMethod
	ProbeRequest    = domain.ProbeRequest
	ProbeResult     = domain.ProbeResult
	NodeProbeResult = domain.NodeProbeResult
	FileRequest     = domain.FileRequest
	FileResult      = domain.FileResult
)

// Store is the storage boundary accepted by NewWithStore. Keys are safe
// relative slash-separated paths.
type Store interface {
	Read(ctx context.Context, key string) ([]byte, error)
	Write(ctx context.Context, key string, value []byte) error
	// CompareAndSwap atomically replaces the exact old bytes. A nil oldValue
	// matches a missing key and a nil newValue deletes the key.
	CompareAndSwap(ctx context.Context, key string, oldValue, newValue []byte) (bool, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]Entry, error)
	Stat(ctx context.Context, key string) (Entry, error)
}

type Entry struct {
	Key     string    `json:"key" yaml:"key"`
	Size    int64     `json:"size" yaml:"size"`
	IsDir   bool      `json:"is_dir" yaml:"is_dir"`
	ModTime time.Time `json:"mod_time" yaml:"mod_time"`
}

// Stage constants mirror the domain ones.
const (
	StageNodes = domain.StageNodes
	StageFile  = domain.StageFile

	FileKindStatic       = domain.FileKindStatic
	FileKindMihomo       = domain.FileKindMihomo
	FileKindSingBox      = domain.FileKindSingBox
	FileKindShadowrocket = domain.FileKindShadowrocket

	ProbeTCPConnect = domain.ProbeTCPConnect
	ProbeUDPNTP     = domain.ProbeUDPNTP
	ProbeURLTest    = domain.ProbeURLTest

	SubscriptionTypeRemote     = domain.SubscriptionTypeRemote
	SubscriptionTypeLocal      = domain.SubscriptionTypeLocal
	SubscriptionTypeCollection = domain.SubscriptionTypeCollection

	NodeTypeShadowsocks  = domain.NodeTypeShadowsocks
	NodeTypeShadowsocksR = domain.NodeTypeShadowsocksR
	NodeTypeVMess        = domain.NodeTypeVMess
	NodeTypeVLESS        = domain.NodeTypeVLESS
	NodeTypeTrojan       = domain.NodeTypeTrojan
	NodeTypeHysteria     = domain.NodeTypeHysteria
	NodeTypeHysteria2    = domain.NodeTypeHysteria2
	NodeTypeTUIC         = domain.NodeTypeTUIC
	NodeTypeMieru        = domain.NodeTypeMieru
	NodeTypeSOCKS        = domain.NodeTypeSOCKS
	NodeTypeHTTP         = domain.NodeTypeHTTP
	NodeTypeWireGuard    = domain.NodeTypeWireGuard
	NodeTypeSnell        = domain.NodeTypeSnell
	NodeTypeAnyTLS       = domain.NodeTypeAnyTLS
)

// New returns a new Engine with all built-in adapters and processors
// registered.
func New() *Engine { return &Engine{service: service.New()} }

// NewWithFS returns a new Engine backed by an afero filesystem for named
// resources.
func NewWithFS(fs afero.Fs) *Engine {
	return &Engine{service: service.New(service.WithFS(fs))}
}

// NewWithStore returns a new Engine backed by a caller-provided store.
func NewWithStore(resourceStore Store) *Engine {
	if resourceStore == nil {
		return New()
	}
	return &Engine{service: service.New(service.WithStore(storeAdapter{Store: resourceStore}))}
}

// Parse converts raw input bytes into NodeIR via the format-specific
// adapter and, optionally, the request's node-stage processor chain.
func (e *Engine) Parse(ctx context.Context, req ParseRequest) (*ParseResult, error) {
	return e.service.Parse(ctx, req)
}

// Render runs the request's node-stage processors (if any) and renders
// the resulting nodes via the adapter for the requested format.
func (e *Engine) Render(ctx context.Context, req RenderRequest) (*RenderResult, error) {
	return e.service.Render(ctx, req)
}

// Convert parses raw input and renders it to the requested target format.
func (e *Engine) Convert(ctx context.Context, req ConvertRequest) (*RenderResult, error) {
	return e.service.Convert(ctx, req)
}

// Probe runs runtime reachability checks over resolved node input.
func (e *Engine) Probe(ctx context.Context, req ProbeRequest) (*ProbeResult, error) {
	return e.service.Probe(ctx, req)
}

func (e *Engine) ValidateFile(ctx context.Context, req FileRequest) (*ValidateResult, error) {
	return e.service.ValidateFile(ctx, req)
}

func (e *Engine) ValidateNodes(ctx context.Context, req ParseRequest) (*ValidateResult, error) {
	return e.service.ValidateNodes(ctx, req)
}

func (e *Engine) Inspect(ctx context.Context, req InspectRequest) (*InspectResult, error) {
	return e.service.Inspect(ctx, req)
}

// GetFile runs the full file flow described in docs/architecture/file-pipeline.md.
func (e *Engine) GetFile(ctx context.Context, req FileRequest) (*FileResult, error) {
	return e.service.GetFile(ctx, req)
}

// GetFileSource resolves a saved file's input before typed compilation and
// file-stage processors are applied.
func (e *Engine) GetFileSource(ctx context.Context, name string) (*FileDocument, error) {
	return e.service.GetFileSource(ctx, name)
}

func (e *Engine) CapabilitySummary() map[string]any {
	return e.service.CapabilitySummary()
}

func (e *Engine) PutSubscription(ctx context.Context, sub Subscription) error {
	return e.service.PutSubscription(ctx, sub)
}

func (e *Engine) PreviewSubscription(ctx context.Context, name string) (*SubscriptionPreviewResult, error) {
	return e.service.PreviewSubscription(ctx, name)
}

func (e *Engine) SubscriptionTraffic(ctx context.Context, req SubscriptionTrafficRequest) (*SubscriptionTrafficResult, error) {
	return e.service.SubscriptionTraffic(ctx, req)
}

func (e *Engine) RenderSubscription(ctx context.Context, req SubscriptionRenderRequest) (*RenderResult, error) {
	return e.service.RenderSubscriptionRequest(ctx, req)
}

func (e *Engine) PutFile(ctx context.Context, file FileSpec) error {
	return e.service.PutFile(ctx, file)
}

func (e *Engine) GetRuntimeSettings(ctx context.Context) (RuntimeSettings, error) {
	return e.service.GetRuntimeSettings(ctx)
}

func (e *Engine) PutRuntimeSettings(ctx context.Context, settings RuntimeSettings) error {
	return e.service.PutRuntimeSettings(ctx, settings)
}

func (e *Engine) DeleteSubscription(ctx context.Context, name string) error {
	return e.service.DeleteSubscription(ctx, name)
}

func (e *Engine) DeleteFile(ctx context.Context, name string) error {
	return e.service.DeleteFile(ctx, name)
}

func (e *Engine) GetSubscription(ctx context.Context, name string) (*Subscription, error) {
	return e.service.GetSubscription(ctx, name)
}

func (e *Engine) ListSubscriptions(ctx context.Context) (*ResourceListResult, error) {
	return e.service.ListSubscriptions(ctx)
}

func (e *Engine) GetFileSpec(ctx context.Context, name string) (*FileSpec, error) {
	return e.service.GetFileSpec(ctx, name)
}

func (e *Engine) ListFiles(ctx context.Context) (*ResourceListResult, error) {
	return e.service.ListFiles(ctx)
}

func (e *Engine) CreateShare(ctx context.Context, req ShareCreateRequest) (*Share, error) {
	result, err := e.service.CreateShare(ctx, req)
	if err != nil {
		return nil, err
	}
	return &result.Share, nil
}

func (e *Engine) ListShares(ctx context.Context) (*ShareListResult, error) {
	return e.service.ListShares(ctx)
}

func (e *Engine) GetShare(ctx context.Context, id string) (*Share, error) {
	return e.service.GetShare(ctx, id)
}

func (e *Engine) DeleteShare(ctx context.Context, id string) error {
	return e.service.DeleteShare(ctx, id)
}

func (e *Engine) RenderShare(ctx context.Context, req ShareRenderRequest) (*ShareRenderResult, error) {
	return e.service.RenderShare(ctx, req)
}

// IsCode is a convenience alias for domain.IsCode so callers can inspect
// engine errors without importing internal packages.
func IsCode(err error, code string) bool {
	return domain.IsCode(err, domain.ErrorCode(code))
}

type storeAdapter struct {
	Store
}

func (s storeAdapter) CompareAndSwap(ctx context.Context, key string, oldValue, newValue []byte) (bool, error) {
	return s.Store.CompareAndSwap(ctx, key, oldValue, newValue)
}

func (s storeAdapter) List(ctx context.Context, prefix string) ([]internalstore.Entry, error) {
	entries, err := s.Store.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]internalstore.Entry, len(entries))
	for i, entry := range entries {
		out[i] = internalstore.Entry{
			Key:     entry.Key,
			Size:    entry.Size,
			IsDir:   entry.IsDir,
			ModTime: entry.ModTime,
		}
	}
	return out, nil
}

func (s storeAdapter) Stat(ctx context.Context, key string) (internalstore.Entry, error) {
	entry, err := s.Store.Stat(ctx, key)
	if err != nil {
		return internalstore.Entry{}, err
	}
	return internalstore.Entry{
		Key:     entry.Key,
		Size:    entry.Size,
		IsDir:   entry.IsDir,
		ModTime: entry.ModTime,
	}, nil
}
