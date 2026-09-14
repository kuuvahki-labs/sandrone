//go:build probe_singbox

package service_test

import (
	"encoding/json/v2"
	"slices"
	"testing"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceSingBoxWebDefaultIsAcceptedByLockedCore(t *testing.T) {
	ctx := t.Context()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "default",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: singBoxWebDefaultSpec(t, []any{"$nodes"})})
	require.NoError(t, err)

	boxContext := include.Context(ctx)
	var options option.Options
	require.NoError(t, options.UnmarshalJSONContext(boxContext, result.Content))
	require.Len(t, options.HTTPClients, 1)
	require.Equal(t, "rule-set-direct", options.HTTPClients[0].Tag)
	require.Empty(t, options.HTTPClients[0].Options().Detour)
	require.NotNil(t, options.Route)
	require.Equal(t, "rule-set-direct", options.Route.DefaultHTTPClient)
	var document map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &document))
	routeRules := document["route"].(map[string]any)["rules"].([]any)
	require.Equal(t, []any{
		map[string]any{
			"type": "logical", "mode": "and", "action": "reject",
			"rules": []any{
				map[string]any{"inbound": []any{"mixed-in"}},
				map[string]any{
					"source_ip_cidr": []any{"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
					"invert":         true,
				},
			},
		},
		map[string]any{"action": "sniff"},
		map[string]any{
			"type": "logical", "mode": "or", "action": "resolve", "server": "dns-local", "strategy": "ipv4_only",
			"rules": []any{
				map[string]any{"domain_regex": []any{"^[^.]+$"}},
				map[string]any{"domain_suffix": []any{
					"lan", "localdomain", "localhost", "local", "home.arpa", "internal", "example", "invalid", "test",
				}},
			},
		},
		map[string]any{
			"type": "logical", "mode": "or", "action": "hijack-dns",
			"rules": []any{map[string]any{"protocol": "dns"}, map[string]any{"port": float64(53)}},
		},
		map[string]any{
			"domain_suffix": []any{"push.apple.com", "akadns.net"},
			"outbound":      "direct",
		},
		map[string]any{"clash_mode": "direct", "outbound": "direct"},
		map[string]any{"clash_mode": "global", "outbound": "Proxy"},
	}, routeRules[:7])
	instance, err := box.New(box.Options{Context: boxContext, Options: options})
	require.NoError(t, err)
	require.NoError(t, instance.Close())

	require.NotNil(t, options.DNS)
	require.Equal(t, "ipv4_only", options.DNS.Strategy.String())
	dnsCNIndex := slices.IndexFunc(options.DNS.Servers, func(server option.DNSServerOptions) bool {
		return server.Tag == "dns-cn"
	})
	require.NotEqual(t, -1, dnsCNIndex)
	directIndex := slices.IndexFunc(options.Outbounds, func(outbound option.Outbound) bool {
		return outbound.Tag == "direct"
	})
	require.NotEqual(t, -1, directIndex)

	// Start only the generated DNS/direct pair so the test exercises dialer
	// initialization without downloading remote rule sets.
	startOptions := option.Options{
		DNS: new(option.DNSOptions{
			Servers: []option.DNSServerOptions{options.DNS.Servers[dnsCNIndex]},
		}),
		HTTPClients: options.HTTPClients,
		Outbounds:   []option.Outbound{options.Outbounds[directIndex]},
		Route: new(option.RouteOptions{
			DefaultHTTPClient: options.Route.DefaultHTTPClient,
		}),
	}
	instance, err = box.New(box.Options{Context: boxContext, Options: startOptions})
	require.NoError(t, err)
	require.NoError(t, instance.Start())
	require.NoError(t, instance.Close())
}

