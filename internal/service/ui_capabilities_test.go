package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
)

type uiCapabilityProbe struct {
	backends []domain.ProbeBackendSummary
}

func (p uiCapabilityProbe) Probe(context.Context, domain.ProbeRequest, []domain.NodeIR, ...probe.Payload) (*domain.ProbeResult, error) {
	return nil, nil
}

func (p uiCapabilityProbe) BackendSummary() []domain.ProbeBackendSummary {
	return p.backends
}

func TestListUICapabilitiesReflectsRuntimeFeatures(t *testing.T) {
	svc := New(WithProbeEngine(uiCapabilityProbe{backends: []domain.ProbeBackendSummary{
		{Method: domain.ProbeTCPConnect, Name: "tcp"},
		{Method: domain.ProbeURLTest, Name: "mihomo", Core: "mihomo"},
	}}), WithSchedulerEnabled(true))

	result, err := svc.ListUICapabilities(context.Background())
	require.NoError(t, err)

	features := make(map[string]domain.UICapability, len(result.Features))
	for _, feature := range result.Features {
		features[feature.Key] = feature
	}
	require.True(t, features["probe.enabled"].Enabled)
	require.True(t, features["scheduler.enabled"].Enabled)
	require.True(t, features["core.mihomo"].Enabled)
	require.False(t, features["core.sing_box"].Enabled)
}

func TestListUICapabilitiesCanDisableScheduler(t *testing.T) {
	result, err := New(WithSchedulerEnabled(false)).ListUICapabilities(context.Background())
	require.NoError(t, err)
	for _, feature := range result.Features {
		if feature.Key == "scheduler.enabled" {
			require.False(t, feature.Enabled)
			require.Equal(t, "scheduler is not available", feature.Reason)
			return
		}
	}
	t.Fatal("scheduler.enabled feature not found")
}
