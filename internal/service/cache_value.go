package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"time"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
)

func readCacheValue[T any](s *Service, ctx context.Context, key string, ttl time.Duration) (cachepkg.JSONItem[T], bool) {
	var zero cachepkg.JSONItem[T]
	if s.cache == nil || key == "" || ttl <= 0 {
		return zero, false
	}
	item, found, err := cachepkg.GetJSON[T](ctx, s.cache, key)
	if err != nil {
		s.log(ctx, slog.LevelWarn, "service cache read failed", "cache_key", key, "error", err.Error())
		return zero, false
	}
	if !found {
		return zero, false
	}
	now := s.now().UTC()
	if deadline := now.Add(ttl); deadline.Before(item.ExpiresAt) {
		if err := cachepkg.SetJSON(ctx, s.cache, key, item.Value, deadline.Sub(now)); err != nil {
			s.log(ctx, slog.LevelWarn, "service cache deadline update failed", "cache_key", key, "error", err.Error())
		} else {
			item.ExpiresAt = deadline
		}
	}
	return item, true
}

// prepareCacheValueWrite loads the business-owned value that a normal write
// will replace. The returned TTL preserves or shortens the existing absolute
// deadline. The first write to a key during refresh starts from the zero value
// and establishes a new deadline.
func prepareCacheValueWrite[T any](s *Service, ctx context.Context, key string, ttl time.Duration) (T, time.Duration, bool) {
	var value T
	if s.cache == nil || key == "" || ttl <= 0 {
		return value, 0, false
	}
	now := s.now().UTC()
	deadline := now.Add(ttl)
	if !cacheRefreshStartsKey(ctx, key) {
		item, found, err := cachepkg.GetJSON[T](ctx, s.cache, key)
		if err != nil {
			s.log(ctx, slog.LevelWarn, "service cache read before write failed", "cache_key", key, "error", err.Error())
		} else if found && item.ExpiresAt.After(now) {
			value = item.Value
			if item.ExpiresAt.Before(deadline) {
				deadline = item.ExpiresAt
			}
		}
	}
	remaining := deadline.Sub(now)
	if remaining <= 0 {
		_ = s.cache.Delete(ctx, key)
		return value, 0, false
	}
	return value, remaining, true
}

func cacheIdentity(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
