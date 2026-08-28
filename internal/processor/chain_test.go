package processor_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

type nodeStub struct {
	name string
	fn   func(in domain.NodeProcessInput) (domain.NodeProcessOutput, error)
}

func (s *nodeStub) Name() string { return s.name }
func (s *nodeStub) ApplyNodes(_ context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	return s.fn(in)
}

type fileStub struct {
	name string
	fn   func(in domain.FileProcessInput) (domain.FileProcessOutput, error)
}

func (s *fileStub) Name() string { return s.name }
func (s *fileStub) ApplyFile(_ context.Context, in domain.FileProcessInput) (domain.FileProcessOutput, error) {
	return s.fn(in)
}

func newTestRegistry(t *testing.T) *processor.Registry {
	t.Helper()
	r := processor.NewRegistry()
	r.RegisterNode("append_tag", func(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
		var params struct {
			Tag string `json:"tag"`
		}
		if err := processor.UnmarshalParams(spec, &params); err != nil {
			return nil, err
		}
		return &nodeStub{name: "append_tag", fn: func(in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
			out := domain.NodeProcessOutput{Nodes: make([]domain.NodeIR, len(in.Nodes))}
			for i, n := range in.Nodes {
				n.Tags = append(append([]string{}, n.Tags...), params.Tag)
				out.Nodes[i] = n
			}
			return out, nil
		}}, nil
	})
	r.RegisterNode("boom_nodes", func(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
		return &nodeStub{name: "boom_nodes", fn: func(in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
			return domain.NodeProcessOutput{}, errors.New("boom")
		}}, nil
	})
	r.RegisterFile("annotate", func(spec domain.ProcessorSpec) (domain.FileProcessor, error) {
		return &fileStub{name: "annotate", fn: func(in domain.FileProcessInput) (domain.FileProcessOutput, error) {
			doc := in.File
			doc.Content = append([]byte(nil), append(in.File.Content, []byte("\n# annotated")...)...)
			return domain.FileProcessOutput{File: doc, Warnings: []domain.Warning{{Code: "ann"}}}, nil
		}}, nil
	})
	r.RegisterNode("dual_stage", func(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
		return &nodeStub{name: "dual_stage", fn: func(in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
			return domain.NodeProcessOutput{Nodes: in.Nodes}, nil
		}}, nil
	})
	r.RegisterFile("dual_stage", func(spec domain.ProcessorSpec) (domain.FileProcessor, error) {
		return &fileStub{name: "dual_stage", fn: func(in domain.FileProcessInput) (domain.FileProcessOutput, error) {
			return domain.FileProcessOutput{File: in.File}, nil
		}}, nil
	})
	return r
}

func TestRunNodesPreservesDeclarationOrder(t *testing.T) {
	r := newTestRegistry(t)
	specs := []domain.ProcessorSpec{
		{Type: "append_tag", Params: rawParams(t, map[string]any{"tag": "first"})},
		{Type: "append_tag", Params: rawParams(t, map[string]any{"tag": "second"})},
	}
	out, err := r.RunNodes(context.Background(), specs, domain.NodeProcessInput{
		Nodes: []domain.NodeIR{{Name: "n"}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"first", "second"}, out.Nodes[0].Tags)
}

func TestRunNodesSkipsDisabledProcessorsAndPreservesTheirConfiguration(t *testing.T) {
	r := newTestRegistry(t)
	disabled := false
	specs := []domain.ProcessorSpec{
		{Type: "unknown-while-disabled", Enabled: &disabled, Params: rawParams(t, map[string]any{"keep": "config"})},
		{Type: "append_tag", Params: rawParams(t, map[string]any{"tag": "active"})},
	}
	out, err := r.RunNodes(context.Background(), specs, domain.NodeProcessInput{
		Nodes: []domain.NodeIR{{Name: "n"}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"active"}, out.Nodes[0].Tags)
	require.Equal(t, `"config"`, string(specs[0].Params["keep"]))
}

func TestRunNodesEmptyChainPassesThrough(t *testing.T) {
	r := newTestRegistry(t)
	nodes := []domain.NodeIR{{Name: "n"}}
	out, err := r.RunNodes(context.Background(), nil, domain.NodeProcessInput{Nodes: nodes})
	require.NoError(t, err)
	require.Equal(t, nodes, out.Nodes)
}

func TestRunNodesPropagatesAppError(t *testing.T) {
	r := newTestRegistry(t)
	_, err := r.RunNodes(context.Background(), []domain.ProcessorSpec{{Type: "boom_nodes"}}, domain.NodeProcessInput{})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeNodeProcessorFailed))
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "boom_nodes", appErr.Processor)
}

