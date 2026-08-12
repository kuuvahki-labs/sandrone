package service_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/inidoc"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceCommunityPresetOrderedNTPUsesExactRawAssets(t *testing.T) {
	tests := []struct {
		name      string
		asset     string
		kind      domain.FileKind
		filename  string
		source    string
		rulesJSON string
		settings  map[string]any
		assert    func(*testing.T, []byte)
	}{
		{
			name:      "Mihomo",
			asset:     "insert-mihomo-rules.js",
			kind:      domain.FileKindMihomo,
			filename:  "ordered-ntp.yaml",
			source:    "{}\n",
			rulesJSON: `["AND,((NETWORK,UDP),(DST-PORT,123)),DIRECT"]`,
			settings: map[string]any{"rules": []string{
				"DOMAIN,service.example,DIRECT",
				"RULE-SET,private,DIRECT",
				"MATCH,Proxy",
			}},
			assert: assertMihomoOrderedNTP,
		},
		{
			name:      "SingBox",
			asset:     "insert-sing-box-rules.js",
			kind:      domain.FileKindSingBox,
			filename:  "ordered-ntp.json",
			source:    "{}",
			rulesJSON: `[{"network":"udp","port":123,"outbound":"direct"}]`,
			settings: map[string]any{"rules": []map[string]any{
				{"domain_suffix": []string{"service.example"}, "outbound": "direct"},
				{"rule_set": []string{"private"}, "outbound": "direct"},
				{"outbound": "Proxy"},
			}},
			assert: assertSingBoxOrderedNTP,
		},
		{
			name:      "Shadowrocket",
			asset:     "insert-shadowrocket-rules.js",
			kind:      domain.FileKindShadowrocket,
			filename:  "ordered-ntp.conf",
			source:    "[General]\r\nprofile = keep\r\n[Rule]\r\nFINAL,old\r\n[Host]\r\nexample.com = 192.0.2.1\r\n[Rule]\r\nFINAL,new\r\n",
			rulesJSON: `["AND,((PROTOCOL,UDP),(DST-PORT,123)),DIRECT"]`,
			settings: map[string]any{
				"groups": []map[string]any{{"name": "Proxy", "type": "select", "proxies": []string{"DIRECT"}}},
				"rules": []string{
					"DOMAIN,service.example,DIRECT",
					"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
					"FINAL,Proxy",
				},
			},
			assert: assertShadowrocketOrderedNTP,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			script := communityPresetRawScript(t, test.asset)
			processor := orderedNTPProcessor(t, script, test.rulesJSON)
			spec := domain.FileSpec{
				Name:   test.filename,
				Kind:   test.kind,
				Source: domain.FileSource{Type: "inline", Content: test.source},
				Config: &domain.FileConfig{Settings: raw(t, test.settings)},
				Processors: []domain.ProcessorSpec{
					processor,
					processor,
				},
			}

			result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

			require.NoError(t, err)
			require.NotNil(t, result)
			test.assert(t, result.Content)
		})
	}
}

func TestServiceCommunityPresetMihomoOrderedScenariosUseExactRawAsset(t *testing.T) {
	script := communityPresetRawScript(t, "insert-mihomo-rules.js")
	spec := domain.FileSpec{
		Name:   "mihomo-scenarios.yaml",
		Kind:   domain.FileKindMihomo,
		Source: domain.FileSource{Type: "inline", Content: "{}\n"},
		Config: &domain.FileConfig{Settings: raw(t, map[string]any{
			"rules": []string{
				"RULE-SET,private,DIRECT",
				"MATCH,Proxy",
			},
		})},
		Processors: []domain.ProcessorSpec{
			orderedRuleProcessor(t, script, "stun-block", "STUN Block", `[
				"AND,((NETWORK,UDP),(DST-PORT,3478)),REJECT",
				"AND,((NETWORK,UDP),(DST-PORT,5349)),REJECT"
			]`),
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
		"AND,((NETWORK,UDP),(DST-PORT,3478)),REJECT",
		"AND,((NETWORK,UDP),(DST-PORT,5349)),REJECT",
		"AND,((NETWORK,UDP),(DST-PORT,443)),REJECT",
		"RULE-SET,private,DIRECT",
		"MATCH,Proxy",
	}, doc["rules"])
	require.NotEqual(t, "off", doc["find-process-mode"])
	require.False(t, hasCaseInsensitiveKeyFragment(doc, "keepalive"))
}

