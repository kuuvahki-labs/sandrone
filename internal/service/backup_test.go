package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

const (
	testBackupMaxCompressedBytes = 32 << 20
	testBackupMaxDecodedBytes    = 64 << 20
	testBackupMaxEntries         = 10_000
)

var (
	testBackupNow       = time.Date(2026, 7, 21, 12, 34, 56, 0, time.FixedZone("test", -7*60*60))
	validBackupManifest = []byte(`{"format":"sandrone-store-backup","storage_schema_version":1,"created_at":"2026-07-21T19:34:56Z","app_version":"0.1.0"}`)
)

func TestBackupExportAndRestoreRoundTripRawStore(t *testing.T) {
	ctx := context.Background()
	source := store.NewFSStore(afero.NewMemMapFs())
	writeBackupTestFile(t, source, "z-unknown/binary.dat", []byte{0x00, 0xff, 0x01, 0x7f})
	writeBackupTestFile(t, source, "subscriptions/example.json", []byte(`{"url":"https://secret.example/sub"}`))
	writeBackupTestFile(t, source, "cache/probe/transient", []byte("discard me"))
	svc := service.New(service.WithStore(source), service.WithClock(func() time.Time { return testBackupNow }))

	first, err := svc.ExportBackup(ctx)
	require.NoError(t, err)
	second, err := svc.ExportBackup(ctx)
	require.NoError(t, err)
	require.Equal(t, "sandrone-backup-20260721T193456Z.zip", first.Filename)
	require.Equal(t, first.Body, second.Body, "fixed input and clock must produce deterministic ZIP bytes")

	archive := readBackupZip(t, first.Body)
	require.Equal(t, []string{
		"manifest.json",
		"data/subscriptions/example.json",
		"data/z-unknown/binary.dat",
	}, archive.names)
	require.Equal(t, []byte(`{"url":"https://secret.example/sub"}`), archive.files["data/subscriptions/example.json"])
	require.Equal(t, []byte{0x00, 0xff, 0x01, 0x7f}, archive.files["data/z-unknown/binary.dat"])
	for _, file := range archive.headers {
		require.Equal(t, testBackupNow.UTC(), file.Modified.UTC())
	}

	var manifest map[string]any
	require.NoError(t, json.Unmarshal(archive.files["manifest.json"], &manifest))
	require.Len(t, manifest, 4)
	require.Equal(t, "sandrone-store-backup", manifest["format"])
	require.Equal(t, float64(1), manifest["storage_schema_version"])
	require.Equal(t, "2026-07-21T19:34:56Z", manifest["created_at"])
	require.Equal(t, buildinfo.Version(), manifest["app_version"])

	target := store.NewFSStore(afero.NewMemMapFs())
	writeBackupTestFile(t, target, "stale/value", []byte("replace me"))
	writeBackupTestFile(t, target, "cache/runtime/stale", []byte("clear me"))
	targetService := service.New(service.WithStore(target), service.WithClock(func() time.Time { return testBackupNow }))
	require.NoError(t, targetService.RestoreBackup(ctx, first.Body))
	require.Equal(t, map[string][]byte{
		"subscriptions/example.json": []byte(`{"url":"https://secret.example/sub"}`),
		"z-unknown/binary.dat":       {0x00, 0xff, 0x01, 0x7f},
	}, snapshotBackupStoreFiles(t, target))

	roundTripped, err := targetService.ExportBackup(ctx)
	require.NoError(t, err)
	require.Equal(t, first.Body, roundTripped.Body)
}

