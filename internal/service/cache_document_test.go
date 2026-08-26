package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestCacheDocumentUsesOneConservativeAbsoluteDeadline(t *testing.T) {
	ctx := withSubscriptionCacheScope(context.Background(), "A")
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	svc := New(WithFS(afero.NewMemMapFs()), WithClock(func() time.Time { return now }))
	key, scoped := persistentCacheKey(ctx, cacheKeyPrefixProbe)
	require.True(t, scoped)
	require.Equal(t, "probe/subscriptions/A", key)

	require.NoError(t, svc.writeCacheEntries(ctx, key, time.Hour, []cacheDocumentUpdate{{ID: "one", Value: "first"}}))
	first, found, err := svc.cache.Get(ctx, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, now.Add(time.Hour), first.ExpiresAt)
	require.NotContains(t, string(first.Value), "expires_at")
	require.NotContains(t, string(first.Value), "stored_at")

	now = now.Add(30 * time.Minute)
	require.NoError(t, svc.writeCacheEntries(ctx, key, 2*time.Hour, []cacheDocumentUpdate{{ID: "two", Value: "second"}}))
	merged, found, err := svc.cache.Get(ctx, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first.ExpiresAt, merged.ExpiresAt, "a normal merge must not extend the document deadline")
	require.Equal(t, map[string]json.RawMessage{
		"one": json.RawMessage(`"first"`),
		"two": json.RawMessage(`"second"`),
	}, decodeCacheDocument(t, merged.Value).Entries)

	now = now.Add(5 * time.Minute)
	require.NoError(t, svc.writeCacheEntries(ctx, key, 10*time.Minute, []cacheDocumentUpdate{{ID: "three", Value: "third"}}))
	shortened, found, err := svc.cache.Get(ctx, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, now.Add(10*time.Minute), shortened.ExpiresAt)

	now = now.Add(time.Minute)
	refreshCtx := withCacheReadBypass(ctx)
	require.NoError(t, svc.writeCacheEntries(refreshCtx, key, 2*time.Hour, []cacheDocumentUpdate{{ID: "fresh", Value: "replacement"}}))
	refreshed, found, err := svc.cache.Get(ctx, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, now.Add(2*time.Hour), refreshed.ExpiresAt)
	require.Equal(t, map[string]json.RawMessage{
		"fresh": json.RawMessage(`"replacement"`),
	}, decodeCacheDocument(t, refreshed.Value).Entries)

	require.NoError(t, svc.writeCacheEntries(refreshCtx, key, time.Hour, []cacheDocumentUpdate{{ID: "also-fresh", Value: "merged"}}))
	refreshed, found, err = svc.cache.Get(ctx, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, now.Add(time.Hour), refreshed.ExpiresAt)
	require.Equal(t, map[string]json.RawMessage{
		"fresh":      json.RawMessage(`"replacement"`),
		"also-fresh": json.RawMessage(`"merged"`),
	}, decodeCacheDocument(t, refreshed.Value).Entries)

	hits := svc.readCacheEntries(ctx, key, []string{"fresh"}, 10*time.Minute)
	require.Contains(t, hits, "fresh")
	shortenedOnHit, found, err := svc.cache.Get(ctx, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, now.Add(10*time.Minute), shortenedOnHit.ExpiresAt, "a shorter effective TTL must also shorten a full-hit document")
}

func decodeCacheDocument(t *testing.T, body []byte) cacheDocument {
	t.Helper()
	var document cacheDocument
	require.NoError(t, json.Unmarshal(body, &document))
	return document
}
