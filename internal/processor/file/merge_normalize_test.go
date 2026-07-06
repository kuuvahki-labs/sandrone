package file

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeMapInterfaceKeys(t *testing.T) {
	in := map[any]any{
		"outer": map[any]any{"inner": []any{1, map[any]any{"k": "v"}}},
	}
	out := normalizeMap(in).(map[string]any)
	inner := out["outer"].(map[string]any)["inner"].([]any)
	require.Equal(t, 1, inner[0])
	require.Equal(t, "v", inner[1].(map[string]any)["k"])
}

func TestOverlayValueDeletesNil(t *testing.T) {
	prev := map[string]any{"keep": 1, "drop": 2, "nested": map[string]any{"a": 1}}
	next := map[string]any{"drop": nil, "nested": map[string]any{"b": 2}}
	out := overlayValue(prev, next).(map[string]any)
	require.Equal(t, 1, out["keep"])
	_, hasDrop := out["drop"]
	require.False(t, hasDrop)
	nested := out["nested"].(map[string]any)
	require.Equal(t, 1, nested["a"])
	require.Equal(t, 2, nested["b"])
}
