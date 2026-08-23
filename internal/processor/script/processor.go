package script

import (
	"context"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

func parseConfig(spec domain.ProcessorSpec) (Config, error) {
	var cfg Config
	if err := processor.UnmarshalParams(spec, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Engine == "" {
		cfg.Engine = "js"
	}
	if cfg.Engine != "js" {
		return cfg, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "only engine=js is supported",
			Processor: spec.Type,
		}
	}
	if err := normalizeScriptSource(&cfg, spec.Type); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func normalizeScriptSource(cfg *Config, processorType string) error {
	cfg.Source.Type = strings.ToLower(strings.TrimSpace(cfg.Source.Type))
	cfg.Source.Name = strings.TrimSpace(cfg.Source.Name)
	cfg.Source.SHA256 = strings.ToLower(strings.TrimSpace(cfg.Source.SHA256))
	structured := cfg.Source.Type != "" || cfg.Source.Content != "" || cfg.Source.Name != "" || cfg.Source.argsPresent || cfg.Source.Args != nil || cfg.Source.Remote != nil || cfg.Source.SHA256 != ""
	legacyContent := strings.TrimSpace(cfg.Content) != ""
	legacyPath := strings.TrimSpace(cfg.Path) != ""
	if structured && (legacyContent || legacyPath) {
		return &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "script requires exactly one source",
			Processor: processorType,
		}
	}
	if !structured {
		switch {
		case legacyContent && legacyPath:
			return &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   "script requires exactly one of path or content",
				Processor: processorType,
			}
		case legacyContent:
			cfg.Source = ScriptSource{Type: "inline", Content: cfg.Content}
			return nil
		case legacyPath:
			cfg.Path = strings.TrimSpace(cfg.Path)
			cfg.Source = ScriptSource{Type: "file", Name: cfg.Path}
			return nil
		default:
			return &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   "script requires exactly one source",
				Processor: processorType,
			}
		}
	}
	if cfg.Source.Args != nil && cfg.Source.Type != "file" {
		return &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "source.args is allowed only for file script sources",
			Processor: processorType,
		}
	}
	switch cfg.Source.Type {
	case "inline":
		if strings.TrimSpace(cfg.Source.Content) == "" {
			return &domain.AppError{Code: domain.CodeProcessorConfigInvalid, Message: "inline script source requires content", Processor: processorType}
		}
	case "file":
		if cfg.Source.Name == "" {
			return &domain.AppError{Code: domain.CodeProcessorConfigInvalid, Message: "file script source requires name", Processor: processorType}
		}
	case "remote":
		if cfg.Source.Remote == nil || strings.TrimSpace(cfg.Source.Remote.URL) == "" {
			return &domain.AppError{Code: domain.CodeProcessorConfigInvalid, Message: "remote script source requires remote.url", Processor: processorType}
		}
	default:
		return &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "script source type must be inline, file, or remote",
			Processor: processorType,
		}
	}
	return nil
}

type ProbeRunner interface {
	Probe(ctx context.Context, req domain.ProbeRequest) (*domain.ProbeResult, error)
}

type ResourceResolver interface {
	ProduceSubscription(ctx context.Context, name string, opts domain.ScriptProduceOptions) (*domain.ScriptSubscriptionProduceResult, error)
	FileContent(ctx context.Context, name string, opts domain.ScriptProduceOptions) (string, error)
}

type registerConfig struct {
	probeRunner      ProbeRunner
	resourceResolver ResourceResolver
	loader           Loader
	defaultTimeout   func() time.Duration
}

type RegisterOption func(*registerConfig)

func WithProbeRunner(runner ProbeRunner) RegisterOption {
	return func(cfg *registerConfig) {
		cfg.probeRunner = runner
	}
}

func WithResourceResolver(resolver ResourceResolver) RegisterOption {
	return func(cfg *registerConfig) {
		cfg.resourceResolver = resolver
	}
}

func WithLoader(loader Loader) RegisterOption {
	return func(cfg *registerConfig) {
		cfg.loader = loader
	}
}

// WithDefaultTimeout resolves the execution timeout used when a script does
// not provide a positive params.timeout_ms override.
func WithDefaultTimeout(resolve func() time.Duration) RegisterOption {
	return func(cfg *registerConfig) {
		cfg.defaultTimeout = resolve
	}
}

// NodeProcessor is the node-stage script processor.
type NodeProcessor struct {
	cfg    Config
	runner *runner
}

func buildNodeProcessorWithProbe(runner ProbeRunner, resolver ResourceResolver, loader Loader, resolveDefaultTimeout func() time.Duration) processor.NodeBuilder {
	return func(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
		cfg, err := parseConfig(spec)
		if err != nil {
			return nil, err
		}
		applyDefaultTimeout(&cfg, resolveDefaultTimeout)
		warnings := []domain.Warning{}
		logs := []string{}
		r, err := newRunner(cfg, newScriptAPI(cfg, &warnings, &logs, runner, resolver), loader)
		if err != nil {
			return nil, err
		}
		return &NodeProcessor{cfg: cfg, runner: r}, nil
	}
}

