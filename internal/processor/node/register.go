package node

import (
	"context"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

type ProbeRunner interface {
	Probe(ctx context.Context, req domain.ProbeRequest) (*domain.ProbeResult, error)
}

type probeAvailability interface {
	ProbeAvailable() bool
}

// Register attaches all built-in node-stage processors to r. The optional
// probe runner is used only by probe.
func Register(r *processor.Registry, probes ...ProbeRunner) {
	var prober ProbeRunner
	if len(probes) > 0 {
		prober = probes[0]
	}
	r.RegisterNodeWithDescriptor("filter", buildFilter, processor.Descriptor{
		Description:     "Keep or drop nodes using a regex or explicit value set.",
		ParamsPrototype: FilterParams{}, Public: true,
		Examples:   []map[string]any{{"action": "keep", "field": "name", "match": "regex", "pattern": "HK|TW"}},
		ErrorCodes: []domain.ErrorCode{domain.CodeProcessorConfigInvalid},
	})
	r.RegisterNodeWithDescriptor("dedup", buildDedup, processor.Descriptor{
		Description:     "Remove duplicate nodes or disambiguate duplicate names with random digits.",
		ParamsPrototype: DedupParams{}, Public: true,
		Examples:   []map[string]any{{"strategy": "name"}, {"strategy": "random_suffix"}},
		ErrorCodes: []domain.ErrorCode{domain.CodeProcessorConfigInvalid},
	})
	r.RegisterNodeWithDescriptor("rename", buildRename, processor.Descriptor{
		Description:     "Clean up or rewrite node names.",
		ParamsPrototype: RenameParams{}, Public: true,
		Examples:   []map[string]any{{"mode": "prefix", "value": "Edge - "}},
		ErrorCodes: []domain.ErrorCode{domain.CodeProcessorConfigInvalid},
	})
	r.RegisterNodeWithDescriptor("sort", buildSort, processor.Descriptor{
		Description:     "Stably sort nodes by one or more fields.",
		ParamsPrototype: SortParams{}, Public: true,
		Examples:   []map[string]any{{"by": "+name"}},
		ErrorCodes: []domain.ErrorCode{domain.CodeProcessorConfigInvalid},
	})
	r.RegisterNodeWithDescriptor("quick_settings", buildQuickSettings, processor.Descriptor{
		Description:     "Apply common transport and protocol toggles to nodes.",
		ParamsPrototype: QuickSettingsParams{}, Public: true,
		Examples:   []map[string]any{{"udp": "enabled", "tfo": "default"}},
		ErrorCodes: []domain.ErrorCode{domain.CodeProcessorConfigInvalid},
	})
	r.RegisterNodeWithDescriptor("probe", buildProbe(prober), processor.Descriptor{
		Description:     "Probe node availability and optionally annotate, filter, or sort results.",
		ParamsPrototype: ProbeParams{}, Public: true,
		Effects:  processor.Effects{Probes: true},
		Examples: []map[string]any{{"method": "url_test", "core": "sing-box", "fail_mode": "keep", "timeout_ms": 5000}},
		ErrorCodes: []domain.ErrorCode{
			domain.CodeProcessorConfigInvalid,
			domain.CodeProbeBackendUnavailable,
			domain.CodeProbeCoreUnavailable,
			domain.CodeProbeCoreStartFailed,
			domain.CodeProbeCoreAPIFailed,
			domain.CodeProbeNodeUnsupported,
			domain.CodeProbeInvalidTarget,
			domain.CodeProbeTimeout,
			domain.CodeProbeTCPFailed,
			domain.CodeProbeUDPNTPFailed,
		},
	})
}