func TestBackupRestoreValidatesReloadsAndSecuresSettings(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	resourceStore := store.NewFSStore(afero.NewBasePathFs(afero.NewOsFs(), root))
	svc := service.New(service.WithStore(resourceStore))

	replacement := projectsettings.Default()
	replacement.HTTP.Listen = "127.0.0.1:2237"
	replacement.Appearance.ThemeMode = "light"
	replacement.Subscriptions.AutoLoadTraffic = true
	body, err := json.Marshal(replacement)
	require.NoError(t, err)
	archive := validBackupZip(t, backupZipMember{name: "data/settings.json", body: body})

	require.NoError(t, svc.RestoreBackup(ctx, archive))
	snapshot, err := svc.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "light", snapshot.Effective.Appearance.ThemeMode)
	require.True(t, snapshot.Effective.Subscriptions.AutoLoadTraffic)
	require.Equal(t, "127.0.0.1:1137", snapshot.Effective.HTTP.Listen)
	require.Contains(t, snapshot.RestartRequired, "http.listen")

	info, err := os.Stat(filepath.Join(root, "settings.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestBackupRestoreRejectsInvalidSettingsWithoutMutation(t *testing.T) {
	ctx := context.Background()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	writeBackupTestFile(t, resourceStore, "old/keep", []byte("unchanged"))
	before := snapshotBackupStoreFiles(t, resourceStore)
	svc := service.New(service.WithStore(resourceStore))
	archive := validBackupZip(t, backupZipMember{
		name: "data/settings.json",
		body: []byte(`{"schema_version":1,"cache_defaults":{"probe_ttl_seconds":-1}}`),
	})

	err := svc.RestoreBackup(ctx, archive)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeBackupInvalid), "error = %v", err)
	require.Equal(t, before, snapshotBackupStoreFiles(t, resourceStore))
}

func TestBackupExportOmitsStartupAuthentication(t *testing.T) {
	ctx := context.Background()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	svc := service.New(service.WithStore(resourceStore), service.WithClock(func() time.Time { return testBackupNow }))
	update := projectSettingsUpdate(projectsettings.Default())
	_, err := svc.PutSettings(ctx, update)
	require.NoError(t, err)

	result, err := svc.ExportBackup(ctx)
	require.NoError(t, err)
	archive := readBackupZip(t, result.Body)
	settingsBody := string(archive.files["data/settings.json"])
	require.NotContains(t, settingsBody, `"token"`)
	require.NotContains(t, settingsBody, `"token_required"`)
}

func TestBackupExportExcludesExactAndNestedCacheKeys(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cacheKey string
	}{
		{name: "exact cache key", cacheKey: "cache"},
		{name: "nested cache key", cacheKey: "cache/runtime/value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resourceStore := store.NewFSStore(afero.NewMemMapFs())
			writeBackupTestFile(t, resourceStore, tc.cacheKey, []byte("transient secret"))
			writeBackupTestFile(t, resourceStore, "cacheable", []byte("persistent"))
			svc := service.New(service.WithStore(resourceStore), service.WithClock(func() time.Time { return testBackupNow }))

			result, err := svc.ExportBackup(context.Background())
			require.NoError(t, err)
			archive := readBackupZip(t, result.Body)
			require.Equal(t, []string{"manifest.json", "data/cacheable"}, archive.names)
			require.Equal(t, []byte("persistent"), archive.files["data/cacheable"])
		})
	}
}

func projectSettingsUpdate(value domain.Settings) domain.SettingsUpdate {
	return domain.SettingsUpdate{
		SchemaVersion:    value.SchemaVersion,
		HTTP:             value.HTTP,
		MCP:              value.MCP,
		Log:              value.Log,
		RemoteDefaults:   value.RemoteDefaults,
		ProbeDefaults:    value.ProbeDefaults,
		CacheDefaults:    value.CacheDefaults,
		Appearance:       value.Appearance,
		Subscriptions:    value.Subscriptions,
		ScheduledRefresh: value.ScheduledRefresh,
	}
}

func TestBackupEmptyStoreArchiveRestoresByClearingAllFiles(t *testing.T) {
	ctx := context.Background()
	empty := store.NewFSStore(afero.NewMemMapFs())
	exporter := service.New(service.WithStore(empty), service.WithClock(func() time.Time { return testBackupNow }))
	result, err := exporter.ExportBackup(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"manifest.json"}, readBackupZip(t, result.Body).names)

	target := store.NewFSStore(afero.NewMemMapFs())
	writeBackupTestFile(t, target, "settings.json", []byte("old"))
	writeBackupTestFile(t, target, "cache/remote/value", []byte("cached"))
	restorer := service.New(service.WithStore(target))
	require.NoError(t, restorer.RestoreBackup(ctx, result.Body))
	require.Empty(t, snapshotBackupStoreFiles(t, target))
}

