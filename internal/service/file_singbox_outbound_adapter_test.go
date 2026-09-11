//go:build probe_singbox

package service_test

import (
	"encoding/json/v2"
	"maps"
	"testing"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceSingBoxOutboundAdapterRefreshesMembersWithoutChangingSpec(t *testing.T) {
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	spec := singBoxOutboundAdapterSpec(t, map[string]any{
		"type": "urltest", "tag": "Proxy", "outbounds": []string{"$nodes"},
		"filter": "(?i)^(hk|香港)", "exclude-filter": "(?i)slow",
		"url": "https://example.com/check", "interval": "5m",
	})
	saved, err := json.Marshal(spec)
	require.NoError(t, err)

	for _, test := range []struct {
		name    string
		nodes   []string
		members []any
	}{
		{"initial", []string{"hk-first", "HK-slow", "香港-二", "JP-other"}, []any{"hk-first", "香港-二"}},
		{"updated", []string{"JP-new", "HK-new", "香港-三"}, []any{"HK-new", "香港-三"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			nodes := make([]domain.NodeIR, 0, len(test.nodes))
			for _, name := range test.nodes {
				nodes = append(nodes, singBoxOutboundAdapterNode(name))
			}
			putSingBoxOutboundAdapterNodes(t, svc, nodes)
			result, err := svc.GetFile(t.Context(), domain.FileRequest{Spec: spec})
			require.NoError(t, err)
			doc := decodeSingBoxCommunityPresetResult(t, result.Content)
			outbounds := requireAnySlice(t, doc["outbounds"])
			group := requireStringMapWithField(t, outbounds, "tag", "Proxy")
			require.Equal(t, test.members, group["outbounds"])
			require.NotContains(t, group, "filter")
			require.NotContains(t, group, "exclude-filter")
			// Filtering membership must retain definitions of excluded nodes.
			for _, name := range test.nodes {
				requireStringMapWithField(t, outbounds, "tag", name)
			}
			after, err := json.Marshal(spec)
			require.NoError(t, err)
			require.JSONEq(t, string(saved), string(after))
			assertSingBoxOutboundAdapterCoreAccepts(t, result.Content)
		})
	}
}

func TestServiceSingBoxOutboundAdapterUsesRenderedOutboundsAndEndpoints(t *testing.T) {
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	putSingBoxOutboundAdapterNodes(t, svc, []domain.NodeIR{
		singBoxOutboundAdapterNode("HK-ss"),
		{
			Name: "HK-skipped", Type: domain.NodeTypeShadowsocksR,
			Server: "192.0.2.2", Port: 8388, Cipher: "aes-128-cfb", Password: "example-password",
			ShadowsocksR: &domain.ShadowsocksROptions{Protocol: "origin", Obfs: "plain"},
		},
		{
			Name: "HK-wg", Type: domain.NodeTypeWireGuard, Server: "192.0.2.3", Port: 51820,
			WireGuard: &domain.WireGuardOptions{
				PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAE=",
				Address:    []string{"10.0.0.2/32"},
				Peers: []domain.WireGuardPeer{{
					Server: "192.0.2.3", Port: 51820,
					PublicKey:  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAI=",
					AllowedIPs: []string{"0.0.0.0/0"},
				}},
			},
		},
	})
	spec := singBoxOutboundAdapterSpec(t, map[string]any{
		"type": "selector", "tag": "Proxy", "outbounds": []string{"$nodes"}, "filter": "^HK-",
	})
	spec.Processors[0].Params["args"] = params(t, map[string]any{"args": map[string]any{"default_outbound": "HK-wg"}})["args"]
	result, err := svc.GetFile(t.Context(), domain.FileRequest{Spec: spec})
	require.NoError(t, err)
	doc := decodeSingBoxCommunityPresetResult(t, result.Content)
	require.Equal(t, "HK-wg", doc["route"].(map[string]any)["final"])
	group := requireStringMapWithField(t, requireAnySlice(t, doc["outbounds"]), "tag", "Proxy")
	require.Equal(t, []any{"HK-ss", "HK-wg"}, group["outbounds"])
	requireStringMapWithField(t, requireAnySlice(t, doc["endpoints"]), "tag", "HK-wg")
	require.NotContains(t, string(result.Content), "HK-skipped")
	require.Condition(t, func() bool {
		for _, warning := range result.Report.Warnings {
			if warning.Node == "HK-skipped" && warning.Code == "render_node_skipped" {
				return true
			}
		}
		return false
	}, "unsupported node must be skipped by the renderer, not by parsing")
	// Constructing a WireGuard device requires with_gvisor, which is not part
	// of the default test tags. Validate its core schema without creating one.
	var options option.Options
	require.NoError(t, options.UnmarshalJSONContext(include.Context(t.Context()), result.Content))
}

func TestServiceSingBoxOutboundAdapterPreservesProcessorOrderAndIsRepeatable(t *testing.T) {
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	putSingBoxOutboundAdapterNodes(t, svc, []domain.NodeIR{
		singBoxOutboundAdapterNode("HK-first"), singBoxOutboundAdapterNode("HK-second"), singBoxOutboundAdapterNode("JP-other"),
	})
	spec := singBoxOutboundAdapterSpec(t, map[string]any{
		"type": "selector", "tag": "Proxy", "outbounds": []string{"$nodes"}, "filter": "^HK-", "default": "HK-second",
	})
	filter := spec.Processors[0]
	before := singBoxOutboundAdapterScriptProcessor(t, "Reorder candidates", `function main(input, api) {
		const doc = api.json.parse(input.file.content);
		const group = doc.outbounds.find(function (outbound) { return outbound.tag === "Proxy"; });
		if (group.filter !== "^HK-" || group.outbounds.indexOf("JP-other") < 0) throw new Error("filter ran before preceding processor");
		group.outbounds = ["HK-second", "JP-other", "HK-first", "HK-second"];
		input.file.content = api.json.stringify(doc);
		return input;
	}`)
	after := singBoxOutboundAdapterScriptProcessor(t, "Check resolved members", `function main(input, api) {
		const doc = api.json.parse(input.file.content);
		const group = doc.outbounds.find(function (outbound) { return outbound.tag === "Proxy"; });
		if (Object.prototype.hasOwnProperty.call(group, "filter")) throw new Error("filter ran after following processor");
		if (group.outbounds.join(",") !== "HK-second,HK-first") throw new Error("members were not filtered in order");
		input.file.content = api.json.stringify(doc);
		return input;
	}`)
	spec.Processors = []domain.ProcessorSpec{before, filter, after, filter}
	result, err := svc.GetFile(t.Context(), domain.FileRequest{Spec: spec})
	require.NoError(t, err)
	doc := decodeSingBoxCommunityPresetResult(t, result.Content)
	group := requireStringMapWithField(t, requireAnySlice(t, doc["outbounds"]), "tag", "Proxy")
	require.Equal(t, []any{"HK-second", "HK-first"}, group["outbounds"])
	require.Equal(t, "HK-second", group["default"])
	assertSingBoxOutboundAdapterCoreAccepts(t, result.Content)
}

func TestServiceSingBoxOutboundAdapterRejectsInvalidGroupsWithoutPartialResult(t *testing.T) {
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	putSingBoxOutboundAdapterNodes(t, svc, []domain.NodeIR{singBoxOutboundAdapterNode("HK-node"), singBoxOutboundAdapterNode("JP-node")})
	for _, test := range []struct {
		name    string
		options map[string]any
	}{
		{"invalid include", map[string]any{"filter": "["}},
		{"invalid exclude", map[string]any{"filter": ".*", "exclude-filter": "["}},
		{"empty match", map[string]any{"filter": "^US"}},
		{"case sensitive by default", map[string]any{"filter": "^hk"}},
		{"exclude all", map[string]any{"filter": ".*", "exclude-filter": ".*"}},
		{"invalid default", map[string]any{"filter": "^HK", "default": "JP-node"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			group := map[string]any{"type": "selector", "tag": "香港选择", "outbounds": []string{"$nodes"}}
			maps.Copy(group, test.options)
			spec := singBoxOutboundAdapterSpec(t, group)
			result, err := svc.GetFile(t.Context(), domain.FileRequest{Spec: spec})
			require.Nil(t, result)
			require.ErrorContains(t, err, "香港选择")
			require.True(t, domain.IsCode(err, domain.CodeScriptRuntime), "got %v", err)
		})
	}
}

func singBoxOutboundAdapterSpec(t *testing.T, group map[string]any) *domain.FileSpec {
	t.Helper()
	return &domain.FileSpec{
		Name: "regex-groups.json", Kind: domain.FileKindSingBox,
		Config: &domain.FileConfig{
			Subscriptions: []string{"regex-nodes"},
			Settings:      completeTypedSettings(t, map[string]any{"groups": []map[string]any{group}}),
		},
		Processors: []domain.ProcessorSpec{singBoxOutboundAdapterScriptProcessor(t, "sing-box 出站配置适配", communityPresetRawScript(t, "sing-box-outbound-adapter.js"))},
	}
}

func singBoxOutboundAdapterScriptProcessor(t *testing.T, name, script string) domain.ProcessorSpec {
	t.Helper()
	return domain.ProcessorSpec{
		Name: name, Type: "script", Stage: domain.StageFile,
		Params: params(t, map[string]any{"source": inlineScriptSource(script)}),
	}
}

func singBoxOutboundAdapterNode(name string) domain.NodeIR {
	return domain.NodeIR{
		Name: name, Type: domain.NodeTypeShadowsocks, Server: "192.0.2.1", Port: 8388,
		Cipher: "aes-128-gcm", Password: "example-password",
	}
}

func putSingBoxOutboundAdapterNodes(t *testing.T, svc *service.Service, nodes []domain.NodeIR) {
	t.Helper()
	content, err := json.Marshal(map[string]any{"nodes": nodes})
	require.NoError(t, err)
	require.NoError(t, svc.PutSubscription(t.Context(), domain.Subscription{
		Name: "regex-nodes", Type: domain.SubscriptionTypeLocal, Format: "json-nodes", Content: string(content),
	}))
}

func assertSingBoxOutboundAdapterCoreAccepts(t *testing.T, content []byte) {
	t.Helper()
	ctx := include.Context(t.Context())
	var options option.Options
	require.NoError(t, options.UnmarshalJSONContext(ctx, content))
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	require.NoError(t, err)
	require.NoError(t, instance.Close())
}
