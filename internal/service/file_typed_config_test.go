package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceStaticFileKindRemainsCompatible(t *testing.T) {
	spec := domain.FileSpec{
		Name:   "plain.txt",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "hello"},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Equal(t, "hello", string(result.Content))
	require.Equal(t, "static", result.File.Kind)
}

func TestServiceMihomoFileGeneratesCompleteConfig(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "default",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#hk-node",
	}))
	spec := domain.FileSpec{
		Name:   "default.yaml",
		Kind:   domain.FileKindMihomo,
		Source: domain.FileSource{},
		Config: &domain.FileConfig{
			Subscriptions: []string{"default"},
		},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Equal(t, "application/yaml", result.ContentType)
	require.Equal(t, "mihomo", result.File.Kind)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(result.Content, &doc))
	proxies := doc["proxies"].([]any)
	require.Len(t, proxies, 1)
	require.Equal(t, "hk-node", proxies[0].(map[string]any)["name"])
	groups := doc["proxy-groups"].([]any)
	require.NotEmpty(t, groups)
	require.Equal(t, "Proxy", groups[0].(map[string]any)["name"])
	require.Equal(t, "Auto", groups[1].(map[string]any)["name"])
	require.Equal(t, "https://cp.cloudflare.com", groups[1].(map[string]any)["url"])
	require.Contains(t, doc, "rule-providers")
	rules := doc["rules"].([]any)
	require.Contains(t, rules, "RULE-SET,private,DIRECT")
	require.Contains(t, rules, "MATCH,Proxy")
}

func TestServiceMihomoFileUsesExplicitGroupsRuleSetsAndRules(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "default",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#hk-node",
	}))
	spec := domain.FileSpec{
		Name:   "custom.yaml",
		Kind:   domain.FileKindMihomo,
		Source: domain.FileSource{},
		Config: &domain.FileConfig{
			Subscriptions: []string{"default"},
			Settings: raw(t, map[string]any{
				"groups": []map[string]any{{
					"name":    "Manual",
					"type":    "select",
					"proxies": []any{"hk-node", "DIRECT"},
				}},
				"rule_sets": []map[string]any{{
					"name":     "manual",
					"type":     "inline",
					"behavior": "classical",
					"payload":  []any{"DOMAIN-SUFFIX,example.com"},
				}},
				"rules": []string{"RULE-SET,manual,Manual", "MATCH,DIRECT"},
			}),
		},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(result.Content, &doc))
	require.Equal(t, []any{map[string]any{
		"name":    "Manual",
		"type":    "select",
		"proxies": []any{"hk-node", "DIRECT"},
	}}, doc["proxy-groups"])
	require.Equal(t, map[string]any{
		"manual": map[string]any{
			"type":     "inline",
			"behavior": "classical",
			"payload":  []any{"DOMAIN-SUFFIX,example.com"},
		},
	}, doc["rule-providers"])
	require.Equal(t, []any{"RULE-SET,manual,Manual", "MATCH,DIRECT"}, doc["rules"])
}

