package service_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/inidoc"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceCommunityPresetMihomoOrderedScenariosUseExactRawAsset(t *testing.T) {
	script := communityPresetRawScript(t, "insert-mihomo-rules.js")
	spec := domain.FileSpec{
		Name:   "mihomo-scenarios.yaml",
		Kind:   domain.FileKindMihomo,
		Source: domain.FileSource{Type: "inline", Content: "{}\n"},
		Config: &domain.FileConfig{Settings: completeTypedSettings(t, map[string]any{
			"rules": []string{
				"RULE-SET,private,DIRECT",
				"MATCH,Proxy",
			},
		})},
		Processors: []domain.ProcessorSpec{
			orderedRuleProcessor(t, script, "quic-fallback", "QUIC Fallback", `[
				"AND,((NETWORK,UDP),(DST-PORT,443)),REJECT"
			]`),
		},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.NotNil(t, result)
	doc := decodeMihomoCommunityPresetResult(t, result.Content)
	require.Equal(t, []any{
		"AND,((NETWORK,UDP),(DST-PORT,443)),REJECT",
		"RULE-SET,private,DIRECT",
		"MATCH,Proxy",
	}, doc["rules"])
	require.NotEqual(t, "off", doc["find-process-mode"])
	require.False(t, hasCaseInsensitiveKeyFragment(doc, "keepalive"))
}

func TestServiceCommunityPresetMihomoTailscaleNativeGeneratesFullFile(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "native-node",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:example-password@example.com:8388#Native-Node",
	}))
	script := communityPresetRawScript(t, "mihomo-tailscale-native.js")
	spec := domain.FileSpec{
		Name: "mihomo-tailscale-native.yaml",
		Kind: domain.FileKindMihomo,
		Source: domain.FileSource{Type: "inline", Content: `dns:
  fake-ip-filter:
    - base.example
    - +.ts.net
    - +.ts.net
  nameserver-policy:
    existing.example: system
tun:
  route-exclude-address:
    - 192.0.2.0/24
    - 100.64.0.0/10
    - fd7a:115c:a1e0::/48
    - 100.64.0.0/10
`},
		Config: &domain.FileConfig{
			Subscriptions: []string{"native-node"},
			Settings: completeTypedSettings(t, map[string]any{
				"rules": []string{
					"DOMAIN,user.example,DIRECT",
					"RULE-SET,private,DIRECT",
					"MATCH,LockedFinal",
				},
			}),
		},
		Processors: []domain.ProcessorSpec{
			mihomoTailscaleNativeProcessor(t, script, "tskey-auth-test"),
			mihomoTailscaleNativeProcessor(t, script, "tskey-auth-test"),
		},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.NotNil(t, result)
	doc := decodeMihomoCommunityPresetResult(t, result.Content)
	proxies := requireAnySlice(t, doc["proxies"])
	require.Len(t, proxies, 2)
	require.Equal(t, "Native-Node", requireStringMap(t, proxies[0])["name"])
	require.Equal(t, map[string]any{
		"name":          "TAILSCALE",
		"type":          "tailscale",
		"auth-key":      "tskey-auth-test",
		"ephemeral":     false,
		"udp":           true,
		"accept-routes": false,
	}, requireStringMap(t, proxies[1]))
	require.Equal(t, []any{
		"DOMAIN,user.example,DIRECT",
		"DOMAIN-SUFFIX,ts.net,TAILSCALE",
		"IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve",
		"IP-CIDR6,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve",
		"RULE-SET,private,DIRECT",
		"MATCH,LockedFinal",
	}, doc["rules"])
	dns := requireStringMap(t, doc["dns"])
	require.Equal(t, []any{"base.example", "+.ts.net"}, dns["fake-ip-filter"])
	require.Equal(t, map[string]any{
		"existing.example": "system",
		"+.ts.net":         "100.100.100.100",
	}, dns["nameserver-policy"])
	tun := requireStringMap(t, doc["tun"])
	require.Equal(t, []any{"192.0.2.0/24"}, tun["route-exclude-address"])
	require.NotContains(t, strings.ToLower(string(result.Content)), "exit-node")
}

