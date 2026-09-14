//go:build probe_mihomo

package service_test

import (
	"bytes"
	"encoding/json/v2"
	"testing"

	mihomoconfig "github.com/metacubex/mihomo/config"
	// Mihomo config.Parse links its runtime general updater from this package.
	_ "github.com/metacubex/mihomo/hub/executor"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceMihomoWebDNSIsAcceptedByLockedCore(t *testing.T) {
	spec := domain.FileSpec{
		Name: "dns.yaml",
		Kind: domain.FileKindMihomo,
		Source: domain.FileSource{Type: "inline", Content: `mixed-port: 7890
mode: rule
ipv6: false
sniffer:
  enable: true
  override-destination: false
  skip-domain:
    - "Mijia Cloud"
    - "dlg.io.mi.com"
    - "+.push.apple.com"
    - "+.akadns.net"
  sniff:
    HTTP:
      ports: [80, 8080, 8880]
    TLS:
      ports: [443, 8443]
    QUIC:
      ports: [443, 8443]
tun:
  enable: true
  stack: mixed
  auto-route: true
  strict-route: true
  auto-detect-interface: true
  dns-hijack:
    - any:53
    - tcp://any:53
  route-exclude-address:
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
    - 169.254.0.0/16
    - 17.249.0.0/16
    - 17.252.0.0/16
    - 17.57.144.0/22
    - 17.188.128.0/18
    - 17.188.20.0/23
    - fe80::/10
    - fc00::/7
    - 2620:149:a44::/48
    - 2403:300:a42::/48
    - 2403:300:a51::/48
    - 2a01:b740:a42::/48
    - 224.0.0.251/32
    - ff02::fb/128
dns:
  enable: true
  ipv6: false
  prefer-h3: true
  enhanced-mode: fake-ip
  fake-ip-filter:
    - "*"
    - geosite:private
    - geosite:connectivity-check
    - "+.lan"
    - "+.local"
    - "+.market.xiaomi.com"
    - "Mijia Cloud"
    - "dlg.io.mi.com"
    - "localhost.ptlogin2.qq.com"
    - "localhost.sec.qq.com"
    - "localhost.*.weixin.qq.com"
    - "*.icloud.com"
    - "+.push.apple.com"
    - "+.akadns.net"
    - "time.*.com"
    - "ntp.*.com"
    - "+.pool.ntp.org"
    - "stun.*.*"
    - "stun.*.*.*"
  default-nameserver:
    - "https://223.5.5.5/dns-query#DIRECT"
    - "https://223.6.6.6/dns-query#DIRECT"
  nameserver-policy:
    "geosite:private":
      - system
    "+.push.apple.com":
      - system
    "+.akadns.net":
      - system
    "rule-set:cn":
      - "https://223.5.5.5/dns-query#DIRECT"
      - "https://223.6.6.6/dns-query#DIRECT"
  nameserver:
    - "https://cloudflare-dns.com/dns-query"
    - "https://dns.google/dns-query"
  proxy-server-nameserver:
    - "https://223.5.5.5/dns-query#DIRECT"
    - "https://223.6.6.6/dns-query#DIRECT"
  direct-nameserver:
    - "https://223.5.5.5/dns-query#DIRECT"
    - "https://223.6.6.6/dns-query#DIRECT"
proxies: []
proxy-groups: []
rule-providers: {}
rules: []
`},
		Config: &domain.FileConfig{Settings: completeTypedSettings(t, map[string]any{
			"groups": []map[string]any{
				{"name": "Proxy", "type": "select", "proxies": []any{"DIRECT"}},
			},
			"rule_sets": []map[string]any{
				{
					"name": "cn", "type": "inline", "behavior": "domain",
					"payload": []any{"DOMAIN-SUFFIX,cn"},
				},
			},
			"rules": []any{
				"DST-PORT,853,Proxy",
				"RULE-SET,cn,DIRECT",
				"MATCH,Proxy",
			},
		})},
	}

	result, err := service.New().GetFile(t.Context(), domain.FileRequest{Spec: &spec})
	require.NoError(t, err)

	parsed, err := mihomoconfig.Parse(result.Content)
	require.NoError(t, err)
	require.False(t, parsed.General.IPv6)
	require.False(t, parsed.DNS.IPv6)
	require.Contains(t, string(result.Content), "dlg.io.mi.com")
	for _, proxy := range parsed.Proxies {
		proxy := proxy
		t.Cleanup(func() { require.NoError(t, proxy.Close()) })
	}
	require.Contains(t, parsed.Proxies, "Proxy")
}

func TestServiceMihomoAdaptiveGroupIsAcceptedByLockedCore(t *testing.T) {
	ctx := t.Context()
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
			Settings: completeTypedSettings(t, map[string]any{
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
