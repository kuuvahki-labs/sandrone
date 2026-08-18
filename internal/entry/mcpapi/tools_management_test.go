package mcpapi_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

func TestSubscriptionLifecycle(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	put := callToolSuccess(t, ctx, session, "sandrone_put_subscription", map[string]any{
		"name":    "agent-sub",
		"type":    "local",
		"format":  "uri-list",
		"content": "ss://aes-128-gcm:secret@example.com:8388#first",
	})
	require.Equal(t, true, put["ok"])
	require.Equal(t, "sandrone://subscriptions/agent-sub", put["resource_uri"])

	listed := callToolSuccess(t, ctx, session, "sandrone_list_resources", map[string]any{
		"kind": "subscription",
	})
	require.Len(t, listed["items"], 1)
	require.Equal(t, "agent-sub", listed["items"].([]any)[0].(map[string]any)["name"])

	var definition map[string]any
	readJSONResource(t, ctx, session, "sandrone://subscriptions/agent-sub", &definition)
	require.Equal(t, "first", definitionNodeName(t, definition["content"].(string)))

	preview := callToolSuccess(t, ctx, session, "sandrone_preview_subscription", map[string]any{
		"name": "agent-sub",
	})
	require.Equal(t, float64(1), preview["after_count"])

	rendered := callToolSuccess(t, ctx, session, "sandrone_render_subscription", map[string]any{
		"name":   "agent-sub",
		"format": "uri-list",
	})
	require.Contains(t, rendered["body"], "#first")

	overwrite := callToolSuccess(t, ctx, session, "sandrone_put_subscription", map[string]any{
		"name":    "agent-sub",
		"type":    "local",
		"format":  "uri-list",
		"content": "ss://aes-128-gcm:secret@example.com:8388#changed",
	})
	require.Equal(t, "sandrone://subscriptions/agent-sub", overwrite["resource_uri"])
	readJSONResource(t, ctx, session, "sandrone://subscriptions/agent-sub", &definition)
	require.Equal(t, "changed", definitionNodeName(t, definition["content"].(string)))

	deleted := callToolSuccess(t, ctx, session, "sandrone_delete_subscription", map[string]any{"name": "agent-sub"})
	require.Equal(t, true, deleted["ok"])
	require.Equal(t, true, deleted["deleted"])
	require.Equal(t, "sandrone://subscriptions/agent-sub", deleted["resource_uri"])

	_, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "sandrone://subscriptions/agent-sub"})
	require.Error(t, err)
	require.Contains(t, callToolError(t, ctx, session, "sandrone_delete_subscription", map[string]any{
		"name": "agent-sub",
	}), "requested resource was not found")
}

func TestToolAnnotations(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	result, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	tools := make(map[string]*mcp.Tool, len(result.Tools))
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
	}

	for _, name := range []string{
		"sandrone_put_subscription",
		"sandrone_delete_subscription",
		"sandrone_put_file",
		"sandrone_delete_file",
	} {
		tool := tools[name]
		require.NotNil(t, tool, name)
		require.NotNil(t, tool.Annotations, name)
		require.False(t, tool.Annotations.ReadOnlyHint, name)
		require.NotNil(t, tool.Annotations.DestructiveHint, name)
		require.True(t, *tool.Annotations.DestructiveHint, name)
		require.True(t, tool.Annotations.IdempotentHint, name)
		require.NotNil(t, tool.Annotations.OpenWorldHint, name)
		require.False(t, *tool.Annotations.OpenWorldHint, name)
	}

	for _, name := range []string{"sandrone_get_file", "sandrone_validate_file"} {
		tool := tools[name]
		require.NotNil(t, tool, name)
		require.NotNil(t, tool.Annotations, name)
		require.True(t, tool.Annotations.ReadOnlyHint, name)
		require.NotNil(t, tool.Annotations.OpenWorldHint, name)
		require.True(t, *tool.Annotations.OpenWorldHint, name)
	}
}

func definitionNodeName(t *testing.T, content string) string {
	t.Helper()
	index := strings.LastIndex(content, "#")
	require.NotEqual(t, -1, index, "subscription node name missing")
	return content[index+1:]
}
