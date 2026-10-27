//go:build probe_singbox

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

func TestLockedSingBoxTransportPathEscapes(t *testing.T) {
	for _, typ := range []string{"httpupgrade", "ws", "http"} {
		for _, path := range []string{"/private/%2.", "/query?token=%2.", "/fragment#%2."} {
			t.Run(typ+path, func(t *testing.T) {
				body, err := json.Marshal(map[string]any{"outbounds": []any{map[string]any{
					"type": "vmess", "tag": "fixture", "server": "127.0.0.1", "server_port": 9,
					"uuid": "11111111-1111-1111-1111-111111111111", "security": "auto",
					"transport": map[string]any{"type": typ, "path": path},
				}}})
				require.NoError(t, err)
				ctx := include.Context(t.Context())
				var options option.Options
				require.NoError(t, options.UnmarshalJSONContext(ctx, body))
				_, err = box.New(box.Options{Context: ctx, Options: options})
				require.ErrorContains(t, err, "parse path: invalid URL escape")
				bad := transportPathNode(typ, path, false)
				good := transportPathNode(typ, "/good%2Fpath?token=%25#fragment", false)
				good.Name = "neighbor"
				body, report, err := singbox.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{bad, good}, domain.RenderOptions{})
				require.NoError(t, err)
				require.Equal(t, 1, report.SuccessCount)
				require.Len(t, report.Warnings, 1)
				var filtered option.Options
				require.NoError(t, filtered.UnmarshalJSONContext(ctx, body))
				instance, err := box.New(box.Options{Context: ctx, Options: filtered})
				require.NoError(t, err)
				require.NoError(t, instance.Close())
			})
		}
	}
}
