//go:build probe_mihomo

package mihomo_test

import (
	"testing"

	upstream "github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/transport/tuic/common"
	"github.com/stretchr/testify/require"
)

func TestLockedMihomoTUICCongestionControlFallback(t *testing.T) {
	for _, algorithm := range []string{"", "cubic", "new_reno", "bbr", "bbr_meta_v1", "bbr_meta_v2", "bbr_tag=fixture"} {
		t.Run(algorithm, func(t *testing.T) {
			proxy, err := upstream.ParseProxy(map[string]any{
				"type": "tuic", "name": "fixture", "server": "127.0.0.1", "port": 9,
				"uuid": "11111111-1111-1111-1111-111111111111", "password": "fixture-password",
				"sni": "public.example", "congestion-controller": algorithm,
			})
			require.NoError(t, err)
			require.NoError(t, proxy.Close())
		})
	}
	// Unknown names do not select a controller. This returns without touching
	// the connection, unlike a recognized algorithm, so no network is needed.
	require.NotPanics(t, func() { common.SetCongestionController(nil, "bbr_tag=fixture", 0, "") })
}
