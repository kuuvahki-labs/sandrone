package domain_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestWarningsWithValidUTF8DoesNotMutateSource(t *testing.T) {
	original := []domain.Warning{{Code: "bad\xff", Message: "bad\xff", Node: "bad\xff", Field: "bad\xff", Source: "bad\xff", Target: "bad\xff",
		NodeContext: &domain.WarningNodeContext{Format: "bad\xff", Name: "bad\xff", Type: "bad\xff", Server: "bad\xff", RawLine: "bad\xff", Raw: map[string]any{"field": "bad\xff"}},
	}}
	clean := domain.WarningsWithValidUTF8(original)
	_, err := json.Marshal(clean)
	require.NoError(t, err)
	require.Nil(t, clean[0].NodeContext.Raw)
	require.Equal(t, "bad\xff", original[0].Node)
	require.Equal(t, "bad\xff", original[0].NodeContext.Name)
	require.Equal(t, "bad\xff", original[0].NodeContext.Raw["field"])
}
