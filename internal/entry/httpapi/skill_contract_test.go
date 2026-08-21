package httpapi_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func TestSandroneSkillSupportsHTTPWithoutMandatoryMCP(t *testing.T) {
	skill := readRepoFile(t, "../../../skills/sandrone/SKILL.md")
	metadata := readRepoFile(t, "../../../skills/sandrone/agents/openai.yaml")
	workflows := readRepoFile(t, "../../../skills/sandrone/references/workflows.md")

	require.Contains(t, skill, "SANDRONE_URL")
	require.Contains(t, skill, "scripts/sandrone-api.sh")
	require.Contains(t, skill, "HTTP")
	require.Contains(t, skill, "MCP")
	require.Contains(t, skill, "2026-07-28")
	require.Contains(t, skill, "legacy initialize/initialized session lifecycle")
	require.NotContains(t, skill, "compatibility:")
	require.NotContains(t, skill, "Use Sandrone MCP as the only execution plane")

	require.NotContains(t, metadata, `type: "mcp"`)
	httpIndex := strings.Index(skill, "HTTP script")
	mcpIndex := strings.Index(skill, "MCP")
	require.NotEqual(t, -1, httpIndex)
	require.NotEqual(t, -1, mcpIndex)
	require.Less(t, httpIndex, mcpIndex)

	for _, endpoint := range []string{
		"/v1/capabilities/formats",
		"/v1/capabilities/formats/{direction}/{format}",
		"/v1/capabilities/ui",
		"/v1/convert",
		"/v1/subscriptions/{name}/render",
		"/v1/schemas/processors",
		"/v1/schemas/file-kinds/{kind}",
		"/v1/schemas/script-api/v1",
		"/v1/schemas/subscription",
		"/v1/schemas/file-spec",
	} {
		require.Contains(t, workflows, endpoint)
	}
}

func TestSandroneHTTPShapeDocumentationIsLinked(t *testing.T) {
	schemas := readRepoFile(t, "../../../docs/reference/http-api/schemas.md")
	conversion := readRepoFile(t, "../../../docs/reference/http-api/conversion.md")
	subscriptions := readRepoFile(t, "../../../docs/reference/http-api/subscriptions.md")
	httpIndex := readRepoFile(t, "../../../docs/reference/http-api/README.md")
	mcpReference := readRepoFile(t, "../../../docs/reference/mcp.md")

	for _, route := range []string{
		"GET /v1/schemas",
		"GET /v1/schemas/processors",
		"GET /v1/schemas/file-kinds",
		"GET /v1/schemas/processors/{stage}/{type}",
		"GET /v1/schemas/file-kinds/{kind}",
		"GET /v1/schemas/script-api/v1",
		"GET /v1/schemas/subscription",
		"GET /v1/schemas/file-spec",
	} {
		require.Contains(t, schemas, route)
	}
	require.Contains(t, conversion, "POST /v1/convert")
	require.Contains(t, subscriptions, "POST /v1/subscriptions/{name}/render")
	require.Contains(t, httpIndex, "schemas.md")
	require.Contains(t, mcpReference, "SANDRONE_URL")
	require.Contains(t, mcpReference, "可选")
}
