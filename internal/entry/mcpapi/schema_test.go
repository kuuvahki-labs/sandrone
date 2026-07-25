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

func TestToolSchemasExposeObjectValuesAndBoundaryValidation(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{
		MCP: app.MCPConfig{AllowManagementTools: true},
	})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	result, err := session.ListTools(ctx, nil)
	require.NoError(t, err)

	require.JSONEq(t, `{"type":"object"}`, extractSchema(
		t, result.Tools, "sandrone_convert", "properties.parse_processors.items.properties.params",
	))
	require.JSONEq(t, `{"type":"object"}`, extractSchema(
		t, result.Tools, "sandrone_put_file", "properties.config.properties.settings",
	))
	require.JSONEq(t, `[
		{"required":["content","from_format"],"not":{"required":["remote"]}},
		{"required":["remote"],"not":{"required":["content"]}}
	]`, extractSchema(t, result.Tools, "sandrone_convert", "oneOf"))
	require.JSONEq(t, `[
		{"required":["file"],"not":{"required":["spec"]}},
		{"required":["spec"],"not":{"required":["file"]}}
	]`, extractSchema(t, result.Tools, "sandrone_validate_file", "oneOf"))

	require.JSONEq(t, `["spec","source","render"]`, extractSchema(
		t, result.Tools, "sandrone_get_file", "properties.mode.enum",
	))
	require.JSONEq(t, `["tcp_connect","udp_ntp","url_test"]`, extractSchema(
		t, result.Tools, "sandrone_probe_nodes", "properties.method.enum",
	))
	require.JSONEq(t, `["mihomo","sing-box"]`, extractSchema(
		t, result.Tools, "sandrone_probe_nodes", "properties.core.enum",
	))
	require.JSONEq(t, `["uri","uri-list","base64","mihomo","sing-box","json-nodes"]`, extractSchema(
		t, result.Tools, "sandrone_convert", "properties.from_format.enum",
	))
	require.JSONEq(t, `["base64","mihomo-proxies","shadowrocket-proxies","sing-box-outbounds","json-nodes","uri-list"]`, extractSchema(
		t, result.Tools, "sandrone_convert", "properties.to_format.enum",
	))
	require.JSONEq(t, `["base64","mihomo-proxies","shadowrocket-proxies","sing-box-outbounds","json-nodes","uri-list"]`, extractSchema(
		t, result.Tools, "sandrone_convert", "properties.options.properties.format.enum",
	))
	require.JSONEq(t, `["auto","uri","uri-list","base64","mihomo","sing-box","json-nodes"]`, extractSchema(
		t, result.Tools, "sandrone_put_subscription", "properties.format.enum",
	))
	require.JSONEq(t, `["inline_nodes","inline","local","remote","ref","subscription"]`, extractSchema(
		t, result.Tools, "sandrone_probe_nodes", "properties.input.properties.type.enum",
	))
	require.JSONEq(t, `["uri","uri-list","base64","mihomo","sing-box","json-nodes"]`, extractSchema(
		t, result.Tools, "sandrone_probe_nodes", "properties.input.properties.format.enum",
	))
	require.JSONEq(t, `["name","kind","source"]`, extractSchema(
		t, result.Tools, "sandrone_put_file", "required",
	))
	require.JSONEq(t, `["kind","source"]`, extractSchema(
		t, result.Tools, "sandrone_validate_file", "properties.spec.required",
	))
	requireSchemaPathMissing(
		t, result.Tools, "sandrone_put_file", "properties.source.required",
	)

	for _, path := range []string{
		"properties.remote.properties.timeout_ms.minimum",
		"properties.remote.properties.cache_ttl_seconds.minimum",
		"properties.input.properties.timeout_ms.minimum",
		"properties.input.properties.cache_ttl_seconds.minimum",
		"properties.timeout_ms.minimum",
		"properties.attempts.minimum",
		"properties.concurrency.minimum",
		"properties.cache_ttl_seconds.minimum",
	} {
		require.JSONEq(t, `0`, extractSchema(t, result.Tools, schemaTool(path), path), path)
	}
}

