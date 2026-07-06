package service_test

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceParseDropsSemanticallyInvalidNodes(t *testing.T) {
	t.Parallel()

	svc := service.New()
	result, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format: "json-nodes",
		Content: []byte(`{"nodes":[
  {"id":"valid","name":"valid","type":"ss","server":"example.com","port":443,"cipher":"aes-128-gcm","password":"secret"},
  {"id":"invalid","name":"invalid","type":"trojan","server":"https://bad.example/path","port":0,"password":"must-not-leak"}
]}`),
	})

	require.NoError(t, err)
	require.Len(t, result.Nodes, 1)
	require.Equal(t, "valid", result.Nodes[0].ID)
	require.Len(t, result.Report.Warnings, 1)
	require.Equal(t, "node_validation_dropped", result.Report.Warnings[0].Code)
	require.Equal(t, "invalid", result.Report.Warnings[0].Node)
	require.NotContains(t, result.Report.Warnings[0].Message, "must-not-leak")
}

func TestServiceRenderValidatesDirectAndProcessorProducedNodes(t *testing.T) {
	t.Parallel()

	svc := service.New(service.WithProcessor(func(registry *processor.Registry) {
		registry.RegisterNode("invalidate", func(domain.ProcessorSpec) (domain.NodeProcessor, error) {
			return invalidatingProcessor{}, nil
		})
	}))
	valid := domain.NodeIR{
		ID: "valid", Name: "valid", Type: domain.NodeTypeShadowsocks,
		Server: "example.com", Port: 443, Cipher: "aes-128-gcm", Password: "secret",
	}

	_, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "json-nodes",
		Nodes:  []domain.NodeIR{valid},
		Processors: []domain.ProcessorSpec{
			{Type: "invalidate"},
		},
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeNodeValidationFailed))
}

func TestServiceRenderAllowsProcessorToFilterEveryValidNode(t *testing.T) {
	t.Parallel()

	svc := service.New()
	result, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "json-nodes",
		Nodes: []domain.NodeIR{{
			Name: "valid", Type: domain.NodeTypeShadowsocks, Server: "example.com",
			Port: 443, Cipher: "aes-128-gcm", Password: "secret",
		}},
		Processors: []domain.ProcessorSpec{{
			Type: "filter",
			Params: params(t, map[string]any{
				"action": "keep", "field": "name", "match": "regex", "pattern": "^does-not-match$",
			}),
		}},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
}

type invalidatingProcessor struct{}

func (invalidatingProcessor) Name() string { return "invalidate" }

func (invalidatingProcessor) ApplyNodes(_ context.Context, input domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	nodes := append([]domain.NodeIR{}, input.Nodes...)
	for index := range nodes {
		nodes[index].Port = 0
	}
	return domain.NodeProcessOutput{Nodes: nodes}, nil
}

func TestServiceParseFailsWhenEveryInputNodeIsInvalid(t *testing.T) {
	t.Parallel()

	svc := service.New()
	_, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format:  "json-nodes",
		Content: []byte(`{"nodes":[{"name":"bad","type":"trojan","server":"bad.example","port":0,"password":"secret"}]}`),
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeNodeValidationFailed))
}

func TestServiceValidateNodesReturnsSemanticDiagnosticsWithoutDroppingToAnError(t *testing.T) {
	t.Parallel()

	svc := service.New()
	result, err := svc.ValidateNodes(context.Background(), domain.ParseRequest{
		Format:  "json-nodes",
		Content: []byte(`{"nodes":[{"id":"bad","name":"bad","type":"trojan","server":"bad.example","port":0,"password":"secret"}]}`),
	})

	require.NoError(t, err)
	require.False(t, result.OK)
	require.Equal(t, domain.ValidationCounts{Input: 1, Invalid: 1, Errors: 1}, result.Counts)
	require.Len(t, result.Issues, 1)
	require.Equal(t, "port", result.Issues[0].Field)
}

func TestServiceSubscriptionPreviewValidatesBaseAndProcessedNodes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "inline", Type: domain.SubscriptionTypeCollection,
		Nodes: []domain.NodeIR{
			{Name: "valid", Type: domain.NodeTypeShadowsocks, Server: "example.com", Port: 443, Cipher: "aes-128-gcm", Password: "secret"},
			{Name: "invalid", Type: domain.NodeTypeTrojan, Server: "bad.example", Password: "secret"},
		},
	}))

	preview, err := svc.PreviewSubscription(ctx, "inline")
	require.NoError(t, err)
	require.Equal(t, 1, preview.BeforeCount)
	require.Equal(t, 1, preview.AfterCount)
	require.Contains(t, warningCodes(preview.Report.Warnings), "node_validation_dropped")
}

func warningCodes(warnings []domain.Warning) []string {
	codes := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		codes = append(codes, warning.Code)
	}
	return codes
}
