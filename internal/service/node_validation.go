package service

import (
	"fmt"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
)

func validateNodeBatch(nodes []domain.NodeIR, stage nodevalidation.Stage, target string) (nodevalidation.Result, []domain.Warning, error) {
	nodes = normalizeNodeUUIDs(nodes)
	result := nodevalidation.Validate(nodes, stage, target)
	if result.Counts.Invalid == 0 {
		return result, nil, nil
	}
	warnings := validationDropWarnings(nodes, result.Issues, stage, target)
	if result.Counts.Input > 0 && result.Counts.Valid == 0 {
		return result, warnings, domain.NewError(
			domain.CodeNodeValidationFailed,
			fmt.Sprintf("all %d node(s) failed semantic validation", result.Counts.Input),
		)
	}
	return result, warnings, nil
}

func validationDropWarnings(nodes []domain.NodeIR, issues []domain.ValidationIssue, stage nodevalidation.Stage, target string) []domain.Warning {
	counts := map[int]int{}
	fields := map[int]string{}
	for _, issue := range issues {
		if issue.NodeIndex == nil {
			continue
		}
		index := *issue.NodeIndex
		counts[index]++
		if fields[index] == "" {
			fields[index] = issue.Field
		}
	}
	warnings := make([]domain.Warning, 0, len(counts))
	for index, node := range nodes {
		count := counts[index]
		if count == 0 {
			continue
		}
		nodeIndex := index
		nodeID := node.ID
		if nodeID == "" {
			nodeID = node.Name
		}
		warnings = append(warnings, domain.Warning{
			Code:      "node_validation_dropped",
			Message:   fmt.Sprintf("node failed semantic validation and was dropped (%d issue(s))", count),
			Node:      nodeID,
			NodeIndex: &nodeIndex,
			Field:     fields[index],
			Source:    string(stage),
			Target:    target,
		})
	}
	return warnings
}
