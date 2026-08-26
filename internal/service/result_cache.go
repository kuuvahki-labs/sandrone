package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const (
	cacheKeyPrefixSubscriptionRender = "subscription_render"
	cacheKeyPrefixFileRender         = "file_render"
	maxResultCacheBodyBytes          = 16 << 20
)

type resultCacheBuild struct {
	Version  string `json:"version"`
	Revision string `json:"revision,omitempty"`
}

type resultCacheExecutionSettings struct {
	Remote domain.RemoteDefaults `json:"remote"`
	Probe  domain.ProbeDefaults  `json:"probe"`
	Script domain.ScriptDefaults `json:"script"`
}

type cacheDependencyRevision struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Revision string `json:"revision"`
}

type cachedRenderResult struct {
	Result       domain.RenderResult       `json:"result"`
	Dependencies []cacheDependencyRevision `json:"dependencies,omitempty"`
}

type cachedFileResult struct {
	Result       domain.FileResult         `json:"result"`
	Dependencies []cacheDependencyRevision `json:"dependencies,omitempty"`
}

type subscriptionRenderCacheValue struct {
	Results map[string]cachedRenderResult `json:"results"`
}

type fileRenderCacheValue struct {
	Results map[string]cachedFileResult `json:"results"`
}

func currentResultCacheBuild() resultCacheBuild {
	return resultCacheBuild{Version: buildinfo.Version(), Revision: buildinfo.Revision()}
}

func (s *Service) resultCacheExecutionSettings() resultCacheExecutionSettings {
	settings := s.currentSettings()
	return resultCacheExecutionSettings{
		Remote: settings.RemoteDefaults,
		Probe:  settings.ProbeDefaults,
		Script: settings.ScriptDefaults,
	}
}

func (s *Service) subscriptionRenderTTLSeconds(override *int) int {
	if override != nil {
		return *override
	}
	settings := s.currentSettings()
	return settings.CacheDefaults.SubscriptionRenderTTLSeconds
}

func (s *Service) fileRenderTTLSeconds(override *int) int {
	if override != nil {
		return *override
	}
	settings := s.currentSettings()
	return settings.CacheDefaults.FileRenderTTLSeconds
}

func (s *Service) subscriptionRenderCacheEntryID(sub domain.Subscription, format string, req domain.RequestInfo) (string, error) {
	return cacheIdentity(struct {
		Build        resultCacheBuild             `json:"build"`
		Settings     resultCacheExecutionSettings `json:"settings"`
		Subscription domain.Subscription          `json:"subscription"`
		Format       string                       `json:"format"`
		Request      domain.RequestInfo           `json:"request,omitempty"`
	}{
		Build: currentResultCacheBuild(), Settings: s.resultCacheExecutionSettings(),
		Subscription: sub, Format: format, Request: req,
	})
}

func (s *Service) fileRenderCacheEntryID(spec domain.FileSpec, req domain.FileRequest) (string, error) {
	return cacheIdentity(struct {
		Build    resultCacheBuild             `json:"build"`
		Settings resultCacheExecutionSettings `json:"settings"`
		File     domain.FileSpec              `json:"file"`
		Target   string                       `json:"target,omitempty"`
		Request  domain.RequestInfo           `json:"request,omitempty"`
		Meta     map[string]string            `json:"meta,omitempty"`
	}{
		Build: currentResultCacheBuild(), Settings: s.resultCacheExecutionSettings(),
		File: spec, Target: req.Target, Request: req.Request, Meta: req.Meta,
	})
}

func (s *Service) readSubscriptionRenderCache(ctx context.Context, entryID string, ttl time.Duration) *domain.RenderResult {
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixSubscriptionRender)
	if s.cache == nil || entryID == "" || !owned {
		return nil
	}
	item, found := readCacheValue[subscriptionRenderCacheValue](s, ctx, key, ttl)
	if !found {
		return nil
	}
	cached, found := item.Value.Results[entryID]
	if !found || !s.cacheDependenciesCurrent(ctx, cached.Dependencies) {
		return nil
	}
	out := cached.Result.Clone()
	out.Cached = true
	return out
}

