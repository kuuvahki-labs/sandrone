package mcpapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

func TestPublishedSchemasMatchRealProcessorAndFileDecoders(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	for _, descriptor := range rt.Service.Registry().PublicDescriptors() {
		uri := "sandrone://schemas/processors/" + string(descriptor.Stage) + "/" + descriptor.Type
		var document struct {
			ParamsSchema *jsonschema.Schema `json:"params_schema"`
			Examples     []map[string]any   `json:"examples"`
		}
		readJSONResource(t, ctx, session, uri, &document)
		resolved, err := document.ParamsSchema.Resolve(nil)
		require.NoError(t, err, uri)
		for index, example := range document.Examples {
			require.NoError(t, resolved.Validate(example), "%s example %d", uri, index)
			spec := domain.ProcessorSpec{
				Type: descriptor.Type, Stage: descriptor.Stage,
				Params: rawParams(t, example),
			}
			switch descriptor.Stage {
			case domain.StageNodes:
				_, err = rt.Service.Registry().BuildNode(spec)
			case domain.StageFile:
				_, err = rt.Service.Registry().BuildFile(spec)
			default:
				t.Fatalf("unexpected stage %q", descriptor.Stage)
			}
			require.NoError(t, err, "%s example %d", uri, index)
		}
	}

	for _, capability := range rt.Service.FileKindCapabilities() {
		uri := "sandrone://schemas/file-kinds/" + string(capability.Kind)
		var document struct {
			SettingsSchema    *jsonschema.Schema `json:"settings_schema"`
			SettingsSupported bool               `json:"settings_supported"`
			Examples          []map[string]any   `json:"examples"`
		}
		readJSONResource(t, ctx, session, uri, &document)
		if capability.Kind == domain.FileKindStatic {
			require.False(t, document.SettingsSupported)
			require.Nil(t, document.SettingsSchema)
			for _, settings := range []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`{"future":true}`)} {
				spec := domain.FileSpec{
					Name: "static.txt", Kind: domain.FileKindStatic,
					Source: domain.FileSource{Type: "inline", Content: "static"},
					Config: &domain.FileConfig{Settings: settings},
				}
				require.Error(t, rt.Service.PutFile(ctx, spec))
			}
			continue
		}
		require.True(t, document.SettingsSupported)
		resolved, err := document.SettingsSchema.Resolve(nil)
		require.NoError(t, err, uri)
		for index, example := range document.Examples {
			settings := exampleSettings(t, example)
			require.NoError(t, resolved.Validate(settings), "%s example %d", uri, index)
			body, err := json.Marshal(example)
			require.NoError(t, err)
			var spec domain.FileSpec
			require.NoError(t, json.Unmarshal(body, &spec))
			err = rt.Service.PutFile(ctx, spec)
			require.NoError(t, err, "%s example %d", uri, index)
		}
	}
}

func TestPublishedFileSettingsSchemasRejectDecoderInvalidNullAndUnknownFields(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	tests := []struct {
		kind     domain.FileKind
		settings string
	}{
		{kind: domain.FileKindMihomo, settings: `{"groups":null}`},
		{kind: domain.FileKindSingBox, settings: `{"groups":null}`},
		{kind: domain.FileKindShadowrocket, settings: `{"groups":null}`},
		{kind: domain.FileKindShadowrocket, settings: `{"groups":[null]}`},
		{kind: domain.FileKindShadowrocket, settings: `{"groups":[{"name":"Proxy","type":"select","proxies":["DIRECT"],"interval":null}]}`},
		{kind: domain.FileKindMihomo, settings: `{"future":true}`},
		{kind: domain.FileKindSingBox, settings: `{"future":true}`},
		{kind: domain.FileKindShadowrocket, settings: `{"future":true}`},
	}
	for _, test := range tests {
		t.Run(string(test.kind)+"/"+test.settings, func(t *testing.T) {
			var document struct {
				SettingsSchema *jsonschema.Schema `json:"settings_schema"`
			}
			readJSONResource(t, ctx, session, "sandrone://schemas/file-kinds/"+string(test.kind), &document)
			resolved, err := document.SettingsSchema.Resolve(nil)
			require.NoError(t, err)
			var settings any
			require.NoError(t, json.Unmarshal([]byte(test.settings), &settings))
			require.Error(t, resolved.Validate(settings))

			spec := domain.FileSpec{
				Name: string(test.kind), Kind: test.kind,
				Config: &domain.FileConfig{Settings: json.RawMessage(test.settings)},
			}
			require.Error(t, rt.Service.PutFile(ctx, spec))
		})
	}
}

