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
	"github.com/kuuvahki-labs/sandrone/internal/store"
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

func TestSubscriptionRenderCacheValidatesSavedDependencyRevisions(t *testing.T) {
	ctx := context.Background()
	fs := afero.NewMemMapFs()
	resourceStore := store.NewFSStore(fs)
	ttl := 60
	svc := service.New(service.WithStore(resourceStore))
	putLeaf := func(server string) {
		require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
			Name: "B", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
			Content: "ss://aes-128-gcm:secret@" + server + ":8388#node-b",
		}))
	}
	putLeaf("before.example")
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "A", Type: domain.SubscriptionTypeCollection,
		Inputs:                []domain.NodeInput{{Name: "b", Type: "subscription", Ref: domain.ResourceRef{Kind: "subscription", Name: "B"}, Required: true}},
		RenderCacheTTLSeconds: &ttl,
	}))

	first, err := svc.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{Name: "A", Format: "uri-list"})
	require.NoError(t, err)
	require.False(t, first.Cached)
	second, err := svc.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{Name: "A", Format: "uri-list"})
	require.NoError(t, err)
	require.True(t, second.Cached)

	putLeaf("after.example")
	third, err := svc.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{Name: "A", Format: "uri-list"})
	require.NoError(t, err)
	require.False(t, third.Cached)
	require.Contains(t, string(third.Body), "after.example")
	require.NotContains(t, string(third.Body), "before.example")

	entries, err := resourceStore.List(ctx, "cache/subscription_render/subscriptions")
	require.NoError(t, err)
	files := 0
	for _, entry := range entries {
		if !entry.IsDir {
			files++
		}
	}
	require.Equal(t, 1, files)
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
	putProjectSettings(t, svc, ctx, func(update *domain.SettingsUpdate) {
		update.CacheDefaults = domain.CacheDefaults{FileRenderTTLSeconds: 60}
	})
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

func TestServiceResourceMutationsKeepSemanticEntriesAndDeletesRemoveScope(t *testing.T) {
	ctx := context.Background()
	resultCache := newTestCache()
	svc := service.New(
		service.WithFS(afero.NewMemMapFs()),
		service.WithCache(resultCache),
	)

	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "sub", Type: domain.SubscriptionTypeLocal,
	}))
	require.Empty(t, resultCache.deleted)

	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name: "file", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "body"},
	}))
	require.Empty(t, resultCache.deleted)

	putProjectSettings(t, svc, ctx, nil)
	require.Empty(t, resultCache.deleted)

	require.NoError(t, svc.DeleteSubscription(ctx, "sub"))
	require.ElementsMatch(t, []string{
		"remote_fetch/subscriptions/sub",
		"probe/subscriptions/sub",
		"subscription_traffic/subscriptions/sub",
		"subscription_render/subscriptions/sub",
		"file_render/subscriptions/sub",
	}, resultCache.deleted)

	resultCache.deleted = nil
	require.NoError(t, svc.DeleteFile(ctx, "file"))
	require.ElementsMatch(t, []string{
		"remote_fetch/files/file",
		"probe/files/file",
		"subscription_traffic/files/file",
		"subscription_render/files/file",
		"file_render/files/file",
	}, resultCache.deleted)
}
