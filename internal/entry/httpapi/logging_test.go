package httpapi_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
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
