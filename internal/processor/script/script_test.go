package script_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/processor/script"
)

func params(t *testing.T, m map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	for k, v := range m {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		out[k] = b
	}
	return out
}

func registerScript(r *processor.Registry, opts ...script.RegisterOption) {
	loader := script.WithLoader(func(_ context.Context, source script.ScriptSource) (string, string, error) {
		body, err := os.ReadFile(source.Name)
		return string(body), source.Name, err
	})
	script.Register(r, append([]script.RegisterOption{loader}, opts...)...)
}

type scriptProbeRunner struct {
	requests []domain.ProbeRequest
	result   *domain.ProbeResult
	err      error
}

func (r *scriptProbeRunner) Probe(_ context.Context, req domain.ProbeRequest) (*domain.ProbeResult, error) {
	r.requests = append(r.requests, req)
	return r.result, r.err
}

type blockingScriptProbeRunner struct{}

func (r *blockingScriptProbeRunner) Probe(ctx context.Context, _ domain.ProbeRequest) (*domain.ProbeResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestScriptNodesFilterRename(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": fileScriptSource(filepath.Join("testdata", "nodes_filter_rename.js")),
		}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{
		Target: "mihomo",
		Nodes: []domain.NodeIR{
			{Name: "a", Type: domain.NodeTypeShadowsocks, Server: "x", Port: 1, Cipher: "aes-128-gcm", Password: "p"},
			{Name: "b", Type: domain.NodeTypeHTTP, Server: "x", Port: 80},
			{Name: "c", Type: domain.NodeTypeVMess, Server: "x", Port: 443, UUID: "11111111-1111-1111-1111-111111111111"},
		},
	})
	require.NoError(t, err)
	require.Len(t, out.Nodes, 2)
	require.Equal(t, "[mihomo]a", out.Nodes[0].Name)
	require.Equal(t, "[mihomo]c", out.Nodes[1].Name)

	require.NotNil(t, out.Nodes[0].Raw)
	require.Contains(t, out.Nodes[0].Raw, "script.ext.touched_by")

	require.NotContains(t, warningCodes(out.Nodes[0].Warnings), "script_modified")
}

func TestScriptNodeRuntimeIDSurvivesObjectSpreadWithoutPublicField(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(`function main(input) {
  const node = input.nodes[0];
  if (Object.keys(node).some(function(key) { return key.includes("runtime"); })) {
    throw new Error("runtime ID leaked through Object.keys");
  }
  if (JSON.stringify(node).includes("runtime-1")) {
    throw new Error("runtime ID leaked through JSON");
  }
  input.nodes = [{...node, name: "renamed"}];
  return input;
}`),
		}),
	})
	require.NoError(t, err)
	node := domain.NodeIR{
		Name: "original", Type: domain.NodeTypeShadowsocks, Server: "example.com",
		Port: 443, Cipher: "aes-128-gcm", Password: "secret",
	}
	domain.SetNodeRuntimeID(&node, "runtime-1")

	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{node}})

	require.NoError(t, err)
	require.Len(t, out.Nodes, 1)
	require.Equal(t, "renamed", out.Nodes[0].Name)
	require.Equal(t, "runtime-1", domain.NodeRuntimeID(out.Nodes[0]))
}

func TestScriptRebuiltNodeDoesNotInheritRuntimeID(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(`function main(input) {
  const node = input.nodes[0];
  input.nodes = [{
    name: "rebuilt",
    type: node.type,
    server: node.server,
    port: node.port,
    cipher: node.cipher,
    password: node.password
  }];
  return input;
}`),
		}),
	})
	require.NoError(t, err)
	node := domain.NodeIR{
		Name: "original", Type: domain.NodeTypeShadowsocks, Server: "example.com",
		Port: 443, Cipher: "aes-128-gcm", Password: "secret",
	}
	domain.SetNodeRuntimeID(&node, "runtime-1")

	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{node}})

	require.NoError(t, err)
	require.Len(t, out.Nodes, 1)
	require.Equal(t, "rebuilt", out.Nodes[0].Name)
	require.Empty(t, domain.NodeRuntimeID(out.Nodes[0]))
}

