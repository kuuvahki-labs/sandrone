package service

import (
	"context"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const (
	cacheLayerSubscriptionRender = "subscription_render"
	cacheLayerFileRender         = "file_render"
	resultCacheSchemaVersion     = "v1"
	maxResultCacheBodyBytes      = 16 << 20
)

type resultCacheBuild struct {
	Schema   string `json:"schema"`
	Version  string `json:"version"`
	Revision string `json:"revision,omitempty"`
}

func currentResultCacheBuild() resultCacheBuild {
	return resultCacheBuild{
		Schema:   resultCacheSchemaVersion,
		Version:  buildinfo.Version(),
		Revision: buildinfo.Revision(),
	}
}

func (s *Service) subscriptionRenderTTLSeconds(ctx context.Context, override *int) (int, error) {
	if override != nil {
		return *override, nil
	}
	settings, err := s.EffectiveRuntimeSettings(ctx)
	if err != nil {
		return 0, err
	}
	return settings.CacheDefaults.SubscriptionRenderTTLSeconds, nil
}

func (s *Service) fileRenderTTLSeconds(ctx context.Context, override *int) (int, error) {
	if override != nil {
		return *override, nil
	}
	settings, err := s.EffectiveRuntimeSettings(ctx)
	if err != nil {
		return 0, err
	}
	return settings.CacheDefaults.FileRenderTTLSeconds, nil
}

func subscriptionRenderCacheKey(sub domain.Subscription, format string, req domain.RequestInfo) (string, error) {
	return cachepkg.HashKey(cacheLayerSubscriptionRender, struct {
		Build        resultCacheBuild    `json:"build"`
		Subscription domain.Subscription `json:"subscription"`
		Format       string              `json:"format"`
		Request      domain.RequestInfo  `json:"request,omitempty"`
	}{
		Build:        currentResultCacheBuild(),
		Subscription: sub,
		Format:       format,
		Request:      req,
	})
}

func fileRenderCacheKey(spec domain.FileSpec, req domain.FileRequest) (string, error) {
	return cachepkg.HashKey(cacheLayerFileRender, struct {
		Build   resultCacheBuild   `json:"build"`
		File    domain.FileSpec    `json:"file"`
		Target  string             `json:"target,omitempty"`
		Request domain.RequestInfo `json:"request,omitempty"`
		Meta    map[string]string  `json:"meta,omitempty"`
	}{
		Build:   currentResultCacheBuild(),
		File:    spec,
		Target:  req.Target,
		Request: req.Request,
		Meta:    req.Meta,
	})
}

func (s *Service) readSubscriptionRenderCache(ctx context.Context, key string) *domain.RenderResult {
	if s.cache == nil || key == "" {
		return nil
	}
	var result domain.RenderResult
	if !s.cache.GetJSON(ctx, key, &result) {
		return nil
	}
	out := cloneRenderResult(&result)
	out.Cached = true
	return out
}

func (s *Service) writeSubscriptionRenderCache(ctx context.Context, key string, ttlSeconds int, result *domain.RenderResult) {
	if s.cache == nil || key == "" || ttlSeconds <= 0 || result == nil || len(result.Body) > maxResultCacheBodyBytes {
		return
	}
	out := cloneRenderResult(result)
	out.Cached = false
	_ = s.cache.PutJSON(ctx, key, time.Duration(ttlSeconds)*time.Second, out)
}

func (s *Service) readFileRenderCache(ctx context.Context, key string) *domain.FileResult {
	if s.cache == nil || key == "" {
		return nil
	}
	var result domain.FileResult
	if !s.cache.GetJSON(ctx, key, &result) {
		return nil
	}
	out := cloneFileResult(&result)
	out.Cached = true
	return out
}

func (s *Service) writeFileRenderCache(ctx context.Context, key string, ttlSeconds int, result *domain.FileResult) {
	if s.cache == nil || key == "" || ttlSeconds <= 0 || result == nil || len(result.Content) > maxResultCacheBodyBytes {
		return
	}
	out := cloneFileResult(result)
	out.Cached = false
	_ = s.cache.PutJSON(ctx, key, time.Duration(ttlSeconds)*time.Second, out)
}

func (s *Service) invalidateResultCaches(ctx context.Context) {
	if s.cache == nil {
		return
	}
	_ = s.cache.DeleteLayer(ctx, cacheLayerSubscriptionRender)
	_ = s.cache.DeleteLayer(ctx, cacheLayerFileRender)
}
