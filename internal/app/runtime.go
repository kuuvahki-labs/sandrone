// Package app assembles Sandrone application runtime dependencies and server configuration.
package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

const (
	DefaultDataDir = "./data"
	DefaultListen  = "127.0.0.1:1137"
	DefaultMCPPath = "/mcp"
)

type Config struct {
	DataDir           string
	Storage           StorageConfig
	HTTP              HTTPConfig
	MCP               MCPConfig
	Log               LogConfig
	StoredSettings    domain.Settings
	EffectiveSettings domain.Settings
	OverrideSources   map[string]string
}

type HTTPConfig struct {
	Listen string
	Token  string
}

type MCPConfig struct {
	Path           string
	MaxOutputBytes int
}

type LogConfig struct {
	Level string
}

type Runtime struct {
	Service *service.Service
	Config  Config
	Logger  *slog.Logger
}

type StoreFactory func(context.Context, string, StorageConfig) (store.Store, error)

type runtimeOptions struct {
	schedulerEnabled bool
	probeEngine      service.ProbeEngine
	storeFactory     StoreFactory
}

type RuntimeOption func(*runtimeOptions)

func WithSchedulerEnabled(enabled bool) RuntimeOption {
	return func(options *runtimeOptions) {
		options.schedulerEnabled = enabled
	}
}

func WithProbeEngine(engine service.ProbeEngine) RuntimeOption {
	return func(options *runtimeOptions) {
		options.probeEngine = engine
	}
}

func WithStoreFactory(factory StoreFactory) RuntimeOption {
	return func(options *runtimeOptions) {
		if factory != nil {
			options.storeFactory = factory
		}
	}
}

type Entrypoint interface {
	Name() string
	Run(ctx context.Context, rt *Runtime) error
}

func NewRuntime(cfg Config, logger *slog.Logger) (*Runtime, error) {
	return NewRuntimeContext(context.Background(), cfg, logger)
}

func NewRuntimeContext(ctx context.Context, cfg Config, logger *slog.Logger, opts ...RuntimeOption) (*Runtime, error) {
	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir
	}
	if cfg.Storage.Backend == "" {
		cfg.Storage.Backend = StorageFilesystem
	}
	options := runtimeOptions{schedulerEnabled: true, storeFactory: NewStore}
	for _, option := range opts {
		option(&options)
	}
	cfg = withProgrammaticOverrideSources(cfg)
	rawStore, err := options.storeFactory(ctx, cfg.DataDir, cfg.Storage)
	if err != nil {
		return nil, err
	}
	coordinator := store.Coordinate(rawStore)
	settingsStore := store.NewSettingsStore(coordinator)
	cfg, err = ResolveSettings(ctx, cfg, settingsStore)
	if err != nil {
		return nil, err
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	if logger == nil {
		var err error
		logger, err = NewLogger(cfg.Log, os.Stderr)
		if err != nil {
			return nil, err
		}
	}
	serviceOptions := []service.Option{
		service.WithStore(coordinator),
		service.WithProjectSettings(settingsStore, cfg.StoredSettings, cfg.EffectiveSettings, cfg.OverrideSources),
		service.WithLogger(logger),
		service.WithSchedulerEnabled(options.schedulerEnabled),
	}
	if options.probeEngine != nil {
		serviceOptions = append(serviceOptions, service.WithProbeEngine(options.probeEngine))
	}
	svc := service.New(serviceOptions...)
	return &Runtime{
		Service: svc,
		Config:  cfg,
		Logger:  logger,
	}, nil
}

func Defaults(cfg Config) Config {
	defaults := projectsettings.Default()
	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir
	}
	if cfg.Storage.Backend == "" {
		cfg.Storage.Backend = StorageFilesystem
	}
	if cfg.HTTP.Listen == "" {
		cfg.HTTP.Listen = defaults.HTTP.Listen
	}
	if cfg.MCP.Path == "" {
		cfg.MCP.Path = defaults.MCP.Path
	}
	if cfg.MCP.MaxOutputBytes == 0 {
		cfg.MCP.MaxOutputBytes = defaults.MCP.MaxOutputBytes
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = defaults.Log.Level
	}
	return cfg
}

func Validate(cfg Config) error {
	if cfg.DataDir == "" {
		return domain.NewError(domain.CodeInvalidArgument, "data dir is required")
	}
	if cfg.HTTP.Listen != "" {
		host, _, err := net.SplitHostPort(cfg.HTTP.Listen)
		if err != nil {
			return domain.WrapError(domain.CodeInvalidArgument, "invalid HTTP listen address", err)
		}
		if !isLocalHost(host) && cfg.HTTP.Token == "" {
			return domain.NewError(domain.CodeInvalidArgument, "binding HTTP to a non-local address requires --token")
		}
	}
	if err := projectsettings.ValidateMCPPath(cfg.MCP.Path); err != nil {
		return err
	}
	if cfg.MCP.MaxOutputBytes < 0 {
		return domain.NewError(domain.CodeInvalidArgument, "MCP max output bytes must be non-negative")
	}
	if _, err := parseLogLevel(cfg.Log.Level); err != nil {
		return err
	}
	return nil
}

func NewLogger(cfg LogConfig, out io.Writer) (*slog.Logger, error) {
	if out == nil {
		out = io.Discard
	}
	level, err := parseLogLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: level})), nil
}

func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, domain.NewError(domain.CodeInvalidArgument, "unsupported log level "+level)
	}
}

func isLocalHost(host string) bool {
	host = strings.Trim(host, "[]")
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func IsContextDone(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
