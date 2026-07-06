package file_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	fileproc "github.com/kuuvahki-labs/sandrone/internal/processor/file"
)

func makeFileRegistry() *processor.Registry {
	r := processor.NewRegistry()
	fileproc.Register(r)
	return r
}

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

func TestInjectNodesMihomoReplaceWithJSONPointer(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "inject_nodes", Params: params(t, map[string]any{
		"from": "nodes",
		"path": "/proxies",
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{
			Name: "mihomo.yaml",
			Kind: "mihomo",
			Content: []byte(`mixed-port: 7890
proxies: []
rules:
  - MATCH,DIRECT
`),
		},
		Parts: []domain.FilePart{
			{
				Name: "nodes", Role: "nodes", Kind: "yaml",
				Content: []byte("proxies:\n  - name: node-a\n    type: ss\n    server: example.com\n    port: 8388\n"),
			},
		},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "name: node-a")
	require.Contains(t, string(out.File.Content), "rules:")
}

func TestInjectNodesMihomoShorthandPath(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "inject_nodes", Params: params(t, map[string]any{"path": "proxies"})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Name: "x", Kind: "mihomo", Content: []byte("proxies: []\n")},
		Parts: []domain.FilePart{
			{Name: "nodes", Role: "nodes", Kind: "yaml", Content: []byte("- name: a\n  type: ss\n")},
		},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "name: a")
}

func TestInjectNodesSingBoxOutbounds(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "inject_nodes", Params: params(t, map[string]any{"path": "/outbounds"})})
	require.NoError(t, err)
	base := `{
  "log": {"level": "info"},
  "outbounds": [],
  "route": {"rules": []}
}`
	rendered := `{"outbounds":[{"type":"shadowsocks","tag":"a","server":"example.com","server_port":8388}]}`
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Name: "sb.json", Kind: "sing-box", Content: []byte(base)},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "json", Content: []byte(rendered)}},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "\"tag\": \"a\"")
}

func TestInjectNodesAppendMode(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "inject_nodes", Params: params(t, map[string]any{"mode": "append"})})
	require.NoError(t, err)
	base := `proxies:
  - name: existing
    type: ss
`
	rendered := "proxies:\n  - name: added\n    type: ss\n"
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "mihomo", Content: []byte(base)},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "yaml", Content: []byte(rendered)}},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "existing")
	require.Contains(t, string(out.File.Content), "added")
}

func TestInjectNodesErrorsWhenPathMissing(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "inject_nodes", Params: params(t, map[string]any{"path": "/proxies"})})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "mihomo", Content: []byte("mixed-port: 7890\n")},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "yaml", Content: []byte("proxies: []\n")}},
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileProcessorFailed))
}

func TestInjectNodesJSONAppendMode(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "inject_nodes", Stage: domain.StageFile,
		Params: params(t, map[string]any{"path": "/outbounds", "mode": "append"}),
	})
	require.NoError(t, err)
	base := `{"outbounds":[{"type":"direct","tag":"direct"}]}`
	rendered := `{"outbounds":[{"type":"shadowsocks","tag":"n1","server":"x","server_port":1}]}`
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "sing-box", Content: []byte(base)},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "json", Content: []byte(rendered)}},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), `"tag": "n1"`)
	require.Contains(t, string(out.File.Content), `"tag": "direct"`)
}

func TestInjectNodesDefaultProxiesPath(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "inject_nodes", Stage: domain.StageFile})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "mihomo", Content: []byte("mixed-port: 7890\nproxies: []\n")},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "yaml", Content: []byte("- name: a\n  type: ss\n  server: x\n  port: 1\n")}},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "name: a")
}

func TestInjectNodesSelectPartByName(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "inject_nodes", Stage: domain.StageFile,
		Params: params(t, map[string]any{"from": "rendered", "path": "/proxies"}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "mihomo", Content: []byte("proxies: []\n")},
		Parts: []domain.FilePart{
			{Name: "other", Role: "nodes", Kind: "yaml", Content: []byte("- name: skip\n  type: ss\n")},
			{Name: "rendered", Role: "resource", Kind: "yaml", Content: []byte("- name: picked\n  type: ss\n  server: x\n  port: 1\n")},
		},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "picked")
	require.NotContains(t, string(out.File.Content), "skip")
}

