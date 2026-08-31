package probe

import (
	"fmt"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func dimensionsForResults(results []domain.NodeProbeResult) []domain.ProbeReportDimension {
	type key struct {
		method string
		core   string
	}
	order := []key{}
	byKey := map[key]*domain.ProbeReportDimension{}
	for _, result := range results {
		k := key{method: result.Method, core: result.Core}
		dim := byKey[k]
		if dim == nil {
			order = append(order, k)
			dim = &domain.ProbeReportDimension{
				Method:      k.method,
				Core:        k.core,
				ErrorCounts: map[string]int{},
			}
			byKey[k] = dim
		}
		switch {
		case result.Alive:
			dim.SuccessCount++
		case result.ErrorCode == string(domain.CodeProbeNodeUnsupported):
			dim.UnsupportedCount++
		default:
			dim.FailureCount++
			if result.ErrorCode != "" {
				dim.ErrorCounts[result.ErrorCode]++
			}
		}
		if result.CacheHit {
			dim.CacheHitCount++
		}
	}
	out := make([]domain.ProbeReportDimension, 0, len(order))
	for _, k := range order {
		dim := *byKey[k]
		if len(dim.ErrorCounts) == 0 {
			dim.ErrorCounts = nil
		}
		out = append(out, dim)
	}
	return out
}

func successResult(req domain.ProbeRequest, node domain.NodeIR, durationMS int, checkedAt time.Time) domain.NodeProbeResult {
	if durationMS < 1 {
		durationMS = 1
	}
	return domain.NodeProbeResult{
		RuntimeID:  domain.NodeRuntimeID(node),
		NodeName:   node.Name,
		Method:     string(req.Method),
		Target:     targetFromRequest(req, node),
		Core:       req.Core,
		Alive:      true,
		DurationMS: durationMS,
		CheckedAt:  checkedAt,
	}
}

func resultForError(req domain.ProbeRequest, node domain.NodeIR, code string, err error, checkedAt time.Time) domain.NodeProbeResult {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return domain.NodeProbeResult{
		RuntimeID: domain.NodeRuntimeID(node),
		NodeName:  node.Name,
		Method:    string(req.Method),
		Target:    targetFromRequest(req, node),
		Core:      req.Core,
		Alive:     false,
		CheckedAt: checkedAt,
		ErrorCode: code,
		Error:     msg,
	}
}

// ReportForResults rebuilds aggregate probe diagnostics for an ordered node
// result set. Service uses it after merging cached and freshly probed nodes.
func ReportForResults(backend, version, method, core string, nodes []domain.NodeIR, results []domain.NodeProbeResult) domain.Report {
	probeReport := &domain.ProbeReport{
		Backend:      backend,
		Method:       method,
		Core:         core,
		ErrorCounts:  map[string]int{},
		SuccessCount: 0,
		FailureCount: 0,
	}
	if version != "" {
		probeReport.BackendVersion = version
	}
	for _, result := range results {
		if result.CacheHit {
			probeReport.CacheHitCount++
		}
		if result.Alive {
			probeReport.SuccessCount++
			continue
		}
		if result.ErrorCode == string(domain.CodeProbeNodeUnsupported) {
			probeReport.UnsupportedCount++
			continue
		}
		probeReport.FailureCount++
		if result.ErrorCode != "" {
			probeReport.ErrorCounts[result.ErrorCode]++
		}
	}
	warnings := probeWarnings(nodes, results)
	if len(probeReport.ErrorCounts) == 0 {
		probeReport.ErrorCounts = nil
	}
	probeReport.Dimensions = dimensionsForResults(results)
	return domain.Report{Probe: probeReport, Warnings: warnings}
}

func probeWarnings(nodes []domain.NodeIR, results []domain.NodeProbeResult) []domain.Warning {
	warnings := make([]domain.Warning, 0, len(results))
	for i, result := range results {
		if result.Alive || result.ErrorCode == "" {
			continue
		}
		warning := domain.Warning{
			Code:    result.ErrorCode,
			Message: probeWarningMessage(result),
		}
		if i < len(nodes) {
			node := nodes[i]
			nodeIndex := i
			warning.Node = node.Name
			warning.NodeIndex = &nodeIndex
			warning.NodeContext = probeWarningNodeContext(node)
		} else if result.NodeName != "" {
			warning.Node = result.NodeName
			warning.NodeContext = &domain.WarningNodeContext{Name: result.NodeName}
		}
		warnings = append(warnings, warning)
	}
	return warnings
}

func probeWarningMessage(result domain.NodeProbeResult) string {
	if result.Error != "" {
		return fmt.Sprintf("%s: %s", result.ErrorCode, result.Error)
	}
	return fmt.Sprintf("probe result reported %s", result.ErrorCode)
}

func probeWarningNodeContext(node domain.NodeIR) *domain.WarningNodeContext {
	return &domain.WarningNodeContext{
		Format: node.SourceFormat,
		Name:   node.Name,
		Type:   node.Type,
		Server: node.Server,
		Port:   node.Port,
	}
}
