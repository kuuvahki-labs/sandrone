package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
	"github.com/kuuvahki-labs/sandrone/internal/entry/webui"
)

type serveOptions struct {
	listen    string
	token     string
	path      string
	maxOutput int
	logLevel  string
}

func newServeCommand(cfg *config) *cobra.Command {
	opts := serveOptions{
		listen:    firstNonEmpty(cfg.env[EnvListen], app.DefaultListen),
		token:     cfg.env[EnvToken],
		path:      firstNonEmpty(cfg.env[EnvMCPPath], app.DefaultMCPPath),
		maxOutput: 1 << 20,
		logLevel:  firstNonEmpty(cfg.env[EnvLogLevel], "info"),
	}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run HTTP API, Web UI, and MCP on one listener",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newServeRuntime(cmd, cfg, opts)
			if err != nil {
				return err
			}
			return runEntrypoint(cmd.Context(), newServeServer(rt), rt)
		},
	}
	cmd.Flags().StringVar(&opts.listen, "listen", opts.listen, "listen address")
	cmd.Flags().StringVar(&opts.token, "token", opts.token, "bearer token for HTTP and MCP HTTP")
	cmd.Flags().StringVar(&opts.logLevel, "log-level", opts.logLevel, "log level: debug, info, warn, or error")
	addMCPFlags(cmd, &opts)
	return cmd
}

func newServeServer(rt *app.Runtime) *httpapi.Server {
	mcpServer := mcpapi.New(rt)
	return httpapi.New(
		rt,
		httpapi.WithMCP(rt.Config.MCP.Path, mcpServer.Handler()),
		httpapi.WithWebUI(newWebUIHandler(rt.Config)),
	)
}

func addMCPFlags(cmd *cobra.Command, opts *serveOptions) {
	cmd.Flags().StringVar(&opts.path, "path", opts.path, "MCP HTTP path")
	cmd.Flags().IntVar(&opts.maxOutput, "max-output-bytes", opts.maxOutput, "maximum MCP inline output bytes")
}

func newServeRuntime(cmd *cobra.Command, cfg *config, opts serveOptions) (*app.Runtime, error) {
	if !flagChanged(cmd, "max-output-bytes") {
		value, err := environmentInt(cfg.env, EnvMCPMaxOutputBytes, opts.maxOutput)
		if err != nil {
			return nil, err
		}
		opts.maxOutput = value
	}
	overrideSources := startupOverrideSources(cmd, cfg.env)
	storageConfig, err := app.StorageConfigFromEnv(cfg.env)
	if err != nil {
		return nil, err
	}
	appCfg := app.Config{
		DataDir:         cfg.dataDir,
		Storage:         storageConfig,
		OverrideSources: overrideSources,
		HTTP: app.HTTPConfig{
			Listen: opts.listen,
			Token:  opts.token,
		},
		MCP: app.MCPConfig{
			Path:           opts.path,
			MaxOutputBytes: opts.maxOutput,
		},
		Log: app.LogConfig{
			Level: opts.logLevel,
		},
	}
	rt, err := cfg.runtimeFactory(appCfg)
	if err != nil {
		return nil, err
	}
	if rt == nil {
		return nil, errors.New("runtime factory returned nil")
	}
	return rt, nil
}

func startupOverrideSources(cmd *cobra.Command, env map[string]string) map[string]string {
	sources := map[string]string{}
	for _, item := range []struct {
		path   string
		envKey string
		flag   string
	}{
		{path: "http.listen", envKey: EnvListen, flag: "listen"},
		{path: "mcp.path", envKey: EnvMCPPath, flag: "path"},
		{path: "mcp.max_output_bytes", envKey: EnvMCPMaxOutputBytes, flag: "max-output-bytes"},
		{path: "log.level", envKey: EnvLogLevel, flag: "log-level"},
	} {
		if env[item.envKey] != "" {
			sources[item.path] = "environment"
		}
		if flagChanged(cmd, item.flag) {
			sources[item.path] = "flag"
		}
	}
	return sources
}

func flagChanged(cmd *cobra.Command, name string) bool {
	flag := cmd.Flags().Lookup(name)
	return flag != nil && flag.Changed
}

func environmentInt(env map[string]string, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(env[key])
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return value, nil
}

func newWebUIHandler(cfg app.Config) http.Handler {
	return webui.Handler(
		webui.WithReservedPrefixes("/v1", cfg.MCP.Path, "/s"),
	)
}

func runEntrypoint(ctx context.Context, entry app.Entrypoint, rt *app.Runtime) error {
	schedulerCtx, stopScheduler := context.WithCancel(ctx)
	schedulerDone := make(chan struct{})
	go func() {
		rt.Service.RunScheduledRefresh(schedulerCtx)
		close(schedulerDone)
	}()
	err := entry.Run(ctx, rt)
	stopScheduler()
	<-schedulerDone
	if app.IsContextDone(err) || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
