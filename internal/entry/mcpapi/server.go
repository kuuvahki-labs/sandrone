// Package mcpapi implements Sandrone's MCP server entrypoint.
package mcpapi

import (
	"context"
	"net/http"
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

type renderOutput struct {
	ContentType    string            `json:"content_type,omitempty"`
	Body           string            `json:"body,omitempty"`
	BodyOmitted    bool              `json:"body_omitted,omitempty"`
	BodyBytes      int               `json:"body_bytes,omitempty"`
	MaxOutputBytes int               `json:"max_output_bytes,omitempty"`
	Spec           *fileSpecOutput   `json:"spec,omitempty"`
	Source         *fileSourceOutput `json:"source,omitempty"`
	Report         domain.Report     `json:"report,omitempty"`
	ResourceURI    string            `json:"resource_uri,omitempty"`
}

type probeOutput struct {
	Results []domain.NodeProbeResult `json:"results"`
	Report  domain.Report            `json:"report,omitempty"`
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
	registerPrompts(s.server, rt)
	return s
}

func (s *Server) Name() string { return "mcp" }

func (s *Server) Run(ctx context.Context, rt *app.Runtime) error {
	if rt != nil && rt != s.rt {
		s.rt = rt
		s.server = newSDKServer(rt)
		registerTools(s.server, rt)
		registerResources(s.server, rt)
		registerPrompts(s.server, rt)
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
	registerPrompts(server, rt)
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
	convertOpenWorld := true
	registerDiscoveryTools(server, rt)
	registerSubscriptionTools(server, rt)
	registerFileTools(server, rt)
	addTool(server, &mcp.Tool{
		Name:        "sandrone_convert",
		Description: "Convert node content directly to a target format, including json-nodes export.",
		InputSchema: convertInputSchema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &convertOpenWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in convertInput) (*mcp.CallToolResult, renderOutput, error) {
		parseProcessors, err := processorSpecsDomain(in.ParseProcessors)
		if err != nil {
			return nil, renderOutput{}, err
		}
		renderProcessors, err := processorSpecsDomain(in.RenderProcessors)
		if err != nil {
			return nil, renderOutput{}, err
		}
		result, err := rt.Service.Convert(ctx, domain.ConvertRequest{
			FromFormat:       in.FromFormat,
			ToFormat:         in.ToFormat,
			Content:          []byte(in.Content),
			Remote:           in.Remote,
			ParseProcessors:  parseProcessors,
			RenderProcessors: renderProcessors,
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
	addTool(server, &mcp.Tool{
		Name:        "sandrone_probe_nodes",
		Description: "Probe node reachability from a NodeInput.",
		InputSchema: probeInputSchema(),
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
	if rt.Config.MCP.AllowManagementTools {
		registerManagementTools(server, rt)
	}
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
	out.Body, out.BodyOmitted, out.BodyBytes, out.MaxOutputBytes =
		limitBody(out.Body, rt.Config.MCP.MaxOutputBytes)
	return out
}

func limitBody(body string, limit int) (limited string, omitted bool, bodyBytes int, maxOutputBytes int) {
	if limit <= 0 || len(body) <= limit {
		return body, false, 0, 0
	}
	return "", true, len(body), limit
}
