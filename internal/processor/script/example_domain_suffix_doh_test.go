package script_test

import (
	"encoding/json/v2"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

func TestExampleDomainSuffixDOHMihomo(t *testing.T) {
	input := `dns:
  enable: true
  nameserver: [https://default.example.org/dns-query]
  direct-nameserver: [https://direct.example.org/dns-query]
  proxy-server-nameserver: [https://bootstrap.example.org/dns-query]
  nameserver-policy:
    '+.example.com': [https://old.example.org/dns-query]
    '+.unrelated.example': system
rules: ['MATCH,PROXY']
`
	args := map[string]any{
		"suffixes": "+.EXAMPLE.COM., example.net,example.com",
		"doh":      "https://dns.example.org:8443/custom-dns",
	}
	out := applyExampleFileScript(t, "domain-suffix-doh.js", "mihomo", input, args)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out.File.Content, &doc))
	dns := doc["dns"].(map[string]any)
	policy := dns["nameserver-policy"].(map[string]any)
	for _, suffix := range []string{"+.example.com", "+.example.net"} {
		require.Equal(t, []any{"https://dns.example.org:8443/custom-dns#DIRECT"}, policy[suffix])
	}
	require.Equal(t, "system", policy["+.unrelated.example"])
	require.Equal(t, true, dns["direct-nameserver-follow-policy"])
	require.Equal(t, []any{"https://default.example.org/dns-query"}, dns["nameserver"])
	require.Equal(t, []any{"https://direct.example.org/dns-query"}, dns["direct-nameserver"])
	require.Equal(t, []any{"https://bootstrap.example.org/dns-query"}, dns["proxy-server-nameserver"])
	require.Equal(t, []any{"MATCH,PROXY"}, doc["rules"])
	again := applyExampleFileScript(t, "domain-suffix-doh.js", "mihomo", string(out.File.Content), args)
	require.Equal(t, out.File.Content, again.File.Content)
}

func TestExampleDomainSuffixDOHSingBox(t *testing.T) {
	input := `{"dns":{
  "servers":[{"type":"https","tag":"dns-cn","server":"192.0.2.53"}],
  "rules":[{"rule_set":["cn"],"action":"route","server":"dns-cn"}],
  "final":"dns-cn"
},"route":{"final":"PROXY"},"outbounds":[{"type":"direct","tag":"direct"}]}`
	args := map[string]any{
		"suffixes": `["example.com","example.net"]`,
		"doh":      "https://dns.example.org:8443/custom-dns",
	}
	out := applyExampleFileScript(t, "domain-suffix-doh.js", "sing-box", input, args)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.File.Content, &doc))
	dns := doc["dns"].(map[string]any)
	servers := requireObjectList(t, dns["servers"])
	require.Len(t, servers, 2)
	require.Equal(t, map[string]any{
		"type": "https", "tag": "sandrone-suffix-doh", "server": "dns.example.org",
		"server_port": float64(8443), "path": "/custom-dns", "domain_resolver": "dns-cn",
	}, servers[1])
	rules := requireObjectList(t, dns["rules"])
	require.Len(t, rules, 2)
	require.Equal(t, map[string]any{
		"domain_suffix": []any{"example.com", "example.net"}, "action": "route", "server": "sandrone-suffix-doh",
	}, rules[0])
	require.Equal(t, map[string]any{"rule_set": []any{"cn"}, "action": "route", "server": "dns-cn"}, rules[1])
	require.Equal(t, "dns-cn", dns["final"])
	require.Equal(t, map[string]any{"final": "PROXY"}, doc["route"])
	require.Equal(t, []any{map[string]any{"type": "direct", "tag": "direct"}}, doc["outbounds"])
	again := applyExampleFileScript(t, "domain-suffix-doh.js", "sing-box", string(out.File.Content), args)
	require.JSONEq(t, string(out.File.Content), string(again.File.Content))
	args["doh"] = "https://192.0.2.54"
	args["suffixes"] = "example.net"
	updated := applyExampleFileScript(t, "domain-suffix-doh.js", "sing-box", string(out.File.Content), args)
	require.NoError(t, json.Unmarshal(updated.File.Content, &doc))
	dns = doc["dns"].(map[string]any)
	servers = requireObjectList(t, dns["servers"])
	require.Len(t, servers, 2)
	require.Equal(t, map[string]any{
		"type": "https", "tag": "sandrone-suffix-doh", "server": "192.0.2.54",
		"server_port": float64(443), "path": "/dns-query",
	}, servers[1])
	rules = requireObjectList(t, dns["rules"])
	require.Len(t, rules, 2)
	require.Equal(t, []any{"example.net"}, rules[0].(map[string]any)["domain_suffix"])
}

