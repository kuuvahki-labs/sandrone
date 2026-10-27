package singbox_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func transportPathNode(typ, path string, upgrade bool) domain.NodeIR {
	return domain.NodeIR{Name: "fixture", Type: domain.NodeTypeVMess, Server: "127.0.0.1", Port: 9,
		UUID: "11111111-1111-1111-1111-111111111111", Cipher: "auto",
		Transport: &domain.TransportOptions{Type: typ, Path: path, V2RayHTTPUpgrade: upgrade},
	}
}

func TestRenderSingBoxTransportPathEscapes(t *testing.T) {
	for _, transport := range []struct {
		typ     string
		upgrade bool
	}{{"httpupgrade", false}, {"ws", true}, {"websocket", true}, {"ws", false}, {"websocket", false}, {"http", false}} {
		for _, path := range []string{"/private/%2.", "/private/%", "/private/%2", "/ok?token=%2.", "/ok#%2."} {
			t.Run(transport.typ+path, func(t *testing.T) {
				bad := transportPathNode(transport.typ, path, transport.upgrade)
				good := transportPathNode(transport.typ, "/good%2Fpath", transport.upgrade)
				good.Name = "neighbor"
				nodes := []domain.NodeIR{bad, good}
				body, report, err := singbox.NewRenderer().RenderWithReport(t.Context(), nodes, domain.RenderOptions{})
				require.NoError(t, err)
				require.Equal(t, 1, report.SuccessCount)
				require.Len(t, report.Warnings, 1)
				require.Equal(t, "render_node_skipped", report.Warnings[0].Code)
				require.Equal(t, 0, *report.Warnings[0].NodeIndex)
				require.Contains(t, report.Warnings[0].Message, "invalid URL escape")
				require.NotContains(t, report.Warnings[0].Message, path)
				var output struct {
					Outbounds []map[string]any `json:"outbounds"`
				}
				require.NoError(t, json.Unmarshal(body, &output))
				require.Len(t, output.Outbounds, 1)
				require.Equal(t, "neighbor", output.Outbounds[0]["tag"])
				require.Equal(t, "/good%2Fpath", output.Outbounds[0]["transport"].(map[string]any)["path"])
				require.Equal(t, path, nodes[0].Transport.Path)
				_, report, err = singbox.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{bad}, domain.RenderOptions{})
				require.True(t, domain.IsCode(err, domain.CodeRenderFailed))
				require.Zero(t, report.SuccessCount)
			})
		}
		for _, path := range []string{"", "/", "relative", "/escaped%2Fpath", "/literal%252.", "/query?token=%25#fragment", "/space and 中文"} {
			t.Run(transport.typ+"_valid_"+path, func(t *testing.T) {
				node := transportPathNode(transport.typ, path, transport.upgrade)
				body, report, err := singbox.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{node}, domain.RenderOptions{})
				require.NoError(t, err)
				require.Empty(t, report.Warnings)
				var output struct {
					Outbounds []struct {
						Transport struct {
							Path string `json:"path"`
						} `json:"transport"`
					} `json:"outbounds"`
				}
				require.NoError(t, json.Unmarshal(body, &output))
				require.Equal(t, path, output.Outbounds[0].Transport.Path)
				require.Equal(t, path, node.Transport.Path)
			})
		}
	}
}
