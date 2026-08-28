package mcpapi

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kuuvahki-labs/sandrone/internal/agentcatalog"
	"github.com/kuuvahki-labs/sandrone/internal/app"
)

type putResourceOutput struct {
	OK          bool   `json:"ok"`
	ResourceURI string `json:"resource_uri"`
}

type deleteResourceOutput struct {
	OK          bool   `json:"ok"`
	Deleted     bool   `json:"deleted"`
	ResourceURI string `json:"resource_uri"`
}

func registerManagementTools(server *mcp.Server, rt *app.Runtime) {
	destructive := true
	openWorld := false
	annotations := &mcp.ToolAnnotations{
		DestructiveHint: &destructive,
		IdempotentHint:  true,
		OpenWorldHint:   &openWorld,
	}

	addTool(server, &mcp.Tool{
		Name:        "sandrone_put_subscription",
		Description: "Store a named subscription resource.",
		InputSchema: agentcatalog.SubscriptionSchema(),
		Annotations: annotations,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in putSubscriptionInput) (*mcp.CallToolResult, putResourceOutput, error) {
		if err := validateRequiredPublicResourceName("subscription name", in.Name); err != nil {
			return nil, putResourceOutput{}, err
		}
		sub, err := in.domain()
		if err != nil {
			return nil, putResourceOutput{}, err
		}
		if err := rt.Service.PutSubscription(ctx, sub); err != nil {
			return nil, putResourceOutput{}, err
		}
		return nil, putResourceOutput{
			OK: true, ResourceURI: definitionResourceURI("subscriptions", in.Name),
		}, nil
	})

	addTool(server, &mcp.Tool{
		Name:        "sandrone_delete_subscription",
		Description: "Delete a named subscription resource.",
		InputSchema: deleteSubscriptionInputSchema(),
		Annotations: annotations,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deleteSubscriptionInput) (*mcp.CallToolResult, deleteResourceOutput, error) {
		if err := validateRequiredPublicResourceName("subscription name", in.Name); err != nil {
			return nil, deleteResourceOutput{}, err
		}
		if err := rt.Service.DeleteSubscription(ctx, in.Name); err != nil {
			return nil, deleteResourceOutput{}, err
		}
		return nil, deleteResourceOutput{
			OK: true, Deleted: true, ResourceURI: definitionResourceURI("subscriptions", in.Name),
		}, nil
	})

	addTool(server, &mcp.Tool{
		Name:        "sandrone_put_file",
		Description: "Store a named FileSpec.",
		InputSchema: agentcatalog.FileSpecSchema(true),
		Annotations: annotations,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in putFileInput) (*mcp.CallToolResult, putResourceOutput, error) {
		if err := validateRequiredPublicResourceName("file name", in.Name); err != nil {
			return nil, putResourceOutput{}, err
		}
		spec, err := in.domain()
		if err != nil {
			return nil, putResourceOutput{}, err
		}
		if err := rt.Service.PutFile(ctx, spec); err != nil {
			return nil, putResourceOutput{}, err
		}
		return nil, putResourceOutput{
			OK: true, ResourceURI: definitionResourceURI("files", in.Name),
		}, nil
	})

	addTool(server, &mcp.Tool{
		Name:        "sandrone_delete_file",
		Description: "Delete a named FileSpec.",
		InputSchema: deleteFileInputSchema(),
		Annotations: annotations,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deleteFileInput) (*mcp.CallToolResult, deleteResourceOutput, error) {
		if err := validateRequiredPublicResourceName("file name", in.Name); err != nil {
			return nil, deleteResourceOutput{}, err
		}
		if err := rt.Service.DeleteFile(ctx, in.Name); err != nil {
			return nil, deleteResourceOutput{}, err
		}
		return nil, deleteResourceOutput{
			OK: true, Deleted: true, ResourceURI: definitionResourceURI("files", in.Name),
		}, nil
	})
}

func definitionResourceURI(collection, name string) string {
	return "sandrone://" + collection + "/" + url.PathEscape(name)
}
