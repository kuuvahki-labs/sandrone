package service

import (
	"context"
	"encoding/json"
	"time"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const (
	cacheKeyPrefixSubscriptionSnapshot = "subscription_snapshot"
	maxSubscriptionSnapshotCacheBytes  = 16 << 20

	snapshotCacheStatusDisabled = "disabled"
	snapshotCacheStatusHit      = "hit"
	snapshotCacheStatusMiss     = "miss"
	snapshotCacheStatusBypass   = "bypass"
)

type cachedSubscriptionSnapshot struct {
	Before           domain.NodeSet            `json:"before"`
	BeforeRuntimeIDs []string                  `json:"before_runtime_ids"`
	After            domain.NodeSet            `json:"after"`
	AfterRuntimeIDs  []string                  `json:"after_runtime_ids"`
	Dependencies     []cacheDependencyRevision `json:"dependencies,omitempty"`
}

type subscriptionSnapshotCacheValue struct {
	Results map[string]cachedSubscriptionSnapshot `json:"results"`
}

func (s *Service) subscriptionSnapshotTTLSeconds(override *int) int {
	if override != nil {
		return *override
	}
	return s.currentSettings().CacheDefaults.SubscriptionSnapshotTTLSeconds
}

func (s *Service) subscriptionSnapshotCacheEntryID(sub domain.Subscription, req subscriptionExecutionRequest) (string, error) {
	return cacheIdentity(struct {
		Build        cacheBuild             `json:"build"`
		Settings     cacheExecutionSettings `json:"settings"`
		Subscription domain.Subscription    `json:"subscription"`
		InputName    string                 `json:"input_name,omitempty"`
		Request      domain.RequestInfo     `json:"request,omitempty"`
	}{
		Build: currentCacheBuild(), Settings: s.cacheExecutionSettings(),
		Subscription: sub, InputName: req.Name, Request: req.Request,
	})
}

func (s *Service) readSubscriptionSnapshotCache(ctx context.Context, entryID string, ttl time.Duration) *subscriptionExecutionResult {
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixSubscriptionSnapshot)
	if s.cache == nil || entryID == "" || !owned {
		return nil
	}
	item, found := readCacheValue[subscriptionSnapshotCacheValue](s, ctx, key, ttl)
	if !found {
		return nil
	}
	cached, found := item.Value.Results[entryID]
	if !found || !s.cacheDependenciesCurrent(ctx, cached.Dependencies) {
		return nil
	}
	before := cached.Before.Clone()
	after := cached.After.Clone()
	if !restoreNodeRuntimeIDs(before.Nodes, cached.BeforeRuntimeIDs) || !restoreNodeRuntimeIDs(after.Nodes, cached.AfterRuntimeIDs) {
		return nil
	}
	return &subscriptionExecutionResult{
		Before:              before,
		After:               after,
		snapshotCacheStatus: snapshotCacheStatusHit,
	}
}

func (s *Service) writeSubscriptionSnapshotCache(
	ctx context.Context,
	subscriptionName string,
	entryID string,
	ttlSeconds int,
	result *subscriptionExecutionResult,
) {
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixSubscriptionSnapshot)
	if s.cache == nil || entryID == "" || ttlSeconds <= 0 || result == nil || result.Before == nil || result.After == nil || !owned {
		return
	}
	refs := make([]domain.ResourceRef, 0, 1+len(result.Before.Dependencies)+len(result.After.Dependencies))
	refs = append(refs, domain.ResourceRef{Kind: "subscription", Name: subscriptionName})
	refs = append(refs, result.Before.Dependencies...)
	refs = append(refs, result.After.Dependencies...)
	dependencies, err := s.snapshotCacheDependencies(ctx, refs)
	if err != nil {
		return
	}
	cached := cachedSubscriptionSnapshot{
		Before:           *result.Before.Clone(),
		BeforeRuntimeIDs: nodeRuntimeIDs(result.Before.Nodes),
		After:            *result.After.Clone(),
		AfterRuntimeIDs:  nodeRuntimeIDs(result.After.Nodes),
		Dependencies:     dependencies,
	}
	body, err := json.Marshal(cached)
	if err != nil || len(body) > maxSubscriptionSnapshotCacheBytes {
		return
	}
	ttl := time.Duration(ttlSeconds) * time.Second
	value, remaining, ok := prepareCacheValueWrite[subscriptionSnapshotCacheValue](s, ctx, key, ttl)
	if !ok {
		return
	}
	if value.Results == nil {
		value.Results = map[string]cachedSubscriptionSnapshot{}
	}
	value.Results[entryID] = cached
	_ = cachepkg.SetJSON(ctx, s.cache, key, value, remaining)
}

func nodeRuntimeIDs(nodes []domain.NodeIR) []string {
	ids := make([]string, len(nodes))
	for index, node := range nodes {
		ids[index] = domain.NodeRuntimeID(node)
	}
	return ids
}

func restoreNodeRuntimeIDs(nodes []domain.NodeIR, ids []string) bool {
	if len(nodes) != len(ids) {
		return false
	}
	seen := make(map[string]struct{}, len(ids))
	for index, id := range ids {
		if id == "" {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
		domain.SetNodeRuntimeID(&nodes[index], id)
	}
	return true
}
