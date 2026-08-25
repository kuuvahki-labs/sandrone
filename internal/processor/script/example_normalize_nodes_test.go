package script_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

func exampleNormalizeNodesPath() string {
	return filepath.Join("..", "..", "..", "examples", "scripts", "normalize-nodes.js")
}

func buildExampleNormalizer(t *testing.T, args map[string]any) domain.NodeProcessor {
	t.Helper()
	registry := processor.NewRegistry()
	registerScript(registry)
	proc, err := registry.BuildNode(domain.ProcessorSpec{
		Type:  "script",
		Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": fileScriptSource(exampleNormalizeNodesPath()),
			"args":   args,
		}),
	})
	require.NoError(t, err)
	return proc
}

func applyExampleNormalizer(t *testing.T, args map[string]any, nodes []domain.NodeIR) domain.NodeProcessOutput {
	t.Helper()
	out, err := buildExampleNormalizer(t, args).ApplyNodes(context.Background(), domain.NodeProcessInput{
		Target:  "mihomo",
		Nodes:   nodes,
		Context: domain.NodeContext{InputName: "example-source"},
	})
	require.NoError(t, err)
	return out
}

func TestExampleNormalizeNodesPipelineWithMetadataIsIdempotent(t *testing.T) {
	reality := &domain.TLSOptions{
		Enabled: true,
		Reality: &domain.RealityOptions{Enabled: true, PublicKey: "public-key", ShortID: "01"},
	}
	nodes := []domain.NodeIR{
		{
			Name: "香港 IPLC 家宽 2x", Type: domain.NodeTypeVLESS, Server: "one.example.com", Port: 443,
			UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none", TLS: reality,
			Meta: map[string]string{"keep": "yes", "normalize.city": "stale"},
		},
		{
			Name: "香港 IPLC 家宽 2x", Type: domain.NodeTypeVLESS, Server: "two.example.com", Port: 443,
			UUID: "22222222-2222-2222-2222-222222222222", Encryption: "none", TLS: reality,
		},
		{
			Name: "另一个名字", Type: domain.NodeTypeVLESS, Server: "one.example.com", Port: 443,
			UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none", TLS: reality,
		},
		{
			Name: "流量剩余 100 GB", Type: domain.NodeTypeShadowsocks, Server: "info.example.com", Port: 8388,
			Cipher: "aes-128-gcm", Password: "secret",
		},
	}

	args := map[string]any{"write_meta": true}
	first := applyExampleNormalizer(t, args, nodes)
	require.Equal(t, []string{
		"🇭🇰 香港 01 IPLC 家宽 2× VLESS",
		"🇭🇰 香港 02 IPLC 家宽 2× VLESS",
	}, []string{first.Nodes[0].Name, first.Nodes[1].Name})
	require.Equal(t, []string{"one.example.com", "two.example.com"}, []string{first.Nodes[0].Server, first.Nodes[1].Server})
	require.Equal(t, map[string]string{
		"keep":                      "yes",
		"normalize.version":         "1",
		"normalize.original_name":   "香港 IPLC 家宽 2x",
		"normalize.region_code":     "HK",
		"normalize.region":          "香港",
		"normalize.region_en":       "Hong Kong",
		"normalize.index":           "01",
		"normalize.line":            "IPLC",
		"normalize.features":        "家宽",
		"normalize.multiplier":      "2",
		"normalize.protocol":        "VLESS",
		"normalize.protocol_detail": "VLESS Reality",
		"normalize.security":        "Reality",
		"normalize.source":          "example-source",
	}, first.Nodes[0].Meta)
	require.ElementsMatch(t, []string{
		"node_normalize_information_filtered",
		"node_normalize_connection_deduped",
	}, warningCodes(first.Warnings))

	second := applyExampleNormalizer(t, args, first.Nodes)
	require.Equal(t, first.Nodes, second.Nodes)
	require.Empty(t, second.Warnings)
}