func TestPublishedFileSettingsSchemasAcceptDecoderValidNestedValues(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	tests := []struct {
		kind     domain.FileKind
		settings string
	}{
		{kind: domain.FileKindMihomo, settings: `{"groups":[],"rule_sets":[],"rules":[]}`},
		{kind: domain.FileKindSingBox, settings: `{"groups":[],"rule_sets":[],"rules":[]}`},
		{
			kind: domain.FileKindShadowrocket,
			settings: `{"groups":[{"name":"Proxy","type":"select","proxies":["DIRECT"],"interval":1}],
				"rule_sets":[],"rules":["FINAL,Proxy"]}`,
		},
	}
	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			var document struct {
				SettingsSchema *jsonschema.Schema `json:"settings_schema"`
			}
			readJSONResource(t, ctx, session, "sandrone://schemas/file-kinds/"+string(test.kind), &document)
			resolved, err := document.SettingsSchema.Resolve(nil)
			require.NoError(t, err)
			var settings any
			require.NoError(t, json.Unmarshal([]byte(test.settings), &settings))
			require.NoError(t, resolved.Validate(settings))

			spec := domain.FileSpec{
				Name: string(test.kind) + "-nested", Kind: test.kind,
				Config: &domain.FileConfig{Settings: json.RawMessage(test.settings)},
			}
			require.NoError(t, rt.Service.PutFile(ctx, spec))
		})
	}
}

func TestPublishedFileSettingsSchemasExcludeEditorOnlyAdaptiveGroups(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	for _, kind := range []domain.FileKind{
		domain.FileKindMihomo,
		domain.FileKindShadowrocket,
	} {
		t.Run(string(kind), func(t *testing.T) {
			var document fileKindSchemaDocument
			readJSONResource(t, ctx, session, "sandrone://schemas/file-kinds/"+string(kind), &document)
			properties, ok := document.SettingsSchema["properties"].(map[string]any)
			require.True(t, ok)
			require.NotContains(t, properties, "adaptive_groups")
			require.Contains(t, properties, "groups")
			require.Contains(t, properties, "rule_sets")
			require.Contains(t, properties, "rules")
		})
	}
}

func TestPublishedProcessorSchemasRejectBuilderUnknownFields(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	var document struct {
		ParamsSchema *jsonschema.Schema `json:"params_schema"`
	}
	readJSONResource(t, ctx, session, "sandrone://schemas/processors/nodes/rename", &document)
	resolved, err := document.ParamsSchema.Resolve(nil)
	require.NoError(t, err)
	params := map[string]any{"mode": "prefix", "value": "x", "future": true}
	require.Error(t, resolved.Validate(params))
	_, err = rt.Service.Registry().BuildNode(domain.ProcessorSpec{
		Type: "rename", Stage: domain.StageNodes, Params: rawParams(t, params),
	})
	require.Error(t, err)
}

