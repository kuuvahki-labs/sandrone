package shared_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestRawWarningsSortsSkipsAndUsesTargetMessage(t *testing.T) {
	node := domain.NodeIR{
		Name: "node-a",
		Raw: map[string]json.RawMessage{
			"z":    json.RawMessage(`1`),
			"skip": json.RawMessage(`2`),
			"a":    json.RawMessage(`3`),
		},
	}

	warnings := shared.RawWarnings(node, map[string]bool{"skip": true}, "uri-list")

	require.Equal(t, []domain.Warning{
		{
			Code:    "render_lossy_field",
			Message: "field preserved in NodeIR Raw but not emitted by uri-list renderer",
			Node:    "node-a",
			Field:   "a",
			Target:  "uri-list",
		},
		{
			Code:    "render_lossy_field",
			Message: "field preserved in NodeIR Raw but not emitted by uri-list renderer",
			Node:    "node-a",
			Field:   "z",
			Target:  "uri-list",
		},
	}, warnings)

	mihomoWarnings := shared.RawWarnings(node, nil, "mihomo-proxies")
	require.Len(t, mihomoWarnings, 3)
	require.Equal(t, "field preserved in NodeIR Raw but not emitted by mihomo renderer", mihomoWarnings[0].Message)
	require.Nil(t, shared.RawWarnings(domain.NodeIR{}, nil, "uri-list"))
}

func TestMarshalStableJSONAndMergeWarnings(t *testing.T) {
	body, err := shared.MarshalStableJSON(map[string]any{"b": 2, "a": 1}, true)
	require.NoError(t, err)
	require.Equal(t, "{\n  \"a\": 1,\n  \"b\": 2\n}", string(body))

	body, err = shared.MarshalStableJSON(map[string]any{"a": 1}, false)
	require.NoError(t, err)
	require.Equal(t, `{"a":1}`, string(body))

	report := domain.RenderReport{Warnings: []domain.Warning{{Code: "existing"}}, LostFields: 1}
	shared.MergeWarnings(&report, []domain.Warning{{Code: "new"}, {Code: "new2"}})
	require.Equal(t, 3, report.LostFields)
	require.Equal(t, []domain.Warning{{Code: "existing"}, {Code: "new"}, {Code: "new2"}}, report.Warnings)
}
