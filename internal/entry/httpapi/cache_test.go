package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
	"github.com/kuuvahki-labs/sandrone/internal/service"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func TestClearCacheRequiresAuthenticationAndDisablesHTTPStorage(t *testing.T) {
	rt, _ := cacheHTTPRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	server := httpapi.New(rt)

	unauthorized := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodDelete, "/v1/cache", nil))
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)

	request := httptest.NewRequest(http.MethodDelete, "/v1/cache", nil)
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Empty(t, response.Body.String())
}

func TestClearCacheDeletesAllPersistentKeysAndIsIdempotent(t *testing.T) {
	rt, resourceStore := cacheHTTPRuntime(t, app.Config{})
	cache := cachepkg.New(resourceStore, time.Now)
	ctx := context.Background()
	keys := make([]string, 0, 3)
	for _, prefix := range []string{
		"remote_fetch",
		"probe",
		"subscription_snapshot",
	} {
		key := prefix + "/subscriptions/all"
		require.NoError(t, cache.Set(ctx, key, []byte(`{"entries":{"entry":"value"}}`), time.Hour))
		keys = append(keys, "cache/"+key+".json")
	}

	server := httpapi.New(rt)
	for range 2 {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/v1/cache", nil))
		require.Equal(t, http.StatusNoContent, response.Code)
	}
	for _, key := range keys {
		_, err := resourceStore.Stat(ctx, key)
		require.Error(t, err)
	}
}

func TestCacheStatisticsEndpointDoesNotExist(t *testing.T) {
	rt, _ := cacheHTTPRuntime(t, app.Config{})
	response := httptest.NewRecorder()
	httpapi.New(rt).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/cache", nil))
	require.Equal(t, http.StatusMethodNotAllowed, response.Code)
}

func TestClearCacheMapsFailuresWithoutLeakingCause(t *testing.T) {
	secret := "private backend detail"
	rt := testRuntime(t, app.Config{})
	rt.Service = service.New(service.WithCache(&cacheHTTPDeleteFailCache{err: errors.New(secret)}))

	response := httptest.NewRecorder()
	httpapi.New(rt).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/v1/cache", nil))
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.JSONEq(t, `{"error":{"code":"cache_operation_failed","message":"cache clear failed"}}`, response.Body.String())
	require.NotContains(t, response.Body.String(), secret)
}

func cacheHTTPRuntime(t *testing.T, cfg app.Config) (*app.Runtime, store.Store) {
	t.Helper()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	rt := testRuntime(t, cfg)
	rt.Service = service.New(service.WithStore(resourceStore), service.WithLogger(rt.Logger))
	return rt, resourceStore
}

type cacheHTTPDeleteFailCache struct {
	err error
}

func (c *cacheHTTPDeleteFailCache) Get(context.Context, string) (cachepkg.Item, bool, error) {
	return cachepkg.Item{}, false, nil
}

func (c *cacheHTTPDeleteFailCache) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

func (c *cacheHTTPDeleteFailCache) Delete(context.Context, string) error {
	return nil
}

func (c *cacheHTTPDeleteFailCache) Clear(context.Context) error { return c.err }
