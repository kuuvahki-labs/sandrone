package mcpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

func TestResourceDiscoveryListsFixedResourcesAndDefinitionTemplates(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	resources, err := session.ListResources(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, []string{
		"sandrone://capabilities",
		"sandrone://schemas/processors",
		"sandrone://schemas/script-api/v1",
	}, resourceURIs(resources.Resources))

	templates, err := session.ListResourceTemplates(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, []string{
		"sandrone://files/{name}",
		"sandrone://schemas/file-kinds/{kind}",
		"sandrone://schemas/processors/{stage}/{type}",
		"sandrone://subscriptions/{name}",
	}, resourceTemplateURIs(templates.ResourceTemplates))
}

func TestSchemaResourcesExposeOwnerCatalogs(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	var summary struct {
		Processors []struct {
			Type  string `json:"type"`
			Stage string `json:"stage"`
			URI   string `json:"uri"`
		} `json:"processors"`
	}
	readJSONResource(t, ctx, session, "sandrone://schemas/processors", &summary)
	require.NotEmpty(t, summary.Processors)

	seen := map[string]bool{}
	for _, item := range summary.Processors {
		require.Contains(t, []string{"nodes", "file"}, item.Stage)
		require.Equal(t, "sandrone://schemas/processors/"+item.Stage+"/"+item.Type, item.URI)
		require.NotEqual(t, "inject_nodes", item.Type)

		var detail processorSchemaDocument
		readJSONResource(t, ctx, session, item.URI, &detail)
		require.Equal(t, item.Type, detail.Type)
		require.Equal(t, item.Stage, detail.Stage)
		require.NotEmpty(t, detail.Description)
		require.Equal(t, "object", detail.ParamsSchema["type"])
		require.NotNil(t, detail.Effects)
		require.NotEmpty(t, detail.Examples)
		require.NotEmpty(t, detail.ErrorCodes)
		seen[item.Stage+":"+item.Type] = true
	}
	require.True(t, seen["nodes:script"])
	require.True(t, seen["file:script"])

	for _, kind := range []string{"static", "mihomo", "sing-box", "shadowrocket"} {
		var detail fileKindSchemaDocument
		readJSONResource(t, ctx, session, "sandrone://schemas/file-kinds/"+kind, &detail)
		require.Equal(t, kind, detail.Kind)
		require.NotEmpty(t, detail.Description)
		require.NotNil(t, detail.SourceRules["required"])
		require.NotEmpty(t, detail.SourceRules["allowed_types"])
		require.NotEmpty(t, detail.Examples)
		if kind == "static" {
			require.False(t, detail.SettingsSupported)
			require.Nil(t, detail.SettingsSchema)
		} else {
			require.True(t, detail.SettingsSupported)
			require.Equal(t, "object", detail.SettingsSchema["type"])
		}
	}

	var script struct {
		Version      int                       `json:"version"`
		ConfigSchema map[string]any            `json:"config_schema"`
		Envelopes    map[string]map[string]any `json:"envelopes"`
		Methods      []scriptMethodResource    `json:"methods"`
		Sources      []struct {
			Type       string `json:"type"`
			Controlled bool   `json:"controlled"`
		} `json:"sources"`
	}
	readJSONResource(t, ctx, session, "sandrone://schemas/script-api/v1", &script)
	require.Equal(t, 1, script.Version)
	require.Equal(t, "object", script.ConfigSchema["type"])
	require.Contains(t, script.Envelopes, "nodes")
	require.Contains(t, script.Envelopes, "file")
	require.NotEmpty(t, script.Methods)
	for _, method := range script.Methods {
		require.NotEmpty(t, method.Name)
		require.NotEmpty(t, method.Stages)
		require.NotNil(t, method.Arguments)
		require.NotEmpty(t, method.RecommendedArity)
		require.NotEmpty(t, method.ZeroArguments)
		require.NotEmpty(t, method.Returns.Kind)
		require.NotNil(t, method.ErrorCodes)
	}
	requireScriptArguments(t, script.Methods, "api.subscription.produce", []string{"name", "options"}, []bool{true, false}, "1-2")
	requireScriptArguments(t, script.Methods, "api.file.content", []string{"name", "options"}, []bool{true, false}, "1-2")
	requireScriptArguments(t, script.Methods, "api.probe", []string{"nodes", "options"}, []bool{true, false}, "1-2")
	require.Equal(t, []string{"nodes", "file"}, scriptMethod(t, script.Methods, "api.file.content").Stages)
	require.NotEmpty(t, scriptMethod(t, script.Methods, "api.file.content").RuntimeRequirement)
	require.Equal(t, "void", scriptMethod(t, script.Methods, "api.log").Returns.Kind)
	require.Equal(t, "void", scriptMethod(t, script.Methods, "api.warn").Returns.Kind)
	require.NotEmpty(t, scriptMethodErrorCodes(t, script.Methods, "api.subscription.produce"))
	require.NotEmpty(t, scriptMethodErrorCodes(t, script.Methods, "api.file.content"))
	require.NotEmpty(t, scriptMethodErrorCodes(t, script.Methods, "api.probe"))
	require.Equal(t, []string{"inline", "file", "remote"}, scriptSourceTypes(script.Sources))
	for _, source := range script.Sources {
		require.True(t, source.Controlled)
	}
}