func TestScriptLoadsFileSourceAtExecutionWithContext(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "execution")
	calls := 0
	r := processor.NewRegistry()
	registerScript(r, script.WithLoader(func(loadCtx context.Context, source script.ScriptSource) (string, string, error) {
		calls++
		require.Equal(t, "execution", loadCtx.Value(contextKey{}))
		require.Equal(t, "rename.js", source.Name)
		return `function main(input) {
  input.nodes.forEach(function(node) { node.name = "runtime-" + node.name; });
  return input;
}`, source.Name, nil
	}))

	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type:  "script",
		Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": fileScriptSource("rename.js"),
		}),
	})
	require.NoError(t, err)
	require.Zero(t, calls)

	out, err := proc.ApplyNodes(ctx, domain.NodeProcessInput{
		Nodes: []domain.NodeIR{{Name: "a", Type: domain.NodeTypeShadowsocks}},
	})

	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Equal(t, "runtime-a", out.Nodes[0].Name)
}

func TestScriptFileAppendsGroups(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageFile,
		Params: params(t, map[string]any{
			"source": fileScriptSource(filepath.Join("testdata", "file_append_groups.js")),
		}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "mihomo", Content: []byte("mode: rule\nproxies: []\n")},
		Parts: []domain.FilePart{
			{Name: "nodes", Role: "nodes", Kind: "yaml", Nodes: []domain.NodeIR{
				{Name: "alpha", Type: domain.NodeTypeShadowsocks},
				{Name: "beta", Type: domain.NodeTypeVMess},
			}},
		},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "AUTO")
	require.Contains(t, string(out.File.Content), "alpha")
	require.Contains(t, string(out.File.Content), "beta")
}

func TestScriptTimeout(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source":     fileScriptSource(filepath.Join("testdata", "timeout.js")),
			"timeout_ms": 100,
		}),
	})
	require.NoError(t, err)
	_, err = proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "a", Type: domain.NodeTypeShadowsocks}}})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeScriptTimeout))
}

