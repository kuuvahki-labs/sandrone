package script_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestExampleSubscriptionFilterGroupPrependsMihomoGroup(t *testing.T) {
	input := `proxies:
  - {name: "Premium HK", type: ss, server: hk.example.com, port: 8388}
  - {name: "Standard JP", type: ss, server: jp.example.com, port: 8388}
proxy-groups:
  - name: "🚀 节点选择"
    type: select
    proxies: [DIRECT, "精品节点", "Standard JP", "精品节点"]
`
	out := applyExampleFileScript(t, "subscription-filter-group.js", "mihomo", input, map[string]any{
		"filter":       "(?i)premium|iplc",
		"target_group": "🚀 节点选择",
		"group_name":   "精品节点",
	})

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out.File.Content, &doc))
	groups := requireObjectList(t, doc["proxy-groups"])
	require.Equal(t, []any{"精品节点", "DIRECT", "Standard JP"}, requireNamedObject(t, groups, "name", "🚀 节点选择")["proxies"])
	require.Equal(t, map[string]any{
		"name":                "精品节点",
		"type":                "select",
		"include-all-proxies": true,
		"filter":              "(?i)premium|iplc",
	}, requireNamedObject(t, groups, "name", "精品节点"))
}

func TestExampleSubscriptionFilterGroupUpdatesMihomoGroupIdempotently(t *testing.T) {
	input := `proxy-groups:
  - {name: Main, type: select, proxies: [Filtered, DIRECT]}
  - {name: Filtered, type: select, proxies: [old-node]}
  - {name: Filtered, type: select, proxies: [duplicate]}
`
	args := map[string]any{
		"filter":       "new-keyword",
		"target_group": "Main",
		"group_name":   "Filtered",
	}
	first := applyExampleFileScript(t, "subscription-filter-group.js", "mihomo", input, args)
	second := applyExampleFileScript(t, "subscription-filter-group.js", "mihomo", string(first.File.Content), args)

	var firstDoc, secondDoc map[string]any
	require.NoError(t, yaml.Unmarshal(first.File.Content, &firstDoc))
	require.NoError(t, yaml.Unmarshal(second.File.Content, &secondDoc))
	require.Equal(t, firstDoc, secondDoc)
	groups := requireObjectList(t, firstDoc["proxy-groups"])
	require.Len(t, groups, 2)
	require.Equal(t, []any{"Filtered", "DIRECT"}, requireNamedObject(t, groups, "name", "Main")["proxies"])
	require.Equal(t, "new-keyword", requireNamedObject(t, groups, "name", "Filtered")["filter"])
}

func TestExampleSubscriptionFilterGroupPrependsShadowrocketGroupIdempotently(t *testing.T) {
	input := `[General]
ipv6 = true

[Proxy Group]
Main = select,DIRECT,Filtered,Other,Filtered
Filtered = select,old-node
Filtered = select,duplicate

[Rule]
FINAL,Main
`
	args := map[string]any{
		"filter":       "(?i)premium|iplc",
		"target_group": "Main",
		"group_name":   "Filtered",
	}
	first := applyExampleFileScript(t, "subscription-filter-group.js", "shadowrocket", input, args)
	second := applyExampleFileScript(t, "subscription-filter-group.js", "shadowrocket", string(first.File.Content), args)

	require.Equal(t, string(first.File.Content), string(second.File.Content))
	require.Contains(t, string(first.File.Content), "Main = select,Filtered,DIRECT,Other\n")
	require.Contains(t, string(first.File.Content), "Filtered = select,policy-regex-filter=(?i)premium|iplc\n")
	require.Equal(t, 1, countLines(string(first.File.Content), "Filtered = "))
}

func countLines(content, prefix string) int {
	count := 0
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}