func TestServiceMihomoYAMLOverridePresetsRunAfterTypedCompilation(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "default",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#hk-node",
	}))
	preset := func(content string) domain.ProcessorSpec {
		return domain.ProcessorSpec{Type: "merge", Stage: domain.StageFile, Params: params(t, map[string]any{
			"mode": "yaml_override", "content": content,
		})}
	}
	spec := domain.FileSpec{
		Name: "custom.yaml",
		Kind: domain.FileKindMihomo,
		Source: domain.FileSource{Type: "inline", Content: `allow-lan: true
lan-allowed-ips:
  - 10.0.0.0/8
dns:
  fake-ip-filter:
    - base
proxies: []
proxy-groups: []
rule-providers: {}
rules: []`},
		Config: &domain.FileConfig{
			Subscriptions: []string{"default"},
			Settings: raw(t, map[string]any{
				"groups":    []map[string]any{{"name": "Manual", "type": "select", "proxies": []any{"hk-node", "DIRECT"}}},
				"rule_sets": []map[string]any{{"name": "manual", "type": "inline", "behavior": "classical", "payload": []any{"DOMAIN-SUFFIX,example.com"}}},
				"rules":     []string{"RULE-SET,manual,Manual", "MATCH,DIRECT"},
			}),
		},
		Processors: []domain.ProcessorSpec{
			preset("sniffer!:\n  enable: true\n  sniff:\n    HTTP:\n      ports: [80]\n"),
			preset("tun!:\n  enable: true\n  route-exclude-address: [10.0.0.0/8]\n"),
			preset("dns:\n  fake-ip-filter+: [+.tailscale.com, +.ts.net]\n  nameserver-policy:\n    <+.ts.net>: 100.100.100.100\ntun:\n  route-exclude-address+: [100.64.0.0/10, 'fd7a:115c:a1e0::/48']\n"),
			preset("lan-allowed-ips+: [100.64.0.0/10, 'fd7a:115c:a1e0::/48']\n"),
		},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(result.Content, &doc))
	require.Len(t, doc["proxies"], 1)
	require.Equal(t, "hk-node", doc["proxies"].([]any)[0].(map[string]any)["name"])
	require.Equal(t, []any{map[string]any{"name": "Manual", "type": "select", "proxies": []any{"hk-node", "DIRECT"}}}, doc["proxy-groups"])
	require.Equal(t, []any{"RULE-SET,manual,Manual", "MATCH,DIRECT"}, doc["rules"])
	require.Contains(t, doc["rule-providers"].(map[string]any), "manual")
	require.Equal(t, true, doc["sniffer"].(map[string]any)["enable"])
	require.Equal(t, []any{"10.0.0.0/8", "100.64.0.0/10", "fd7a:115c:a1e0::/48"}, doc["tun"].(map[string]any)["route-exclude-address"])
	require.Equal(t, []any{"base", "+.tailscale.com", "+.ts.net"}, doc["dns"].(map[string]any)["fake-ip-filter"])
	require.Equal(t, "100.100.100.100", doc["dns"].(map[string]any)["nameserver-policy"].(map[string]any)["+.ts.net"])
	require.Equal(t, []any{"10.0.0.0/8", "100.64.0.0/10", "fd7a:115c:a1e0::/48"}, doc["lan-allowed-ips"])
}

func TestServiceSingBoxFileGeneratesCompleteConfig(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "default",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#sg-node",
	}))
	spec := domain.FileSpec{
		Name:   "default.json",
		Kind:   domain.FileKindSingBox,
		Source: domain.FileSource{},
		Config: &domain.FileConfig{
			Subscriptions: []string{"default"},
		},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Equal(t, "application/json", result.ContentType)
	require.Equal(t, "sing-box", result.File.Kind)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &doc))
	outbounds := doc["outbounds"].([]any)
	require.NotEmpty(t, outbounds)
	require.Equal(t, "selector", outbounds[0].(map[string]any)["type"])
	require.Equal(t, "Proxy", outbounds[0].(map[string]any)["tag"])
	require.Equal(t, "Auto", outbounds[1].(map[string]any)["tag"])
	require.Equal(t, "https://cp.cloudflare.com", outbounds[1].(map[string]any)["url"])
	require.True(t, containsOutboundTag(outbounds, "sg-node"))
	route := doc["route"].(map[string]any)
	require.NotEmpty(t, route["rule_set"])
	rules := route["rules"].([]any)
	require.Equal(t, "direct", rules[0].(map[string]any)["outbound"])
	require.Equal(t, "Proxy", rules[len(rules)-1].(map[string]any)["outbound"])
}

func TestServiceSingBoxFileUsesExplicitGroupsRuleSetsAndRules(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "default",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#sg-node",
	}))
	spec := domain.FileSpec{
		Name:   "custom.json",
		Kind:   domain.FileKindSingBox,
		Source: domain.FileSource{},
		Config: &domain.FileConfig{
			Subscriptions: []string{"default"},
			Settings: raw(t, map[string]any{
				"groups": []map[string]any{{
					"type":      "selector",
					"tag":       "Manual",
					"outbounds": []any{"sg-node", "direct"},
				}},
				"rule_sets": []map[string]any{{
					"type":  "inline",
					"tag":   "manual",
					"rules": []any{map[string]any{"domain_suffix": []any{"example.com"}}},
				}},
				"rules": []map[string]any{
					{"rule_set": []any{"manual"}, "outbound": "Manual"},
					{"outbound": "direct"},
				},
			}),
		},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &doc))
	outbounds := doc["outbounds"].([]any)
	require.Equal(t, map[string]any{
		"type":      "selector",
		"tag":       "Manual",
		"outbounds": []any{"sg-node", "direct"},
	}, outbounds[0])
	require.True(t, containsOutboundTag(outbounds, "sg-node"))
	route := doc["route"].(map[string]any)
	require.Equal(t, []any{map[string]any{
		"type":  "inline",
		"tag":   "manual",
		"rules": []any{map[string]any{"domain_suffix": []any{"example.com"}}},
	}}, route["rule_set"])
	require.Equal(t, []any{
		map[string]any{"rule_set": []any{"manual"}, "outbound": "Manual"},
		map[string]any{"outbound": "direct"},
	}, route["rules"])
}

