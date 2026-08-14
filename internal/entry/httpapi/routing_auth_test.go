package httpapi_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
	"github.com/kuuvahki-labs/sandrone/internal/entry/webui"
)

func TestWebUIMountServesSPARoutesWithoutShadowingAPI(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt, httpapi.WithWebUI(webui.HandlerWithFS(fstest.MapFS{
		"index.html": {Data: []byte("<html><title>Sandrone</title><body>Sandrone</body></html>")},
	})))

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Sandrone")

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/setup", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Sandrone")

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"items"`)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/unknown", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebUIServesWithoutBearerTokenWhenTokenAuthIsEnabled(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	server := httpapi.New(rt, httpapi.WithWebUI(webui.HandlerWithFS(fstest.MapFS{
		"index.html":    {Data: []byte(`<html><title>Sandrone</title><body><script src="/assets/app.js"></script></body></html>`)},
		"assets/app.js": {Data: []byte(`console.log("Sandrone")`)},
	})))

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Sandrone")

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Sandrone")

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCustomMCPPathStaysProtectedWhenWebUIIsMounted(t *testing.T) {
	rt := testRuntime(t, app.Config{
		HTTP: app.HTTPConfig{Token: "secret"},
		MCP:  app.MCPConfig{Path: "/agent"},
	})
	mcp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("mcp"))
	})
	server := httpapi.New(
		rt,
		httpapi.WithMCP(rt.Config.MCP.Path, mcp),
		httpapi.WithWebUI(webui.HandlerWithFS(
			fstest.MapFS{"index.html": {Data: []byte("<html>Sandrone</html>")}},
			webui.WithReservedPrefixes("/v1", rt.Config.MCP.Path, "/s"),
		)),
	)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/agent", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)

	req := httptest.NewRequest(http.MethodGet, "/agent", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "mcp", w.Body.String())
}

func TestUnknownV1EndpointIsNotRegistered(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/unknown", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestTokenAuth(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, w.Code)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/v1/unknown", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestVersionIsPublicWhenTokenAuthIsEnabled(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/version", nil))

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, fmt.Sprintf(
		`{"name":"sandrone","version":%q,"revision":%q}`,
		buildinfo.Version(),
		buildinfo.Revision(),
	), w.Body.String())
}

func TestVersionRejectsNonGETMethodsWithoutAuthentication(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	server := httpapi.New(rt)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/version", nil))

	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestVersionTrailingSlashRemainsAWebUIRoute(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	server := httpapi.New(rt, httpapi.WithWebUI(webui.HandlerWithFS(fstest.MapFS{
		"index.html": {Data: []byte("<html>Sandrone Web UI</html>")},
	})))

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/version/", nil))

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "<html>Sandrone Web UI</html>", w.Body.String())
	require.NotContains(t, w.Body.String(), `"version"`)
}
