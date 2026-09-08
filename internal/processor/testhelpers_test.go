package processor_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

func rawParams(t *testing.T, m map[string]any) map[string]jsontext.Value {
	t.Helper()
	out := map[string]jsontext.Value{}
	for k, v := range m {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		out[k] = b
	}
	return out
}
