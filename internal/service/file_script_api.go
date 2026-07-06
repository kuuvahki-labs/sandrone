package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	scriptproc "github.com/kuuvahki-labs/sandrone/internal/processor/script"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

type fileScriptContextKey struct{}

type fileScriptContext struct {
	service *Service
	req     domain.FileRequest
	state   *fileResolveState
}

func withFileScriptContext(ctx context.Context, svc *Service, req domain.FileRequest, state *fileResolveState) context.Context {
	return context.WithValue(ctx, fileScriptContextKey{}, &fileScriptContext{service: svc, req: req, state: state})
}

func fileScriptContextFrom(ctx context.Context) (*fileScriptContext, bool) {
	value, ok := ctx.Value(fileScriptContextKey{}).(*fileScriptContext)
	return value, ok && value != nil && value.service != nil && value.state != nil
}

func (s *Service) loadScriptSource(source scriptproc.ScriptSource) (string, string, error) {
	switch source.Type {
	case "file":
		return s.loadScriptFileResource(source.Name)
	case "remote":
		return s.loadRemoteScriptSource(source)
	case "inline":
		return source.Content, "<inline>", nil
	default:
		return "", "", domain.NewError(domain.CodeProcessorConfigInvalid, "script source type must be inline, file, or remote")
	}
}

func (s *Service) loadScriptFileResource(path string) (string, string, error) {
	name := strings.TrimSpace(path)
	if name == "" {
		return "", "", domain.NewError(domain.CodeProcessorConfigInvalid, "script file resource name is required")
	}
	if strings.HasPrefix(name, "files/") {
		return "", "", domain.NewError(domain.CodeProcessorConfigInvalid, "script path must be a file resource name, not a raw files/ key")
	}
	if _, err := store.CleanKey(name); err != nil {
		return "", "", domain.WrapError(domain.CodeProcessorConfigInvalid, "invalid script file resource name", err)
	}
	if s.metaStore == nil {
		return "", "", storeUnavailable()
	}
	spec, err := s.metaStore.GetFile(context.Background(), name)
	if err != nil {
		return "", "", err
	}
	switch strings.ToLower(strings.TrimSpace(spec.Source.Type)) {
	case "local":
		body, _, err := s.readLocalFileSource(context.Background(), spec)
		if err != nil {
			return "", "", err
		}
		return string(body), name, nil
	case "inline":
		return spec.Source.Content, name, nil
	case "remote":
		body, _, err := s.loadRemoteScriptSource(scriptproc.ScriptSource{Type: "remote", Remote: spec.Source.Remote})
		if err != nil {
			return "", "", err
		}
		return body, name, nil
	default:
		return "", "", domain.NewError(domain.CodeProcessorConfigInvalid, "script file resource must use a local or remote source")
	}
}

func (s *Service) loadRemoteScriptSource(source scriptproc.ScriptSource) (string, string, error) {
	if source.Remote == nil || strings.TrimSpace(source.Remote.URL) == "" {
		return "", "", domain.NewError(domain.CodeProcessorConfigInvalid, "remote script source requires remote.url")
	}
	if s.fetcher == nil {
		return "", "", domain.NewError(domain.CodeNotImplemented, "remote fetcher is not configured")
	}
	result, err := s.fetchRemoteCached(context.Background(), *source.Remote)
	if err != nil {
		return "", "", err
	}
	expectedHash := strings.ToLower(strings.TrimSpace(source.SHA256))
	if expectedHash != "" && expectedHash != result.ContentHash {
		return "", "", domain.NewError(domain.CodeProcessorConfigInvalid, fmt.Sprintf("remote script sha256 mismatch: expected %s, got %s", expectedHash, result.ContentHash))
	}
	return string(result.Body), source.Remote.URL, nil
}

func fileMemoKey(name string, args map[string]string) string {
	if len(args) == 0 {
		return name
	}
	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(name)
	for _, key := range keys {
		b.WriteByte(0)
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(args[key])
	}
	return b.String()
}

