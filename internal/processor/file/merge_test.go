package file_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	fileproc "github.com/kuuvahki-labs/sandrone/internal/processor/file"
)

func TestMergePartsAppend(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "a", Role: "base", Content: []byte("line-1")},
		{Name: "b", Role: "base", Content: []byte("line-2")},
	}
	out, err := fileproc.MergeParts(parts, "text", fileproc.MergeParams{Mode: "append"})
	require.NoError(t, err)
	require.Equal(t, "line-1\nline-2", string(out))
}

func TestMergePartsReplace(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "a", Role: "base", Content: []byte("first")},
		{Name: "b", Role: "base", Content: []byte("second")},
	}
	out, err := fileproc.MergeParts(parts, "text", fileproc.MergeParams{Mode: "replace"})
	require.NoError(t, err)
	require.Equal(t, "second", string(out))
}

func TestMergePartsYAMLOverlay(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "a", Role: "base", Content: []byte("a: 1\nshared: keep\nnested:\n  b: from-a\n")},
		{Name: "b", Role: "base", Content: []byte("a: 2\nnested:\n  c: from-b\n")},
	}
	out, err := fileproc.MergeParts(parts, "yaml", fileproc.MergeParams{Mode: "yaml_overlay"})
	require.NoError(t, err)
	body := string(out)
	require.Contains(t, body, "a: 2")
	require.Contains(t, body, "shared: keep")
	require.Contains(t, body, "b: from-a")
	require.Contains(t, body, "c: from-b")
}

func TestMergePartsYAMLOverrideAppliesOrderedArrayOperations(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "current", Role: "base", Content: []byte("dns:\n  fake-ip-filter:\n    - base\n")},
		{Name: "preset", Role: "base", Content: []byte("dns:\n  +fake-ip-filter:\n    - before\n  fake-ip-filter!:\n    - reset\n  fake-ip-filter+:\n    - after\n")},
	}

	out, err := fileproc.MergeParts(parts, "mihomo", fileproc.MergeParams{Mode: "yaml_override"})

	require.NoError(t, err)
	require.Equal(t, []any{"reset", "after"}, yamlPath(t, out, "dns", "fake-ip-filter"))
}

func TestMergePartsYAMLOverrideSupportsNestedPrependAppendAndMissingArrays(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "current", Role: "base", Content: []byte("dns:\n  fake-ip-filter:\n    - base\n")},
		{Name: "preset", Role: "base", Content: []byte("dns:\n  +fake-ip-filter:\n    - before\n  fake-ip-filter+:\n    - base\n  nameserver+:\n    - https://dns.example/dns-query\n")},
	}

	out, err := fileproc.MergeParts(parts, "mihomo", fileproc.MergeParams{Mode: "yaml_override"})

	require.NoError(t, err)
	require.Equal(t, []any{"before", "base", "base"}, yamlPath(t, out, "dns", "fake-ip-filter"))
	require.Equal(t, []any{"https://dns.example/dns-query"}, yamlPath(t, out, "dns", "nameserver"))
}

func TestMergePartsYAMLOverrideCreatesMissingParentObjectsForArrayOperations(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "current", Role: "base", Content: []byte("mode: rule\n")},
		{Name: "preset", Role: "base", Content: []byte("dns:\n  fake-ip-filter+:\n    - +.tailscale.com\n")},
	}

	out, err := fileproc.MergeParts(parts, "mihomo", fileproc.MergeParams{Mode: "yaml_override"})

	require.NoError(t, err)
	require.Equal(t, []any{"+.tailscale.com"}, yamlPath(t, out, "dns", "fake-ip-filter"))
	require.Nil(t, yamlPath(t, out, "dns", "fake-ip-filter+"))
}

func TestMergePartsYAMLOverrideSupportsLiteralKeysReplacementAndDeletion(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "current", Role: "base", Content: []byte("dns:\n  nameserver-policy:\n    old: system\ntun:\n  device: mihomo\n  auto-route: true\n")},
		{Name: "preset", Role: "base", Content: []byte("dns:\n  nameserver-policy:\n    <+.ts.net>: 100.100.100.100\ntun!:\n  enable: true\n  stack: mixed\n")},
		{Name: "cleanup", Role: "base", Content: []byte("dns:\n  nameserver-policy:\n    old: null\n")},
	}

	out, err := fileproc.MergeParts(parts, "mihomo", fileproc.MergeParams{Mode: "yaml_override"})

	require.NoError(t, err)
	require.Equal(t, "100.100.100.100", yamlPath(t, out, "dns", "nameserver-policy", "+.ts.net"))
	require.Nil(t, yamlPath(t, out, "dns", "nameserver-policy", "old"))
	require.Equal(t, map[string]any{"enable": true, "stack": "mixed"}, yamlPath(t, out, "tun"))
}

