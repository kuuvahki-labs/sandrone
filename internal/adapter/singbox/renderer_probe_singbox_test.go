//go:build probe_singbox

package singbox_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	uriadapter "github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestRenderSingBoxHysteriaMbpsBandwidthIsAcceptedByLockedCore(t *testing.T) {
	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{{
		Name: "hy", Type: domain.NodeTypeHysteria, Server: "example.com", Port: 8443,
		TLS:      &domain.TLSOptions{Enabled: true},
		Hysteria: &domain.HysteriaOptions{AuthString: "secret", UpMbps: 55, DownMbps: 100},
	}}, domain.RenderOptions{Format: "sing-box-outbounds"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	outbound := decodeSingBoxHysteriaOutbound(t, out)
	require.Equal(t, float64(55), outbound["up_mbps"])
	require.Equal(t, float64(100), outbound["down_mbps"])
	require.NotContains(t, outbound, "up")
	require.NotContains(t, outbound, "down")
	requireLockedSingBoxOptions(t, out)
}

func TestRenderSingBoxHysteriaExplicitBandwidthIsAcceptedByLockedCore(t *testing.T) {
	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{{
		Name: "hy", Type: domain.NodeTypeHysteria, Server: "example.com", Port: 8443,
		TLS:      &domain.TLSOptions{Enabled: true},
		Hysteria: &domain.HysteriaOptions{AuthString: "secret", Up: "55 Bps", Down: "640 KBps"},
	}}, domain.RenderOptions{Format: "sing-box-outbounds"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	outbound := decodeSingBoxHysteriaOutbound(t, out)
	require.Equal(t, "55 Bps", outbound["up"])
	require.Equal(t, "640 KBps", outbound["down"])
	require.NotContains(t, outbound, "up_mbps")
	require.NotContains(t, outbound, "down_mbps")
	requireLockedSingBoxOptions(t, out)
}

func TestRenderSingBoxHysteriaCanonicalBandwidthMatrixIsAcceptedByLockedCore(t *testing.T) {
	tests := []struct {
		name        string
		hysteria    domain.HysteriaOptions
		wantUp      any
		wantDown    any
		wantUpBps   uint64
		wantDownBps uint64
	}{
		{
			name:     "lowercase bps converts losslessly to bytes",
			hysteria: domain.HysteriaOptions{Up: "56 bps", Down: "80 bps"},
			wantUp:   "7 Bps", wantDown: "10 Bps", wantUpBps: 7, wantDownBps: 10,
		},
		{
			name:     "explicit bytes per second",
			hysteria: domain.HysteriaOptions{Up: "55 Bps", Down: "100 Bps"},
			wantUp:   "55 Bps", wantDown: "100 Bps", wantUpBps: 55, wantDownBps: 100,
		},
		{
			name:     "explicit kilobits per second",
			hysteria: domain.HysteriaOptions{Up: "55 Kbps", Down: "100 Kbps"},
			wantUp:   "55 Kbps", wantDown: "100 Kbps", wantUpBps: 6_875, wantDownBps: 12_500,
		},
		{
			name:     "explicit kilobytes per second",
			hysteria: domain.HysteriaOptions{Up: "55 KBps", Down: "100 KBps"},
			wantUp:   "55 KBps", wantDown: "100 KBps", wantUpBps: 55_000, wantDownBps: 100_000,
		},
		{
			name:     "integer Mbps",
			hysteria: domain.HysteriaOptions{UpMbps: 55, DownMbps: 100},
			wantUp:   float64(55), wantDown: float64(100), wantUpBps: 6_875_000, wantDownBps: 12_500_000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{{
				Name: "hy", Type: domain.NodeTypeHysteria, Server: "example.com", Port: 8443,
				TLS:      &domain.TLSOptions{Enabled: true},
				Hysteria: &test.hysteria,
			}}, domain.RenderOptions{Format: "sing-box-outbounds"})

			require.NoError(t, err)
			require.Equal(t, 1, report.SuccessCount)
			outbound := decodeSingBoxHysteriaOutbound(t, out)
			if test.hysteria.UpMbps > 0 {
				require.Equal(t, test.wantUp, outbound["up_mbps"])
				require.Equal(t, test.wantDown, outbound["down_mbps"])
			} else {
				require.Equal(t, test.wantUp, outbound["up"])
				require.Equal(t, test.wantDown, outbound["down"])
			}
			options := decodeLockedSingBoxOptions(t, out)
			lockedHysteria, ok := options.Outbounds[0].Options.(*option.HysteriaOutboundOptions)
			require.True(t, ok)
			if lockedHysteria.Up != nil {
				require.Equal(t, test.wantUpBps, lockedHysteria.Up.Value())
				require.Equal(t, test.wantDownBps, lockedHysteria.Down.Value())
			} else {
				require.Equal(t, int(test.wantUpBps/125_000), lockedHysteria.UpMbps)
				require.Equal(t, int(test.wantDownBps/125_000), lockedHysteria.DownMbps)
			}
		})
	}
}