func TestSelectSpecsRoutesByStage(t *testing.T) {
	r := newTestRegistry(t)
	specs := []domain.ProcessorSpec{
		{Type: "append_tag"},
		{Type: "annotate"},
	}
	nodeSpecs, err := r.SelectSpecs(specs, domain.StageNodes)
	require.NoError(t, err)
	require.Len(t, nodeSpecs, 1)
	require.Equal(t, "append_tag", nodeSpecs[0].Type)

	fileSpecs, err := r.SelectSpecs(specs, domain.StageFile)
	require.NoError(t, err)
	require.Len(t, fileSpecs, 1)
	require.Equal(t, "annotate", fileSpecs[0].Type)
}

func TestResolveStageAmbiguous(t *testing.T) {
	r := newTestRegistry(t)
	_, err := r.SelectSpecs([]domain.ProcessorSpec{{Type: "dual_stage"}}, domain.StageNodes)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}

func TestResolveStageUnknownType(t *testing.T) {
	r := newTestRegistry(t)
	_, err := r.SelectSpecs([]domain.ProcessorSpec{{Type: "nope"}}, domain.StageNodes)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorUnknown))
}

func TestRunFileChain(t *testing.T) {
	r := newTestRegistry(t)
	in := domain.FileProcessInput{File: domain.FileDocument{Name: "x", Content: []byte("a")}}
	out, err := r.RunFile(context.Background(), []domain.ProcessorSpec{{Type: "annotate", Stage: domain.StageFile}}, in)
	require.NoError(t, err)
	require.Contains(t, string(out.File.Content), "# annotated")
	require.Len(t, out.Warnings, 1)
}

func TestRunFileSkipsDisabledProcessors(t *testing.T) {
	r := newTestRegistry(t)
	disabled := false
	in := domain.FileProcessInput{File: domain.FileDocument{Name: "x", Content: []byte("a")}}
	out, err := r.RunFile(context.Background(), []domain.ProcessorSpec{{
		Type: "annotate", Stage: domain.StageFile, Enabled: &disabled,
	}}, in)
	require.NoError(t, err)
	require.Equal(t, "a", string(out.File.Content))
	require.Empty(t, out.Warnings)
}

func TestRunFileDoesNotMutateInputParts(t *testing.T) {
	r := newTestRegistry(t)
	meta := map[string]string{"k": "v"}
	in := domain.FileProcessInput{
		File: domain.FileDocument{Name: "x", Content: []byte("base"), Meta: meta},
		Parts: []domain.FilePart{{
			Name: "p1", Role: "base", Content: []byte("part"),
			Nodes: []domain.NodeIR{{Name: "n", Type: domain.NodeTypeShadowsocks}},
		}},
	}
	_, err := r.RunFile(context.Background(), []domain.ProcessorSpec{{Type: "annotate", Stage: domain.StageFile}}, in)
	require.NoError(t, err)
	require.Equal(t, []byte("base"), in.File.Content)
	require.Equal(t, "v", in.File.Meta["k"])
	require.Equal(t, []byte("part"), in.Parts[0].Content)
	require.Equal(t, "n", in.Parts[0].Nodes[0].Name)
}

func TestRunNodesWrapsGenericError(t *testing.T) {
	r := processor.NewRegistry()
	r.RegisterNode("generic_err", func(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
		return &nodeStub{name: "generic_err", fn: func(in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
			return domain.NodeProcessOutput{}, errors.New("plain")
		}}, nil
	})
	_, err := r.RunNodes(context.Background(), []domain.ProcessorSpec{{Type: "generic_err"}}, domain.NodeProcessInput{})
	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, domain.CodeNodeProcessorFailed, appErr.Code)
	require.Equal(t, "generic_err", appErr.Processor)
}

func TestUnmarshalParamsRejectsUnknown(t *testing.T) {
	spec := domain.ProcessorSpec{
		Type:   "x",
		Params: rawParams(t, map[string]any{"extra": "fail"}),
	}
	var target struct {
		Known string `json:"known"`
	}
	err := processor.UnmarshalParams(spec, &target)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}
