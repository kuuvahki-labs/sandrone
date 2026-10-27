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

func tuicCongestionNode(algorithm string) domain.NodeIR {
	return domain.NodeIR{Name: "fixture", Type: domain.NodeTypeTUIC, Server: "127.0.0.1", Port: 9,
		UUID: "11111111-1111-1111-1111-111111111111", Password: "fixture-password",
		TLS:  &domain.TLSOptions{Enabled: true, ServerName: "public.example"},
		TUIC: &domain.TUICOptions{CongestionControl: algorithm},
	}
}

func TestRenderSingBoxTUICCongestionControl(t *testing.T) {
	for _, algorithm := range []string{"bbr_tag=fixture", "bbr_meta_v1", "bbr_meta_v2", "BBR", " bbr ", "reno"} {
		t.Run(algorithm, func(t *testing.T) {
			bad := tuicCongestionNode(algorithm)
			good := tuicCongestionNode("bbr")
			good.Name = "neighbor"
			nodes := []domain.NodeIR{bad, good}
			body, report, err := singbox.NewRenderer().RenderWithReport(t.Context(), nodes, domain.RenderOptions{})
			require.NoError(t, err)
			require.Equal(t, 1, report.SuccessCount)
			require.Len(t, report.Warnings, 1)
			require.Equal(t, "render_node_skipped", report.Warnings[0].Code)
			require.Equal(t, 0, *report.Warnings[0].NodeIndex)
			require.Contains(t, report.Warnings[0].Message, "unsupported sing-box TUIC congestion control")
			require.NotContains(t, report.Warnings[0].Message, algorithm)
			require.NotContains(t, report.Warnings[0].Message, "fixture-password")
			var output struct {
				Outbounds []map[string]any `json:"outbounds"`
			}
			require.NoError(t, json.Unmarshal(body, &output))
			require.Len(t, output.Outbounds, 1)
			require.Equal(t, "neighbor", output.Outbounds[0]["tag"])
			require.Equal(t, "bbr", output.Outbounds[0]["congestion_control"])
			require.Equal(t, algorithm, nodes[0].TUIC.CongestionControl)
			_, report, err = singbox.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{bad}, domain.RenderOptions{})
			require.True(t, domain.IsCode(err, domain.CodeRenderFailed))
			require.Zero(t, report.SuccessCount)
		})
	}
	for _, algorithm := range []string{"", "cubic", "new_reno", "bbr"} {
		t.Run("valid_"+algorithm, func(t *testing.T) {
			node := tuicCongestionNode(algorithm)
			body, report, err := singbox.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{node}, domain.RenderOptions{})
			require.NoError(t, err)
			require.Empty(t, report.Warnings)
			var output struct {
				Outbounds []struct {
					CongestionControl string `json:"congestion_control"`
				} `json:"outbounds"`
			}
			require.NoError(t, json.Unmarshal(body, &output))
			require.Equal(t, algorithm, output.Outbounds[0].CongestionControl)
			require.Equal(t, algorithm, node.TUIC.CongestionControl)
		})
	}
}

func TestTUICMihomoCongestionNamesRemainUnchanged(t *testing.T) {
	for _, algorithm := range []string{"bbr_meta_v1", "bbr_meta_v2", "bbr_tag=fixture"} {
		node := tuicCongestionNode(algorithm)
		body, report, err := mihomo.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{node}, domain.RenderOptions{})
		require.NoError(t, err)
		require.Empty(t, report.Warnings)
		var output struct {
			Proxies []map[string]any `yaml:"proxies"`
		}
		require.NoError(t, yaml.Unmarshal(body, &output))
		require.Equal(t, algorithm, output.Proxies[0]["congestion-controller"])
	}
}
