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
	require.Equal(t, "stdio", got.MCP.Transport)
	require.Equal(t, "/mcp", got.MCP.Path)
	require.Equal(t, 1<<20, got.MCP.MaxOutputBytes)
	require.Equal(t, "info", got.Log.Level)
	require.Equal(t, "dark", got.Appearance.ThemeMode)
	require.Equal(t, "auto", got.Appearance.Locale)
	require.False(t, got.Subscriptions.AutoLoadTraffic)
	require.Equal(t, 60, got.CacheDefaults.SubscriptionTrafficTTLSeconds)
}

func TestApplyUpdatePreservesOmittedTokenAndClearsExplicitEmptyToken(t *testing.T) {
	current := settings.Default()
	current.HTTP.Token = "secret"

	preserved, err := settings.ApplyUpdate(current, updateFromSettings(current, nil))
	require.NoError(t, err)
	require.Equal(t, "secret", preserved.HTTP.Token)

	empty := ""
	cleared, err := settings.ApplyUpdate(preserved, updateFromSettings(preserved, &empty))
	require.NoError(t, err)
	require.Empty(t, cleared.HTTP.Token)
}

func TestSettingsViewRedactsToken(t *testing.T) {
	value := settings.Default()
	value.HTTP.Token = "must-not-leak"

	body, err := json.Marshal(settings.View(value))

	require.NoError(t, err)
	require.NotContains(t, string(body), "must-not-leak")
	require.Contains(t, string(body), `"token_configured":true`)
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

func updateFromSettings(value domain.Settings, token *string) domain.SettingsUpdate {
	return domain.SettingsUpdate{
		SchemaVersion: value.SchemaVersion,
		HTTP: domain.HTTPSettingsUpdate{
			Listen:        value.HTTP.Listen,
			Token:         token,
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
