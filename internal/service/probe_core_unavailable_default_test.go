//go:build !probe_mihomo && !probe_singbox

package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceURLTestUnavailableSkipsRender(t *testing.T) {
	svc := service.New()
	_, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name: "unsupported",
				Type: "not-a-renderable-node",
			}},
		},
		Method: domain.ProbeURLTest,
		Core:   "mihomo",
		URL:    "http://127.0.0.1/generate_204",
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeCoreUnavailable))
}
