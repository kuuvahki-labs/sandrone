package service

import (
	"context"
	"strings"
	"time"

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
	if req.Refresh {
		ctx = withCacheReadBypass(ctx)
	}

	sub, err := s.metaStore.GetSubscription(ctx, name)
	if err != nil {
		return nil, err
	}
	ctx = withSubscriptionCacheScope(ctx, sub.Name)
	sub, err = normalizeSubscription(sub)
	if err != nil {
		return nil, err
	}
	if sub.Type != domain.SubscriptionTypeRemote {
		return nil, domain.NewError(domain.CodeInvalidArgument, "subscription traffic requires remote subscription")
	}
	ttlSeconds := s.subscriptionTrafficTTLSeconds()
	entryID, err := subscriptionTrafficCacheEntryID(s.remoteInputWithDefaults(*sub.Remote))
	if err != nil {
		return nil, err
	}
	if !req.Refresh {
		if cached := s.readSubscriptionTrafficCache(ctx, entryID, ttlSeconds); cached != nil {
			cached.Cached = true
			return cached, nil
		}
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
	s.writeSubscriptionTrafficCache(ctx, entryID, ttlSeconds, result)
	return cloneSubscriptionTrafficResult(result), nil
}

func (s *Service) subscriptionTrafficTTLSeconds() int {
	settings := s.currentSettings()
	return settings.CacheDefaults.SubscriptionTrafficTTLSeconds
}

func (s *Service) readSubscriptionTrafficCache(ctx context.Context, entryID string, ttlSeconds int) *domain.SubscriptionTrafficResult {
	if ttlSeconds <= 0 {
		return nil
	}
	key, scoped := persistentCacheKey(ctx, cacheKeyPrefixSubscriptionTraffic)
	if !scoped {
		return nil
	}
	c := s.cache
	if c == nil {
		return nil
	}
	var result domain.SubscriptionTrafficResult
	if !s.readCacheJSON(ctx, key, entryID, time.Duration(ttlSeconds)*time.Second, &result) {
		return nil
	}
	return cloneSubscriptionTrafficValue(result)
}

func (s *Service) writeSubscriptionTrafficCache(ctx context.Context, entryID string, ttlSeconds int, result *domain.SubscriptionTrafficResult) {
	if ttlSeconds <= 0 || result == nil {
		return
	}
	key, scoped := persistentCacheKey(ctx, cacheKeyPrefixSubscriptionTraffic)
	if !scoped {
		return
	}
	c := s.cache
	if c == nil {
		return
	}
	_ = s.writeCacheJSON(ctx, key, entryID, time.Duration(ttlSeconds)*time.Second, cloneSubscriptionTrafficResult(result))
}

func subscriptionTrafficCacheEntryID(input domain.RemoteInput) (string, error) {
	return cacheIdentity(struct {
		URL       string `json:"url"`
		UserAgent string `json:"user_agent,omitempty"`
		Proxy     string `json:"proxy,omitempty"`
	}{URL: input.URL, UserAgent: input.UserAgent, Proxy: input.Proxy})
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
