// Package mcpapi implements Sandrone's MCP server entrypoint.
package mcpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type Server struct {
	rt     *app.Runtime
	server *mcp.Server
}

type convertInput struct {
	FromFormat       string                 `json:"from_format" jsonschema:"input format, for example uri-list"`
	ToFormat         string                 `json:"to_format" jsonschema:"target render format"`
	Content          string                 `json:"content,omitempty" jsonschema:"input node content"`
	Remote           *domain.RemoteInput    `json:"remote,omitempty" jsonschema:"HTTP(S) subscription input to fetch before parsing"`
	ParseProcessors  []domain.ProcessorSpec `json:"parse_processors,omitempty"`
	RenderProcessors []domain.ProcessorSpec `json:"render_processors,omitempty"`
	Options          domain.RenderOptions   `json:"options,omitempty"`
	Meta             map[string]string      `json:"meta,omitempty"`
}

type renderOutput struct {
	ContentType string           `json:"content_type,omitempty"`
	Body        string           `json:"body,omitempty"`
	Spec        *domain.FileSpec `json:"spec,omitempty"`
	Report      domain.Report    `json:"report,omitempty"`
	ResourceURI string           `json:"resource_uri,omitempty"`
}

type getFileInput struct {
	File   string `json:"file" jsonschema:"stored FileSpec name"`
	Mode   string `json:"mode,omitempty" jsonschema:"render or spec"`
	Target string `json:"target,omitempty"`
}

type validateFileInput struct {
	File   string           `json:"file,omitempty" jsonschema:"stored FileSpec name"`
	Spec   *domain.FileSpec `json:"spec,omitempty" jsonschema:"inline FileSpec"`
	Target string           `json:"target,omitempty"`
}

type probeInput struct {
	Input           domain.NodeInput   `json:"input" jsonschema:"node input to probe"`
	Layer           domain.ProbeLayer  `json:"layer,omitempty"`
	Method          domain.ProbeMethod `json:"method,omitempty"`
	Core            string             `json:"core,omitempty"`
	URL             string             `json:"url,omitempty"`
	NTPServer       string             `json:"ntp_server,omitempty"`
	ExpectedStatus  string             `json:"expected_status,omitempty"`
	TimeoutMS       int                `json:"timeout_ms,omitempty"`
	Attempts        int                `json:"attempts,omitempty"`
	Concurrency     int                `json:"concurrency,omitempty"`
	CacheTTLSeconds int                `json:"cache_ttl_seconds,omitempty"`
	Meta            map[string]string  `json:"meta,omitempty"`
}

type probeOutput struct {
	Results []domain.NodeProbeResult `json:"results"`
	Report  domain.Report            `json:"report,omitempty"`
}

type putSubscriptionInput struct {
	domain.Subscription
}

type putFileInput struct {
	domain.FileSpec
}

type okOutput struct {
	OK bool `json:"ok"`
}

type inspectOutput struct {
	Capabilities map[string]any `json:"capabilities"`
	Report       domain.Report  `json:"report,omitempty"`
}

func New(rt *app.Runtime) *Server {
	s := &Server{rt: rt}
	s.server = newSDKServer(rt)
	registerTools(s.server, rt)
	registerResources(s.server, rt)
	registerPrompts(s.server)
	return s
}

func (s *Server) Name() string { return "mcp" }

func (s *Server) Run(ctx context.Context, rt *app.Runtime) error {
	if rt != nil && rt != s.rt {
		s.rt = rt
		s.server = newSDKServer(rt)
		registerTools(s.server, rt)
		registerResources(s.server, rt)
		registerPrompts(s.server)
	}
	s.rt.Logger.Info("starting MCP server", "transport", "stdio")
	err := s.server.Run(ctx, &mcp.StdioTransport{})
	if err != nil && !app.IsContextDone(err) {
		s.rt.Logger.Error("MCP server stopped with error", "transport", "stdio", "error", err)
		return err
	}
	s.rt.Logger.Info("MCP server stopped", "transport", "stdio")
	return err
}

func (s *Server) Handler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s.server
	}, &mcp.StreamableHTTPOptions{
		JSONResponse: true,
		Logger:       s.rt.Logger,
	})
}

func SDKServer(rt *app.Runtime) *mcp.Server {
	server := newSDKServer(rt)
	registerTools(server, rt)
	registerResources(server, rt)
	registerPrompts(server)
	return server
}

