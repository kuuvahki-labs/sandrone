package singbox_test

import (
	"context"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestRenderSingBoxSkipsUnsupportedSSPluginWithoutDowngrade(t *testing.T) {
	t.Parallel()
	for _, plugin := range []string{"gost-plugin", "shadow-tls", "unknown-plugin", " "} {
		t.Run(plugin, func(t *testing.T) {
			t.Parallel()
			bad := domain.NodeIR{Name: "unsupported", Type: domain.NodeTypeShadowsocks, Server: "bad.example", Port: 8388,
				Cipher: "aes-256-gcm", Password: "secret", Plugin: plugin, PluginOptions: map[string]any{"raw": "mode=secret-option"}}
			good := domain.NodeIR{Name: "neighbor", Type: domain.NodeTypeShadowsocks, Server: "good.example", Port: 8388,
				Cipher: "aes-256-gcm", Password: "secret"}
			renderer := singbox.NewRenderer()
			body, report, err := renderer.RenderWithReport(context.Background(), []domain.NodeIR{bad, good}, domain.RenderOptions{})
			require.NoError(t, err)
			require.Equal(t, 1, report.SuccessCount)
			require.Len(t, report.Warnings, 1)
			require.Equal(t, "render_node_skipped", report.Warnings[0].Code)
			require.Equal(t, "unsupported", report.Warnings[0].Node)
			require.Equal(t, "sing-box-outbounds", report.Warnings[0].Target)
			require.Equal(t, 0, *report.Warnings[0].NodeIndex)
			require.NotContains(t, report.Warnings[0].Message, "secret-option")
			require.NotContains(t, string(body), "bad.example")
			require.Contains(t, string(body), "good.example")
			require.Equal(t, plugin, bad.Plugin)
			require.Equal(t, "mode=secret-option", bad.PluginOptions["raw"])
			_, report, err = renderer.RenderWithReport(context.Background(), []domain.NodeIR{bad}, domain.RenderOptions{})
			require.True(t, domain.IsCode(err, domain.CodeRenderFailed))
			require.Zero(t, report.SuccessCount)
		})
	}
}

func TestRenderSingBoxKeepsSupportedSSPluginAliases(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ input, want string }{
		{"", ""}, {"obfs", "obfs-local"}, {"obfs-local", "obfs-local"}, {" SIMPLE-OBFS ", "obfs-local"},
		{"v2ray-plugin", "v2ray-plugin"}, {" V2RAY-PLUGIN ", "v2ray-plugin"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			body, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{{
				Name: "ss", Type: domain.NodeTypeShadowsocks, Server: "proxy.example", Port: 8388,
				Cipher: "aes-256-gcm", Password: "secret", Plugin: tc.input,
			}}, domain.RenderOptions{})
			require.NoError(t, err)
			require.Empty(t, report.Warnings)
			var doc struct {
				Outbounds []map[string]any `json:"outbounds"`
			}
			require.NoError(t, json.Unmarshal(body, &doc))
			if tc.want == "" {
				require.NotContains(t, doc.Outbounds[0], "plugin")
			} else {
				require.Equal(t, tc.want, doc.Outbounds[0]["plugin"])
			}
		})
	}
}
