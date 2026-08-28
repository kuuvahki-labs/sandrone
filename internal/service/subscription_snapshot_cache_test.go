package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestSubscriptionSnapshotCacheIsSharedAcrossPreviewRenderAndTypedFile(t *testing.T) {
	ctx := context.Background()
	processorCalls := 0
	svc := service.New(
		service.WithFS(afero.NewMemMapFs()),
		service.WithProcessor(func(registry *processor.Registry) {
			registry.RegisterNode("count_snapshot", func(domain.ProcessorSpec) (domain.NodeProcessor, error) {
				return snapshotCountingProcessor{calls: &processorCalls}, nil
			})
		}),
	)
	putProjectSettings(t, svc, ctx, func(update *domain.SettingsUpdate) {
		update.CacheDefaults.SubscriptionSnapshotTTLSeconds = 60
	})
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "shared",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node",
		Processors: []domain.ProcessorSpec{{
			Type: "count_snapshot", Stage: domain.StageNodes,
		}},
	}))

	preview, err := svc.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "shared"})
	require.NoError(t, err)
	require.Equal(t, "miss", preview.SnapshotCacheStatus)
	require.Equal(t, 1, processorCalls)

	for _, format := range []string{"mihomo-proxies", "sing-box-outbounds", "shadowrocket-proxies"} {
		_, err := svc.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{Name: "shared", Format: format})
		require.NoError(t, err)
	}
	require.Equal(t, 1, processorCalls)
	produced, err := svc.ProduceSubscription(ctx, "shared", domain.ScriptProduceOptions{})
	require.NoError(t, err)
	require.Len(t, produced.Nodes, 1)
	require.Equal(t, 1, processorCalls)

	for _, spec := range []domain.FileSpec{
		{Name: "mihomo.yaml", Kind: domain.FileKindMihomo, Config: &domain.FileConfig{Subscriptions: []string{"shared"}}},
		{Name: "sing-box.json", Kind: domain.FileKindSingBox, Config: &domain.FileConfig{Subscriptions: []string{"shared"}}},
		{Name: "shadowrocket.conf", Kind: domain.FileKindShadowrocket, Config: &domain.FileConfig{Subscriptions: []string{"shared"}}},
	} {
		_, err := svc.GetFile(ctx, domain.FileRequest{Spec: &spec})
		require.NoError(t, err)
	}
	require.Equal(t, 1, processorCalls)

	cached, err := svc.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "shared"})
	require.NoError(t, err)
	require.Equal(t, "hit", cached.SnapshotCacheStatus)
	require.Equal(t, preview.StatusCounts, cached.StatusCounts)
	require.Equal(t, preview.Nodes[0].Status, cached.Nodes[0].Status)
	require.Equal(t, preview.Nodes[0].After.Name, cached.Nodes[0].After.Name)
	require.Equal(t, 1, processorCalls)

	refreshed, err := svc.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "shared", Refresh: true})
	require.NoError(t, err)
	require.Equal(t, "bypass", refreshed.SnapshotCacheStatus)
	require.Equal(t, 2, processorCalls)
}

func TestSubscriptionSnapshotCacheIdentityIncludesRequestAndNotRenderTarget(t *testing.T) {
	ctx := context.Background()
	processorCalls := 0
	svc := service.New(
		service.WithFS(afero.NewMemMapFs()),
		service.WithProcessor(func(registry *processor.Registry) {
			registry.RegisterNode("count_snapshot", func(domain.ProcessorSpec) (domain.NodeProcessor, error) {
				return snapshotCountingProcessor{calls: &processorCalls}, nil
			})
		}),
	)
	putProjectSettings(t, svc, ctx, func(update *domain.SettingsUpdate) {
		update.CacheDefaults.SubscriptionSnapshotTTLSeconds = 60
	})
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "variant", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content:    "ss://aes-128-gcm:secret@example.com:8388#node",
		Processors: []domain.ProcessorSpec{{Type: "count_snapshot", Stage: domain.StageNodes}},
	}))

	_, err := svc.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{
		Name: "variant", Format: "mihomo-proxies", Request: domain.RequestInfo{Args: map[string]string{"region": "hk"}},
	})
	require.NoError(t, err)
	_, err = svc.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{
		Name: "variant", Format: "sing-box-outbounds", Request: domain.RequestInfo{Args: map[string]string{"region": "hk"}},
	})
	require.NoError(t, err)
	require.Equal(t, 1, processorCalls)

	_, err = svc.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{
		Name: "variant", Format: "uri-list", Request: domain.RequestInfo{Args: map[string]string{"region": "us"}},
	})
	require.NoError(t, err)
	require.Equal(t, 2, processorCalls)
}

