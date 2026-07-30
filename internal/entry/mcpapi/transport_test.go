package mcpapi_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

func TestStreamableHTTPSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := httptest.NewServer(mcpapi.New(testRuntime(t, app.Config{})).Handler())
	defer server.Close()

	session := connectClient(t, ctx, &mcp.StreamableClientTransport{Endpoint: server.URL})
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	require.NotEmpty(t, tools.Tools)
	resources, err := session.ListResources(ctx, nil)
	require.NoError(t, err)
	require.NotEmpty(t, resources.Resources)
	prompts, err := session.ListPrompts(ctx, nil)
	require.NoError(t, err)
	require.NotEmpty(t, prompts.Prompts)

	inspect, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sandrone_inspect_capabilities",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, inspect.IsError)
	inspectJSON, err := json.Marshal(inspect.StructuredContent)
	require.NoError(t, err)
	require.Contains(t, string(inspectJSON), `"capabilities"`)

	schema, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "sandrone://schemas/script-api/v1"})
	require.NoError(t, err)
	require.Len(t, schema.Contents, 1)
	require.Equal(t, "application/json", schema.Contents[0].MIMEType)
	require.Contains(t, schema.Contents[0].Text, `"version": 1`)
}
