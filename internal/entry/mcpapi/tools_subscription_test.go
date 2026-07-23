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

func TestSubscriptionPreviewReturnsDiffWarningsAndReceivesArgs(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, localProcessedSubscription(t, "local")))
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_preview_subscription",
		Arguments: map[string]any{
			"name": "local",
			"args": map[string]any{"prefix": "preview-"},
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var output domain.SubscriptionPreviewResult
	decodeStructuredContent(t, result, &output)
	require.Equal(t, "local", output.SubscriptionName)
	require.Equal(t, 1, output.BeforeCount)
	require.Equal(t, 1, output.AfterCount)
	require.Equal(t, 1, output.StatusCounts["modified"])
	require.Equal(t, "node-a", output.Nodes[0].Before.Name)
	require.Equal(t, "preview-renamed-node-a", output.Nodes[0].After.Name)
	require.Contains(t, output.Report.Warnings, domain.Warning{
		Code: "request_args", Message: "preview-",
	})
}

func TestSubscriptionRenderReturnsContentReportAndReceivesArgs(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, localProcessedSubscription(t, "local")))
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_render_subscription",
		Arguments: map[string]any{
			"name":   "local",
			"format": "json-nodes",
			"args":   map[string]any{"prefix": "render-"},
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var output struct {
		ContentType string        `json:"content_type"`
		Body        string        `json:"body"`
		Report      domain.Report `json:"report"`
	}
	decodeStructuredContent(t, result, &output)
	require.Equal(t, "application/json", output.ContentType)
	require.Contains(t, output.Body, `"name": "render-renamed-node-a"`)
	require.Equal(t, "subscription_render", output.Report.Kind)
	require.Contains(t, output.Report.Warnings, domain.Warning{
		Code: "request_args", Message: "render-",
	})

	body, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	require.NotContains(t, string(body), `"spec":`)
	require.NotContains(t, string(body), `"resource_uri":`)
}

func TestSubscriptionTrafficReturnsRemoteServiceResult(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(
		[]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"),
	)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1024; download=2048; total=10240")
		_, _ = w.Write([]byte(encoded))
	}))
	defer remote.Close()

	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "remote", Type: domain.SubscriptionTypeRemote, Format: "base64",
		Remote: &domain.RemoteInput{URL: remote.URL},
	}))
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sandrone_get_subscription_traffic",
		Arguments: map[string]any{"name": "remote", "refresh": true},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var output domain.SubscriptionTrafficResult
	decodeStructuredContent(t, result, &output)
	require.Equal(t, "remote", output.SubscriptionName)
	require.Equal(t, domain.SubscriptionTypeRemote, output.Type)
	require.Equal(t, "base64", output.Format)
	require.False(t, output.Cached)
	require.NotNil(t, output.Traffic)
	require.Equal(t, "remote", output.Traffic.SourceName)
	require.Equal(t, remote.URL, output.Traffic.SourceURL)
	require.Equal(t, int64(1024), output.Traffic.UploadBytes)
	require.Equal(t, int64(2048), output.Traffic.DownloadBytes)
	require.Equal(t, int64(3072), output.Traffic.UsedBytes)
	total := int64(10240)
	require.Equal(t, &total, output.Traffic.TotalBytes)
}

func TestSubscriptionPreviewRenderTrafficBoundaries(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "local", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))
	unavailable := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	unavailableURL := unavailable.URL
	unavailable.Close()
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "unavailable", Type: domain.SubscriptionTypeRemote, Format: "uri-list",
		Remote: &domain.RemoteInput{URL: unavailableURL},
	}))
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	tests := []struct {
		name      string
		tool      string
		arguments map[string]any
		want      string
	}{
		{
			name: "preview missing resource", tool: "sandrone_preview_subscription",
			arguments: map[string]any{"name": "missing"}, want: "requested resource was not found",
		},
		{
			name: "render missing resource", tool: "sandrone_render_subscription",
			arguments: map[string]any{"name": "missing", "format": "uri-list"}, want: "requested resource was not found",
		},
		{
			name: "traffic missing resource", tool: "sandrone_get_subscription_traffic",
			arguments: map[string]any{"name": "missing"}, want: "requested resource was not found",
		},
		{
			name: "preview multi segment name", tool: "sandrone_preview_subscription",
			arguments: map[string]any{"name": "groups/live"}, want: "single path segment",
		},
		{
			name: "render multi segment name", tool: "sandrone_render_subscription",
			arguments: map[string]any{"name": "groups/live", "format": "uri-list"}, want: "single path segment",
		},
		{
			name: "traffic multi segment name", tool: "sandrone_get_subscription_traffic",
			arguments: map[string]any{"name": "groups/live"}, want: "single path segment",
		},
		{
			name: "unsupported render format", tool: "sandrone_render_subscription",
			arguments: map[string]any{"name": "local", "format": "future"}, want: "does not equal any of",
		},
		{
			name: "remote fetch failure", tool: "sandrone_preview_subscription",
			arguments: map[string]any{"name": "unavailable"}, want: "remote",
		},
		{
			name: "traffic local subscription", tool: "sandrone_get_subscription_traffic",
			arguments: map[string]any{"name": "local"}, want: "requires remote subscription",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Contains(t, callToolError(t, ctx, session, test.tool, test.arguments), test.want)
		})
	}
}

func TestSubscriptionPreviewRenderTrafficAnnotationsAreReadOnlyAndOpenWorld(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	result, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	tools := map[string]*mcp.Tool{}
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
	}
	for _, name := range []string{
		"sandrone_preview_subscription",
		"sandrone_render_subscription",
		"sandrone_get_subscription_traffic",
	} {
		tool := tools[name]
		require.NotNil(t, tool, name)
		require.NotNil(t, tool.Annotations, name)
		require.True(t, tool.Annotations.ReadOnlyHint, name)
		require.NotNil(t, tool.Annotations.OpenWorldHint, name)
		require.True(t, *tool.Annotations.OpenWorldHint, name)
	}
}

func localProcessedSubscription(t *testing.T, name string) domain.Subscription {
	t.Helper()
	return domain.Subscription{
		Name: name, Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{
			{
				Type: "rename", Stage: domain.StageNodes,
				Params: subscriptionProcessorParams(t, map[string]any{
					"mode": "prefix", "value": "renamed-",
				}),
			},
			{
				Type: "script", Stage: domain.StageNodes,
				Params: subscriptionProcessorParams(t, map[string]any{
					"source": map[string]any{
						"type": "inline",
						"content": `function main(input, api) {
  var prefix = (input.args && input.args.prefix) || "";
  input.nodes.forEach(function(node) { node.name = prefix + node.name; });
  api.warn({code: "request_args", message: prefix});
  return input;
}`,
					},
				}),
			},
		},
	}
}

func subscriptionProcessorParams(t *testing.T, values map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		body, err := json.Marshal(value)
		require.NoError(t, err)
		out[key] = body
	}
	return out
}
