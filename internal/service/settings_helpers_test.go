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
		SchemaVersion:  value.SchemaVersion,
		HTTP:           domain.HTTPSettingsUpdate{Listen: value.HTTP.Listen, TokenRequired: value.HTTP.TokenRequired},
		MCP:            value.MCP,
		WebUI:          value.WebUI,
		Log:            value.Log,
		RemoteDefaults: value.RemoteDefaults,
		ProbeDefaults:  value.ProbeDefaults,
		CacheDefaults:  value.CacheDefaults,
		Appearance:     value.Appearance,
		Subscriptions:  value.Subscriptions,
	}
	if mutate != nil {
		mutate(&update)
	}
	_, err := svc.PutSettings(ctx, update)
	require.NoError(t, err)
}
