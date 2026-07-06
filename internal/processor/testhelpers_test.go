package processor_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func rawParams(t *testing.T, m map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	for k, v := range m {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		out[k] = b
	}
	return out
}