func TestServiceCommunityPresetMihomoTailscaleExternalGeneratesDistinctFullFile(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "external-node",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:example-password@example.com:8388#External-Node",
	}))
	spec := domain.FileSpec{
		Name: "mihomo-tailscale-external.yaml",
		Kind: domain.FileKindMihomo,
		Source: domain.FileSource{Type: "inline", Content: `dns:
  fake-ip-filter:
    - base.example
tun:
  route-exclude-address:
    - 192.0.2.0/24
`},
		Config: &domain.FileConfig{
			Subscriptions: []string{"external-node"},
			Settings: completeTypedSettings(t, map[string]any{
				"rules": []string{
					"DOMAIN,user.example,DIRECT",
					"RULE-SET,private,DIRECT",
					"MATCH,LockedFinal",
				},
			}),
		},
		Processors: []domain.ProcessorSpec{mihomoMergeProcessor(t, "Tailscale 共存", `# sandrone:mihomo-preset=tailscale
dns:
  fake-ip-filter+:
    - "+.ts.net"
  nameserver-policy:
    "<+.ts.net>": 100.100.100.100
tun:
  route-exclude-address+:
    - 100.64.0.0/10
    - fd7a:115c:a1e0::/48`)},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.NotNil(t, result)
	doc := decodeMihomoCommunityPresetResult(t, result.Content)
	proxies := requireAnySlice(t, doc["proxies"])
	require.Len(t, proxies, 1)
	require.Equal(t, "External-Node", requireStringMap(t, proxies[0])["name"])
	require.NotEqual(t, "tailscale", requireStringMap(t, proxies[0])["type"])
	require.Equal(t, []any{
		"DOMAIN,user.example,DIRECT",
		"RULE-SET,private,DIRECT",
		"MATCH,LockedFinal",
	}, doc["rules"])
	dns := requireStringMap(t, doc["dns"])
	require.Equal(t, []any{"base.example", "+.ts.net"}, dns["fake-ip-filter"])
	require.Equal(t, "100.100.100.100", requireStringMap(t, dns["nameserver-policy"])["+.ts.net"])
	tun := requireStringMap(t, doc["tun"])
	require.Equal(t, []any{"192.0.2.0/24", "100.64.0.0/10", "fd7a:115c:a1e0::/48"}, tun["route-exclude-address"])
	assertNoTailscaleSecretsOrExitNode(t, doc, result.Content)
}

func TestServiceCommunityPresetMihomoTailscaleNativeRejectsIncompatibleNamedProxy(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "incompatible-node",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:example-password@example.com:8388#TAILSCALE",
	}))
	spec := domain.FileSpec{
		Name:   "mihomo-tailscale-incompatible.yaml",
		Kind:   domain.FileKindMihomo,
		Source: domain.FileSource{Type: "inline", Content: "dns: {}\ntun: {}\n"},
		Config: &domain.FileConfig{
			Subscriptions: []string{"incompatible-node"},
			Settings: completeTypedSettings(t, map[string]any{
				"rules": []string{"MATCH,LockedFinal"},
			}),
		},
		Processors: []domain.ProcessorSpec{
			mihomoTailscaleNativeProcessor(t, communityPresetRawScript(t, "mihomo-tailscale-native.js")),
		},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeScriptRuntime), "got %v", err)
	require.Contains(t, err.Error(), "Sandrone Mihomo Tailscale native preset found incompatible proxy named TAILSCALE")
}