func TestServiceSingBoxFilePreservesExplicitRouteFinal(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	spec := domain.FileSpec{
		Name: "localized.json",
		Kind: domain.FileKindSingBox,
		Source: domain.FileSource{
			Type:    "inline",
			Content: `{"route":{"final":"🚀 节点选择"}}`,
		},
		Config: &domain.FileConfig{Settings: raw(t, map[string]any{
			"groups":    []map[string]any{{"type": "selector", "tag": "🚀 节点选择", "outbounds": []any{"direct"}}},
			"rule_sets": []map[string]any{},
			"rules":     []map[string]any{{"outbound": "🚀 节点选择"}},
		})},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &doc))
	require.Equal(t, "🚀 节点选择", doc["route"].(map[string]any)["final"])
}

func TestServiceTypedConfigRejectsUnknownSetting(t *testing.T) {
	spec := domain.FileSpec{
		Name:   "default.yaml",
		Kind:   domain.FileKindMihomo,
		Source: domain.FileSource{},
		Config: &domain.FileConfig{Settings: raw(t, map[string]any{"group_preset": "basic"})},
	}

	_, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
}

func TestServiceTypedConfigRunsFileProcessorsAfterGeneration(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	spec := domain.FileSpec{
		Name:   "default.yaml",
		Kind:   domain.FileKindMihomo,
		Source: domain.FileSource{},
		Config: &domain.FileConfig{},
		Processors: []domain.ProcessorSpec{
			{
				Type:  "merge",
				Stage: domain.StageFile,
				Params: params(t, map[string]any{
					"mode":    "yaml_overlay",
					"content": "mixed-port: 9999\nmode: global\n",
				}),
			},
			{
				Type:  "script",
				Stage: domain.StageFile,
				Params: params(t, map[string]any{
					"source": inlineScriptSource(`function main(input) {
  input.file.content = input.file.content + "\n# target=" + input.target;
  return input;
}`),
				}),
			},
		},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Contains(t, string(result.Content), "mixed-port: 9999")
	require.Contains(t, string(result.Content), "mode: global")
	require.Contains(t, string(result.Content), "proxy-groups:")
	require.Contains(t, string(result.Content), "# target=mihomo")
}

func TestServiceTypedFileScriptSourceCyclesFailAcrossNodeStage(t *testing.T) {
	tests := map[string]struct {
		scriptSource string
		files        []domain.FileSpec
	}{
		"direct": {
			scriptSource: "a.yaml",
			files: []domain.FileSpec{{
				Name:   "a.yaml",
				Kind:   domain.FileKindStatic,
				Source: domain.FileSource{Type: "inline", Content: "function main(input) { return input; }"},
			}},
		},
		"indirect": {
			scriptSource: "b.js",
			files: []domain.FileSpec{
				{
					Name:   "a.yaml",
					Kind:   domain.FileKindStatic,
					Source: domain.FileSource{Type: "inline", Content: "function main(input) { return input; }"},
				},
				{
					Name:   "b.js",
					Kind:   domain.FileKindStatic,
					Source: domain.FileSource{Type: "inline", Content: "function main(input) { return input; }"},
					Processors: []domain.ProcessorSpec{{
						Type:  "script",
						Stage: domain.StageFile,
						Params: params(t, map[string]any{
							"source": fileScriptSource("a.yaml"),
						}),
					}},
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			svc := service.New(service.WithFS(afero.NewMemMapFs()))
			for _, file := range test.files {
				require.NoError(t, svc.PutFile(ctx, file))
			}
			require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
				Name:    "nodes",
				Type:    domain.SubscriptionTypeLocal,
				Format:  "uri-list",
				Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
				Processors: []domain.ProcessorSpec{{
					Type:  "script",
					Stage: domain.StageNodes,
					Params: params(t, map[string]any{
						"source": fileScriptSource(test.scriptSource),
					}),
				}},
			}))
			outer := domain.FileSpec{
				Name:   "a.yaml",
				Kind:   domain.FileKindMihomo,
				Source: domain.FileSource{},
				Config: &domain.FileConfig{Subscriptions: []string{"nodes"}},
			}

			_, err := svc.GetFile(ctx, domain.FileRequest{Spec: &outer})

			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeFileDependencyCycle), "got %v", err)
		})
	}
}

