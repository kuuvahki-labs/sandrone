package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceCacheDoesNotRequireResourceStore(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#node"))
	}))
	defer server.Close()

	svc := service.New(service.WithCache(newTestCache()))
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
	require.Equal(t, 1, requests)
}

type testCache struct {
	mu      sync.Mutex
	values  map[string][]byte
	deleted []string
}

func newTestCache() *testCache {
	return &testCache{values: map[string][]byte{}}
}

func (c *testCache) GetJSON(_ context.Context, key string, out any) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	body, ok := c.values[key]
	if !ok {
		return false
	}
	return json.Unmarshal(body, out) == nil
}

func (c *testCache) PutJSON(_ context.Context, key string, ttl time.Duration, value any) error {
	if ttl <= 0 {
		return nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = body
	return nil
}

func (c *testCache) DeleteLayer(_ context.Context, layer string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleted = append(c.deleted, layer)
	prefix := "cache/" + strings.TrimSpace(layer) + "/"
	for key := range c.values {
		if strings.HasPrefix(key, prefix) {
			delete(c.values, key)
		}
	}
	return nil
}
