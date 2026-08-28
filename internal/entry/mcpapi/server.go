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

type inspectOutput struct {
	domain.InspectResult
	Catalogs inspectCatalogs `json:"catalogs"`
}

type inspectCatalogs struct {
	Formats    string `json:"formats"`
	Schemas    string `json:"schemas"`
	Processors string `json:"processors"`
	FileKinds  string `json:"file_kinds"`
}

func New(rt *app.Runtime) *Server {
	s := &Server{rt: rt}
	s.server = newSDKServer(rt)
	registerTools(s.server, rt)
	registerResources(s.server, rt)
	registerPrompts(s.server, rt)
	return s
}

func (s *Server) Handler() http.Handler {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s.server
	}, &mcp.StreamableHTTPOptions{
		Stateless:                    true,
		JSONResponse:                 true,
		Logger:                       s.rt.Logger,
		MaxRequestBodyBytes:          mcp.DefaultMaxRequestBodyBytes,
		PropagateRequestCancellation: true,
	})
	return strictProtocolHTTPHandler(handler)
}

func SDKServer(rt *app.Runtime) *mcp.Server {
	server := newSDKServer(rt)
	registerTools(server, rt)
	registerResources(server, rt)
	registerPrompts(server, rt)
	return server
}

func newSDKServer(rt *app.Runtime) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "sandrone", Version: buildinfo.Version()}, &mcp.ServerOptions{
		Instructions: "Sandrone converts node content and project resources through its service layer.",
		Logger:       rt.Logger,
		Capabilities: &mcp.ServerCapabilities{
			Tools:     &mcp.ToolCapabilities{},
			Resources: &mcp.ResourceCapabilities{},
			Prompts:   &mcp.PromptCapabilities{},
		},
	})
	server.AddReceivingMiddleware(protocolPolicyMiddleware)
	return server
}

func registerTools(server *mcp.Server, rt *app.Runtime) {
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
	registerManagementTools(server, rt)
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