func TestRenderSingBoxHysteriaUnrepresentableLowercaseBpsSkipsOnlyThatNode(t *testing.T) {
	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{
		{
			Name: "unrepresentable", Type: domain.NodeTypeHysteria, Server: "invalid.example", Port: 8443,
			TLS:      &domain.TLSOptions{Enabled: true},
			Hysteria: &domain.HysteriaOptions{Up: "55 bps", Down: "56 bps"},
		},
		{Name: "valid", Type: domain.NodeTypeHTTP, Server: "valid.example", Port: 8080},
	}, domain.RenderOptions{Format: "sing-box-outbounds"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Len(t, report.Warnings, 1)
	require.Equal(t, "render_node_skipped", report.Warnings[0].Code)
	require.Equal(t, "unrepresentable", report.Warnings[0].Node)
	requireLockedSingBoxOptions(t, out)
	outbound := decodeSingBoxHysteriaOutbound(t, out)
	require.Equal(t, "valid", outbound["tag"])
}

func TestRenderSingBoxHysteriaInvalidBandwidthSkipsOnlyInvalidNode(t *testing.T) {
	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{
		{
			Name: "invalid", Type: domain.NodeTypeHysteria, Server: "invalid.example", Port: 8443,
			TLS:      &domain.TLSOptions{Enabled: true},
			Hysteria: &domain.HysteriaOptions{AuthString: "secret", Up: "55", DownMbps: 100},
		},
		{Name: "valid", Type: domain.NodeTypeHTTP, Server: "valid.example", Port: 8080},
	}, domain.RenderOptions{Format: "sing-box-outbounds"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Len(t, report.Warnings, 1)
	require.Equal(t, "render_node_skipped", report.Warnings[0].Code)
	require.Equal(t, "invalid", report.Warnings[0].Node)
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(out, &doc))
	require.Len(t, doc.Outbounds, 1)
	require.Equal(t, "valid", doc.Outbounds[0]["tag"])
	requireLockedSingBoxOptions(t, out)
}

func TestRenderSingBoxHysteriaMihomoBareBandwidthUsesMbps(t *testing.T) {
	nodes, _, err := mihomo.NewParser().Parse(context.Background(), []byte(`
proxies:
  - {name: hy, type: hysteria, server: example.com, port: 8443, up: "55", down: "100", auth-str: secret, tls: true}
`))
	require.NoError(t, err)

	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	outbound := decodeSingBoxHysteriaOutbound(t, out)
	require.Equal(t, float64(55), outbound["up_mbps"])
	require.Equal(t, float64(100), outbound["down_mbps"])
	requireLockedSingBoxOptions(t, out)
}

func TestRenderSingBoxHysteriaURIBandwidthUsesMbps(t *testing.T) {
	nodes, _, err := uriadapter.NewParser().Parse(context.Background(), []byte(
		"hysteria://example.com:8443?auth=secret&up=55&down=100#hy",
	))
	require.NoError(t, err)

	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	requireLockedSingBoxOptions(t, out)
	outbound := decodeSingBoxHysteriaOutbound(t, out)
	require.Equal(t, float64(55), outbound["up_mbps"])
	require.Equal(t, float64(100), outbound["down_mbps"])
	require.Equal(t, "c2VjcmV0", outbound["auth"])
	require.NotContains(t, outbound, "auth_str")
}

func TestRenderSingBoxHysteriaAuthVariantsUseDistinctLockedFields(t *testing.T) {
	for _, test := range []struct {
		name      string
		hysteria  *domain.HysteriaOptions
		wantField string
		wantValue string
		omitField string
	}{
		{
			name: "binary auth", hysteria: &domain.HysteriaOptions{Auth: "secret", UpMbps: 55, DownMbps: 100},
			wantField: "auth", wantValue: "c2VjcmV0", omitField: "auth_str",
		},
		{
			name: "string auth", hysteria: &domain.HysteriaOptions{AuthString: "secret", UpMbps: 55, DownMbps: 100},
			wantField: "auth_str", wantValue: "secret", omitField: "auth",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{{
				Name: "hy", Type: domain.NodeTypeHysteria, Server: "example.com", Port: 8443,
				TLS: &domain.TLSOptions{Enabled: true}, Hysteria: test.hysteria,
			}}, domain.RenderOptions{Format: "sing-box-outbounds"})

			require.NoError(t, err)
			require.Equal(t, 1, report.SuccessCount)
			requireLockedSingBoxOptions(t, out)
			outbound := decodeSingBoxHysteriaOutbound(t, out)
			require.Equal(t, test.wantValue, outbound[test.wantField])
			require.NotContains(t, outbound, test.omitField)
		})
	}
}

func TestRenderSingBoxHysteriaAuthRoundTripPreservesDecodedBytes(t *testing.T) {
	nodes, _, err := singbox.NewParser().Parse(context.Background(), []byte(`{
		"outbounds":[{"type":"hysteria","tag":"hy","server":"example.com","server_port":8443,"auth":"c2VjcmV0","up_mbps":55,"down_mbps":100,"tls":{"enabled":true}}]
	}`))
	require.NoError(t, err)

	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	options := decodeLockedSingBoxOptions(t, out)
	require.Len(t, options.Outbounds, 1)
	lockedHysteria, ok := options.Outbounds[0].Options.(*option.HysteriaOutboundOptions)
	require.True(t, ok)
	require.Equal(t, []byte("secret"), lockedHysteria.Auth)
	outbound := decodeSingBoxHysteriaOutbound(t, out)
	require.Equal(t, "c2VjcmV0", outbound["auth"])
	require.NotEqual(t, "YzJWamNtVjA=", outbound["auth"])
	require.Equal(t, "secret", nodes[0].Hysteria.Auth)
}

func decodeSingBoxHysteriaOutbound(t *testing.T, out []byte) map[string]any {
	t.Helper()
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(out, &doc))
	require.Len(t, doc.Outbounds, 1)
	return doc.Outbounds[0]
}

func requireLockedSingBoxOptions(t *testing.T, out []byte) {
	t.Helper()
	decodeLockedSingBoxOptions(t, out)
}

func decodeLockedSingBoxOptions(t *testing.T, out []byte) option.Options {
	t.Helper()
	boxContext := include.Context(context.Background())
	var options option.Options
	require.NoError(t, options.UnmarshalJSONContext(boxContext, out))
	return options
}
