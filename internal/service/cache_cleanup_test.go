package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestClearCacheDeletesAllPersistentLayersInCanonicalOrder(t *testing.T) {
	cache := newTestCache()
	cache.values["cache/probe/one.json"] = json.RawMessage(`"probe"`)
	cache.values["cache/file_render/two.json"] = json.RawMessage(`"file"`)
	svc := service.New(service.WithCache(cache))

	require.NoError(t, svc.ClearCache(context.Background()))
	require.Equal(t, []string{
		"remote_fetch",
		"probe",
		"subscription_traffic",
		"subscription_render",
		"file_render",
	}, cache.deleted)
	require.Empty(t, cache.values)

	require.NoError(t, svc.ClearCache(context.Background()))
}

func TestClearCacheWithoutPersistentCacheIsNoOp(t *testing.T) {
	require.NoError(t, service.New().ClearCache(context.Background()))
}

func TestClearCacheStopsAfterLayerFailure(t *testing.T) {
	want := errors.New("delete failed")
	cache := &failingDeleteCache{failLayer: "probe", err: want}
	svc := service.New(service.WithCache(cache))

	err := svc.ClearCache(context.Background())
	require.True(t, domain.IsCode(err, domain.CodeCacheOperationFailed), "error = %v", err)
	require.ErrorIs(t, err, want)
	require.Equal(t, []string{"remote_fetch", "probe"}, cache.deleted)
}

type failingDeleteCache struct {
	failLayer string
	err       error
	deleted   []string
}

func (c *failingDeleteCache) GetJSON(context.Context, string, any) bool { return false }

func (c *failingDeleteCache) PutJSON(context.Context, string, time.Duration, any) error { return nil }

func (c *failingDeleteCache) DeleteLayer(_ context.Context, layer string) error {
	c.deleted = append(c.deleted, strings.TrimSpace(layer))
	if layer == c.failLayer {
		return c.err
	}
	return nil
}
