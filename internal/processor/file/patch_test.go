package file_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func ops(t *testing.T, list []map[string]any) map[string]json.RawMessage {
	t.Helper()
	out, err := json.Marshal(list)
	require.NoError(t, err)
	return map[string]json.RawMessage{"ops": out}
}

func TestYAMLPatchCopyAndMove(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "yaml_patch", Params: ops(t, []map[string]any{
		{"op": "copy", "from": "/src", "path": "/dst"},
		{"op": "move", "from": "/src", "path": "/moved"},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "yaml", Content: []byte("src: value\n")},
	})
	require.NoError(t, err)
	body := string(out.File.Content)
	require.Contains(t, body, "dst: value")
	require.Contains(t, body, "moved: value")
	require.NotContains(t, body, "src:")
}

func TestYAMLPatchAddReplaceRemove(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "yaml_patch", Params: ops(t, []map[string]any{
		{"op": "add", "path": "/mode", "value": "rule"},
		{"op": "replace", "path": "/mixed-port", "value": 8080},
		{"op": "remove", "path": "/old"},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "yaml", Content: []byte("mixed-port: 7890\nold: drop\n")},
	})
	require.NoError(t, err)
	body := string(out.File.Content)
	require.Contains(t, body, "mode: rule")
	require.Contains(t, body, "mixed-port: 8080")
	require.NotContains(t, body, "old:")
}

func TestJSONPatchAppendIntoArray(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "add", "path": "/list/-", "value": "added"},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"list":["a","b"]}`)},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "\"added\"")
}

func TestJSONPatchMoveCopyTest(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "copy", "from": "/src", "path": "/copy"},
		{"op": "move", "from": "/src", "path": "/moved"},
		{"op": "test", "path": "/moved", "value": "x"},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"src":"x"}`)},
	})
	require.NoError(t, err)
	body := string(out.File.Content)
	require.Contains(t, body, "\"copy\": \"x\"")
	require.Contains(t, body, "\"moved\": \"x\"")
	require.NotContains(t, body, "\"src\"")
}

func TestJSONPatchInvalidArrayIndex(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "add", "path": "/items/not-an-index", "value": "x"},
	})})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"items":["a"]}`)},
	})
	require.Error(t, err)
}

func TestJSONPatchDecodeEmptyValue(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "add", "path": "/flag", "value": nil},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{}`)},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), `"flag": null`)
}

func TestYAMLPatchUnsupportedOp(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "yaml_patch", Params: ops(t, []map[string]any{
		{"op": "noop", "path": "/a"},
	})})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "yaml", Content: []byte("a: 1\n")},
	})
	require.Error(t, err)
}

func TestJSONPatchAddNestedPath(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "add", "path": "/route/rules/-", "value": map[string]any{"type": "direct"}},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"route":{"rules":[]}}`)},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), `"direct"`)
}

func TestYAMLPatchRemoveNestedKey(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "yaml_patch", Params: ops(t, []map[string]any{
		{"op": "remove", "path": "/parent/child"},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "yaml", Content: []byte("parent:\n  child: gone\n  keep: yes\n")},
	})
	require.NoError(t, err)
	require.NotContains(t, string(out.File.Content), "gone")
	require.Contains(t, string(out.File.Content), "keep")
}

func TestJSONPatchRemoveNestedKey(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "remove", "path": "/parent/child"},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"parent":{"child":"gone","keep":"yes"}}`)},
	})
	require.NoError(t, err)
	require.NotContains(t, string(out.File.Content), "gone")
	require.Contains(t, string(out.File.Content), "yes")
}

func TestJSONPatchReplaceDeepPath(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "replace", "path": "/a/b/c", "value": 42},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"a":{"b":{"c":1}}}`)},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), `"c": 42`)
}

func TestYAMLPatchRemoveMissingPath(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "yaml_patch", Params: ops(t, []map[string]any{
		{"op": "remove", "path": "/does-not-exist"},
	})})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "yaml", Content: []byte("a: 1\n")},
	})
	require.Error(t, err)
}

func TestJSONPatchTestMismatch(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "test", "path": "/a", "value": "expected"},
	})})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"a":"actual"}`)},
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileProcessorFailed))
}

func TestPatchBuildRejectsEmptyOps(t *testing.T) {
	r := makeFileRegistry()
	_, err := r.BuildFile(domain.ProcessorSpec{Type: "yaml_patch"})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))

	_, err = r.BuildFile(domain.ProcessorSpec{Type: "json_patch"})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}

func TestJSONPatchEscapedPointerTokens(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "replace", "path": "/a~1b/tilde~0key", "value": 2},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"a/b":{"tilde~key":1}}`)},
	})
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), `"tilde~key": 2`)
}

func TestJSONPatchReplaceRoot(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "replace", "path": "", "value": map[string]any{"root": true}},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"old":true}`)},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"root": true}`, string(out.File.Content))
}

func TestJSONPatchRemoveArrayElement(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "remove", "path": "/items/1"},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"items":["a","b","c"]}`)},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"items":["a","c"]}`, string(out.File.Content))
}

func TestJSONPatchCopyDeepCopiesNestedValues(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "copy", "from": "/template", "path": "/copy"},
		{"op": "replace", "path": "/template/nested/0/name", "value": "changed"},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"template":{"nested":[{"name":"original"}]}}`)},
	})
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.File.Content, &doc))
	copied := doc["copy"].(map[string]any)["nested"].([]any)[0].(map[string]any)
	updated := doc["template"].(map[string]any)["nested"].([]any)[0].(map[string]any)
	require.Equal(t, "original", copied["name"])
	require.Equal(t, "changed", updated["name"])
}

func TestJSONPatchRejectsAppendInMiddle(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "add", "path": "/items/-/name", "value": "x"},
	})})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"items":[]}`)},
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileProcessorFailed))
}

func TestJSONPatchRejectsScalarTraversal(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: ops(t, []map[string]any{
		{"op": "replace", "path": "/a/b", "value": "x"},
	})})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "json", Content: []byte(`{"a":1}`)},
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileProcessorFailed))
}

func TestJSONPatchRejectsInvalidValue(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "json_patch", Params: map[string]json.RawMessage{
		"ops": []byte(`[{"op":"add","path":"/a","value":`),
	}})
	require.Error(t, err)
	require.Nil(t, proc)
}