func TestScriptDefaultTimeoutCanBeInjectedAndExplicitTimeoutWins(t *testing.T) {
	t.Run("injected default", func(t *testing.T) {
		r := processor.NewRegistry()
		registerScript(r, script.WithDefaultTimeout(func() time.Duration { return 10 * time.Millisecond }))
		proc, err := r.BuildNode(domain.ProcessorSpec{
			Type: "script", Stage: domain.StageNodes,
			Params: params(t, map[string]any{"source": fileScriptSource(filepath.Join("testdata", "timeout.js"))}),
		})
		require.NoError(t, err)

		_, err = proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "a", Type: domain.NodeTypeShadowsocks}}})
		require.Error(t, err)
		require.True(t, domain.IsCode(err, domain.CodeScriptTimeout))
	})

	t.Run("explicit override", func(t *testing.T) {
		r := processor.NewRegistry()
		registerScript(r, script.WithDefaultTimeout(func() time.Duration { return time.Millisecond }))
		proc, err := r.BuildNode(domain.ProcessorSpec{
			Type: "script", Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input) {
  var end = Date.now() + 20;
  while (Date.now() < end) {}
  return input;
}`),
				"timeout_ms": 200,
			}),
		})
		require.NoError(t, err)

		_, err = proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "a", Type: domain.NodeTypeShadowsocks}}})
		require.NoError(t, err)
	})
}

func TestScriptRegistryAmbiguousWithoutStage(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	_, err := r.SelectSpecs([]domain.ProcessorSpec{{Type: "script"}}, domain.StageNodes)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}

func TestScriptSourceStrictDecodeStillRejectsUnknownFields(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)

	_, err := r.BuildNode(domain.ProcessorSpec{
		Type:  "script",
		Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": map[string]any{
				"type": "file",
				"name": "rename.js",
				"args": map[string]string{"prefix": "removed"},
			},
		}),
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid), "got %v", err)
	require.ErrorContains(t, err, "unknown field")
}

func TestScriptLegacyInlineContent(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"content": "function main(input, api) { input.nodes.forEach(function(n) { n.name = n.name + '!' }); return input; }",
		}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "a", Type: domain.NodeTypeShadowsocks},
	}})
	require.NoError(t, err)
	require.Equal(t, "a!", out.Nodes[0].Name)
}

func TestScriptStructuredInlineSource(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": map[string]any{
				"type":    "inline",
				"content": "function main(input) { input.nodes.forEach(function(n) { n.name = 'src-' + n.name }); return input; }",
			},
		}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "a", Type: domain.NodeTypeShadowsocks},
	}})
	require.NoError(t, err)
	require.Equal(t, "src-a", out.Nodes[0].Name)
}

func TestScriptNoopReturnKeepsEnvelopeAndAppendsAPIWarnings(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"id":     "noop-script",
			"source": inlineScriptSource("function main(input, api) { api.warn({message: 'heads up', node: input.nodes[0].name}); }"),
		}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "a", Type: domain.NodeTypeShadowsocks},
	}})
	require.NoError(t, err)
	require.Len(t, out.Nodes, 1)
	require.Equal(t, "a", out.Nodes[0].Name)
	require.Len(t, out.Warnings, 1)
	require.Equal(t, "script_warning", out.Warnings[0].Code)
	require.Equal(t, "heads up", out.Warnings[0].Message)
	require.Empty(t, out.Nodes[0].Warnings)
}

func TestScriptLegacyPathSource(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"path": filepath.Join("testdata", "nodes_filter_rename.js"),
		}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{
		Target: "mihomo",
		Nodes:  []domain.NodeIR{{Name: "a", Type: domain.NodeTypeShadowsocks}},
	})
	require.NoError(t, err)
	require.Equal(t, "[mihomo]a", out.Nodes[0].Name)
}

func TestScriptParseConfigRequiresContent(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	_, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{}),
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}

func TestScriptParseConfigRejectsPathAndContent(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	_, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"path":    filepath.Join("testdata", "nodes_filter_rename.js"),
			"content": "function main(input, api) { return input; }",
		}),
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
	require.ErrorContains(t, err, "path or content")
}

func TestScriptFileProcessor(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageFile,
		Params: params(t, map[string]any{
			"source": fileScriptSource(filepath.Join("testdata", "file_touch.js")),
		}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "yaml", Content: []byte("mode: rule\n")},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "# script-touched")
}

func TestScriptWithArgs(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": fileScriptSource(filepath.Join("testdata", "nodes_with_args.js")),
			"args":   map[string]any{"prefix": "ARG-"},
		}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{
		Nodes: []domain.NodeIR{{Name: "a", Type: domain.NodeTypeShadowsocks}},
	})
	require.NoError(t, err)
	require.Equal(t, "ARG-a", out.Nodes[0].Name)
}

func TestScriptCompileError(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	_, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{"source": inlineScriptSource("function main( broken")}),
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeScriptRuntime))
}

func TestScriptInlineContentConfig(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	_, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{"engine": "python"}),
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}

func TestScriptAPIExercise(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": fileScriptSource(filepath.Join("testdata", "api_exercise.js")),
		}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{
		Nodes: []domain.NodeIR{{Name: "a", Type: domain.NodeTypeShadowsocks, Server: "x", Port: 1}},
	})
	require.NoError(t, err)
	require.Equal(t, "foo:2c26b46b", out.Nodes[0].Name)
	var foundExt bool
	for _, w := range out.Warnings {
		if w.Code == "script_ext_field" {
			foundExt = true
		}
	}
	require.True(t, foundExt)
}

func TestScriptAPIINIParseAndStringifyOrderedDocument(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageFile,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(`
function main(input, api) {
  const doc = api.ini.parse(input.file.content);
  if (!doc.bom || doc.newline !== "\r\n" || !doc.trailing_newline) {
    throw new Error("document metadata was not preserved");
  }
  if (doc.sections.length !== 2 || doc.sections[1].name !== "Rule") {
    throw new Error("ordered sections were not preserved");
  }
  doc.newline = "\n";
  doc.trailing_newline = false;
  doc.sections[0].lines[0] = "dns-server = 8.8.8.8";
  doc.sections.push({name: "Rule", lines: ["DOMAIN,example.com,Proxy"]});
  input.file.content = api.ini.stringify(doc);
  return input;
}`),
		}),
	})
	require.NoError(t, err)

	body := append([]byte{0xef, 0xbb, 0xbf}, []byte(
		"# generated\r\n[General]\r\ndns-server = 1.1.1.1\r\n[Rule]\r\nFINAL,Proxy\r\n",
	)...)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "shadowrocket", Content: body},
	})

	require.NoError(t, err)
	require.Equal(t, append([]byte{0xef, 0xbb, 0xbf}, []byte(
		"# generated\n[General]\ndns-server = 8.8.8.8\n[Rule]\nFINAL,Proxy\n[Rule]\nDOMAIN,example.com,Proxy",
	)...), out.File.Content)
}

func TestScriptAPIINIAvailableInNodesStageAndHandlesZeroArguments(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(`
function main(input, api) {
  if (api.ini.parse() !== undefined) throw new Error("parse() must return undefined");
  if (api.ini.stringify() !== "") throw new Error("stringify() must return an empty string");
  const text = api.ini.stringify({
    bom: false,
    newline: "\n",
    trailing_newline: true,
    preamble: [],
    sections: [{name: "General", lines: ["profile = test"]}]
  });
  input.nodes[0].name = api.ini.parse(text).sections[0].lines[0];
  return input;
}`),
		}),
	})
	require.NoError(t, err)

	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{
		Nodes: []domain.NodeIR{{Name: "node", Type: domain.NodeTypeShadowsocks}},
	})

	require.NoError(t, err)
	require.Equal(t, "profile = test", out.Nodes[0].Name)
}

func TestScriptAPIINIOverrideUsesFileOverrideSemantics(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageFile,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(`
function main(input, api) {
  input.file.content = api.ini.override(input.file.content,
    "[General]\nfoo = new\n[Rule+]\nDOMAIN,example.com,Proxy\n");
  return input;
}`),
		}),
	})
	require.NoError(t, err)

	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{
			Kind:    "shadowrocket",
			Content: []byte("[General]\r\nfoo = old\r\n[Rule]\r\nFINAL,DIRECT\r\n"),
		},
	})

	require.NoError(t, err)
	require.Equal(t,
		"[General]\r\nfoo = new\r\n[Rule]\r\nFINAL,DIRECT\r\nDOMAIN,example.com,Proxy\r\n",
		string(out.File.Content),
	)
}

func TestScriptAPIINIRejectsInvalidModelAndMissingOverrideArgument(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{
			name: "invalid model",
			script: `function main(input, api) {
  api.ini.stringify({bom: false, newline: "\n", trailing_newline: false,
    preamble: [], sections: [{name: "General", lines: ["[Rule]"]}]});
  return input;
}`,
		},
		{
			name:   "missing override patch",
			script: `function main(input, api) { api.ini.override("[General]\n"); return input; }`,
		},
		{
			name: "unknown document field",
			script: `function main(input, api) {
  api.ini.stringify({bom: false, newline: "\n", trailing_newline: false,
    preamble: [], sections: [], extra: true});
  return input;
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := processor.NewRegistry()
			registerScript(r)
			proc, err := r.BuildFile(domain.ProcessorSpec{
				Type: "script", Stage: domain.StageFile,
				Params: params(t, map[string]any{
					"source": inlineScriptSource(tt.script),
				}),
			})
			require.NoError(t, err)

			_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
				File: domain.FileDocument{Kind: "shadowrocket", Content: []byte("[General]\n")},
			})

			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeScriptRuntime))
		})
	}
}

