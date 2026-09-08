package backup_test

import (
	"context"
	"fmt"
	"io/fs"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/backup"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func TestSnapshotReplaceAndRestoreDirectories(t *testing.T) {
	for _, virtual := range []bool{false, true} {
		name := "filesystem"
		if virtual {
			name = "virtual directories"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			base := store.NewFSStore(afero.NewMemMapFs())
			var resourceStore store.Store = base
			if virtual {
				resourceStore = virtualDirectoryStore{base}
			}
			require.NoError(t, resourceStore.Write(ctx, "old/nested/value", []byte("original")))
			require.NoError(t, resourceStore.Write(ctx, "cache/nested/value", []byte("discard")))
			snapshot, err := backup.Capture(ctx, resourceStore)
			require.NoError(t, err)

			// Replace an old directory with a file as well as creating new directories.
			require.NoError(t, snapshot.Replace(ctx, resourceStore, map[string][]byte{
				"old":              []byte("replacement"),
				"new/nested/value": []byte("partial replacement"),
			}))
			entries, err := backup.ReadEntries(ctx, resourceStore)
			require.NoError(t, err)
			require.Equal(t, []backup.Entry{
				{Key: "new/nested/value", Body: []byte("partial replacement")},
				{Key: "old", Body: []byte("replacement")},
			}, entries)

			// Rollback must also clean virtual directories from partial replacement writes.
			require.NoError(t, snapshot.Restore(ctx, resourceStore))
			entries, err = backup.ReadEntries(ctx, resourceStore)
			require.NoError(t, err)
			require.Equal(t, []backup.Entry{{Key: "old/nested/value", Body: []byte("original")}}, entries)
			_, err = resourceStore.Stat(ctx, "cache")
			require.ErrorIs(t, err, fs.ErrNotExist)
		})
	}
}

func TestSnapshotDeleteErrorsRemainFatal(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
		err  error
	}{
		{name: "missing file", key: "old/value", err: fs.ErrNotExist},
		{name: "directory permission", key: "old", err: fs.ErrPermission},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			resourceStore := deleteErrorStore{
				Store: store.NewFSStore(afero.NewMemMapFs()),
				key:   tc.key,
				err:   tc.err,
			}
			require.NoError(t, resourceStore.Write(ctx, "old/value", []byte("original")))
			snapshot, err := backup.Capture(ctx, resourceStore)
			require.NoError(t, err)
			err = snapshot.Replace(ctx, resourceStore, map[string][]byte{"new": []byte("replacement")})
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, "delete current Store files", backup.StoreOperation(err))
			_, err = resourceStore.Read(ctx, "new")
			require.ErrorIs(t, err, fs.ErrNotExist)
		})
	}
}

// Like S3, directory entries returned by List cannot be deleted as objects.
// Removing the backing filesystem directory keeps this fake's subsequent List accurate.
type virtualDirectoryStore struct {
	*store.FSStore
}

func (s virtualDirectoryStore) Delete(ctx context.Context, key string) error {
	entry, err := s.Stat(ctx, key)
	if err != nil {
		return err
	}
	if err := s.FSStore.Delete(ctx, key); err != nil {
		return err
	}
	if entry.IsDir {
		return fmt.Errorf("delete virtual directory: %w", fs.ErrNotExist)
	}
	return nil
}

type deleteErrorStore struct {
	store.Store
	key string
	err error
}

func (s deleteErrorStore) Delete(ctx context.Context, key string) error {
	if key == s.key {
		return fmt.Errorf("delete: %w", s.err)
	}
	return s.Store.Delete(ctx, key)
}