func TestBackupRestoreReplacesExistingDirectoryTreeWithFile(t *testing.T) {
	ctx := context.Background()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	writeBackupTestFile(t, resourceStore, "a/b", []byte("old nested file"))
	svc := service.New(service.WithStore(resourceStore))
	archive := validBackupZip(t, backupZipMember{name: "data/a", body: []byte("new file")})

	require.NoError(t, svc.RestoreBackup(ctx, archive))
	require.Equal(t, map[string][]byte{"a": []byte("new file")}, snapshotBackupStoreFiles(t, resourceStore))
}

func TestBackupRestoreRollbackReplacesNewDirectoryTreeWithOldFile(t *testing.T) {
	ctx := context.Background()
	baseStore := store.NewFSStore(afero.NewMemMapFs())
	writeBackupTestFile(t, baseStore, "a", []byte("old file"))
	failingStore := &backupWriteFailStore{Store: baseStore, failKey: "z-fail"}
	svc := service.New(service.WithStore(failingStore))
	archive := validBackupZip(t,
		backupZipMember{name: "data/a/b", body: []byte("partial nested file")},
		backupZipMember{name: "data/z-fail", body: []byte("failure")},
	)

	err := svc.RestoreBackup(ctx, archive)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeBackupRestoreFailed), "error = %v", err)
	require.Equal(t, map[string][]byte{"a": []byte("old file")}, snapshotBackupStoreFiles(t, baseStore))
}