func newSDKServer(rt *app.Runtime) *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{Name: "sandrone", Version: buildinfo.Version()}, &mcp.ServerOptions{
		Instructions: "Sandrone converts node content and project resources through its service layer.",
		Logger:       rt.Logger,
	})
}

func registerTools(server *mcp.Server, rt *app.Runtime) {
	readonly := true
	openWorld := false
	convertOpenWorld := true
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandrone_convert",
		Description: "Convert node content directly to a target format, including json-nodes export.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &convertOpenWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in convertInput) (*mcp.CallToolResult, renderOutput, error) {
		result, err := rt.Service.Convert(ctx, domain.ConvertRequest{
			FromFormat:       in.FromFormat,
			ToFormat:         in.ToFormat,
			Content:          []byte(in.Content),
			Remote:           in.Remote,
			ParseProcessors:  in.ParseProcessors,
			RenderProcessors: in.RenderProcessors,
			Options:          in.Options,
			Meta:             in.Meta,
		})
		if err != nil {
			return nil, renderOutput{}, err
		}
		out := renderOutput{
			ContentType: result.ContentType,
			Body:        string(result.Body),
			Report:      result.Report,
		}
		return nil, limitedRenderOutput(rt, out), nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandrone_get_file",
		Description: "Get a stored file: render content by default, or return the FileSpec with mode spec.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getFileInput) (*mcp.CallToolResult, renderOutput, error) {
		if err := validateRequiredPublicResourceName("file name", in.File); err != nil {
			return nil, renderOutput{}, err
		}
		switch strings.ToLower(strings.TrimSpace(in.Mode)) {
		case "spec":
			spec, err := rt.Service.GetFileSpec(ctx, in.File)
			if err != nil {
				return nil, renderOutput{}, err
			}
			return nil, renderOutput{Spec: spec, ResourceURI: "sandrone://files/" + url.PathEscape(in.File)}, nil
		case "", "render":
		default:
			return nil, renderOutput{}, domain.NewError(domain.CodeInvalidArgument, "unsupported file mode")
		}
		result, err := rt.Service.GetFile(ctx, domain.FileRequest{
			Name:   in.File,
			Target: in.Target,
		})
		if err != nil {
			return nil, renderOutput{}, err
		}
		out := renderOutput{
			ContentType: result.ContentType,
			Body:        string(result.Content),
			Report:      result.Report,
		}
		return nil, limitedRenderOutput(rt, out), nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandrone_probe_nodes",
		Description: "Probe node reachability from a NodeInput.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &readonly,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in probeInput) (*mcp.CallToolResult, probeOutput, error) {
		result, err := rt.Service.Probe(ctx, domain.ProbeRequest{
			Input:           in.Input,
			Layer:           in.Layer,
			Method:          in.Method,
			Core:            in.Core,
			URL:             in.URL,
			NTPServer:       in.NTPServer,
			ExpectedStatus:  in.ExpectedStatus,
			TimeoutMS:       in.TimeoutMS,
			Attempts:        in.Attempts,
			Concurrency:     in.Concurrency,
			CacheTTLSeconds: in.CacheTTLSeconds,
			Meta:            in.Meta,
		})
		if err != nil {
			return nil, probeOutput{}, err
		}
		return nil, probeOutput{Results: result.Results, Report: result.Report}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandrone_validate_file",
		Description: "Generate and validate a FileSpec without bypassing service file flow.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in validateFileInput) (*mcp.CallToolResult, renderOutput, error) {
		if err := validateOptionalPublicResourceName("file name", in.File); err != nil {
			return nil, renderOutput{}, err
		}
		result, err := rt.Service.ValidateFile(ctx, domain.FileRequest{Name: in.File, Spec: in.Spec, Target: in.Target})
		if err != nil {
			return nil, renderOutput{}, err
		}
		return nil, renderOutput{Report: result.Report}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandrone_inspect_capabilities",
		Description: "Inspect Sandrone formats, processors and probe capabilities.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, inspectOutput, error) {
		result, err := rt.Service.Inspect(ctx, domain.InspectRequest{})
		if err != nil {
			return nil, inspectOutput{}, err
		}
		return nil, inspectOutput{Capabilities: result.Capabilities, Report: result.Report}, nil
	})
	if rt.Config.MCP.ReadOnly || !rt.Config.MCP.AllowManagementTools {
		return
	}
	destructive := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandrone_put_subscription",
		Description: "Store a named subscription resource.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &destructive,
			IdempotentHint:  true,
			OpenWorldHint:   &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in putSubscriptionInput) (*mcp.CallToolResult, okOutput, error) {
		if err := validateRequiredPublicResourceName("subscription name", in.Name); err != nil {
			return nil, okOutput{}, err
		}
		if err := rt.Service.PutSubscription(ctx, in.Subscription); err != nil {
			return nil, okOutput{}, err
		}
		return nil, okOutput{OK: true}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sandrone_put_file",
		Description: "Store a named FileSpec.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &destructive,
			IdempotentHint:  true,
			OpenWorldHint:   &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in putFileInput) (*mcp.CallToolResult, okOutput, error) {
		if err := validateRequiredPublicResourceName("file name", in.Name); err != nil {
			return nil, okOutput{}, err
		}
		if err := rt.Service.PutFile(ctx, in.FileSpec); err != nil {
			return nil, okOutput{}, err
		}
		return nil, okOutput{OK: true}, nil
	})
}

