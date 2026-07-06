//go:build probe_singbox

package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceSingBoxURLTestRejectsInvalidURL(t *testing.T) {
	svc := service.New()
	_, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{
			Type: "inline_nodes",
			Nodes: []domain.NodeIR{{
				Name:   "local-http-proxy",
				Type:   domain.NodeTypeHTTP,
				Server: "127.0.0.1",
				Port:   8080,
			}},
		},
		Method:    domain.ProbeURLTest,
		Core:      "sing-box",
		URL:       "ftp://127.0.0.1/generate_204",
		TimeoutMS: 2000,
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeInvalidTarget))
}
