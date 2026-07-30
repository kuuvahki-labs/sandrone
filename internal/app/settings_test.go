package app_test

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func TestResolveSettingsUsesOptionalFileThenOverrides(t *testing.T) {
	repo := testSettingsStore()
	file := projectsettings.Default()
	file.HTTP.Listen = "127.0.0.1:2237"
	file.Log.Level = "warn"
	require.NoError(t, repo.Put(context.Background(), file))

	cfg, err := app.ResolveSettings(context.Background(), app.Config{
		DataDir: t.TempDir(),
		HTTP:    app.HTTPConfig{Listen: "127.0.0.1:3237"},
		OverrideSources: map[string]string{
			"http.listen": "environment",
		},
	}, repo)

	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:3237", cfg.HTTP.Listen)
	require.Equal(t, "warn", cfg.Log.Level)
	require.Equal(t, "127.0.0.1:2237", cfg.StoredSettings.HTTP.Listen)
	require.Equal(t, "127.0.0.1:3237", cfg.EffectiveSettings.HTTP.Listen)
	require.Equal(t, "environment", cfg.OverrideSources["http.listen"])
}

func TestResolveSettingsUsesDefaultsWhenFileIsMissing(t *testing.T) {
	repo := testSettingsStore()

	cfg, err := app.ResolveSettings(context.Background(), app.Config{DataDir: t.TempDir()}, repo)

	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:1137", cfg.HTTP.Listen)
	require.Equal(t, "info", cfg.Log.Level)
	require.Equal(t, projectsettings.Default(), cfg.StoredSettings)
}

func TestResolveSettingsRejectsInvalidExistingFile(t *testing.T) {
	raw := store.NewFSStore(afero.NewMemMapFs())
	require.NoError(t, raw.Write(context.Background(), store.SettingsKey, []byte(`{"schema_version":2}`)))
	repo := store.NewSettingsStore(store.Coordinate(raw))

	_, err := app.ResolveSettings(context.Background(), app.Config{DataDir: t.TempDir()}, repo)

	require.Error(t, err)
	require.False(t, os.IsNotExist(err))
}

func testSettingsStore() *store.SettingsStore {
	return store.NewSettingsStore(store.Coordinate(store.NewFSStore(afero.NewMemMapFs())))
}