func (s *Service) writeSubscriptionRenderCache(ctx context.Context, entryID string, ttlSeconds int, result *domain.RenderResult) {
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixSubscriptionRender)
	if s.cache == nil || entryID == "" || ttlSeconds <= 0 || result == nil || len(result.Body) > maxResultCacheBodyBytes || !owned {
		return
	}
	dependencies, err := s.snapshotCacheDependencies(ctx, result.Report.Dependencies)
	if err != nil {
		return
	}
	out := result.Clone()
	out.Cached = false
	ttl := time.Duration(ttlSeconds) * time.Second
	value, remaining, ok := prepareCacheValueWrite[subscriptionRenderCacheValue](s, ctx, key, ttl)
	if !ok {
		return
	}
	if value.Results == nil {
		value.Results = map[string]cachedRenderResult{}
	}
	value.Results[entryID] = cachedRenderResult{
		Result: *out, Dependencies: dependencies,
	}
	_ = cachepkg.SetJSON(ctx, s.cache, key, value, remaining)
}

func (s *Service) readFileRenderCache(ctx context.Context, entryID string, ttl time.Duration) *domain.FileResult {
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixFileRender)
	if s.cache == nil || entryID == "" || !owned {
		return nil
	}
	item, found := readCacheValue[fileRenderCacheValue](s, ctx, key, ttl)
	if !found {
		return nil
	}
	cached, found := item.Value.Results[entryID]
	if !found || !s.cacheDependenciesCurrent(ctx, cached.Dependencies) {
		return nil
	}
	out := cached.Result.Clone()
	out.Cached = true
	return out
}

func (s *Service) writeFileRenderCache(ctx context.Context, entryID string, ttlSeconds int, result *domain.FileResult) {
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixFileRender)
	if s.cache == nil || entryID == "" || ttlSeconds <= 0 || result == nil || len(result.Content) > maxResultCacheBodyBytes || !owned {
		return
	}
	dependencies, err := s.snapshotCacheDependencies(ctx, result.Report.Dependencies)
	if err != nil {
		return
	}
	out := result.Clone()
	out.Cached = false
	ttl := time.Duration(ttlSeconds) * time.Second
	value, remaining, ok := prepareCacheValueWrite[fileRenderCacheValue](s, ctx, key, ttl)
	if !ok {
		return
	}
	if value.Results == nil {
		value.Results = map[string]cachedFileResult{}
	}
	value.Results[entryID] = cachedFileResult{
		Result: *out, Dependencies: dependencies,
	}
	_ = cachepkg.SetJSON(ctx, s.cache, key, value, remaining)
}

func (s *Service) snapshotCacheDependencies(ctx context.Context, refs []domain.ResourceRef) ([]cacheDependencyRevision, error) {
	unique := map[string]domain.ResourceRef{}
	for _, ref := range refs {
		kind := strings.ToLower(strings.TrimSpace(ref.Kind))
		name := strings.TrimSpace(ref.Name)
		if name == "" || (kind != "subscription" && kind != "file") {
			continue
		}
		ref.Kind = kind
		ref.Name = name
		unique[kind+"\x00"+name] = ref
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]cacheDependencyRevision, 0, len(keys))
	for _, key := range keys {
		ref := unique[key]
		revision, err := s.cacheResourceRevision(ctx, ref)
		if err != nil {
			return nil, err
		}
		out = append(out, cacheDependencyRevision{Kind: ref.Kind, Name: ref.Name, Revision: revision})
	}
	return out, nil
}

func (s *Service) cacheDependenciesCurrent(ctx context.Context, dependencies []cacheDependencyRevision) bool {
	for _, dependency := range dependencies {
		revision, err := s.cacheResourceRevision(ctx, domain.ResourceRef{Kind: dependency.Kind, Name: dependency.Name})
		if err != nil || revision != dependency.Revision {
			return false
		}
	}
	return true
}

func (s *Service) cacheResourceRevision(ctx context.Context, ref domain.ResourceRef) (string, error) {
	if s.metaStore == nil {
		return "", storeUnavailable()
	}
	switch strings.ToLower(strings.TrimSpace(ref.Kind)) {
	case "subscription":
		subscription, err := s.metaStore.GetSubscription(ctx, strings.TrimSpace(ref.Name))
		if err != nil {
			return "", err
		}
		return cacheIdentity(subscription)
	case "file":
		file, err := s.metaStore.GetFile(ctx, strings.TrimSpace(ref.Name))
		if err != nil {
			return "", err
		}
		return cacheIdentity(file)
	default:
		return "", domain.NewError(domain.CodeInvalidArgument, "unsupported cache dependency kind")
	}
}