func TestBackupRestoreRejectsInvalidArchivesWithoutMutation(t *testing.T) {
	invalidCode := domain.ErrorCode("backup_invalid")
	incompatibleCode := domain.ErrorCode("backup_incompatible")
	tests := []struct {
		name    string
		archive func(*testing.T) []byte
		code    domain.ErrorCode
	}{
		{name: "empty body", archive: func(*testing.T) []byte { return nil }, code: invalidCode},
		{name: "not a zip", archive: func(*testing.T) []byte { return []byte("not a zip") }, code: invalidCode},
		{name: "missing manifest", archive: func(t *testing.T) []byte {
			return makeBackupZip(t, backupZipMember{name: "data/value", body: []byte("value")})
		}, code: invalidCode},
		{name: "malformed manifest", archive: func(t *testing.T) []byte {
			return makeBackupZip(t, backupZipMember{name: "manifest.json", body: []byte("{")})
		}, code: invalidCode},
		{name: "manifest has extra field", archive: func(t *testing.T) []byte {
			return makeBackupZip(t, backupZipMember{name: "manifest.json", body: []byte(`{"format":"sandrone-store-backup","storage_schema_version":1,"created_at":"2026-07-21T19:34:56Z","app_version":"0.1.0","entries":[]}`)})
		}, code: invalidCode},
		{name: "manifest has duplicate JSON field", archive: func(t *testing.T) []byte {
			return makeBackupZip(t, backupZipMember{name: "manifest.json", body: []byte(`{"format":"sandrone-store-backup","\u0066ormat":"sandrone-store-backup","storage_schema_version":1,"created_at":"2026-07-21T19:34:56Z","app_version":"0.1.0"}`)})
		}, code: invalidCode},
		{name: "manifest is missing a field", archive: func(t *testing.T) []byte {
			return makeBackupZip(t, backupZipMember{name: "manifest.json", body: []byte(`{"format":"sandrone-store-backup","storage_schema_version":1,"created_at":"2026-07-21T19:34:56Z"}`)})
		}, code: invalidCode},
		{name: "manifest version has null type", archive: func(t *testing.T) []byte {
			return makeBackupZip(t, backupZipMember{name: "manifest.json", body: []byte(`{"format":"sandrone-store-backup","storage_schema_version":null,"created_at":"2026-07-21T19:34:56Z","app_version":"0.1.0"}`)})
		}, code: invalidCode},
		{name: "manifest timestamp is invalid", archive: func(t *testing.T) []byte {
			return makeBackupZip(t, backupZipMember{name: "manifest.json", body: []byte(`{"format":"sandrone-store-backup","storage_schema_version":1,"created_at":"yesterday","app_version":"0.1.0"}`)})
		}, code: invalidCode},
		{name: "manifest format is invalid", archive: func(t *testing.T) []byte {
			return makeBackupZip(t, backupZipMember{name: "manifest.json", body: []byte(`{"format":"another-backup","storage_schema_version":1,"created_at":"2026-07-21T19:34:56Z","app_version":"0.1.0"}`)})
		}, code: invalidCode},
		{name: "storage schema is incompatible", archive: func(t *testing.T) []byte {
			return makeBackupZip(t, backupZipMember{name: "manifest.json", body: []byte(`{"format":"sandrone-store-backup","storage_schema_version":2,"created_at":"2026-07-21T19:34:56Z","app_version":"0.1.0"}`)})
		}, code: incompatibleCode},
		{name: "duplicate manifest", archive: func(t *testing.T) []byte {
			return makeBackupZip(t,
				backupZipMember{name: "manifest.json", body: validBackupManifest},
				backupZipMember{name: "manifest.json", body: validBackupManifest},
			)
		}, code: invalidCode},
		{name: "unknown root member", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: "notes.txt", body: []byte("no")})
		}, code: invalidCode},
		{name: "directory member", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: "data/directory/", mode: os.ModeDir | 0o755})
		}, code: invalidCode},
		{name: "symlink member", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: "data/link", body: []byte("target"), mode: os.ModeSymlink | 0o777})
		}, code: invalidCode},
		{name: "duplicate data key", archive: func(t *testing.T) []byte {
			return validBackupZip(t,
				backupZipMember{name: "data/duplicate", body: []byte("one")},
				backupZipMember{name: "data/duplicate", body: []byte("two")},
			)
		}, code: invalidCode},
		{name: "file key conflicts with descendant key", archive: func(t *testing.T) []byte {
			return validBackupZip(t,
				backupZipMember{name: "data/a", body: []byte("file")},
				backupZipMember{name: "data/a/b", body: []byte("descendant")},
			)
		}, code: invalidCode},
		{name: "traversal key", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: "data/../escape", body: []byte("no")})
		}, code: invalidCode},
		{name: "absolute key", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: "data//absolute", body: []byte("no")})
		}, code: invalidCode},
		{name: "empty path segment", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: "data/parent//child", body: []byte("no")})
		}, code: invalidCode},
		{name: "backslash key", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: `data/parent\child`, body: []byte("no")})
		}, code: invalidCode},
		{name: "NUL key", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: "data/parent\x00child", body: []byte("no")})
		}, code: invalidCode},
		{name: "Windows drive absolute key", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: "data/C:/x", body: []byte("no")})
		}, code: invalidCode},
		{name: "Windows drive relative key", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: "data/C:x", body: []byte("no")})
		}, code: invalidCode},
		{name: "exact cache key", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: "data/cache", body: []byte("no")})
		}, code: invalidCode},
		{name: "nested cache key", archive: func(t *testing.T) []byte {
			return validBackupZip(t, backupZipMember{name: "data/cache/value", body: []byte("no")})
		}, code: invalidCode},
		{name: "invalid member crc", archive: corruptBackupMemberCRC, code: invalidCode},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			resourceStore := store.NewFSStore(afero.NewMemMapFs())
			writeBackupTestFile(t, resourceStore, "old/keep", []byte("existing secret"))
			writeBackupTestFile(t, resourceStore, "cache/stale", []byte("existing cache"))
			before := snapshotBackupStoreFiles(t, resourceStore)
			svc := service.New(service.WithStore(resourceStore))

			err := svc.RestoreBackup(ctx, tc.archive(t))
			require.Error(t, err)
			require.True(t, domain.IsCode(err, tc.code), "error = %v", err)
			require.Equal(t, before, snapshotBackupStoreFiles(t, resourceStore), "invalid backup mutated the Store")
		})
	}
}

func TestBackupRestoreEnforcesCompressedDecodedAndEntryLimits(t *testing.T) {
	tooLargeCode := domain.ErrorCode("backup_too_large")
	tests := []struct {
		name    string
		archive func(*testing.T) []byte
	}{
		{name: "compressed bytes", archive: func(*testing.T) []byte {
			return make([]byte, testBackupMaxCompressedBytes+1)
		}},
		{name: "decoded bytes", archive: makeDecodedLimitBackupZip},
		{name: "data entry count", archive: makeEntryLimitBackupZip},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resourceStore := store.NewFSStore(afero.NewMemMapFs())
			writeBackupTestFile(t, resourceStore, "old/keep", []byte("unchanged"))
			before := snapshotBackupStoreFiles(t, resourceStore)
			svc := service.New(service.WithStore(resourceStore))

			err := svc.RestoreBackup(context.Background(), tc.archive(t))
			require.Error(t, err)
			require.True(t, domain.IsCode(err, tooLargeCode), "error = %v", err)
			require.Equal(t, before, snapshotBackupStoreFiles(t, resourceStore))
		})
	}
}