func TestServiceSingBoxWebDefaultRejectsEmptyURLTest(t *testing.T) {
	spec := singBoxWebDefaultSpec(t, []any{})
	spec.Config.Subscriptions = nil

	result, err := service.New().GetFile(t.Context(), domain.FileRequest{Spec: spec})
	require.NoError(t, err)

	boxContext := include.Context(t.Context())
	var options option.Options
	require.NoError(t, options.UnmarshalJSONContext(boxContext, result.Content))
	_, err = box.New(box.Options{Context: boxContext, Options: options})
	require.ErrorContains(t, err, "missing tags")
}

func singBoxWebDefaultSpec(t *testing.T, autoMembers []any) *domain.FileSpec {
	t.Helper()
	return &domain.FileSpec{
		Name: "default.json",
		Kind: domain.FileKindSingBox,
		Source: domain.FileSource{Type: "inline", Content: `{
  "log": { "level": "info" },
  "http_clients": [
    { "tag": "rule-set-direct" }
  ],
  "dns": {
    "servers": [
      { "type": "local", "tag": "dns-local", "neighbor_domain": [".", ".lan"] },
      { "type": "https", "tag": "dns-cn", "server": "223.5.5.5" },
      { "type": "https", "tag": "dns-remote", "server": "1.1.1.1", "detour": "Proxy" },
      {
        "type": "fakeip",
        "tag": "dns-fakeip",
        "inet4_range": "198.18.0.0/15",
        "inet6_range": "fc00::/18"
      }
    ],
    "rules": [
      {
        "preferred_by": ["dns-local"],
        "action": "route",
        "server": "dns-local"
      },
      {
        "domain_regex": ["^[^.]+$"],
        "domain_suffix": [
          "lan",
          "localdomain",
          "localhost",
          "local",
          "home.arpa",
          "internal",
          "example",
          "invalid",
          "test"
        ],
        "action": "route",
        "server": "dns-local"
      },
      {
        "domain_suffix": ["push.apple.com", "akadns.net"],
        "action": "route",
        "server": "dns-local"
      },
      {
        "domain": [
          "Mijia Cloud",
          "dlg.io.mi.com",
          "localhost.ptlogin2.qq.com",
          "localhost.sec.qq.com"
        ],
        "domain_suffix": ["market.xiaomi.com", "pool.ntp.org"],
        "domain_regex": [
          "^[^.]+\\.icloud\\.com$",
          "^localhost\\.[^.]+\\.weixin\\.qq\\.com$",
          "^time\\.[^.]+\\.com$",
          "^ntp\\.[^.]+\\.com$",
          "^stun\\.[^.]+\\.[^.]+$",
          "^stun\\.[^.]+\\.[^.]+\\.[^.]+$"
        ],
        "action": "route",
        "server": "dns-remote"
      },
      { "query_type": ["A", "AAAA"], "action": "route", "server": "dns-fakeip" },
      { "rule_set": ["cn"], "action": "route", "server": "dns-cn" }
    ],
    "final": "dns-remote",
    "strategy": "ipv4_only"
  },
  "inbounds": [
    { "type": "mixed", "tag": "mixed-in", "listen": "0.0.0.0", "listen_port": 2080 },
    {
      "type": "tun",
      "tag": "tun-in",
      "address": ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
      "auto_route": true,
      "strict_route": true,
      "route_exclude_address": [
        "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16",
        "17.249.0.0/16", "17.252.0.0/16", "17.57.144.0/22", "17.188.128.0/18",
        "17.188.20.0/23", "fe80::/10", "fc00::/7", "2620:149:a44::/48",
        "2403:300:a42::/48", "2403:300:a51::/48", "2a01:b740:a42::/48",
        "224.0.0.251/32", "ff02::fb/128"
      ]
    }
  ],
  "outbounds": [],
  "route": {
    "auto_detect_interface": true,
    "default_domain_resolver": "dns-cn",
    "default_http_client": "rule-set-direct",
    "rule_set": [],
    "rules": [
      {
        "type": "logical",
        "mode": "and",
        "rules": [
          { "inbound": ["mixed-in"] },
          {
            "source_ip_cidr": ["127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"],
            "invert": true
          }
        ],
        "action": "reject"
      },
      { "action": "sniff" },
      {
        "type": "logical",
        "mode": "or",
        "rules": [
          { "domain_regex": ["^[^.]+$"] },
          {
            "domain_suffix": [
              "lan",
              "localdomain",
              "localhost",
              "local",
              "home.arpa",
              "internal",
              "example",
              "invalid",
              "test"
            ]
          }
        ],
        "action": "resolve",
        "server": "dns-local",
        "strategy": "ipv4_only"
      },
      {
        "type": "logical",
        "mode": "or",
        "rules": [{ "protocol": "dns" }, { "port": 53 }],
        "action": "hijack-dns"
      },
      {
        "domain_suffix": ["push.apple.com", "akadns.net"],
        "outbound": "direct"
      },
      { "clash_mode": "direct", "outbound": "direct" },
      { "clash_mode": "global", "outbound": "Proxy" }
    ]
  },
  "experimental": { "cache_file": { "enabled": true, "store_fakeip": true } }
}`},
		Config: &domain.FileConfig{
			Subscriptions: []string{"default"},
			Settings: completeTypedSettings(t, map[string]any{
				"groups": []map[string]any{
					{"type": "selector", "tag": "Proxy", "outbounds": []any{"Auto", "$nodes", "direct", "block"}},
					{
						"type": "urltest", "tag": "Auto", "outbounds": autoMembers,
						"url": "https://cp.cloudflare.com", "interval": "5m", "tolerance": 50,
					},
					{"type": "selector", "tag": "Ad Block", "outbounds": []any{"block", "direct", "Proxy"}},
					{"type": "selector", "tag": "Private", "outbounds": []any{"direct", "Proxy", "Auto", "block"}},
					{"type": "selector", "tag": "China", "outbounds": []any{"direct", "Proxy", "Auto", "block"}},
					{"type": "selector", "tag": "Global", "outbounds": []any{"Proxy", "Auto", "direct", "block"}},
					{"type": "selector", "tag": "Final", "outbounds": []any{"Proxy", "Auto", "direct", "block"}},
				},
				"rule_sets": []map[string]any{
					singBoxRemoteRuleSet("category-ads-all", "geosite", "category-ads-all"),
					singBoxRemoteRuleSet("private", "geosite", "private"),
					singBoxRemoteRuleSet("private-ip", "geoip", "private"),
					singBoxRemoteRuleSet("category-doh", "geosite", "category-doh"),
					singBoxRemoteRuleSet("cn", "geosite", "cn"),
					singBoxRemoteRuleSet("cn-ip", "geoip", "cn"),
					singBoxRemoteRuleSet("geolocation-!cn", "geosite", "geolocation-!cn"),
				},
				"rules": []map[string]any{
					{"port": 853, "outbound": "Proxy"},
					{"rule_set": []any{"category-ads-all"}, "outbound": "Ad Block"},
					{"rule_set": []any{"private"}, "outbound": "Private"},
					{"rule_set": []any{"category-doh"}, "outbound": "Proxy"},
					{"rule_set": []any{"cn"}, "outbound": "China"},
					{"rule_set": []any{"geolocation-!cn"}, "outbound": "Global"},
					{"action": "resolve"},
					{"rule_set": []any{"private-ip"}, "outbound": "Private"},
					{"rule_set": []any{"cn-ip"}, "outbound": "China"},
					{"outbound": "Final"},
				},
			}),
		},
		Processors: []domain.ProcessorSpec{
			singBoxOutboundAdapterProcessor(t, map[string]any{"default_outbound": "Proxy"}),
		},
	}
}

func singBoxRemoteRuleSet(tag, directory, file string) map[string]any {
	return map[string]any{
		"type":            "remote",
		"tag":             tag,
		"format":          "binary",
		"update_interval": "1d",
		"url":             "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/" + directory + "/" + file + ".srs",
	}
}
