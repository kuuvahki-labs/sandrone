//go:build probe_mihomo

package mihomo_test

import (
	"context"
	"testing"

	upstream "github.com/metacubex/mihomo/adapter"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestRenderMihomoHysteria2RealmIsAcceptedByLockedCore(t *testing.T) {
	out, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{{
		Name: "hy2", Type: domain.NodeTypeHysteria2, Server: "dummy.example.com", Port: 443, Password: "secret",
		TLS: &domain.TLSOptions{Enabled: true},
		Hysteria: &domain.HysteriaOptions{Realm: &domain.HysteriaRealmOptions{
			Enabled: true, ServerURL: "https://realm.example.com", RealmID: "realm", STUNServers: []string{"stun.example.com"},
		}},
	}}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Empty(t, report.Warnings)

	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(out, &doc))
	require.Len(t, doc.Proxies, 1)
	proxy, err := upstream.ParseProxy(doc.Proxies[0])
	require.NoError(t, err)
	require.NoError(t, proxy.Close())
}

func TestLockedMihomoRejectsInvalidHysteria2Fingerprint(t *testing.T) {
	_, err := upstream.ParseProxy(map[string]any{
		"name": "hy2", "type": "hysteria2", "server": "example.com", "port": 443,
		"password": "secret", "fingerprint": "chrome",
	})
	require.Error(t, err)
}
