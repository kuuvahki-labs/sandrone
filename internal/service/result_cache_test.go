package service_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceSubscriptionRenderResultCacheAndRefresh(t *testing.T) {
	ctx := context.Background()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = fmt.Fprintf(w, "ss://aes-128-gcm:secret@example.com:8388#node-%d", calls)
	}))
	defer server.Close()

	ttl := 60
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:                  "remote",
		Type:                  domain.SubscriptionTypeRemote,
		Format:                "uri-list",
		Remote:                &domain.RemoteInput{URL: server.URL},
		RenderCacheTTLSeconds: &ttl,
	}))

	first, err := svc.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{
		Name: "remote", Format: "uri-list",
	})
	require.NoError(t, err)
	require.False(t, first.Cached)
	require.Contains(t, string(first.Body), "node-1")

	second, err := svc.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{
		Name: "remote", Format: "uri-list",
	})
	require.NoError(t, err)
	require.True(t, second.Cached)
	require.Equal(t, first.Body, second.Body)
	require.Equal(t, 1, calls)

	refreshed, err := svc.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{
		Name: "remote", Format: "uri-list", Refresh: true,
	})
	require.NoError(t, err)
	require.False(t, refreshed.Cached)
	require.Contains(t, string(refreshed.Body), "node-2")
	require.Equal(t, 2, calls)

	afterRefresh, err := svc.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{
		Name: "remote", Format: "uri-list",
	})
	require.NoError(t, err)
	require.True(t, afterRefresh.Cached)
	require.Equal(t, refreshed.Body, afterRefresh.Body)
	require.Equal(t, 2, calls)
}

func TestServiceFileRenderResultCacheAndRefresh(t *testing.T) {
	ctx := context.Background()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = fmt.Fprintf(w, "remote-%d", calls)
	}))
	defer server.Close()

	ttl := 60
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name: "remote.txt",
		Kind: domain.FileKindStatic,
		Source: domain.FileSource{
			Type:   "remote",
			Remote: &domain.RemoteInput{URL: server.URL},
		},
		RenderCacheTTLSeconds: &ttl,
	}))

	first, err := svc.GetFile(ctx, domain.FileRequest{Name: "remote.txt"})
	require.NoError(t, err)
	require.False(t, first.Cached)
	require.Equal(t, "remote-1", string(first.Content))

	second, err := svc.GetFile(ctx, domain.FileRequest{Name: "remote.txt"})
	require.NoError(t, err)
	require.True(t, second.Cached)
	require.Equal(t, first.Content, second.Content)
	require.Equal(t, 1, calls)

	refreshed, err := svc.GetFile(ctx, domain.FileRequest{Name: "remote.txt", Refresh: true})
	require.NoError(t, err)
	require.False(t, refreshed.Cached)
	require.Equal(t, "remote-2", string(refreshed.Content))
	require.Equal(t, 2, calls)

	afterRefresh, err := svc.GetFile(ctx, domain.FileRequest{Name: "remote.txt"})
	require.NoError(t, err)
	require.True(t, afterRefresh.Cached)
	require.Equal(t, refreshed.Content, afterRefresh.Content)
	require.Equal(t, 2, calls)
}

func TestServiceExplicitZeroDisablesInheritedFileRenderCache(t *testing.T) {
	ctx := context.Background()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = fmt.Fprintf(w, "remote-%d", calls)
	}))
	defer server.Close()

	disabled := 0
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutRuntimeSettings(ctx, domain.RuntimeSettings{
		CacheDefaults: domain.CacheDefaults{FileRenderTTLSeconds: 60},
	}))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:                  "remote.txt",
		Kind:                  domain.FileKindStatic,
		Source:                domain.FileSource{Type: "remote", Remote: &domain.RemoteInput{URL: server.URL}},
		RenderCacheTTLSeconds: &disabled,
	}))

	first, err := svc.GetFile(ctx, domain.FileRequest{Name: "remote.txt"})
	require.NoError(t, err)
	second, err := svc.GetFile(ctx, domain.FileRequest{Name: "remote.txt"})
	require.NoError(t, err)
	require.False(t, first.Cached)
	require.False(t, second.Cached)
	require.Equal(t, "remote-1", string(first.Content))
	require.Equal(t, "remote-2", string(second.Content))
	require.Equal(t, 2, calls)
}

func TestServiceValidateFileBypassesFileRenderResultCache(t *testing.T) {
	ctx := context.Background()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = fmt.Fprintf(w, "remote-%d", calls)
	}))
	defer server.Close()

	ttl := 60
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:                  "remote.txt",
		Kind:                  domain.FileKindStatic,
		Source:                domain.FileSource{Type: "remote", Remote: &domain.RemoteInput{URL: server.URL}},
		RenderCacheTTLSeconds: &ttl,
	}))
	_, err := svc.GetFile(ctx, domain.FileRequest{Name: "remote.txt"})
	require.NoError(t, err)

	validated, err := svc.ValidateFile(ctx, domain.FileRequest{Name: "remote.txt"})
	require.NoError(t, err)
	require.True(t, validated.OK)
	require.Equal(t, 2, calls)

	cached, err := svc.GetFile(ctx, domain.FileRequest{Name: "remote.txt"})
	require.NoError(t, err)
	require.True(t, cached.Cached)
	require.Equal(t, "remote-1", string(cached.Content))
	require.Equal(t, 2, calls)
}

func TestServiceResourceMutationsInvalidateResultCacheLayers(t *testing.T) {
	ctx := context.Background()
	resultCache := newTestCache()
	svc := service.New(
		service.WithFS(afero.NewMemMapFs()),
		service.WithCache(resultCache),
	)

	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "sub", Type: domain.SubscriptionTypeLocal,
	}))
	require.ElementsMatch(t, []string{
		"subscription_traffic", "subscription_render", "file_render",
	}, resultCache.deleted)

	resultCache.deleted = nil
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name: "file", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "body"},
	}))
	require.ElementsMatch(t, []string{
		"subscription_render", "file_render",
	}, resultCache.deleted)

	resultCache.deleted = nil
	require.NoError(t, svc.PutRuntimeSettings(ctx, domain.RuntimeSettings{}))
	require.ElementsMatch(t, []string{
		"subscription_render", "file_render",
	}, resultCache.deleted)
}