type countingFileProcessor struct {
	calls *int
}

func (p countingFileProcessor) Name() string { return "count" }

func (p countingFileProcessor) ApplyFile(_ context.Context, in domain.FileProcessInput) (domain.FileProcessOutput, error) {
	(*p.calls)++
	return domain.FileProcessOutput{File: in.File}, nil
}

func TestServiceTypedFileScriptSourcesReuseFileMemoAndRecordDependencies(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	calls := 0
	svc.Registry().RegisterFile("count", func(domain.ProcessorSpec) (domain.FileProcessor, error) {
		return countingFileProcessor{calls: &calls}, nil
	})
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:   "helper.js",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "function main(input) { return input; }"},
		Processors: []domain.ProcessorSpec{{
			Type:  "count",
			Stage: domain.StageFile,
		}},
	}))
	for _, name := range []string{"first", "second"} {
		require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
			Name:    name,
			Type:    domain.SubscriptionTypeLocal,
			Format:  "uri-list",
			Content: "ss://aes-128-gcm:secret@example.com:8388#" + name,
			Processors: []domain.ProcessorSpec{{
				Type:  "script",
				Stage: domain.StageNodes,
				Params: params(t, map[string]any{
					"source": fileScriptSource("helper.js"),
				}),
			}},
		}))
	}
	spec := domain.FileSpec{
		Name:   "config.yaml",
		Kind:   domain.FileKindMihomo,
		Source: domain.FileSource{},
		Config: &domain.FileConfig{Subscriptions: []string{"first", "second"}},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Contains(t, result.Report.Dependencies, domain.ResourceRef{Kind: "subscription", Name: "first"})
	require.Contains(t, result.Report.Dependencies, domain.ResourceRef{Kind: "subscription", Name: "second"})
	require.Contains(t, result.Report.Dependencies, domain.ResourceRef{Kind: "file", Name: "helper.js"})
}

func TestServiceSubscriptionNodesAreCanonicalAcrossPreviewRenderAndTypedFile(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "canonical", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node",
		Processors: []domain.ProcessorSpec{{
			Type: "script", Stage: domain.StageNodes,
			Params: params(t, map[string]any{"source": inlineScriptSource(`function main(input) {
  var prefix = input.target ? input.target : "canonical";
  input.nodes.forEach(function(node) { node.name = prefix + "-" + node.name; });
  return input;
}`)}),
		}},
	}))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name: "config.yaml", Kind: domain.FileKindMihomo,
		Config: &domain.FileConfig{Subscriptions: []string{"canonical"}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "canonical")
	require.NoError(t, err)
	require.Len(t, preview.Nodes, 1)
	require.Equal(t, "canonical-node", preview.Nodes[0].After.Name)
	rendered, err := svc.RenderSubscription(ctx, "canonical", "mihomo-proxies", domain.RequestInfo{})
	require.NoError(t, err)
	require.Contains(t, string(rendered.Body), "canonical-node")
	file, err := svc.GetFile(ctx, domain.FileRequest{Name: "config.yaml"})
	require.NoError(t, err)
	require.Contains(t, string(file.Content), "canonical-node")
	require.NotContains(t, string(file.Content), "mihomo-proxies-node")
}

func TestServiceTypedNodeScriptsDoNotGainFileAPIFromResolutionContext(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:   "helper.js",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "helper"},
	}))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "nodes",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input, api) {
  var rejected = false;
  try {
    api.file.content("helper.js");
  } catch (error) {
    rejected = true;
  }
  if (!rejected) throw new Error("resolution context exposed api.file.content");
  return input;
}`),
			}),
		}},
	}))
	spec := domain.FileSpec{
		Name:   "config.yaml",
		Kind:   domain.FileKindMihomo,
		Source: domain.FileSource{},
		Config: &domain.FileConfig{Subscriptions: []string{"nodes"}},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	require.NotContains(t, result.Report.Dependencies, domain.ResourceRef{Kind: "file", Name: "helper.js"})
}

func containsOutboundTag(outbounds []any, tag string) bool {
	for _, item := range outbounds {
		outbound, ok := item.(map[string]any)
		if ok && outbound["tag"] == tag {
			return true
		}
	}
	return false
}
