package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestNodeLineageIsRuntimeOnly(t *testing.T) {
	node := domain.NodeIR{Name: "node", Type: domain.NodeTypeShadowsocks}
	domain.SetNodeLineage(&node, "origin-1")

	require.Equal(t, "origin-1", domain.NodeLineage(node))

	jsonBody, err := json.Marshal(node)
	require.NoError(t, err)
	require.NotContains(t, string(jsonBody), "origin-1")
	require.NotContains(t, string(jsonBody), "lineage")

	yamlBody, err := yaml.Marshal(node)
	require.NoError(t, err)
	require.NotContains(t, string(yamlBody), "origin-1")
	require.NotContains(t, string(yamlBody), "lineage")

	domain.ClearNodeLineage(&node)
	require.Empty(t, domain.NodeLineage(node))
}
