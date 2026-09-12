package httpapi_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSandroneSkillDoesNotRequireMCPDependency(t *testing.T) {
	content, err := os.ReadFile("../../../skills/sandrone/agents/openai.yaml")
	require.NoError(t, err)
	var metadata struct {
		Dependencies struct {
			Tools []struct {
				Type string `yaml:"type"`
			} `yaml:"tools"`
		} `yaml:"dependencies"`
	}
	require.NoError(t, yaml.Unmarshal(content, &metadata))
	for _, tool := range metadata.Dependencies.Tools {
		require.NotEqual(t, "mcp", tool.Type, "HTTP usage must not require an MCP connection")
	}
}
