package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func TestScheduledRefreshContinuesAfterTargetFailure(t *testing.T) {
	svc := New()
	var got []string
	svc.scheduledRefreshTarget = func(_ context.Context, target domain.ScheduledRefreshTarget) error {
		got = append(got, target.Name)
		if target.Name == "broken" {
			return domain.NewError(domain.CodeFileInputNotFound, "missing")
		}
		return nil
	}

	svc.runScheduledRefresh(context.Background(), []domain.ScheduledRefreshTarget{
		{Kind: "file", Name: "first"},
		{Kind: "file", Name: "broken"},
		{Kind: "subscription", Name: "last"},
	}, nil)

	require.Equal(t, []string{"first", "broken", "last"}, got)
	status := svc.ScheduledRefreshStatus(context.Background())
	require.False(t, status.Running)
	require.Equal(t, 2, status.LastSuccessCount)
	require.Equal(t, 1, status.LastFailureCount)
	require.NotNil(t, status.LastStartedAt)
	require.NotNil(t, status.LastCompletedAt)
}

func TestScheduledRefreshTargetSucceedsWhenProbeProcessorIsUnavailable(t *testing.T) {
	svc := New(WithFS(afero.NewMemMapFs()), WithProbeEngine(probe.NewDisabled()))
	require.NoError(t, svc.PutSubscription(t.Context(), domain.Subscription{
		Name: "imported", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node",
		Processors: []domain.ProcessorSpec{{
			Type: "probe", Stage: domain.StageNodes,
		}},
	}))

	svc.runScheduledRefresh(t.Context(), []domain.ScheduledRefreshTarget{{
		Kind: "subscription", Name: "imported",
	}}, nil)

	status := svc.ScheduledRefreshStatus(t.Context())
	require.Equal(t, 1, status.LastSuccessCount)
	require.Zero(t, status.LastFailureCount)
}

func TestScheduledRefreshSkipsOverlappingRun(t *testing.T) {
	now := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.Local)
	svc := New(WithClock(func() time.Time { return now }))
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	svc.scheduledRefreshTarget = func(context.Context, domain.ScheduledRefreshTarget) error {
		once.Do(func() { close(started) })
		<-release
		return nil
	}
	targets := []domain.ScheduledRefreshTarget{{Kind: "file", Name: "one"}}
	schedule, err := cron.ParseStandard("0 * * * *")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		svc.runScheduledRefresh(context.Background(), targets, schedule)
		close(done)
	}()
	<-started
	now = time.Date(2026, time.August, 11, 11, 0, 0, 0, time.Local)

	svc.runScheduledRefresh(context.Background(), targets, schedule)

	status := svc.ScheduledRefreshStatus(context.Background())
	require.True(t, status.Running)
	require.Equal(t, 1, status.SkippedCount)
	require.NotNil(t, status.LastSkippedAt)
	require.Equal(t, time.Date(2026, time.August, 11, 12, 0, 0, 0, time.Local), *status.NextRunAt)
	close(release)
	<-done
}

func TestScheduledRefreshCancellationReachesActiveTarget(t *testing.T) {
	svc := New()
	started := make(chan struct{})
	svc.scheduledRefreshTarget = func(ctx context.Context, _ domain.ScheduledRefreshTarget) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.runScheduledRefresh(ctx, []domain.ScheduledRefreshTarget{{Kind: "file", Name: "one"}}, nil)
		close(done)
	}()
	<-started
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduled refresh did not stop after cancellation")
	}
	require.False(t, svc.ScheduledRefreshStatus(context.Background()).Running)
}

func TestScheduledRefreshSchedulerAppliesDynamicSettings(t *testing.T) {
	fs := afero.NewMemMapFs()
	coordinator := store.Coordinate(store.NewFSStore(fs))
	repository := store.NewSettingsStore(coordinator)
	value := projectsettings.Default()
	svc := New(WithStore(coordinator), WithProjectSettings(repository, value, value, nil))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunScheduledRefresh(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	update := scheduledRefreshSettingsUpdate(value)
	update.ScheduledRefresh = domain.ScheduledRefreshSettings{
		Enabled:  true,
		Schedule: "@every 1m",
		Targets:  []domain.ScheduledRefreshTarget{{Kind: "file", Name: "missing"}},
	}
	_, err := svc.PutSettings(context.Background(), update)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		status := svc.ScheduledRefreshStatus(context.Background())
		return status.Enabled && status.NextRunAt != nil
	}, time.Second, 10*time.Millisecond)

	update.ScheduledRefresh.Enabled = false
	_, err = svc.PutSettings(context.Background(), update)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		status := svc.ScheduledRefreshStatus(context.Background())
		return !status.Enabled && status.NextRunAt == nil
	}, time.Second, 10*time.Millisecond)
}

