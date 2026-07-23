package mcpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
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

func TestStdioSmoke(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sandrone")
	buildCtx, cancelBuild := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "./cmd/sandrone")
	build.Dir = repositoryRoot(t)
	buildOutput, err := build.CombinedOutput()
	require.NoError(t, err, "%s", buildOutput)
	cancelBuild()

	protocolCtx, cancelProtocol := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelProtocol()
	cmd := exec.Command(
		binary,
		"--data-dir", t.TempDir(),
		"serve", "mcp", "--transport", "stdio",
	)
	cmd.Dir = repositoryRoot(t)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	transport := &mcp.CommandTransport{
		Command:           cmd,
		TerminateDuration: 2 * time.Second,
	}
	session := connectClient(t, protocolCtx, transport)
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = session.Close()
		}
	})
	require.NotEmpty(t, session.InitializeResult().ProtocolVersion)

	tools, err := session.ListTools(protocolCtx, nil)
	require.NoError(t, err)
	require.NotEmpty(t, tools.Tools)

	call, err := session.CallTool(protocolCtx, &mcp.CallToolParams{
		Name:      "sandrone_inspect_capabilities",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, call.IsError)
	callJSON, err := json.Marshal(call.StructuredContent)
	require.NoError(t, err)
	require.Contains(t, string(callJSON), `"capabilities"`)

	require.NoError(t, session.Close(), "stderr: %s", stderr.String())
	closed = true
	require.NotNil(t, cmd.ProcessState)
	require.True(t, cmd.ProcessState.Success(), "stderr: %s", stderr.String())
	require.Contains(t, stderr.String(), "starting MCP server")
	require.Contains(t, stderr.String(), "MCP server stopped")
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	return root
}