func TestScriptAPIProbeFiltersNodesAndAppendsWarnings(t *testing.T) {
	runner := &scriptProbeRunner{result: &domain.ProbeResult{
		Results: []domain.NodeProbeResult{
			{NodeName: "fast", Method: "tcp_connect", Alive: true, DurationMS: 12},
			{NodeName: "dead", Method: "tcp_connect", Alive: false, ErrorCode: "probe_tcp_failed"},
		},
		Report: domain.Report{Warnings: []domain.Warning{{Code: "probe_tcp_failed", Message: "one failed"}}},
	}}
	r := processor.NewRegistry()
	registerScript(r, script.WithProbeRunner(runner))
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(`
function main(input, api) {
  const result = api.probe(input.nodes, {
    method: "tcp_connect",
    timeout_ms: 123,
    concurrency: 2,
    cache_ttl_seconds: 60,
    meta: { source: "script" }
  });
  input.nodes = input.nodes.filter(function(_, index) {
    return result.results[index] && result.results[index].alive;
  });
  input.nodes[0].meta = input.nodes[0].meta || {};
  input.nodes[0].meta["probe.duration_ms"] = String(result.results[0].duration_ms);
  return input;
}`),
		}),
	})
	require.NoError(t, err)

	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "fast", Type: domain.NodeTypeShadowsocks, Server: "fast.example", Port: 443, Cipher: "aes-128-gcm", Password: "p"},
		{Name: "dead", Type: domain.NodeTypeShadowsocks, Server: "dead.example", Port: 443, Cipher: "aes-128-gcm", Password: "p"},
	}})

	require.NoError(t, err)
	require.Len(t, runner.requests, 1)
	require.Equal(t, domain.ProbeMethod("tcp_connect"), runner.requests[0].Method)
	require.Equal(t, 123, runner.requests[0].TimeoutMS)
	require.Equal(t, 2, runner.requests[0].Concurrency)
	require.Equal(t, 60, runner.requests[0].CacheTTLSeconds)
	require.Equal(t, "script", runner.requests[0].Meta["source"])
	require.Equal(t, "inline_nodes", runner.requests[0].Input.Type)
	require.Len(t, runner.requests[0].Input.Nodes, 2)
	require.Len(t, out.Nodes, 1)
	require.Equal(t, "fast", out.Nodes[0].Name)
	require.Equal(t, "12", out.Nodes[0].Meta["probe.duration_ms"])
	require.Contains(t, warningCodes(out.Warnings), "probe_tcp_failed")
}

