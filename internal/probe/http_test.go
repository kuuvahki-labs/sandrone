package probe

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestNodeHTTPExplicitUnavailableCoreDoesNotFallback(t *testing.T) {
	require.False(t, NodeHTTPAvailable("unknown"))
	require.Equal(t, NodeHTTPAvailable("sing-box"), NodeHTTPAvailable(""))
	for _, core := range []string{"unknown", "sing-box", "mihomo"} {
		if NodeHTTPAvailable(core) {
			continue
		}
		_, err := RequestNodeHTTP(context.Background(), nil, domain.NodeHTTPOptions{Core: core}, Payload{})
		require.True(t, domain.IsCode(err, domain.CodeProbeBackendUnavailable), "%s: %v", core, err)
	}
}
