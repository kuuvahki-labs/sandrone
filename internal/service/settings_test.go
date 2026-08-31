package service_test

import (
	"context"
	"encoding/json"
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
	update.ScriptDefaults.TimeoutMS = 3500

	after, err := svc.PutSettings(context.Background(), update)

	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:2237", after.Settings.HTTP.Listen)
	require.Equal(t, "127.0.0.1:1137", after.Effective.HTTP.Listen)
	require.Equal(t, "light", after.Effective.Appearance.ThemeMode)
	require.True(t, after.Effective.Subscriptions.AutoLoadTraffic)
	require.Equal(t, 9000, after.Effective.RemoteDefaults.TimeoutMS)
	require.Equal(t, 3500, after.Effective.ScriptDefaults.TimeoutMS)
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

func TestServiceSettingsCanClearRemoteUserAgent(t *testing.T) {
	svc := newProjectSettingsService()
	before, err := svc.GetSettings(context.Background())
	require.NoError(t, err)

	custom := settingsUpdate(before.Settings)
	custom.RemoteDefaults.UserAgent = "Sandrone Custom"
	stored, err := svc.PutSettings(context.Background(), custom)
	require.NoError(t, err)
	require.Equal(t, "Sandrone Custom", stored.Settings.RemoteDefaults.UserAgent)
	require.Equal(t, "Sandrone Custom", stored.Effective.RemoteDefaults.UserAgent)

	cleared := settingsUpdate(stored.Settings)
	cleared.RemoteDefaults.UserAgent = ""
	after, err := svc.PutSettings(context.Background(), cleared)
	require.NoError(t, err)
	require.Empty(t, after.Settings.RemoteDefaults.UserAgent)
	require.Empty(t, after.Effective.RemoteDefaults.UserAgent)

	readBack, err := svc.GetSettings(context.Background())
	require.NoError(t, err)
	require.Empty(t, readBack.Settings.RemoteDefaults.UserAgent)
	require.Empty(t, readBack.Effective.RemoteDefaults.UserAgent)
}

func TestServiceSettingsPersistsIgnoredWarnings(t *testing.T) {
	svc := newProjectSettingsService()
	before, err := svc.GetSettings(t.Context())
	require.NoError(t, err)
	update := settingsUpdate(before.Settings)
	update.Subscriptions.IgnoredWarnings = []domain.IgnoredWarning{{
		Code:   " parse_unknown_field ",
		Field:  " uri.query.mode ",
		Source: " uri-list ",
	}}

	after, err := svc.PutSettings(t.Context(), update)

	require.NoError(t, err)
	want := []domain.IgnoredWarning{{
		Code:   "parse_unknown_field",
		Field:  "uri.query.mode",
		Source: "uri-list",
	}}
	require.Equal(t, want, after.Settings.Subscriptions.IgnoredWarnings)
	require.Equal(t, want, after.Effective.Subscriptions.IgnoredWarnings)
	readBack, err := svc.GetSettings(t.Context())
	require.NoError(t, err)
	require.Equal(t, want, readBack.Settings.Subscriptions.IgnoredWarnings)
}

func TestServiceSettingsRejectsDuplicateIgnoredWarnings(t *testing.T) {
	svc := newProjectSettingsService()
	before, err := svc.GetSettings(t.Context())
	require.NoError(t, err)
	update := settingsUpdate(before.Settings)
	ignored := domain.IgnoredWarning{Code: "probe_timeout"}
	update.Subscriptions.IgnoredWarnings = []domain.IgnoredWarning{ignored, ignored}

	_, err = svc.PutSettings(t.Context(), update)

	require.ErrorContains(t, err, "duplicate ignored warning")
}

func TestServiceSettingsApplyScriptTimeoutToSubsequentProcessors(t *testing.T) {
	svc := newProjectSettingsService()
	before, err := svc.GetSettings(context.Background())
	require.NoError(t, err)
	update := settingsUpdate(before.Settings)
	update.ScriptDefaults.TimeoutMS = 5
	_, err = svc.PutSettings(context.Background(), update)
	require.NoError(t, err)

	source, err := json.Marshal(map[string]any{
		"type": "inline",
		"content": `function main(input) {
  var end = Date.now() + 30;
  while (Date.now() < end) {}
  return input;
}`,
	})
	require.NoError(t, err)
	spec := domain.ProcessorSpec{
		Type:   "script",
		Stage:  domain.StageNodes,
		Params: map[string]json.RawMessage{"source": source},
	}
	input := domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "a", Type: domain.NodeTypeShadowsocks}}}

	proc, err := svc.Registry().BuildNode(spec)
	require.NoError(t, err)
	_, err = proc.ApplyNodes(context.Background(), input)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeScriptTimeout))

	update.ScriptDefaults.TimeoutMS = 200
	_, err = svc.PutSettings(context.Background(), update)
	require.NoError(t, err)
	proc, err = svc.Registry().BuildNode(spec)
	require.NoError(t, err)
	_, err = proc.ApplyNodes(context.Background(), input)
	require.NoError(t, err)
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