func TestServiceCommunityPresetSingBoxOrderedScenariosUseExactRawAsset(t *testing.T) {
	script := communityPresetRawScript(t, "insert-sing-box-rules.js")
	spec := domain.FileSpec{
		Name: "sing-box-ordered-scenarios.json",
		Kind: domain.FileKindSingBox,
		Source: domain.FileSource{Type: "inline", Content: `{
			"route": {
				"final": "LockedRouteFinal",
				"rules": []
			}
		}`},
		Config: &domain.FileConfig{Settings: completeTypedSettings(t, map[string]any{
			"rules": []map[string]any{
				{"domain_suffix": []string{"user.example"}, "outbound": "direct"},
				{"rule_set": []string{"private"}, "outbound": "direct"},
				{"outbound": "LockedFinal"},
			},
		})},
		Processors: []domain.ProcessorSpec{
			orderedRuleProcessor(t, script, "quic-fallback", "QUIC Fallback", `[{"protocol":"quic","action":"reject"}]`),
		},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.NotNil(t, result)
	doc := decodeSingBoxCommunityPresetResult(t, result.Content)
	route := requireStringMap(t, doc["route"])
	require.Equal(t, "LockedRouteFinal", route["final"])
	require.Equal(t, []any{
		map[string]any{"domain_suffix": []any{"user.example"}, "outbound": "direct"},
		map[string]any{"protocol": "quic", "action": "reject"},
		map[string]any{"rule_set": []any{"private"}, "outbound": "direct"},
		map[string]any{"outbound": "LockedFinal"},
	}, route["rules"])
}

func TestServiceCommunityPresetSingBoxTailscaleNativeGeneratesFullFile(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "sing-box-native-node",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:example-password@example.com:8388#Native-Node",
	}))
	script := communityPresetRawScript(t, "sing-box-tailscale-native.js")
	processor := singBoxTailscaleProcessor(t, "Tailscale 原生接管", script, "tskey-auth-test")
	spec := domain.FileSpec{
		Name: "sing-box-tailscale-native.json",
		Kind: domain.FileKindSingBox,
		Source: domain.FileSource{Type: "inline", Content: `{
			"dns": {
				"servers": [{"type":"local","tag":"dns-local"}],
				"rules": [{"domain_suffix":["user-dns.example"],"server":"dns-local"}],
				"final": "LockedDNSFinal"
			},
			"inbounds": [{
				"type":"tun",
				"tag":"tun-in",
				"address":["172.19.0.1/30","fdfe:dcba:9876::1/126"],
				"route_exclude_address":["192.0.2.0/24","100.64.0.0/10","fd7a:115c:a1e0::/48","100.64.0.0/10"]
			}],
			"outbounds": [],
			"endpoints": [],
			"route": {"final":"LockedRouteFinal","rule_set":[],"rules":[]}
		}`},
		Config: &domain.FileConfig{
			Subscriptions: []string{"sing-box-native-node"},
			Settings: completeTypedSettings(t, map[string]any{
				"rules": []map[string]any{
					{"domain_suffix": []string{"user.example"}, "outbound": "direct"},
					{"rule_set": []string{"private"}, "outbound": "direct"},
					{"outbound": "LockedFinal"},
				},
			}),
		},
		Processors: []domain.ProcessorSpec{processor, processor},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.NotNil(t, result)
	doc := decodeSingBoxCommunityPresetResult(t, result.Content)
	outbounds := requireAnySlice(t, doc["outbounds"])
	node := requireStringMapWithField(t, outbounds, "tag", "Native-Node")
	require.Equal(t, "shadowsocks", node["type"])
	require.Equal(t, []any{
		map[string]any{
			"type":          "tailscale",
			"tag":           "ts-ep",
			"auth_key":      "tskey-auth-test",
			"ephemeral":     false,
			"accept_routes": false,
		},
	}, doc["endpoints"])
	dns := requireStringMap(t, doc["dns"])
	require.Equal(t, "LockedDNSFinal", dns["final"])
	require.Equal(t, []any{
		map[string]any{"type": "local", "tag": "dns-local"},
		map[string]any{
			"type":                     "tailscale",
			"tag":                      "ts-dns",
			"endpoint":                 "ts-ep",
			"accept_default_resolvers": false,
		},
	}, dns["servers"])
	require.Equal(t, []any{
		map[string]any{"domain_suffix": []any{"user-dns.example"}, "server": "dns-local"},
		map[string]any{"ip_accept_any": true, "server": "ts-dns"},
	}, dns["rules"])
	inbounds := requireAnySlice(t, doc["inbounds"])
	require.Equal(t, []any{"192.0.2.0/24"}, requireStringMap(t, inbounds[0])["route_exclude_address"])
	route := requireStringMap(t, doc["route"])
	require.Equal(t, "LockedRouteFinal", route["final"])
	require.Equal(t, []any{
		map[string]any{"domain_suffix": []any{"user.example"}, "outbound": "direct"},
		map[string]any{
			"preferred_by": []any{"ts-ep"},
			"action":       "route",
			"outbound":     "ts-ep",
		},
		map[string]any{"rule_set": []any{"private"}, "outbound": "direct"},
		map[string]any{"outbound": "LockedFinal"},
	}, route["rules"])
	require.NotContains(t, strings.ToLower(string(result.Content)), "exit_node")
}