func registerResources(server *mcp.Server, rt *app.Runtime) {
	addJSONResource(server, "sandrone://capabilities", "capabilities", "Sandrone capability summary", func(context.Context) (any, error) {
		return rt.Service.CapabilitySummary(), nil
	})
	addResourceTemplate(server, "sandrone://subscriptions/{name}", "subscriptions", "Stored subscription resources", func(ctx context.Context, name string) (any, error) {
		return rt.Service.GetSubscription(ctx, name)
	})
	addResourceTemplate(server, "sandrone://files/{name}", "files", "Stored file specs", func(ctx context.Context, name string) (any, error) {
		return rt.Service.GetFileSpec(ctx, name)
	})
}

func registerPrompts(server *mcp.Server) {
	for name, text := range map[string]string{
		"convert_nodes":            "Convert node content with sandrone_convert; use to_format json-nodes when IR export is needed, or sandrone_get_file for stored FileSpec output.",
		"diagnose_conversion_loss": "Compare the parse and render reports, focus on warnings and lost fields, then explain likely target format limitations.",
		"design_mihomo_file":       "Design a Sandrone FileSpec for a complete mihomo configuration using source plus file-stage script processors and api.subscription.produce.",
		"design_sing_box_file":     "Design a Sandrone FileSpec for a complete sing-box configuration using source plus file-stage script processors and api.subscription.produce.",
		"explain_report":           "Explain a Sandrone report by grouping dependencies, source references, render statistics, probe statistics, and warnings.",
	} {
		server.AddPrompt(&mcp.Prompt{Name: name, Description: text}, func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Description: text,
				Messages: []*mcp.PromptMessage{{
					Role:    "user",
					Content: &mcp.TextContent{Text: text},
				}},
			}, nil
		})
	}
}

func addJSONResource(server *mcp.Server, uri, name, desc string, load func(context.Context) (any, error)) {
	server.AddResource(&mcp.Resource{URI: uri, Name: name, Description: desc, MIMEType: "application/json"}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		value, err := load(ctx)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, value)
	})
}

func addResourceTemplate(server *mcp.Server, tmpl, name, desc string, load func(context.Context, string) (any, error)) {
	server.AddResourceTemplate(&mcp.ResourceTemplate{URITemplate: tmpl, Name: name, Description: desc, MIMEType: "application/json"}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		name, err := resourceName(req.Params.URI)
		if err != nil {
			return nil, err
		}
		value, err := load(ctx, name)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, value)
	})
}

func jsonResource(uri string, value any) (*mcp.ReadResourceResult, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(body),
		}},
	}, nil
}

func resourceName(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	name := strings.TrimPrefix(parsed.EscapedPath(), "/")
	if name == "" {
		return "", domain.NewError(domain.CodeInvalidArgument, "resource name is required")
	}
	name, err = url.PathUnescape(name)
	if err != nil {
		return "", err
	}
	if err := validateRequiredPublicResourceName("resource name", name); err != nil {
		return "", err
	}
	return name, nil
}

func validateOptionalPublicResourceName(label string, name string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	return validateRequiredPublicResourceName(label, name)
}

func validateRequiredPublicResourceName(label string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.NewError(domain.CodeInvalidArgument, label+" is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || name == "." || name == ".." {
		return domain.NewError(domain.CodeInvalidArgument, label+" must be a single path segment")
	}
	return nil
}

func limitedRenderOutput(rt *app.Runtime, out renderOutput) renderOutput {
	limit := rt.Config.MCP.MaxOutputBytes
	if limit <= 0 || len(out.Body) <= limit {
		return out
	}
	out.Body = ""
	return out
}
