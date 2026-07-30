package service_test

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func TestServiceSettingsSaveAppliesDynamicGroupsAndDefersStartupGroups(t *testing.T) {
	svc := newProjectSettingsService()
	before, err := svc.GetSettings(context.Background())
	require.NoError(t, err)
	update := settingsUpdate(before.Settings)
	update.HTTP.Listen = "127.0.0.1:2237"
	update.Appearance.ThemeMode = "light"
	update.Subscriptions.AutoLoadTraffic = true
	update.RemoteDefaults.TimeoutMS = 9000

	after, err := svc.PutSettings(context.Background(), update)

	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:2237", after.Settings.HTTP.Listen)
	require.Equal(t, "127.0.0.1:1137", after.Effective.HTTP.Listen)
	require.Equal(t, "light", after.Effective.Appearance.ThemeMode)
	require.True(t, after.Effective.Subscriptions.AutoLoadTraffic)
	require.Equal(t, 9000, after.Effective.RemoteDefaults.TimeoutMS)
	require.Equal(t, []string{"http.listen"}, after.RestartRequired)
}

func TestServiceSettingsReportsOverridesWithoutRestartDuplicates(t *testing.T) {
	stored := projectsettings.Default()
	effective := stored
	effective.HTTP.Listen = "127.0.0.1:3237"
	svc := newProjectSettingsServiceWithState(stored, effective, map[string]string{
		"http.listen": "environment",
	})
	update := settingsUpdate(projectsettings.View(stored))
	update.HTTP.Listen = "127.0.0.1:2237"

	after, err := svc.PutSettings(context.Background(), update)

	require.NoError(t, err)
	require.Equal(t, map[string]string{"http.listen": "environment"}, after.Overrides)
	require.Empty(t, after.RestartRequired)
	require.Equal(t, "127.0.0.1:3237", after.Effective.HTTP.Listen)
}

func newProjectSettingsService() *service.Service {
	value := projectsettings.Default()
	return newProjectSettingsServiceWithState(value, value, nil)
}

func newProjectSettingsServiceWithState(stored, effective domain.Settings, overrides map[string]string) *service.Service {
	raw := store.NewFSStore(afero.NewMemMapFs())
	coordinator := store.Coordinate(raw)
	repository := store.NewSettingsStore(coordinator)
	return service.New(
		service.WithStore(coordinator),
		service.WithProjectSettings(repository, stored, effective, overrides),
	)
}

func settingsUpdate(value domain.SettingsView) domain.SettingsUpdate {
	return domain.SettingsUpdate{
		SchemaVersion: value.SchemaVersion,
		HTTP: domain.HTTPSettingsUpdate{
			Listen:        value.HTTP.Listen,
			TokenRequired: value.HTTP.TokenRequired,
		},
		MCP:            value.MCP,
		WebUI:          value.WebUI,
		Log:            value.Log,
		RemoteDefaults: value.RemoteDefaults,
		ProbeDefaults:  value.ProbeDefaults,
		CacheDefaults:  value.CacheDefaults,
		Appearance:     value.Appearance,
		Subscriptions:  value.Subscriptions,
	}
}
