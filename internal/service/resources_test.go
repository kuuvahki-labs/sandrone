package service_test

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServicePutFileAndGetFileByName(t *testing.T) {
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	spec := domain.FileSpec{
		Name:   "stored/base.yaml",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "key: value\n"},
	}
	require.NoError(t, svc.PutFile(context.Background(), spec))

	result, err := svc.GetFile(context.Background(), domain.FileRequest{Name: "stored/base.yaml"})
	require.NoError(t, err)
	require.Equal(t, "key: value\n", string(result.File.Content))
}

func TestServiceGetFileSourceReturnsStoredAndBuiltinContentBeforeCompilation(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:   "stored/base.yaml",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "custom: true\n"},
	}))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name: "default.json",
		Kind: domain.FileKindSingBox,
	}))

	stored, err := svc.GetFileSource(ctx, "stored/base.yaml")
	require.NoError(t, err)
	require.Equal(t, "custom: true\n", string(stored.Content))
	require.Equal(t, "static", stored.Kind)
	require.Equal(t, "text/plain; charset=utf-8", stored.MediaType)

	builtin, err := svc.GetFileSource(ctx, "default.json")
	require.NoError(t, err)
	require.Equal(t, "sing-box", builtin.Kind)
	require.Equal(t, "application/json", builtin.MediaType)
	require.JSONEq(t, `{
  "log": { "level": "info" },
  "inbounds": [],
  "outbounds": [],
  "route": {
    "rule_set": [],
    "rules": []
  }
}`, string(builtin.Content))
}

func TestServiceGetFileSourceReturnsShadowrocketBuiltinBase(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name: "default.conf",
		Kind: domain.FileKindShadowrocket,
	}))

	builtin, err := svc.GetFileSource(ctx, "default.conf")

	require.NoError(t, err)
	require.Equal(t, "shadowrocket", builtin.Kind)
	require.Equal(t, "text/plain; charset=utf-8", builtin.MediaType)
	require.Equal(t, "[General]\n", string(builtin.Content))
}

func TestServiceListResources(t *testing.T) {
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(context.Background(), domain.Subscription{Name: "sub", Type: domain.SubscriptionTypeLocal, Format: "uri-list"}))
	require.NoError(t, svc.PutFile(context.Background(), domain.FileSpec{Name: "mihomo.yaml", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "proxies: []\n"}}))

	subscriptions, err := svc.ListSubscriptions(context.Background())
	require.NoError(t, err)
	require.Equal(t, []domain.ResourceSummary{{Kind: "subscription", Type: "local", Name: "sub", Format: "uri-list", Size: subscriptions.Items[0].Size}}, subscriptions.Items)

	files, err := svc.ListFiles(context.Background())
	require.NoError(t, err)
	require.Len(t, files.Items, 1)
	require.Equal(t, "inline", files.Items[0].Type)
	require.Equal(t, "static", files.Items[0].Target)
}

func TestServiceListFilesExposesTypedConfigKind(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:   "default.yaml",
		Kind:   domain.FileKindMihomo,
		Source: domain.FileSource{},
		Config: &domain.FileConfig{Subscriptions: []string{"default"}},
	}))

	files, err := svc.ListFiles(ctx)

	require.NoError(t, err)
	require.Len(t, files.Items, 1)
	require.Empty(t, files.Items[0].Type)
	require.Equal(t, "mihomo", files.Items[0].Target)
}

func TestServiceDeleteResourcesUpdatesResourceLists(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))

	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{Name: "remote/provider", Type: domain.SubscriptionTypeLocal, Format: "uri-list"}))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{Name: "live/default", Type: domain.SubscriptionTypeCollection}))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{Name: "stored/base.yaml", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "body"}}))

	assertResourceCounts := func(subscriptions, files int) {
		subscriptionList, err := svc.ListSubscriptions(ctx)
		require.NoError(t, err)
		require.Len(t, subscriptionList.Items, subscriptions)
		fileList, err := svc.ListFiles(ctx)
		require.NoError(t, err)
		require.Len(t, fileList.Items, files)
	}

	assertResourceCounts(2, 1)

	require.NoError(t, svc.DeleteSubscription(ctx, "remote/provider"))
	assertResourceCounts(1, 1)

	require.NoError(t, svc.DeleteSubscription(ctx, "live/default"))
	assertResourceCounts(0, 1)

	require.NoError(t, svc.DeleteFile(ctx, "stored/base.yaml"))
	assertResourceCounts(0, 0)

	subscriptions, err := svc.ListSubscriptions(ctx)
	require.NoError(t, err)
	require.Empty(t, subscriptions.Items)
	files, err := svc.ListFiles(ctx)
	require.NoError(t, err)
	require.Empty(t, files.Items)

	_, err = svc.GetSubscription(ctx, "remote/provider")
	require.True(t, os.IsNotExist(err))
	_, err = svc.GetSubscription(ctx, "live/default")
	require.True(t, os.IsNotExist(err))
	_, err = svc.GetFileSpec(ctx, "stored/base.yaml")
	require.True(t, os.IsNotExist(err))

	assertResourceCounts(0, 0)
}

func TestServiceStoreUnavailableErrors(t *testing.T) {
	svc := service.New()
	require.Error(t, svc.PutSubscription(context.Background(), domain.Subscription{Name: "sub", Type: domain.SubscriptionTypeLocal}))
	require.Error(t, svc.PutFile(context.Background(), domain.FileSpec{Name: "file", Kind: domain.FileKindStatic}))
}

func TestServiceRejectsNegativeResourceRenderCacheTTL(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	negative := -1

	err := svc.PutSubscription(ctx, domain.Subscription{
		Name:                  "sub",
		Type:                  domain.SubscriptionTypeLocal,
		RenderCacheTTLSeconds: &negative,
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)

	err = svc.PutFile(ctx, domain.FileSpec{
		Name:                  "file",
		Kind:                  domain.FileKindStatic,
		Source:                domain.FileSource{Type: "inline", Content: "body"},
		RenderCacheTTLSeconds: &negative,
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
}
