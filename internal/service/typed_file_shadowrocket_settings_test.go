package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitShadowrocketRuleUsesOnlyTopLevelCommas(t *testing.T) {
	tests := []struct {
		name string
		rule string
		want []string
	}{
		{
			name: "logical expression",
			rule: "AND,((DOMAIN,www.example.com),(DST-PORT,123)),DIRECT",
			want: []string{"AND", "((DOMAIN,www.example.com),(DST-PORT,123))", "DIRECT"},
		},
		{
			name: "escaped regex parenthesis",
			rule: `URL-REGEX,^https://example\.com/\(item,REJECT`,
			want: []string{"URL-REGEX", `^https://example\.com/\(item`, "REJECT"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, splitShadowrocketRule(test.rule))
		})
	}
}

func TestDecodeShadowrocketSettingsRejectsIncompleteKnownRules(t *testing.T) {
	tests := []struct {
		name string
		rule string
	}{
		{name: "final policy", rule: "FINAL"},
		{name: "match policy", rule: "MATCH"},
		{name: "domain policy", rule: "DOMAIN,example.com"},
		{name: "domain value", rule: "DOMAIN,,DIRECT"},
		{name: "empty domain policy", rule: "DOMAIN,example.com,"},
		{name: "logical policy", rule: "AND,((DOMAIN,example.com),(DST-PORT,443))"},
		{name: "rule set policy", rule: "RULE-SET,https://example.com/a.list"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{"rules": []string{test.rule}})
			require.NoError(t, err)

			_, err = decodeShadowrocketFileSettings(raw)

			require.ErrorContains(t, err, "config.settings.rules[0]")
		})
	}
}

func TestDecodeShadowrocketSettingsRejectsUnbalancedLogicalRules(t *testing.T) {
	raw := json.RawMessage(`{"rules":["AND,((DOMAIN,example.com),(DST-PORT,443))),DIRECT"]}`)

	_, err := decodeShadowrocketFileSettings(raw)

	require.ErrorContains(t, err, "config.settings.rules[0]")
}

func TestDecodeShadowrocketSettingsKeepsUnknownRulesOpen(t *testing.T) {
	raw := json.RawMessage(`{
		"rules":[
			"PROCESS-NAME,Safari,opaque-policy",
			"IP-CIDR6,2001:db8::/32,opaque-policy,no-resolve",
			"FUTURE-RULE,custom,opaque-policy,extension"
		]
	}`)

	settings, err := decodeShadowrocketFileSettings(raw)

	require.NoError(t, err)
	require.Equal(t, []string{
		"PROCESS-NAME,Safari,opaque-policy",
		"IP-CIDR6,2001:db8::/32,opaque-policy,no-resolve",
		"FUTURE-RULE,custom,opaque-policy,extension",
	}, settings.Rules)
}
