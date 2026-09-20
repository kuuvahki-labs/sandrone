package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
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

func TestRemoteFetchCacheIgnoresLegacySubscriptionRecords(t *testing.T) {
	ctx := withSubscriptionCacheOwner(t.Context(), "A")
	svc := New(WithFS(afero.NewMemMapFs()))
	key, owned := ownedCacheKey(ctx, cacheKeyPrefixRemoteFetch)
	require.True(t, owned)
	entryID, err := remoteFetchCacheEntryID(domain.RemoteInput{URL: "https://example.test/sub"})
	require.NoError(t, err)
	require.NoError(t, cachepkg.SetJSON(ctx, svc.cache, key, remoteFetchCacheValue{
		Records: map[string]remoteFetchCacheRecord{
			entryID: {
				Body:       []byte("temporary upstream error"),
				Headers:    http.Header{"Content-Type": []string{"text/plain"}},
				StatusCode: http.StatusOK,
			},
		},
	}, time.Hour))

	require.Nil(t, svc.readRemoteFetchCache(ctx, key, entryID, time.Hour))
	fileCtx := withFileCacheOwner(t.Context(), "base")
	fileKey, owned := ownedCacheKey(fileCtx, cacheKeyPrefixRemoteFetch)
	require.True(t, owned)
	require.NoError(t, cachepkg.SetJSON(fileCtx, svc.cache, fileKey, remoteFetchCacheValue{
		Records: map[string]remoteFetchCacheRecord{
			entryID: {Body: []byte("legacy file body"), StatusCode: http.StatusOK},
		},
	}, time.Hour))
	require.NotNil(t, svc.readRemoteFetchCache(fileCtx, fileKey, entryID, time.Hour))

	valid := &remoteInputResult{
		Body:       []byte("ss://aes-128-gcm:secret@example.com:8388#node"),
		Headers:    http.Header{"Content-Type": []string{"text/plain"}},
		StatusCode: http.StatusOK,
	}
	require.NoError(t, svc.writeRemoteFetchCache(ctx, key, entryID, time.Hour, valid))
	require.NotNil(t, svc.readRemoteFetchCache(ctx, key, entryID, time.Hour))
}
