package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceDisabledProbeFailsBeforeResolvingInput(t *testing.T) {
	svc := service.New(service.WithProbeEngine(probe.NewDisabled()))
	_, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{Ref: domain.ResourceRef{Kind: "subscription", Name: "missing-resource"}},
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeBackendUnavailable))
}

func TestDisabledProbeAndSchedulerUICapabilities(t *testing.T) {
	svc := service.New(
		service.WithProbeEngine(probe.NewDisabled()),
		service.WithSchedulerEnabled(false),
	)
	result, err := svc.ListUICapabilities(context.Background())
	require.NoError(t, err)
	for _, feature := range result.Features {
		switch feature.Key {
		case "probe.enabled", "core.mihomo", "core.sing_box", "scheduler.enabled":
			require.False(t, feature.Enabled, feature.Key)
		}
	}
}
