package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestClearCacheClearsConfiguredBackend(t *testing.T) {
	cache := newTestCache()
	cache.values["probe/subscriptions/A"] = cachepkg.Item{Value: []byte(`"probe"`), ExpiresAt: time.Now().Add(time.Hour)}
	cache.values["subscription_snapshot/files/B"] = cachepkg.Item{Value: []byte(`"nodes"`), ExpiresAt: time.Now().Add(time.Hour)}
	svc := service.New(service.WithCache(cache))

	require.NoError(t, svc.ClearCache(context.Background()))
	require.Equal(t, 1, cache.clearCount)
	require.Empty(t, cache.values)

	require.NoError(t, svc.ClearCache(context.Background()))
	require.Equal(t, 2, cache.clearCount)
}

func TestResourceDeletionClearsAllOwnedCacheLayers(t *testing.T) {
	ctx := context.Background()
	cache := newTestCache()
	svc := service.New(service.WithFS(afero.NewMemMapFs()), service.WithCache(cache))

	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "sub", Type: domain.SubscriptionTypeLocal,
	}))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name: "file", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "body"},
	}))
	require.Empty(t, cache.deleted)

	require.NoError(t, svc.DeleteSubscription(ctx, "sub"))
	require.ElementsMatch(t, []string{
		"remote_fetch/subscriptions/sub",
		"probe/subscriptions/sub",
		"subscription_snapshot/subscriptions/sub",
	}, cache.deleted)

	cache.deleted = nil
	require.NoError(t, svc.DeleteFile(ctx, "file"))
	require.ElementsMatch(t, []string{
		"remote_fetch/files/file",
		"probe/files/file",
		"subscription_snapshot/files/file",
	}, cache.deleted)
}

func TestClearCacheWithoutPersistentCacheIsNoOp(t *testing.T) {
	require.NoError(t, service.New().ClearCache(context.Background()))
}

func TestClearCacheMapsBackendFailure(t *testing.T) {
	want := errors.New("delete failed")
	cache := &failingClearCache{err: want}
	svc := service.New(service.WithCache(cache))

	err := svc.ClearCache(context.Background())
	require.True(t, domain.IsCode(err, domain.CodeCacheOperationFailed), "error = %v", err)
	require.ErrorIs(t, err, want)
	require.Equal(t, 1, cache.calls)
}

type failingClearCache struct {
	err   error
	calls int
}

func (c *failingClearCache) Get(context.Context, string) (cachepkg.Item, bool, error) {
	return cachepkg.Item{}, false, nil
}

func (c *failingClearCache) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

func (c *failingClearCache) Delete(context.Context, string) error { return nil }

func (c *failingClearCache) Clear(context.Context) error {
	c.calls++
	return c.err
}
