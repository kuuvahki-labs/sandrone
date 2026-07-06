package httpapi_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
	"github.com/kuuvahki-labs/sandrone/internal/service"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

const backupHTTPMaxBodyBytes = 32 << 20

var backupHTTPNow = time.Date(2026, 7, 21, 12, 34, 56, 0, time.FixedZone("test", -7*60*60))

func TestBackupEndpointsRequireBearerAuthentication(t *testing.T) {
	archive := exportHTTPBackup(t, map[string][]byte{"settings/runtime.json": []byte(`{"value":"new"}`)})
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	rt := backupHTTPRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}}, resourceStore)
	server := httpapi.New(rt)

	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{name: "download", method: http.MethodGet, path: "/v1/backup"},
		{name: "restore", method: http.MethodPost, path: "/v1/backup/restore", body: archive},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body)))
			require.Equal(t, http.StatusUnauthorized, w.Code)

			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer secret")
			w = httptest.NewRecorder()
			server.Handler().ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestBackupDownloadReturnsZipWithSafeHeaders(t *testing.T) {
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	writeHTTPBackupFile(t, resourceStore, "private/value.bin", []byte{0x00, 0xff, 0x01})
	rt := backupHTTPRuntime(t, app.Config{}, resourceStore)
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/backup", nil))

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/zip", w.Header().Get("Content-Type"))
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	disposition, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
	require.NoError(t, err)
	require.Equal(t, "attachment", disposition)
	require.Equal(t, "sandrone-backup-20260721T193456Z.zip", params["filename"])

	files := readHTTPBackupZip(t, w.Body.Bytes())
	require.Contains(t, files, "manifest.json")
	require.Equal(t, []byte{0x00, 0xff, 0x01}, files["data/private/value.bin"])
}

func TestBackupRestoreReplacesStoreFromRawZipBody(t *testing.T) {
	archive := exportHTTPBackup(t, map[string][]byte{
		"settings/runtime.json": []byte(`{"value":"new"}`),
		"unknown/raw.bin":       {0x00, 0xff, 0x01},
	})
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	writeHTTPBackupFile(t, resourceStore, "stale/value", []byte("remove me"))
	writeHTTPBackupFile(t, resourceStore, "cache/transient", []byte("clear me"))
	rt := backupHTTPRuntime(t, app.Config{}, resourceStore)
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/backup/restore", bytes.NewReader(archive)))

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.JSONEq(t, `{"ok":true}`, w.Body.String())
	require.Equal(t, map[string][]byte{
		"settings/runtime.json": []byte(`{"value":"new"}`),
		"unknown/raw.bin":       {0x00, 0xff, 0x01},
	}, snapshotHTTPBackupStore(t, resourceStore))
}

func TestBackupRestoreRejectsEmptyBody(t *testing.T) {
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	rt := backupHTTPRuntime(t, app.Config{}, resourceStore)
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/backup/restore", nil))

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestBackupRestoreEnforcesBounded32MiBRequestBody(t *testing.T) {
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	rt := backupHTTPRuntime(t, app.Config{}, resourceStore)
	server := httpapi.New(rt)
	body := &countingBackupBody{}

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/backup/restore", body))

	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	require.LessOrEqual(t, body.bytesRead, int64(backupHTTPMaxBodyBytes+1))
	require.Contains(t, w.Body.String(), `"code": "backup_too_large"`)
}

func TestBackupRestoreMapsArchiveErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       func(*testing.T) []byte
		wantStatus int
		wantCode   string
	}{
		{
			name:       "invalid archive",
			body:       func(*testing.T) []byte { return []byte("not a zip") },
			wantStatus: http.StatusBadRequest,
			wantCode:   "backup_invalid",
		},
		{
			name: "incompatible schema",
			body: func(t *testing.T) []byte {
				return backupHTTPZip(t, []byte(`{"format":"sandrone-store-backup","storage_schema_version":2,"created_at":"2026-07-21T19:34:56Z","app_version":"0.1.0"}`))
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "backup_incompatible",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resourceStore := store.NewFSStore(afero.NewMemMapFs())
			rt := backupHTTPRuntime(t, app.Config{}, resourceStore)
			server := httpapi.New(rt)

			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/backup/restore", bytes.NewReader(tc.body(t))))

			require.Equal(t, tc.wantStatus, w.Code)
			require.Contains(t, w.Body.String(), `"code": "`+tc.wantCode+`"`)
		})
	}
}