func TestServiceCommunityPresetSingBoxTailscaleExternalGeneratesDistinctFullFile(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "sing-box-external-node",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:example-password@example.com:8388#External-Node",
	}))
	script := communityPresetRawScript(t, "sing-box-tailscale-external.js")
	processor := singBoxTailscaleProcessor(t, "Tailscale 共存", script)
	spec := domain.FileSpec{
		Name: "sing-box-tailscale-external.json",
		Kind: domain.FileKindSingBox,
		Source: domain.FileSource{Type: "inline", Content: `{
			"dns": {
				"servers": [{"type":"local","tag":"dns-local"}],
				"rules": [{"domain_suffix":["user-dns.example"],"server":"dns-local"}],
				"final": "LockedDNSFinal"
			},
			"inbounds": [{
				"type":"tun",
				"tag":"tun-in",
				"address":["172.19.0.1/30","fdfe:dcba:9876::1/126"],
				"route_exclude_address":["192.0.2.0/24","100.64.0.0/10","100.64.0.0/10"]
			}],
			"outbounds": [],
			"endpoints": [],
			"route": {"final":"LockedRouteFinal","rule_set":[],"rules":[]}
		}`},
		Config: &domain.FileConfig{
			Subscriptions: []string{"sing-box-external-node"},
			Settings: completeTypedSettings(t, map[string]any{
				"rules": []map[string]any{
					{"domain_suffix": []string{"user.example"}, "outbound": "direct"},
					{"rule_set": []string{"private"}, "outbound": "direct"},
					{"outbound": "LockedFinal"},
				},
			}),
		},
		Processors: []domain.ProcessorSpec{processor, processor},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.NotNil(t, result)
	doc := decodeSingBoxCommunityPresetResult(t, result.Content)
	outbounds := requireAnySlice(t, doc["outbounds"])
	node := requireStringMapWithField(t, outbounds, "tag", "External-Node")
	require.Equal(t, "shadowsocks", node["type"])
	require.Empty(t, requireAnySlice(t, doc["endpoints"]))
	dns := requireStringMap(t, doc["dns"])
	require.Equal(t, "LockedDNSFinal", dns["final"])
	require.Equal(t, []any{
		map[string]any{"type": "local", "tag": "dns-local"},
		map[string]any{"type": "udp", "tag": "ts-dns", "server": "100.100.100.100"},
	}, dns["servers"])
	require.Equal(t, []any{
		map[string]any{"domain_suffix": []any{"user-dns.example"}, "server": "dns-local"},
		map[string]any{
			"domain_suffix": []any{"ts.net"},
			"action":        "route",
			"server":        "ts-dns",
		},
	}, dns["rules"])
	inbounds := requireAnySlice(t, doc["inbounds"])
	require.Equal(t, []any{
		"192.0.2.0/24",
		"100.64.0.0/10",
		"fd7a:115c:a1e0::/48",
	}, requireStringMap(t, inbounds[0])["route_exclude_address"])
	route := requireStringMap(t, doc["route"])
	require.Equal(t, "LockedRouteFinal", route["final"])
	require.Equal(t, []any{
		map[string]any{"domain_suffix": []any{"user.example"}, "outbound": "direct"},
		map[string]any{"rule_set": []any{"private"}, "outbound": "direct"},
		map[string]any{"outbound": "LockedFinal"},
	}, route["rules"])
	assertNoTailscaleSecretsOrExitNode(t, doc, result.Content)
}

