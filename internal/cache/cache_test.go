package cache_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

type storedCacheEnvelope struct {
	ExpiresAt time.Time `json:"expires_at"`
	Encoding  string    `json:"encoding,omitempty"`
	Value     []byte    `json:"value"`
}

func TestCacheStoresOneOpaqueValueWithOneTTL(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	c := cache.New(resourceStore, func() time.Time { return now })

	require.NoError(t, c.Set(ctx, "probe/subscriptions/group/all", []byte(`{"entries":{"one":1}}`), time.Minute))
	item, found, err := c.Get(ctx, "probe/subscriptions/group/all")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte(`{"entries":{"one":1}}`), item.Value)
	require.Equal(t, now.Add(time.Minute), item.ExpiresAt)

	_, err = resourceStore.Stat(ctx, "cache/probe/subscriptions/group/all.json")
	require.NoError(t, err)
	now = now.Add(time.Minute)
	_, found, err = c.Get(ctx, "probe/subscriptions/group/all")
	require.NoError(t, err)
	require.False(t, found)
}

func TestCacheSetReplacesValueAndTTL(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	c := cache.New(store.NewFSStore(afero.NewMemMapFs()), func() time.Time { return now })
	require.NoError(t, c.Set(ctx, "remote_fetch/files/config", []byte("old"), time.Minute))
	now = now.Add(30 * time.Second)
	require.NoError(t, c.Set(ctx, "remote_fetch/files/config", []byte("new"), 2*time.Minute))

	item, found, err := c.Get(ctx, "remote_fetch/files/config")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("new"), item.Value)
	require.Equal(t, now.Add(2*time.Minute), item.ExpiresAt)
}

func TestCacheCompressesLargeCompressibleValuesTransparently(t *testing.T) {
	ctx := context.Background()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	c := cache.New(resourceStore, time.Now)
	value := bytes.Repeat([]byte(`{"alive":false,"error":"connection refused"}`), 4096)
	require.NoError(t, c.Set(ctx, "probe/subscriptions/all", value, time.Minute))

	body, err := resourceStore.Read(ctx, "cache/probe/subscriptions/all.json")
	require.NoError(t, err)
	var envelope storedCacheEnvelope
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.Equal(t, "gzip", envelope.Encoding)
	require.Less(t, len(envelope.Value), len(value))
	require.Less(t, len(body), len(value))

	item, found, err := c.Get(ctx, "probe/subscriptions/all")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, value, item.Value)
}

func TestCacheKeepsLargeIncompressibleValuesRaw(t *testing.T) {
	ctx := context.Background()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	c := cache.New(resourceStore, time.Now)
	value := make([]byte, 16<<10)
	random := rand.New(rand.NewPCG(1, 2))
	for index := range value {
		value[index] = byte(random.Uint32())
	}
	require.NoError(t, c.Set(ctx, "remote_fetch/subscriptions/random", value, time.Minute))

	body, err := resourceStore.Read(ctx, "cache/remote_fetch/subscriptions/random.json")
	require.NoError(t, err)
	var envelope storedCacheEnvelope
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.Empty(t, envelope.Encoding)
	require.Equal(t, value, envelope.Value)
}

func TestCacheRejectsCorruptAndUnknownCompressedValues(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	c := cache.New(resourceStore, func() time.Time { return now })
	writeEnvelope := func(key, encoding string, value []byte) {
		t.Helper()
		body, err := json.Marshal(storedCacheEnvelope{ExpiresAt: now.Add(time.Minute), Encoding: encoding, Value: value})
		require.NoError(t, err)
		require.NoError(t, resourceStore.Write(ctx, "cache/"+key+".json", body))
	}

	writeEnvelope("probe/subscriptions/corrupt", "gzip", []byte("not gzip"))
	_, _, err := c.Get(ctx, "probe/subscriptions/corrupt")
	require.ErrorContains(t, err, "decode cache key")

	writeEnvelope("probe/subscriptions/unknown", "brotli", []byte("value"))
	_, _, err = c.Get(ctx, "probe/subscriptions/unknown")
	require.ErrorContains(t, err, `unsupported cache encoding "brotli"`)
}

func TestCacheMissingCorruptAndInvalidKeys(t *testing.T) {
	ctx := context.Background()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	c := cache.New(resourceStore, time.Now)

	_, found, err := c.Get(ctx, "remote_fetch/subscriptions/missing")
	require.NoError(t, err)
	require.False(t, found)
	require.NoError(t, resourceStore.Write(ctx, "cache/remote_fetch/subscriptions/bad.json", []byte(`{bad json`)))
	_, _, err = c.Get(ctx, "remote_fetch/subscriptions/bad")
	require.Error(t, err)
	require.Error(t, c.Set(ctx, "../escape", []byte("value"), time.Minute))
}

func TestCacheDeleteAndClear(t *testing.T) {
	ctx := context.Background()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	c := cache.New(resourceStore, time.Now)
	require.NoError(t, c.Set(ctx, "probe/subscriptions/A", []byte("probe"), time.Minute))
	require.NoError(t, c.Set(ctx, "file_render/files/B", []byte("file"), time.Minute))

	require.NoError(t, c.Delete(ctx, "probe/subscriptions/A"))
	_, err := resourceStore.Stat(ctx, "cache/probe/subscriptions/A.json")
	require.True(t, errors.Is(err, os.ErrNotExist), "got %v", err)
	require.NoError(t, c.Clear(ctx))
	_, err = resourceStore.Stat(ctx, "cache/file_render/files/B.json")
	require.True(t, errors.Is(err, os.ErrNotExist), "got %v", err)
	require.NoError(t, c.Clear(ctx))
}

func TestCacheNonPositiveTTLDeletesExistingValue(t *testing.T) {
	ctx := context.Background()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	c := cache.New(resourceStore, time.Now)
	require.NoError(t, c.Set(ctx, "probe/subscriptions/A", []byte("probe"), time.Minute))
	require.NoError(t, c.Set(ctx, "probe/subscriptions/A", []byte("ignored"), 0))
	_, found, err := c.Get(ctx, "probe/subscriptions/A")
	require.NoError(t, err)
	require.False(t, found)
}

func TestCacheAllowsEmptyOpaqueValue(t *testing.T) {
	ctx := context.Background()
	c := cache.New(store.NewFSStore(afero.NewMemMapFs()), time.Now)
	require.NoError(t, c.Set(ctx, "empty/value", nil, time.Minute))
	item, found, err := c.Get(ctx, "empty/value")
	require.NoError(t, err)
	require.True(t, found)
	require.Empty(t, item.Value)
}
