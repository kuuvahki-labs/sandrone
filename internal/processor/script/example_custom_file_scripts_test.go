package script_test

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestExampleCustomRoutingUsesArgumentsForAllKinds(t *testing.T) {
	rules := `[
  {"type":"domain_suffix","value":"service.example","route":"direct"},
  {"type":"port","value":8443,"route":"reject"}
]`

	t.Run("mihomo", func(t *testing.T) {
		out := applyExampleFileScript(t, "custom-routing.js", "mihomo",
			"rules:\n  - MATCH,Proxy\n", map[string]any{"rules": rules})
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(out.File.Content, &doc))
		require.Equal(t, []any{
			"DOMAIN-SUFFIX,service.example,DIRECT",
			"DST-PORT,8443,REJECT",
			"MATCH,Proxy",
		}, doc["rules"])
	})

	t.Run("sing-box without a template anchor", func(t *testing.T) {
		input := `{
  "outbounds":[
    {"type":"selector","tag":"Proxy","outbounds":["direct"]},
    {"type":"direct","tag":"direct"},
    {"type":"block","tag":"block"}
  ],
  "route":{
    "rules":[
      {"action":"sniff"},
      {"clash_mode":"direct","outbound":"direct"},
      {"clash_mode":"global","outbound":"Proxy"}
    ],
    "final":"Proxy"
  }
}`
		out := applyExampleFileScript(t, "custom-routing.js", "sing-box", input, map[string]any{"rules": rules})
		var doc map[string]any
		require.NoError(t, json.Unmarshal(out.File.Content, &doc))
		routeRules := doc["route"].(map[string]any)["rules"].([]any)
		require.Equal(t, map[string]any{
			"domain_suffix": []any{"service.example"},
			"action":        "route",
			"outbound":      "direct",
		}, routeRules[3])
		require.Equal(t, map[string]any{
			"port":   float64(8443),
			"action": "reject",
		}, routeRules[4])
	})

	t.Run("shadowrocket", func(t *testing.T) {
		out := applyExampleFileScript(t, "custom-routing.js", "shadowrocket",
			"[Rule]\nFINAL,Proxy\n", map[string]any{"rules": rules})
		require.True(t, strings.HasPrefix(string(out.File.Content),
			"[Rule]\nDOMAIN-SUFFIX,service.example,DIRECT\nDST-PORT,8443,REJECT\n"))
	})
}

func TestExampleCustomRoutingResolvesParameterizedRoutes(t *testing.T) {
	input := `{
  "outbounds":[
    {"type":"selector","tag":"Proxy","outbounds":["AI"]},
    {"type":"selector","tag":"AI","outbounds":["node"]},
    {"type":"direct","tag":"direct"},
    {"type":"shadowsocks","tag":"node","server":"node.example","server_port":8388}
  ],
  "route":{"rules":[{"outbound":"Proxy"}]}
}`
	out := applyExampleFileScript(t, "custom-routing.js", "sing-box", input, map[string]any{
		"rules":  `[{"type":"domain","value":"chat.example","route":"ai"}]`,
		"routes": `{"ai":{"mihomo":"AI","sing-box":"AI","shadowrocket":"AI"}}`,
	})
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.File.Content, &doc))
	rules := doc["route"].(map[string]any)["rules"].([]any)
	require.Equal(t, map[string]any{
		"domain":   []any{"chat.example"},
		"action":   "route",
		"outbound": "AI",
	}, rules[0])
}

func TestExampleCustomRoutingInitializesMissingSingBoxRules(t *testing.T) {
	input := `{
  "outbounds":[{"type":"direct","tag":"direct"}],
  "route":{"final":"direct"}
}`
	out := applyExampleFileScript(t, "custom-routing.js", "sing-box", input, map[string]any{
		"rules": `[{"type":"domain_suffix","value":"service.example","route":"direct"}]`,
	})
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.File.Content, &doc))
	rules := doc["route"].(map[string]any)["rules"].([]any)
	require.Equal(t, []any{map[string]any{
		"domain_suffix": []any{"service.example"},
		"action":        "route",
		"outbound":      "direct",
	}}, rules)
}