func TestHTTPAndMCPPublishEquivalentSchemaDocuments(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{
		MCP: app.MCPConfig{AllowManagementTools: true},
	})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()
	httpHandler := httpapi.New(rt).Handler()

	toolsResult, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	tools := make(map[string]*mcp.Tool, len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		tools[tool.Name] = tool
	}

	tests := []struct {
		name     string
		httpPath string
		mcpURI   string
		mcpTool  string
	}{
		{
			name:     "processor summary",
			httpPath: "/v1/schemas/processors",
			mcpURI:   "sandrone://schemas/processors",
		},
		{
			name:     "script API",
			httpPath: "/v1/schemas/script-api/v1",
			mcpURI:   "sandrone://schemas/script-api/v1",
		},
		{
			name:     "subscription",
			httpPath: "/v1/schemas/subscription",
			mcpTool:  "sandrone_put_subscription",
		},
		{
			name:     "file spec",
			httpPath: "/v1/schemas/file-spec",
			mcpTool:  "sandrone_put_file",
		},
	}
	for _, descriptor := range rt.Service.Registry().PublicDescriptors() {
		tests = append(tests, struct {
			name     string
			httpPath string
			mcpURI   string
			mcpTool  string
		}{
			name:     "processor " + string(descriptor.Stage) + "/" + descriptor.Type,
			httpPath: "/v1/schemas/processors/" + string(descriptor.Stage) + "/" + descriptor.Type,
			mcpURI:   "sandrone://schemas/processors/" + string(descriptor.Stage) + "/" + descriptor.Type,
		})
	}
	for _, capability := range rt.Service.FileKindCapabilities() {
		tests = append(tests, struct {
			name     string
			httpPath string
			mcpURI   string
			mcpTool  string
		}{
			name:     "file kind " + string(capability.Kind),
			httpPath: "/v1/schemas/file-kinds/" + string(capability.Kind),
			mcpURI:   "sandrone://schemas/file-kinds/" + string(capability.Kind),
		})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			httpResponse := httptest.NewRecorder()
			httpHandler.ServeHTTP(httpResponse, httptest.NewRequest(http.MethodGet, test.httpPath, nil))
			require.Equal(t, http.StatusOK, httpResponse.Code)

			var mcpBody []byte
			if test.mcpURI != "" {
				resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: test.mcpURI})
				require.NoError(t, err)
				require.Len(t, resource.Contents, 1)
				mcpBody = []byte(resource.Contents[0].Text)
			} else {
				tool := tools[test.mcpTool]
				require.NotNil(t, tool, test.mcpTool)
				mcpBody, err = json.Marshal(tool.InputSchema)
				require.NoError(t, err)
			}

			require.Equal(
				t,
				decodeJSONDocument(t, mcpBody),
				decodeJSONDocument(t, httpResponse.Body.Bytes()),
			)
		})
	}
}

func decodeJSONDocument(t *testing.T, body []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var document any
	require.NoError(t, decoder.Decode(&document))
	return document
}

