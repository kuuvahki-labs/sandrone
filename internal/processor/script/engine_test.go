package script

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeForJS(t *testing.T) {
	in := map[any]any{
		"nested": map[any]any{"k": []any{1, 2}},
		"list":   []any{map[any]any{"x": 1}},
	}
	out := normalizeForJS(in).(map[string]any)
	require.Equal(t, 1, out["nested"].(map[string]any)["k"].([]any)[0])
	require.Equal(t, 1, out["list"].([]any)[0].(map[string]any)["x"])
}
