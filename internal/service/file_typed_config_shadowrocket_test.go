package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceShadowrocketFileRejectsSubscriptions(t *testing.T) {
	spec := domain.FileSpec{
		Name:   "default.conf",
		Kind:   domain.FileKindShadowrocket,
		Source: domain.FileSource{Type: "inline", Content: "[General]\nipv6 = false\n\n[Host]\nlocalhost = 127.0.0.1\n"},
		Config: &domain.FileConfig{Subscriptions: []string{"default"}},
	}

	_, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	require.ErrorContains(t, err, `file kind "shadowrocket" does not allow subscriptions`)
}

func TestServiceShadowrocketFileUsesAppProxyWithoutSubscriptions(t *testing.T) {
	spec := domain.FileSpec{
		Name: "config-only.conf",
		Kind: domain.FileKindShadowrocket,
		Config: &domain.FileConfig{Settings: shadowrocketSettings(t, map[string]any{
			"groups": []map[string]any{
				{"name": "Proxy", "type": "select", "proxies": []string{"PROXY", "DIRECT", "REJECT"}},
			},
		})},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	body := string(result.Content)
	require.Contains(t, body, "[Proxy]\n[Proxy Group]\nProxy = select,PROXY,DIRECT,REJECT")
	require.NotContains(t, body, "Auto =")
	require.Contains(t, body, "FINAL,Proxy")
}

func TestServiceShadowrocketFileUsesExplicitGroupsRuleSetsAndRules(t *testing.T) {
	spec := domain.FileSpec{
		Name: "custom.conf",
		Kind: domain.FileKindShadowrocket,
		Config: &domain.FileConfig{
			Settings: shadowrocketSettings(t, map[string]any{
				"adaptive_groups": map[string]any{
					"type": "url-test", "regions": []string{"hk", "jp"},
				},
				"groups": []map[string]any{
					{"name": "Manual", "type": "select", "proxies": []string{"PROXY", "DIRECT"}},
					{
						"name": "Auto", "type": "url-test", "policy-regex-filter": "(?i)hk",
						"interval": 600, "timeout": 5, "tolerance": 20,
						"hidden": false,
					},
				},
				"rule_sets": []map[string]any{
					{"name": "private", "type": "rule-set", "url": "https://rules.example/private.list"},
					{"name": "ads", "type": "domain-set", "url": "https://rules.example/ads.list"},
				},
				"rules": []string{
					"RULE-SET,private,DIRECT",
					"DOMAIN-SET,https://direct.example/ads.list,REJECT",
					"FINAL,Manual",
				},
			}),
		},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	body := string(result.Content)
	require.Contains(t, body, "Manual = select,PROXY,DIRECT")
	require.Contains(t, body, "Auto = url-test,policy-regex-filter=(?i)hk,interval=600,timeout=5,tolerance=20,hidden=0")
	require.Contains(t, body, "RULE-SET,https://rules.example/private.list,DIRECT")
	require.Contains(t, body, "DOMAIN-SET,https://direct.example/ads.list,REJECT")
	require.Contains(t, body, "FINAL,Manual")
}

func TestServiceShadowrocketPreservesRulePolicyExtensionParameters(t *testing.T) {
	spec := domain.FileSpec{
		Name: "extensions.conf", Kind: domain.FileKindShadowrocket,
		Config: &domain.FileConfig{Settings: shadowrocketSettings(t, map[string]any{
			"rules": []string{
				"DOMAIN,ad.com,REJECT,extended-matching,pre-matching",
				"AND,((DOMAIN,www.example.com),(DST-PORT,123)),DIRECT",
				"FINAL,Proxy",
			},
		})},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Contains(t, string(result.Content), "DOMAIN,ad.com,REJECT,extended-matching,pre-matching")
	require.Contains(t, string(result.Content), "AND,((DOMAIN,www.example.com),(DST-PORT,123)),DIRECT")
}

func TestServiceShadowrocketAcceptsDocumentedRulePolicies(t *testing.T) {
	policies := []string{
		"PROXY", "DIRECT", "TAILSCALE", "REJECT", "REJECT-DICT", "REJECT-ARRAY",
		"REJECT-200", "REJECT-IMG", "REJECT-TINYGIF", "REJECT-DROP", "REJECT-NO-DROP",
	}
	rules := []string{"AND,((PROTOCOL,UDP),(DST-PORT,443)),REJECT-NO-DROP"}
	for _, policy := range policies {
		rules = append(rules, "DOMAIN,example.com,"+policy)
	}
	spec := domain.FileSpec{
		Name: "documented-policies.conf", Kind: domain.FileKindShadowrocket,
		Config: &domain.FileConfig{Settings: shadowrocketSettings(t, map[string]any{
			"groups": []any{},
			"rules":  rules,
		})},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	body := string(result.Content)
	for _, rule := range rules {
		require.Contains(t, body, rule)
	}
}

func TestServiceShadowrocketAcceptsDocumentedGroupPolicies(t *testing.T) {
	spec := domain.FileSpec{
		Name: "documented-group-policies.conf", Kind: domain.FileKindShadowrocket,
		Config: &domain.FileConfig{Settings: shadowrocketSettings(t, map[string]any{
			"groups": []map[string]any{{
				"name": "AI", "type": "select", "proxies": []string{"PROXY", "DIRECT", "REJECT"},
			}},
			"rules": []string{"FINAL,PROXY"},
		})},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Contains(t, string(result.Content), "AI = select,PROXY,DIRECT,REJECT")
	require.Contains(t, string(result.Content), "FINAL,PROXY")
}

func TestServiceShadowrocketLeavesUnrecognizedRawRuleKindsOpaque(t *testing.T) {
	spec := domain.FileSpec{
		Name: "opaque-rules.conf", Kind: domain.FileKindShadowrocket,
		Config: &domain.FileConfig{Settings: shadowrocketSettings(t, map[string]any{
			"rules": []string{
				"PROCESS-NAME,Safari,opaque-policy",
				"IP-CIDR6,2001:db8::/32,opaque-policy,no-resolve",
			},
		})},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Contains(t, string(result.Content), "PROCESS-NAME,Safari,opaque-policy")
	require.Contains(t, string(result.Content), "IP-CIDR6,2001:db8::/32,opaque-policy,no-resolve")
}

func TestServiceShadowrocketExplicitEmptySettingsStayEmpty(t *testing.T) {
	spec := domain.FileSpec{
		Name: "empty.conf", Kind: domain.FileKindShadowrocket,
		Config: &domain.FileConfig{Settings: json.RawMessage(`{
			"groups":[], "rule_sets":[], "rules":[]
		}`)},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Equal(t, "", sectionBody(string(result.Content), "Proxy Group"))
	require.Equal(t, "", sectionBody(string(result.Content), "Rule"))
}

func TestServiceShadowrocketReplacesOwnedSectionsAndPreservesBase(t *testing.T) {
	base := "\ufeff# keep\r\n[General]\r\nipv6 = false\r\n\r\n[Proxy]\r\nold = ss, old.example, 1\r\n\r\n[Host]\r\n# untouched\r\nlocalhost = 127.0.0.1\r\n\r\n[proxy]\r\nold-duplicate = ss, old.example, 2\r\n\r\n[Proxy Group]\r\nOld = select,old\r\n\r\n[Rule]\r\nFINAL,Old\r\n"
	spec := domain.FileSpec{
		Name: "preserve.conf", Kind: domain.FileKindShadowrocket,
		Source: domain.FileSource{Type: "inline", Content: base},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	body := string(result.Content)
	require.True(t, strings.HasPrefix(body, "\ufeff# keep\r\n"))
	require.Contains(t, body, "[General]\r\nipv6 = false\r\n")
	require.Contains(t, body, "[Host]\r\n# untouched\r\nlocalhost = 127.0.0.1\r\n")
	require.Equal(t, 1, countSection(body, "Proxy"))
	require.Equal(t, "", sectionBody(body, "Proxy"))
	require.NotContains(t, body, "old.example")
}

func TestServiceShadowrocketSettingsValidation(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		path     string
	}{
		{name: "unknown top-level field", settings: `{"future":true}`, path: "config.settings.future"},
		{name: "unknown nested field", settings: `{"groups":[{"name":"Proxy","type":"select","proxies":["DIRECT"],"future":true}]}`, path: "config.settings.groups[0].future"},
		{name: "case-mismatched nested field", settings: `{"groups":[{"Name":"Proxy","type":"select","proxies":["DIRECT"]}],"rules":[]}`, path: "config.settings.groups[0].Name"},
		{name: "nested null field", settings: `{"groups":[{"name":"Proxy","type":"select","proxies":["DIRECT"],"interval":null}]}`, path: "config.settings.groups[0].interval"},
		{name: "missing group name", settings: `{"groups":[{"type":"select","proxies":["DIRECT"]}]}`, path: "config.settings.groups[0].name"},
		{name: "comment group name", settings: `{"groups":[{"name":"#hidden","type":"select","proxies":["DIRECT"]}],"rules":[]}`, path: "config.settings.groups[0].name"},
		{name: "trimmed comment group name", settings: `{"groups":[{"name":"  #hidden","type":"select","proxies":["DIRECT"]}],"rules":[]}`, path: "config.settings.groups[0].name"},
		{name: "semicolon group name", settings: `{"groups":[{"name":";hidden","type":"select","proxies":["DIRECT"]}],"rules":[]}`, path: "config.settings.groups[0].name"},
		{name: "section group name", settings: `{"groups":[{"name":"[broken","type":"select","proxies":["DIRECT"]}],"rules":[]}`, path: "config.settings.groups[0].name"},
		{name: "proxy action group name", settings: `{"groups":[{"name":"PROXY","type":"select","proxies":["DIRECT"]}],"rules":[]}`, path: "config.settings.groups[0].name"},
		{name: "tailscale action group name", settings: `{"groups":[{"name":"TAILSCALE","type":"select","proxies":["DIRECT"]}],"rules":[]}`, path: "config.settings.groups[0].name"},
		{name: "reject action group name", settings: `{"groups":[{"name":"REJECT-DROP","type":"select","proxies":["DIRECT"]}],"rules":[]}`, path: "config.settings.groups[0].name"},
		{name: "unsupported group type", settings: `{"groups":[{"name":"Proxy","type":"relay","proxies":["DIRECT"]}]}`, path: "config.settings.groups[0].type"},
		{name: "both member sources", settings: `{"groups":[{"name":"Proxy","type":"select","proxies":["DIRECT"],"policy-regex-filter":".*"}]}`, path: "config.settings.groups[0]"},
		{name: "missing member source", settings: `{"groups":[{"name":"Proxy","type":"select"}]}`, path: "config.settings.groups[0]"},
		{name: "duplicate group", settings: `{"groups":[{"name":"Proxy","type":"select","proxies":["DIRECT"]},{"name":"Proxy","type":"select","proxies":["DIRECT"]}]}`, path: "config.settings.groups[1].name"},
		{name: "group cycle", settings: `{"groups":[{"name":"A","type":"select","proxies":["B"]},{"name":"B","type":"select","proxies":["A"]}]}`, path: "config.settings.groups"},
		{name: "bad interval", settings: `{"groups":[{"name":"Auto","type":"url-test","proxies":["PROXY"],"interval":0}]}`, path: "config.settings.groups[0].interval"},
		{name: "removed policy select name", settings: `{"groups":[{"name":"Proxy","type":"select","proxies":["DIRECT"],"policy-select-name":"DIRECT"}],"rules":[]}`, path: "config.settings.groups[0].policy-select-name"},
		{name: "removed select", settings: `{"groups":[{"name":"Proxy","type":"select","proxies":["DIRECT"],"select":0}],"rules":[]}`, path: "config.settings.groups[0].select"},
		{name: "missing final policy", settings: `{"rules":["FINAL"]}`, path: "config.settings.rules[0]"},
		{name: "missing match policy", settings: `{"rules":["MATCH"]}`, path: "config.settings.rules[0]"},
		{name: "missing domain policy", settings: `{"rules":["DOMAIN,example.com"]}`, path: "config.settings.rules[0]"},
		{name: "empty domain value", settings: `{"rules":["DOMAIN,,DIRECT"]}`, path: "config.settings.rules[0]"},
		{name: "empty domain policy", settings: `{"rules":["DOMAIN,example.com,"]}`, path: "config.settings.rules[0]"},
		{name: "missing logical policy", settings: `{"rules":["AND,((DOMAIN,example.com),(DST-PORT,443))"]}`, path: "config.settings.rules[0]"},
		{name: "missing rule set policy", settings: `{"rule_sets":[{"name":"a","type":"rule-set","url":"https://example.com/a"}],"rules":["RULE-SET,a"]}`, path: "config.settings.rules[0]"},
		{name: "rule section escape", settings: `{"rules":["[General]"]}`, path: "config.settings.rules[0]"},
		{name: "group filter newline injection", settings: `{"groups":[{"name":"Proxy","type":"select","policy-regex-filter":".*\n[Rule]\nFINAL,REJECT"}],"rules":[]}`, path: "config.settings.groups[0].policy-regex-filter"},
		{name: "group filter comma injection", settings: `{"groups":[{"name":"Proxy","type":"select","policy-regex-filter":".*,hidden=1"}],"rules":[]}`, path: "config.settings.groups[0].policy-regex-filter"},
		{name: "bad adaptive type", settings: `{"adaptive_groups":{"type":"fallback"}}`, path: "config.settings.adaptive_groups.type"},
		{name: "removed adaptive count", settings: `{"adaptive_groups":{"minimum_node_count":2}}`, path: "config.settings.adaptive_groups.minimum_node_count"},
		{name: "bad adaptive region", settings: `{"adaptive_groups":{"regions":["moon"]}}`, path: "config.settings.adaptive_groups.regions[0]"},
		{name: "duplicate rule set", settings: `{"rule_sets":[{"name":"a","type":"rule-set","url":"https://example.com/a"},{"name":"a","type":"rule-set","url":"https://example.com/b"}]}`, path: "config.settings.rule_sets[1].name"},
		{name: "bad rule set type", settings: `{"rule_sets":[{"name":"a","type":"classical","url":"https://example.com/a"}]}`, path: "config.settings.rule_sets[0].type"},
		{name: "bad rule set URL", settings: `{"rule_sets":[{"name":"a","type":"rule-set","url":"file:///tmp/a"}]}`, path: "config.settings.rule_sets[0].url"},
		{name: "rule set URL comma injection", settings: `{"rule_sets":[{"name":"a","type":"rule-set","url":"https://example.com/a,REJECT"}],"rules":[]}`, path: "config.settings.rule_sets[0].url"},
		{name: "mismatched rule set type", settings: `{"rule_sets":[{"name":"a","type":"domain-set","url":"https://example.com/a"}],"rules":["RULE-SET,a,Proxy"]}`, path: "config.settings.rules[0]"},
		{name: "unresolved rule set", settings: `{"rule_sets":[],"rules":["RULE-SET,missing,Proxy"]}`, path: "config.settings.rules[0]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := domain.FileSpec{
				Name: "bad.conf", Kind: domain.FileKindShadowrocket,
				Config: &domain.FileConfig{Settings: json.RawMessage(test.settings)},
			}

			_, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

			require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
			require.ErrorContains(t, err, `file kind "shadowrocket"`)
			require.ErrorContains(t, err, test.path)
		})
	}
}

func TestServiceShadowrocketRejectsUnresolvedPoliciesAfterRenderingNodes(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		path     string
	}{
		{name: "group member", settings: `{"groups":[{"name":"Proxy","type":"select","proxies":["missing"]}],"rules":[]}`, path: "config.settings.groups[0].proxies"},
		{name: "rule-only policy used as group member", settings: `{"groups":[{"name":"Proxy","type":"select","proxies":["TAILSCALE"]}],"rules":[]}`, path: "config.settings.groups[0].proxies"},
		{name: "rule policy", settings: `{"groups":[],"rules":["FINAL,missing"]}`, path: "config.settings.rules[0]"},
		{name: "implicit final without Proxy", settings: `{"groups":[]}`, path: "config.settings.rules[4]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := domain.FileSpec{
				Name: "bad.conf", Kind: domain.FileKindShadowrocket,
				Config: &domain.FileConfig{Settings: json.RawMessage(test.settings)},
			}

			_, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

			require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
			require.ErrorContains(t, err, test.path)
		})
	}
}

func TestServiceShadowrocketRejectsNodesMacro(t *testing.T) {
	spec := domain.FileSpec{
		Name: "invalid.conf", Kind: domain.FileKindShadowrocket,
		Config: &domain.FileConfig{Settings: json.RawMessage(`{"groups":[{"name":"Proxy","type":"select","proxies":["$nodes"]}],"rules":[]}`)},
	}

	_, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	require.ErrorContains(t, err, `config.settings.groups[0].proxies references unknown policy "$nodes"`)
}

func TestServiceShadowrocketINIOverrideRunsAfterTypedCompilation(t *testing.T) {
	spec := domain.FileSpec{
		Name: "override.conf", Kind: domain.FileKindShadowrocket,
		Source: domain.FileSource{Type: "inline", Content: "[General]\nmode = rule\n"},
		Processors: []domain.ProcessorSpec{{
			Type: "merge", Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"mode":    "ini_override",
				"content": "[General]\nmode = global\n[Rule+]\nDOMAIN,example.com,DIRECT\n",
			}),
		}},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	body := string(result.Content)
	require.Contains(t, body, "[General]\nmode = global\n")
	require.Contains(t, body, "[Proxy]\n[Proxy Group]")
	require.Contains(t, body, "FINAL,Proxy\nDOMAIN,example.com,DIRECT")
}

func shadowrocketSettings(t *testing.T, value any) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(value)
	require.NoError(t, err)
	return body
}

func countSection(body, name string) int {
	want := "[" + strings.ToLower(name) + "]"
	count := 0
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if strings.ToLower(strings.TrimSpace(line)) == want {
			count++
		}
	}
	return count
}

func sectionBody(body, name string) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	header := "[" + strings.ToLower(name) + "]"
	inside := false
	section := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if inside {
				break
			}
			inside = strings.ToLower(trimmed) == header
			continue
		}
		if inside {
			section = append(section, line)
		}
	}
	return strings.TrimSpace(strings.Join(section, "\n"))
}
