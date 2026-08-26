package probe

import (
	"context"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type DisabledEngine struct{}

func NewDisabled() *DisabledEngine {
	return &DisabledEngine{}
}

func (*DisabledEngine) ProbeAvailable() bool {
	return false
}

func (*DisabledEngine) BackendSummary() []domain.ProbeBackendSummary {
	return []domain.ProbeBackendSummary{}
}

func (*DisabledEngine) SelectCore(domain.ProbeRequest, []domain.NodeIR) (string, bool) {
	return "", false
}

func (*DisabledEngine) ResolveBackend(domain.ProbeRequest) (domain.ProbeBackendSummary, error) {
	return domain.ProbeBackendSummary{}, domain.NewError(domain.CodeProbeBackendUnavailable, "probe backend is not available")
}

func (*DisabledEngine) Probe(context.Context, domain.ProbeRequest, []domain.NodeIR, ...Payload) (*domain.ProbeResult, error) {
	return nil, domain.NewError(domain.CodeProbeBackendUnavailable, "probe backend is not available")
}
