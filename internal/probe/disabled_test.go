package probe

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestDisabledEngineHasNoBackendsOrCore(t *testing.T) {
	engine := NewDisabled()
	require.False(t, engine.ProbeAvailable())
	require.Empty(t, engine.BackendSummary())
	core, ok := engine.SelectCore(domain.ProbeRequest{}, nil)
	require.False(t, ok)
	require.Empty(t, core)
}

func TestDisabledEngineReturnsBackendUnavailable(t *testing.T) {
	_, err := NewDisabled().Probe(context.Background(), domain.ProbeRequest{}, nil)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeBackendUnavailable))
}
