//go:build !probe_mihomo && !probe_singbox

package probe_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
)

func TestURLTestUnavailableByDefault(t *testing.T) {
	engine := probe.New()
	_, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method: domain.ProbeURLTest,
		Core:   "mihomo",
		URL:    "http://127.0.0.1/generate_204",
	}, []domain.NodeIR{{Name: "node", Server: "127.0.0.1", Port: 8080}})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeCoreUnavailable))
}

func TestUDPNTPUnavailableByDefaultDoesNotFallBack(t *testing.T) {
	engine := probe.New()
	_, err := engine.Probe(context.Background(), domain.ProbeRequest{
		Method:    domain.ProbeAuto,
		NTPServer: "time.example.com",
	}, []domain.NodeIR{{
		Name:   "hy2",
		Type:   domain.NodeTypeHysteria2,
		Server: "127.0.0.1",
		Port:   443,
	}})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeCoreUnavailable))
}
