package domain

import (
	"encoding/json/v2"
	"errors"
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
			require.ErrorIs(t, err, json.ErrUnknownName)
			semantic, ok := errors.AsType[*json.SemanticError](err)
			require.True(t, ok)
			require.Equal(t, "/"+field, string(semantic.JSONPointer))
		})
	}
}

func TestFileConfigJSONReportsUnknownFieldPointer(t *testing.T) {
	var config FileConfig
	err := json.Unmarshal([]byte(`{"z_field":true,"a_field":true}`), &config)
	require.ErrorIs(t, err, json.ErrUnknownName)
	semantic, ok := errors.AsType[*json.SemanticError](err)
	require.True(t, ok)
	require.Equal(t, "/z_field", string(semantic.JSONPointer))
}