func TestServiceCommunityPresetMihomoMergeScenariosApplyExactFields(t *testing.T) {
	tests := []struct {
		name            string
		processorName   string
		content         string
		wantTun         map[string]any
		wantProcessMode string
	}{
		{
			name:          "UDP P2P EIM",
			processorName: "UDP/P2P EIM",
			content: `# sandrone:mihomo-preset=udp-p2p-eim
tun:
  endpoint-independent-nat: true`,
			wantTun: map[string]any{"endpoint-independent-nat": true},
		},
		{
			name:          "Linux TUN acceleration",
			processorName: "Linux/OpenWrt TUN Acceleration",
			content: `# sandrone:mihomo-preset=linux-tun-acceleration
find-process-mode: strict
tun:
  auto-route: true
  auto-redirect: true`,
			wantTun: map[string]any{
				"auto-route":    true,
				"auto-redirect": true,
			},
			wantProcessMode: "strict",
		},
		{
			name:          "Windows relaxed route",
			processorName: "Windows Relaxed Route",
			content: `# sandrone:mihomo-preset=windows-relaxed-route
tun:
  strict-route: false`,
			wantTun: map[string]any{"strict-route": false},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := domain.FileSpec{
				Name:   "mihomo-merge-scenario.yaml",
				Kind:   domain.FileKindMihomo,
				Source: domain.FileSource{Type: "inline", Content: "{}\n"},
				Config: &domain.FileConfig{Settings: raw(t, map[string]any{
					"rules": []string{"MATCH,Proxy"},
				})},
				Processors: []domain.ProcessorSpec{
					mihomoMergeProcessor(t, test.processorName, test.content),
				},
			}

			result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

			require.NoError(t, err)
			require.NotNil(t, result)
			doc := decodeMihomoCommunityPresetResult(t, result.Content)
			require.Equal(t, test.wantTun, doc["tun"])
			if test.wantProcessMode == "" {
				require.NotContains(t, doc, "find-process-mode")
			} else {
				require.Equal(t, test.wantProcessMode, doc["find-process-mode"])
			}
			require.NotEqual(t, "off", doc["find-process-mode"])
			require.False(t, hasCaseInsensitiveKeyFragment(doc, "keepalive"))
		})
	}
}

