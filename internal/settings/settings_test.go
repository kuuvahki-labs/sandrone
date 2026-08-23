package settings_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/settings"
)

func TestDefaultSettingsContainsWholeProject(t *testing.T) {
	got := settings.Default()

	require.Equal(t, 1, got.SchemaVersion)
	require.Equal(t, "127.0.0.1:1137", got.HTTP.Listen)
	require.Equal(t, "/mcp", got.MCP.Path)
	require.Equal(t, 1<<20, got.MCP.MaxOutputBytes)
	require.Equal(t, "info", got.Log.Level)
	require.Empty(t, got.RemoteDefaults.UserAgent)
	require.Equal(t, "dark", got.Appearance.ThemeMode)
	require.Equal(t, "auto", got.Appearance.Locale)
	require.False(t, got.Subscriptions.AutoLoadTraffic)
	require.Zero(t, got.CacheDefaults.ProbeTTLSeconds)
	require.Equal(t, 60, got.CacheDefaults.SubscriptionTrafficTTLSeconds)
	require.Equal(t, "https://cp.cloudflare.com", got.ProbeDefaults.URL)
	require.Equal(t, 2000, got.ScriptDefaults.TimeoutMS)
	require.False(t, got.ScheduledRefresh.Enabled)
	require.Equal(t, "@every 10m", got.ScheduledRefresh.Schedule)
	require.Empty(t, got.ScheduledRefresh.Targets)
}

func TestStoredAndPublicSettingsOmitRemovedStartupFields(t *testing.T) {
	value := settings.Default()

	stored, err := json.Marshal(value)
	require.NoError(t, err)
	public, err := json.Marshal(settings.View(value))
	require.NoError(t, err)

	for _, body := range [][]byte{stored, public} {
		require.NotContains(t, string(body), `"user_agent"`)
		require.NotContains(t, string(body), `"token"`)
		require.NotContains(t, string(body), `"token_required"`)
		require.NotContains(t, string(body), `"token_configured"`)
		require.NotContains(t, string(body), `"transport"`)
		require.NotContains(t, string(body), `"allow_management_tools"`)
		require.NotContains(t, string(body), `"webui"`)
	}
}

func TestDecodeIgnoresUnknownProjectFields(t *testing.T) {
	body, err := json.Marshal(settings.Default())
	require.NoError(t, err)

	var stored map[string]any
	require.NoError(t, json.Unmarshal(body, &stored))
	stored["future"] = true
	probeDefaults := stored["probe_defaults"].(map[string]any)
	probeDefaults["cache_ttl_seconds"] = 60
	body, err = json.Marshal(stored)
	require.NoError(t, err)

	got, err := settings.Decode(body)
	require.NoError(t, err)
	require.Zero(t, got.CacheDefaults.ProbeTTLSeconds)
}

func TestNormalizePreservesEmptyAndExplicitRemoteUserAgents(t *testing.T) {
	for _, userAgent := range []string{"", "Sandrone Client", "sandrone/0"} {
		t.Run(userAgent, func(t *testing.T) {
			value := settings.Default()
			value.RemoteDefaults.UserAgent = userAgent

			got, err := settings.Normalize(value)

			require.NoError(t, err)
			require.Equal(t, userAgent, got.RemoteDefaults.UserAgent)
		})
	}
}

func TestDecodeMigratesLegacyStartupAuthenticationAndMCPTransport(t *testing.T) {
	value, err := settings.Decode([]byte(`{
		"schema_version": 1,
		"http": {
			"listen": "127.0.0.1:2237",
			"token": "legacy-secret",
			"token_required": true
		},
		"mcp": {
			"transport": "stdio",
			"path": "/agent",
			"allow_management_tools": true,
			"max_output_bytes": 2048
		},
		"webui": {"static_dir": "/legacy/static"},
		"log": {"level": "warn"}
	}`))
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:2237", value.HTTP.Listen)
	require.Equal(t, "/agent", value.MCP.Path)
	require.Equal(t, 2000, value.ScriptDefaults.TimeoutMS)

	body, err := json.Marshal(value)
	require.NoError(t, err)
	require.NotContains(t, string(body), "legacy-secret")
	require.NotContains(t, string(body), `"token_required"`)
	require.NotContains(t, string(body), `"transport"`)
	require.NotContains(t, string(body), `"allow_management_tools"`)
	require.NotContains(t, string(body), `"webui"`)
}

func TestNormalizeRejectsInvalidProjectFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.Settings)
	}{
		{
			name: "schema version",
			mutate: func(value *domain.Settings) {
				value.SchemaVersion = 2
			},
		},
		{
			name: "theme mode",
			mutate: func(value *domain.Settings) {
				value.Appearance.ThemeMode = "sepia"
			},
		},
		{
			name: "locale",
			mutate: func(value *domain.Settings) {
				value.Appearance.Locale = "fr-FR"
			},
		},
		{
			name: "script timeout",
			mutate: func(value *domain.Settings) {
				value.ScriptDefaults.TimeoutMS = -1
			},
		},
		{
			name: "probe ttl",
			mutate: func(value *domain.Settings) {
				value.CacheDefaults.ProbeTTLSeconds = -1
			},
		},
		{
			name: "subscription traffic ttl",
			mutate: func(value *domain.Settings) {
				value.CacheDefaults.SubscriptionTrafficTTLSeconds = -1
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value := settings.Default()
			tc.mutate(&value)

			_, err := settings.Normalize(value)

			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
		})
	}
}

func TestNormalizeDefaultsMissingScriptTimeout(t *testing.T) {
	value := settings.Default()
	value.ScriptDefaults.TimeoutMS = 0

	got, err := settings.Normalize(value)

	require.NoError(t, err)
	require.Equal(t, 2000, got.ScriptDefaults.TimeoutMS)
}

func TestNormalizeRejectsMCPPathsThatBypassSharedAuthentication(t *testing.T) {
	for _, path := range []string{"/", "/healthz", "/version", "/convert", "/s", "/s/share"} {
		t.Run(path, func(t *testing.T) {
			value := settings.Default()
			value.MCP.Path = path

			_, err := settings.Normalize(value)

			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
			require.Contains(t, err.Error(), "conflicts with public route")
		})
	}
}

func TestNormalizeScheduledRefresh(t *testing.T) {
	value := settings.Default()
	value.ScheduledRefresh = domain.ScheduledRefreshSettings{
		Enabled:  true,
		Schedule: " 0 * * * * ",
		Targets: []domain.ScheduledRefreshTarget{
			{Kind: "subscription", Name: " primary "},
			{Kind: "file", Name: "client.yaml"},
		},
	}

	got, err := settings.Normalize(value)

	require.NoError(t, err)
	require.Equal(t, "0 * * * *", got.ScheduledRefresh.Schedule)
	require.Equal(t, []domain.ScheduledRefreshTarget{
		{Kind: "subscription", Name: "primary"},
		{Kind: "file", Name: "client.yaml"},
	}, got.ScheduledRefresh.Targets)
}

func TestNormalizeRejectsInvalidScheduledRefresh(t *testing.T) {
	tests := []struct {
		name     string
		settings domain.ScheduledRefreshSettings
	}{
		{name: "seconds field", settings: domain.ScheduledRefreshSettings{Schedule: "0 0 * * * *"}},
		{name: "short interval", settings: domain.ScheduledRefreshSettings{Schedule: "@every 59s"}},
		{name: "cron timezone", settings: domain.ScheduledRefreshSettings{Schedule: "CRON_TZ=UTC 0 * * * *"}},
		{name: "timezone", settings: domain.ScheduledRefreshSettings{Schedule: "TZ=UTC 0 * * * *"}},
		{name: "unsupported target", settings: domain.ScheduledRefreshSettings{Schedule: "@daily", Targets: []domain.ScheduledRefreshTarget{{Kind: "share", Name: "one"}}}},
		{name: "empty target name", settings: domain.ScheduledRefreshSettings{Schedule: "@daily", Targets: []domain.ScheduledRefreshTarget{{Kind: "file", Name: " "}}}},
		{name: "duplicate target", settings: domain.ScheduledRefreshSettings{Schedule: "@daily", Targets: []domain.ScheduledRefreshTarget{{Kind: "file", Name: "one"}, {Kind: "file", Name: "one"}}}},
		{name: "enabled without targets", settings: domain.ScheduledRefreshSettings{Enabled: true, Schedule: "@daily"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value := settings.Default()
			value.ScheduledRefresh = tc.settings

			_, err := settings.Normalize(value)

			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
		})
	}
}