func TestMergePartsYAMLOverrideReportsPartAndPathOnTypeError(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "current", Role: "base", Content: []byte("dns:\n  fake-ip-filter: enabled\n")},
		{Name: "sniffer-preset", Role: "base", Content: []byte("dns:\n  fake-ip-filter+:\n    - extra\n")},
	}

	_, err := fileproc.MergeParts(parts, "mihomo", fileproc.MergeParams{Mode: "yaml_override"})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileMergeFailed))
	appErr, ok := errors.AsType[*domain.AppError](err)
	require.True(t, ok)
	require.Equal(t, "sniffer-preset", appErr.Part)
	require.Equal(t, "/dns/fake-ip-filter", appErr.Path)
}

func TestMergeProcessorYAMLOverrideFailsAtomically(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "merge", Params: params(t, map[string]any{
		"mode":    "yaml_override",
		"content": "dns:\n  fake-ip-filter+: invalid\n",
	})})
	require.NoError(t, err)
	original := domain.FileDocument{Kind: "mihomo", Content: []byte("dns:\n  fake-ip-filter:\n    - base\n")}

	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{File: original})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileMergeFailed))
	require.Equal(t, original, out.File)
	appErr, ok := errors.AsType[*domain.AppError](err)
	require.True(t, ok)
	require.Equal(t, "overlay", appErr.Part)
	require.Equal(t, "/dns/fake-ip-filter", appErr.Path)
}

func yamlPath(t *testing.T, body []byte, path ...string) any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(body, &doc))
	var current any = doc
	for _, key := range path {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = mapping[key]
	}
	return current
}

func TestMergePartsJSONOverlayDeletesKey(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "a", Role: "base", Content: []byte(`{"keep":1,"drop":2,"nested":{"x":1}}`)},
		{Name: "b", Role: "base", Content: []byte(`{"drop":null,"nested":{"y":2}}`)},
	}
	out, err := fileproc.MergeParts(parts, "json", fileproc.MergeParams{Mode: "json_overlay"})
	require.NoError(t, err)
	body := string(out)
	require.Contains(t, body, `"keep": 1`)
	require.NotContains(t, body, `"drop"`)
	require.Contains(t, body, `"y": 2`)
}

func TestMergePartsJSONOverlay(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "a", Role: "base", Content: []byte(`{"a":1,"nested":{"x":1}}`)},
		{Name: "b", Role: "base", Content: []byte(`{"a":2,"nested":{"y":2}}`)},
	}
	out, err := fileproc.MergeParts(parts, "json", fileproc.MergeParams{Mode: "json_overlay"})
	require.NoError(t, err)
	require.Contains(t, string(out), "\"a\": 2")
	require.Contains(t, string(out), "\"x\": 1")
	require.Contains(t, string(out), "\"y\": 2")
}

func TestMergePartsJSONOverrideSupportsOrderedOperations(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "current", Role: "base", Content: []byte(`{
  "dns": {
    "fake-ip-filter": ["base"],
    "fallback": ["base"],
    "nameserver-policy": {"old": "system"}
  },
  "tun": {"device": "sing-box", "auto-route": true},
  "remove": "me"
}`)},
		{Name: "preset", Role: "base", Content: []byte(`{
  "dns": {
    "+fake-ip-filter": ["before"],
    "fake-ip-filter!": ["reset"],
    "fake-ip-filter+": ["after"],
    "+fallback": ["before"],
    "nameserver+": ["https://dns.example/dns-query"],
    "nameserver-policy": {"<+.ts.net>": "100.100.100.100", "old": null}
  },
  "tun!": {"enable": true, "stack": "mixed"},
  "remove": null
}`)},
	}

	out, err := fileproc.MergeParts(parts, "sing-box", fileproc.MergeParams{Mode: "json_override"})

	require.NoError(t, err)
	require.True(t, json.Valid(out), "json_override output must be valid JSON: %s", out)
	require.Equal(t, []any{"reset", "after"}, jsonPath(t, out, "dns", "fake-ip-filter"))
	require.Equal(t, []any{"before", "base"}, jsonPath(t, out, "dns", "fallback"))
	require.Equal(t, []any{"https://dns.example/dns-query"}, jsonPath(t, out, "dns", "nameserver"))
	require.Equal(t, "100.100.100.100", jsonPath(t, out, "dns", "nameserver-policy", "+.ts.net"))
	require.Equal(t, map[string]any{"enable": true, "stack": "mixed"}, jsonPath(t, out, "tun"))

	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	require.NotContains(t, doc, "remove")
	policy, ok := jsonPath(t, out, "dns", "nameserver-policy").(map[string]any)
	require.True(t, ok)
	require.NotContains(t, policy, "old")
}

