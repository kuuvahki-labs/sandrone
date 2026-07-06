package httpapi_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
)

func testRuntime(t *testing.T, cfg app.Config) *app.Runtime {
	t.Helper()
	cfg.DataDir = t.TempDir()
	rt, err := app.NewRuntime(cfg, nil)
	require.NoError(t, err)
	return rt
}

func params(t *testing.T, m map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	for key, value := range m {
		body, err := json.Marshal(value)
		require.NoError(t, err)
		out[key] = body
	}
	return out
}

func inlineScriptSource(content string) map[string]any {
	return map[string]any{"type": "inline", "content": content}
}
