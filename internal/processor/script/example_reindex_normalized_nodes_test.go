package script_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

func exampleReindexNormalizedNodesPath() string {
	return filepath.Join("..", "..", "..", "examples", "scripts", "reindex-normalized-nodes.js")
}

func applyExampleReindexNormalizedNodes(t *testing.T, args map[string]any, nodes []domain.NodeIR) domain.NodeProcessOutput {
	t.Helper()
	proc := buildExampleReindexNormalizedNodes(t, args)
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: nodes})
	require.NoError(t, err)
	return out
}

func buildExampleReindexNormalizedNodes(t *testing.T, args map[string]any) domain.NodeProcessor {
	t.Helper()
	registry := processor.NewRegistry()
	registerScript(registry)
	proc, err := registry.BuildNode(domain.ProcessorSpec{
		Type:  "script",
		Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": fileScriptSource(exampleReindexNormalizedNodesPath()),
			"args":   args,
		}),
	})
	require.NoError(t, err)
	return proc
}

func TestExampleReindexNormalizedNodesDefaultTemplateMatchesNormalizer(t *testing.T) {
	readDefaultTemplate := func(path string) string {
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		runtime := goja.New()
		_, err = runtime.RunString(string(body))
		require.NoError(t, err)
		return runtime.Get("DEFAULT_TEMPLATE").String()
	}

	require.Equal(t, readDefaultTemplate(exampleNormalizeNodesPath()), readDefaultTemplate(exampleReindexNormalizedNodesPath()))
}

func TestExampleReindexNormalizedNodesConsumesNormalizeTemplateAndSharesSequenceAcrossSources(t *testing.T) {
	normalized := applyExampleNormalizer(t, map[string]any{"separator": " · ", "sort": false}, []domain.NodeIR{
		{Name: "ProviderA 🇭🇰 香港 08 IPLC", Type: domain.NodeTypeShadowsocks, Server: "hk-one.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "one"},
		{Name: "ProviderB 🇭🇰 香港 03 家宽", Type: domain.NodeTypeShadowsocks, Server: "hk-two.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "two"},
		{Name: "ProviderA 🇯🇵 日本 05", Type: domain.NodeTypeShadowsocks, Server: "jp.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "three"},
	})
	reordered := []domain.NodeIR{normalized.Nodes[1], normalized.Nodes[2], normalized.Nodes[0]}

	out := applyExampleReindexNormalizedNodes(t, map[string]any{"separator": " · "}, reordered)
	require.Equal(t, []string{
		"ProviderB · 🇭🇰 · 香港 · 01 · 家宽",
		"ProviderA · 🇯🇵 · 日本 · 01",
		"ProviderA · 🇭🇰 · 香港 · 02 · IPLC",
	}, []string{out.Nodes[0].Name, out.Nodes[1].Name, out.Nodes[2].Name})
	require.Equal(t, []string{
		"hk-two.example.com", "jp.example.com", "hk-one.example.com",
	}, []string{out.Nodes[0].Server, out.Nodes[1].Server, out.Nodes[2].Server})

	second := applyExampleReindexNormalizedNodes(t, map[string]any{"separator": " · "}, out.Nodes)
	require.Equal(t, out.Nodes, second.Nodes)
}

func TestExampleReindexNormalizedNodesUsesSeparatorAndExcludesMatchingNames(t *testing.T) {
	template := "{airport}{separator}{index}{separator}{flag}{separator}{region}{separator}{features}"
	out := applyExampleReindexNormalizedNodes(t, map[string]any{
		"index_width": 3,
		"separator":   " · ",
		"template":    template,
		"exclude_regex": []string{
			"^Personal",
		},
	}, []domain.NodeIR{
		{Name: "ProviderA · 12 · 🇭🇰 · 香港 · IPLC", Type: domain.NodeTypeShadowsocks, Server: "one.example.com", Port: 8388},
		{Name: "Personal · 7 · 🇭🇰 · 香港", Type: domain.NodeTypeShadowsocks, Server: "personal.example.com", Port: 8388},
		{Name: "ProviderB · 99 · 🇭🇰 · 香港 · 家宽", Type: domain.NodeTypeShadowsocks, Server: "two.example.com", Port: 8388},
		{Name: "没有规范化的名称 09", Type: domain.NodeTypeShadowsocks, Server: "three.example.com", Port: 8388},
		{Name: "ProviderA · 5 · 🇯🇵 · 日本 · VLESS", Type: domain.NodeTypeShadowsocks, Server: "four.example.com", Port: 8388},
	})

	require.Equal(t, []string{
		"ProviderA · 001 · 🇭🇰 · 香港 · IPLC",
		"Personal · 7 · 🇭🇰 · 香港",
		"ProviderB · 002 · 🇭🇰 · 香港 · 家宽",
		"没有规范化的名称 09",
		"ProviderA · 001 · 🇯🇵 · 日本 · VLESS",
	}, []string{out.Nodes[0].Name, out.Nodes[1].Name, out.Nodes[2].Name, out.Nodes[3].Name, out.Nodes[4].Name})
	require.Contains(t, warningCodes(out.Warnings), "node_reindex_template_unmatched")
}

func TestExampleReindexNormalizedNodesUsesDefaultTemplateWithSpaceInsideValues(t *testing.T) {
	out := applyExampleReindexNormalizedNodes(t, nil, []domain.NodeIR{
		{Name: "ProviderA 🇭🇰 香港 08 CN2 GIA", Type: domain.NodeTypeShadowsocks, Server: "one.example.com", Port: 8388},
		{Name: "ProviderB 🇭🇰 香港 3 家宽", Type: domain.NodeTypeShadowsocks, Server: "two.example.com", Port: 8388},
	})

	require.Equal(t, []string{
		"ProviderA 🇭🇰 香港 01 CN2 GIA",
		"ProviderB 🇭🇰 香港 02 家宽",
	}, []string{out.Nodes[0].Name, out.Nodes[1].Name})
}

func TestExampleReindexNormalizedNodesRejectsInvalidTemplate(t *testing.T) {
	for _, template := range []string{
		"{flag}{separator}{region}",
		"{airport}{separator}{index}",
		"{unsupported}{separator}{index}{separator}{flag}",
	} {
		t.Run(template, func(t *testing.T) {
			proc := buildExampleReindexNormalizedNodes(t, map[string]any{"template": template})
			_, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "ProviderA 01"}}})
			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeScriptRuntime))
		})
	}
}