func TestMergePartsJSONOverrideRejectsNonJSONInput(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "current", Role: "base", Content: []byte(`{"dns":{"enable":true}}`)},
		{Name: "preset", Role: "base", Content: []byte("dns:\n  enable: false\n")},
	}

	_, err := fileproc.MergeParts(parts, "sing-box", fileproc.MergeParams{Mode: "json_override"})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileMergeFailed))
	appErr, ok := errors.AsType[*domain.AppError](err)
	require.True(t, ok)
	require.Equal(t, "preset", appErr.Part)
	require.Empty(t, appErr.Path)
}

func TestMergePartsINIOverrideAppliesSelectedPartsSequentially(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "base", Role: "base", Content: []byte("[General]\nmode=rule\n[Rule]\nBASE\n")},
		{Name: "first", Role: "resource", Content: []byte("[General]\nmode=global\n[Rule+]\nONE\n")},
		{Name: "ignored", Role: "resource", Content: []byte("[General]\nmode=ignored\n")},
		{Name: "second", Role: "resource", Content: []byte("[+Rule]\nZERO\n[General]\nlog=true\n")},
	}

	out, err := fileproc.MergeParts(parts, "shadowrocket", fileproc.MergeParams{
		Mode:    "ini_override",
		Include: []string{"base", "first", "second"},
	})

	require.NoError(t, err)
	require.Equal(t, "[General]\nmode=global\nlog=true\n[Rule]\nZERO\nBASE\nONE\n", string(out))
}

func TestMergePartsINIOverrideReportsOverlayPartAndSectionPath(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "base", Role: "base", Content: []byte("[Rule]\nBASE\n")},
		{Name: "broken-overlay", Role: "base", Content: []byte("[Rule-]\nMATCH,DIRECT\n")},
	}

	_, err := fileproc.MergeParts(parts, "shadowrocket", fileproc.MergeParams{Mode: "ini_override"})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileMergeFailed))
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "broken-overlay", appErr.Part)
	require.Equal(t, "/Rule", appErr.Path)
}

func TestMergePartsINIOverrideReportsInvalidBasePart(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "invalid-base", Role: "base", Content: []byte("[]\n")},
	}

	_, err := fileproc.MergeParts(parts, "shadowrocket", fileproc.MergeParams{Mode: "ini_override"})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileMergeFailed))
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "invalid-base", appErr.Part)
	require.Empty(t, appErr.Path)
}

func TestMergeProcessorAppliesInlineINIOverrideToCurrentDocument(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "merge", Params: params(t, map[string]any{
		"mode":    "ini_override",
		"content": "[General]\nmode=global\n[Rule+]\nFINAL\n",
	})})
	require.NoError(t, err)

	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "shadowrocket", Content: []byte("\xEF\xBB\xBF[General]\r\nmode=rule\r\n[Rule]\r\nBASE\r\n")},
	})

	require.NoError(t, err)
	require.Equal(t, "\xEF\xBB\xBF[General]\r\nmode=global\r\n[Rule]\r\nBASE\r\nFINAL\r\n", string(out.File.Content))
}

func TestMergeProcessorINIOverrideFailsAtomicallyWithPartAndPath(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "merge", Params: params(t, map[string]any{
		"mode":    "ini_override",
		"content": "[Rule-]\nMATCH,DIRECT\n",
	})})
	require.NoError(t, err)
	original := domain.FileDocument{
		Name:    "config",
		Kind:    "shadowrocket",
		Content: []byte("[Rule]\nBASE\n"),
		Meta:    map[string]string{"keep": "exact"},
	}

	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{File: original})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileMergeFailed))
	require.Equal(t, original, out.File)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "overlay", appErr.Part)
	require.Equal(t, "/Rule", appErr.Path)
}

