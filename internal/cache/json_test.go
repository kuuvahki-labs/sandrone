package cache_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

type JSONCacheFixture struct {
	Name  string   `json:"name"`
	Nodes []string `json:"nodes"`
}

func TestCacheJSONRoundTripPreservesDeadline(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	backend := cache.New(store.NewFSStore(afero.NewMemMapFs()), func() time.Time { return now })
	want := JSONCacheFixture{Name: "A", Nodes: []string{"one", "two"}}

	require.NoError(t, cache.SetJSON(ctx, backend, "probe/subscriptions/A", want, time.Hour))
	item, found, err := cache.GetJSON[JSONCacheFixture](ctx, backend, "probe/subscriptions/A")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, want, item.Value)
	require.Equal(t, now.Add(time.Hour), item.ExpiresAt)
}

func TestCacheJSONMissingAndDecodeErrors(t *testing.T) {
	ctx := context.Background()
	backend := cache.New(store.NewFSStore(afero.NewMemMapFs()), time.Now)

	_, found, err := cache.GetJSON[JSONCacheFixture](ctx, backend, "missing")
	require.NoError(t, err)
	require.False(t, found)

	require.NoError(t, backend.Set(ctx, "invalid", []byte(`{invalid`), time.Hour))
	_, found, err = cache.GetJSON[JSONCacheFixture](ctx, backend, "invalid")
	require.ErrorContains(t, err, `decode cache key "invalid" JSON`)
	require.False(t, found)

	err = cache.SetJSON(ctx, backend, "unsupported", math.Inf(1), time.Hour)
	require.ErrorContains(t, err, `encode cache key "unsupported" JSON`)
}
