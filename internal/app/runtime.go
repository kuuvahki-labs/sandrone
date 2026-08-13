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

	"github.com/spf13/afero"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
	"github.com/kuuvahki-labs/sandrone/internal/store"
	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

const (
	DefaultDataDir = "./data"
	DefaultListen  = "127.0.0.1:1137"
	DefaultMCPPath = "/mcp"
)

type Config struct {
	DataDir           string
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
	Path                 string
	AllowManagementTools bool
	MaxOutputBytes       int
}

type LogConfig struct {
	Level string
}

type Runtime struct {
	Service *service.Service
	Engine  *sandrone.Engine
	Config  Config
	Logger  *slog.Logger
}

type Entrypoint interface {
	Name() string
	Run(ctx context.Context, rt *Runtime) error
}

func NewRuntime(cfg Config, logger *slog.Logger) (*Runtime, error) {
	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir
	}
	cfg = withProgrammaticOverrideSources(cfg)
	fs := afero.NewBasePathFs(afero.NewOsFs(), cfg.DataDir)
	coordinator := store.Coordinate(store.NewFSStore(fs))
	settingsStore := store.NewSettingsStore(coordinator)
	var err error
	cfg, err = ResolveSettings(context.Background(), cfg, settingsStore)
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
	svc := service.New(
		service.WithStore(coordinator),
		service.WithProjectSettings(settingsStore, cfg.StoredSettings, cfg.EffectiveSettings, cfg.OverrideSources),
		service.WithLogger(logger),
		service.WithSchedulerEnabled(true),
	)
	return &Runtime{
		Service: svc,
		Engine:  sandrone.NewWithFS(fs),
		Config:  cfg,
		Logger:  logger,
	}, nil
}

func Defaults(cfg Config) Config {
	defaults := projectsettings.Default()
	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir
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
