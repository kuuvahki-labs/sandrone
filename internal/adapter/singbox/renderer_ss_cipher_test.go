package singbox_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestRenderSingBoxSkipsUnsupportedSSCipherOnlyForTarget(t *testing.T) {
	for _, cipher := range []string{"chacha20", "aes-128-ccm", "chacha8-ietf-poly1305", "aegis-128l", "unknown-cipher"} {
		t.Run(cipher, func(t *testing.T) {
			bad := domain.NodeIR{Name: "unsupported", Type: domain.NodeTypeShadowsocks, Server: "127.0.0.1", Port: 9, Cipher: cipher, Password: "fixture-password"}
			good := domain.NodeIR{Name: "neighbor", Type: domain.NodeTypeShadowsocks, Server: "127.0.0.1", Port: 9, Cipher: "chacha20-ietf", Password: "fixture-password"}
			nodes := []domain.NodeIR{bad, good}
			body, report, err := singbox.NewRenderer().RenderWithReport(t.Context(), nodes, domain.RenderOptions{})
			require.NoError(t, err)
			require.Equal(t, 1, report.SuccessCount)
			require.Len(t, report.Warnings, 1)
			warning := report.Warnings[0]
			require.Equal(t, "render_node_skipped", warning.Code)
			require.Equal(t, "sing-box-outbounds", warning.Target)
			require.Equal(t, "unsupported", warning.Node)
			require.Equal(t, 0, *warning.NodeIndex)
			require.Contains(t, warning.Message, "unsupported sing-box shadowsocks cipher")
			require.NotContains(t, warning.Message, "fixture-password")
			var output struct {
				Outbounds []map[string]any `json:"outbounds"`
			}
			require.NoError(t, json.Unmarshal(body, &output))
			require.Len(t, output.Outbounds, 1)
			require.Equal(t, "neighbor", output.Outbounds[0]["tag"])
			require.Equal(t, "chacha20-ietf", output.Outbounds[0]["method"])
			require.Equal(t, []domain.NodeIR{bad, good}, nodes)

			_, report, err = singbox.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{bad}, domain.RenderOptions{})
			require.True(t, domain.IsCode(err, domain.CodeRenderFailed))
			require.Zero(t, report.SuccessCount)
			require.Len(t, report.Warnings, 1)
		})
	}
}

func TestSSLegacyChaCha20RemainsAvailableToMihomo(t *testing.T) {
	nodes := []domain.NodeIR{{Name: "legacy", Type: domain.NodeTypeShadowsocks, Server: "127.0.0.1", Port: 9, Cipher: "chacha20", Password: "fixture"}}
	body, report, err := mihomo.NewRenderer().RenderWithReport(t.Context(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Empty(t, report.Warnings)
	var output struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(body, &output))
	require.Len(t, output.Proxies, 1)
	require.Equal(t, "chacha20", output.Proxies[0]["cipher"])
	require.Equal(t, "chacha20", nodes[0].Cipher)
}
