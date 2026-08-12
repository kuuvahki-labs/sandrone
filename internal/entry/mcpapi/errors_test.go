package mcpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

type structuredToolError struct {
	Error struct {
		Code         string `json:"code"`
		Message      string `json:"message"`
		Field        string `json:"field,omitempty"`
		ResourceKind string `json:"resource_kind,omitempty"`
		ResourceName string `json:"resource_name,omitempty"`
		Source       string `json:"source,omitempty"`
		Target       string `json:"target,omitempty"`
		File         string `json:"file,omitempty"`
		Part         string `json:"part,omitempty"`
		Processor    string `json:"processor,omitempty"`
		Path         string `json:"path,omitempty"`
	} `json:"error"`
}

func TestStructuredToolErrorsInputValidationForEveryAlwaysRegisteredTool(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	tests := []struct {
		tool      string
		arguments map[string]any
		field     string
	}{
		{
			tool: "sandrone_convert",
			arguments: map[string]any{
				"content": "", "from_format": "uri-list", "to_format": "uri-list",
				"parse_processors": []any{
					map[string]any{"type": "rename", "stage": "nodes"},
					map[string]any{"type": "rename", "stage": "future"},
				},
			},
			field: "parse_processors[1].stage",
		},
		{tool: "sandrone_probe_nodes", arguments: map[string]any{}, field: "input"},
		{tool: "sandrone_list_resources", arguments: map[string]any{"limit": 201}, field: "limit"},
		{tool: "sandrone_inspect", arguments: map[string]any{"unexpected": true}, field: "unexpected"},
		{tool: "sandrone_preview_subscription", arguments: map[string]any{}, field: "name"},
		{tool: "sandrone_render_subscription", arguments: map[string]any{"name": "demo"}, field: "format"},
		{tool: "sandrone_get_subscription_traffic", arguments: map[string]any{}, field: "name"},
		{tool: "sandrone_get_file", arguments: map[string]any{}, field: "file"},
		{tool: "sandrone_validate_file", arguments: map[string]any{}, field: ""},
	}
	for _, test := range tests {
		t.Run(test.tool, func(t *testing.T) {
			body := callStructuredToolError(t, ctx, session, test.tool, test.arguments)
			require.Equal(t, string(domain.CodeInvalidArgument), body.Error.Code)
			require.NotEmpty(t, body.Error.Message)
			require.Equal(t, test.field, body.Error.Field)
		})
	}
}

func TestStructuredToolErrorsServiceFailuresForEveryAlwaysRegisteredTool(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	tests := []struct {
		name         string
		tool         string
		arguments    map[string]any
		code         domain.ErrorCode
		resourceKind string
		resourceName string
	}{
		{
			name: "convert parse", tool: "sandrone_convert",
			arguments: map[string]any{"content": "not-a-node", "from_format": "uri-list", "to_format": "uri-list"},
			code:      domain.CodeParseFailed,
		},
		{
			name: "probe invalid node", tool: "sandrone_probe_nodes",
			arguments: map[string]any{"input": map[string]any{"name": "bad", "type": "inline", "format": "uri-list", "content": "not-a-node"}},
			code:      domain.CodeParseFailed,
		},
		{
			name: "list cursor", tool: "sandrone_list_resources",
			arguments: map[string]any{"cursor": "invalid"},
			code:      domain.CodeInvalidArgument,
		},
		{
			name: "preview missing", tool: "sandrone_preview_subscription",
			arguments: map[string]any{"name": "missing"},
			code:      domain.CodeFileInputNotFound, resourceKind: "subscription", resourceName: "missing",
		},
		{
			name: "render missing", tool: "sandrone_render_subscription",
			arguments: map[string]any{"name": "missing", "format": "uri-list"},
			code:      domain.CodeFileInputNotFound, resourceKind: "subscription", resourceName: "missing",
		},
		{
			name: "traffic missing", tool: "sandrone_get_subscription_traffic",
			arguments: map[string]any{"name": "missing"},
			code:      domain.CodeFileInputNotFound, resourceKind: "subscription", resourceName: "missing",
		},
		{
			name: "get file missing", tool: "sandrone_get_file",
			arguments: map[string]any{"file": "missing", "mode": "render"},
			code:      domain.CodeFileInputNotFound, resourceKind: "file", resourceName: "missing",
		},
		{
			name: "validate file missing", tool: "sandrone_validate_file",
			arguments: map[string]any{"file": "missing"},
			code:      domain.CodeFileInputNotFound, resourceKind: "file", resourceName: "missing",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := callStructuredToolError(t, ctx, session, test.tool, test.arguments)
			require.Equal(t, string(test.code), body.Error.Code)
			require.NotEmpty(t, body.Error.Message)
			require.Equal(t, test.resourceKind, body.Error.ResourceKind)
			require.Equal(t, test.resourceName, body.Error.ResourceName)
		})
	}
}

