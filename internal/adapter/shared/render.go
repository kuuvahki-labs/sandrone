package shared

import (
	"encoding/json"
	"sort"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func ParseUnknownWarningsWithContext(node domain.NodeIR, raw map[string]json.RawMessage, source string, nodeIndex *int, nodeContext *domain.WarningNodeContext) []domain.Warning {
	if len(raw) == 0 {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	warnings := make([]domain.Warning, 0, len(keys))
	for _, key := range keys {
		warnings = append(warnings, domain.Warning{
			Code:        "parse_unknown_field",
			Message:     "field preserved in NodeIR Raw",
			Node:        node.Name,
			Field:       key,
			Source:      source,
			NodeIndex:   nodeIndex,
			NodeContext: nodeContext,
		})
	}
	return warnings
}

func RawWarnings(node domain.NodeIR, skip map[string]bool, target string) []domain.Warning {
	if len(node.Raw) == 0 {
		return nil
	}
	keys := make([]string, 0, len(node.Raw))
	for key := range node.Raw {
		if skip != nil && skip[key] {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	warnings := make([]domain.Warning, 0, len(keys))
	for _, key := range keys {
		message := "field preserved in NodeIR Raw but not emitted by " + target + " renderer"
		if target == "mihomo-proxies" {
			message = "field preserved in NodeIR Raw but not emitted by mihomo renderer"
		}
		warnings = append(warnings, domain.Warning{
			Code:    "render_lossy_field",
			Message: message,
			Node:    node.Name,
			Field:   key,
			Target:  target,
		})
	}
	return warnings
}

func RenderLossyWarning(node domain.NodeIR, target, field, message string) domain.Warning {
	if message == "" {
		message = "field represented in NodeIR but not emitted by " + target + " renderer"
	}
	return domain.Warning{
		Code:    "render_lossy_field",
		Message: message,
		Node:    node.Name,
		Field:   field,
		Target:  target,
	}
}

func RenderNodeSkippedWarning(node domain.NodeIR, target string, err error) domain.Warning {
	message := "node skipped by " + target + " renderer"
	if err != nil {
		message = err.Error()
	}
	return domain.Warning{
		Code:    "render_node_skipped",
		Message: message,
		Node:    node.Name,
		Field:   string(node.Type),
		Target:  target,
		NodeContext: &domain.WarningNodeContext{
			Format: target,
			Name:   node.Name,
			Type:   node.Type,
			Server: node.Server,
			Port:   node.Port,
		},
	}
}

func NoRenderableNodesError(report domain.RenderReport) error {
	message := "no renderable nodes"
	for _, warning := range report.Warnings {
		if warning.Code == "render_node_skipped" && warning.Message != "" {
			message += ": " + warning.Message
			break
		}
	}
	return domain.NewError(domain.CodeRenderFailed, message)
}

func MarshalStableJSON(v any, indent bool) ([]byte, error) {
	if indent {
		return json.MarshalIndent(v, "", "  ")
	}
	return json.Marshal(v)
}

func MergeWarnings(report *domain.RenderReport, warnings []domain.Warning) {
	report.Warnings = append(report.Warnings, warnings...)
	report.LostFields += len(warnings)
}
