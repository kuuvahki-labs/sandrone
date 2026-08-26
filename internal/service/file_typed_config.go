package service

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) resolveConfigFile(ctx context.Context, spec domain.FileSpec, req domain.FileRequest, state *fileResolveState) (domain.FileDocument, *domain.SourceRef, []domain.Warning, error) {
	driver, err := s.typedFiles.Lookup(spec.Kind)
	if err != nil {
		return domain.FileDocument{}, nil, nil, err
	}
	descriptor := driver.Descriptor()
	base := domain.FileDocument{
		Name:      spec.Name,
		Kind:      string(spec.Kind),
		MediaType: descriptor.MediaType,
		Content:   append([]byte(nil), descriptor.DefaultBase...),
		Meta:      cloneStringMap(spec.Meta),
	}
	sourceRef := &domain.SourceRef{Kind: "builtin", Name: string(spec.Kind) + "-default"}
	if strings.TrimSpace(spec.Source.Type) != "" {
		base, sourceRef, err = s.resolveFileSource(ctx, spec)
		if err != nil {
			return domain.FileDocument{}, nil, nil, err
		}
		base.Kind = string(spec.Kind)
		base.MediaType = descriptor.MediaType
	}
	var config domain.FileConfig
	if spec.Config != nil {
		config = *spec.Config
	}
	nodes, err := s.configNodes(ctx, trimStringList(config.Subscriptions), req, state)
	if err != nil {
		return domain.FileDocument{}, nil, nil, err
	}
	rendered, warnings, err := s.renderTypedFileNodes(ctx, descriptor, nodes)
	if err != nil {
		return domain.FileDocument{}, nil, nil, err
	}
	body, err := driver.Compile(ctx, typedFileCompileInput{
		Base:          base.Content,
		RenderedNodes: rendered,
		Settings:      config.Settings,
	})
	if err != nil {
		return domain.FileDocument{}, nil, nil, err
	}
	base.Content = body
	return base, sourceRef, warnings, nil
}

func (s *Service) renderTypedFileNodes(ctx context.Context, descriptor typedFileDescriptor, nodes []domain.NodeIR) ([]byte, []domain.Warning, error) {
	renderer, ok := s.renderers[descriptor.NodeRenderFormat]
	if !ok {
		return nil, nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("file kind %q requires node renderer %q", descriptor.Kind, descriptor.NodeRenderFormat))
	}
	body, report, err := s.renderWithReport(ctx, renderer, nodes, domain.RenderOptions{Format: descriptor.NodeRenderFormat})
	if err != nil {
		return nil, nil, err
	}
	return body, report.Warnings, nil
}

func trimStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (s *Service) configNodes(ctx context.Context, subscriptions []string, req domain.FileRequest, state *fileResolveState) ([]domain.NodeIR, error) {
	if len(subscriptions) == 0 {
		return nil, nil
	}
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	subState := newSubscriptionResolveState()
	nodes := []domain.NodeIR{}
	for _, name := range subscriptions {
		sub, err := s.metaStore.GetSubscription(ctx, name)
		if err != nil {
			return nil, err
		}
		subCtx := withSubscriptionCacheOwner(ctx, sub.Name)
		nodeSet, err := s.materializeSubscription(subCtx, sub, domain.FileRequest{
			Name:    name,
			Target:  req.Target,
			Request: req.Request,
			Meta:    req.Meta,
		}, subState)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, nodeSet.Nodes...)
		if state != nil {
			state.dynamicDeps = appendResourceRef(state.dynamicDeps, domain.ResourceRef{Kind: "subscription", Name: name})
			for _, dep := range nodeSet.Dependencies {
				state.dynamicDeps = appendResourceRef(state.dynamicDeps, dep)
			}
		}
	}
	return nodes, nil
}

func renderedYAMLList(body []byte, key string) ([]any, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, "parse rendered mihomo proxies", err)
	}
	return anyList(doc[key]), nil
}

func namesFromMihomoProxies(proxies []any) []string {
	names := []string{}
	for _, item := range proxies {
		proxy, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := proxy["name"].(string); ok && name != "" {
			names = append(names, name)
		}
	}
	return uniqueStrings(names)
}

func namesFromSingBoxOutbounds(outbounds []any, endpoints []any) []string {
	names := []string{}
	for _, item := range append(outbounds, endpoints...) {
		outbound, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if tag, ok := outbound["tag"].(string); ok && tag != "" {
			names = append(names, tag)
		}
	}
	return uniqueStrings(names)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func anyList(value any) []any {
	if value == nil {
		return []any{}
	}
	list, ok := value.([]any)
	if !ok {
		return []any{}
	}
	return list
}

func mapValue(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if out, ok := value.(map[string]any); ok {
		return out
	}
	return map[string]any{}
}