func TestPublishedSchemasProjectDedicatedConstraintTags(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	var quick processorSchemaDocument
	readJSONResource(t, ctx, session, "sandrone://schemas/processors/nodes/quick_settings", &quick)
	require.Equal(t, "UDP relay override", schemaProperty(t, quick.ParamsSchema, "udp")["description"])
	require.Equal(t, []any{"default", "enabled", "disabled"}, schemaProperty(t, quick.ParamsSchema, "udp")["enum"])
	require.Equal(t, "default", schemaProperty(t, quick.ParamsSchema, "udp")["default"])

	var probe processorSchemaDocument
	readJSONResource(t, ctx, session, "sandrone://schemas/processors/nodes/probe", &probe)
	require.EqualValues(t, 0, schemaProperty(t, probe.ParamsSchema, "timeout_ms")["minimum"])

	var patch processorSchemaDocument
	readJSONResource(t, ctx, session, "sandrone://schemas/processors/file/json_patch", &patch)
	require.EqualValues(t, 1, schemaProperty(t, patch.ParamsSchema, "ops")["minItems"])

	var script processorSchemaDocument
	readJSONResource(t, ctx, session, "sandrone://schemas/processors/nodes/script", &script)
	source := schemaProperty(t, script.ParamsSchema, "source")
	sourceProperties := source["properties"].(map[string]any)
	require.Equal(t, []any{"inline", "file", "remote"}, sourceProperties["type"].(map[string]any)["enum"])
	require.Equal(t, "^[0-9a-f]{64}$", sourceProperties["sha256"].(map[string]any)["pattern"])

	var shadowrocket fileKindSchemaDocument
	readJSONResource(t, ctx, session, "sandrone://schemas/file-kinds/shadowrocket", &shadowrocket)
	groups := schemaProperty(t, shadowrocket.SettingsSchema, "groups")
	group := groups["items"].(map[string]any)
	properties := group["properties"].(map[string]any)
	require.EqualValues(t, 1, properties["interval"].(map[string]any)["minimum"])
	require.EqualValues(t, 86400, properties["interval"].(map[string]any)["maximum"])
}

