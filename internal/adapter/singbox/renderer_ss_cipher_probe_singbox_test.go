//go:build probe_singbox

package singbox_test

import (
	"encoding/base64"
	"encoding/json/v2"
	"strings"
	"testing"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	sscipher "github.com/sagernet/sing-shadowsocks2/cipher"
	"github.com/sagernet/sing-shadowsocks2/shadowaead"
	"github.com/sagernet/sing-shadowsocks2/shadowaead_2022"
	"github.com/sagernet/sing-shadowsocks2/shadowstream"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestLockedSingBoxSSCipherRegistryDistinguishesChaCha20(t *testing.T) {
	for _, cipher := range []string{"chacha20", "chacha20-ietf"} {
		t.Run(cipher, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"outbounds": []map[string]any{{
				"type": "shadowsocks", "tag": "fixture", "server": "127.0.0.1", "server_port": 9,
				"method": cipher, "password": "fixture",
			}}})
			require.NoError(t, err)
			ctx := include.Context(t.Context())
			var options option.Options
			require.NoError(t, options.UnmarshalJSONContext(ctx, body))
			instance, err := box.New(box.Options{Context: ctx, Options: options})
			if cipher == "chacha20" {
				require.ErrorContains(t, err, "unknown method: chacha20")
				return
			}
			require.NoError(t, err)
			require.NoError(t, instance.Close())
		})
	}
}

func TestRenderSingBoxSSCipherSetMatchesLockedCoreAndIsolatesUnsupported(t *testing.T) {
	// Use the actual dependency lists so a core upgrade adding a cipher exposes
	// any stale renderer restriction instead of copying the production allowlist.
	ciphers := []string{sscipher.MethodNone}
	ciphers = append(ciphers, shadowstream.MethodList...)
	ciphers = append(ciphers, shadowaead.MethodList...)
	ciphers = append(ciphers, shadowaead_2022.MethodList...)
	nodes := []domain.NodeIR{{Name: "unsupported", Type: domain.NodeTypeShadowsocks, Server: "127.0.0.1", Port: 9, Cipher: "chacha20", Password: "fixture"}}
	for _, cipher := range ciphers {
		password := "fixture"
		if strings.HasPrefix(cipher, "2022-") {
			keySize := 32
			if cipher == "2022-blake3-aes-128-gcm" {
				keySize = 16
			}
			password = base64.StdEncoding.EncodeToString(make([]byte, keySize))
		}
		nodes = append(nodes, domain.NodeIR{Name: cipher, Type: domain.NodeTypeShadowsocks, Server: "127.0.0.1", Port: 9, Cipher: cipher, Password: password})
	}
	body, report, err := singbox.NewRenderer().RenderWithReport(t.Context(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, len(ciphers), report.SuccessCount)
	require.Len(t, report.Warnings, 1)
	require.Equal(t, "render_node_skipped", report.Warnings[0].Code)
	require.Equal(t, 0, *report.Warnings[0].NodeIndex)
	ctx := include.Context(t.Context())
	var options option.Options
	require.NoError(t, options.UnmarshalJSONContext(ctx, body))
	require.Len(t, options.Outbounds, len(ciphers))
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	require.NoError(t, err)
	require.NoError(t, instance.Close())
}
