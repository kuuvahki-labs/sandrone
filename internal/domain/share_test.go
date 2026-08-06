package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestShareJSONOmitsUnsetOptionalTimes(t *testing.T) {
	tests := []struct {
		name  string
		value any
		keys  []string
	}{
		{
			name: "share",
			value: domain.Share{
				ID: "long-lived", TargetKind: "file", TargetName: "client",
			},
			keys: []string{"created_at", "updated_at", "valid_from", "valid_until"},
		},
		{
			name: "create request",
			value: domain.ShareCreateRequest{
				TargetKind: "file", TargetName: "client",
			},
			keys: []string{"valid_from", "valid_until"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(test.value)
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(body, &fields))
			for _, key := range test.keys {
				require.NotContains(t, fields, key)
			}
		})
	}
}

func TestShareJSONKeepsSetOptionalTimes(t *testing.T) {
	validUntil := time.Date(2026, 7, 19, 12, 30, 0, 0, time.UTC)
	body, err := json.Marshal(domain.Share{
		ID: "expiring", TargetKind: "file", TargetName: "client", ValidUntil: validUntil,
	})
	require.NoError(t, err)

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	require.JSONEq(t, `"2026-07-19T12:30:00Z"`, string(fields["valid_until"]))
}