func scriptMethodErrorCodes(t *testing.T, methods []scriptMethodResource, name string) []string {
	t.Helper()
	return scriptMethod(t, methods, name).ErrorCodes
}

func requireScriptArguments(t *testing.T, methods []scriptMethodResource, name string, want []string, required []bool, recommendedArity string) {
	t.Helper()
	method := scriptMethod(t, methods, name)
	names := make([]string, len(method.Arguments))
	for index, argument := range method.Arguments {
		require.Equal(t, index, argument.Position)
		require.NotNil(t, argument.Schema)
		require.Equal(t, required[index], argument.Required)
		names[index] = argument.Name
	}
	require.Equal(t, want, names)
	require.Equal(t, recommendedArity, method.RecommendedArity)
	require.True(t, method.ExtraArgumentsIgnored)
}

func scriptMethod(t *testing.T, methods []scriptMethodResource, name string) scriptMethodResource {
	t.Helper()
	for _, method := range methods {
		if method.Name == name {
			return method
		}
	}
	require.FailNow(t, "script method not found", name)
	return scriptMethodResource{}
}

func TestSchemaResourcesRejectNonCanonicalTemplateKeys(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	for _, uri := range []string{
		"sandrone://schemas/processors/parse/filter",
		"sandrone://schemas/processors/nodes/inject_nodes",
		"sandrone://schemas/processors/nodes/filter%2Ffuture",
		"sandrone://schemas/file-kinds/future",
		"sandrone://schemas/file-kinds/mihomo%2Ffuture",
	} {
		_, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
		require.Error(t, err, uri)
	}
}

func readJSONResource(t *testing.T, ctx context.Context, session *mcp.ClientSession, uri string, out any) {
	t.Helper()
	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	require.NoError(t, err, uri)
	require.Len(t, resource.Contents, 1, uri)
	require.Equal(t, "application/json", resource.Contents[0].MIMEType)
	require.NoError(t, json.Unmarshal([]byte(resource.Contents[0].Text), out), uri)
}

func resourceURIs(resources []*mcp.Resource) []string {
	uris := make([]string, len(resources))
	for index, resource := range resources {
		uris[index] = resource.URI
	}
	return uris
}

func resourceTemplateURIs(templates []*mcp.ResourceTemplate) []string {
	uris := make([]string, len(templates))
	for index, template := range templates {
		uris[index] = template.URITemplate
	}
	return uris
}

func scriptSourceTypes(sources []struct {
	Type       string `json:"type"`
	Controlled bool   `json:"controlled"`
}) []string {
	types := make([]string, len(sources))
	for index, source := range sources {
		types[index] = source.Type
	}
	return types
}

type processorSchemaDocument struct {
	Type         string           `json:"type"`
	Stage        string           `json:"stage"`
	Description  string           `json:"description"`
	ParamsSchema map[string]any   `json:"params_schema"`
	Effects      map[string]any   `json:"effects"`
	Examples     []map[string]any `json:"examples"`
	ErrorCodes   []string         `json:"error_codes"`
}

type fileKindSchemaDocument struct {
	Kind              string           `json:"kind"`
	Description       string           `json:"description"`
	SettingsSupported bool             `json:"settings_supported"`
	SettingsSchema    map[string]any   `json:"settings_schema"`
	SourceRules       map[string]any   `json:"source_rules"`
	Defaults          map[string]any   `json:"defaults"`
	Examples          []map[string]any `json:"examples"`
}

type scriptMethodResource struct {
	Name               string   `json:"name"`
	Stages             []string `json:"stages"`
	RuntimeRequirement string   `json:"runtime_requirement"`
	Arguments          []struct {
		Position int             `json:"position"`
		Name     string          `json:"name"`
		Schema   json.RawMessage `json:"schema"`
		Required bool            `json:"required"`
	} `json:"arguments"`
	RecommendedArity      string `json:"recommended_arity"`
	ExtraArgumentsIgnored bool   `json:"extra_arguments_ignored"`
	ZeroArguments         string `json:"zero_arguments"`
	Returns               struct {
		Kind   string          `json:"kind"`
		Schema json.RawMessage `json:"schema"`
	} `json:"returns"`
	ErrorCodes []string `json:"error_codes"`
}
