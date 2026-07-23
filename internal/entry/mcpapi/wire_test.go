package mcpapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

func TestProcessorParamsAcceptOrdinaryObjects(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	tests := []struct {
		name          string
		processorsKey string
		processors    []any
		want          string
		notWant       string
	}{
		{
			name:          "rename",
			processorsKey: "render_processors",
			processors: []any{map[string]any{
				"type":  "rename",
				"stage": "nodes",
				"params": map[string]any{
					"mode":  "prefix",
					"value": "HK-",
				},
			}},
			want: "HK-hk-a",
		},
		{
			name:          "filter",
			processorsKey: "parse_processors",
			processors: []any{map[string]any{
				"type":  "filter",
				"stage": "nodes",
				"params": map[string]any{
					"action":  "keep",
					"field":   "name",
					"match":   "regex",
					"pattern": "^hk-",
				},
			}},
			want:    "hk-a",
			notWant: "us-a",
		},
		{
			name:          "inline script",
			processorsKey: "render_processors",
			processors: []any{map[string]any{
				"type":  "script",
				"stage": "nodes",
				"params": map[string]any{
					"source": map[string]any{
						"type":    "inline",
						"content": `function main(input) { input.nodes.forEach((node) => node.name = "JS-" + node.name); return input; }`,
					},
				},
			}},
			want: "JS-hk-a",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arguments := map[string]any{
				"from_format": "uri-list",
				"to_format":   "json-nodes",
				"content": "ss://aes-128-gcm:secret@example.com:8388#hk-a\n" +
					"ss://aes-128-gcm:secret@example.com:8388#us-a",
				test.processorsKey: test.processors,
			}
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "sandrone_convert",
				Arguments: arguments,
			})
			require.NoError(t, err)
			require.False(t, result.IsError)
			body, err := json.Marshal(result)
			require.NoError(t, err)
			require.Contains(t, string(body), test.want)
			if test.notWant != "" {
				require.NotContains(t, string(body), test.notWant)
			}
		})
	}
}

func TestTypedSettingsAcceptOrdinaryObjectBeforeDriverValidation(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{
		MCP: app.MCPConfig{AllowManagementTools: true},
	})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	body := callToolError(t, ctx, session, "sandrone_put_file", map[string]any{
		"name": "bad.yaml",
		"kind": "mihomo",
		"source": map[string]any{
			"type":    "inline",
			"content": "proxies: []",
		},
		"config": map[string]any{
			"settings": map[string]any{"future": true},
		},
	})
	require.Contains(t, body, "config.settings.future")
	require.NotContains(t, body, "want array")
}

func TestTypedFileEmptySourceUsesBuiltinBaseForPutAndValidate(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{
		MCP: app.MCPConfig{AllowManagementTools: true},
	})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	put, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_put_file",
		Arguments: map[string]any{
			"name":   "builtin.yaml",
			"kind":   "mihomo",
			"source": map[string]any{},
		},
	})
	require.NoError(t, err)
	require.False(t, put.IsError)

	source, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_get_file",
		Arguments: map[string]any{
			"file": "builtin.yaml",
			"mode": "source",
		},
	})
	require.NoError(t, err)
	require.False(t, source.IsError)
	sourceBody, err := json.Marshal(source)
	require.NoError(t, err)
	require.Contains(t, string(sourceBody), "proxy-groups")

	validate, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_validate_file",
		Arguments: map[string]any{
			"spec": map[string]any{
				"kind":   "mihomo",
				"source": map[string]any{},
			},
		},
	})
	require.NoError(t, err)
	require.False(t, validate.IsError)
}

func TestPutFileRequiresNameAtMCPBoundary(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{
		MCP: app.MCPConfig{AllowManagementTools: true},
	})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	body := callToolError(t, ctx, session, "sandrone_put_file", map[string]any{
		"kind":   "mihomo",
		"source": map[string]any{},
	})
	require.Contains(t, body, "missing properties")
	require.Contains(t, body, "name")
}

func TestSubscriptionAutoFormatRoundTripsThroughMCP(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{
		MCP: app.MCPConfig{AllowManagementTools: true},
	})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	put, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_put_subscription",
		Arguments: map[string]any{
			"name":    "auto-sub",
			"type":    "local",
			"format":  "auto",
			"content": "ss://aes-128-gcm:secret@example.com:8388#auto-node",
		},
	})
	require.NoError(t, err)
	require.False(t, put.IsError)

	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "sandrone://subscriptions/auto-sub",
	})
	require.NoError(t, err)
	require.Len(t, resource.Contents, 1)
	require.Contains(t, resource.Contents[0].Text, `"format": "auto"`)
}