func TestScriptAPIProbeRejectsNodesOutsideScriptInput(t *testing.T) {
	runner := &scriptProbeRunner{result: &domain.ProbeResult{}}
	r := processor.NewRegistry()
	registerScript(r, script.WithProbeRunner(runner))
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(`
function main(input, api) {
  api.probe([{
    name: "extra",
    type: "ss",
    server: "extra.example",
    port: 443,
    cipher: "aes-128-gcm",
    password: "p"
  }], { method: "tcp_connect" });
  return input;
}`),
		}),
	})
	require.NoError(t, err)

	_, err = proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "fast", Type: domain.NodeTypeShadowsocks, Server: "fast.example", Port: 443, Cipher: "aes-128-gcm", Password: "p"},
	}})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeScriptRuntime))
	require.ErrorContains(t, err, "subset")
	require.Empty(t, runner.requests)
}

func TestScriptAPIProbeRespectsScriptTimeout(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r, script.WithProbeRunner(&blockingScriptProbeRunner{}))
	proc, err := r.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"timeout_ms": 100,
			"source": inlineScriptSource(`
function main(input, api) {
  api.probe(input.nodes, { method: "tcp_connect" });
  return input;
}`),
		}),
	})
	require.NoError(t, err)

	_, err = proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "fast", Type: domain.NodeTypeShadowsocks, Server: "fast.example", Port: 443, Cipher: "aes-128-gcm", Password: "p"},
	}})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeScriptTimeout))
}

func warningCodes(warnings []domain.Warning) []string {
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, warning.Code)
	}
	return out
}

func inlineScriptSource(content string) map[string]any {
	return map[string]any{"type": "inline", "content": content}
}

func fileScriptSource(name string) map[string]any {
	return map[string]any{"type": "file", "name": name}
}
