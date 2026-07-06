package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileConfigJSONAcceptsOnlyOrchestrationEnvelope(t *testing.T) {
	var config FileConfig
	require.NoError(t, json.Unmarshal([]byte(`{
		"subscriptions":["primary"],
		"settings":{"groups":[],"rules":[]}
	}`), &config))
	require.Equal(t, []string{"primary"}, config.Subscriptions)
	require.JSONEq(t, `{"groups":[],"rules":[]}`, string(config.Settings))
}

func TestFileConfigJSONRejectsLegacyCompilerFields(t *testing.T) {
	for _, field := range []string{"groups", "rule_sets", "rules", "group_preset", "ruleset_preset"} {
		t.Run(field, func(t *testing.T) {
			var config FileConfig
			err := json.Unmarshal([]byte(`{"`+field+`":[]}`), &config)
			require.ErrorContains(t, err, `config.`+field)
		})
	}
}

func TestFileConfigJSONReportsUnknownFieldsDeterministically(t *testing.T) {
	var config FileConfig
	err := json.Unmarshal([]byte(`{"z_field":true,"a_field":true}`), &config)
	require.EqualError(t, err, "config.a_field: unknown field")
}
