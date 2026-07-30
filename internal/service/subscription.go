package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
)

type subscriptionResolveState struct {
	stack map[string]bool
	memo  map[string]*domain.NodeSet
}

type subscriptionBaseNodes struct {
	Nodes        []domain.NodeIR
	Dependencies []domain.ResourceRef
	Sources      []domain.SourceInfo
	Warnings     []domain.Warning
	Traffic      []domain.SubscriptionTrafficItem
	Meta         map[string]string
	Source       *domain.SourceInfo
}

func newSubscriptionResolveState() *subscriptionResolveState {
	return &subscriptionResolveState{
		stack: map[string]bool{},
		memo:  map[string]*domain.NodeSet{},
	}
}

func normalizeSubscription(sub domain.Subscription) (domain.Subscription, error) {
	if sub.RenderCacheTTLSeconds != nil && *sub.RenderCacheTTLSeconds < 0 {
		return domain.Subscription{}, domain.NewError(domain.CodeInvalidArgument, "subscription render_cache_ttl_seconds must be non-negative")
	}
	sub.Type = domain.SubscriptionType(strings.ToLower(strings.TrimSpace(string(sub.Type))))
	switch sub.Type {
	case domain.SubscriptionTypeRemote:
		if sub.Remote == nil || strings.TrimSpace(sub.Remote.URL) == "" {
			return domain.Subscription{}, domain.NewError(domain.CodeInvalidArgument, "remote subscription requires remote.url")
		}
		sub.Content = ""
		sub.Inputs = nil
	case domain.SubscriptionTypeLocal:
		sub.Remote = nil
		sub.Inputs = nil
	case domain.SubscriptionTypeCollection:
		sub.Remote = nil
		sub.Content = ""
		sub.Format = ""
	default:
		return domain.Subscription{}, domain.NewError(domain.CodeInvalidArgument, "subscription type must be remote, local, or collection")
	}
	return sub, nil
}

func (s *Service) materializeSubscription(ctx context.Context, sub domain.Subscription, req domain.FileRequest, state *subscriptionResolveState) (*domain.NodeSet, error) {
	if state == nil {
		state = newSubscriptionResolveState()
	}
	normalized, err := normalizeSubscription(sub)
	if err != nil {
		return nil, err
	}
	sub = normalized
	if sub.Name != "" {
		if state.stack[sub.Name] {
			return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("subscription dependency cycle at %q", sub.Name))
		}
		if cached, ok := state.memo[sub.Name]; ok {
			return cloneNodeSet(cached), nil
		}
		state.stack[sub.Name] = true
		defer delete(state.stack, sub.Name)
	}
	base, err := s.subscriptionBaseNodes(ctx, sub, req, state)
	if err != nil {
		return nil, err
	}
	validatedBase, baseValidationWarnings, err := validateNodeBatch(base.Nodes, nodevalidation.StageNormalized, req.Target)
	if err != nil {
		return nil, err
	}
	base.Nodes = validatedBase.Nodes
	base.Warnings = append(base.Warnings, baseValidationWarnings...)
	processed, err := s.registry.RunNodes(ctx, sub.Processors, domain.NodeProcessInput{
		Target: req.Target,
		Nodes:  append([]domain.NodeIR{}, base.Nodes...),
		Context: domain.NodeContext{
			InputName:    firstNonEmptyString(req.Name, sub.Name),
			Dependencies: append([]domain.ResourceRef{}, base.Dependencies...),
			Sources:      append([]domain.SourceInfo{}, base.Sources...),
			Meta:         cloneStringMap(base.Meta),
		},
		Request: req.Request,
	})
	if err != nil {
		return nil, err
	}
	validatedProcessed, processedValidationWarnings, err := validateNodeBatch(processed.Nodes, nodevalidation.StageProcessed, req.Target)
	if err != nil {
		return nil, err
	}
	processed.Nodes = validatedProcessed.Nodes
	processed.Warnings = append(processed.Warnings, processedValidationWarnings...)
	out := &domain.NodeSet{
		Nodes:        append([]domain.NodeIR{}, processed.Nodes...),
		Dependencies: append([]domain.ResourceRef{}, base.Dependencies...),
		Sources:      append([]domain.SourceInfo{}, base.Sources...),
		Warnings:     append(append([]domain.Warning{}, base.Warnings...), processed.Warnings...),
		Traffic:      cloneSubscriptionTrafficItems(base.Traffic),
		Meta:         cloneStringMap(base.Meta),
	}
	if sub.Name != "" {
		state.memo[sub.Name] = cloneNodeSet(out)
	}
	return out, nil
}

