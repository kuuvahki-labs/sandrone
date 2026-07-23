package script_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/processor/script"
)

func TestScriptAPIProbeAcceptsOmittedAndNullOptionsAndIgnoresExtras(t *testing.T) {
	runner := &scriptProbeRunner{result: &domain.ProbeResult{}}
	registry := processor.NewRegistry()
	registerScript(registry, script.WithProbeRunner(runner))
	proc, err := registry.BuildNode(domain.ProcessorSpec{
		Type: "script", Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(`
function main(input, api) {
  var zeroRejected = false;
  try { api.probe(); } catch (error) { zeroRejected = true; }
  if (!zeroRejected) throw new Error("zero arguments must fail");
  api.probe(input.nodes);
  api.probe(input.nodes, null, "ignored");
  api.probe(input.nodes, undefined);
  return input;
}`),
		}),
	})
	require.NoError(t, err)

	_, err = proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{
		Name: "node", Type: domain.NodeTypeShadowsocks, Server: "example.com", Port: 443,
	}}})

	require.NoError(t, err)
	require.Len(t, runner.requests, 3)
}
