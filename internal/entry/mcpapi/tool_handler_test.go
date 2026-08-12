package mcpapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

type bodyLimitedOutput struct {
	ContentType    string        `json:"content_type"`
	Body           string        `json:"body"`
	BodyOmitted    bool          `json:"body_omitted"`
	BodyBytes      int           `json:"body_bytes"`
	MaxOutputBytes int           `json:"max_output_bytes"`
	Report         domain.Report `json:"report"`
}

func TestBodyLimitConvertReportsExplicitOmissionAndExactBoundary(t *testing.T) {
	const content = "ss://aes-128-gcm:secret@example.com:8388#node-a"
	full := callBodyTool(t, app.MCPConfig{MaxOutputBytes: 4096}, "sandrone_convert", map[string]any{
		"content": content, "from_format": "uri-list", "to_format": "uri-list",
	})
	require.NotEmpty(t, full.Body)

	omitted := callBodyTool(t, app.MCPConfig{MaxOutputBytes: len(full.Body) - 1}, "sandrone_convert", map[string]any{
		"content": content, "from_format": "uri-list", "to_format": "uri-list",
	})
	require.Empty(t, omitted.Body)
	require.True(t, omitted.BodyOmitted)
	require.Equal(t, len(full.Body), omitted.BodyBytes)
	require.Equal(t, len(full.Body)-1, omitted.MaxOutputBytes)
	require.NotEmpty(t, omitted.Report.Kind)

	exact := callBodyTool(t, app.MCPConfig{MaxOutputBytes: len(full.Body)}, "sandrone_convert", map[string]any{
		"content": content, "from_format": "uri-list", "to_format": "uri-list",
	})
	require.Equal(t, full.Body, exact.Body)
	require.False(t, exact.BodyOmitted)
	require.Zero(t, exact.BodyBytes)
	require.Zero(t, exact.MaxOutputBytes)
}

func TestBodyLimitSubscriptionRenderReportsExplicitOmission(t *testing.T) {
	const content = "ss://aes-128-gcm:secret@example.com:8388#node-a"
	full := callSubscriptionRenderBody(t, content, 4096)
	require.NotEmpty(t, full.Body)

	exact := callSubscriptionRenderBody(t, content, len(full.Body))
	require.Equal(t, full.Body, exact.Body)
	require.False(t, exact.BodyOmitted)
	require.Zero(t, exact.BodyBytes)
	require.Zero(t, exact.MaxOutputBytes)

	omitted := callSubscriptionRenderBody(t, content, len(full.Body)-1)
	require.Empty(t, omitted.Body)
	require.True(t, omitted.BodyOmitted)
	require.Equal(t, len(full.Body), omitted.BodyBytes)
	require.Equal(t, len(full.Body)-1, omitted.MaxOutputBytes)
	require.NotEmpty(t, omitted.Report.Kind)
}

func callSubscriptionRenderBody(t *testing.T, content string, limit int) bodyLimitedOutput {
	t.Helper()
	ctx := context.Background()
	rt := testRuntime(t, app.Config{MCP: app.MCPConfig{MaxOutputBytes: limit}})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "local", Type: domain.SubscriptionTypeLocal, Format: "uri-list", Content: content,
	}))
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sandrone_render_subscription",
		Arguments: map[string]any{"name": "local", "format": "uri-list"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	var output bodyLimitedOutput
	decodeStructuredContent(t, result, &output)
	return output
}

func TestBodyLimitFileRenderReportsExplicitOmission(t *testing.T) {
	full := callFileRenderBody(t, "long body", 4096)
	require.Equal(t, "long body", full.Body)

	exact := callFileRenderBody(t, "long body", len(full.Body))
	require.Equal(t, full.Body, exact.Body)
	require.False(t, exact.BodyOmitted)
	require.Zero(t, exact.BodyBytes)
	require.Zero(t, exact.MaxOutputBytes)

	omitted := callFileRenderBody(t, "long body", len(full.Body)-1)
	require.Empty(t, omitted.Body)
	require.True(t, omitted.BodyOmitted)
	require.Equal(t, len(full.Body), omitted.BodyBytes)
	require.Equal(t, len(full.Body)-1, omitted.MaxOutputBytes)
	require.NotEmpty(t, omitted.Report.Kind)
}

func callFileRenderBody(t *testing.T, content string, limit int) bodyLimitedOutput {
	t.Helper()
	ctx := context.Background()
	rt := testRuntime(t, app.Config{MCP: app.MCPConfig{MaxOutputBytes: limit}})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name: "large.txt", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: content},
	}))
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sandrone_get_file",
		Arguments: map[string]any{"file": "large.txt", "mode": "render"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	var output bodyLimitedOutput
	decodeStructuredContent(t, result, &output)
	return output
}

func TestBodyLimitDoesNotClaimGlobalLimitsForNonBodyOutputs(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{MCP: app.MCPConfig{MaxOutputBytes: 1}})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	for _, call := range []*mcp.CallToolParams{
		{Name: "sandrone_list_resources"},
		{Name: "sandrone_inspect"},
	} {
		result, err := session.CallTool(ctx, call)
		require.NoError(t, err)
		require.False(t, result.IsError)
		body, err := json.Marshal(result.StructuredContent)
		require.NoError(t, err)
		require.NotContains(t, string(body), "body_omitted")
		require.NotContains(t, string(body), "max_output_bytes")
	}

	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	for _, tool := range tools.Tools {
		if tool.Name != "sandrone_probe_nodes" && tool.Name != "sandrone_preview_subscription" {
			continue
		}
		body, err := json.Marshal(tool.OutputSchema)
		require.NoError(t, err)
		require.NotContains(t, string(body), "body_omitted")
		require.NotContains(t, string(body), "max_output_bytes")
	}
}

func TestToolOutputSchemaAllowsSuccessAndStructuredError(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	var convertTool *mcp.Tool
	for _, tool := range tools.Tools {
		if tool.Name == "sandrone_convert" {
			convertTool = tool
			break
		}
	}
	require.NotNil(t, convertTool)
	schemaJSON, err := json.Marshal(convertTool.OutputSchema)
	require.NoError(t, err)
	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal(schemaJSON, &schema))
	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	require.NoError(t, err)

	for _, arguments := range []map[string]any{
		{"content": "ss://aes-128-gcm:secret@example.com:8388#node", "from_format": "uri-list", "to_format": "uri-list"},
		{"from_format": "uri-list", "to_format": "uri-list"},
	} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "sandrone_convert", Arguments: arguments,
		})
		require.NoError(t, err)
		bodyJSON, err := json.Marshal(result.StructuredContent)
		require.NoError(t, err)
		var body map[string]any
		require.NoError(t, json.Unmarshal(bodyJSON, &body))
		require.NoError(t, resolved.Validate(body))
	}
}

func callBodyTool(t *testing.T, cfg app.MCPConfig, tool string, arguments map[string]any) bodyLimitedOutput {
	t.Helper()
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{MCP: cfg})))
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: arguments})
	require.NoError(t, err)
	require.False(t, result.IsError)
	var output bodyLimitedOutput
	decodeStructuredContent(t, result, &output)
	return output
}
