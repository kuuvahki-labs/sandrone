package service

import (
	"context"
	"log/slog"
	"path"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) deleteCacheOwner(ctx context.Context, resourceKind, resourceName string) {
	if s.cache == nil {
		return
	}
	for _, prefix := range ownedCacheKeyPrefixes {
		_ = s.cache.Delete(ctx, path.Join(prefix, resourceKind, resourceName))
	}
}

var ownedCacheKeyPrefixes = []string{
	cacheKeyPrefixRemoteFetch,
	cacheKeyPrefixProbe,
	cacheKeyPrefixSubscriptionSnapshot,
}

// ClearCache deletes every value owned by the configured cache backend.
func (s *Service) ClearCache(ctx context.Context) error {
	if s.cache == nil {
		return nil
	}
	if err := s.cache.Clear(ctx); err != nil {
		return domain.WrapError(
			domain.CodeCacheOperationFailed,
			"cache clear failed",
			err,
		)
	}
	s.log(ctx, slog.LevelInfo, "service cache cleared", "operation", "cache_clear")
	return nil
}
