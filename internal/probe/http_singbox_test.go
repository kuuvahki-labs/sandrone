//go:build probe_singbox

package probe

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestNodeHTTPStartFailure(t *testing.T) {
	_, err := RequestNodeHTTP(context.Background(), []domain.NodeIR{{Name: "node"}}, domain.NodeHTTPOptions{URL: "https://example.com/", Method: "GET", TimeoutMS: 5000, Concurrency: 4, MaxResponseBytes: 65536}, Payload{Body: []byte(`{"outbounds":[{"type":"http","tag":"node","server":"127.0.0.1","server_port":80,"detour":"missing"}]}`)})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeCoreStartFailed), "%v", err)
}
