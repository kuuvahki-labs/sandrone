package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/settings"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func TestSettingsStoreMissingFileIsOptional(t *testing.T) {
	repo := store.NewSettingsStore(store.Coordinate(store.NewFSStore(afero.NewMemMapFs())))

	_, err := repo.Get(context.Background())

	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestSettingsStoreWritesRootFileWithPrivateMode(t *testing.T) {
	dir := t.TempDir()
	fs := afero.NewBasePathFs(afero.NewOsFs(), dir)
	repo := store.NewSettingsStore(store.Coordinate(store.NewFSStore(fs)))
	value := settings.Default()
	value.HTTP.Token = "secret"

	require.NoError(t, repo.Put(context.Background(), value))

	info, err := os.Stat(filepath.Join(dir, "settings.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	require.NoDirExists(t, filepath.Join(dir, "settings"))

	got, err := repo.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "secret", got.HTTP.Token)
	require.Equal(t, value.Appearance, got.Appearance)
}

func TestSettingsStoreReplacesExistingFileWithoutLeavingTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	fs := afero.NewBasePathFs(afero.NewOsFs(), dir)
	repo := store.NewSettingsStore(store.Coordinate(store.NewFSStore(fs)))
	first := settings.Default()
	first.HTTP.Listen = "127.0.0.1:1137"
	second := settings.Default()
	second.HTTP.Listen = "127.0.0.1:2237"

	require.NoError(t, repo.Put(context.Background(), first))
	require.NoError(t, repo.Put(context.Background(), second))

	got, err := repo.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:2237", got.HTTP.Listen)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "settings.json", entries[0].Name())
}

func TestSettingsStoreRejectsTrailingJSONValues(t *testing.T) {
	fs := afero.NewMemMapFs()
	raw := store.NewFSStore(fs)
	require.NoError(t, raw.Write(context.Background(), store.SettingsKey, []byte(`{"schema_version":1} {"schema_version":1}`)))
	repo := store.NewSettingsStore(store.Coordinate(raw))

	_, err := repo.Get(context.Background())

	require.Error(t, err)
}
