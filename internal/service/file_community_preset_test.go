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
	return domain.ProcessorSpec{
		Name:  "Traditional NTP Direct",
		Type:  "script",
		Stage: domain.StageFile,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(script),
			"args": map[string]any{
				"preset_id":  "ntp-direct",
				"rules_json": rulesJSON,
			},
		}),
	}
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