func TestExampleNormalizeNodesMetadataDefaultsToDisabled(t *testing.T) {
	out := applyExampleNormalizer(t, nil, []domain.NodeIR{{
		Name: "香港", Type: domain.NodeTypeShadowsocks, Server: "one.example.com", Port: 8388,
		Cipher: "aes-128-gcm", Password: "one", Meta: map[string]string{"keep": "yes"},
	}})

	require.Equal(t, map[string]string{"keep": "yes"}, out.Nodes[0].Meta)
}

func TestExampleNormalizeNodesConnectionDedupPreservesSemanticRawFields(t *testing.T) {
	out := applyExampleNormalizer(t, nil, []domain.NodeIR{
		{
			Name: "香港 one", Type: domain.NodeTypeShadowsocks, Server: "same.example.com", Port: 8388,
			Cipher: "aes-128-gcm", Password: "secret", SourceFormat: "mihomo",
			Raw: map[string]json.RawMessage{"provider.option": json.RawMessage(`"one"`)},
		},
		{
			Name: "香港 two", Type: domain.NodeTypeShadowsocks, Server: "same.example.com", Port: 8388,
			Cipher: "aes-128-gcm", Password: "secret", SourceFormat: "sing-box",
			Raw: map[string]json.RawMessage{"provider.option": json.RawMessage(`"two"`)},
		},
	})

	require.Len(t, out.Nodes, 2)
	require.Equal(t, []string{"mihomo", "sing-box"}, []string{out.Nodes[0].SourceFormat, out.Nodes[1].SourceFormat})
}

func TestExampleNormalizeNodesDetailedProtocolAndCustomSeparator(t *testing.T) {
	out := applyExampleNormalizer(t, map[string]any{
		"separator":     " · ",
		"protocol_mode": "detailed",
		"write_meta":    true,
		"template":      "{flag}{separator}{region}{separator}{index}{separator}{city}{separator}{line}{separator}{features}{separator}{multiplier}{separator}{protocol}{separator}{ip_stack}",
	}, []domain.NodeIR{{
		Name: "US LAX CN2-GIA Native GPT ˣ²", Type: domain.NodeTypeVLESS,
		Server: "2001:db8::1", Port: 443, UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none",
		Flow: domain.VLESSFlowVision,
		TLS: &domain.TLSOptions{Enabled: true, Reality: &domain.RealityOptions{
			Enabled: true, PublicKey: "public-key", ShortID: "01",
		}},
		Transport: &domain.TransportOptions{Type: "grpc", ServiceName: "proxy"},
	}})

	require.Equal(t, "🇺🇸 · 美国 · 01 · 洛杉矶 · CN2 GIA · 原生/GPT · 2× · VLESS Reality gRPC Vision · IPv6", out.Nodes[0].Name)
	require.Equal(t, "US LAX CN2-GIA Native GPT ˣ²", out.Nodes[0].Meta["normalize.original_name"])
	require.Equal(t, "洛杉矶", out.Nodes[0].Meta["normalize.city"])
	require.Equal(t, "CN2 GIA", out.Nodes[0].Meta["normalize.line"])
	require.Equal(t, "原生/GPT", out.Nodes[0].Meta["normalize.features"])
	require.Equal(t, "2", out.Nodes[0].Meta["normalize.multiplier"])
	require.Equal(t, "VLESS Reality gRPC Vision", out.Nodes[0].Meta["normalize.protocol_detail"])
	require.Equal(t, "gRPC", out.Nodes[0].Meta["normalize.transport"])
	require.Equal(t, "Vision", out.Nodes[0].Meta["normalize.flow"])
	require.Equal(t, "IPv6", out.Nodes[0].Meta["normalize.ip_stack"])
}

