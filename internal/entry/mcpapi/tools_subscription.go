package mcpapi

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type subscriptionPreviewInput struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args,omitempty"`
}

type subscriptionRenderInput struct {
	Name    string            `json:"name"`
	Format  string            `json:"format"`
	Args    map[string]string `json:"args,omitempty"`
	Refresh bool              `json:"refresh,omitempty"`
}

type subscriptionRenderOutput struct {
	ContentType    string        `json:"content_type,omitempty"`
	Body           string        `json:"body,omitempty"`
	BodyOmitted    bool          `json:"body_omitted,omitempty"`
	BodyBytes      int           `json:"body_bytes,omitempty"`
	MaxOutputBytes int           `json:"max_output_bytes,omitempty"`
	Report         domain.Report `json:"report,omitempty"`
}

type subscriptionTrafficInput struct {
	Name    string `json:"name"`
	Refresh bool   `json:"refresh,omitempty"`
}

func registerSubscriptionTools(server *mcp.Server, rt *app.Runtime) {
	openWorld := true
	addTool(server, &mcp.Tool{
		Name:        "sandrone_preview_subscription",
		Description: "Preview a stored subscription before and after its processors run.",
		InputSchema: subscriptionPreviewInputSchema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in subscriptionPreviewInput) (*mcp.CallToolResult, domain.SubscriptionPreviewResult, error) {
		if err := validateRequiredPublicResourceName("subscription name", in.Name); err != nil {
			return nil, domain.SubscriptionPreviewResult{}, err
		}
		result, err := rt.Service.PreviewSubscription(ctx, in.Name, in.Args)
		if err != nil {
			return nil, domain.SubscriptionPreviewResult{}, err
		}
		return nil, *result, nil
	})

	addTool(server, &mcp.Tool{
		Name:        "sandrone_render_subscription",
		Description: "Render a stored subscription to a target format.",
		InputSchema: subscriptionRenderInputSchema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in subscriptionRenderInput) (*mcp.CallToolResult, subscriptionRenderOutput, error) {
		if err := validateRequiredPublicResourceName("subscription name", in.Name); err != nil {
			return nil, subscriptionRenderOutput{}, err
		}
		result, err := rt.Service.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{
			Name: in.Name, Format: in.Format,
			Request: domain.RequestInfo{Args: in.Args},
			Refresh: in.Refresh,
		})
		if err != nil {
			return nil, subscriptionRenderOutput{}, err
		}
		return nil, limitedSubscriptionRenderOutput(rt, subscriptionRenderOutput{
			ContentType: result.ContentType,
			Body:        string(result.Body),
			Report:      result.Report,
		}), nil
	})

	addTool(server, &mcp.Tool{
		Name:        "sandrone_get_subscription_traffic",
		Description: "Get usage metadata reported by a stored remote subscription.",
		InputSchema: subscriptionTrafficInputSchema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in subscriptionTrafficInput) (*mcp.CallToolResult, domain.SubscriptionTrafficResult, error) {
		if err := validateRequiredPublicResourceName("subscription name", in.Name); err != nil {
			return nil, domain.SubscriptionTrafficResult{}, err
		}
		result, err := rt.Service.SubscriptionTraffic(ctx, domain.SubscriptionTrafficRequest{
			Name:    in.Name,
			Refresh: in.Refresh,
		})
		if err != nil {
			return nil, domain.SubscriptionTrafficResult{}, err
		}
		return nil, *result, nil
	})
}

func limitedSubscriptionRenderOutput(rt *app.Runtime, out subscriptionRenderOutput) subscriptionRenderOutput {
	out.Body, out.BodyOmitted, out.BodyBytes, out.MaxOutputBytes =
		limitBody(out.Body, rt.Config.MCP.MaxOutputBytes)
	return out
}
