package service

import (
	"context"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type subscriptionBaseNodes struct {
	Nodes        []domain.NodeIR
	Dependencies []domain.ResourceRef
	Sources      []domain.SourceInfo
	Warnings     []domain.Warning
	Traffic      []domain.SubscriptionTrafficItem
	Meta         map[string]string
	Source       *domain.SourceInfo
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

func (s *Service) subscriptionBaseNodes(ctx context.Context, sub domain.Subscription, req subscriptionExecutionRequest, state *subscriptionExecutionState) (*subscriptionBaseNodes, error) {
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
		Traffic:  domain.CloneSubscriptionTrafficItems(traffic),
		Meta:     cloneStringMap(sub.Meta),
		Source:   parsed.Source,
	}, nil
}

func (s *Service) collectionSubscriptionBaseNodes(ctx context.Context, sub domain.Subscription, req subscriptionExecutionRequest, state *subscriptionExecutionState) (*subscriptionBaseNodes, error) {
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
		Traffic:      domain.CloneSubscriptionTrafficItems(traffic),
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
	ctx = withSubscriptionCacheOwner(ctx, sub.Name)
	ttlSeconds := s.subscriptionRenderTTLSeconds(sub.RenderCacheTTLSeconds)
	cacheEntryID := ""
	if ttlSeconds > 0 {
		cacheEntryID, err = s.subscriptionRenderCacheEntryID(sub, format, req)
		if err != nil {
			return nil, err
		}
		if !request.Refresh {
			if cached := s.readSubscriptionRenderCache(ctx, cacheEntryID, time.Duration(ttlSeconds)*time.Second); cached != nil {
				return cached, nil
			}
		}
	}
	execution, err := s.executeSubscription(ctx, sub, subscriptionExecutionRequest{
		Name:    name,
		Request: req,
		Refresh: request.Refresh,
	}, newSubscriptionExecutionState())
	if err != nil {
		return nil, err
	}
	nodeSet := execution.After
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
	s.writeSubscriptionRenderCache(ctx, cacheEntryID, ttlSeconds, rendered)
	return rendered, nil
}

func sourceRefsFromSources(sources []domain.SourceInfo) []domain.SourceRef {
	refs := []domain.SourceRef{}
	for _, source := range sources {
		refs = append(refs, source.SourceRefs...)
	}
	return refs
}

func isSubscriptionCycleError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "subscription dependency cycle")
}