func TestExampleNormalizeNodesDropsFinalNameConflicts(t *testing.T) {
	nodes := []domain.NodeIR{
		{Name: "香港 one", Type: domain.NodeTypeShadowsocks, Server: "one.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "one"},
		{Name: "香港 two", Type: domain.NodeTypeShadowsocks, Server: "two.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "two"},
	}
	out := applyExampleNormalizer(t, map[string]any{"template": "{region}"}, nodes)

	require.Len(t, out.Nodes, 1)
	require.Equal(t, "香港", out.Nodes[0].Name)
	require.Equal(t, "one.example.com", out.Nodes[0].Server)
	require.Contains(t, warningCodes(out.Warnings), "node_normalize_name_deduped")
}

func TestExampleNormalizeNodesCanFailOnFinalNameConflicts(t *testing.T) {
	proc := buildExampleNormalizer(t, map[string]any{"template": "{region}", "name_conflict": "error"})
	_, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "香港 one", Type: domain.NodeTypeShadowsocks, Server: "one.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "one"},
		{Name: "香港 two", Type: domain.NodeTypeShadowsocks, Server: "two.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "two"},
	}})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeScriptRuntime))
	require.ErrorContains(t, err, "最终节点名称重复")
}

func TestExampleNormalizeNodesFilteringUnknownAndCustomTags(t *testing.T) {
	out := applyExampleNormalizer(t, map[string]any{
		"include_regex":     []string{"VLESS|香港"},
		"exclude_regex":     "测试专用",
		"multiplier_filter": "high",
		"unknown_region":    "drop",
		"custom_tags":       map[string]string{"自建": "Private"},
	}, []domain.NodeIR{
		{Name: "香港 自建 3倍", Type: domain.NodeTypeVLESS, Server: "one.example.com", Port: 443, UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none"},
		{Name: "香港 测试专用 3倍", Type: domain.NodeTypeVLESS, Server: "two.example.com", Port: 443, UUID: "22222222-2222-2222-2222-222222222222", Encryption: "none"},
		{Name: "未知 自建 3倍", Type: domain.NodeTypeVLESS, Server: "three.example.com", Port: 443, UUID: "33333333-3333-3333-3333-333333333333", Encryption: "none"},
		{Name: "香港 自建", Type: domain.NodeTypeVLESS, Server: "four.example.com", Port: 443, UUID: "44444444-4444-4444-4444-444444444444", Encryption: "none"},
	})

	require.Len(t, out.Nodes, 1)
	require.Equal(t, "🇭🇰 香港 01 Private 3× VLESS", out.Nodes[0].Name)
	require.Contains(t, warningCodes(out.Warnings), "node_normalize_filtered")
}

func TestExampleNormalizeNodesFormattingAliasesAndMultiplierVariants(t *testing.T) {
	out := applyExampleNormalizer(t, map[string]any{
		"sort":             false,
		"region_style":     "en",
		"show_flag":        false,
		"always_index":     false,
		"unknown_template": "{original}{separator}{protocol}",
	}, []domain.NodeIR{
		{Name: "Hong Kong x0.2", Type: domain.NodeTypeShadowsocks, Server: "one.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "one"},
		{Name: "🇯🇵 ×3", Type: domain.NodeTypeShadowsocks, Server: "two.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "two"},
		{Name: "東京 0.1x", Type: domain.NodeTypeShadowsocks, Server: "three.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "three"},
		{Name: "United Kingdom ˣ¹⁰", Type: domain.NodeTypeShadowsocks, Server: "four.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "four"},
		{Name: "Mystery VLESS", Type: domain.NodeTypeVLESS, Server: "five.example.com", Port: 443, UUID: "55555555-5555-5555-5555-555555555555", Encryption: "none"},
	})

	require.Equal(t, []string{
		"Hong Kong 0.2× SS",
		"Japan 01 3× SS",
		"Japan 02 东京 0.1× SS",
		"United Kingdom 10× SS",
		"Mystery VLESS",
	}, []string{out.Nodes[0].Name, out.Nodes[1].Name, out.Nodes[2].Name, out.Nodes[3].Name, out.Nodes[4].Name})

	second := applyExampleNormalizer(t, map[string]any{
		"sort":             false,
		"region_style":     "en",
		"show_flag":        false,
		"always_index":     false,
		"unknown_template": "{original}{separator}{protocol}",
	}, out.Nodes)
	require.Equal(t, out.Nodes, second.Nodes)
}

func TestExampleNormalizeNodesRegionRegistryAndAllProtocols(t *testing.T) {
	body, err := os.ReadFile(exampleNormalizeNodesPath())
	require.NoError(t, err)
	runtime := goja.New()
	_, err = runtime.RunString(string(body))
	require.NoError(t, err)
	var regions []struct {
		Code string `json:"code"`
		ZH   string `json:"zh"`
	}
	require.NoError(t, runtime.ExportTo(runtime.Get("REGIONS"), &regions))
	require.Len(t, regions, 189)

	regionNodes := make([]domain.NodeIR, 0, len(regions))
	for index, region := range regions {
		regionNodes = append(regionNodes, domain.NodeIR{
			Name: region.Code, Type: domain.NodeTypeShadowsocks,
			Server: fmt.Sprintf("node-%d.example.com", index), Port: uint16(1000 + index),
			Cipher: "aes-128-gcm", Password: fmt.Sprintf("secret-%d", index),
		})
	}
	regionOutput := applyExampleNormalizer(t, map[string]any{"sort": false}, regionNodes)
	require.Len(t, regionOutput.Nodes, 189)
	for index, region := range regions {
		require.Contains(t, regionOutput.Nodes[index].Name, region.ZH)
	}

	protocols := []struct {
		typeName domain.NodeType
		label    string
	}{
		{domain.NodeTypeShadowsocks, "SS"}, {domain.NodeTypeShadowsocksR, "SSR"},
		{domain.NodeTypeVMess, "VMess"}, {domain.NodeTypeVLESS, "VLESS"},
		{domain.NodeTypeTrojan, "Trojan"}, {domain.NodeTypeHysteria, "HY"},
		{domain.NodeTypeHysteria2, "HY2"}, {domain.NodeTypeTUIC, "TUIC"},
		{domain.NodeTypeMieru, "Mieru"}, {domain.NodeTypeSOCKS, "SOCKS"},
		{domain.NodeTypeHTTP, "HTTP"}, {domain.NodeTypeWireGuard, "WG"},
		{domain.NodeTypeSnell, "Snell"}, {domain.NodeTypeAnyTLS, "AnyTLS"},
	}
	protocolNodes := make([]domain.NodeIR, 0, len(protocols))
	for index, protocol := range protocols {
		protocolNodes = append(protocolNodes, domain.NodeIR{
			Name: fmt.Sprintf("香港 %02d", index), Type: protocol.typeName,
			Server: fmt.Sprintf("protocol-%d.example.com", index), Port: uint16(2000 + index),
		})
	}
	protocolOutput := applyExampleNormalizer(t, map[string]any{"sort": false}, protocolNodes)
	for index, protocol := range protocols {
		require.Contains(t, protocolOutput.Nodes[index].Name, protocol.label)
	}
}

func TestExampleNormalizeNodesRegionIndexPreservesMatchingSemantics(t *testing.T) {
	out := applyExampleNormalizer(t, map[string]any{"sort": false}, []domain.NodeIR{
		{Name: "Hysteria2-SG-07", Type: domain.NodeTypeHysteria2, Server: "one.example.com", Port: 443},
		{Name: "US 香港", Type: domain.NodeTypeShadowsocks, Server: "two.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "two"},
		{Name: "edge-SingaporePremium", Type: domain.NodeTypeShadowsocks, Server: "three.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "three"},
		{Name: "prefixHKGsuffix", Type: domain.NodeTypeShadowsocks, Server: "four.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "four"},
	})

	require.Equal(t, []string{
		"🇸🇬 新加坡 01 HY2",
		"🇭🇰 香港 01 SS",
		"🇸🇬 新加坡 02 Edge SS",
		"prefixHKGsuffix SS",
	}, []string{out.Nodes[0].Name, out.Nodes[1].Name, out.Nodes[2].Name, out.Nodes[3].Name})
}

func TestExampleNormalizeNodesRegionIndexAcceptsNumberedShortAliases(t *testing.T) {
	body, err := os.ReadFile(exampleNormalizeNodesPath())
	require.NoError(t, err)
	runtime := goja.New()
	_, err = runtime.RunString(string(body))
	require.NoError(t, err)

	tests := []struct {
		name string
		want string
	}{
		{name: "[Hy2]CCS US1", want: "US"},
		{name: "HK02", want: "HK"},
		{name: "LAX3", want: "US"},
		{name: "USAFAST"},
		{name: "prefixUS1suffix"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := runtime.RunString(fmt.Sprintf("detectRegion(%q)", test.name))
			require.NoError(t, err)
			if test.want == "" {
				require.True(t, goja.IsNull(value))
				return
			}
			require.Equal(t, test.want, value.ToObject(runtime).Get("code").String())
		})
	}

	out := applyExampleNormalizer(t, map[string]any{"sort": false}, []domain.NodeIR{{
		Name: "[Hy2]CCS US1", Type: domain.NodeTypeHysteria2, Server: "us.example.com", Port: 443,
	}})
	require.Len(t, out.Nodes, 1)
	require.Equal(t, "🇺🇸 美国 01 HY2", out.Nodes[0].Name)
}

func TestExampleNormalizeNodesStringCleanupPreservesLiteralSeparatorSemantics(t *testing.T) {
	body, err := os.ReadFile(exampleNormalizeNodesPath())
	require.NoError(t, err)
	runtime := goja.New()
	_, err = runtime.RunString(string(body))
	require.NoError(t, err)

	cleaned, err := runtime.RunString(`cleanRenderedName(".*.*Hong Kong.*", ".*")`)
	require.NoError(t, err)
	require.Equal(t, "Hong Kong", cleaned.String())

	stripped, err := runtime.RunString(`stripGeneratedParts("Hong Kong | VLESS Reality", "", {detailed: "VLESS Reality", base: "VLESS"})`)
	require.NoError(t, err)
	require.Equal(t, "Hong Kong", stripped.String())

	untouched, err := runtime.RunString(`stripGeneratedParts("NotVLESS", "", {detailed: "VLESS", base: "VLESS"})`)
	require.NoError(t, err)
	require.Equal(t, "NotVLESS", untouched.String())
}

func TestExampleNormalizeNodesHandlesLargeSubscriptionWithinTimeout(t *testing.T) {
	const nodeCount = 1725
	const otherCount = 318
	regionCodes := []string{"HK", "JP", "SG", "US", "GB", "DE", "NL"}
	nodes := make([]domain.NodeIR, 0, nodeCount)
	for index := 0; index < nodeCount-otherCount; index++ {
		nodes = append(nodes, domain.NodeIR{
			Name: fmt.Sprintf("%s-%d", regionCodes[index%len(regionCodes)], index),
			Type: domain.NodeTypeShadowsocks, Server: fmt.Sprintf("region-%d.example.com", index),
			Port: 8388, Cipher: "aes-128-gcm", Password: fmt.Sprintf("region-%d", index),
		})
	}
	for index := 0; index < otherCount; index++ {
		nodes = append(nodes, domain.NodeIR{
			Name: fmt.Sprintf("❓Other_%d", index+1),
			Type: domain.NodeTypeShadowsocks, Server: fmt.Sprintf("other-%d.example.com", index),
			Port: 8388, Cipher: "aes-128-gcm", Password: fmt.Sprintf("other-%d", index),
		})
	}

	out := applyExampleNormalizer(t, nil, nodes)
	require.Len(t, out.Nodes, nodeCount)
}
