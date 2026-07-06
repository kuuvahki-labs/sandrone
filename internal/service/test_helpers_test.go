package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
)

func raw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
func params(t *testing.T, m map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	for k, v := range m {
		out[k] = raw(t, v)
	}
	return out
}

func inlineScriptSource(content string) map[string]any {
	return map[string]any{"type": "inline", "content": content}
}

func fileScriptSource(name string) map[string]any {
	return map[string]any{"type": "file", "name": name}
}

type targetRecorder struct {
	targets *[]string
}

func (p targetRecorder) Name() string { return "record_target" }

func (p targetRecorder) ApplyNodes(_ context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	*p.targets = append(*p.targets, in.Target)
	return domain.NodeProcessOutput{Nodes: append([]domain.NodeIR{}, in.Nodes...)}, nil
}

type fakeProbeEngine struct {
	probe func(context.Context, domain.ProbeRequest, []domain.NodeIR, ...probe.Payload) (*domain.ProbeResult, error)
}

func (e fakeProbeEngine) Probe(ctx context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, payloads ...probe.Payload) (*domain.ProbeResult, error) {
	return e.probe(ctx, req, nodes, payloads...)
}

type stubTagProcessor struct{}

func (stubTagProcessor) Name() string { return "tag_x" }

func (stubTagProcessor) ApplyNodes(_ context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	out := make([]domain.NodeIR, len(in.Nodes))
	for i, n := range in.Nodes {
		n.Tags = append(append([]string{}, n.Tags...), "x")
		out[i] = n
	}
	return domain.NodeProcessOutput{Nodes: out}, nil
}