func TestStructuredToolErrorsPreserveProcessorContextWithoutLeakingCause(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_convert",
		Arguments: map[string]any{
			"content":     "ss://aes-128-gcm:secret@example.com:8388#node",
			"from_format": "uri-list",
			"to_format":   "uri-list",
			"parse_processors": []any{map[string]any{
				"type":   "rename",
				"params": map[string]any{"mode": "not-valid", "credential": "do-not-leak"},
			}},
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	bodyJSON, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	var body structuredToolError
	require.NoError(t, json.Unmarshal(bodyJSON, &body))
	require.Equal(t, string(domain.CodeProcessorConfigInvalid), body.Error.Code)
	require.Equal(t, "rename", body.Error.Processor)
	require.Equal(t, "parse_processors[0].params.credential", body.Error.Field)
	require.NotContains(t, string(bodyJSON), "do-not-leak")
}

func TestStructuredToolErrorsOmitAmbiguousProcessorField(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	body := callStructuredToolError(t, ctx, session, "sandrone_convert", map[string]any{
		"content":     "ss://aes-128-gcm:secret@example.com:8388#node",
		"from_format": "uri-list",
		"to_format":   "uri-list",
		"parse_processors": []any{
			map[string]any{
				"type":   "rename",
				"params": map[string]any{"mode": "prefix", "value": "ok-"},
			},
			map[string]any{
				"type":   "rename",
				"params": map[string]any{"mode": "not-valid"},
			},
		},
	})
	require.Equal(t, string(domain.CodeProcessorConfigInvalid), body.Error.Code)
	require.Equal(t, "rename", body.Error.Processor)
	require.Empty(t, body.Error.Field)
}

func TestStructuredToolErrorsPreserveScriptTimeout(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	body := callStructuredToolError(t, ctx, session, "sandrone_convert", map[string]any{
		"content":     "ss://aes-128-gcm:secret@example.com:8388#node",
		"from_format": "uri-list",
		"to_format":   "uri-list",
		"parse_processors": []any{map[string]any{
			"type":  "script",
			"stage": "nodes",
			"params": map[string]any{
				"source": map[string]any{
					"type":    "inline",
					"content": `function main(input, api) { while (true) {} }`,
				},
				"timeout_ms": 10,
			},
		}},
	})
	require.Equal(t, string(domain.CodeScriptTimeout), body.Error.Code)
	require.Equal(t, "script", body.Error.Processor)
}

func TestStructuredToolErrorsPreserveRemoteFetchCodeWithoutLeakingBody(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("credential=do-not-leak"))
	}))
	defer remote.Close()

	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_convert",
		Arguments: map[string]any{
			"remote":    map[string]any{"url": remote.URL},
			"to_format": "uri-list",
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	bodyJSON, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(bodyJSON), "do-not-leak")
	var body structuredToolError
	structuredJSON, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(structuredJSON, &body))
	require.Equal(t, string(domain.CodeFileInputNotFound), body.Error.Code)
	require.Equal(t, "remote.url", body.Error.Field)
}

func TestStructuredToolErrorsSanitizeRemoteSourceURL(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer remote.Close()
	secretURL := strings.Replace(remote.URL, "http://", "http://user:pass@", 1) +
		"/private-path?token=do-not-leak#fragment-secret"

	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_convert",
		Arguments: map[string]any{
			"remote":    map[string]any{"url": secretURL},
			"to_format": "uri-list",
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	bodyJSON, err := json.Marshal(result)
	require.NoError(t, err)
	for _, secret := range []string{"user", "pass", "private-path", "token", "do-not-leak", "fragment-secret"} {
		require.NotContains(t, string(bodyJSON), secret)
	}
	var body structuredToolError
	structuredJSON, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(structuredJSON, &body))
	require.Equal(t, remote.URL, body.Error.Source)
}

func TestStructuredToolErrorsPreserveTypedSettingsDecodeCodeAndResource(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	body := callStructuredToolError(t, ctx, session, "sandrone_validate_file", map[string]any{
		"spec": map[string]any{
			"name": "bad.yaml",
			"kind": "mihomo",
			"source": map[string]any{
				"type":    "inline",
				"content": "proxies: []\n",
			},
			"config": map[string]any{
				"settings": map[string]any{"groups": nil},
			},
		},
	})
	require.Equal(t, string(domain.CodeInvalidArgument), body.Error.Code)
	require.Equal(t, "spec.config.settings.groups", body.Error.Field)
	require.Equal(t, "file", body.Error.ResourceKind)
	require.Equal(t, "bad.yaml", body.Error.ResourceName)
}

func callStructuredToolError(
	t *testing.T,
	ctx context.Context,
	session *mcp.ClientSession,
	tool string,
	arguments map[string]any,
) structuredToolError {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: arguments})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	bodyJSON, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	var body structuredToolError
	require.NoError(t, json.Unmarshal(bodyJSON, &body))
	require.Equal(t, body.Error.Code+": "+body.Error.Message, text.Text)
	return body
}
