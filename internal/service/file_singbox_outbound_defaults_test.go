//go:build probe_singbox

package service_test

import (
	"encoding/json/v2"
	"testing"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceSingBoxOutboundAdapterSetsOnlyExplicitDefault(t *testing.T) {
	for _, test := range []struct {
		name string
		args map[string]any
		base string
		want any
	}{
		{"missing parameter preserves final", nil, `{"route":{"final":"existing"}}`, "existing"},
		{"missing parameter preserves absence", nil, `{}`, nil},
		{"empty parameter preserves final", map[string]any{"default_outbound": ""}, `{"route":{"final":"existing"}}`, "existing"},
		{"blank parameter preserves absence", map[string]any{"default_outbound": " \t"}, `{}`, nil},
		{"selector overrides final", map[string]any{"default_outbound": "Manual"}, `{"route":{"final":"existing"}}`, "Manual"},
		{"ordinary outbound overrides final", map[string]any{"default_outbound": "direct"}, `{}`, "direct"},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := domain.FileSpec{
				Name: "defaults.json", Kind: domain.FileKindSingBox,
				Source: domain.FileSource{Type: "inline", Content: test.base},
				Config: &domain.FileConfig{Settings: completeTypedSettings(t, map[string]any{
					"groups": []map[string]any{{"type": "selector", "tag": "Manual", "outbounds": []string{"direct"}}},
				})},
				Processors: []domain.ProcessorSpec{singBoxOutboundAdapterProcessor(t, test.args)},
			}
			before, err := json.Marshal(spec)
			require.NoError(t, err)
			result, err := service.New().GetFile(t.Context(), domain.FileRequest{Spec: &spec})
			require.NoError(t, err)
			doc := decodeSingBoxCommunityPresetResult(t, result.Content)
			route := doc["route"].(map[string]any)
			require.Equal(t, test.want, route["final"])
			if test.want == nil {
				require.NotContains(t, route, "final")
			}
			after, err := json.Marshal(spec)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after))
		})
	}
}

func TestServiceSingBoxOutboundAdapterRejectsInvalidDefaultWithoutPartialResult(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		args map[string]any
	}{
		{"missing target", `{"outbounds":[]}`, map[string]any{"default_outbound": "missing"}},
		{"duplicate outbounds", `{"outbounds":[{"tag":"duplicate"},{"tag":"duplicate"}]}`, map[string]any{"default_outbound": "duplicate"}},
		{"duplicate across endpoint and outbound", `{"outbounds":[{"tag":"duplicate"}],"endpoints":[{"tag":"duplicate"}]}`, map[string]any{"default_outbound": "duplicate"}},
		{"invalid parameter", `{"outbounds":[]}`, map[string]any{"default_outbound": 7}},
		{"invalid route", `{"outbounds":[{"tag":"direct"}],"route":[]}`, map[string]any{"default_outbound": "direct"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := domain.FileSpec{
				Name: "defaults.json", Kind: domain.FileKindStatic,
				Source:     domain.FileSource{Type: "inline", Content: test.body},
				Processors: []domain.ProcessorSpec{singBoxOutboundAdapterProcessor(t, test.args)},
			}
			result, err := service.New().GetFile(t.Context(), domain.FileRequest{Spec: &spec})
			require.Nil(t, result)
			require.True(t, domain.IsCode(err, domain.CodeScriptRuntime), "got %v", err)
		})
	}
}

func TestServiceSingBoxOutboundAdapterUsesProcessorOrderForDefault(t *testing.T) {
	adapt := singBoxOutboundAdapterProcessor(t, map[string]any{"default_outbound": "created"})
	create := singBoxOutboundAdapterScriptProcessor(t, "Create target", `function main(input, api) {
		const doc = api.json.parse(input.file.content);
		doc.outbounds.push({type: "direct", tag: "created"});
		input.file.content = api.json.stringify(doc);
		return input;
	}`)
	spec := domain.FileSpec{
		Name: "ordered.json", Kind: domain.FileKindStatic,
		Source:     domain.FileSource{Type: "inline", Content: `{"outbounds":[]}`},
		Processors: []domain.ProcessorSpec{adapt, create},
	}
	svc := service.New()
	result, err := svc.GetFile(t.Context(), domain.FileRequest{Spec: &spec})
	require.Nil(t, result)
	require.True(t, domain.IsCode(err, domain.CodeScriptRuntime), "got %v", err)
	spec.Processors = []domain.ProcessorSpec{create, adapt, adapt}
	result, err = svc.GetFile(t.Context(), domain.FileRequest{Spec: &spec})
	require.NoError(t, err)
	doc := decodeSingBoxCommunityPresetResult(t, result.Content)
	require.Equal(t, "created", doc["route"].(map[string]any)["final"])
	assertSingBoxOutboundDefaultStarts(t, result.Content)
}

func TestServiceSingBoxOutboundAdapterRejectsRequestDefaultOverride(t *testing.T) {
	spec := domain.FileSpec{
		Name: "defaults.json", Kind: domain.FileKindStatic,
		Source:     domain.FileSource{Type: "inline", Content: `{"outbounds":[{"type":"direct","tag":"direct"}]}`},
		Processors: []domain.ProcessorSpec{singBoxOutboundAdapterProcessor(t, map[string]any{"default_outbound": "direct"})},
	}
	result, err := service.New().GetFile(t.Context(), domain.FileRequest{
		Spec: &spec, Request: domain.RequestInfo{Args: map[string]string{"default_outbound": "direct"}},
	})
	require.Nil(t, result)
	require.True(t, domain.IsCode(err, domain.CodeScriptRuntime), "got %v", err)
	require.ErrorContains(t, err, "request")
}

func singBoxOutboundAdapterProcessor(t *testing.T, args map[string]any) domain.ProcessorSpec {
	t.Helper()
	processor := singBoxOutboundAdapterScriptProcessor(t, "Outbound adaptation", communityPresetRawScript(t, "sing-box-outbound-adapter.js"))
	if args != nil {
		processor.Params["args"] = params(t, map[string]any{"args": args})["args"]
	}
	return processor
}

func assertSingBoxOutboundDefaultStarts(t *testing.T, content []byte) {
	t.Helper()
	ctx := include.Context(t.Context())
	var options option.Options
	require.NoError(t, options.UnmarshalJSONContext(ctx, content))
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, instance.Close()) })
	require.NoError(t, instance.Start())
}
