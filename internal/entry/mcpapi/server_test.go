package mcpapi_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

func TestReadonlyToolsAndResourceRead(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{Name: "sub", Type: domain.SubscriptionTypeLocal, Format: "uri-list"}))
	server := mcpapi.SDKServer(rt)
	session := connect(t, ctx, server)
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	names := map[string]bool{}
	var convertTool *mcp.Tool
	for _, tool := range tools.Tools {
		names[tool.Name] = true
		if tool.Name == "sandrone_convert" {
			convertTool = tool
		}
	}
	require.True(t, names["sandrone_convert"])
	require.True(t, names["sandrone_get_file"])
	require.True(t, names["sandrone_inspect_capabilities"])
	require.False(t, names["sandrone_put_subscription"])
	require.NotNil(t, convertTool)
	require.NotNil(t, convertTool.Annotations)
	require.NotNil(t, convertTool.Annotations.OpenWorldHint)
	require.True(t, *convertTool.Annotations.OpenWorldHint)
	toolJSON, err := json.Marshal(convertTool)
	require.NoError(t, err)
	require.Contains(t, string(toolJSON), `"remote"`)
	require.Contains(t, string(toolJSON), `"url"`)
	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "sandrone://subscriptions/sub"})
	require.NoError(t, err)
	require.Len(t, resource.Contents, 1)
	require.Contains(t, resource.Contents[0].Text, `"format": "uri-list"`)

}

func TestServerReportsBuildVersionDuringInitialize(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	require.Equal(t, "0.1.0", session.InitializeResult().ServerInfo.Version)
}

func TestConvertToolAcceptsRemoteInput(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	sub := base64.StdEncoding.EncodeToString([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sub))
	}))
	defer subServer.Close()

	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_convert",
		Arguments: map[string]any{
			"from_format": "",
			"to_format":   "json-nodes",
			"remote":      map[string]any{"url": subServer.URL},
		},
	})
	require.NoError(t, err)
	body, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(body), "remote-node")
	require.Contains(t, string(body), subServer.URL)
}

func TestManagementToolsCanBeEnabled(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{MCP: app.MCPConfig{ReadOnly: false, AllowManagementTools: true}})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	require.True(t, names["sandrone_put_subscription"])
}

func TestMCPPublicResourceNamesRejectSlash(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{MCP: app.MCPConfig{ReadOnly: false, AllowManagementTools: true}})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	for _, uri := range []string{
		"sandrone://subscriptions/remote%2Fprovider",
		"sandrone://files/files%2Fdefault.yaml",
	} {
		t.Run(uri, func(t *testing.T) {
			_, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
			require.Error(t, err)
			require.Contains(t, err.Error(), "resource name must be a single path segment")
		})
	}

	for _, tt := range []struct {
		name      string
		tool      string
		arguments map[string]any
		want      string
	}{
		{
			name:      "get file",
			tool:      "sandrone_get_file",
			arguments: map[string]any{"file": "files/default.yaml"},
			want:      "file name must be a single path segment",
		},
		{
			name:      "validate file",
			tool:      "sandrone_validate_file",
			arguments: map[string]any{"file": "files/default.yaml"},
			want:      "file name must be a single path segment",
		},
		{
			name:      "put subscription",
			tool:      "sandrone_put_subscription",
			arguments: map[string]any{"name": "remote/provider", "type": "local", "format": "uri-list"},
			want:      "subscription name must be a single path segment",
		},
		{
			name:      "put file",
			tool:      "sandrone_put_file",
			arguments: map[string]any{"name": "files/default.yaml", "kind": "static", "source": map[string]any{"type": "inline", "content": "body"}},
			want:      "file name must be a single path segment",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := callToolError(t, ctx, session, tt.tool, tt.arguments)
			require.Contains(t, body, tt.want)
		})
	}
}

func TestMCPPutFileRejectsNonCanonicalKindAndLegacyConfigWire(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{MCP: app.MCPConfig{ReadOnly: false, AllowManagementTools: true}})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	tests := []struct {
		name      string
		arguments map[string]any
		want      string
	}{
		{
			name:      "missing kind",
			arguments: map[string]any{"name": "bad.yaml", "source": map[string]any{"type": "inline", "content": "body"}},
			want:      "kind",
		},
		{
			name:      "case variant",
			arguments: map[string]any{"name": "bad.yaml", "kind": "Mihomo", "source": map[string]any{"type": "inline"}},
			want:      `file kind \"Mihomo\"`,
		},
		{
			name:      "legacy config",
			arguments: map[string]any{"name": "bad.yaml", "kind": "mihomo", "source": map[string]any{"type": "inline"}, "config": map[string]any{"groups": []any{}}},
			want:      `unexpected additional properties [\"groups\"]`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := callToolError(t, ctx, session, "sandrone_put_file", test.arguments)
			require.Contains(t, body, test.want)
		})
	}
}

func callToolError(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, arguments map[string]any) string {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	require.NoError(t, err)
	require.True(t, result.IsError)
	body, err := json.Marshal(result)
	require.NoError(t, err)
	return string(body)
}

func connect(t *testing.T, ctx context.Context, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	return session
}

func testRuntime(t *testing.T, cfg app.Config) *app.Runtime {
	t.Helper()
	cfg.DataDir = t.TempDir()
	rt, err := app.NewRuntime(cfg, nil)
	require.NoError(t, err)
	return rt
}
