// Package processor registers and runs Sandrone processing stages.
package processor

import (
	"context"
	"errors"
	"fmt"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

// SelectSpecs returns the subset of specs that belong to stage, preserving
// the original order. Specs with empty stage are routed using registry.ResolveStage.
func (r *Registry) SelectSpecs(specs []domain.ProcessorSpec, stage domain.Stage) ([]domain.ProcessorSpec, error) {
	out := make([]domain.ProcessorSpec, 0, len(specs))
	for _, spec := range specs {
		resolved, err := r.ResolveStage(spec)
		if err != nil {
			return nil, err
		}
		if resolved == stage {
			s := spec
			s.Stage = resolved
			out = append(out, s)
		}
	}
	return out, nil
}

// RunNodes constructs and executes the node processor chain.
// Specs unrelated to the nodes stage are skipped.
func (r *Registry) RunNodes(ctx context.Context, specs []domain.ProcessorSpec, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	stageSpecs, err := r.SelectSpecs(specs, domain.StageNodes)
	if err != nil {
		return domain.NodeProcessOutput{Nodes: in.Nodes}, err
	}
	current := domain.NodeProcessOutput{Nodes: cloneNodes(in.Nodes)}
	for _, spec := range stageSpecs {
		proc, err := r.BuildNode(spec)
		if err != nil {
			return current, err
		}
		stageIn := in
		stageIn.Nodes = current.Nodes
		out, err := proc.ApplyNodes(ctx, stageIn)
		if err != nil {
			return current, wrapStageErr(domain.CodeNodeProcessorFailed, proc.Name(), err)
		}
		current.Nodes = out.Nodes
		current.Warnings = append(current.Warnings, out.Warnings...)
	}
	return current, nil
}

// RunFile constructs and executes the file processor chain.
func (r *Registry) RunFile(ctx context.Context, specs []domain.ProcessorSpec, in domain.FileProcessInput) (domain.FileProcessOutput, error) {
	stageSpecs, err := r.SelectSpecs(specs, domain.StageFile)
	if err != nil {
		return domain.FileProcessOutput{File: in.File}, err
	}
	current := domain.FileProcessOutput{File: cloneFile(in.File)}
	parts := cloneParts(in.Parts)
	for _, spec := range stageSpecs {
		proc, err := r.BuildFile(spec)
		if err != nil {
			return current, err
		}
		stageIn := in
		stageIn.File = current.File
		stageIn.Parts = parts
		out, err := proc.ApplyFile(ctx, stageIn)
		if err != nil {
			return current, wrapStageErr(domain.CodeFileProcessorFailed, proc.Name(), err)
		}
		current.File = out.File
		current.Warnings = append(current.Warnings, out.Warnings...)
	}
	return current, nil
}

func wrapStageErr(code domain.ErrorCode, processor string, err error) error {
	if err == nil {
		return nil
	}
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		if appErr.Processor == "" {
			appErr.Processor = processor
		}
		return appErr
	}
	return &domain.AppError{
		Code:      code,
		Message:   fmt.Sprintf("%s failed", processor),
		Processor: processor,
		Cause:     err,
	}
}

func cloneNodes(nodes []domain.NodeIR) []domain.NodeIR {
	out := make([]domain.NodeIR, len(nodes))
	copy(out, nodes)
	return out
}

func cloneFile(doc domain.FileDocument) domain.FileDocument {
	out := doc
	if doc.Content != nil {
		out.Content = append([]byte{}, doc.Content...)
	}
	if doc.Parts != nil {
		out.Parts = cloneParts(doc.Parts)
	}
	if doc.Meta != nil {
		m := make(map[string]string, len(doc.Meta))
		for k, v := range doc.Meta {
			m[k] = v
		}
		out.Meta = m
	}
	if doc.Warnings != nil {
		out.Warnings = append([]domain.Warning{}, doc.Warnings...)
	}
	return out
}

func cloneParts(parts []domain.FilePart) []domain.FilePart {
	if parts == nil {
		return nil
	}
	out := make([]domain.FilePart, len(parts))
	for i, p := range parts {
		clone := p
		if p.Content != nil {
			clone.Content = append([]byte{}, p.Content...)
		}
		if p.Nodes != nil {
			clone.Nodes = append([]domain.NodeIR{}, p.Nodes...)
		}
		out[i] = clone
	}
	return out
}
