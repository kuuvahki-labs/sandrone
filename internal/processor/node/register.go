package node

import (
	"context"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

type ProbeRunner interface {
	Probe(ctx context.Context, req domain.ProbeRequest) (*domain.ProbeResult, error)
}

// Register attaches all built-in node-stage processors to r. The optional
// probe runner is used only by probe.
func Register(r *processor.Registry, probes ...ProbeRunner) {
	var prober ProbeRunner
	if len(probes) > 0 {
		prober = probes[0]
	}
	r.RegisterNode("filter", buildFilter)
	r.RegisterNode("dedup", buildDedup)
	r.RegisterNode("rename", buildRename)
	r.RegisterNode("sort", buildSort)
	r.RegisterNode("quick_settings", buildQuickSettings)
	r.RegisterNode("probe", buildProbe(prober))
}
