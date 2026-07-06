package mihomo_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestParseMihomoMieruPromotesOfficialFields(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: mieru
    type: mieru
    server: mieru.example.com
    port: 34567
    transport: TCP
    udp: true
    username: user
    password: pass
    multiplexing: MULTIPLEXING_LOW
    handshake-mode: HANDSHAKE_STANDARD
    traffic-pattern: pattern
    private-thing: value
`))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeMieru, got.Type)
	require.Equal(t, "mieru.example.com", got.Server)
	require.Equal(t, uint16(34567), got.Port)
	require.Equal(t, "user", got.Username)
	require.Equal(t, "pass", got.Password)
	require.NotNil(t, got.Dialer)
	require.NotNil(t, got.Dialer.UDPRelay)
	require.True(t, *got.Dialer.UDPRelay)
	require.NotNil(t, got.Mieru)
	require.Equal(t, "TCP", got.Mieru.Transport)
	require.Equal(t, "MULTIPLEXING_LOW", got.Mieru.Multiplexing)
	require.Equal(t, "HANDSHAKE_STANDARD", got.Mieru.HandshakeMode)
	require.Equal(t, "pattern", got.Mieru.TrafficPattern)
	require.JSONEq(t, `"value"`, string(got.Raw["mihomo.private-thing"]))
	require.Len(t, source.Warnings, 1)
	require.Equal(t, "mihomo.private-thing", source.Warnings[0].Field)
}

func TestParseMihomoMieruPortRange(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: mieru-range
    type: mieru
    server: mieru.example.com
    port-range: 9998-9999
    transport: UDP
    username: user
    password: pass
`))

	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeMieru, got.Type)
	require.Zero(t, got.Port)
	require.NotNil(t, got.Mieru)
	require.Equal(t, "9998-9999", got.Mieru.PortRange)
	require.Equal(t, "UDP", got.Mieru.Transport)
}

func TestRenderMihomoMieruPortAndPortRange(t *testing.T) {
	r := mihomo.NewRenderer()
	udp := true
	out, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{
		{
			Name:     "mieru-port",
			Type:     domain.NodeTypeMieru,
			Server:   "mieru.example.com",
			Port:     34567,
			Username: "user",
			Password: "pass",
			Dialer:   &domain.DialerOptions{UDPRelay: &udp},
			Mieru: &domain.MieruOptions{
				Transport:      "TCP",
				Multiplexing:   "MULTIPLEXING_LOW",
				HandshakeMode:  "HANDSHAKE_STANDARD",
				TrafficPattern: "pattern",
			},
		},
		{
			Name:     "mieru-range",
			Type:     domain.NodeTypeMieru,
			Server:   "mieru.example.com",
			Username: "user",
			Password: "pass",
			Mieru: &domain.MieruOptions{
				PortRange: "9998-9999",
				Transport: "UDP",
			},
		},
	}, domain.RenderOptions{Format: "mihomo-proxies"})

	require.NoError(t, err)
	require.Equal(t, 2, report.SuccessCount)
	require.Empty(t, report.Warnings)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out, &doc))
	proxies := doc["proxies"].([]any)
	first := proxies[0].(map[string]any)
	require.Equal(t, "mieru", first["type"])
	require.Equal(t, 34567, first["port"])
	require.NotContains(t, first, "port-range")
	require.Equal(t, "TCP", first["transport"])
	require.Equal(t, true, first["udp"])
	require.Equal(t, "MULTIPLEXING_LOW", first["multiplexing"])
	require.Equal(t, "HANDSHAKE_STANDARD", first["handshake-mode"])
	require.Equal(t, "pattern", first["traffic-pattern"])

	second := proxies[1].(map[string]any)
	require.Equal(t, "mieru", second["type"])
	require.NotContains(t, second, "port")
	require.Equal(t, "9998-9999", second["port-range"])
	require.Equal(t, "UDP", second["transport"])

	body := string(out)
	for _, earlierLater := range [][2]string{
		{"type: mieru", "server: mieru.example.com"},
		{"server: mieru.example.com", "port: 34567"},
		{"port: 34567", "transport: TCP"},
		{"transport: TCP", "username: user"},
		{"username: user", "password: pass"},
		{"password: pass", "udp: true"},
		{"udp: true", "multiplexing: MULTIPLEXING_LOW"},
		{"multiplexing: MULTIPLEXING_LOW", "handshake-mode: HANDSHAKE_STANDARD"},
		{"handshake-mode: HANDSHAKE_STANDARD", "traffic-pattern: pattern"},
	} {
		require.Less(t, strings.Index(body, earlierLater[0]), strings.Index(body, earlierLater[1]), body)
	}
}
