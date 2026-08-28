package service

import (
	"context"
	"strings"
	"time"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type subscriptionTrafficCacheValue struct {
	EntryID string                           `json:"entry_id"`
	Result  domain.SubscriptionTrafficResult `json:"result"`
}

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
	ctx = withSubscriptionCacheOwner(ctx, sub.Name)
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

	base, err := s.subscriptionBaseNodes(ctx, sub, subscriptionExecutionRequest{Refresh: req.Refresh}, newSubscriptionExecutionState())
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
	return result.Clone(), nil
}

func (s *Service) subscriptionTrafficTTLSeconds() int {
	settings := s.currentSettings()
	return settings.CacheDefaults.SubscriptionTrafficTTLSeconds
}

func (s *Service) readSubscriptionTrafficCache(ctx context.Context, entryID string, ttlSeconds int) *domain.SubscriptionTrafficResult {
	if ttlSeconds <= 0 {
		return nil
	}
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixSubscriptionTraffic)
	if !owned {
		return nil
	}
	c := s.cache
	if c == nil {
		return nil
	}
	item, found := readCacheValue[subscriptionTrafficCacheValue](s, ctx, key, time.Duration(ttlSeconds)*time.Second)
	if !found || item.Value.EntryID != entryID {
		return nil
	}
	return (&item.Value.Result).Clone()
}

func (s *Service) writeSubscriptionTrafficCache(ctx context.Context, entryID string, ttlSeconds int, result *domain.SubscriptionTrafficResult) {
	if ttlSeconds <= 0 || result == nil {
		return
	}
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixSubscriptionTraffic)
	if !owned {
		return
	}
	if s.cache == nil {
		return
	}
	_, remaining, ok := prepareCacheValueWrite[subscriptionTrafficCacheValue](s, ctx, key, time.Duration(ttlSeconds)*time.Second)
	if !ok {
		return
	}
	cloned := result.Clone()
	_ = cachepkg.SetJSON(ctx, s.cache, key, subscriptionTrafficCacheValue{EntryID: entryID, Result: *cloned}, remaining)
}

func subscriptionTrafficCacheEntryID(input domain.RemoteInput) (string, error) {
	return cacheIdentity(struct {
		URL       string `json:"url"`
		UserAgent string `json:"user_agent,omitempty"`
		Proxy     string `json:"proxy,omitempty"`
	}{URL: input.URL, UserAgent: input.UserAgent, Proxy: input.Proxy})
}