func (s *Service) subscriptionPreviewNodes(ctx context.Context, sub domain.Subscription, req domain.FileRequest, state *subscriptionResolveState) (*domain.NodeSet, *domain.NodeSet, error) {
	if state == nil {
		state = newSubscriptionResolveState()
	}
	normalized, err := normalizeSubscription(sub)
	if err != nil {
		return nil, nil, err
	}
	sub = normalized
	if sub.Name != "" {
		if state.stack[sub.Name] {
			return nil, nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("subscription dependency cycle at %q", sub.Name))
		}
		state.stack[sub.Name] = true
		defer delete(state.stack, sub.Name)
	}
	base, err := s.subscriptionBaseNodes(ctx, sub, req, state)
	if err != nil {
		return nil, nil, err
	}
	validatedBase, baseValidationWarnings, err := validateNodeBatch(base.Nodes, nodevalidation.StageNormalized, req.Target)
	if err != nil {
		return nil, nil, err
	}
	base.Nodes = validatedBase.Nodes
	base.Warnings = append(base.Warnings, baseValidationWarnings...)
	before := &domain.NodeSet{
		Nodes:        append([]domain.NodeIR{}, base.Nodes...),
		Dependencies: append([]domain.ResourceRef{}, base.Dependencies...),
		Sources:      append([]domain.SourceInfo{}, base.Sources...),
		Warnings:     append([]domain.Warning{}, base.Warnings...),
		Traffic:      cloneSubscriptionTrafficItems(base.Traffic),
		Meta:         cloneStringMap(base.Meta),
	}
	processed, err := s.registry.RunNodes(ctx, sub.Processors, domain.NodeProcessInput{
		Target: req.Target,
		Nodes:  append([]domain.NodeIR{}, before.Nodes...),
		Context: domain.NodeContext{
			InputName:    firstNonEmptyString(req.Name, sub.Name),
			Dependencies: append([]domain.ResourceRef{}, before.Dependencies...),
			Sources:      append([]domain.SourceInfo{}, before.Sources...),
			Meta:         cloneStringMap(before.Meta),
		},
		Request: req.Request,
	})
	if err != nil {
		return nil, nil, err
	}
	validatedProcessed, processedValidationWarnings, err := validateNodeBatch(processed.Nodes, nodevalidation.StageProcessed, req.Target)
	if err != nil {
		return nil, nil, err
	}
	processed.Nodes = validatedProcessed.Nodes
	processed.Warnings = append(processed.Warnings, processedValidationWarnings...)
	after := &domain.NodeSet{
		Nodes:        append([]domain.NodeIR{}, processed.Nodes...),
		Dependencies: append([]domain.ResourceRef{}, before.Dependencies...),
		Sources:      append([]domain.SourceInfo{}, before.Sources...),
		Warnings:     append(append([]domain.Warning{}, before.Warnings...), processed.Warnings...),
		Traffic:      cloneSubscriptionTrafficItems(before.Traffic),
		Meta:         cloneStringMap(before.Meta),
	}
	return before, after, nil
}

func (s *Service) subscriptionBaseNodes(ctx context.Context, sub domain.Subscription, req domain.FileRequest, state *subscriptionResolveState) (*subscriptionBaseNodes, error) {
	switch sub.Type {
	case domain.SubscriptionTypeRemote, domain.SubscriptionTypeLocal:
		return s.parseSubscriptionBaseNodes(ctx, sub)
	case domain.SubscriptionTypeCollection:
		return s.collectionSubscriptionBaseNodes(ctx, sub, req, state)
	default:
		return nil, domain.NewError(domain.CodeInvalidArgument, "subscription type must be remote, local, or collection")
	}
}

func (s *Service) parseSubscriptionBaseNodes(ctx context.Context, sub domain.Subscription) (*subscriptionBaseNodes, error) {
	var (
		parsed         *parseInputResult
		traffic        []domain.SubscriptionTrafficItem
		trafficWarning []domain.Warning
		err            error
	)
	if sub.Type == domain.SubscriptionTypeRemote {
		remoteInput, fetchErr := s.fetchRemoteInput(ctx, *sub.Remote)
		if fetchErr != nil {
			return nil, fetchErr
		}
		traffic, trafficWarning = s.subscriptionTrafficFromRemote(sub, remoteInput)
		parsed, err = s.parseNodeContent(ctx, sub.Format, remoteInput.Body, true, &remoteInput.SourceRef)
	} else {
		parsed, err = s.parseNodeContent(ctx, sub.Format, []byte(sub.Content), isAutoNodeFormat(sub.Format), nil)
	}
	if err != nil {
		return nil, err
	}
	if parsed == nil {
		return nil, domain.NewError(domain.CodeParseFailed, "subscription parser returned no result")
	}
	warnings := []domain.Warning{}
	if parsed.Source != nil {
		warnings = append(warnings, parsed.Source.Warnings...)
	}
	for _, node := range parsed.Nodes {
		warnings = append(warnings, node.Warnings...)
	}
	warnings = append(warnings, trafficWarning...)
	return &subscriptionBaseNodes{
		Nodes:    append([]domain.NodeIR{}, parsed.Nodes...),
		Sources:  sourcesSlice(parsed.Source),
		Warnings: warnings,
		Traffic:  cloneSubscriptionTrafficItems(traffic),
		Meta:     cloneStringMap(sub.Meta),
		Source:   parsed.Source,
	}, nil
}

