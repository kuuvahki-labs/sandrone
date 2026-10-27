//go:build probe_mihomo

package singbox_test

import (
	"testing"

	mihomoadapter "github.com/metacubex/mihomo/adapter"
	"github.com/stretchr/testify/require"
)

func TestLockedMihomoSSCipherRegistryRetainsLegacyChaCha20(t *testing.T) {
	for _, cipher := range []string{"chacha20", "chacha20-ietf", "aes-128-ccm"} {
		t.Run(cipher, func(t *testing.T) {
			proxy, err := mihomoadapter.ParseProxy(map[string]any{
				"type": "ss", "name": "fixture", "server": "127.0.0.1", "port": 9,
				"cipher": cipher, "password": "fixture",
			})
			require.NoError(t, err)
			require.NoError(t, proxy.Close())
		})
	}
}
