package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
)

// Render runs the request's node-stage processors (if any) and then the
// adapter's renderer for the requested format.
func (s *Service) Render(ctx context.Context, req domain.RenderRequest) (*domain.RenderResult, error) {
	start := time.Now()
	result, err := s.render(ctx, req)
	if err != nil {
		s.log(ctx, slog.LevelError, "service render failed",
			"operation", "render",
			"format", req.Format,
			"target", req.Target,
			"node_count", len(req.Nodes),
			"processor_count", len(req.Processors),
			"duration_ms", elapsedMillis(start),
			"error", err.Error(),
		)
		return nil, err
	}
	s.log(ctx, slog.LevelInfo, "service render completed",
		"operation", "render",
		"format", req.Format,
		"target", req.Target,
		"node_count", len(req.Nodes),
		"rendered_count", result.Report.Render.SuccessCount,
		"processor_count", len(req.Processors),
		"warning_count", len(result.Report.Warnings),
		"duration_ms", elapsedMillis(start),
	)
	return result, nil
}

func (s *Service) render(ctx context.Context, req domain.RenderRequest) (*domain.RenderResult, error) {
	renderer, ok := s.renderers[normalizeFormat(req.Format)]
	if !ok {
		return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported render format %q", req.Format))
	}
	nodes := req.Nodes
	warnings := []domain.Warning{}
	validated, validationWarnings, err := validateNodeBatch(nodes, nodevalidation.StageRender, req.Target)
	if err != nil {
		return nil, err
	}
	nodes = validated.Nodes
	warnings = append(warnings, validationWarnings...)
	if len(req.Processors) > 0 {
		processed, err := s.registry.RunNodes(ctx, req.Processors, domain.NodeProcessInput{
			Target: req.Target,
			Nodes:  nodes,
		})
		if err != nil {
			return nil, err
		}
		nodes = processed.Nodes
		warnings = append(warnings, processed.Warnings...)
		validated, validationWarnings, err = validateNodeBatch(nodes, nodevalidation.StageProcessed, req.Target)
		if err != nil {
			return nil, err
		}
		nodes = validated.Nodes
		warnings = append(warnings, validationWarnings...)
	}
	body, renderReport, err := s.renderWithReport(ctx, renderer, nodes, req.Options)
	if err != nil {
		return nil, err
	}
	report := domain.Report{Render: renderReport, Warnings: append(warnings, renderReport.Warnings...)}
	report = s.prepareReport("render", report)
	result := &domain.RenderResult{
		ContentType: contentTypeFor(req.Format),
		Body:        body,
		Report:      report,
	}
	return result, nil
}

func (s *Service) renderWithReport(ctx context.Context, renderer Renderer, nodes []domain.NodeIR, opt domain.RenderOptions) ([]byte, domain.RenderReport, error) {
	if rr, ok := renderer.(reportingRenderer); ok {
		return rr.RenderWithReport(ctx, nodes, opt)
	}
	body, err := renderer.Render(ctx, nodes, opt)
	if err != nil {
		return nil, domain.RenderReport{}, err
	}
	return body, domain.RenderReport{SuccessCount: len(nodes)}, nil
}