func TestServiceCommunityPresetSingBoxStructureScenariosUseExactRawAsset(t *testing.T) {
	script := communityPresetRawScript(t, "update-sing-box-tun.js")
	base := singBoxStructureScenarioBase()
	original := decodeSingBoxCommunityPresetResult(t, []byte(base))
	originalInbounds := requireAnySlice(t, original["inbounds"])
	originalDNS := requireStringMap(t, original["dns"])

	tests := []struct {
		operation string
		assertTun func(*testing.T, map[string]any)
	}{
		{
			operation: "ensure-tun",
			assertTun: func(t *testing.T, tun map[string]any) {
				require.Equal(t, originalInbounds[2], tun)
			},
		},
		{
			operation: "ipv4-only",
			assertTun: func(t *testing.T, tun map[string]any) {
				require.Equal(t, []any{"172.19.0.1/30"}, tun["address"])
				require.Equal(t, originalInbounds[2].(map[string]any)["custom"], tun["custom"])
				require.Equal(t, originalInbounds[2].(map[string]any)["route_exclude_address"], tun["route_exclude_address"])
			},
		},
		{
			operation: "udp-p2p-eim",
			assertTun: func(t *testing.T, tun map[string]any) {
				require.True(t, tun["endpoint_independent_nat"].(bool))
			},
		},
		{
			operation: "linux-tun-acceleration",
			assertTun: func(t *testing.T, tun map[string]any) {
				require.True(t, tun["auto_route"].(bool))
				require.True(t, tun["auto_redirect"].(bool))
			},
		},
		{
			operation: "mptcp-direct",
			assertTun: func(t *testing.T, tun map[string]any) {
				require.True(t, tun["exclude_mptcp"].(bool))
			},
		},
		{
			operation: "windows-relaxed-route",
			assertTun: func(t *testing.T, tun map[string]any) {
				require.False(t, tun["strict_route"].(bool))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.operation, func(t *testing.T) {
			spec := domain.FileSpec{
				Name:   "sing-box-" + test.operation + ".json",
				Kind:   domain.FileKindSingBox,
				Source: domain.FileSource{Type: "inline", Content: base},
				Config: &domain.FileConfig{Settings: raw(t, map[string]any{
					"rules": []map[string]any{{"outbound": "LockedFinal"}},
				})},
				Processors: []domain.ProcessorSpec{singBoxStructureProcessor(t, script, test.operation)},
			}

			result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

			require.NoError(t, err)
			require.NotNil(t, result)
			doc := decodeSingBoxCommunityPresetResult(t, result.Content)
			inbounds := requireAnySlice(t, doc["inbounds"])
			require.Len(t, inbounds, 3)
			require.Equal(t, originalInbounds[0], inbounds[0], "mixed inbound changed or moved")
			require.Equal(t, originalInbounds[1], inbounds[1], "custom inbound changed or moved")
			tun := requireStringMap(t, inbounds[2])
			test.assertTun(t, tun)
			require.Equal(t, original["experimental"], doc["experimental"])

			dns := requireStringMap(t, doc["dns"])
			require.Equal(t, originalDNS["servers"], dns["servers"])
			require.Equal(t, originalDNS["rules"], dns["rules"])
			if test.operation == "ipv4-only" {
				require.Equal(t, "ipv4_only", dns["strategy"])
			} else {
				require.Equal(t, originalDNS["strategy"], dns["strategy"])
			}
		})
	}
}

func TestServiceCommunityPresetSingBoxEnsureTunAppendsOnlyWhenAbsent(t *testing.T) {
	script := communityPresetRawScript(t, "update-sing-box-tun.js")
	spec := domain.FileSpec{
		Name: "sing-box-ensure-tun.json",
		Kind: domain.FileKindSingBox,
		Source: domain.FileSource{Type: "inline", Content: `{
			"inbounds": [
				{"type":"mixed","tag":"mixed-in","listen":"::1"},
				{"type":"direct","tag":"custom-in","metadata":{"ipv6":"2001:db8::8"}}
			],
			"route":{"rules":[]}
		}`},
		Config: &domain.FileConfig{Settings: raw(t, map[string]any{
			"rules": []map[string]any{{"outbound": "LockedFinal"}},
		})},
		Processors: []domain.ProcessorSpec{
			singBoxStructureProcessor(t, script, "ensure-tun"),
			singBoxStructureProcessor(t, script, "ensure-tun"),
		},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.NotNil(t, result)
	doc := decodeSingBoxCommunityPresetResult(t, result.Content)
	inbounds := requireAnySlice(t, doc["inbounds"])
	require.Equal(t, []any{
		map[string]any{"type": "mixed", "tag": "mixed-in", "listen": "::1"},
		map[string]any{"type": "direct", "tag": "custom-in", "metadata": map[string]any{"ipv6": "2001:db8::8"}},
		map[string]any{
			"type":         "tun",
			"tag":          "tun-in",
			"address":      []any{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
			"auto_route":   true,
			"strict_route": true,
		},
	}, inbounds)
}

func TestServiceCommunityPresetSingBoxStructureRejectsAmbiguousTunWithoutPartial(t *testing.T) {
	script := communityPresetRawScript(t, "update-sing-box-tun.js")
	spec := domain.FileSpec{
		Name: "sing-box-ambiguous-tun.json",
		Kind: domain.FileKindSingBox,
		Source: domain.FileSource{Type: "inline", Content: `{
			"inbounds": [
				{"type":"tun","tag":"first"},
				{"type":"tun","tag":"second"}
			],
			"route":{"rules":[]}
		}`},
		Config: &domain.FileConfig{Settings: raw(t, map[string]any{
			"rules": []map[string]any{{"outbound": "LockedFinal"}},
		})},
		Processors: []domain.ProcessorSpec{singBoxStructureProcessor(t, script, "ensure-tun")},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeScriptRuntime), "got %v", err)
	require.Contains(t, err.Error(), "Sandrone sing-box structure preset found ambiguous TUN inbounds")
}

func TestServiceCommunityPresetSingBoxStructureRejectsManagedOperationOverride(t *testing.T) {
	script := communityPresetRawScript(t, "update-sing-box-tun.js")
	spec := domain.FileSpec{
		Name:   "sing-box-managed-operation.json",
		Kind:   domain.FileKindSingBox,
		Source: domain.FileSource{Type: "inline", Content: singBoxStructureScenarioBase()},
		Config: &domain.FileConfig{Settings: raw(t, map[string]any{
			"rules": []map[string]any{{"outbound": "LockedFinal"}},
		})},
		Processors: []domain.ProcessorSpec{singBoxStructureProcessor(t, script, "mptcp-direct")},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{
		Spec: &spec,
		Request: domain.RequestInfo{Args: map[string]string{
			"operation": "windows-relaxed-route",
		}},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeScriptRuntime), "got %v", err)
	require.Contains(t, err.Error(), "Sandrone sing-box structure preset operation cannot be overridden by request args")
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
		Config: &domain.FileConfig{Settings: raw(t, map[string]any{
			"rules": []map[string]any{
				{"domain_suffix": []string{"user.example"}, "outbound": "direct"},
				{"rule_set": []string{"private"}, "outbound": "direct"},
				{"outbound": "LockedFinal"},
			},
		})},
		Processors: []domain.ProcessorSpec{
			orderedRuleProcessor(t, script, "stun-block", "STUN Block", `[{"protocol":"stun","action":"reject"}]`),
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
		map[string]any{"protocol": "stun", "action": "reject"},
		map[string]any{"protocol": "quic", "action": "reject"},
		map[string]any{"rule_set": []any{"private"}, "outbound": "direct"},
		map[string]any{"outbound": "LockedFinal"},
	}, route["rules"])
}

func TestServiceCommunityPresetOrderedNTPRejectsNoSafeAnchorWithoutPartial(t *testing.T) {
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
			rulesJSON: `["AND,((NETWORK,UDP),(DST-PORT,123)),DIRECT"]`,
			settings:  map[string]any{"rules": []string{"DOMAIN,service.example,DIRECT"}},
			errorKind: "mihomo",
		},
		{
			name:      "SingBox",
			asset:     "insert-sing-box-rules.js",
			kind:      domain.FileKindSingBox,
			filename:  "no-anchor.json",
			rulesJSON: `[{"network":"udp","port":123,"outbound":"direct"}]`,
			settings: map[string]any{"rules": []map[string]any{
				{"domain_suffix": []string{"service.example"}, "outbound": "direct"},
			}},
			errorKind: "sing-box",
		},
		{
			name:      "Shadowrocket",
			asset:     "insert-shadowrocket-rules.js",
			kind:      domain.FileKindShadowrocket,
			filename:  "no-anchor.conf",
			rulesJSON: `["AND,((PROTOCOL,UDP),(DST-PORT,123)),DIRECT"]`,
			settings: map[string]any{
				"groups": []map[string]any{{"name": "Proxy", "type": "select", "proxies": []string{"DIRECT"}}},
				"rules":  []string{"DOMAIN,service.example,DIRECT"},
			},
			errorKind: "shadowrocket",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := domain.FileSpec{
				Name:       test.filename,
				Kind:       test.kind,
				Config:     &domain.FileConfig{Settings: raw(t, test.settings)},
				Processors: []domain.ProcessorSpec{orderedNTPProcessor(t, communityPresetRawScript(t, test.asset), test.rulesJSON)},
			}

			result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

			require.Nil(t, result)
			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeScriptRuntime), "got %v", err)
			require.Contains(t, err.Error(), "Sandrone preset ntp-direct cannot find a safe "+test.errorKind+" rule anchor")
		})
	}
}

