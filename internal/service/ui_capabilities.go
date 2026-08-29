package service

import (
	"context"
	"slices"
	"sort"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type probeCapabilityProvider interface {
	BackendSummary() []domain.ProbeBackendSummary
}

func (s *Service) ListUICapabilities(_ context.Context) (*domain.UICapabilityListResult, error) {
	features := map[string]domain.UICapability{}
	setFeature := func(key string, enabled bool, reason string, dependencies ...string) {
		features[key] = domain.UICapability{
			Key:          key,
			Enabled:      enabled,
			Reason:       reason,
			Dependencies: slices.Clone(dependencies),
		}
	}

	backendSummary := s.probeBackendSummary()
	probeEnabled := s.probeEnabled()
	setFeature("probe.enabled", probeEnabled, capabilityReason(probeEnabled, "probe backend is not available"))

	cores := map[string]bool{}
	for _, backend := range backendSummary {
		if backend.Core != "" {
			cores[backend.Core] = true
		}
	}
	setFeature("core.mihomo", cores["mihomo"], capabilityReason(cores["mihomo"], "mihomo probe backend is not available"), "probe.enabled")
	setFeature("core.sing_box", cores["sing-box"], capabilityReason(cores["sing-box"], "sing-box probe backend is not available"), "probe.enabled")

	setFeature("scheduler.enabled", s.schedulerEnabled, capabilityReason(s.schedulerEnabled, "scheduler is not available"))

	items := make([]domain.UICapability, 0, len(features))
	for _, feature := range features {
		feature.Dependencies = slices.Clone(feature.Dependencies)
		items = append(items, feature)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return &domain.UICapabilityListResult{Features: items}, nil
}

func (s *Service) probeEnabled() bool {
	if availability, ok := s.prober.(probeAvailability); ok {
		return availability.ProbeAvailable()
	}
	if _, ok := s.prober.(probeCapabilityProvider); ok {
		return len(s.probeBackendSummary()) > 0
	}
	return s.prober != nil
}

func (s *Service) probeBackendSummary() []domain.ProbeBackendSummary {
	provider, ok := s.prober.(probeCapabilityProvider)
	if !ok {
		return []domain.ProbeBackendSummary{}
	}
	return provider.BackendSummary()
}

func capabilityReason(enabled bool, reason string) string {
	if enabled {
		return ""
	}
	return reason
}
