//go:build probe_singbox

package service_test

import (
	"context"
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
	ctx := context.Background()
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
	instance, err := box.New(box.Options{Context: boxContext, Options: options})
	require.NoError(t, err)
	require.NoError(t, instance.Close())
}

func TestServiceSingBoxWebDefaultRejectsEmptyURLTest(t *testing.T) {
	spec := singBoxWebDefaultSpec(t, []any{})
	spec.Config.Subscriptions = nil

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: spec})
	require.NoError(t, err)

	boxContext := include.Context(context.Background())
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
  "dns": {
    "servers": [
      { "type": "local", "tag": "dns-local" },
      { "type": "https", "tag": "dns-cn", "server": "223.5.5.5", "detour": "direct" },
      { "type": "https", "tag": "dns-remote", "server": "1.1.1.1", "detour": "Proxy" }
    ],
    "rules": [
      { "rule_set": ["private"], "action": "route", "server": "dns-local" },
      { "rule_set": ["cn"], "action": "route", "server": "dns-cn" }
    ],
    "final": "dns-remote",
    "strategy": "prefer_ipv4"
  },
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "address": ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
      "auto_route": true,
      "strict_route": true,
      "route_exclude_address": [
        "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
        "169.254.0.0/16", "fe80::/10", "fc00::/7",
        "224.0.0.251/32", "ff02::fb/128"
      ]
    },
    { "type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": 2080 }
  ],
  "outbounds": [],
  "route": {
    "auto_detect_interface": true,
    "default_domain_resolver": "dns-cn",
    "rule_set": [],
    "rules": []
  },
  "experimental": { "cache_file": { "enabled": true } }
}`},
		Config: &domain.FileConfig{
			Subscriptions: []string{"default"},
			Settings: raw(t, map[string]any{
				"groups": []map[string]any{
					{"type": "selector", "tag": "Proxy", "outbounds": []any{"Auto", "$nodes", "direct", "block"}},
					{
						"type": "urltest", "tag": "Auto", "outbounds": autoMembers,
						"url": "https://cp.cloudflare.com", "interval": "5m",
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
					singBoxRemoteRuleSet("category-companies@cn", "geosite", "category-companies@cn"),
					singBoxRemoteRuleSet("cn", "geosite", "cn"),
					singBoxRemoteRuleSet("cn-ip", "geoip", "cn"),
					singBoxRemoteRuleSet("geolocation-!cn", "geosite", "geolocation-!cn"),
				},
				"rules": []map[string]any{
					{"port": 853, "outbound": "Proxy"},
					{"rule_set": []any{"category-ads-all"}, "outbound": "Ad Block"},
					{"rule_set": []any{"private"}, "outbound": "Private"},
					{"rule_set": []any{"category-doh"}, "outbound": "Proxy"},
					{"rule_set": []any{"category-companies@cn"}, "outbound": "China"},
					{"rule_set": []any{"cn"}, "outbound": "China"},
					{"rule_set": []any{"geolocation-!cn"}, "outbound": "Global"},
					{"action": "resolve"},
					{"rule_set": []any{"private-ip"}, "outbound": "Private"},
					{"rule_set": []any{"cn-ip"}, "outbound": "China"},
					{"outbound": "Final"},
				},
			}),
		},
		Processors: []domain.ProcessorSpec{{
			Name:  "Sniff & DNS Hijack",
			Type:  "merge",
			Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"mode":    "json_override",
				"content": `{"route":{"+rules":[{"action":"sniff"},{"type":"logical","mode":"or","rules":[{"protocol":"dns"},{"port":53}],"action":"hijack-dns"}]}}`,
			}),
		}},
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
