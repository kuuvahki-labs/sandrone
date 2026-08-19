package uri_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/jsonnodes"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	uriadapter "github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestParseURIHysteriaMPortAlias(t *testing.T) {
	t.Run("hysteria", func(t *testing.T) {
		node := parseAliasNode(t, "hy://example.com:443?mport=8443-8450%3B9443#hy")

		require.Equal(t, []string{"8443-8450", "9443"}, node.Hysteria.ServerPorts)
		require.NotContains(t, node.Raw, "uri.query.mport")
	})

	t.Run("hysteria2 and renderers", func(t *testing.T) {
		node := parseAliasNode(t, "hy2://secret@example.com:443?mport=8443-8450,9443#hy2")

		require.Equal(t, []string{"8443-8450", "9443"}, node.Hysteria.ServerPorts)
		require.NotContains(t, node.Raw, "uri.query.mport")

		mihomoBody, mihomoReport, err := mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
		require.NoError(t, err)
		require.Empty(t, mihomoReport.Warnings)
		require.Contains(t, string(mihomoBody), "ports: 8443-8450,9443")

		singBoxBody, singBoxReport, err := singbox.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
		require.NoError(t, err)
		require.Empty(t, singBoxReport.Warnings)
		require.Contains(t, string(singBoxBody), `"server_ports": [`)
		require.Contains(t, string(singBoxBody), `"8443-8450"`)
	})

	t.Run("authority wins conflict", func(t *testing.T) {
		node := parseAliasNode(t, "hy2://secret@example.com:443,444?mport=555-556#hy2")

		require.Equal(t, []string{"443", "444"}, node.Hysteria.ServerPorts)
		require.JSONEq(t, `"555-556"`, string(node.Raw["uri.query.mport"]))
	})

	t.Run("invalid remains raw", func(t *testing.T) {
		node := parseAliasNode(t, "hy2://secret@example.com:443?mport=9000-8000#hy2")

		require.Empty(t, node.Hysteria.ServerPorts)
		require.JSONEq(t, `"9000-8000"`, string(node.Raw["uri.query.mport"]))
	})
}

func TestParseURITUICHyphenAliases(t *testing.T) {
	node := parseAliasNode(t, "tuic://11111111-1111-1111-1111-111111111111:secret@example.com:443?congestion-control=bbr&reduce-rtt=1&disable-sni=true#tuic")

	require.Equal(t, "bbr", node.TUIC.CongestionControl)
	require.True(t, node.TUIC.ReduceRTT)
	require.True(t, node.TLS.DisableSNI)
	for _, key := range []string{"congestion-control", "reduce-rtt", "disable-sni"} {
		require.NotContains(t, node.Raw, "uri.query."+key)
	}

	mihomoBody, _, err := mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Contains(t, string(mihomoBody), "congestion-controller: bbr")
	require.Contains(t, string(mihomoBody), "reduce-rtt: true")

	singBoxBody, singBoxReport, err := singbox.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Contains(t, string(singBoxBody), `"congestion_control": "bbr"`)
	require.Contains(t, string(singBoxBody), `"disable_sni": true`)
	require.Contains(t, warningFields(singBoxReport.Warnings), "tuic.reduce_rtt")
}

func TestParseURITUICCanonicalSpellingWinsAliasConflict(t *testing.T) {
	node := parseAliasNode(t, "tuic://11111111-1111-1111-1111-111111111111:secret@example.com:443?congestion_control=cubic&congestion-control=bbr&reduce_rtt=false&reduce-rtt=true&disable_sni=0&disable-sni=1#tuic")

	require.Equal(t, "cubic", node.TUIC.CongestionControl)
	require.False(t, node.TUIC.ReduceRTT)
	require.False(t, node.TLS.DisableSNI)
	for key, value := range map[string]string{
		"congestion-control": `"bbr"`,
		"reduce-rtt":         `"true"`,
		"disable-sni":        `"1"`,
	} {
		require.JSONEq(t, value, string(node.Raw["uri.query."+key]))
	}
	for _, key := range []string{"congestion_control", "reduce_rtt", "disable_sni"} {
		require.NotContains(t, node.Raw, "uri.query."+key)
	}
}

func TestParseURIShadowsocksUOTAndTFOAliases(t *testing.T) {
	node := parseAliasNode(t, "ss://aes-128-gcm:secret@example.com:8388?uot=1&tfo=true#ss")

	require.NotNil(t, node.UDPOverTCP)
	require.True(t, node.UDPOverTCP.Enabled)
	require.NotNil(t, node.Dialer)
	require.True(t, node.Dialer.TFO)
	require.NotContains(t, node.Raw, "uri.query.uot")
	require.NotContains(t, node.Raw, "uri.query.tfo")

	mihomoBody, mihomoReport, err := mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Empty(t, mihomoReport.Warnings)
	require.Contains(t, string(mihomoBody), "udp-over-tcp: true")
	require.Contains(t, string(mihomoBody), "tfo: true")

	singBoxBody, singBoxReport, err := singbox.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Empty(t, singBoxReport.Warnings)
	require.Contains(t, string(singBoxBody), `"udp_over_tcp": true`)
	require.Contains(t, string(singBoxBody), `"tcp_fast_open": true`)
}

func TestParseURIAnyTLSUDPRelayAlias(t *testing.T) {
	t.Run("recognized", func(t *testing.T) {
		node := parseAliasNode(t, "anytls://secret@example.com:443?udp=1#anytls")

		require.NotNil(t, node.Dialer)
		require.NotNil(t, node.Dialer.UDPRelay)
		require.True(t, *node.Dialer.UDPRelay)
		require.NotContains(t, node.Raw, "uri.query.udp")

		body, report, err := jsonnodes.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
		require.NoError(t, err)
		require.Empty(t, report.Warnings)
		var rendered []domain.NodeIR
		require.NoError(t, json.Unmarshal(body, &rendered))
		require.True(t, *rendered[0].Dialer.UDPRelay)

		_, mihomoReport, err := mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
		require.NoError(t, err)
		require.Contains(t, warningFields(mihomoReport.Warnings), "dialer.udp_relay")
	})

	t.Run("unknown remains raw", func(t *testing.T) {
		node := parseAliasNode(t, "anytls://secret@example.com:443?udp=maybe#anytls")

		require.Nil(t, node.Dialer)
		require.JSONEq(t, `"maybe"`, string(node.Raw["uri.query.udp"]))
	})
}

func parseAliasNode(t *testing.T, raw string) domain.NodeIR {
	t.Helper()
	nodes, _, err := uriadapter.NewParser().Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	return nodes[0]
}
