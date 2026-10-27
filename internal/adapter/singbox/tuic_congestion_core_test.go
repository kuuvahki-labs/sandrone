//go:build probe_singbox && with_quic

package singbox_test

import (
	"encoding/json/v2"
	"testing"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestLockedSingBoxTUICCongestionControl(t *testing.T) {
	for _, algorithm := range []string{"", "cubic", "new_reno", "bbr", "bbr_tag=fixture", "bbr_meta_v1", "bbr_meta_v2", "BBR", " bbr ", "reno"} {
		t.Run(algorithm, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"outbounds": []any{map[string]any{
				"type": "tuic", "tag": "fixture", "server": "127.0.0.1", "server_port": 9,
				"uuid": "11111111-1111-1111-1111-111111111111", "password": "fixture-password",
				"tls": map[string]any{"enabled": true, "server_name": "public.example"}, "congestion_control": algorithm,
			}}})
			require.NoError(t, err)
			ctx := include.Context(t.Context())
			var options option.Options
			require.NoError(t, options.UnmarshalJSONContext(ctx, body))
			instance, coreErr := box.New(box.Options{Context: ctx, Options: options})
			_, report, renderErr := singbox.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{tuicCongestionNode(algorithm)}, domain.RenderOptions{})
			if coreErr != nil {
				require.ErrorContains(t, coreErr, "unknown congestion control algorithm")
				require.True(t, domain.IsCode(renderErr, domain.CodeRenderFailed))
				require.Zero(t, report.SuccessCount)
				return
			}
			require.NoError(t, instance.Close())
			require.NoError(t, renderErr)
			require.Equal(t, 1, report.SuccessCount)
			require.Empty(t, report.Warnings)
		})
	}
}