func TestScheduledRefreshRunsCurrentTargetsImmediately(t *testing.T) {
	fs := afero.NewMemMapFs()
	coordinator := store.Coordinate(store.NewFSStore(fs))
	repository := store.NewSettingsStore(coordinator)
	value := projectsettings.Default()
	svc := New(WithStore(coordinator), WithProjectSettings(repository, value, value, nil))
	refreshed := make(chan domain.ScheduledRefreshTarget, 1)
	release := make(chan struct{})
	svc.scheduledRefreshTarget = func(_ context.Context, target domain.ScheduledRefreshTarget) error {
		refreshed <- target
		<-release
		return nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		svc.RunScheduledRefresh(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	update := scheduledRefreshSettingsUpdate(value)
	update.ScheduledRefresh = domain.ScheduledRefreshSettings{
		Enabled: true, Schedule: "@every 1m",
		Targets: []domain.ScheduledRefreshTarget{{Kind: "subscription", Name: "provider"}},
	}
	_, err := svc.PutSettings(t.Context(), update)
	require.NoError(t, err)
	require.NoError(t, svc.RunScheduledRefreshNow(t.Context()))
	require.True(t, svc.ScheduledRefreshStatus(t.Context()).Running)

	select {
	case target := <-refreshed:
		require.Equal(t, domain.ScheduledRefreshTarget{Kind: "subscription", Name: "provider"}, target)
	case <-time.After(time.Second):
		t.Fatal("immediate scheduled refresh did not start")
	}
	close(release)
	require.Eventually(t, func() bool {
		status := svc.ScheduledRefreshStatus(t.Context())
		return !status.Running && status.LastSuccessCount == 1 && status.LastFailureCount == 0
	}, time.Second, 10*time.Millisecond)
}

func TestScheduledRefreshRunNowRequiresEnabledConfiguration(t *testing.T) {
	svc := New()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		svc.RunScheduledRefresh(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	err := svc.RunScheduledRefreshNow(t.Context())
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
}

func TestScheduledRefreshRunNowRejectsUnavailableScheduler(t *testing.T) {
	err := New(WithSchedulerEnabled(false)).RunScheduledRefreshNow(t.Context())
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeNotImplemented))
}

func TestDisabledSchedulerPreservesStoredSettingsWithoutScheduling(t *testing.T) {
	fs := afero.NewMemMapFs()
	coordinator := store.Coordinate(store.NewFSStore(fs))
	repository := store.NewSettingsStore(coordinator)
	value := projectsettings.Default()
	svc := New(
		WithStore(coordinator),
		WithProjectSettings(repository, value, value, nil),
		WithSchedulerEnabled(false),
	)
	update := scheduledRefreshSettingsUpdate(value)
	update.ScheduledRefresh = domain.ScheduledRefreshSettings{
		Enabled: true, Schedule: "@every 1m",
		Targets: []domain.ScheduledRefreshTarget{{Kind: "file", Name: "imported"}},
	}

	snapshot, err := svc.PutSettings(t.Context(), update)
	require.NoError(t, err)
	require.True(t, snapshot.Settings.ScheduledRefresh.Enabled)
	require.False(t, snapshot.Effective.ScheduledRefresh.Enabled)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		svc.RunScheduledRefresh(ctx)
		close(done)
	}()
	cancel()
	<-done

	status := svc.ScheduledRefreshStatus(t.Context())
	require.False(t, status.Enabled)
	require.Nil(t, status.NextRunAt)
	require.Nil(t, status.LastStartedAt)
}

func TestScheduledRefreshUpdatesNextRun(t *testing.T) {
	svc := New(WithClock(func() time.Time {
		return time.Date(2026, time.August, 11, 10, 0, 0, 0, time.Local)
	}))
	svc.scheduledRefreshTarget = func(context.Context, domain.ScheduledRefreshTarget) error { return nil }
	schedule, err := cron.ParseStandard("0 * * * *")
	require.NoError(t, err)

	svc.runScheduledRefresh(context.Background(), []domain.ScheduledRefreshTarget{{Kind: "file", Name: "one"}}, schedule)

	status := svc.ScheduledRefreshStatus(context.Background())
	require.Equal(t, time.Date(2026, time.August, 11, 11, 0, 0, 0, time.Local), *status.NextRunAt)
}

func scheduledRefreshSettingsUpdate(value domain.Settings) domain.SettingsUpdate {
	return domain.SettingsUpdate{
		SchemaVersion:    value.SchemaVersion,
		HTTP:             value.HTTP,
		MCP:              value.MCP,
		Log:              value.Log,
		RemoteDefaults:   value.RemoteDefaults,
		ProbeDefaults:    value.ProbeDefaults,
		ScriptDefaults:   value.ScriptDefaults,
		CacheDefaults:    value.CacheDefaults,
		Appearance:       value.Appearance,
		Subscriptions:    value.Subscriptions,
		ScheduledRefresh: value.ScheduledRefresh,
	}
}