func TestToolSchemaEnumsRejectUnknownValues(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{
		MCP: app.MCPConfig{AllowManagementTools: true},
	})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	tests := []struct {
		name      string
		tool      string
		arguments map[string]any
	}{
		{
			name: "parse format",
			tool: "sandrone_convert",
			arguments: map[string]any{
				"from_format": "future",
				"to_format":   "json-nodes",
				"content":     "ss://aes-128-gcm:secret@example.com:8388#node",
			},
		},
		{
			name: "render format",
			tool: "sandrone_convert",
			arguments: map[string]any{
				"from_format": "uri-list",
				"to_format":   "future",
				"content":     "ss://aes-128-gcm:secret@example.com:8388#node",
			},
		},
		{
			name: "render options format",
			tool: "sandrone_convert",
			arguments: map[string]any{
				"from_format": "uri-list",
				"to_format":   "json-nodes",
				"content":     "ss://aes-128-gcm:secret@example.com:8388#node",
				"options":     map[string]any{"format": "future"},
			},
		},
		{
			name: "subscription format",
			tool: "sandrone_put_subscription",
			arguments: map[string]any{
				"name":    "bad-format",
				"type":    "local",
				"format":  "future",
				"content": "ss://aes-128-gcm:secret@example.com:8388#node",
			},
		},
		{
			name: "node input type",
			tool: "sandrone_probe_nodes",
			arguments: map[string]any{
				"input": map[string]any{"name": "probe", "type": "future"},
			},
		},
		{
			name: "node input format",
			tool: "sandrone_probe_nodes",
			arguments: map[string]any{
				"input": map[string]any{
					"name":    "probe",
					"type":    "inline",
					"format":  "future",
					"content": "ss://aes-128-gcm:secret@example.com:8388#node",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := callToolError(t, ctx, session, test.tool, test.arguments)
			require.Contains(t, body, "does not equal any of")
		})
	}
}

func schemaTool(path string) string {
	if len(path) >= len("properties.remote") && path[:len("properties.remote")] == "properties.remote" {
		return "sandrone_convert"
	}
	return "sandrone_probe_nodes"
}

func extractSchema(t *testing.T, tools []*mcp.Tool, toolName, path string) string {
	t.Helper()
	var value any
	for _, tool := range tools {
		if tool.Name != toolName {
			continue
		}
		body, err := json.Marshal(tool.InputSchema)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &value))
		break
	}
	require.NotNil(t, value, toolName)
	for _, part := range splitSchemaPath(path) {
		switch current := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = current[part]
			require.True(t, ok, "%s missing %s", path, part)
		default:
			require.FailNow(t, "schema path is not an object", "%s at %s", path, part)
		}
	}
	body, err := json.Marshal(value)
	require.NoError(t, err)
	return string(body)
}

func requireSchemaPathMissing(t *testing.T, tools []*mcp.Tool, toolName, path string) {
	t.Helper()
	var value any
	for _, tool := range tools {
		if tool.Name != toolName {
			continue
		}
		body, err := json.Marshal(tool.InputSchema)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &value))
		break
	}
	require.NotNil(t, value, toolName)
	parts := splitSchemaPath(path)
	for index, part := range parts {
		current, ok := value.(map[string]any)
		require.True(t, ok, "%s at %s", path, part)
		value, ok = current[part]
		if index == len(parts)-1 {
			require.False(t, ok, "%s unexpectedly exists", path)
			return
		}
		require.True(t, ok, "%s missing %s", path, part)
	}
}

func splitSchemaPath(path string) []string {
	var parts []string
	for path != "" {
		index := 0
		for index < len(path) && path[index] != '.' {
			index++
		}
		parts = append(parts, path[:index])
		if index == len(path) {
			break
		}
		path = path[index+1:]
	}
	return parts
}
