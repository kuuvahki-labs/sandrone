package probe

import (
	"fmt"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func dimensionsForResults(results []domain.NodeProbeResult) []domain.ProbeReportDimension {
	type key struct {
		layer  string
		method string
		core   string
	}
	order := []key{}
	byKey := map[key]*domain.ProbeReportDimension{}
	for _, result := range results {
		k := key{layer: result.Layer, method: result.Method, core: result.Core}
		dim := byKey[k]
		if dim == nil {
			order = append(order, k)
			dim = &domain.ProbeReportDimension{
				Layer:       k.layer,
				Method:      k.method,
				Core:        k.core,
				ErrorCounts: map[string]int{},
			}
			byKey[k] = dim
		}
		if result.Alive {
			dim.SuccessCount++
		} else {
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

func combineReports(reports []domain.Report, nodes []domain.NodeIR, results []domain.NodeProbeResult) domain.Report {
	report := domain.Report{
		Probe: &domain.ProbeReport{
			Backend:     "mixed",
			ErrorCounts: map[string]int{},
			Dimensions:  dimensionsForResults(results),
		},
		Warnings: probeWarnings(nodes, results),
	}
	for _, result := range results {
		if result.Alive {
			report.Probe.SuccessCount++
		} else {
			report.Probe.FailureCount++
			if result.ErrorCode != "" {
				report.Probe.ErrorCounts[result.ErrorCode]++
			}
		}
		if result.CacheHit {
			report.Probe.CacheHitCount++
		}
	}
	for _, source := range reports {
		for _, warning := range source.Warnings {
			if warning.Node == "" && warning.NodeIndex == nil {
				report.Warnings = append(report.Warnings, warning)
			}
		}
	}
	if len(report.Probe.ErrorCounts) == 0 {
		report.Probe.ErrorCounts = nil
	}
	return report
}

func successResult(req domain.ProbeRequest, node domain.NodeIR, durationMS int, checkedAt time.Time) domain.NodeProbeResult {
	if durationMS < 1 {
		durationMS = 1
	}
	return domain.NodeProbeResult{
		NodeID:     node.ID,
		NodeName:   node.Name,
		Layer:      string(req.Layer),
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
		NodeID:    node.ID,
		NodeName:  node.Name,
		Layer:     string(req.Layer),
		Method:    string(req.Method),
		Target:    targetFromRequest(req, node),
		Core:      req.Core,
		Alive:     false,
		CheckedAt: checkedAt,
		ErrorCode: code,
		Error:     msg,
	}
}

func reportForResults(backend, version, layer, method, core string, nodes []domain.NodeIR, results []domain.NodeProbeResult) domain.Report {
	probeReport := &domain.ProbeReport{
		Backend:      backend,
		Layer:        layer,
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
		if result.Alive {
			probeReport.SuccessCount++
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
