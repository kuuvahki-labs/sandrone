package processor_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

func TestRegistryBuildUnknownProcessors(t *testing.T) {
	r := processor.NewRegistry()
	_, err := r.BuildNode(domain.ProcessorSpec{Type: "missing"})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorUnknown))

	_, err = r.BuildFile(domain.ProcessorSpec{Type: "missing"})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorUnknown))
}

func TestRegistryResolveStageInference(t *testing.T) {
	r := processor.NewRegistry()
	r.RegisterNode("only_nodes", func(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
		return &nodeStub{name: "only_nodes", fn: func(in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
			return domain.NodeProcessOutput{Nodes: in.Nodes}, nil
		}}, nil
	})
	stage, err := r.ResolveStage(domain.ProcessorSpec{Type: "only_nodes"})
	require.NoError(t, err)
	require.Equal(t, domain.StageNodes, stage)

	_, err = r.ResolveStage(domain.ProcessorSpec{Type: "absent"})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorUnknown))
}
