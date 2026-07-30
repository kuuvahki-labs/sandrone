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
	require.Equal(t, "dark", got.Appearance.ThemeMode)
	require.Equal(t, "auto", got.Appearance.Locale)
	require.False(t, got.Subscriptions.AutoLoadTraffic)
	require.Equal(t, 60, got.CacheDefaults.SubscriptionTrafficTTLSeconds)
}

func TestStoredAndPublicSettingsOmitStartupAuthenticationAndMCPTransport(t *testing.T) {
	value := settings.Default()

	stored, err := json.Marshal(value)
	require.NoError(t, err)
	public, err := json.Marshal(settings.View(value))
	require.NoError(t, err)

	for _, body := range [][]byte{stored, public} {
		require.NotContains(t, string(body), `"token"`)
		require.NotContains(t, string(body), `"token_required"`)
		require.NotContains(t, string(body), `"token_configured"`)
		require.NotContains(t, string(body), `"transport"`)
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
		"webui": {"static_dir": ""},
		"log": {"level": "warn"}
	}`))
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:2237", value.HTTP.Listen)
	require.Equal(t, "/agent", value.MCP.Path)
	require.True(t, value.MCP.AllowManagementTools)

	body, err := json.Marshal(value)
	require.NoError(t, err)
	require.NotContains(t, string(body), "legacy-secret")
	require.NotContains(t, string(body), `"token_required"`)
	require.NotContains(t, string(body), `"transport"`)
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
