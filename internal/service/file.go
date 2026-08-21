package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

func (s *Service) ValidateFile(ctx context.Context, req domain.FileRequest) (*domain.ValidateResult, error) {
	state := &fileResolveState{
		stack: map[string]bool{},
		memo:  map[string]*domain.FileResult{},
	}
	result, err := s.getFile(ctx, req, state)
	if err != nil {
		return nil, err
	}
	report := s.prepareReport("validate_file", result.Report)
	return &domain.ValidateResult{OK: true, Report: report}, nil
}

// GetFile runs the file flow:
//
//  1. Read the FileSpec source into a FileDocument.
//  2. Run file-stage processors in declaration order.
//  3. Return the processed file content.
func (s *Service) GetFile(ctx context.Context, req domain.FileRequest) (*domain.FileResult, error) {
	if req.Refresh {
		ctx = withCacheReadBypass(ctx)
	}
	state := &fileResolveState{
		stack: map[string]bool{},
		memo:  map[string]*domain.FileResult{},
	}
	if req.Spec != nil || strings.TrimSpace(req.Name) == "" {
		return s.getFile(ctx, req, state)
	}
	spec, err := s.resolveSpec(ctx, req)
	if err != nil {
		return nil, err
	}
	ttlSeconds := s.fileRenderTTLSeconds(spec.RenderCacheTTLSeconds)
	cacheKey := ""
	if ttlSeconds > 0 {
		cacheKey, err = fileRenderCacheKey(spec, req)
		if err != nil {
			return nil, err
		}
		if !req.Refresh {
			if cached := s.readFileRenderCache(ctx, cacheKey); cached != nil {
				return cached, nil
			}
		}
	}
	req.Spec = &spec
	result, err := s.getFile(ctx, req, state)
	if err != nil {
		return nil, err
	}
	result.Cached = false
	s.writeFileRenderCache(ctx, cacheKey, ttlSeconds, result)
	return result, nil
}

func (s *Service) getFile(ctx context.Context, req domain.FileRequest, state *fileResolveState) (*domain.FileResult, error) {
	spec, err := s.resolveSpec(ctx, req)
	if err != nil {
		return nil, err
	}
	ctx = processor.WithTraceScope(ctx, "file:"+firstNonEmptyString(spec.Name, req.Name, "inline"))
	if spec.Name != "" {
		if state.stack[spec.Name] {
			return nil, &domain.AppError{
				Code:    domain.CodeFileDependencyCycle,
				Message: fmt.Sprintf("file dependency cycle at %q", spec.Name),
				File:    spec.Name,
			}
		}
		if cached, ok := state.memo[fileMemoKey(spec.Name, req.Request.Args)]; ok {
			return cloneFileResult(cached), nil
		}
		state.stack[spec.Name] = true
		defer delete(state.stack, spec.Name)
	}
	ctx = withFileResolutionContext(ctx, req, state)
	report := domain.Report{}
	doc, sourceRef, compileWarnings, err := s.resolveFileDocument(ctx, spec, req, state)
	if err != nil {
		return nil, err
	}
	report.Warnings = append(report.Warnings, compileWarnings...)
	if sourceRef != nil {
		report.SourceRefs = append(report.SourceRefs, *sourceRef)
	}

	ctx = withFileScriptContext(ctx, s, req, state)
	fileOut, err := s.registry.RunFile(ctx, spec.Processors, domain.FileProcessInput{
		Target:  string(spec.Kind),
		File:    doc,
		Request: req.Request,
	})
	if err != nil {
		return nil, err
	}
	doc = fileOut.File
	report.Warnings = append(report.Warnings, fileOut.Warnings...)

	report.Dependencies = append(report.Dependencies, state.dynamicDeps...)
	report = s.prepareReport("file", report)

	result := &domain.FileResult{
		File:        doc,
		Content:     append([]byte{}, doc.Content...),
		ContentType: doc.MediaType,
		Report:      report,
	}
	if spec.Name != "" {
		state.memo[fileMemoKey(spec.Name, req.Request.Args)] = cloneFileResult(result)
	}
	return result, nil
}

func (s *Service) resolveFileDocument(ctx context.Context, spec domain.FileSpec, req domain.FileRequest, state *fileResolveState) (domain.FileDocument, *domain.SourceRef, []domain.Warning, error) {
	switch spec.Kind {
	case domain.FileKindStatic:
		doc, ref, err := s.resolveFileSource(ctx, spec)
		if err != nil {
			return domain.FileDocument{}, nil, nil, err
		}
		doc.Kind = string(domain.FileKindStatic)
		return doc, ref, nil, nil
	default:
		return s.resolveConfigFile(ctx, spec, req, state)
	}
}

func (s *Service) resolveFileSource(ctx context.Context, spec domain.FileSpec) (domain.FileDocument, *domain.SourceRef, error) {
	source := spec.Source
	switch strings.ToLower(strings.TrimSpace(source.Type)) {
	case "inline":
		return domain.FileDocument{
			Name:    spec.Name,
			Content: []byte(source.Content),
			Meta:    cloneStringMap(spec.Meta),
		}, &domain.SourceRef{Kind: "inline", Name: spec.Name}, nil
	case "remote":
		if source.Remote == nil || strings.TrimSpace(source.Remote.URL) == "" {
			return domain.FileDocument{}, nil, domain.NewError(domain.CodeInvalidArgument, "remote file source requires remote.url")
		}
		result, err := s.fetchRemoteCached(ctx, *source.Remote)
		if err != nil {
			return domain.FileDocument{}, nil, err
		}
		return domain.FileDocument{
			Name:    spec.Name,
			Content: append([]byte{}, result.Body...),
			Meta:    cloneStringMap(spec.Meta),
		}, &result.SourceRef, nil
	default:
		return domain.FileDocument{}, nil, domain.NewError(domain.CodeInvalidArgument, "file source type must be inline or remote")
	}
}

func (s *Service) resolveSpec(ctx context.Context, req domain.FileRequest) (domain.FileSpec, error) {
	var spec domain.FileSpec
	if req.Spec != nil {
		spec = *req.Spec
	} else {
		if req.Name == "" {
			return domain.FileSpec{}, domain.NewError(domain.CodeInvalidArgument, "FileRequest.Spec or FileRequest.Name is required")
		}
		if s.metaStore == nil {
			return domain.FileSpec{}, storeUnavailable()
		}
		stored, err := s.metaStore.GetFile(ctx, req.Name)
		if err != nil {
			return domain.FileSpec{}, err
		}
		spec = stored
	}
	if spec.Name == "" {
		spec.Name = req.Name
	}
	if err := s.validateFileSpecStructure(spec); err != nil {
		return domain.FileSpec{}, err
	}
	return spec, nil
}

type fileResolveState struct {
	stack       map[string]bool
	memo        map[string]*domain.FileResult
	dynamicDeps []domain.ResourceRef
}