func TestServiceCommunityPresetShadowrocketTailscaleNativeGeneratesConfigOnlyFile(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	script := communityPresetRawScript(t, "insert-shadowrocket-rules.js")
	processor := orderedRuleProcessor(t, script, "tailscale-native", "Tailscale 原生接管", `[
		"DOMAIN-SUFFIX,ts.net,TAILSCALE",
		"IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve",
		"IP-CIDR,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve"
	]`)
	spec := domain.FileSpec{
		Name:   "shadowrocket-tailscale-native.conf",
		Kind:   domain.FileKindShadowrocket,
		Source: domain.FileSource{Type: "inline", Content: "[General]\nprofile = keep\n"},
		Config: &domain.FileConfig{
			Settings: completeTypedSettings(t, map[string]any{
				"groups": []map[string]any{
					{"name": "Proxy", "type": "select", "proxies": []string{"PROXY", "DIRECT"}},
				},
				"rules": []string{
					"DOMAIN,user.example,DIRECT",
					"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
					"FINAL,Proxy",
				},
			}),
		},
		Processors: []domain.ProcessorSpec{processor, processor},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.NotNil(t, result)
	model, err := inidoc.ParseModel(result.Content)
	require.NoError(t, err)
	require.Empty(t, modelSectionLines(t, model, "Proxy"))
	require.Equal(t, []string{
		"DOMAIN-SUFFIX,ts.net,TAILSCALE",
		"IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve",
		"IP-CIDR,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve",
		"DOMAIN,user.example,DIRECT",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
		"FINAL,Proxy",
	}, modelSectionLines(t, model, "Rule"))
	require.NotContains(t, strings.ToLower(string(result.Content)), "module")
	assertNoTailscaleSecretsOrExitNode(t, map[string]any{}, result.Content)
}

func TestServiceCommunityPresetShadowrocketTailscaleExternalGeneratesFullFile(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	script := communityPresetRawScript(t, "shadowrocket-tailscale-external.js")
	processor := domain.ProcessorSpec{
		Name:  "Tailscale 共存",
		Type:  "script",
		Stage: domain.StageFile,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(script),
		}),
	}
	spec := domain.FileSpec{
		Name: "shadowrocket-tailscale-external.conf",
		Kind: domain.FileKindShadowrocket,
		Source: domain.FileSource{Type: "inline", Content: "[General]\n" +
			"profile = keep\n" +
			"skip-proxy = 192.168.0.0/16,100.64.0.0/10\n" +
			"tun-excluded-routes = 192.168.0.0/16\n\n" +
			"[Host]\nexample.com = 192.0.2.1\n"},
		Config: &domain.FileConfig{
			Settings: completeTypedSettings(t, map[string]any{
				"groups": []map[string]any{
					{"name": "Proxy", "type": "select", "proxies": []string{"PROXY", "DIRECT"}},
				},
				"rules": []string{
					"DOMAIN,user.example,DIRECT",
					"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
					"FINAL,Proxy",
				},
			}),
		},
		Processors: []domain.ProcessorSpec{processor, processor},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.NotNil(t, result)
	model, err := inidoc.ParseModel(result.Content)
	require.NoError(t, err)
	require.Empty(t, modelSectionLines(t, model, "Proxy"))
	require.Equal(t, []string{
		"profile = keep",
		"skip-proxy = 192.168.0.0/16,100.64.0.0/10,fd7a:115c:a1e0::/48",
		"tun-excluded-routes = 192.168.0.0/16,100.64.0.0/10,fd7a:115c:a1e0::/48",
		"",
	}, modelSectionLines(t, model, "General"))
	require.Equal(t, []string{
		"example.com = 192.0.2.1",
	}, modelSectionLines(t, model, "Host"))
	require.Equal(t, []string{
		"DOMAIN-SUFFIX,ts.net,DIRECT",
		"IP-CIDR,100.64.0.0/10,DIRECT,no-resolve",
		"IP-CIDR,fd7a:115c:a1e0::/48,DIRECT,no-resolve",
		"DOMAIN,user.example,DIRECT",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
		"FINAL,Proxy",
	}, modelSectionLines(t, model, "Rule"))
	require.Contains(t, modelSectionLines(t, model, "General")[2], "100.64.0.0/10")
	assertNoTailscaleSecretsOrExitNode(t, map[string]any{}, result.Content)
}