func TestExampleCustomHostsUsesArgumentsForAllKinds(t *testing.T) {
	args := map[string]any{
		"hosts":        `{"Router.LAN.":"192.0.2.1","service.lan":"2001:db8::1"}`,
		"sing_box_tag": "local-overrides",
	}

	t.Run("mihomo", func(t *testing.T) {
		out := applyExampleFileScript(t, "custom-hosts.js", "mihomo",
			"hosts:\n  existing.lan: 192.0.2.2\n", args)
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(out.File.Content, &doc))
		require.Equal(t, map[string]any{
			"existing.lan": "192.0.2.2",
			"router.lan":   "192.0.2.1",
			"service.lan":  "2001:db8::1",
		}, doc["hosts"])
	})

	t.Run("sing-box", func(t *testing.T) {
		input := `{"dns":{
  "servers":[{"type":"local","tag":"dns-local"}],
  "rules":[{"domain_suffix":["lan"],"action":"route","server":"dns-local"}],
  "final":"dns-local"
}}`
		first := applyExampleFileScript(t, "custom-hosts.js", "sing-box", input, args)
		second := applyExampleFileScript(t, "custom-hosts.js", "sing-box", string(first.File.Content), args)
		require.JSONEq(t, string(first.File.Content), string(second.File.Content))

		var doc map[string]any
		require.NoError(t, json.Unmarshal(first.File.Content, &doc))
		dns := doc["dns"].(map[string]any)
		servers := requireObjectList(t, dns["servers"])
		require.Equal(t, map[string]any{
			"type": "hosts", "tag": "local-overrides",
			"predefined": map[string]any{
				"router.lan":  "192.0.2.1",
				"service.lan": "2001:db8::1",
			},
		}, requireNamedObject(t, servers, "tag", "local-overrides"))
		rules := requireObjectList(t, dns["rules"])
		require.Equal(t, map[string]any{
			"domain": []any{"router.lan", "service.lan"},
			"action": "route", "server": "local-overrides",
		}, rules[0])
	})

	t.Run("shadowrocket", func(t *testing.T) {
		out := applyExampleFileScript(t, "custom-hosts.js", "shadowrocket",
			"[Host]\nexisting.lan = 192.0.2.2\n", args)
		require.Contains(t, string(out.File.Content), "router.lan = 192.0.2.1\n")
		require.Contains(t, string(out.File.Content), "service.lan = 2001:db8::1\n")
		require.Contains(t, string(out.File.Content), "existing.lan = 192.0.2.2\n")
	})
}

func TestExampleRealIPDomainsUsesArgumentsForAllKinds(t *testing.T) {
	args := map[string]any{
		"domain":        "host.example",
		"domain_suffix": `["Service.Example.","other.example"]`,
	}

	t.Run("mihomo", func(t *testing.T) {
		input := "dns:\n  enhanced-mode: fake-ip\n  fake-ip-filter: [existing.example]\n"
		out := applyExampleFileScript(t, "real-ip-domains.js", "mihomo", input, args)
		var doc map[string]any
		require.NoError(t, yaml.Unmarshal(out.File.Content, &doc))
		dns := doc["dns"].(map[string]any)
		require.Equal(t, []any{
			"existing.example", "host.example", "+.service.example", "+.other.example",
		}, dns["fake-ip-filter"])
	})

	t.Run("sing-box", func(t *testing.T) {
		input := `{"dns":{
  "servers":[
    {"type":"local","tag":"dns-local"},
    {"type":"https","tag":"dns-real","server":"192.0.2.53"},
    {"type":"fakeip","tag":"dns-fake","inet4_range":"198.18.0.0/15"}
  ],
  "rules":[
    {"domain_suffix":["lan"],"action":"route","server":"dns-local"},
    {"query_type":["A","AAAA"],"action":"route","server":"dns-fake"}
  ],
  "final":"dns-real"
}}`
		first := applyExampleFileScript(t, "real-ip-domains.js", "sing-box", input, args)
		second := applyExampleFileScript(t, "real-ip-domains.js", "sing-box", string(first.File.Content), args)
		require.JSONEq(t, string(first.File.Content), string(second.File.Content))

		var doc map[string]any
		require.NoError(t, json.Unmarshal(first.File.Content, &doc))
		rules := requireObjectList(t, doc["dns"].(map[string]any)["rules"])
		require.Equal(t, "dns-local", rules[0].(map[string]any)["server"])
		require.Equal(t, map[string]any{
			"domain":        []any{"host.example"},
			"domain_suffix": []any{"service.example", "other.example"},
			"action":        "route",
			"server":        "dns-real",
		}, rules[1])
		require.Equal(t, "dns-fake", rules[2].(map[string]any)["server"])
	})

	t.Run("shadowrocket", func(t *testing.T) {
		out := applyExampleFileScript(t, "real-ip-domains.js", "shadowrocket",
			"[General]\nalways-real-ip = existing.example\n", args)
		require.Contains(t, string(out.File.Content),
			"always-real-ip = existing.example,host.example,service.example,*.service.example,other.example,*.other.example\n")
	})
}