func TestExampleDomainSuffixDOHSingBoxIPAndMultipleInstances(t *testing.T) {
	input := `{"dns":{"servers":[]}}`
	for _, endpoint := range []string{"https://192.0.2.53", "https://[2001:db8::53]"} {
		t.Run(endpoint, func(t *testing.T) {
			args := map[string]any{"suffixes": []string{"example.com"}, "doh": endpoint}
			out := applyExampleFileScript(t, "domain-suffix-doh.js", "sing-box", input, args)
			args["suffixes"] = "example.net"
			args["tag"] = "another-suffix-doh"
			out = applyExampleFileScript(t, "domain-suffix-doh.js", "sing-box", string(out.File.Content), args)
			var doc map[string]any
			require.NoError(t, json.Unmarshal(out.File.Content, &doc))
			dns := doc["dns"].(map[string]any)
			servers := requireObjectList(t, dns["servers"])
			require.Len(t, servers, 2)
			require.Equal(t, "/dns-query", servers[0].(map[string]any)["path"])
			rules := requireObjectList(t, dns["rules"])
			require.Equal(t, "another-suffix-doh", rules[0].(map[string]any)["server"])
			require.Equal(t, "sandrone-suffix-doh", rules[1].(map[string]any)["server"])
		})
	}
}

func TestExampleDomainSuffixDOHShadowrocket(t *testing.T) {
	input := `# profile
[General]
dns-server = https://default.example.org/dns-query
direct-dns-server = https://direct.example.org/dns-query
fallback-dns-server = https://fallback.example.org/dns-query#proxy
[Host]
# example.com = keep this comment
EXAMPLE.COM = server:192.0.2.53
*.example.com = server:https://old.example.org/dns-query
other.example = 192.0.2.10
[Rule]
# rules
DOMAIN-SUFFIX,com,PROXY
DOMAIN-SUFFIX,example.com,PROXY,force-remote-dns
DOMAIN-SUFFIX,other.example,PROXY
FINAL,PROXY
[host]
example.com = server:192.0.2.54
*.unrelated.example = server:system
[rule]
domain-suffix, EXAMPLE.COM , DIRECT
DOMAIN,other.example,DIRECT
[URL Rewrite]
^https://old.example/ https://new.example/ 302
`
	args := map[string]any{
		"suffixes": "example.com,example.net,EXAMPLE.COM",
		"doh":      "https://dns.example.org:8443/query-dns",
	}
	expected := `# profile
[General]
dns-server = https://default.example.org/dns-query
direct-dns-server = https://direct.example.org/dns-query
fallback-dns-server = https://fallback.example.org/dns-query#proxy
[Host]
# example.com = keep this comment
other.example = 192.0.2.10
example.com = server:https://dns.example.org:8443/query-dns
*.example.com = server:https://dns.example.org:8443/query-dns
example.net = server:https://dns.example.org:8443/query-dns
*.example.net = server:https://dns.example.org:8443/query-dns
[Rule]
DOMAIN-SUFFIX,example.com,DIRECT
DOMAIN-SUFFIX,example.net,DIRECT
# rules
DOMAIN-SUFFIX,com,PROXY
DOMAIN-SUFFIX,other.example,PROXY
FINAL,PROXY
[host]
*.unrelated.example = server:system
[rule]
DOMAIN,other.example,DIRECT
[URL Rewrite]
^https://old.example/ https://new.example/ 302
`
	for _, newline := range []string{"\n", "\r\n"} {
		t.Run(newline, func(t *testing.T) {
			out := applyExampleFileScript(t, "domain-suffix-doh.js", "shadowrocket", strings.ReplaceAll(input, "\n", newline), args)
			require.Equal(t, strings.ReplaceAll(expected, "\n", newline), string(out.File.Content))
			again := applyExampleFileScript(t, "domain-suffix-doh.js", "shadowrocket", string(out.File.Content), args)
			require.Equal(t, out.File.Content, again.File.Content)
		})
	}
}

