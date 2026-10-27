//go:build probe_singbox && with_wireguard

package service

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestNodeHTTPWireGuardDoesNotPreventHealthyProxyRequest(t *testing.T) {
	proxy, count := nodeHTTPTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	wg := domain.NodeIR{Name: "wg", Type: domain.NodeTypeWireGuard, Server: "127.0.0.1", Port: 9,
		WireGuard: &domain.WireGuardOptions{
			PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", Address: []string{"10.0.0.2/32"},
			Peers: []domain.WireGuardPeer{{Server: "127.0.0.1", Port: 9,
				PublicKey: "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=", AllowedIPs: []string{"0.0.0.0/0"}}},
		},
	}
	results, err := New().RequestNodeHTTP(context.Background(), []domain.NodeIR{wg, proxy}, domain.NodeHTTPOptions{
		Core: "sing-box", URL: "http://192.0.2.1/", TimeoutMS: 100,
	})
	require.NoError(t, err, "WireGuard must initialize even when its peer cannot respond")
	require.Len(t, results, 2)
	require.NotEmpty(t, results[0].Error)
	require.Equal(t, "ok", results[1].Body)
	require.EqualValues(t, 1, count.Load())
}
