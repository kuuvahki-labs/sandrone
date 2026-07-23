package cli

import (
	"context"
	"errors"
	"net/http"
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
	tokenRequired  bool
	webUIStaticDir string
	transport      string
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
		transport:      "stdio",
		path:           app.DefaultMCPPath,
		maxOutput:      1 << 20,
		logLevel:       firstNonEmpty(cfg.env[EnvLogLevel], "info"),
	}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run HTTP API and MCP entrypoints",
	}
	cmd.PersistentFlags().StringVar(&opts.listen, "listen", opts.listen, "listen address")
	cmd.PersistentFlags().StringVar(&opts.token, "token", opts.token, "bearer token for HTTP and MCP HTTP")
	cmd.PersistentFlags().BoolVar(&opts.tokenRequired, "token-required", false, "require bearer token even on local listeners")
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
			rt, err := newServeRuntime(cfg, *opts, "streamable-http")
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
		Short: "Run the MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newServeRuntime(cfg, *opts, opts.transport)
			if err != nil {
				return err
			}
			mcpServer := mcpapi.New(rt)
			if normalizeTransport(opts.transport) == "streamable-http" {
				httpServer := httpapi.New(rt, httpapi.WithMCP(rt.Config.MCP.Path, mcpServer.Handler()))
				return runEntrypoint(cmd.Context(), httpServer, rt)
			}
			return runEntrypoint(cmd.Context(), mcpServer, rt)
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
			rt, err := newServeRuntime(cfg, *opts, "streamable-http")
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
	cmd.Flags().StringVar(&opts.transport, "transport", opts.transport, "MCP transport: stdio or streamable-http")
	cmd.Flags().StringVar(&opts.path, "path", opts.path, "MCP HTTP path")
	cmd.Flags().BoolVar(&opts.management, "allow-management-tools", opts.management, "register MCP management tools")
	cmd.Flags().IntVar(&opts.maxOutput, "max-output-bytes", opts.maxOutput, "maximum MCP inline output bytes")
}

func newServeRuntime(cfg *config, opts serveOptions, transport string) (*app.Runtime, error) {
	if transport != "" {
		opts.transport = transport
	}
	appCfg := app.Config{
		DataDir: cfg.dataDir,
		HTTP: app.HTTPConfig{
			Listen:        opts.listen,
			Token:         opts.token,
			TokenRequired: opts.tokenRequired,
		},
		MCP: app.MCPConfig{
			Transport:            normalizeTransport(opts.transport),
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

func normalizeTransport(transport string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(transport)), "_", "-")
}
