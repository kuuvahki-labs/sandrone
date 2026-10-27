package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

// Convert composes Parse and Render for the common node conversion flow. Entry
// points should call this instead of piping json-nodes themselves.
func (s *Service) Convert(ctx context.Context, req domain.ConvertRequest) (*domain.RenderResult, error) {
	start := time.Now()
	if _, ok := s.renderers[normalizeFormat(req.ToFormat)]; !ok {
		return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported render format %q", req.ToFormat))
	}
	parsed, err := s.parse(ctx, domain.ParseRequest{
		Format:      req.FromFormat,
		Content:     req.Content,
		Remote:      req.Remote,
		RemoteBatch: req.RemoteBatch,
		Target:      req.ToFormat,
		Processors:  req.ParseProcessors,
		Meta:        req.Meta,
	})
	if err != nil {
		s.log(ctx, slog.LevelError, "service convert failed",
			"operation", "convert",
			"from_format", req.FromFormat,
			"to_format", req.ToFormat,
			"parse_processor_count", len(req.ParseProcessors),
			"render_processor_count", len(req.RenderProcessors),
			"remote", req.Remote != nil,
			"duration_ms", elapsedMillis(start),
			"error", err.Error(),
		)
		if parsed != nil && len(parsed.Report.SourceResults) > 0 {
			report := parsed.Report
			report.Kind, report.Status = "convert", "failed"
			return &domain.RenderResult{Report: s.prepareReport("convert", report)}, err
		}
		return nil, err
	}
	options := req.Options
	if options.Format == "" {
		options.Format = req.ToFormat
	}
	rendered, err := s.render(ctx, domain.RenderRequest{
		Format:     req.ToFormat,
		Target:     req.ToFormat,
		Nodes:      parsed.Nodes,
		Processors: req.RenderProcessors,
		Options:    options,
	})
	if err != nil {
		s.log(ctx, slog.LevelError, "service convert failed",
			"operation", "convert",
			"from_format", req.FromFormat,
			"to_format", req.ToFormat,
			"node_count", len(parsed.Nodes),
			"parse_processor_count", len(req.ParseProcessors),
			"render_processor_count", len(req.RenderProcessors),
			"remote", req.Remote != nil,
			"duration_ms", elapsedMillis(start),
			"error", err.Error(),
		)
		if parsed != nil && len(parsed.Report.SourceResults) > 0 {
			report := parsed.Report
			report.Kind, report.Status = "convert", "failed"
			return &domain.RenderResult{Report: s.prepareReport("convert", report)}, err
		}
		return nil, err
	}
	report := rendered.Report
	report.Kind = "convert"
	report.SourceResults = parsed.Report.SourceResults
	if parsed.Report.Status != "" {
		report.Status = parsed.Report.Status
	}
	report.SourceRefs = append(append([]domain.SourceRef{}, parsed.Report.SourceRefs...), report.SourceRefs...)
	report.Warnings = append(append([]domain.Warning{}, parsed.Report.Warnings...), report.Warnings...)
	report = s.prepareReport("convert", report)
	rendered.Report = report
	s.log(ctx, slog.LevelInfo, "service convert completed",
		"operation", "convert",
		"from_format", req.FromFormat,
		"to_format", req.ToFormat,
		"node_count", len(parsed.Nodes),
		"parse_processor_count", len(req.ParseProcessors),
		"render_processor_count", len(req.RenderProcessors),
		"warning_count", len(rendered.Report.Warnings),
		"remote", req.Remote != nil,
		"duration_ms", elapsedMillis(start),
	)
	return rendered, nil
}

// ConvertPublic runs the processor-free conversion flow and applies the public
// network policy to request-level remote inputs.
func (s *Service) ConvertPublic(ctx context.Context, req domain.ConvertRequest) (*domain.RenderResult, error) {
	if req.RemoteBatch != nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "public convert does not support remote_batch")
	}
	if len(req.ParseProcessors) > 0 || len(req.RenderProcessors) > 0 || len(req.Meta) > 0 || req.Options.Format != "" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "public convert does not support processors, meta, or render options")
	}
	if req.Remote != nil && (req.Remote.UserAgent != "" || req.Remote.Proxy != "" || req.Remote.TimeoutMS != 0 || req.Remote.CacheTTLSeconds != 0) {
		return nil, domain.NewError(domain.CodeInvalidArgument, "public convert remote input only supports url")
	}
	return s.Convert(withPublicRemoteFetch(ctx), req)
}
