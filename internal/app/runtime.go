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
	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

const (
	DefaultDataDir = "./data"
	DefaultListen  = "127.0.0.1:1137"
	DefaultMCPPath = "/mcp"
)

type Config struct {
	DataDir string
	HTTP    HTTPConfig
	MCP     MCPConfig
	WebUI   WebUIConfig
	Log     LogConfig
}

type HTTPConfig struct {
	Listen        string
	Token         string
	TokenRequired bool
}

type MCPConfig struct {
	Transport            string
	Path                 string
	AllowManagementTools bool
	MaxOutputBytes       int
}

type WebUIConfig struct {
	StaticDir string
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
	cfg = Defaults(cfg)
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
	fs := afero.NewBasePathFs(afero.NewOsFs(), cfg.DataDir)
	svc := service.New(service.WithFS(fs), service.WithLogger(logger))
	return &Runtime{
		Service: svc,
		Engine:  sandrone.NewWithFS(fs),
		Config:  cfg,
		Logger:  logger,
	}, nil
}

func Defaults(cfg Config) Config {
	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir
	}
	if cfg.HTTP.Listen == "" {
		cfg.HTTP.Listen = DefaultListen
	}
	if cfg.MCP.Transport == "" {
		cfg.MCP.Transport = "stdio"
	}
	if cfg.MCP.Path == "" {
		cfg.MCP.Path = DefaultMCPPath
	}
	if cfg.MCP.MaxOutputBytes == 0 {
		cfg.MCP.MaxOutputBytes = 1 << 20
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
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
		if !isLocalHost(host) && cfg.HTTP.Token == "" && !cfg.HTTP.TokenRequired {
			return domain.NewError(domain.CodeInvalidArgument, "binding HTTP to a non-local address requires --token")
		}
	}
	if cfg.HTTP.TokenRequired && cfg.HTTP.Token == "" {
		return domain.NewError(domain.CodeInvalidArgument, "--token is required when token auth is enabled")
	}
	switch normalizeTransport(cfg.MCP.Transport) {
	case "stdio", "streamable-http":
	default:
		return domain.NewError(domain.CodeInvalidArgument, "unsupported MCP transport "+cfg.MCP.Transport)
	}
	if cfg.MCP.Path != "" && !strings.HasPrefix(cfg.MCP.Path, "/") {
		return domain.NewError(domain.CodeInvalidArgument, "MCP path must start with /")
	}
	if cfg.MCP.MaxOutputBytes < 0 {
		return domain.NewError(domain.CodeInvalidArgument, "MCP max output bytes must be non-negative")
	}
	if _, err := parseLogLevel(cfg.Log.Level); err != nil {
		return err
	}
	if cfg.WebUI.StaticDir != "" {
		info, err := os.Stat(cfg.WebUI.StaticDir)
		if err != nil {
			return domain.WrapError(domain.CodeInvalidArgument, "invalid Web UI static dir", err)
		}
		if !info.IsDir() {
			return domain.NewError(domain.CodeInvalidArgument, "Web UI static dir must be a directory")
		}
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

func TokenRequired(cfg HTTPConfig) bool {
	if cfg.TokenRequired {
		return true
	}
	return cfg.Token != ""
}

func normalizeTransport(transport string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(transport)), "_", "-")
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
