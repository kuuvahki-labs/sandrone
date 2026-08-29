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
	"github.com/kuuvahki-labs/sandrone/internal/probe"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/service"
	"github.com/kuuvahki-labs/sandrone/internal/store"
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

func TestSubscriptionSnapshotCacheSharesProbedRefreshWithDisabledRuntime(t *testing.T) {
	ctx := t.Context()
	sharedStore := store.Coordinate(store.NewFSStore(afero.NewMemMapFs()))
	probeCalls := 0
	producer := service.New(
		service.WithStore(sharedStore),
		service.WithProbeEngine(fakeProbeEngine{probe: func(_ context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, _ ...probe.Payload) (*domain.ProbeResult, error) {
			probeCalls++
			results := make([]domain.NodeProbeResult, len(nodes))
			for index, node := range nodes {
				results[index] = domain.NodeProbeResult{
					NodeName: node.Name, Method: string(req.Method),
					Alive: index == 0, DurationMS: index + 1,
					CheckedAt: time.Date(2026, 8, 29, 1, 2, 3, 0, time.UTC),
				}
			}
			return &domain.ProbeResult{Results: results}, nil
		}}),
	)
	putProjectSettings(t, producer, ctx, func(update *domain.SettingsUpdate) {
		update.CacheDefaults.SubscriptionSnapshotTTLSeconds = 60
	})
	probeProcessor := domain.ProcessorSpec{
		Type: "probe", Stage: domain.StageNodes,
		Params: params(t, map[string]any{"fail_mode": "drop"}),
	}
	putSubscription := func(name string) {
		t.Helper()
		require.NoError(t, producer.PutSubscription(ctx, domain.Subscription{
			Name: name, Type: domain.SubscriptionTypeLocal, Format: "uri-list",
			Content: "ss://aes-128-gcm:secret@example.com:8388#alive\n" +
				"ss://aes-128-gcm:secret@example.net:8388#failed",
			Processors: []domain.ProcessorSpec{probeProcessor},
		}))
	}
	putSubscription("scheduled")
	putSubscription("cold")

	consumer := service.New(
		service.WithStore(sharedStore),
		service.WithProbeEngine(probe.NewDisabled()),
		service.WithSchedulerEnabled(false),
	)
	require.NoError(t, consumer.ReloadSettings(ctx))

	refreshed, err := producer.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{
		Name: "scheduled", Refresh: true,
	})
	require.NoError(t, err)
	require.Equal(t, "bypass", refreshed.SnapshotCacheStatus)
	require.Equal(t, 1, refreshed.AfterCount)
	require.Equal(t, 1, probeCalls)

	cached, err := consumer.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "scheduled"})
	require.NoError(t, err)
	require.Equal(t, "hit", cached.SnapshotCacheStatus)
	require.Equal(t, 1, cached.AfterCount)
	require.NotContains(t, warningCodeSet(cached.Report.Warnings), "probe_skipped_backend_unavailable")
	require.Equal(t, 1, probeCalls)

	degraded, err := consumer.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "cold"})
	require.NoError(t, err)
	require.Equal(t, "miss", degraded.SnapshotCacheStatus)
	require.Equal(t, 2, degraded.AfterCount)
	requireWarningCode(t, degraded.Report.Warnings, "probe_skipped_backend_unavailable")

	materialized, err := producer.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "cold"})
	require.NoError(t, err)
	require.Equal(t, "miss", materialized.SnapshotCacheStatus)
	require.Equal(t, 1, materialized.AfterCount)
	require.Equal(t, 2, probeCalls)

	cached, err = consumer.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "cold"})
	require.NoError(t, err)
	require.Equal(t, "hit", cached.SnapshotCacheStatus)
	require.Equal(t, 1, cached.AfterCount)
	require.NotContains(t, warningCodeSet(cached.Report.Warnings), "probe_skipped_backend_unavailable")
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

func warningCodeSet(warnings []domain.Warning) map[string]struct{} {
	codes := make(map[string]struct{}, len(warnings))
	for _, warning := range warnings {
		codes[warning.Code] = struct{}{}
	}
	return codes
}
