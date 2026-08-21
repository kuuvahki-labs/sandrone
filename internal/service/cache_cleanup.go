package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

var persistentCacheLayers = []string{
	cacheLayerRemoteFetch,
	cacheLayerProbe,
	cacheLayerSubscriptionTraffic,
	cacheLayerSubscriptionRender,
	cacheLayerFileRender,
}

// ClearCache deletes every persistent cache layer. Deletion is intentionally
// non-transactional: completed layers stay cleared if a later layer fails.
func (s *Service) ClearCache(ctx context.Context) error {
	if s.cache == nil {
		return nil
	}
	for _, layer := range persistentCacheLayers {
		if err := s.cache.DeleteLayer(ctx, layer); err != nil {
			return domain.WrapError(
				domain.CodeCacheOperationFailed,
				fmt.Sprintf("cache clear failed for layer %q", layer),
				err,
			)
		}
	}
	s.log(ctx, slog.LevelInfo, "service cache cleared", "operation", "cache_clear")
	return nil
}
