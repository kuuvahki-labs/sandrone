package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestTransientInputDoesNotUsePersistentCache(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#node"))
	}))
	defer server.Close()

	persistentCache := newTestCache()
	svc := service.New(service.WithCache(persistentCache))
	req := domain.ParseRequest{
		Format: "uri-list",
		Remote: &domain.RemoteInput{
			URL:             server.URL,
			CacheTTLSeconds: 60,
		},
	}

	first, err := svc.Parse(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, first.Nodes, 1)
	second, err := svc.Parse(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, second.Nodes, 1)
	require.Equal(t, 2, requests)
	require.Empty(t, persistentCache.values)
	require.Empty(t, persistentCache.deleted)
	require.Zero(t, persistentCache.clearCount)
}

type testCache struct {
	mu         sync.Mutex
	values     map[string]cachepkg.Item
	deleted    []string
	clearCount int
	now        func() time.Time
}

func newTestCache() *testCache {
	return &testCache{values: map[string]cachepkg.Item{}, now: time.Now}
}

func (c *testCache) Get(_ context.Context, key string) (cachepkg.Item, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, found := c.values[key]
	if !found || !c.now().Before(item.ExpiresAt) {
		return cachepkg.Item{}, false, nil
	}
	item.Value = append([]byte(nil), item.Value...)
	return item, true, nil
}

func (c *testCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ttl <= 0 {
		delete(c.values, key)
		return nil
	}
	c.values[key] = cachepkg.Item{Value: append([]byte(nil), value...), ExpiresAt: c.now().Add(ttl)}
	return nil
}

func (c *testCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleted = append(c.deleted, key)
	delete(c.values, key)
	return nil
}

func (c *testCache) Clear(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearCount++
	c.values = map[string]cachepkg.Item{}
	return nil
}
