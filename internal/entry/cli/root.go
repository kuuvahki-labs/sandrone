package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

const defaultDataDir = "./data"

type engine interface {
	Parse(context.Context, sandrone.ParseRequest) (*sandrone.ParseResult, error)
	Render(context.Context, sandrone.RenderRequest) (*sandrone.RenderResult, error)
	Convert(context.Context, sandrone.ConvertRequest) (*sandrone.RenderResult, error)
	Probe(context.Context, sandrone.ProbeRequest) (*sandrone.ProbeResult, error)
	GetFile(context.Context, sandrone.FileRequest) (*sandrone.FileResult, error)
	ValidateFile(context.Context, sandrone.FileRequest) (*sandrone.ValidateResult, error)
	ValidateNodes(context.Context, sandrone.ParseRequest) (*sandrone.ValidateResult, error)
	Inspect(context.Context, sandrone.InspectRequest) (*sandrone.InspectResult, error)
}

type engineFactory func(dataDir string) engine

type config struct {
	stdin          io.Reader
	stdout         io.Writer
	stderr         io.Writer
	env            map[string]string
	dataDir        string
	engineFactory  engineFactory
	runtimeFactory runtimeFactory
}

type runtimeFactory func(app.Config) (*app.Runtime, error)

// Option customizes a CLI command tree. Tests use it to inject streams and a
// recording engine without changing command behavior.
type Option func(*config)

func WithStreams(stdin io.Reader, stdout, stderr io.Writer) Option {
	return func(cfg *config) {
		if stdin != nil {
			cfg.stdin = stdin
		}
		if stdout != nil {
			cfg.stdout = stdout
		}
		if stderr != nil {
			cfg.stderr = stderr
		}
	}
}

func WithEnv(env map[string]string) Option {
	return func(cfg *config) {
		cfg.env = env
	}
}

func WithEngineFactory(factory func(dataDir string) engine) Option {
	return func(cfg *config) {
		if factory != nil {
			cfg.engineFactory = factory
		}
	}
}

func WithRuntimeFactory(factory func(app.Config) (*app.Runtime, error)) Option {
	return func(cfg *config) {
		if factory != nil {
			cfg.runtimeFactory = factory
		}
	}
}

// Execute builds and runs the production CLI against process stdio and args.
func Execute(ctx context.Context) int {
	return ExecuteContext(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}

// ExecuteContext is the testable execution boundary used by main and command
// tests. It prints command errors to stderr and returns a process-style code.
func ExecuteContext(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return ExecuteWithOptions(ctx, args, WithStreams(stdin, stdout, stderr))
}

func ExecuteWithOptions(ctx context.Context, args []string, opts ...Option) int {
	cfg := newConfig(opts...)
	cmd := newRootCommand(cfg)
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(cfg.stderr, err)
		return 1
	}
	return 0
}

func NewRootCommand(opts ...Option) *cobra.Command {
	cfg := newConfig(opts...)
	return newRootCommand(cfg)
}

func newRootCommand(cfg *config) *cobra.Command {
	root := &cobra.Command{
		Use:           "sandrone",
		Short:         "Convert proxy nodes and generated client files",
		Version:       buildinfo.Version(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetIn(cfg.stdin)
	root.SetOut(cfg.stdout)
	root.SetErr(cfg.stderr)
	root.PersistentFlags().StringVar(&cfg.dataDir, "data-dir", cfg.dataDir, "resource data directory")
	root.AddCommand(
		newConvertCommand(cfg),
		newProbeCommand(cfg),
		newValidateCommand(cfg),
		newInspectCommand(cfg),
		newFileCommand(cfg),
		newDoctorCommand(cfg),
		newServeCommand(cfg),
	)
	return root
}

func newConfig(opts ...Option) *config {
	cfg := &config{
		stdin:         os.Stdin,
		stdout:        os.Stdout,
		stderr:        os.Stderr,
		env:           osEnv(),
		engineFactory: newEngine,
		runtimeFactory: func(cfg app.Config) (*app.Runtime, error) {
			return app.NewRuntime(cfg, nil)
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	cfg.dataDir = firstNonEmpty(cfg.env[EnvDataDir], defaultDataDir)
	return cfg
}

func osEnv() map[string]string {
	env := map[string]string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

func newEngine(dataDir string) engine {
	fs := afero.NewBasePathFs(afero.NewOsFs(), dataDir)
	return sandrone.NewWithFS(fs)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