func optionalRequestArgs(argSets ...map[string]string) map[string]string {
	var out map[string]string
	for _, args := range argSets {
		if len(args) == 0 {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		for key, value := range args {
			out[key] = value
		}
	}
	return out
}

func mergeRequestArgs(base domain.RequestInfo, args map[string]string) domain.RequestInfo {
	out := base
	if base.Args != nil || args != nil {
		out.Args = map[string]string{}
		for key, value := range base.Args {
			out.Args[key] = value
		}
		for key, value := range args {
			out.Args[key] = value
		}
	}
	if base.Meta != nil {
		out.Meta = cloneStringMap(base.Meta)
	}
	return out
}

func (s *Service) ProduceSubscription(ctx context.Context, name string, opts domain.ScriptProduceOptions) (*domain.ScriptSubscriptionProduceResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "subscription name is required")
	}
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	req := domain.FileRequest{
		Name:    name,
		Target:  strings.TrimSpace(opts.Target),
		Request: domain.RequestInfo{Args: cloneArgs(opts.Args)},
	}
	if fctx, ok := fileScriptContextFrom(ctx); ok {
		fctx.state.dynamicDeps = appendResourceRef(fctx.state.dynamicDeps, domain.ResourceRef{Kind: "subscription", Name: name})
		req.Request = mergeRequestArgs(fctx.req.Request, opts.Args)
		req.Meta = fctx.req.Meta
	}

	sub, err := s.metaStore.GetSubscription(ctx, name)
	if err != nil {
		return nil, err
	}
	nodeSet, err := s.materializeSubscription(ctx, sub, req, newSubscriptionResolveState())
	if err != nil {
		return nil, err
	}
	report := reportForProducedSubscription(name, nodeSet)
	if req.Target == "" {
		report = s.prepareReport("subscription_produce", report)
		return &domain.ScriptSubscriptionProduceResult{
			Kind:   "nodes",
			Nodes:  append([]domain.NodeIR{}, nodeSet.Nodes...),
			Report: report,
		}, nil
	}
	renderer, ok := s.renderers[normalizeFormat(req.Target)]
	if !ok {
		return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported render target %q", req.Target))
	}
	body, renderReport, err := s.renderWithReport(ctx, renderer, nodeSet.Nodes, domain.RenderOptions{Format: req.Target})
	if err != nil {
		return nil, err
	}
	report.Render = renderReport
	report.Warnings = append(report.Warnings, renderReport.Warnings...)
	report = s.prepareReport("subscription_produce", report)
	return &domain.ScriptSubscriptionProduceResult{
		Kind:    "content",
		Target:  req.Target,
		Content: string(body),
		Report:  report,
	}, nil
}

func (s *Service) FileContent(ctx context.Context, name string, opts domain.ScriptProduceOptions) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", domain.NewError(domain.CodeInvalidArgument, "file name is required")
	}
	fctx, ok := fileScriptContextFrom(ctx)
	if !ok {
		return "", domain.NewError(domain.CodeInvalidArgument, "api.file.content is available only during file rendering")
	}
	fctx.state.dynamicDeps = appendResourceRef(fctx.state.dynamicDeps, domain.ResourceRef{Kind: "file", Name: name})
	result, err := fctx.service.getFile(ctx, domain.FileRequest{
		Name:    name,
		Request: mergeRequestArgs(fctx.req.Request, opts.Args),
		Meta:    fctx.req.Meta,
	}, fctx.state)
	if err != nil {
		return "", err
	}
	return string(result.Content), nil
}

func cloneArgs(args map[string]string) map[string]string {
	if args == nil {
		return nil
	}
	out := make(map[string]string, len(args))
	for key, value := range args {
		out[key] = value
	}
	return out
}

func appendResourceRef(refs []domain.ResourceRef, ref domain.ResourceRef) []domain.ResourceRef {
	if ref.Kind == "" || ref.Name == "" {
		return refs
	}
	for _, existing := range refs {
		if existing == ref {
			return refs
		}
	}
	return append(refs, ref)
}

func reportForProducedSubscription(name string, nodeSet *domain.NodeSet) domain.Report {
	report := domain.Report{
		Dependencies: []domain.ResourceRef{{Kind: "subscription", Name: name}},
	}
	if nodeSet == nil {
		return report
	}
	for _, dep := range nodeSet.Dependencies {
		report.Dependencies = appendResourceRef(report.Dependencies, dep)
	}
	for _, source := range nodeSet.Sources {
		report.SourceRefs = append(report.SourceRefs, source.SourceRefs...)
		report.Warnings = append(report.Warnings, source.Warnings...)
	}
	report.Warnings = append(report.Warnings, nodeSet.Warnings...)
	return report
}
