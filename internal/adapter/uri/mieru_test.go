package uri_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestParseMieruURISingleNode(t *testing.T) {
	p := uri.NewParser()
	nodes, source, err := p.ParseList(context.Background(), []byte("mierus://user:pass@example.com?port=443&protocol=TCP&profile=simple"))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeMieru, got.Type)
	require.Equal(t, "simple:443/TCP", got.Name)
	require.Equal(t, "example.com", got.Server)
	require.Equal(t, uint16(443), got.Port)
	require.Equal(t, "user", got.Username)
	require.Equal(t, "pass", got.Password)
	require.NotNil(t, got.Dialer)
	require.NotNil(t, got.Dialer.UDPRelay)
	require.True(t, *got.Dialer.UDPRelay)
	require.NotNil(t, got.Mieru)
	require.Equal(t, "TCP", got.Mieru.Transport)
}

func TestParseMieruURIMultiplePortProtocolPairs(t *testing.T) {
	p := uri.NewParser()
	raw := "mierus://user:pass@1.2.3.4?handshake-mode=HANDSHAKE_NO_WAIT&multiplexing=MULTIPLEXING_HIGH&port=6666&port=9998-9999&port=6489&port=4896&profile=default&protocol=TCP&protocol=TCP&protocol=UDP&protocol=UDP&traffic-pattern=pattern"

	nodes, _, err := p.ParseList(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 4)
	require.Equal(t, "default:6666/TCP", nodes[0].Name)
	require.Equal(t, uint16(6666), nodes[0].Port)
	require.Equal(t, "TCP", nodes[0].Mieru.Transport)
	require.Equal(t, "default:9998-9999/TCP", nodes[1].Name)
	require.Zero(t, nodes[1].Port)
	require.Equal(t, "9998-9999", nodes[1].Mieru.PortRange)
	require.Equal(t, "TCP", nodes[1].Mieru.Transport)
	require.Equal(t, "default:6489/UDP", nodes[2].Name)
	require.Equal(t, uint16(6489), nodes[2].Port)
	require.Equal(t, "UDP", nodes[2].Mieru.Transport)
	require.Equal(t, "default:4896/UDP", nodes[3].Name)
	require.Equal(t, uint16(4896), nodes[3].Port)
	require.Equal(t, "UDP", nodes[3].Mieru.Transport)
	for _, node := range nodes {
		require.Equal(t, "MULTIPLEXING_HIGH", node.Mieru.Multiplexing)
		require.Equal(t, "HANDSHAKE_NO_WAIT", node.Mieru.HandshakeMode)
		require.Equal(t, "pattern", node.Mieru.TrafficPattern)
	}
}

func TestParseMieruURIFragmentOverridesProfile(t *testing.T) {
	p := uri.NewParser()
	nodes, _, err := p.ParseList(context.Background(), []byte("mierus://user:pass@example.com?port=443&protocol=TCP&profile=default#myproxy"))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "myproxy:443/TCP", nodes[0].Name)
}

func TestParseMieruURIPortProtocolMismatchFails(t *testing.T) {
	p := uri.NewParser()
	_, _, err := p.ParseList(context.Background(), []byte("mierus://user:pass@example.com?port=443&port=8443&protocol=TCP&profile=bad"))

	require.ErrorContains(t, err, "mieru port and protocol counts must match")
}

func TestRenderMieruURIRoundTrip(t *testing.T) {
	p := uri.NewParser()
	parsed, _, err := p.ParseList(context.Background(), []byte("mierus://user:pass@example.com?port=443&protocol=TCP&profile=simple"))
	require.NoError(t, err)

	r := uri.NewRenderer()
	out, report, err := r.RenderWithReport(context.Background(), parsed, domain.RenderOptions{Format: "uri-list"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Empty(t, report.Warnings)
	require.True(t, strings.HasPrefix(string(out), "mierus://user:pass@example.com?"))
	require.Contains(t, string(out), "port=443")
	require.Contains(t, string(out), "profile=simple")
	require.Contains(t, string(out), "protocol=TCP")

	roundTripped, _, err := p.ParseList(context.Background(), out)
	require.NoError(t, err)
	require.Equal(t, parsed, roundTripped)
}

func TestRenderMieruURIPortRangeRoundTrip(t *testing.T) {
	p := uri.NewParser()
	parsed, _, err := p.ParseList(context.Background(), []byte("mierus://user:pass@example.com?port=9998-9999&protocol=UDP&profile=range"))
	require.NoError(t, err)

	r := uri.NewRenderer()
	out, _, err := r.RenderWithReport(context.Background(), parsed, domain.RenderOptions{Format: "uri-list"})

	require.NoError(t, err)
	require.Contains(t, string(out), "port=9998-9999")
	require.Contains(t, string(out), "protocol=UDP")
	roundTripped, _, err := p.ParseList(context.Background(), out)
	require.NoError(t, err)
	require.Equal(t, parsed, roundTripped)
}

func TestRenderMieruURIRequiresRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		node domain.NodeIR
	}{
		{
			name: "server",
			node: domain.NodeIR{Type: domain.NodeTypeMieru, Username: "user", Password: "pass", Port: 443, Mieru: &domain.MieruOptions{Transport: "TCP"}},
		},
		{
			name: "username",
			node: domain.NodeIR{Type: domain.NodeTypeMieru, Server: "example.com", Password: "pass", Port: 443, Mieru: &domain.MieruOptions{Transport: "TCP"}},
		},
		{
			name: "password",
			node: domain.NodeIR{Type: domain.NodeTypeMieru, Server: "example.com", Username: "user", Port: 443, Mieru: &domain.MieruOptions{Transport: "TCP"}},
		},
		{
			name: "transport",
			node: domain.NodeIR{Type: domain.NodeTypeMieru, Server: "example.com", Username: "user", Password: "pass", Port: 443, Mieru: &domain.MieruOptions{}},
		},
		{
			name: "port-or-range",
			node: domain.NodeIR{Type: domain.NodeTypeMieru, Server: "example.com", Username: "user", Password: "pass", Mieru: &domain.MieruOptions{Transport: "TCP"}},
		},
	}

	r := uri.NewRenderer()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := r.RenderWithReport(context.Background(), []domain.NodeIR{tc.node}, domain.RenderOptions{Format: "uri-list"})
			require.ErrorContains(t, err, "missing mieru URI fields")
		})
	}
}
