package mcpapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestFileSourceOutputPreservesFileDocument(t *testing.T) {
	index := 2
	input := domain.FileDocument{
		Name:      "bundle.json",
		Kind:      "sing-box",
		MediaType: "application/json",
		Encoding:  "utf-8",
		Content:   []byte(`{"root":true}`),
		Meta:      map[string]string{"owner": "agent"},
		Warnings: []domain.Warning{{
			Code: "source_warning", Message: "kept", NodeIndex: &index, Field: "root",
		}},
		Parts: []domain.FilePart{{
			Name: "nodes", Role: "nodes", Kind: "json",
			SourceRef: domain.SourceRef{
				Kind: "remote", Name: "provider", URL: "https://example.com/sub",
			},
			Content:     []byte(`{"part":true}`),
			ContentHash: "sha256:abc",
			Nodes: []domain.NodeIR{{
				Name: "node-a", Type: domain.NodeTypeShadowsocks,
				Server: "example.com", Port: 8388,
				PluginOptions: map[string]any{
					"counter": json.Number("9007199254740993"),
				},
				Raw: map[string]json.RawMessage{
					"plugin": json.RawMessage(`{"mode":"fast","counter":9007199254740993}`),
				},
			}},
		}},
	}

	output, err := newFileSourceOutput(input)

	require.NoError(t, err)
	require.Equal(t, "bundle.json", output.Name)
	require.Equal(t, "sing-box", output.Kind)
	require.Equal(t, "application/json", output.MediaType)
	require.Equal(t, "utf-8", output.Encoding)
	require.Equal(t, `{"root":true}`, output.Content)
	require.Equal(t, input.Meta, output.Meta)
	require.Equal(t, input.Warnings, output.Warnings)
	require.Len(t, output.Parts, 1)
	require.Equal(t, "nodes", output.Parts[0].Name)
	require.Equal(t, "nodes", output.Parts[0].Role)
	require.Equal(t, "json", output.Parts[0].Kind)
	require.Equal(t, input.Parts[0].SourceRef, output.Parts[0].SourceRef)
	require.Equal(t, `{"part":true}`, output.Parts[0].Content)
	require.Equal(t, "sha256:abc", output.Parts[0].ContentHash)
	require.Equal(t, "node-a", output.Parts[0].Nodes[0]["name"])
	rawPlugin := output.Parts[0].Nodes[0]["raw"].(map[string]any)["plugin"].(map[string]any)
	require.Equal(t, "fast", rawPlugin["mode"])
	require.Equal(t, json.Number("9007199254740993"), rawPlugin["counter"])
	require.Equal(t, json.Number("9007199254740993"),
		output.Parts[0].Nodes[0]["plugin_options"].(map[string]any)["counter"])

	wire, err := json.Marshal(output)
	require.NoError(t, err)
	require.Contains(t, string(wire), `"content":"{\"root\":true}"`)
	require.Contains(t, string(wire), `"content":"{\"part\":true}"`)
	require.Contains(t, string(wire), `"counter":9007199254740993`)
	require.NotContains(t, string(wire), `"counter":9007199254740992`)
	require.NotContains(t, string(wire), `"counter":"9007199254740993"`)
	require.NotContains(t, string(wire), "eyJyb290Ijp0cnVlfQ==")
	require.NotContains(t, string(wire), "eyJwYXJ0Ijp0cnVlfQ==")
}
