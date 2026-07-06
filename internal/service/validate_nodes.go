package service

import (
	"context"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
)

func (s *Service) ValidateNodes(ctx context.Context, req domain.ParseRequest) (*domain.ValidateResult, error) {
	parsed, err := s.parseRequestInput(ctx, req)
	if err != nil {
		return nil, err
	}
	report := domain.Report{}
	if parsed.Source != nil {
		report.SourceRefs = append(report.SourceRefs, parsed.Source.SourceRefs...)
		report.Warnings = append(report.Warnings, parsed.Source.Warnings...)
	}
	for _, node := range parsed.Nodes {
		report.Warnings = append(report.Warnings, node.Warnings...)
	}
	validated := nodevalidation.Validate(parsed.Nodes, nodevalidation.StageNormalized, req.Target)
	issues := append([]domain.ValidationIssue{}, validated.Issues...)
	counts := validated.Counts
	nodes := validated.Nodes
	if len(req.Processors) > 0 && len(nodes) > 0 {
		processed, processErr := s.registry.RunNodes(ctx, req.Processors, domain.NodeProcessInput{
			Target: req.Target,
			Nodes:  nodes,
			Context: domain.NodeContext{
				Sources: sourcesSlice(parsed.Source),
			},
			Request: domain.RequestInfo{Meta: req.Meta},
		})
		if processErr != nil {
			return nil, processErr
		}
		report.Warnings = append(report.Warnings, processed.Warnings...)
		post := nodevalidation.Validate(processed.Nodes, nodevalidation.StageProcessed, req.Target)
		issues = append(issues, post.Issues...)
		counts.Valid = post.Counts.Valid
		counts.Invalid += post.Counts.Invalid
		counts.Errors += post.Counts.Errors
		counts.Warnings += post.Counts.Warnings
	}
	report = s.prepareReport("validate_nodes", report)
	return &domain.ValidateResult{
		OK:     len(issues) == 0,
		Counts: counts,
		Issues: issues,
		Report: report,
	}, nil
}
