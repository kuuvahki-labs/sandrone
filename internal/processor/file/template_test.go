package file_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestTemplateReplacesVars(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "template", Params: params(t, map[string]any{
		"vars": map[string]string{"env": "prod"},
	})})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "text", Content: []byte("hello {{env}} world")},
	})
	require.NoError(t, err)
	require.Equal(t, "hello prod world", string(out.File.Content))
}

func TestTemplateMissingVarErrors(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "template"})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Kind: "text", Content: []byte("hello {{missing}}")},
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileProcessorFailed))
}

func TestTemplateCustomDelimiters(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{
		Type: "template",
		Params: params(t, map[string]any{
			"open":  "[[",
			"close": "]]",
			"vars":  map[string]string{"x": "ok"},
		}),
	})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Content: []byte("value=[[x]]\n")},
	})
	require.NoError(t, err)
	require.Equal(t, "value=ok\n", string(out.File.Content))
}

func TestTemplateUnterminatedDelimiter(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "template"})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File: domain.FileDocument{Content: []byte("{{broken\n")},
	})
	require.Error(t, err)
}

func TestTemplateUsesRequestMeta(t *testing.T) {
	r := makeFileRegistry()
	proc, err := r.BuildFile(domain.ProcessorSpec{Type: "template"})
	require.NoError(t, err)
	out, err := proc.ApplyFile(context.Background(), domain.FileProcessInput{
		File:    domain.FileDocument{Kind: "text", Content: []byte("trace={{trace_id}}")},
		Request: domain.RequestInfo{Meta: map[string]string{"trace_id": "abc"}},
	})
	require.NoError(t, err)
	require.Equal(t, "trace=abc", string(out.File.Content))
}