func TestBackupRestoreRollsBackOldNonCacheBytesAfterWriteFailure(t *testing.T) {
	ctx := context.Background()
	baseStore := store.NewFSStore(afero.NewMemMapFs())
	writeBackupTestFile(t, baseStore, "old/alpha", []byte("alpha"))
	writeBackupTestFile(t, baseStore, "old/beta", []byte{0x00, 0xff, 0x10})
	writeBackupTestFile(t, baseStore, "cache/runtime", []byte("do not restore"))
	failingStore := &backupWriteFailStore{Store: baseStore, failKey: "new/z-fail"}
	svc := service.New(service.WithStore(failingStore))
	archive := validBackupZip(t,
		backupZipMember{name: "data/new/a-written-first", body: []byte("partial")},
		backupZipMember{name: "data/new/z-fail", body: []byte("failure")},
	)

	err := svc.RestoreBackup(ctx, archive)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeBackupRestoreFailed), "error = %v", err)
	require.Equal(t, map[string][]byte{
		"old/alpha": []byte("alpha"),
		"old/beta":  {0x00, 0xff, 0x10},
	}, snapshotBackupStoreFiles(t, baseStore))
}

func TestBackupRestoreRollbackFailureLogsBothCausesWithoutKeysOrContents(t *testing.T) {
	ctx := context.Background()
	baseStore := store.NewFSStore(afero.NewMemMapFs())
	oldKey := "old/sensitive-old-key"
	newKey := "new/sensitive-new-key"
	oldContent := "old-secret-content"
	newContent := "new-secret-content"
	writeBackupTestFile(t, baseStore, oldKey, []byte(oldContent))
	failingStore := &backupRollbackFailStore{
		Store:           baseStore,
		restoreFailKey:  newKey,
		rollbackFailKey: oldKey,
		restoreError:    errors.New("restore failure mentions sensitive-new-key and new-secret-content"),
		rollbackError:   errors.New("rollback failure mentions sensitive-old-key and old-secret-content"),
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	svc := service.New(service.WithStore(failingStore), service.WithLogger(logger))
	archive := validBackupZip(t, backupZipMember{name: "data/" + newKey, body: []byte(newContent)})

	err := svc.RestoreBackup(ctx, archive)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeBackupRestoreFailed), "error = %v", err)
	logOutput := logs.String()
	require.Contains(t, logOutput, "restore_cause")
	require.Contains(t, logOutput, "rollback_cause")
	for _, secret := range []string{oldKey, newKey, oldContent, newContent} {
		require.NotContains(t, logOutput, secret)
	}
}

func TestBackupRestoreIgnoresCancellationAfterMutationStarts(t *testing.T) {
	baseStore := store.NewFSStore(afero.NewMemMapFs())
	writeBackupTestFile(t, baseStore, "old/one", []byte("old"))
	ctx, cancel := context.WithCancel(context.Background())
	cancelingStore := &backupCancelOnDeleteStore{Store: baseStore, cancel: cancel}
	svc := service.New(service.WithStore(cancelingStore))
	archive := validBackupZip(t, backupZipMember{name: "data/new/one", body: []byte("new")})

	require.NoError(t, svc.RestoreBackup(ctx, archive))
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.Equal(t, map[string][]byte{"new/one": []byte("new")}, snapshotBackupStoreFiles(t, baseStore))
}

type backupZipMember struct {
	name   string
	body   []byte
	mode   os.FileMode
	stored bool
}

type backupZipContents struct {
	names   []string
	files   map[string][]byte
	headers map[string]*zip.File
}

func validBackupZip(t *testing.T, members ...backupZipMember) []byte {
	t.Helper()
	return makeBackupZip(t, append([]backupZipMember{{name: "manifest.json", body: validBackupManifest}}, members...)...)
}

