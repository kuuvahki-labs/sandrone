package script_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
	loader := script.WithLoader(func(source script.ScriptSource) (string, string, error) {
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

func TestScriptRegistryAmbiguousWithoutStage(t *testing.T) {
	r := processor.NewRegistry()
	registerScript(r)
	_, err := r.SelectSpecs([]domain.ProcessorSpec{{Type: "script"}}, domain.StageNodes)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
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

func TestScriptAPIProbeFiltersNodesAndAppendsWarnings(t *testing.T) {
	runner := &scriptProbeRunner{result: &domain.ProbeResult{
		Results: []domain.NodeProbeResult{
			{NodeName: "fast", Layer: "protocol", Method: "tcp_connect", Alive: true, DurationMS: 12},
			{NodeName: "dead", Layer: "protocol", Method: "tcp_connect", Alive: false, ErrorCode: "probe_tcp_failed"},
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
    layer: "protocol",
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
	require.Equal(t, domain.ProbeLayer("protocol"), runner.requests[0].Layer)
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