func TestScriptAPIMetadataMatchesPositionalInvocationAndJSONValues(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "shape-sub", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#positional",
	}))

	var document struct {
		Methods []scriptMethodResource `json:"methods"`
	}
	readJSONResource(t, ctx, session, "sandrone://schemas/script-api/v1", &document)

	for _, name := range []string{"api.json.parse", "api.yaml.parse"} {
		method := scriptMethod(t, document.Methods, name)
		require.Equal(t, "value_or_void", method.Returns.Kind)
		assertSchemaAcceptsJSONValues(t, method.Returns.Schema)
	}
	for _, name := range []string{"api.json.stringify", "api.yaml.stringify"} {
		method := scriptMethod(t, document.Methods, name)
		require.Len(t, method.Arguments, 1)
		assertSchemaAcceptsJSONValues(t, method.Arguments[0].Schema)
	}
	for _, name := range []string{"api.subscription.produce", "api.file.content", "api.probe"} {
		method := scriptMethod(t, document.Methods, name)
		require.False(t, method.Arguments[1].Required)
		assertSchemaAcceptsNull(t, method.Arguments[1].Schema)
	}
	for _, test := range []struct {
		name, arity, zero, returns string
		extraIgnored               bool
	}{
		{name: "api.log", arity: "0+", zero: "returns void", returns: "void", extraIgnored: false},
		{name: "api.warn", arity: "0-1", zero: "returns void without adding a warning", returns: "void", extraIgnored: true},
		{name: "api.yaml.parse", arity: "0-1", zero: "returns void", returns: "value_or_void", extraIgnored: true},
		{name: "api.yaml.stringify", arity: "0-1", zero: "returns empty string", returns: "value", extraIgnored: true},
		{name: "api.json.parse", arity: "0-1", zero: "returns void", returns: "value_or_void", extraIgnored: true},
		{name: "api.json.stringify", arity: "0-1", zero: "returns empty string", returns: "value", extraIgnored: true},
		{name: "api.base64.encode", arity: "0-1", zero: "returns empty string", returns: "value", extraIgnored: true},
		{name: "api.base64.decode", arity: "0-1", zero: "returns empty string", returns: "value", extraIgnored: true},
		{name: "api.hash.sha256", arity: "0-1", zero: "returns empty string", returns: "value", extraIgnored: true},
		{name: "api.subscription.produce", arity: "1-2", zero: "returns invalid_argument error", returns: "value", extraIgnored: true},
		{name: "api.file.content", arity: "1-2", zero: "returns invalid_argument error", returns: "value", extraIgnored: true},
		{name: "api.probe", arity: "1-2", zero: "returns script_runtime error", returns: "value", extraIgnored: true},
	} {
		method := scriptMethod(t, document.Methods, test.name)
		require.Equal(t, test.arity, method.RecommendedArity, test.name)
		require.Equal(t, test.zero, method.ZeroArguments, test.name)
		require.Equal(t, test.returns, method.Returns.Kind, test.name)
		require.Equal(t, test.extraIgnored, method.ExtraArgumentsIgnored, test.name)
	}
	for _, name := range []string{
		"api.warn", "api.yaml.parse", "api.yaml.stringify", "api.json.parse",
		"api.json.stringify", "api.base64.encode", "api.base64.decode", "api.hash.sha256",
	} {
		require.False(t, scriptMethod(t, document.Methods, name).Arguments[0].Required, name)
	}

	var rawDocument struct {
		Methods []map[string]any `json:"methods"`
	}
	readJSONResource(t, ctx, session, "sandrone://schemas/script-api/v1", &rawDocument)
	for _, method := range rawDocument.Methods {
		require.NotContains(t, method, "arity")
		require.Contains(t, method, "recommended_arity")
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sandrone_convert",
		Arguments: map[string]any{
			"from_format": "uri-list",
			"to_format":   "json-nodes",
			"content":     "ss://aes-128-gcm:secret@example.com:8388#before",
			"render_processors": []any{map[string]any{
				"type": "script", "stage": "nodes",
				"params": map[string]any{"source": map[string]any{
					"type": "inline",
					"content": `function main(input, api) {
  if (typeof api.file.content !== "function") throw new Error("file api missing in nodes stage");
  var fileRejected = false;
  try { api.file.content("missing", null); } catch (error) { fileRejected = true; }
  if (!fileRejected) throw new Error("file api node restriction mismatch");
  const produced = [
    api.subscription.produce("shape-sub"),
    api.subscription.produce("shape-sub", null, "ignored"),
    api.subscription.produce("shape-sub", undefined)
  ];
  if (!produced.every((value) => value.kind === "nodes" && value.nodes[0].name === "positional")) throw new Error("positional mismatch");
  if (api.log() !== undefined || api.log("x", 7, null) !== undefined ||
      api.warn() !== undefined || api.warn({message: "shape"}, "ignored") !== undefined) throw new Error("void mismatch");
  const values = [
    api.json.parse("null"), api.json.parse("[1,true]"), api.json.parse("7"),
    api.yaml.parse("null"), api.yaml.parse("- 1\n- true\n"), api.yaml.parse("7"),
    api.json.parse(), api.yaml.parse(), api.json.parse("7", "ignored")
  ];
  if (values[0] !== null || !Array.isArray(values[1]) || values[2] !== 7) throw new Error("json parse mismatch");
  if (values[3] !== null || !Array.isArray(values[4]) || values[5] !== 7) throw new Error("yaml parse mismatch");
  if (values[6] !== undefined || values[7] !== undefined || values[8] !== 7) throw new Error("zero/extra parse mismatch");
  const strings = [api.json.stringify([1,null]), api.json.stringify(7), api.json.stringify(null),
                   api.yaml.stringify([1,null]), api.yaml.stringify(7), api.yaml.stringify(null),
                   api.json.stringify(), api.yaml.stringify(), api.json.stringify(7, "ignored")];
  if (!strings.every((value) => typeof value === "string")) throw new Error("stringify mismatch");
  if (strings[6] !== "" || strings[7] !== "" || strings[8] !== "7") throw new Error("zero/extra stringify mismatch");
  input.nodes[0].name = "shape-ok";
  return input;
}`,
				}},
			}},
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	body, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(body), "shape-ok")
}

func assertSchemaAcceptsNull(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal(raw, &schema))
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	require.NoError(t, resolved.Validate(nil))
}

func assertSchemaAcceptsJSONValues(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal(raw, &schema))
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	for _, value := range []any{nil, true, float64(7), "value", []any{1, nil}, map[string]any{"x": 1}} {
		require.NoError(t, resolved.Validate(value), "%#v", value)
	}
}

func rawParams(t *testing.T, example map[string]any) map[string]json.RawMessage {
	t.Helper()
	params := make(map[string]json.RawMessage, len(example))
	for name, value := range example {
		body, err := json.Marshal(value)
		require.NoError(t, err)
		params[name] = body
	}
	return params
}

func exampleSettings(t *testing.T, example map[string]any) any {
	t.Helper()
	config, ok := example["config"].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	settings, ok := config["settings"]
	if !ok {
		return map[string]any{}
	}
	return settings
}

func schemaProperty(t *testing.T, schema map[string]any, name string) map[string]any {
	t.Helper()
	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	property, ok := properties[name].(map[string]any)
	require.True(t, ok, name)
	return property
}