func makeBackupZip(t *testing.T, members ...backupZipMember) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for _, member := range members {
		header := &zip.FileHeader{Name: member.name, Method: zip.Deflate}
		if member.stored {
			header.Method = zip.Store
		}
		if member.mode != 0 {
			header.SetMode(member.mode)
		}
		entry, err := writer.CreateHeader(header)
		require.NoError(t, err)
		_, err = entry.Write(member.body)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return body.Bytes()
}

func readBackupZip(t *testing.T, body []byte) backupZipContents {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	contents := backupZipContents{files: map[string][]byte{}, headers: map[string]*zip.File{}}
	for _, file := range reader.File {
		contents.names = append(contents.names, file.Name)
		contents.headers[file.Name] = file
		opened, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(opened)
		require.NoError(t, err)
		require.NoError(t, opened.Close())
		contents.files[file.Name] = data
	}
	return contents
}

func corruptBackupMemberCRC(t *testing.T) []byte {
	t.Helper()
	marker := []byte("unique-crc-payload")
	body := makeBackupZip(t,
		backupZipMember{name: "manifest.json", body: validBackupManifest, stored: true},
		backupZipMember{name: "data/value", body: marker, stored: true},
	)
	index := bytes.Index(body, marker)
	require.NotEqual(t, -1, index)
	body[index] ^= 0xff
	return body
}

func makeDecodedLimitBackupZip(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	manifest, err := writer.Create("manifest.json")
	require.NoError(t, err)
	_, err = manifest.Write(validBackupManifest)
	require.NoError(t, err)
	data, err := writer.Create("data/oversized")
	require.NoError(t, err)
	_, err = io.CopyN(data, zeroBackupReader{}, testBackupMaxDecodedBytes+1)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return body.Bytes()
}

func makeEntryLimitBackupZip(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	manifest, err := writer.Create("manifest.json")
	require.NoError(t, err)
	_, err = manifest.Write(validBackupManifest)
	require.NoError(t, err)
	for index := 0; index <= testBackupMaxEntries; index++ {
		_, err = writer.Create(fmt.Sprintf("data/item-%05d", index))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return body.Bytes()
}

type zeroBackupReader struct{}

func (zeroBackupReader) Read(value []byte) (int, error) {
	clear(value)
	return len(value), nil
}

func writeBackupTestFile(t *testing.T, resourceStore store.Store, key string, value []byte) {
	t.Helper()
	require.NoError(t, resourceStore.Write(context.Background(), key, value))
}

func snapshotBackupStoreFiles(t *testing.T, resourceStore store.Store) map[string][]byte {
	t.Helper()
	ctx := context.Background()
	entries, err := resourceStore.List(ctx, "")
	require.NoError(t, err)
	files := make(map[string][]byte)
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		body, err := resourceStore.Read(ctx, entry.Key)
		require.NoError(t, err)
		files[entry.Key] = body
	}
	return files
}

type backupWriteFailStore struct {
	store.Store
	failKey string
	mu      sync.Mutex
	failed  bool
}

func (s *backupWriteFailStore) Write(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key == s.failKey && !s.failed {
		s.failed = true
		return errors.New("injected backup write failure")
	}
	return s.Store.Write(ctx, key, value)
}

type backupRollbackFailStore struct {
	store.Store
	restoreFailKey  string
	rollbackFailKey string
	restoreError    error
	rollbackError   error
	mu              sync.Mutex
	restoreFailed   bool
}

func (s *backupRollbackFailStore) Write(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key == s.restoreFailKey && !s.restoreFailed {
		s.restoreFailed = true
		return s.restoreError
	}
	if key == s.rollbackFailKey && s.restoreFailed {
		return s.rollbackError
	}
	return s.Store.Write(ctx, key, value)
}

type backupCancelOnDeleteStore struct {
	store.Store
	cancel context.CancelFunc
	once   sync.Once
}

func (s *backupCancelOnDeleteStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.Store.Delete(ctx, key); err != nil {
		return err
	}
	s.once.Do(s.cancel)
	return nil
}

func (s *backupCancelOnDeleteStore) Write(ctx context.Context, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Store.Write(ctx, key, value)
}
