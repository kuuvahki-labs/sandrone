package mcpapi_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

const scriptContractNode = "ss://aes-128-gcm:secret@example.com:8388#node-a"

func TestScriptContractInlineNodesAndFileStages(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{
		MCP: app.MCPConfig{AllowManagementTools: true},
	})))
	defer session.Close()

	nodes := callToolSuccess(t, ctx, session, "sandrone_convert", map[string]any{
		"from_format": "uri-list",
		"to_format":   "json-nodes",
		"content":     scriptContractNode,
		"parse_processors": []any{scriptProcessor("nodes", map[string]any{
			"type": "inline",
			"content": `function main(input) {
  input.nodes[0].name = "inline-" + input.nodes[0].name;
  return input;
}`,
		}, nil)},
	})
	require.Contains(t, nodes["body"], "inline-node-a")

	callToolSuccess(t, ctx, session, "sandrone_put_file", map[string]any{
		"name":   "inline.txt",
		"kind":   "static",
		"source": map[string]any{"type": "inline", "content": "file"},
		"processors": []any{scriptProcessor("file", map[string]any{
			"type": "inline",
			"content": `function main(input) {
  input.file.content += "-stage";
  return input;
}`,
		}, nil)},
	})
	file := callToolSuccess(t, ctx, session, "sandrone_get_file", map[string]any{
		"file": "inline.txt",
		"mode": "render",
	})
	require.Equal(t, "file-stage", file["body"])
}

func TestScriptContractControlledStoredAndRemoteSources(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{
		MCP: app.MCPConfig{AllowManagementTools: true},
	})))
	defer session.Close()

	callToolSuccess(t, ctx, session, "sandrone_put_file", map[string]any{
		"name": "rename.js",
		"kind": "static",
		"source": map[string]any{
			"type": "inline",
			"content": `function main(input) {
  input.nodes[0].name = "stored-" + input.nodes[0].name;
  return input;
}`,
		},
	})

	remoteRequests := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		remoteRequests++
		_, _ = w.Write([]byte(`function main(input) {
  input.nodes[0].name = "remote-" + input.nodes[0].name;
  return input;
}`))
	}))
	defer remote.Close()

	permissionVariants := []struct {
		name        string
		permissions map[string]any
	}{
		{name: "without_permissions"},
		{
			name: "with_reserved_permissions",
			permissions: map[string]any{
				"network":   true,
				"resources": []any{"rename.js"},
			},
		},
	}
	for _, variant := range permissionVariants {
		t.Run("stored/"+variant.name, func(t *testing.T) {
			name := "stored-" + variant.name
			putScriptSubscription(t, ctx, session, name, map[string]any{
				"type": "file",
				"name": "rename.js",
			}, variant.permissions)
			stored := callToolSuccess(t, ctx, session, "sandrone_preview_subscription", map[string]any{"name": name})
			require.Equal(t, "stored-node-a", previewNodeName(t, stored))
		})
		t.Run("remote/"+variant.name, func(t *testing.T) {
			name := "remote-" + variant.name
			putScriptSubscription(t, ctx, session, name, map[string]any{
				"type":   "remote",
				"remote": map[string]any{"url": remote.URL},
			}, variant.permissions)
			remotePreview := callToolSuccess(t, ctx, session, "sandrone_preview_subscription", map[string]any{"name": name})
			require.Equal(t, "remote-node-a", previewNodeName(t, remotePreview))
		})
	}
	require.Equal(t, len(permissionVariants), remoteRequests)
}

func TestScriptContractTimeoutReturnsStructuredCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_convert",
		Arguments: map[string]any{
			"from_format": "uri-list",
			"to_format":   "json-nodes",
			"content":     scriptContractNode,
			"parse_processors": []any{scriptProcessor("nodes", map[string]any{
				"type":    "inline",
				"content": `function main(input) { while (true) {} }`,
			}, map[string]any{"timeout_ms": 10})},
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)

	var output struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	body, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(body, &output))
	require.Equal(t, "script_timeout", output.Error.Code)
}

func TestScriptContractHostCapabilitiesAndArbitraryNetworkAreUnavailable(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	permissionVariants := []struct {
		name        string
		permissions map[string]any
	}{
		{name: "without_permissions"},
		{name: "with_reserved_network_permission", permissions: map[string]any{"network": true}},
	}
	checks := []struct {
		name       string
		expression string
	}{
		{name: "require", expression: "typeof require"},
		{name: "process and environment", expression: "typeof process"},
		{name: "filesystem", expression: "typeof Deno + ':' + typeof Bun"},
		{name: "filesystem globals", expression: "typeof readFile + ':' + typeof fs + ':' + typeof os"},
		{name: "fetch", expression: "typeof fetch"},
		{name: "XMLHttpRequest", expression: "typeof XMLHttpRequest"},
		{name: "WebSocket", expression: "typeof WebSocket"},
	}
	for _, variant := range permissionVariants {
		t.Run(variant.name, func(t *testing.T) {
			for _, check := range checks {
				t.Run(check.name, func(t *testing.T) {
					params := map[string]any{}
					if variant.permissions != nil {
						params["permissions"] = variant.permissions
					}
					result := callToolSuccess(t, ctx, session, "sandrone_convert", map[string]any{
						"from_format": "uri-list",
						"to_format":   "json-nodes",
						"content":     scriptContractNode,
						"parse_processors": []any{scriptProcessor("nodes", map[string]any{
							"type": "inline",
							"content": `function main(input) {
  input.nodes[0].name = (` + check.expression + `) + "-" + input.nodes[0].name;
  return input;
}`,
						}, params)},
					})
					require.Contains(t, result["body"], "undefined-node-a")
					require.NotContains(t, result["body"], "object-node-a")
					require.NotContains(t, result["body"], "function-node-a")
				})
			}
		})
	}
}

func scriptProcessor(stage string, source map[string]any, additions map[string]any) map[string]any {
	params := map[string]any{"source": source}
	for name, value := range additions {
		params[name] = value
	}
	return map[string]any{
		"type":   "script",
		"stage":  stage,
		"params": params,
	}
}

func putScriptSubscription(
	t *testing.T,
	ctx context.Context,
	session *mcp.ClientSession,
	name string,
	source map[string]any,
	permissions map[string]any,
) {
	t.Helper()
	params := map[string]any{}
	if permissions != nil {
		params["permissions"] = permissions
	}
	callToolSuccess(t, ctx, session, "sandrone_put_subscription", map[string]any{
		"name":    name,
		"type":    "local",
		"format":  "uri-list",
		"content": base64.StdEncoding.EncodeToString([]byte(scriptContractNode)),
		"processors": []any{
			scriptProcessor("nodes", source, params),
		},
	})
}

func previewNodeName(t *testing.T, preview map[string]any) string {
	t.Helper()
	nodes, ok := preview["nodes"].([]any)
	require.True(t, ok)
	require.Len(t, nodes, 1)
	node, ok := nodes[0].(map[string]any)
	require.True(t, ok)
	after, ok := node["after"].(map[string]any)
	require.True(t, ok)
	name, ok := after["name"].(string)
	require.True(t, ok)
	return name
}
