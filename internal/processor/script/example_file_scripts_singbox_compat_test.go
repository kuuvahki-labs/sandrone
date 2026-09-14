//go:build probe_singbox

package script_test

import (
	"testing"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/require"
)

func TestExampleSubscriptionFilterAndCustomRoutingStartWithLockedSingBox(t *testing.T) {
	input := `{
  "outbounds":[
    {"type":"selector","tag":"Proxy","outbounds":["direct","Premium HK"]},
    {"type":"direct","tag":"direct"},
    {"type":"block","tag":"block"},
    {
      "type":"shadowsocks",
      "tag":"Premium HK",
      "server":"127.0.0.1",
      "server_port":8388,
      "method":"aes-128-gcm",
      "password":"test-password"
    }
  ],
  "route":{"rules":[{"action":"sniff"}],"final":"Proxy"}
}`
	filtered := applyExampleFileScript(t, "subscription-filter-group.js", "sing-box", input, map[string]any{
		"filter":       "(?i)premium",
		"target_group": "Proxy",
		"group_name":   "Premium",
	})
	routed := applyExampleFileScript(t, "custom-routing.js", "sing-box", string(filtered.File.Content), map[string]any{
		"rules": `[
  {"type":"domain_suffix","value":"service.example","route":"direct"},
  {"type":"port","value":8443,"route":"proxy"},
  {"type":"domain","value":"blocked.example","route":"reject"}
]`,
	})
	requireSingBoxStarts(t, routed.File.Content)
}

func TestExampleCustomHostsStartsWithLockedSingBox(t *testing.T) {
	input := `{
  "dns":{
    "servers":[{"type":"local","tag":"dns-local"}],
    "rules":[],
    "final":"dns-local"
  },
  "outbounds":[{"type":"direct","tag":"direct"}],
  "route":{"rules":[],"final":"direct"}
}`
	out := applyExampleFileScript(t, "custom-hosts.js", "sing-box", input, map[string]any{
		"hosts": `{"router.lan":"192.0.2.1","service.lan":"2001:db8::1"}`,
	})
	requireSingBoxStarts(t, out.File.Content)
}

func TestExampleRealIPDomainsStartWithLockedSingBox(t *testing.T) {
	input := `{
  "dns":{
    "servers":[
      {"type":"local","tag":"dns-real"},
      {"type":"fakeip","tag":"dns-fake","inet4_range":"198.18.0.0/15"}
    ],
    "rules":[{"query_type":["A","AAAA"],"action":"route","server":"dns-fake"}],
    "final":"dns-real"
  },
  "outbounds":[{"type":"direct","tag":"direct"}],
  "route":{"rules":[],"final":"direct"}
}`
	out := applyExampleFileScript(t, "real-ip-domains.js", "sing-box", input, map[string]any{
		"domain":        "host.example",
		"domain_suffix": "service.example",
	})
	requireSingBoxStarts(t, out.File.Content)
}

func TestExampleChainProxyStartsWithLockedSingBox(t *testing.T) {
	input := `{
  "outbounds":[
    {"type":"selector","tag":"Proxy","outbounds":["Hong Kong","Landing"]},
    {"type":"direct","tag":"direct"},
    {
      "type":"shadowsocks",
      "tag":"Hong Kong",
      "server":"127.0.0.1",
      "server_port":8388,
      "method":"aes-128-gcm",
      "password":"test-password"
    },
    {
      "type":"shadowsocks",
      "tag":"Landing",
      "server":"127.0.0.1",
      "server_port":8389,
      "method":"aes-128-gcm",
      "password":"test-password"
    }
  ],
  "route":{"rules":[],"final":"Landing Group"}
}`
	out := applyExampleFileScript(t, "chain-proxy.js", "sing-box", input, map[string]any{
		"landing_pattern": "^Landing$",
		"front_proxies":   []string{"Proxy"},
		"front_group":     "Front Group",
		"landing_group":   "Landing Group",
	})
	requireSingBoxStarts(t, out.File.Content)
}

func requireSingBoxStarts(t *testing.T, content []byte) {
	t.Helper()
	ctx := include.Context(t.Context())
	var options option.Options
	require.NoError(t, options.UnmarshalJSONContext(ctx, content))
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, instance.Close())
	})
	require.NoError(t, instance.Start())
}
