package service

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
)

type ownedCacheFixture struct {
	Values map[string]string `json:"values"`
}

func TestBusinessCacheValueUsesOneConservativeAbsoluteDeadline(t *testing.T) {
	ctx := withSubscriptionCacheOwner(context.Background(), "A")
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	svc := New(WithFS(afero.NewMemMapFs()), WithClock(func() time.Time { return now }))
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixProbe)
	require.True(t, owned)
	require.Equal(t, "probe/subscriptions/A", key)

	write := func(writeCtx context.Context, ttl time.Duration, id, content string) {
		t.Helper()
		value, remaining, ok := svc.prepareCacheValueWrite[ownedCacheFixture](writeCtx, key, ttl)
		require.True(t, ok)
		if value.Values == nil {
			value.Values = map[string]string{}
		}
		value.Values[id] = content
		require.NoError(t, cachepkg.SetJSON(writeCtx, svc.cache, key, value, remaining))
	}

	write(ctx, time.Hour, "one", "first")
	first, found, err := svc.cache.Get(ctx, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, now.Add(time.Hour), first.ExpiresAt)
	require.NotContains(t, string(first.Value), "expires_at")
	require.NotContains(t, string(first.Value), "stored_at")

	now = now.Add(30 * time.Minute)
	write(ctx, 2*time.Hour, "two", "second")
	merged, found, err := cachepkg.GetJSON[ownedCacheFixture](ctx, svc.cache, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first.ExpiresAt, merged.ExpiresAt, "a normal full-value write must not extend the deadline")
	require.Equal(t, map[string]string{"one": "first", "two": "second"}, merged.Value.Values)

	now = now.Add(5 * time.Minute)
	write(ctx, 10*time.Minute, "three", "third")
	shortened, found, err := cachepkg.GetJSON[ownedCacheFixture](ctx, svc.cache, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, now.Add(10*time.Minute), shortened.ExpiresAt)

	now = now.Add(time.Minute)
	refreshCtx := withCacheReadBypass(ctx)
	write(refreshCtx, 2*time.Hour, "fresh", "replacement")
	refreshed, found, err := cachepkg.GetJSON[ownedCacheFixture](ctx, svc.cache, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, now.Add(2*time.Hour), refreshed.ExpiresAt)
	require.Equal(t, map[string]string{"fresh": "replacement"}, refreshed.Value.Values)

	write(refreshCtx, time.Hour, "also-fresh", "merged")
	refreshed, found, err = cachepkg.GetJSON[ownedCacheFixture](ctx, svc.cache, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, now.Add(time.Hour), refreshed.ExpiresAt)
	require.Equal(t, map[string]string{"fresh": "replacement", "also-fresh": "merged"}, refreshed.Value.Values)

	item, found := svc.readCacheValue[ownedCacheFixture](ctx, key, 10*time.Minute)
	require.True(t, found)
	require.Contains(t, item.Value.Values, "fresh")
	shortenedOnHit, found, err := cachepkg.GetJSON[ownedCacheFixture](ctx, svc.cache, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, now.Add(10*time.Minute), shortenedOnHit.ExpiresAt, "a shorter effective TTL must also shorten a full hit")
}
