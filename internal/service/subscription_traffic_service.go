package service

import (
	"context"
	"strings"
	"time"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) SubscriptionTraffic(ctx context.Context, req domain.SubscriptionTrafficRequest) (*domain.SubscriptionTrafficResult, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "subscription name is required")
	}
	ttlSeconds, err := s.subscriptionTrafficTTLSeconds(ctx)
	if err != nil {
		return nil, err
	}
	if !req.Refresh {
		if cached := s.readSubscriptionTrafficCache(ctx, name, ttlSeconds); cached != nil {
			cached.Cached = true
			return cached, nil
		}
	}
	if req.Refresh {
		ctx = withCacheReadBypass(ctx)
	}

	sub, err := s.metaStore.GetSubscription(ctx, name)
	if err != nil {
		return nil, err
	}
	sub, err = normalizeSubscription(sub)
	if err != nil {
		return nil, err
	}
	if sub.Type != domain.SubscriptionTypeRemote {
		return nil, domain.NewError(domain.CodeInvalidArgument, "subscription traffic requires remote subscription")
	}

	base, err := s.subscriptionBaseNodes(ctx, sub, domain.FileRequest{}, newSubscriptionResolveState())
	if err != nil {
		return nil, err
	}
	result := &domain.SubscriptionTrafficResult{
		SubscriptionName: sub.Name,
		Type:             sub.Type,
		Format:           sub.Format,
		Traffic:          subscriptionTrafficItem(base.Traffic),
	}
	s.writeSubscriptionTrafficCache(ctx, name, ttlSeconds, result)
	return cloneSubscriptionTrafficResult(result), nil
}

func (s *Service) subscriptionTrafficTTLSeconds(ctx context.Context) (int, error) {
	settings, err := s.EffectiveRuntimeSettings(ctx)
	if err != nil {
		return 0, err
	}
	return settings.CacheDefaults.SubscriptionTrafficTTLSeconds, nil
}

func (s *Service) readSubscriptionTrafficCache(ctx context.Context, name string, ttlSeconds int) *domain.SubscriptionTrafficResult {
	if ttlSeconds <= 0 {
		return nil
	}
	key, err := subscriptionTrafficCacheKey(name)
	if err != nil {
		return nil
	}
	c := s.cache
	if c == nil {
		return nil
	}
	var result domain.SubscriptionTrafficResult
	if !c.GetJSON(ctx, key, &result) {
		return nil
	}
	return cloneSubscriptionTrafficValue(result)
}

func (s *Service) writeSubscriptionTrafficCache(ctx context.Context, name string, ttlSeconds int, result *domain.SubscriptionTrafficResult) {
	if ttlSeconds <= 0 || result == nil {
		return
	}
	key, err := subscriptionTrafficCacheKey(name)
	if err != nil {
		return
	}
	c := s.cache
	if c == nil {
		return
	}
	_ = c.PutJSON(ctx, key, time.Duration(ttlSeconds)*time.Second, cloneSubscriptionTrafficResult(result))
}

func (s *Service) invalidateSubscriptionTrafficCache(ctx context.Context) {
	c := s.cache
	if c == nil {
		return
	}
	_ = c.DeleteLayer(ctx, cacheLayerSubscriptionTraffic)
}

func subscriptionTrafficCacheKey(name string) (string, error) {
	return cachepkg.HashKey(cacheLayerSubscriptionTraffic, strings.TrimSpace(name))
}

func cloneSubscriptionTrafficResult(result *domain.SubscriptionTrafficResult) *domain.SubscriptionTrafficResult {
	if result == nil {
		return nil
	}
	return cloneSubscriptionTrafficValue(*result)
}

func cloneSubscriptionTrafficValue(result domain.SubscriptionTrafficResult) *domain.SubscriptionTrafficResult {
	out := result
	out.Traffic = cloneSubscriptionTrafficItem(result.Traffic)
	return &out
}
