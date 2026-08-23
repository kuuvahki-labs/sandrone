package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
)

func putProjectSettings(t *testing.T, svc *service.Service, ctx context.Context, mutate func(*domain.SettingsUpdate)) {
	t.Helper()
	value := projectsettings.Default()
	update := domain.SettingsUpdate{
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
	if mutate != nil {
		mutate(&update)
	}
	_, err := svc.PutSettings(ctx, update)
	require.NoError(t, err)
}
