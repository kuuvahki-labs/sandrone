//go:build probe_mihomo

package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	mihomoconfig "github.com/metacubex/mihomo/config"
	// Mihomo config.Parse links its runtime general updater from this package.
	_ "github.com/metacubex/mihomo/hub/executor"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceMihomoAdaptiveGroupIsAcceptedByLockedCore(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "default",
		Type:   domain.SubscriptionTypeLocal,
		Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@us.example.com:8388#US-keep\n" +
			"ss://aes-128-gcm:secret@excluded.example.com:8388#US-%E7%BE%8E%E5%B1%9E-excluded\n" +
			"ss://aes-128-gcm:secret@jp.example.com:8388#JP-skip",
	}))
	spec := domain.FileSpec{
		Name: "adaptive.yaml",
		Kind: domain.FileKindMihomo,
		Source: domain.FileSource{Type: "inline", Content: `mixed-port: 7890
proxy-providers:
  provider-only:
    type: inline
    payload:
      - name: US-provider-only
        type: ss
        server: provider.example.com
        port: 8388
        cipher: aes-128-gcm
        password: secret
`},
		Config: &domain.FileConfig{
			Subscriptions: []string{"default"},
			Settings: raw(t, map[string]any{
				"groups": []map[string]any{
					{
						"name":    "Proxy",
						"type":    "select",
						"proxies": []any{"美国节点", "$nodes", "DIRECT"},
					},
					{
						"name":                "美国节点",
						"type":                "load-balance",
						"include-all-proxies": true,
						"filter":              `(?i)(?:美国|美國|United States|America|洛杉矶|洛杉磯|纽约|紐約|西雅图|西雅圖|硅谷|🇺🇸|\bUS\b|\bUSA\b|\bLAX\b|\bSFO\b|\bSJC\b|\bSEA\b|\bNYC\b|\bJFK\b|\bEWR\b|\bIAD\b|\bATL\b|\bORD\b|\bMIA\b|\bDFW\b)`,
						"exclude-filter":      `(?i)(?:美属|美屬|亚美尼亚|亞美尼亞|圣多美|聖多美)`,
						"url":                 "https://cp.cloudflare.com",
						"interval":            300,
						"lazy":                true,
						"strategy":            "sticky-sessions",
					},
				},
				"rules": []string{"MATCH,Proxy"},
			}),
		},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})
	require.NoError(t, err)

	parsed, err := mihomoconfig.Parse(result.Content)
	require.NoError(t, err)
	for _, proxy := range parsed.Proxies {
		proxy := proxy
		t.Cleanup(func() { require.NoError(t, proxy.Close()) })
	}

	provider, ok := parsed.Providers["provider-only"]
	require.True(t, ok)
	require.Len(t, provider.Proxies(), 1)
	require.Equal(t, "US-provider-only", provider.Proxies()[0].Name())

	group, ok := parsed.Proxies["美国节点"]
	require.True(t, ok)
	groupJSON, err := group.Adapter().MarshalJSON()
	require.NoError(t, err)
	var groupState struct {
		All     []string `json:"all"`
		TestURL string   `json:"testUrl"`
		Type    string   `json:"type"`
	}
	require.NoError(t, json.Unmarshal(groupJSON, &groupState))
	require.Equal(t, "LoadBalance", groupState.Type)
	require.Equal(t, "https://cp.cloudflare.com", groupState.TestURL)
	require.Equal(t, []string{"US-keep"}, groupState.All)

	invalid := bytes.Replace(result.Content, []byte("strategy: sticky-sessions"), []byte("strategy: unsupported"), 1)
	require.NotEqual(t, result.Content, invalid)
	_, err = mihomoconfig.Parse(invalid)
	require.ErrorContains(t, err, "unsupported strategy: unsupported")
}
