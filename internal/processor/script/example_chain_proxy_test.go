package script_test

import (
	json "encoding/json/v2"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

func applyExampleFileScript(t *testing.T, filename, kind, content string, args map[string]any) domain.FileProcessOutput {
	t.Helper()
	registry := processor.NewRegistry()
	registerScript(registry)
	proc, err := registry.BuildFile(domain.ProcessorSpec{
		Type:  "script",
		Stage: domain.StageFile,
		Params: params(t, map[string]any{
			"source": fileScriptSource(filepath.Join("..", "..", "..", "examples", "scripts", filename)),
			"args":   args,
		}),
	})
	require.NoError(t, err)

	out, err := proc.ApplyFile(t.Context(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: kind, Content: []byte(content)},
	})
	require.NoError(t, err)
	return out
}

func TestExampleMihomoChainProxyRewritesFinalFile(t *testing.T) {
	input := `proxies:
  - name: 香港普通
    type: ss
    server: hk.example.com
    port: 8388
  - name: 落地 A
    type: ss
    server: landing.example.com
    port: 8388
proxy-groups:
  - name: 香港节点
    type: select
    proxies: [香港普通, 落地 A]
  - name: 置换前置代理
    type: select
    proxies: [DIRECT]
`
	out := applyExampleFileScript(t, "mihomo-chain-proxy.js", "mihomo", input, map[string]any{
		"landing_pattern": "^落地",
		"front_proxies":   []string{"香港节点"},
	})

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out.File.Content, &doc))
	proxies := requireObjectList(t, doc["proxies"])
	require.NotContains(t, requireNamedObject(t, proxies, "name", "香港普通"), "dialer-proxy")
	require.Equal(t, "前置代理", requireNamedObject(t, proxies, "name", "落地 A")["dialer-proxy"])

	groups := requireObjectList(t, doc["proxy-groups"])
	require.Equal(t, []any{"香港普通"}, requireNamedObject(t, groups, "name", "香港节点")["proxies"])
	require.Equal(t, []any{"香港节点"}, requireNamedObject(t, groups, "name", "前置代理")["proxies"])
	require.Equal(t, []any{"落地 A"}, requireNamedObject(t, groups, "name", "落地节点")["proxies"])
}

func TestExampleMihomoChainProxyExtendsRuntimeGroupExclusion(t *testing.T) {
	input := `proxies:
  - {name: 香港普通, type: ss, server: hk.example.com, port: 8388}
  - {name: 落地.A, type: ss, server: landing.example.com, port: 8388}
proxy-groups:
  - name: 香港节点
    type: select
    include-all-proxies: true
    filter: 香港
    exclude-filter: 测试
`
	out := applyExampleFileScript(t, "mihomo-chain-proxy.js", "mihomo", input, map[string]any{
		"landing_pattern": "^落地",
		"front_proxies":   []string{"香港节点"},
	})

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out.File.Content, &doc))
	groups := requireObjectList(t, doc["proxy-groups"])
	require.Equal(t, `测试|^(?:落地\.A)$`, requireNamedObject(t, groups, "name", "香港节点")["exclude-filter"])
}

func TestExampleSingBoxChainProxyRewritesFinalFile(t *testing.T) {
	input := `{
  "outbounds": [
    {"type": "selector", "tag": "香港节点", "outbounds": ["香港普通", "落地 A"]},
    {"type": "direct", "tag": "direct"},
    {"type": "block", "tag": "block"},
    {"type": "shadowsocks", "tag": "香港普通", "server": "hk.example.com", "server_port": 8388},
    {"type": "vless", "tag": "落地 A", "server": "landing.example.com", "server_port": 443}
  ]
}`
	out := applyExampleFileScript(t, "sing-box-chain-proxy.js", "sing-box", input, map[string]any{
		"landing_pattern": "^落地",
		"front_proxies":   []string{"香港节点"},
	})

	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.File.Content, &doc))
	outbounds := requireObjectList(t, doc["outbounds"])
	require.NotContains(t, requireNamedObject(t, outbounds, "tag", "香港普通"), "detour")
	require.Equal(t, "前置代理", requireNamedObject(t, outbounds, "tag", "落地 A")["detour"])
	require.Equal(t, []any{"香港普通"}, requireNamedObject(t, outbounds, "tag", "香港节点")["outbounds"])
	require.Equal(t, []any{"香港节点"}, requireNamedObject(t, outbounds, "tag", "前置代理")["outbounds"])
	require.Equal(t, []any{"落地 A"}, requireNamedObject(t, outbounds, "tag", "落地节点")["outbounds"])
}

func TestExampleChainProxyLeavesFileUnchangedWithoutLandingMatches(t *testing.T) {
	tests := []struct {
		filename string
		kind     string
		content  string
	}{
		{filename: "mihomo-chain-proxy.js", kind: "mihomo", content: "proxies: []\n"},
		{filename: "sing-box-chain-proxy.js", kind: "sing-box", content: `{"outbounds":[]}`},
	}
	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			out := applyExampleFileScript(t, test.filename, test.kind, test.content, map[string]any{
				"landing_pattern": "^落地",
			})
			require.Equal(t, test.content, string(out.File.Content))
			require.Equal(t, []string{"chain_proxy_no_landing_nodes"}, warningCodes(out.Warnings))
		})
	}
}

func requireObjectList(t *testing.T, value any) []any {
	t.Helper()
	values, ok := value.([]any)
	require.True(t, ok)
	for _, value := range values {
		_, ok := value.(map[string]any)
		require.True(t, ok)
	}
	return values
}

func requireNamedObject(t *testing.T, values []any, key, name string) map[string]any {
	t.Helper()
	for _, value := range values {
		item := value.(map[string]any)
		if item[key] == name {
			return item
		}
	}
	t.Fatalf("missing object with %s=%q", key, name)
	return nil
}
