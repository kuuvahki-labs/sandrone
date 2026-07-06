package jsonnodes_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/jsonnodes"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestParserParseArrayAndSetsSourceFormat(t *testing.T) {
	parser := jsonnodes.NewParser()

	nodes, source, err := parser.Parse(context.Background(), []byte(`[
  {
    "name": "node-a",
    "type": "ss",
    "server": "example.com",
    "port": 8388,
    "cipher": "aes-128-gcm",
    "password": "secret"
  }
]`))

	require.NoError(t, err)
	require.Equal(t, "json-nodes", parser.Name())
	require.Equal(t, "json-nodes", source.Format)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeShadowsocks, nodes[0].Type)
	require.Equal(t, "json-nodes", nodes[0].SourceFormat)
}

func TestParserParseWrappedNodesPreservesExistingSourceFormat(t *testing.T) {
	parser := jsonnodes.NewParser()

	nodes, source, err := parser.Parse(context.Background(), []byte(`{
  "nodes": [
    {
      "name": "node-a",
      "type": "socks",
      "server": "example.com",
      "port": 1080,
      "source_format": "custom"
    }
  ]
}`))

	require.NoError(t, err)
	require.Equal(t, "json-nodes", source.Format)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeSOCKS, nodes[0].Type)
	require.Equal(t, "custom", nodes[0].SourceFormat)
}

func TestParserRejectsInvalidJSON(t *testing.T) {
	parser := jsonnodes.NewParser()

	nodes, source, err := parser.Parse(context.Background(), []byte(`{"nodes":`))

	require.Error(t, err)
	require.Nil(t, nodes)
	require.Equal(t, "json-nodes", source.Format)
	require.True(t, domain.IsCode(err, domain.CodeParseFailed), "unexpected error: %v", err)
}

func TestRendererRenderJSONNodes(t *testing.T) {
	renderer := jsonnodes.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:     "node-a",
			Type:     domain.NodeTypeShadowsocks,
			Server:   "example.com",
			Port:     8388,
			Cipher:   "aes-128-gcm",
			Password: "secret",
		},
	}

	out, report, err := renderer.RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, "json-nodes", renderer.Name())
	require.Equal(t, 1, report.SuccessCount)
	require.JSONEq(t, `[
  {
    "name": "node-a",
    "type": "ss",
    "server": "example.com",
    "port": 8388,
    "cipher": "aes-128-gcm",
    "password": "secret"
  }
]`, string(out))

	rendered, err := renderer.Render(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, out, rendered)

	var decoded []domain.NodeIR
	require.NoError(t, json.Unmarshal(rendered, &decoded))
	require.Equal(t, nodes[0].Name, decoded[0].Name)
}
