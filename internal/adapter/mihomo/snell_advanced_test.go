package mihomo_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestMihomoSnellAdvancedRoundTrip(t *testing.T) {
	t.Parallel()

	nodes, _, err := mihomo.NewParser().Parse(context.Background(), []byte(`
proxies:
  - name: snell
    type: snell
    server: snell.example.com
    port: 44046
    psk: secret
    version: 5
    reuse: true
    client-fingerprint: chrome
    obfs-opts:
      mode: shadow-tls
      password: shadow-secret
      host: cdn.example.com
      version: 3
      alpn: [h2, http/1.1]
      fingerprint: sha256:abcd
      certificate: cert-pem
      private-key: key-pem
      skip-cert-verify: true
`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, &domain.SnellOptions{
		Version:           5,
		Reuse:             boolPointer(true),
		ClientFingerprint: "chrome",
		ShadowTLS: &domain.ShadowTLSOptions{
			Password: "shadow-secret", Host: "cdn.example.com", Version: 3,
			ALPN: []string{"h2", "http/1.1"}, Fingerprint: "sha256:abcd",
			Certificate: "cert-pem", PrivateKey: "key-pem", InsecureSkipVerify: true,
		},
	}, nodes[0].Snell)

	rendered, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	body := string(rendered)
	for _, expected := range []string{
		"version: 5", "reuse: true", "client-fingerprint: chrome", "mode: shadow-tls",
		"password: shadow-secret", "version: 3", "certificate: cert-pem", "private-key: key-pem",
	} {
		require.True(t, strings.Contains(body, expected), body)
	}
}

func boolPointer(value bool) *bool { return &value }