func TestMergeProcessorJSONOverrideFailsAtomicallyWithPartAndPath(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "merge", Params: params(t, map[string]any{
		"mode":    "json_override",
		"content": `{"dns":{"fake-ip-filter+":["extra"]}}`,
	})})
	require.NoError(t, err)
	original := domain.FileDocument{Kind: "sing-box", Content: []byte(`{"dns":{"fake-ip-filter":"enabled"}}`)}

	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{File: original})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileMergeFailed))
	require.Equal(t, original, out.File)
	appErr, ok := errors.AsType[*domain.AppError](err)
	require.True(t, ok)
	require.Equal(t, "overlay", appErr.Part)
	require.Equal(t, "/dns/fake-ip-filter", appErr.Path)
}

func jsonPath(t *testing.T, body []byte, path ...string) any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	var current any = doc
	for _, key := range path {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = mapping[key]
	}
	return current
}

func TestMergePartsNoPartsFails(t *testing.T) {
	_, err := fileproc.MergeParts(nil, "text", fileproc.MergeParams{Mode: "append"})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileMergeFailed))
}

func TestMergePartsIncludeSelectsSubset(t *testing.T) {
	parts := []domain.FilePart{
		{Name: "a", Role: "base", Content: []byte("a")},
		{Name: "b", Role: "base", Content: []byte("b")},
		{Name: "c", Role: "base", Content: []byte("c")},
	}
	out, err := fileproc.MergeParts(parts, "text", fileproc.MergeParams{Mode: "append", Include: []string{"c", "a"}})
	require.NoError(t, err)
	require.Equal(t, "c\na", string(out))
}

func TestMergePolicyMapsFields(t *testing.T) {
	params := fileproc.MergePolicy(domain.FileMergePolicy{
		Mode:      "append",
		Include:   []string{"a"},
		Separator: "---",
	})
	require.Equal(t, "append", params.Mode)
	require.Equal(t, []string{"a"}, params.Include)
	require.Equal(t, "---", params.Separator)
}

func TestMergeBuildRejectsUnknownField(t *testing.T) {
	r := makeFileRegistry()
	_, err := r.BuildFile(domain.ProcessorSpec{
		Type:   "merge",
		Params: params(t, map[string]any{"mode": "append", "extra": "x"}),
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}

func TestMergeProcessorRejectsUnknownMode(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type:   "merge",
		Params: params(t, map[string]any{"mode": "unknown-mode"}),
	})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "text"},
		Parts: []domain.FilePart{
			{Name: "a", Role: "base", Content: []byte("x")},
		},
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileMergeFailed))
}

func TestMergeProcessorAppendsInChain(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "merge", Params: params(t, map[string]any{
		"mode":    "append",
		"include": []string{"base", "extra"},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Name: "x", Kind: "text", Content: []byte("ignored")},
		Parts: []domain.FilePart{
			{Name: "base", Role: "base", Content: []byte("hello")},
			{Name: "extra", Role: "resource", Content: []byte("world")},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "hello\nworld", string(out.File.Content))
}

func TestMergeProcessorAppliesInlineYAMLOverlay(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "merge", Params: params(t, map[string]any{
		"mode":    "yaml_overlay",
		"content": "mode: global\ndns:\n  enable: true\nrules: null\n",
	})})
	require.NoError(t, err)

	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "mihomo", Content: []byte("mode: rule\ndns:\n  listen: 0.0.0.0:53\nrules:\n  - MATCH,DIRECT\n")},
	})

	require.NoError(t, err)
	body := string(out.File.Content)
	require.Contains(t, body, "mode: global")
	require.Contains(t, body, "enable: true")
	require.Contains(t, body, "listen: 0.0.0.0:53")
	require.NotContains(t, body, "rules:")
}

func TestMergeProcessorAppliesInlineJSONOverlay(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "merge", Params: params(t, map[string]any{
		"mode":    "json_overlay",
		"content": `{"log":{"level":"debug"},"inbounds":[]}`,
	})})
	require.NoError(t, err)

	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "sing-box", Content: []byte(`{"log":{"disabled":false},"inbounds":[{"type":"mixed"}],"outbounds":[]}`)},
	})

	require.NoError(t, err)
	body := string(out.File.Content)
	require.Contains(t, body, `"disabled": false`)
	require.Contains(t, body, `"level": "debug"`)
	require.Contains(t, body, `"inbounds": []`)
	require.Contains(t, body, `"outbounds": []`)
}
