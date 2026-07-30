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
	listen         string
	token          string
	webUIStaticDir string
	path           string
	management     bool
	maxOutput      int
	logLevel       string
}

func newServeCommand(cfg *config) *cobra.Command {
	opts := serveOptions{
		listen:         firstNonEmpty(cfg.env[EnvListen], app.DefaultListen),
		token:          cfg.env[EnvToken],
		webUIStaticDir: cfg.env[EnvWebUIStaticDir],
		path:           firstNonEmpty(cfg.env[EnvMCPPath], app.DefaultMCPPath),
		maxOutput:      1 << 20,
		logLevel:       firstNonEmpty(cfg.env[EnvLogLevel], "info"),
	}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run HTTP API and MCP entrypoints",
	}
	cmd.PersistentFlags().StringVar(&opts.listen, "listen", opts.listen, "listen address")
	cmd.PersistentFlags().StringVar(&opts.token, "token", opts.token, "bearer token for HTTP and MCP HTTP")
	cmd.PersistentFlags().StringVar(&opts.webUIStaticDir, "webui-static-dir", opts.webUIStaticDir, "directory containing Web UI static assets")
	cmd.PersistentFlags().StringVar(&opts.logLevel, "log-level", opts.logLevel, "log level: debug, info, warn, or error")
	cmd.AddCommand(
		newServeHTTPCommand(cfg, &opts),
		newServeMCPCommand(cfg, &opts),
		newServeAllCommand(cfg, &opts),
	)
	return cmd
}

func newServeHTTPCommand(cfg *config, opts *serveOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "http",
		Short: "Run the HTTP API",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newServeRuntime(cmd, cfg, *opts)
			if err != nil {
				return err
			}
			server := httpapi.New(rt, httpapi.WithWebUI(newWebUIHandler(rt.Config)))
			return runEntrypoint(cmd.Context(), server, rt)
		},
	}
}

func newServeMCPCommand(cfg *config, opts *serveOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run the MCP streamable HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newServeRuntime(cmd, cfg, *opts)
			if err != nil {
				return err
			}
			mcpServer := mcpapi.New(rt)
			httpServer := httpapi.New(rt, httpapi.WithMCP(rt.Config.MCP.Path, mcpServer.Handler()))
			return runEntrypoint(cmd.Context(), httpServer, rt)
		},
	}
	addMCPFlags(cmd, opts)
	return cmd
}

func newServeAllCommand(cfg *config, opts *serveOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Run HTTP API and MCP streamable HTTP on one listener",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newServeRuntime(cmd, cfg, *opts)
			if err != nil {
				return err
			}
			mcpServer := mcpapi.New(rt)
			httpServer := httpapi.New(rt, httpapi.WithMCP(rt.Config.MCP.Path, mcpServer.Handler()), httpapi.WithWebUI(newWebUIHandler(rt.Config)))
			return runEntrypoint(cmd.Context(), httpServer, rt)
		},
	}
	addMCPFlags(cmd, opts)
	return cmd
}

func addMCPFlags(cmd *cobra.Command, opts *serveOptions) {
	cmd.Flags().StringVar(&opts.path, "path", opts.path, "MCP HTTP path")
	cmd.Flags().BoolVar(&opts.management, "allow-management-tools", opts.management, "register MCP management tools")
	cmd.Flags().IntVar(&opts.maxOutput, "max-output-bytes", opts.maxOutput, "maximum MCP inline output bytes")
}

func newServeRuntime(cmd *cobra.Command, cfg *config, opts serveOptions) (*app.Runtime, error) {
	if !flagChanged(cmd, "allow-management-tools") {
		value, err := environmentBool(cfg.env, EnvMCPAllowManagementTools, opts.management)
		if err != nil {
			return nil, err
		}
		opts.management = value
	}
	if !flagChanged(cmd, "max-output-bytes") {
		value, err := environmentInt(cfg.env, EnvMCPMaxOutputBytes, opts.maxOutput)
		if err != nil {
			return nil, err
		}
		opts.maxOutput = value
	}
	overrideSources := startupOverrideSources(cmd, cfg.env)
	appCfg := app.Config{
		DataDir:         cfg.dataDir,
		OverrideSources: overrideSources,
		HTTP: app.HTTPConfig{
			Listen: opts.listen,
			Token:  opts.token,
		},
		MCP: app.MCPConfig{
			Path:                 opts.path,
			AllowManagementTools: opts.management,
			MaxOutputBytes:       opts.maxOutput,
		},
		WebUI: app.WebUIConfig{
			StaticDir: opts.webUIStaticDir,
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
		{path: "mcp.allow_management_tools", envKey: EnvMCPAllowManagementTools, flag: "allow-management-tools"},
		{path: "mcp.max_output_bytes", envKey: EnvMCPMaxOutputBytes, flag: "max-output-bytes"},
		{path: "webui.static_dir", envKey: EnvWebUIStaticDir, flag: "webui-static-dir"},
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

func environmentBool(env map[string]string, key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(env[key])
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return value, nil
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
		webui.WithStaticDir(cfg.WebUI.StaticDir),
		webui.WithReservedPrefixes("/v1", cfg.MCP.Path, "/s"),
	)
}

func runEntrypoint(ctx context.Context, entry app.Entrypoint, rt *app.Runtime) error {
	err := entry.Run(ctx, rt)
	if app.IsContextDone(err) || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
