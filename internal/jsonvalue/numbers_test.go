package jsonvalue_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/jsonvalue"
)

func TestPreserveNumbersRoundTripsNestedProviderValues(t *testing.T) {
	const input = `{"integer":18446744073709551615,"nested":[9007199254740993,1.234567890123456789,1e400],"text":"42","flag":true,"null":null}`
	var values map[string]any
	require.NoError(t, json.Unmarshal([]byte(input), &values, jsonvalue.PreserveNumbers))
	require.Equal(t, jsontext.Value("18446744073709551615"), values["integer"])
	require.Equal(t, []any{jsontext.Value("9007199254740993"), jsontext.Value("1.234567890123456789"), jsontext.Value("1e400")}, values["nested"])
	require.Equal(t, "42", values["text"])
	require.Equal(t, true, values["flag"])
	require.Nil(t, values["null"])
	body, err := json.Marshal(values, json.Deterministic(true))
	require.NoError(t, err)
	require.Equal(t, `{"flag":true,"integer":18446744073709551615,"nested":[9007199254740993,1.234567890123456789,1e400],"null":null,"text":"42"}`, string(body))
}

func TestPreserveNumbersKeepsNativeJSONValidation(t *testing.T) {
	for _, input := range []string{`{"n":1,"n":2}`, `{"n":01}`, `{"n":1} {}`, "{\"s\":\"\xff\"}"} {
		t.Run(input, func(t *testing.T) {
			var value any
			require.Error(t, json.Unmarshal([]byte(input), &value, jsonvalue.PreserveNumbers))
		})
	}
}
