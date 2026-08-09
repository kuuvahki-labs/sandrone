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
	tempRepo := t.TempDir()
	copyFile := func(source, target string) {
		body, readErr := os.ReadFile(source)
		require.NoError(t, readErr)
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
		require.NoError(t, os.WriteFile(target, body, 0o644))
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		copyFile(filepath.Join(repoRoot, name), filepath.Join(tempRepo, name))
	}
	targetPackageDir := filepath.Join(tempRepo, "internal", "entry", "webui")
	entries, err := os.ReadDir(packageDir)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		copyFile(filepath.Join(packageDir, entry.Name()), filepath.Join(targetPackageDir, entry.Name()))
	}
	copyFile(
		filepath.Join(packageDir, "static", ".gitkeep"),
		filepath.Join(targetPackageDir, "static", ".gitkeep"),
	)

	cmd := exec.Command("go", "test", "./internal/entry/webui")
	cmd.Dir = tempRepo
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
