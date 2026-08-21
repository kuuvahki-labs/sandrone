package cache_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func TestCacheJSONHonorsTTLAndLayeredKeys(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	st := store.NewFSStore(afero.NewMemMapFs())
	c := cache.New(st, func() time.Time { return now })
	key, err := cache.HashKey("probe", map[string]any{"node": "a"})
	require.NoError(t, err)
	require.Equal(t, "cache/probe/9ccceaef892eae566be2dd418e9bd0bcde57f11f5da24073defa1fba3c38ea98.json", key)

	require.NoError(t, c.PutJSON(ctx, key, time.Minute, map[string]string{"value": "cached"}))
	body, err := st.Read(ctx, key)
	require.NoError(t, err)
	require.NotContains(t, string(body), "\n")
	require.NotContains(t, string(body), "  ")
	require.True(t, json.Valid(body))

	var got map[string]string
	hit := c.GetJSON(ctx, key, &got)
	require.True(t, hit)
	require.Equal(t, "cached", got["value"])
	require.Contains(t, key, "cache/probe/")

	now = now.Add(2 * time.Minute)
	got = nil
	hit = c.GetJSON(ctx, key, &got)
	require.False(t, hit)
	require.Nil(t, got)
	_, err = st.Stat(ctx, key)
	require.True(t, errors.Is(err, os.ErrNotExist), "got %v", err)
}

func TestCacheJSONIgnoresMissingExpiredAndBadRecords(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	st := store.NewFSStore(afero.NewMemMapFs())
	c := cache.New(st, func() time.Time { return now })
	key, err := cache.HashKey("remote_fetch", "https://example.test/sub")
	require.NoError(t, err)

	var got map[string]string
	hit := c.GetJSON(ctx, key, &got)
	require.False(t, hit)

	require.NoError(t, st.Write(ctx, key, []byte(`{bad json`)))
	hit = c.GetJSON(ctx, key, &got)
	require.False(t, hit)

	body, err := json.Marshal(map[string]any{
		"stored_at":  now.Add(-time.Hour),
		"expires_at": now.Add(-time.Minute),
		"value":      map[string]string{"value": "old"},
	})
	require.NoError(t, err)
	require.NoError(t, st.Write(ctx, key, body))
	hit = c.GetJSON(ctx, key, &got)
	require.False(t, hit)
}

func TestCacheDeleteLayerRemovesOnlyThatLayer(t *testing.T) {
	ctx := context.Background()
	st := store.NewFSStore(afero.NewMemMapFs())
	c := cache.New(st, func() time.Time { return time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC) })
	trafficKey, err := cache.HashKey("subscription_traffic", "remote/live")
	require.NoError(t, err)
	probeKey, err := cache.HashKey("probe", "node-a")
	require.NoError(t, err)
	require.NoError(t, c.PutJSON(ctx, trafficKey, time.Minute, "traffic"))
	require.NoError(t, c.PutJSON(ctx, probeKey, time.Minute, "probe"))

	require.NoError(t, c.DeleteLayer(ctx, "subscription_traffic"))

	_, err = st.Stat(ctx, trafficKey)
	require.True(t, errors.Is(err, os.ErrNotExist), "got %v", err)
	_, err = st.Stat(ctx, probeKey)
	require.NoError(t, err)
}

func TestCacheJSONDoesNotStoreDisabledEntries(t *testing.T) {
	ctx := context.Background()
	st := store.NewFSStore(afero.NewMemMapFs())
	c := cache.New(st, time.Now)
	key, err := cache.HashKey("probe", "disabled")
	require.NoError(t, err)

	require.NoError(t, c.PutJSON(ctx, key, 0, "ignored"))

	_, err = st.Stat(ctx, key)
	require.True(t, errors.Is(err, os.ErrNotExist), "got %v", err)
}