func TestInjectNodesNestedJSONReplace(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "inject_nodes", Stage: domain.StageFile,
		Params: params(t, map[string]any{"path": "/route/rules"}),
	})
	require.NoError(t, err)
	base := `{"route":{"rules":[{"type":"default"}]}}`
	rendered := `{"rules":[{"type":"shadowsocks","tag":"n1"}]}`
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "sing-box", Content: []byte(base)},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "json", Content: []byte(rendered)}},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), `"tag": "n1"`)
}

func TestInjectNodesInvalidJSONBase(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "inject_nodes", Stage: domain.StageFile,
		Params: params(t, map[string]any{"path": "/outbounds"}),
	})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "sing-box", Content: []byte("{broken}")},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "json", Content: []byte(`{"outbounds":[]}`)}},
	})
	require.Error(t, err)
}

func TestInjectNodesYAMAppendMode(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "inject_nodes", Stage: domain.StageFile,
		Params: params(t, map[string]any{"mode": "append"}),
	})
	require.NoError(t, err)
	base := "proxies:\n  - name: existing\n    type: ss\n"
	rendered := "proxies:\n  - name: added\n    type: ss\n    server: x\n    port: 1\n"
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "mihomo", Content: []byte(base)},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "yaml", Content: []byte(rendered)}},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "existing")
	require.Contains(t, string(out.File.Content), "added")
}

func TestInjectNodesJSONPartAsArray(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "inject_nodes", Stage: domain.StageFile,
		Params: params(t, map[string]any{"path": "/outbounds"}),
	})
	require.NoError(t, err)
	base := `{"outbounds":[]}`
	rendered := `[{"type":"direct","tag":"direct"}]`
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "sing-box", Content: []byte(base)},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "json", Content: []byte(rendered)}},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), `"tag": "direct"`)
}

func TestInjectNodesTextKindUsesJSONPart(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "inject_nodes", Stage: domain.StageFile,
		Params: params(t, map[string]any{"path": "/items"}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "text", Content: []byte(`{"items":[]}`)},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "json", Content: []byte(`[{"id":1}]`)}},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), `"id": 1`)
}

func TestInjectNodesDefaultOutboundsPath(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "inject_nodes", Stage: domain.StageFile})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "sing-box", Content: []byte(`{"outbounds":[]}`)},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "json", Content: []byte(`[{"type":"direct","tag":"d"}]`)}},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), `"tag": "d"`)
}

func TestInjectNodesInvalidYAMLBase(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "inject_nodes", Stage: domain.StageFile})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "mihomo", Content: []byte("proxies: [\n")},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "yaml", Content: []byte("- name: a\n  type: ss\n")}},
	})
	require.Error(t, err)
}

func TestInjectNodesPartAsMappingYAML(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "inject_nodes", Stage: domain.StageFile})
	require.NoError(t, err)
	rendered := "proxies:\n  - name: mapped\n    type: ss\n    server: x\n    port: 1\n"
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "mihomo", Content: []byte("proxies: []\n")},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "yaml", Content: []byte(rendered)}},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "mapped")
}

func TestInjectNodesJSONPathNotFound(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "inject_nodes", Stage: domain.StageFile,
		Params: params(t, map[string]any{"path": "/missing"}),
	})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "sing-box", Content: []byte(`{"outbounds":[]}`)},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "json", Content: []byte(`[{"tag":"a"}]`)}},
	})
	require.Error(t, err)
}

func TestInjectNodesUnsupportedFileKind(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "inject_nodes", Stage: domain.StageFile,
		Params: params(t, map[string]any{"path": "/x"}),
	})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "binary", Content: []byte("data")},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "yaml", Content: []byte("- a\n")}},
	})
	require.Error(t, err)
}

func TestInjectNodesErrors(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "inject_nodes", Stage: domain.StageFile})
	require.NoError(t, err)

	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "mihomo", Content: []byte("proxies: []\n")},
	})
	require.Error(t, err)

	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "mihomo", Content: []byte("proxies: []\n")},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "yaml", Content: nil}},
	})
	require.Error(t, err)

	_, err = r.BuildFile(domain.ProcessorSpec{
		Type: "inject_nodes", Stage: domain.StageFile,
		Params: params(t, map[string]any{"mode": "invalid"}),
	})
	require.Error(t, err)
}

func TestInjectNodesNewInjectNodesShim(t *testing.T) {
	proc := fileproc.NewInjectNodes("mihomo", "proxies")
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:  domain.FileDocument{Kind: "mihomo", Content: []byte("proxies: []\n")},
		Parts: []domain.FilePart{{Name: "nodes", Role: "nodes", Kind: "yaml", Content: []byte("- name: a\n  type: ss\n")}},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "name: a")
}