func TestServiceCommunityPresetOrderedRulesRejectNoSafeAnchorWithoutPartial(t *testing.T) {
	tests := []struct {
		name      string
		asset     string
		kind      domain.FileKind
		filename  string
		rulesJSON string
		settings  map[string]any
		errorKind string
	}{
		{
			name:      "Mihomo",
			asset:     "insert-mihomo-rules.js",
			kind:      domain.FileKindMihomo,
			filename:  "no-anchor.yaml",
			rulesJSON: `["AND,((NETWORK,UDP),(DST-PORT,443)),REJECT"]`,
			settings:  map[string]any{"rules": []string{"DOMAIN,service.example,DIRECT"}},
			errorKind: "mihomo",
		},
		{
			name:      "SingBox",
			asset:     "insert-sing-box-rules.js",
			kind:      domain.FileKindSingBox,
			filename:  "no-anchor.json",
			rulesJSON: `[{"protocol":"quic","action":"reject"}]`,
			settings: map[string]any{"rules": []map[string]any{
				{"domain_suffix": []string{"service.example"}, "outbound": "direct"},
			}},
			errorKind: "sing-box",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := domain.FileSpec{
				Name:   test.filename,
				Kind:   test.kind,
				Config: &domain.FileConfig{Settings: completeTypedSettings(t, test.settings)},
				Processors: []domain.ProcessorSpec{orderedRuleProcessor(
					t,
					communityPresetRawScript(t, test.asset),
					"quic-fallback",
					"QUIC Fallback",
					test.rulesJSON,
				)},
			}

			result, err := service.New().GetFile(t.Context(), domain.FileRequest{Spec: &spec})

			require.Nil(t, result)
			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeScriptRuntime), "got %v", err)
			require.Contains(t, err.Error(), "Sandrone preset quic-fallback cannot find a safe "+test.errorKind+" rule anchor")
		})
	}
}

func TestServiceCommunityPresetOrderedRulesRejectManagedRequestArgOverrides(t *testing.T) {
	tests := []struct {
		name          string
		asset         string
		kind          domain.FileKind
		filename      string
		presetID      string
		processorName string
		rulesJSON     string
		settings      map[string]any
	}{
		{
			name:          "Mihomo",
			asset:         "insert-mihomo-rules.js",
			kind:          domain.FileKindMihomo,
			filename:      "managed-args.yaml",
			presetID:      "quic-fallback",
			processorName: "QUIC Fallback",
			rulesJSON:     `["AND,((NETWORK,UDP),(DST-PORT,443)),REJECT"]`,
			settings: map[string]any{"rules": []string{
				"RULE-SET,private,DIRECT",
				"MATCH,Proxy",
			}},
		},
		{
			name:          "SingBox",
			asset:         "insert-sing-box-rules.js",
			kind:          domain.FileKindSingBox,
			filename:      "managed-args.json",
			presetID:      "quic-fallback",
			processorName: "QUIC Fallback",
			rulesJSON:     `[{"protocol":"quic","action":"reject"}]`,
			settings: map[string]any{"rules": []map[string]any{
				{"rule_set": []string{"private"}, "outbound": "direct"},
				{"outbound": "Proxy"},
			}},
		},
		{
			name:          "Shadowrocket",
			asset:         "insert-shadowrocket-rules.js",
			kind:          domain.FileKindShadowrocket,
			filename:      "managed-args.conf",
			presetID:      "tailscale-native",
			processorName: "Tailscale 原生接管",
			rulesJSON: `[
				"DOMAIN-SUFFIX,ts.net,TAILSCALE",
				"IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve",
				"IP-CIDR,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve"
			]`,
			settings: map[string]any{
				"groups": []map[string]any{{"name": "Proxy", "type": "select", "proxies": []string{"DIRECT"}}},
				"rules": []string{
					"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
					"FINAL,Proxy",
				},
			},
		},
	}
	overrides := []struct {
		name  string
		key   string
		value string
	}{
		{name: "rules_json", key: "rules_json", value: `[]`},
		{name: "preset_id", key: "preset_id", value: "request-controlled"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, override := range overrides {
				t.Run(override.name, func(t *testing.T) {
					spec := domain.FileSpec{
						Name:   test.filename,
						Kind:   test.kind,
						Config: &domain.FileConfig{Settings: completeTypedSettings(t, test.settings)},
						Processors: []domain.ProcessorSpec{orderedRuleProcessor(
							t,
							communityPresetRawScript(t, test.asset),
							test.presetID,
							test.processorName,
							test.rulesJSON,
						)},
					}

					result, err := service.New().GetFile(t.Context(), domain.FileRequest{
						Spec: &spec,
						Request: domain.RequestInfo{Args: map[string]string{
							override.key: override.value,
						}},
					})

					require.Error(t, err)
					require.Nil(t, result)
					require.True(t, domain.IsCode(err, domain.CodeScriptRuntime), "got %v", err)
					require.Contains(t, err.Error(), "Sandrone preset arguments cannot be overridden by request args")
				})
			}
		})
	}
}

