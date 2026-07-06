package uri_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	uriadapter "github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestAnyTLSURIRoundTrip(t *testing.T) {
	t.Parallel()

	raw := "anytls://p%40ss@anytls.example.com?security=tls&sni=sni.example.com&idle-session-check-interval=30s&idle-session-timeout=5m&min-idle-session=2#anytls"
	nodes, _, err := uriadapter.NewParser().Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	node := nodes[0]
	require.Equal(t, domain.NodeTypeAnyTLS, node.Type)
	require.Equal(t, "p@ss", node.Password)
	require.Equal(t, uint16(443), node.Port)
	require.Equal(t, &domain.AnyTLSOptions{
		IdleSessionCheckInterval: "30s",
		IdleSessionTimeout:       "5m",
		MinIdleSession:           2,
	}, node.AnyTLS)

	rendered, report, err := uriadapter.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	parsed, err := url.Parse(string(rendered))
	require.NoError(t, err)
	require.Equal(t, "anytls", parsed.Scheme)
	require.Equal(t, "p@ss", parsed.User.Username())
	require.Equal(t, "anytls.example.com:443", parsed.Host)
	require.Equal(t, "30s", parsed.Query().Get("idle-session-check-interval"))
}