func TestServiceCommunityPresetOrderedNTPRejectsManagedRequestArgOverrides(t *testing.T) {
	tests := []struct {
		name      string
		asset     string
		kind      domain.FileKind
		filename  string
		rulesJSON string
		settings  map[string]any
	}{
		{
			name:      "Mihomo",
			asset:     "insert-mihomo-rules.js",
			kind:      domain.FileKindMihomo,
			filename:  "managed-args.yaml",
			rulesJSON: `["AND,((NETWORK,UDP),(DST-PORT,123)),DIRECT"]`,
			settings: map[string]any{"rules": []string{
				"RULE-SET,private,DIRECT",
				"MATCH,Proxy",
			}},
		},
		{
			name:      "SingBox",
			asset:     "insert-sing-box-rules.js",
			kind:      domain.FileKindSingBox,
			filename:  "managed-args.json",
			rulesJSON: `[{"network":"udp","port":123,"outbound":"direct"}]`,
			settings: map[string]any{"rules": []map[string]any{
				{"rule_set": []string{"private"}, "outbound": "direct"},
				{"outbound": "Proxy"},
			}},
		},
		{
			name:      "Shadowrocket",
			asset:     "insert-shadowrocket-rules.js",
			kind:      domain.FileKindShadowrocket,
			filename:  "managed-args.conf",
			rulesJSON: `["AND,((PROTOCOL,UDP),(DST-PORT,123)),DIRECT"]`,
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
						Name:       test.filename,
						Kind:       test.kind,
						Config:     &domain.FileConfig{Settings: raw(t, test.settings)},
						Processors: []domain.ProcessorSpec{orderedNTPProcessor(t, communityPresetRawScript(t, test.asset), test.rulesJSON)},
					}

					result, err := service.New().GetFile(context.Background(), domain.FileRequest{
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

func orderedNTPProcessor(t *testing.T, script, rulesJSON string) domain.ProcessorSpec {
	t.Helper()
	return orderedRuleProcessor(t, script, "ntp-direct", "Traditional NTP Direct", rulesJSON)
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

func singBoxStructureProcessor(t *testing.T, script, operation string) domain.ProcessorSpec {
	t.Helper()
	return domain.ProcessorSpec{
		Name:  operation,
		Type:  "script",
		Stage: domain.StageFile,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(script),
			"args":   map[string]any{"operation": operation},
		}),
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

func singBoxStructureScenarioBase() string {
	return `{
		"dns": {
			"strategy": "prefer_ipv4",
			"servers": [{"type":"udp","tag":"dns-v6","server":"2001:db8::53"}],
			"rules": [{"domain_suffix":["v6.example"],"server":"dns-v6"}]
		},
		"inbounds": [
			{
				"type":"mixed",
				"tag":"mixed-in",
				"listen":"::1",
				"listen_port":2080,
				"users":[{"username":"keep","password":"example"}]
			},
			{
				"type":"direct",
				"tag":"custom-in",
				"listen":"2001:db8::2",
				"network":["tcp","udp"],
				"custom":{"cidr":"2001:db8:1::/64"}
			},
			{
				"type":"tun",
				"tag":"tun-in",
				"address":["172.19.0.1/30","fdfe:dcba:9876::1/126"],
				"auto_route":false,
				"strict_route":true,
				"route_exclude_address":["2001:db8:ffff::/48"],
				"custom":{"ipv6":"2001:db8::9"}
			}
		],
		"experimental":{"unrelated_ipv6":"2001:db8::10"},
		"route":{"final":"LockedFinal","rule_set":[],"rules":[]}
	}`
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

func communityPresetRawScript(t *testing.T, name string) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(testFile), "..", "..", "web", "app", "features", "files", "processors", "scripts", name)
	body, err := os.ReadFile(path)
	require.NoError(t, err, "read exact community preset asset %s", path)
	return string(body)
}

func assertMihomoOrderedNTP(t *testing.T, body []byte) {
	t.Helper()
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(body, &doc))
	require.Equal(t, []any{
		"DOMAIN,service.example,DIRECT",
		"AND,((NETWORK,UDP),(DST-PORT,123)),DIRECT",
		"RULE-SET,private,DIRECT",
		"MATCH,Proxy",
	}, doc["rules"])
}

func assertSingBoxOrderedNTP(t *testing.T, body []byte) {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	route, ok := doc["route"].(map[string]any)
	require.True(t, ok)
	rules, ok := route["rules"].([]any)
	require.True(t, ok)
	require.Len(t, rules, 4)
	require.Equal(t, map[string]any{
		"domain_suffix": []any{"service.example"},
		"outbound":      "direct",
	}, rules[0])
	require.Equal(t, map[string]any{
		"network":  "udp",
		"port":     float64(123),
		"outbound": "direct",
	}, rules[1])
	require.Equal(t, map[string]any{
		"rule_set": []any{"private"},
		"outbound": "direct",
	}, rules[2])
	require.Equal(t, map[string]any{"outbound": "Proxy"}, rules[3])
}

func assertShadowrocketOrderedNTP(t *testing.T, body []byte) {
	t.Helper()
	model, err := inidoc.ParseModel(body)
	require.NoError(t, err)
	var ruleSections [][]string
	for _, section := range model.Sections {
		if strings.EqualFold(section.Name, "Rule") {
			ruleSections = append(ruleSections, section.Lines)
		}
	}
	require.Equal(t, [][]string{{
		"DOMAIN,service.example,DIRECT",
		"AND,((PROTOCOL,UDP),(DST-PORT,123)),DIRECT",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
		"FINAL,Proxy",
	}}, ruleSections)
	require.Equal(t, "\r\n", model.Newline)
	require.Equal(t, []string{"General", "Rule", "Host", "Proxy", "Proxy Group"}, modelSectionNames(model))
	require.Equal(t, []string{"profile = keep"}, model.Sections[0].Lines)
	require.Equal(t, []string{"example.com = 192.0.2.1"}, model.Sections[2].Lines)
}

func modelSectionNames(model inidoc.Model) []string {
	names := make([]string, len(model.Sections))
	for index, section := range model.Sections {
		names[index] = section.Name
	}
	return names
}
