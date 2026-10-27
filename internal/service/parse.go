package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
)

// Parse parses the request content using the format-specific adapter and
// optionally runs the request's processor chain over the resulting nodes.
func (s *Service) Parse(ctx context.Context, req domain.ParseRequest) (*domain.ParseResult, error) {
	start := time.Now()
	result, err := s.parse(ctx, req)
	if err != nil {
		s.log(ctx, slog.LevelError, "service parse failed",
			"operation", "parse",
			"format", req.Format,
			"target", req.Target,
			"processor_count", len(req.Processors),
			"remote", req.Remote != nil,
			"duration_ms", elapsedMillis(start),
			"error", err.Error(),
		)
		return result, err
	}
	s.log(ctx, slog.LevelInfo, "service parse completed",
		"operation", "parse",
		"format", req.Format,
		"target", req.Target,
		"processor_count", len(req.Processors),
		"remote", req.Remote != nil,
		"node_count", len(result.Nodes),
		"warning_count", len(result.Report.Warnings),
		"duration_ms", elapsedMillis(start),
	)
	return result, nil
}

func (s *Service) parse(ctx context.Context, req domain.ParseRequest) (*domain.ParseResult, error) {
	parsed, err := s.parseRequestInput(ctx, req)
	if err != nil {
		if parsed != nil {
			return &domain.ParseResult{Report: s.prepareReport("parse", domain.Report{Status: "failed", SourceResults: parsed.SourceResults})}, err
		}
		return nil, err
	}
	nodes := parsed.Nodes
	source := parsed.Source
	report := domain.Report{Status: parsed.Status, SourceResults: parsed.SourceResults}
	if source != nil {
		report.SourceRefs = append(report.SourceRefs, source.SourceRefs...)
		report.Warnings = append(report.Warnings, source.Warnings...)
	}
	for _, n := range nodes {
		report.Warnings = append(report.Warnings, n.Warnings...)
	}
	validated, validationWarnings, err := validateNodeBatch(nodes, nodevalidation.StageNormalized, req.Target)
	if err != nil {
		return failedBatchParseResult(report), err
	}
	nodes = validated.Nodes
	if req.RemoteBatch != nil && len(nodes) == 0 {
		return failedBatchParseResult(report), domain.NewError(domain.CodeNodeValidationFailed, "remote batch produced no valid nodes")
	}
	report.Warnings = append(report.Warnings, validationWarnings...)
	if len(req.Processors) > 0 {
		processed, err := s.registry.RunNodes(ctx, req.Processors, domain.NodeProcessInput{
			Target: req.Target,
			Nodes:  nodes,
			Context: domain.NodeContext{
				Sources: sourcesSlice(source),
			},
			Request: domain.RequestInfo{Meta: req.Meta},
		})
		if err != nil {
			return failedBatchParseResult(report), err
		}
		nodes = processed.Nodes
		report.Warnings = append(report.Warnings, processed.Warnings...)
		validated, validationWarnings, err = validateNodeBatch(nodes, nodevalidation.StageProcessed, req.Target)
		if err != nil {
			return failedBatchParseResult(report), err
		}
		nodes = validated.Nodes
		report.Warnings = append(report.Warnings, validationWarnings...)
	}
	return &domain.ParseResult{Nodes: nodes, Source: source, Report: report}, nil
}

func (s *Service) invokeParser(ctx context.Context, parser Parser, format string, content []byte) ([]domain.NodeIR, *domain.SourceInfo, error) {
	if uriParser, ok := parser.(*uri.Parser); ok && normalizeFormat(format) != "uri" {
		return uriParser.ParseList(ctx, content)
	}
	return parser.Parse(ctx, content)
}

func failedBatchParseResult(report domain.Report) *domain.ParseResult {
	if len(report.SourceResults) == 0 {
		return nil
	}
	report.Status = "failed"
	return &domain.ParseResult{Report: report}
}
