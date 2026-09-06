package httpapi_test

import (
	"bytes"
	"encoding/json/v2"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
	"github.com/kuuvahki-labs/sandrone/internal/logbuffer"
)

func TestHandlerLogsHTTPRequests(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := app.Config{DataDir: t.TempDir()}
	rt, err := app.NewRuntime(cfg, logger)
	require.NoError(t, err)
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusOK, w.Code)
	out := logs.String()
	require.Contains(t, out, `"msg":"http request completed"`)
	require.Contains(t, out, `"method":"GET"`)
	require.Contains(t, out, `"path":"/healthz"`)
	require.Contains(t, out, `"status":200`)
	require.Contains(t, out, `"duration_ms":`)
}

func TestRuntimeLogEndpoint(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rt, err := app.NewRuntime(app.Config{DataDir: t.TempDir(), HTTP: app.HTTPConfig{Token: "test-token"}}, logger)
	require.NoError(t, err)
	rt.Logger.Info("runtime event", "url", "https://example.com/?token=example")
	handler := httpapi.New(rt).Handler()
	for range 2 {
		request := httptest.NewRequest(http.MethodGet, "/v1/logs", nil)
		request.Header.Set("Authorization", "Bearer test-token")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		var snapshot logbuffer.Snapshot
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &snapshot))
		require.Len(t, snapshot.Entries, 1)
		require.Equal(t, "debug", snapshot.Level)
		require.Equal(t, "runtime event", snapshot.Entries[0].Message)
		require.Equal(t, "https://example.com/?token=example", snapshot.Entries[0].Attrs["url"])
	}
	require.Equal(t, 2, strings.Count(output.String(), "http request completed"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/logs", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.NotContains(t, response.Body.String(), "runtime event")
	snapshot, err := rt.Service.Logs()
	require.NoError(t, err)
	require.Equal(t, "warn", snapshot.Entries[0].Level)
	fresh, err := app.NewRuntime(app.Config{DataDir: t.TempDir()}, nil)
	require.NoError(t, err)
	freshSnapshot, err := fresh.Service.Logs()
	require.NoError(t, err)
	require.NotEqual(t, snapshot.InstanceID, freshSnapshot.InstanceID)
	require.Empty(t, freshSnapshot.Entries)
}