func orderedRuleProcessor(t *testing.T, script, presetID, name, rulesJSON string) domain.ProcessorSpec {
	t.Helper()
	return domain.ProcessorSpec{
		Name:  name,
		Type:  "script",
		Stage: domain.StageFile,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(script),
			"args": map[string]any{
				"preset_id":  presetID,
				"rules_json": rulesJSON,
			},
		}),
	}
}

func mihomoMergeProcessor(t *testing.T, name, content string) domain.ProcessorSpec {
	t.Helper()
	return domain.ProcessorSpec{
		Name:  name,
		Type:  "merge",
		Stage: domain.StageFile,
		Params: params(t, map[string]any{
			"mode":    "yaml_override",
			"content": content,
		}),
	}
}

func mihomoTailscaleNativeProcessor(t *testing.T, script string, authKeys ...string) domain.ProcessorSpec {
	t.Helper()
	authKey := ""
	if len(authKeys) > 0 {
		authKey = authKeys[0]
	}
	return domain.ProcessorSpec{
		Name:  "Tailscale 原生接管",
		Type:  "script",
		Stage: domain.StageFile,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(script),
			"args":   map[string]any{"auth_key": authKey},
		}),
	}
}

func singBoxTailscaleProcessor(t *testing.T, name, script string, authKeys ...string) domain.ProcessorSpec {
	t.Helper()
	paramsValue := map[string]any{"source": inlineScriptSource(script)}
	if len(authKeys) > 0 {
		paramsValue["args"] = map[string]any{"auth_key": authKeys[0]}
	}
	return domain.ProcessorSpec{
		Name:   name,
		Type:   "script",
		Stage:  domain.StageFile,
		Params: params(t, paramsValue),
	}
}

func decodeMihomoCommunityPresetResult(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(body, &doc))
	return doc
}

func decodeSingBoxCommunityPresetResult(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	return doc
}

func requireAnySlice(t *testing.T, value any) []any {
	t.Helper()
	items, ok := value.([]any)
	require.True(t, ok, "expected []any, got %T", value)
	return items
}

func requireStringMap(t *testing.T, value any) map[string]any {
	t.Helper()
	item, ok := value.(map[string]any)
	require.True(t, ok, "expected map[string]any, got %T", value)
	return item
}

func requireStringMapWithField(t *testing.T, values []any, field string, expected any) map[string]any {
	t.Helper()
	for _, value := range values {
		item, ok := value.(map[string]any)
		if ok && item[field] == expected {
			return item
		}
	}
	require.FailNow(t, "missing object with field", "%s=%v", field, expected)
	return nil
}

func hasCaseInsensitiveKeyFragment(value any, fragment string) bool {
	fragment = strings.ToLower(fragment)
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if strings.Contains(strings.ToLower(key), fragment) || hasCaseInsensitiveKeyFragment(child, fragment) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if hasCaseInsensitiveKeyFragment(child, fragment) {
				return true
			}
		}
	}
	return false
}

func assertNoTailscaleSecretsOrExitNode(t *testing.T, doc map[string]any, body []byte) {
	t.Helper()
	for _, fragment := range []string{
		"auth_key",
		"auth-key",
		"control_url",
		"control-url",
		"headscale",
		"exit_node",
		"exit-node",
		"exit node",
	} {
		require.False(t, hasCaseInsensitiveKeyFragment(doc, fragment), "unexpected key fragment %q", fragment)
		require.NotContains(t, strings.ToLower(string(body)), fragment)
	}
}

func modelSectionLines(t *testing.T, model inidoc.Model, name string) []string {
	t.Helper()
	for _, section := range model.Sections {
		if section.Name == name {
			return section.Lines
		}
	}
	require.FailNow(t, "missing INI section", "section %q", name)
	return nil
}

func communityPresetRawScript(t *testing.T, name string) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(testFile), "..", "..", "web", "app", "features", "files", "processors", "scripts", name)
	body, err := os.ReadFile(path)
	require.NoError(t, err, "read exact community preset asset %s", path)
	return string(body)
}