func TestExampleDomainSuffixDOHShadowrocketCreatesSectionsAndUpdatesEndpoint(t *testing.T) {
	input := "\ufeff[General]\ndns-server = system\n"
	args := map[string]any{"suffixes": "example.com", "doh": "https://dns.example.org/query-dns"}
	out := applyExampleFileScript(t, "domain-suffix-doh.js", "shadowrocket", input, args)
	expected := input + "[Host]\n" +
		"example.com = server:https://dns.example.org/query-dns\n" +
		"*.example.com = server:https://dns.example.org/query-dns\n" +
		"[Rule]\nDOMAIN-SUFFIX,example.com,DIRECT\n"
	require.Equal(t, expected, string(out.File.Content))
	args["doh"] = "https://new.example.org/dns-query"
	out = applyExampleFileScript(t, "domain-suffix-doh.js", "shadowrocket", string(out.File.Content), args)
	expected = strings.ReplaceAll(expected, "https://dns.example.org/query-dns", "https://new.example.org/dns-query")
	require.Equal(t, expected, string(out.File.Content))
}

func TestExampleDomainSuffixDOHRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		name, kind, content string
		args                map[string]any
		message             string
	}{
		{"missing suffix", "mihomo", `dns: {enable: true}`, map[string]any{}, "suffixes"},
		{"bad suffix", "mihomo", `dns: {enable: true}`, map[string]any{"suffixes": "a.*.com"}, "suffixes"},
		{"plain HTTP", "mihomo", `dns: {enable: true}`, map[string]any{"doh": "http://dns.example.org"}, "HTTPS"},
		{"proxy fragment", "mihomo", `dns: {enable: true}`, map[string]any{"doh": "https://dns.example.org/dns-query#PROXY"}, "HTTPS"},
		{"query", "mihomo", `dns: {enable: true}`, map[string]any{"doh": "https://dns.example.org/dns-query?key=placeholder"}, "HTTPS"},
		{"bad IPv6", "mihomo", `dns: {enable: true}`, map[string]any{"doh": "https://[1234]"}, "主机名"},
		{"bad port", "mihomo", `dns: {enable: true}`, map[string]any{"doh": "https://dns.example.org:65536"}, "端口"},
		{"disabled DNS", "mihomo", `dns: {enable: false}`, nil, "dns.enable"},
		{"missing bootstrap", "sing-box", `{"dns":{"servers":[]}}`, nil, "bootstrap"},
		{"self bootstrap", "sing-box", `{"dns":{"servers":[]}}`, map[string]any{"bootstrap": "sandrone-suffix-doh"}, "bootstrap"},
		{"legacy DNS", "sing-box", `{"dns":{"servers":[{"tag":"dns-cn","address":"local"}]}}`, nil, "typed DNS"},
		{"INI server separator", "shadowrocket", "[General]\n", map[string]any{"doh": "https://dns.example.org/query,system"}, "逗号"},
		{"unsupported client", "static", "[General]\n", nil, "仅支持"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := map[string]any{"suffixes": "example.com", "doh": "https://dns.example.org"}
			for key, value := range test.args {
				args[key] = value
			}
			if test.name == "missing suffix" {
				delete(args, "suffixes")
			}
			registry := processor.NewRegistry()
			registerScript(registry)
			proc, err := registry.BuildFile(domain.ProcessorSpec{
				Type: "script", Stage: domain.StageFile,
				Params: params(t, map[string]any{
					"source": fileScriptSource(filepath.Join("..", "..", "..", "examples", "scripts", "domain-suffix-doh.js")),
					"args":   args,
				}),
			})
			require.NoError(t, err)
			_, err = proc.ApplyFile(t.Context(), domain.FileProcessInput{
				File: domain.FileDocument{Kind: test.kind, Content: []byte(test.content)},
			})
			require.ErrorContains(t, err, test.message)
		})
	}
}