func (s *Service) collectionSubscriptionBaseNodes(ctx context.Context, sub domain.Subscription, req domain.FileRequest, state *subscriptionResolveState) (*subscriptionBaseNodes, error) {
	if len(sub.Inputs) == 0 {
		if len(sub.Nodes) > 0 {
			return &subscriptionBaseNodes{
				Nodes: append([]domain.NodeIR{}, sub.Nodes...),
				Meta:  cloneStringMap(sub.Meta),
			}, nil
		}
		return nil, domain.NewError(domain.CodeInvalidArgument, "collection subscription requires at least one input")
	}
	nodes := []domain.NodeIR{}
	deps := []domain.ResourceRef{}
	sources := []domain.SourceInfo{}
	warnings := []domain.Warning{}
	traffic := []domain.SubscriptionTrafficItem{}
	meta := cloneStringMap(sub.Meta)
	for _, input := range sub.Inputs {
		nodeSet, err := s.resolveNodeInputWithSubscriptionState(ctx, input, req, state)
		if err != nil {
			if input.Required || isSubscriptionCycleError(err) {
				return nil, err
			}
			warnings = append(warnings, warningForNodeInputError(input, err))
			continue
		}
		if nodeSet == nil {
			continue
		}
		nodes = append(nodes, nodeSet.Nodes...)
		deps = append(deps, nodeSet.Dependencies...)
		sources = append(sources, nodeSet.Sources...)
		warnings = append(warnings, nodeSet.Warnings...)
		traffic = append(traffic, nodeSet.Traffic...)
		meta = mergeStringMaps(meta, nodeSet.Meta)
	}
	return &subscriptionBaseNodes{
		Nodes:        nodes,
		Dependencies: deps,
		Sources:      sources,
		Warnings:     warnings,
		Traffic:      cloneSubscriptionTrafficItems(traffic),
		Meta:         meta,
	}, nil
}

func (s *Service) RenderSubscription(ctx context.Context, name string, format string, req domain.RequestInfo) (*domain.RenderResult, error) {
	return s.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{
		Name: name, Format: format, Request: req,
	})
}

func (s *Service) RenderSubscriptionRequest(ctx context.Context, request domain.SubscriptionRenderRequest) (*domain.RenderResult, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	if request.Refresh {
		ctx = withCacheReadBypass(ctx)
	}
	name := request.Name
	format := request.Format
	req := request.Request
	format = strings.TrimSpace(format)
	if format == "" {
		format = "uri-list"
	}
	name = strings.TrimSpace(name)
	sub, err := s.metaStore.GetSubscription(ctx, name)
	if err != nil {
		return nil, err
	}
	ttlSeconds := s.subscriptionRenderTTLSeconds(sub.RenderCacheTTLSeconds)
	cacheKey := ""
	if ttlSeconds > 0 {
		cacheKey, err = subscriptionRenderCacheKey(sub, format, req)
		if err != nil {
			return nil, err
		}
		if !request.Refresh {
			if cached := s.readSubscriptionRenderCache(ctx, cacheKey); cached != nil {
				return cached, nil
			}
		}
	}
	nodeSet, err := s.materializeSubscription(ctx, sub, domain.FileRequest{
		Name:    name,
		Target:  format,
		Request: req,
	}, newSubscriptionResolveState())
	if err != nil {
		return nil, err
	}
	rendered, err := s.Render(ctx, domain.RenderRequest{
		Format:  format,
		Target:  format,
		Nodes:   append([]domain.NodeIR{}, nodeSet.Nodes...),
		Options: domain.RenderOptions{Format: format},
	})
	if err != nil {
		return nil, err
	}
	report := rendered.Report
	report.Kind = "subscription_render"
	report.Dependencies = append([]domain.ResourceRef{{Kind: "subscription", Name: name}}, nodeSet.Dependencies...)
	report.SourceRefs = append(report.SourceRefs, sourceRefsFromSources(nodeSet.Sources)...)
	report.Warnings = append(append([]domain.Warning{}, nodeSet.Warnings...), report.Warnings...)
	report = s.prepareReport("subscription_render", report)
	rendered.Report = report
	rendered.Cached = false
	s.writeSubscriptionRenderCache(ctx, cacheKey, ttlSeconds, rendered)
	return rendered, nil
}

func sourceRefsFromSources(sources []domain.SourceInfo) []domain.SourceRef {
	refs := []domain.SourceRef{}
	for _, source := range sources {
		refs = append(refs, source.SourceRefs...)
	}
	return refs
}

func cloneNodeSet(set *domain.NodeSet) *domain.NodeSet {
	if set == nil {
		return nil
	}
	out := *set
	out.Nodes = append([]domain.NodeIR{}, set.Nodes...)
	out.Dependencies = append([]domain.ResourceRef{}, set.Dependencies...)
	out.Sources = append([]domain.SourceInfo{}, set.Sources...)
	out.Warnings = append([]domain.Warning{}, set.Warnings...)
	out.Traffic = cloneSubscriptionTrafficItems(set.Traffic)
	out.Meta = cloneStringMap(set.Meta)
	return &out
}

func isSubscriptionCycleError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "subscription dependency cycle")
}
