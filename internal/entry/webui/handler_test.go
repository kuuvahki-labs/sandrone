package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestHandlerServesIndexAndAssets(t *testing.T) {
	handler := testHandler()

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Sandrone")
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assetPath := regexp.MustCompile(`"(/assets/[^"]+\.js)"`).FindStringSubmatch(w.Body.String())
	require.Len(t, assetPath, 2)

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, assetPath[1], nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "javascript")
}

func TestHandlerUsesSPAFallbackForManagementRoutes(t *testing.T) {
	handler := testHandler()

	for _, path := range []string{"/setup", "/sources/demo", "/groups/default?advanced=processors"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusOK, w.Code)
			require.Contains(t, w.Body.String(), "Sandrone")
		})
	}
}

func TestHandlerDoesNotFallbackReservedRoutes(t *testing.T) {
	handler := testHandler()

	for _, path := range []string{"/v1/unknown", "/mcp", "/s/share-id"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestHandlerUsesConfiguredReservedPrefixes(t *testing.T) {
	handler := HandlerWithFS(fstest.MapFS{
		"index.html": {Data: []byte("<html>Sandrone</html>")},
	}, WithReservedPrefixes("/v1", "/agent", "/s"))

	for _, path := range []string{"/agent", "/agent/tools"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusNotFound, w.Code)
		})
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Sandrone")
}

func TestHandlerServesConfiguredStaticDir(t *testing.T) {
	staticDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(staticDir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(staticDir, "index.html"),
		[]byte(`<html><body>Custom Sandrone<script src="/assets/custom.js"></script></body></html>`),
		0o644,
	))
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "assets", "custom.js"), []byte(`console.log("custom")`), 0o644))
	handler := Handler(WithStaticDir(staticDir))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Custom Sandrone")

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/custom.js", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "custom")
}

func TestHandlerReturnsNotFoundWhenStaticAssetsAreMissing(t *testing.T) {
	handler := &handler{}

	for _, path := range []string{"/", "/setup", "/assets/app.js"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestPackageBuildsWithoutGeneratedStaticAssets(t *testing.T) {
	if os.Getenv("SANDRONE_WEBUI_OPTIONAL_STATIC_SUBPROCESS") == "1" {
		return
	}
	packageDir, err := os.Getwd()
	require.NoError(t, err)
	repoRoot := filepath.Clean(filepath.Join(packageDir, "..", "..", ".."))
	staticDir := filepath.Join(packageDir, "static")
	hiddenDir := filepath.Join(packageDir, "static.optional-test-hidden")
	require.NoFileExists(t, hiddenDir)
	if _, err := os.Stat(staticDir); err == nil {
		require.NoError(t, os.Rename(staticDir, hiddenDir))
		defer func() {
			require.NoError(t, os.Rename(hiddenDir, staticDir))
		}()
	} else if !os.IsNotExist(err) {
		require.NoError(t, err)
	}

	cmd := exec.Command("go", "test", "./internal/entry/webui")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "SANDRONE_WEBUI_OPTIONAL_STATIC_SUBPROCESS=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func testHandler() http.Handler {
	return HandlerWithFS(fstest.MapFS{
		"index.html": {
			Data: []byte(`<html><head><title>Sandrone</title></head><body><script type="module" src="/assets/app.js"></script></body></html>`),
		},
		"assets/app.js": {
			Data: []byte(`console.log("Sandrone")`),
		},
	})
}