func TestBackupRestoreMapsStoreFailureToInternalServerError(t *testing.T) {
	secret := "sensitive archive content"
	archive := exportHTTPBackup(t, map[string][]byte{"new/value": []byte(secret)})
	baseStore := store.NewFSStore(afero.NewMemMapFs())
	writeHTTPBackupFile(t, baseStore, "old/value", []byte("keep me"))
	resourceStore := &failHTTPBackupWriteStore{Store: baseStore, failKey: "new/value"}
	rt := backupHTTPRuntime(t, app.Config{}, resourceStore)
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/backup/restore", bytes.NewReader(archive)))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), `"code": "backup_restore_failed"`)
	require.NotContains(t, w.Body.String(), secret)
	require.Equal(t, map[string][]byte{"old/value": []byte("keep me")}, snapshotHTTPBackupStore(t, baseStore))
}

func TestBackupDownloadMapsStoreFailureToInternalServerError(t *testing.T) {
	sensitivePath := "/private/sandrone/subscriptions/customer-secret.json"
	sensitiveValue := "bearer-sensitive-export-token"
	baseStore := store.NewFSStore(afero.NewMemMapFs())
	resourceStore := &failHTTPBackupListStore{
		Store: baseStore,
		err:   fmt.Errorf("list %s containing %s: %w", sensitivePath, sensitiveValue, os.ErrNotExist),
	}
	rt := backupHTTPRuntime(t, app.Config{}, resourceStore)
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/backup", nil))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.JSONEq(t, `{"error":{"code":"internal_error","message":"backup export failed"}}`, w.Body.String())
	require.NotContains(t, w.Body.String(), sensitivePath)
	require.NotContains(t, w.Body.String(), sensitiveValue)
}

func backupHTTPRuntime(t *testing.T, cfg app.Config, resourceStore store.Store) *app.Runtime {
	t.Helper()
	rt := testRuntime(t, cfg)
	rt.Service = service.New(
		service.WithStore(resourceStore),
		service.WithClock(func() time.Time { return backupHTTPNow }),
		service.WithLogger(rt.Logger),
	)
	return rt
}

func exportHTTPBackup(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	resourceStore := store.NewFSStore(afero.NewMemMapFs())
	for key, value := range files {
		writeHTTPBackupFile(t, resourceStore, key, value)
	}
	result, err := service.New(
		service.WithStore(resourceStore),
		service.WithClock(func() time.Time { return backupHTTPNow }),
	).ExportBackup(context.Background())
	require.NoError(t, err)
	return result.Body
}

func backupHTTPZip(t *testing.T, manifest []byte) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	entry, err := writer.Create("manifest.json")
	require.NoError(t, err)
	_, err = entry.Write(manifest)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return body.Bytes()
}

func readHTTPBackupZip(t *testing.T, body []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	files := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		opened, err := file.Open()
		require.NoError(t, err)
		files[file.Name], err = io.ReadAll(opened)
		require.NoError(t, err)
		require.NoError(t, opened.Close())
	}
	return files
}

func writeHTTPBackupFile(t *testing.T, resourceStore store.Store, key string, value []byte) {
	t.Helper()
	require.NoError(t, resourceStore.Write(context.Background(), key, value))
}

func snapshotHTTPBackupStore(t *testing.T, resourceStore store.Store) map[string][]byte {
	t.Helper()
	ctx := context.Background()
	entries, err := resourceStore.List(ctx, "")
	require.NoError(t, err)
	files := make(map[string][]byte)
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		files[entry.Key], err = resourceStore.Read(ctx, entry.Key)
		require.NoError(t, err)
	}
	return files
}

type countingBackupBody struct {
	bytesRead int64
}

func (r *countingBackupBody) Read(body []byte) (int, error) {
	clear(body)
	r.bytesRead += int64(len(body))
	return len(body), nil
}

type failHTTPBackupWriteStore struct {
	store.Store
	failKey string
	mu      sync.Mutex
	failed  bool
}

func (s *failHTTPBackupWriteStore) Write(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key == s.failKey && !s.failed {
		s.failed = true
		return errors.New("injected backup restore failure")
	}
	return s.Store.Write(ctx, key, value)
}

type failHTTPBackupListStore struct {
	store.Store
	err error
}

func (s *failHTTPBackupListStore) List(context.Context, string) ([]store.Entry, error) {
	return nil, s.err
}