func TestSubscriptionSnapshotCacheInvalidatesWhenDependencyChanges(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	putProjectSettings(t, svc, ctx, func(update *domain.SettingsUpdate) {
		update.CacheDefaults.SubscriptionSnapshotTTLSeconds = 60
	})
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "child", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#first",
	}))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "parent", Type: domain.SubscriptionTypeCollection,
		Inputs: []domain.NodeInput{{Type: "subscription", Ref: domain.ResourceRef{Kind: "subscription", Name: "child"}}},
	}))

	first, err := svc.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "parent"})
	require.NoError(t, err)
	require.Equal(t, "miss", first.SnapshotCacheStatus)
	require.Equal(t, "first", first.Nodes[0].After.Name)

	cached, err := svc.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "parent"})
	require.NoError(t, err)
	require.Equal(t, "hit", cached.SnapshotCacheStatus)

	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "child", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#second",
	}))
	afterUpdate, err := svc.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "parent"})
	require.NoError(t, err)
	require.Equal(t, "miss", afterUpdate.SnapshotCacheStatus)
	require.Equal(t, "second", afterUpdate.Nodes[0].After.Name)
}

func TestSubscriptionSnapshotCacheResourceOverrideCanDisableDefault(t *testing.T) {
	ctx := context.Background()
	disabled := 0
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	putProjectSettings(t, svc, ctx, func(update *domain.SettingsUpdate) {
		update.CacheDefaults.SubscriptionSnapshotTTLSeconds = 60
	})
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "disabled", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content:            "ss://aes-128-gcm:secret@example.com:8388#node",
		SnapshotTTLSeconds: &disabled,
	}))

	for range 2 {
		preview, err := svc.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "disabled"})
		require.NoError(t, err)
		require.Equal(t, "disabled", preview.SnapshotCacheStatus)
	}
}

func TestSubscriptionSnapshotTTLCanOutliveRemoteFetchTTL(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	body := "ss://aes-128-gcm:secret@example.com:8388#first"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	cache := newTestCache()
	cache.now = func() time.Time { return now }
	svc := service.New(
		service.WithFS(afero.NewMemMapFs()),
		service.WithCache(cache),
		service.WithClock(func() time.Time { return now }),
	)
	putProjectSettings(t, svc, ctx, func(update *domain.SettingsUpdate) {
		update.CacheDefaults.RemoteFetchTTLSeconds = 1
		update.CacheDefaults.SubscriptionSnapshotTTLSeconds = 10
	})
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "remote", Type: domain.SubscriptionTypeRemote, Format: "uri-list",
		Remote: &domain.RemoteInput{URL: server.URL},
	}))

	first, err := svc.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "remote"})
	require.NoError(t, err)
	require.Equal(t, "first", first.Nodes[0].After.Name)
	require.Equal(t, 1, requests)

	body = "ss://aes-128-gcm:secret@example.com:8388#second"
	now = now.Add(2 * time.Second)
	frozen, err := svc.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "remote"})
	require.NoError(t, err)
	require.Equal(t, "hit", frozen.SnapshotCacheStatus)
	require.Equal(t, "first", frozen.Nodes[0].After.Name)
	require.Equal(t, 1, requests)

	now = now.Add(9 * time.Second)
	fresh, err := svc.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "remote"})
	require.NoError(t, err)
	require.Equal(t, "miss", fresh.SnapshotCacheStatus)
	require.Equal(t, "second", fresh.Nodes[0].After.Name)
	require.Equal(t, 2, requests)
}

type snapshotCountingProcessor struct {
	calls *int
}

func (p snapshotCountingProcessor) Name() string { return "count_snapshot" }

func (p snapshotCountingProcessor) ApplyNodes(_ context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	(*p.calls)++
	return domain.NodeProcessOutput{Nodes: append([]domain.NodeIR{}, in.Nodes...)}, nil
}
