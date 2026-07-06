package singbox_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestSingBoxAnyTLSRoundTrip(t *testing.T) {
	t.Parallel()

	parser := singbox.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`{
  "outbounds": [{
    "type": "anytls",
    "tag": "anytls",
    "server": "anytls.example.com",
    "server_port": 443,
    "password": "secret",
    "idle_session_check_interval": "30s",
    "idle_session_timeout": "5m",
    "min_idle_session": 2,
    "tls": {"enabled": true, "server_name": "sni.example.com"}
  }]
}`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeAnyTLS, nodes[0].Type)
	require.Equal(t, &domain.AnyTLSOptions{
		IdleSessionCheckInterval: "30s",
		IdleSessionTimeout:       "5m",
		MinIdleSession:           2,
	}, nodes[0].AnyTLS)

	rendered, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.JSONEq(t, `{
  "outbounds": [{
    "type": "anytls",
    "tag": "anytls",
    "server": "anytls.example.com",
    "server_port": 443,
    "password": "secret",
    "idle_session_check_interval": "30s",
    "idle_session_timeout": "5m",
    "min_idle_session": 2,
    "tls": {"enabled": true, "server_name": "sni.example.com"}
  }]
}`, string(rendered))
}
