package mihomo_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestMihomoAnyTLSRoundTrip(t *testing.T) {
	t.Parallel()

	parser := mihomo.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: anytls
    type: anytls
    server: anytls.example.com
    port: 443
    password: secret
    idle-session-check-interval: 30
    idle-session-timeout: 300
    min-idle-session: 2
    sni: sni.example.com
    skip-cert-verify: true
`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeAnyTLS, nodes[0].Type)
	require.Equal(t, &domain.AnyTLSOptions{
		IdleSessionCheckInterval: "30s",
		IdleSessionTimeout:       "5m0s",
		MinIdleSession:           2,
	}, nodes[0].AnyTLS)
	require.NotNil(t, nodes[0].TLS)
	require.True(t, nodes[0].TLS.Enabled)

	rendered, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	body := string(rendered)
	for _, expected := range []string{
		"type: anytls",
		"password: secret",
		"idle-session-check-interval: 30",
		"idle-session-timeout: 300",
		"min-idle-session: 2",
	} {
		require.True(t, strings.Contains(body, expected), body)
	}
}
