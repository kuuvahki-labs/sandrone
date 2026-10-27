//go:build !probe_singbox

package probe

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestNodeHTTPUnavailableBuild(t *testing.T) {
	_, err := RequestNodeHTTP(context.Background(), nil, domain.NodeHTTPOptions{}, Payload{})
	require.True(t, domain.IsCode(err, domain.CodeProbeBackendUnavailable))
}