func (p *NodeProcessor) Name() string { return "script" }

func (p *NodeProcessor) ApplyNodes(ctx context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	scriptNodes, err := nodesToScript(in.Nodes)
	if err != nil {
		return domain.NodeProcessOutput{}, &domain.AppError{
			Code:    domain.CodeScriptRuntime,
			Message: "encode nodes for script",
			Cause:   err,
		}
	}
	env := ScriptEnvelope{
		Version: 1,
		Stage:   string(domain.StageNodes),
		Target:  in.Target,
		Context: in.Context,
		Request: in.Request,
		Args:    requestArgs(in.Request),
		Nodes:   scriptNodes,
	}
	updated, err := p.runner.run(ctx, env)
	if err != nil {
		return domain.NodeProcessOutput{}, err
	}
	nodes, warnings, err := scriptToNodes(updated.Nodes)
	if err != nil {
		return domain.NodeProcessOutput{}, &domain.AppError{
			Code:    domain.CodeScriptRuntime,
			Message: "decode script nodes",
			Cause:   err,
		}
	}
	warnings = append(warnings, updated.Warnings...)
	return domain.NodeProcessOutput{Nodes: nodes, Warnings: warnings}, nil
}

// FileProcessor is the file-stage script processor.
type FileProcessor struct {
	cfg    Config
	runner *runner
}

func buildFileProcessorWithResources(resolver ResourceResolver, loader Loader, resolveDefaultTimeout func() time.Duration) processor.FileBuilder {
	return func(spec domain.ProcessorSpec) (domain.FileProcessor, error) {
		cfg, err := parseConfig(spec)
		if err != nil {
			return nil, err
		}
		applyDefaultTimeout(&cfg, resolveDefaultTimeout)
		warnings := []domain.Warning{}
		logs := []string{}
		r, err := newRunner(cfg, newScriptAPI(cfg, &warnings, &logs, nil, resolver), loader)
		if err != nil {
			return nil, err
		}
		return &FileProcessor{cfg: cfg, runner: r}, nil
	}
}

func applyDefaultTimeout(cfg *Config, resolve func() time.Duration) {
	if cfg.TimeoutMS > 0 || resolve == nil {
		return
	}
	if timeout := resolve(); timeout > 0 {
		cfg.TimeoutMS = int(timeout / time.Millisecond)
	}
}

func (p *FileProcessor) Name() string { return "script" }

func (p *FileProcessor) ApplyFile(ctx context.Context, in domain.FileProcessInput) (domain.FileProcessOutput, error) {
	scriptParts, err := partsToScript(in.Parts)
	if err != nil {
		return domain.FileProcessOutput{File: in.File}, &domain.AppError{
			Code:    domain.CodeScriptRuntime,
			Message: "encode parts for script",
			Cause:   err,
		}
	}
	env := ScriptEnvelope{
		Version: 1,
		Stage:   string(domain.StageFile),
		Target:  in.Target,
		Request: in.Request,
		Args:    requestArgs(in.Request),
		File:    fileToScript(in.File),
		Parts:   scriptParts,
	}
	updated, err := p.runner.run(ctx, env)
	if err != nil {
		return domain.FileProcessOutput{File: in.File}, err
	}
	doc := scriptToFile(updated.File, in.File)
	return domain.FileProcessOutput{File: doc, Warnings: updated.Warnings}, nil
}

// Register installs script processors for nodes and file stages. Because the type
// name "script" is shared across stages, ProcessorSpec.Stage must be set
// explicitly when used; the registry's ResolveStage will report
// processor_config_invalid otherwise.
func Register(r *processor.Registry, opts ...RegisterOption) {
	cfg := registerConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	descriptor := processor.Descriptor{
		Description:     "Run sandboxed JavaScript against a processor envelope.",
		ParamsPrototype: Config{},
		Effects: processor.Effects{
			RemoteReads: true, RunsScript: true,
		},
		Examples: []map[string]any{{
			"engine": "js",
			"source": map[string]any{
				"type":    "inline",
				"content": "function main(input) { return input; }",
			},
		}},
		ErrorCodes: []domain.ErrorCode{
			domain.CodeProcessorConfigInvalid,
			domain.CodeScriptTimeout,
			domain.CodeScriptRuntime,
		},
		Public: true,
	}
	nodeDescriptor := descriptor
	nodeDescriptor.Effects.Probes = true
	r.RegisterNodeWithDescriptor("script", buildNodeProcessorWithProbe(cfg.probeRunner, cfg.resourceResolver, cfg.loader, cfg.defaultTimeout), nodeDescriptor)
	r.RegisterFileWithDescriptor("script", buildFileProcessorWithResources(cfg.resourceResolver, cfg.loader, cfg.defaultTimeout), descriptor)
}

func requestArgs(req domain.RequestInfo) map[string]any {
	if len(req.Args) == 0 {
		return nil
	}
	out := make(map[string]any, len(req.Args))
	for key, value := range req.Args {
		out[key] = value
	}
	return out
}
