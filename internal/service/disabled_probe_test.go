package service_test

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceDisabledProbeFailsBeforeResolvingInput(t *testing.T) {
	svc := service.New(service.WithProbeEngine(probe.NewDisabled()))
	_, err := svc.Probe(context.Background(), domain.ProbeRequest{
		Input: domain.NodeInput{Ref: domain.ResourceRef{Kind: "subscription", Name: "missing-resource"}},
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProbeBackendUnavailable))
}

func TestDisabledProbeAndSchedulerUICapabilities(t *testing.T) {
	svc := service.New(
		service.WithProbeEngine(probe.NewDisabled()),
		service.WithSchedulerEnabled(false),
	)
	result, err := svc.ListUICapabilities(context.Background())
	require.NoError(t, err)
	for _, feature := range result.Features {
		switch feature.Key {
		case "probe.enabled", "core.mihomo", "core.sing_box", "scheduler.enabled":
			require.False(t, feature.Enabled, feature.Key)
		}
	}
}

func TestImportedProbeProcessorWarnsAndContinuesWithoutBackend(t *testing.T) {
	ctx := t.Context()
	source := service.New(service.WithFS(afero.NewMemMapFs()))
	probeProcessor := domain.ProcessorSpec{
		Type:  "probe",
		Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"annotate":  true,
			"fail_mode": "error",
			"sort":      "duration",
		}),
	}
	require.NoError(t, source.PutSubscription(ctx, domain.Subscription{
		Name: "imported", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content:    "ss://aes-128-gcm:secret@example.com:8388#node",
		Processors: []domain.ProcessorSpec{probeProcessor},
	}))
	putProjectSettings(t, source, ctx, func(update *domain.SettingsUpdate) {
		update.ScheduledRefresh = domain.ScheduledRefreshSettings{
			Enabled: true, Schedule: "@every 10m",
			Targets: []domain.ScheduledRefreshTarget{{Kind: "subscription", Name: "imported"}},
		}
	})
	archive, err := source.ExportBackup(ctx)
	require.NoError(t, err)

	target := service.New(
		service.WithFS(afero.NewMemMapFs()),
		service.WithProbeEngine(probe.NewDisabled()),
		service.WithSchedulerEnabled(false),
	)
	require.NoError(t, target.RestoreBackup(ctx, archive.Body))

	stored, err := target.GetSubscription(ctx, "imported")
	require.NoError(t, err)
	require.Equal(t, []domain.ProcessorSpec{probeProcessor}, stored.Processors)
	settings, err := target.GetSettings(ctx)
	require.NoError(t, err)
	require.True(t, settings.Settings.ScheduledRefresh.Enabled)
	require.False(t, settings.Effective.ScheduledRefresh.Enabled)

	preview, err := target.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{Name: "imported"})
	require.NoError(t, err)
	require.Equal(t, 1, preview.AfterCount)
	requireWarningCode(t, preview.Report.Warnings, "probe_skipped_backend_unavailable")

	rendered, err := target.RenderSubscriptionRequest(ctx, domain.SubscriptionRenderRequest{
		Name: "imported", Format: "mihomo-proxies",
	})
	require.NoError(t, err)
	requireWarningCode(t, rendered.Report.Warnings, "probe_skipped_backend_unavailable")

	file, err := target.GetFile(ctx, domain.FileRequest{Spec: &domain.FileSpec{
		Name: "config.yaml", Kind: domain.FileKindMihomo,
		Config: &domain.FileConfig{
			Subscriptions: []string{"imported"},
			Settings:      completeTypedSettings(t, map[string]any{}),
		},
	}})
	require.NoError(t, err)
	requireWarningCode(t, file.Report.Warnings, "probe_skipped_backend_unavailable")

	converted, err := target.Convert(ctx, domain.ConvertRequest{
		FromFormat: "uri-list", ToFormat: "mihomo-proxies",
		Content:         []byte("ss://aes-128-gcm:secret@example.com:8388#node"),
		ParseProcessors: []domain.ProcessorSpec{probeProcessor},
	})
	require.NoError(t, err)
	requireWarningCode(t, converted.Report.Warnings, "probe_skipped_backend_unavailable")

	validated, err := target.ValidateNodes(ctx, domain.ParseRequest{
		Format: "uri-list", Content: []byte("ss://aes-128-gcm:secret@example.com:8388#node"),
		Processors: []domain.ProcessorSpec{probeProcessor},
	})
	require.NoError(t, err)
	require.True(t, validated.OK)
	requireWarningCode(t, validated.Report.Warnings, "probe_skipped_backend_unavailable")

	diagnosed, err := target.Diagnose(ctx, domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputNodes, Format: "uri-list",
		Content:    []byte("ss://aes-128-gcm:secret@example.com:8388#node"),
		Processors: []domain.ProcessorSpec{probeProcessor},
	})
	require.NoError(t, err)
	require.Len(t, diagnosed.Stages, 2)
	requireWarningCode(t, diagnosed.Stages[1].Warnings, "probe_skipped_backend_unavailable")
}

func requireWarningCode(t *testing.T, warnings []domain.Warning, code string) {
	t.Helper()
	for _, warning := range warnings {
		if warning.Code == code {
			return
		}
	}
	require.Fail(t, "warning code not found", "code=%s warnings=%v", code, warnings)
}
