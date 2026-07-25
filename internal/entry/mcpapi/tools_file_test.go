package mcpapi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

func TestFileLifecycle(t *testing.T) {
	tests := []struct {
		name          string
		fileName      string
		initialSource string
		updatedSource string
		initialWant   string
		updatedWant   string
		processors    []any
		renderArgs    map[string]string
	}{
		{
			name:          "static",
			fileName:      "agent.txt",
			initialSource: "hello",
			updatedSource: "goodbye",
			initialWant:   "hello:world",
			updatedWant:   "goodbye:world",
			renderArgs:    map[string]string{"name": "world"},
			processors: []any{map[string]any{
				"type": "script", "stage": "file",
				"params": map[string]any{"source": map[string]any{
					"type": "inline",
					"content": `function main(input) {
  input.file.content = input.file.content + ":" + input.args.name;
  return input;
}`,
				}},
			}},
		},
		{
			name:          "mihomo",
			fileName:      "agent.yaml",
			initialSource: "mode: rule\nproxies: []\nproxy-groups: []\nrules: []\n",
			updatedSource: "mode: global\nproxies: []\nproxy-groups: []\nrules: []\n",
			initialWant:   "mode: rule",
			updatedWant:   "mode: global",
		},
		{
			name:          "sing-box",
			fileName:      "agent.json",
			initialSource: `{"log":{"level":"info"},"outbounds":[]}`,
			updatedSource: `{"log":{"level":"debug"},"outbounds":[]}`,
			initialWant:   `"level": "info"`,
			updatedWant:   `"level": "debug"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{
				MCP: app.MCPConfig{AllowManagementTools: true},
			})))
			defer session.Close()

			kind := test.name
			putArgs := filePutArguments(test.fileName, kind, test.initialSource, test.processors)
			put := callToolSuccess(t, ctx, session, "sandrone_put_file", putArgs)
			require.Equal(t, true, put["ok"])
			require.Equal(t, "sandrone://files/"+test.fileName, put["resource_uri"])

			var definition map[string]any
			readJSONResource(t, ctx, session, "sandrone://files/"+test.fileName, &definition)
			require.Equal(t, test.fileName, definition["name"])
			require.Equal(t, kind, definition["kind"])

			spec := callToolSuccess(t, ctx, session, "sandrone_get_file", map[string]any{
				"file": test.fileName,
				"mode": "spec",
			})
			require.Equal(t, "sandrone://files/"+test.fileName, spec["resource_uri"])
			require.Equal(t, test.fileName, spec["spec"].(map[string]any)["name"])
			specSource := spec["spec"].(map[string]any)["source"].(map[string]any)
			require.Equal(t, "inline", specSource["type"])
			require.Equal(t, test.initialSource, specSource["content"])

			source := callToolSuccess(t, ctx, session, "sandrone_get_file", map[string]any{
				"file": test.fileName,
				"mode": "source",
			})
			require.NotContains(t, source, "resource_uri")
			require.NotContains(t, source, "body")
			sourceDocument := source["source"].(map[string]any)
			require.Equal(t, test.fileName, sourceDocument["name"])
			require.Equal(t, kind, sourceDocument["kind"])
			require.NotEmpty(t, sourceDocument["media_type"])
			require.Equal(t, test.initialSource, sourceDocument["content"])
			require.Equal(t, "lifecycle", sourceDocument["meta"].(map[string]any)["scope"])

			renderArguments := map[string]any{
				"file": test.fileName,
				"mode": "render",
			}
			if test.renderArgs != nil {
				renderArguments["args"] = test.renderArgs
			}
			render := callToolSuccess(t, ctx, session, "sandrone_get_file", renderArguments)
			require.Contains(t, render["body"], test.initialWant)
			require.NotContains(t, render, "resource_uri")

			validateArguments := map[string]any{"file": test.fileName}
			if test.renderArgs != nil {
				validateArguments["args"] = test.renderArgs
			}
			validate := callToolSuccess(t, ctx, session, "sandrone_validate_file", validateArguments)
			require.Equal(t, true, validate["ok"])
			require.ElementsMatch(t, []string{"ok", "report"}, mapKeys(validate))

			putArgs = filePutArguments(test.fileName, kind, test.updatedSource, test.processors)
			overwritten := callToolSuccess(t, ctx, session, "sandrone_put_file", putArgs)
			require.Equal(t, "sandrone://files/"+test.fileName, overwritten["resource_uri"])
			renderArguments = map[string]any{
				"file": test.fileName,
				"mode": "render",
			}
			if test.renderArgs != nil {
				renderArguments["args"] = test.renderArgs
			}
			changed := callToolSuccess(t, ctx, session, "sandrone_get_file", renderArguments)
			require.Contains(t, changed["body"], test.updatedWant)

			deleted := callToolSuccess(t, ctx, session, "sandrone_delete_file", map[string]any{"name": test.fileName})
			require.Equal(t, true, deleted["ok"])
			require.Equal(t, true, deleted["deleted"])
			require.Equal(t, "sandrone://files/"+test.fileName, deleted["resource_uri"])

			_, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "sandrone://files/" + test.fileName})
			require.Error(t, err)
		})
	}
}

func filePutArguments(name, kind, content string, processors []any) map[string]any {
	arguments := map[string]any{
		"name": name,
		"kind": kind,
		"source": map[string]any{
			"type":    "inline",
			"content": content,
		},
		"meta": map[string]any{"scope": "lifecycle"},
	}
	if len(processors) > 0 {
		arguments["processors"] = processors
	}
	return arguments
}

func callToolSuccess(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, arguments map[string]any) map[string]any {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	require.NoError(t, err)
	data, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	if result.IsError {
		body, marshalErr := json.Marshal(result)
		require.NoError(t, marshalErr)
		require.FailNow(t, "tool returned error", "%s", body)
	}
	var output map[string]any
	require.NoError(t, json.Unmarshal(data, &output))
	return output
}

func mapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
