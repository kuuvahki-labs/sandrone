package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
	projectsettings "github.com/kuuvahki-labs/sandrone/internal/settings"
)

func TestSettingsEndpointRoundTripOmitsRemovedStartupFields(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)

	update := settingsUpdate()
	update.HTTP.Listen = "127.0.0.1:2237"
	update.Appearance.ThemeMode = "light"
	update.Subscriptions.AutoLoadTraffic = true
	put := httptest.NewRecorder()
	server.Handler().ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/v1/settings", jsonBody(t, update)))
	require.Equal(t, http.StatusOK, put.Code)
	require.NotContains(t, put.Body.String(), `"token"`)
	require.NotContains(t, put.Body.String(), `"token_required"`)
	require.NotContains(t, put.Body.String(), `"token_configured"`)
	require.NotContains(t, put.Body.String(), `"transport"`)
	require.NotContains(t, put.Body.String(), `"webui"`)

	get := httptest.NewRecorder()
	server.Handler().ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/v1/settings", nil))
	require.Equal(t, http.StatusOK, get.Code)

	var snapshot domain.SettingsSnapshot
	require.NoError(t, json.Unmarshal(get.Body.Bytes(), &snapshot))
	require.Equal(t, "127.0.0.1:2237", snapshot.Settings.HTTP.Listen)
	require.Equal(t, "light", snapshot.Effective.Appearance.ThemeMode)
	require.True(t, snapshot.Effective.Subscriptions.AutoLoadTraffic)
	require.Contains(t, snapshot.RestartRequired, "http.listen")

	body, err := os.ReadFile(filepath.Join(rt.Config.DataDir, "settings.json"))
	require.NoError(t, err)
	require.NotContains(t, string(body), `"token"`)
	require.NotContains(t, string(body), `"transport"`)
	require.NotContains(t, string(body), `"webui"`)
}

func TestSettingsEndpointRejectsRemovedStartupFields(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)
	base, err := json.Marshal(settingsUpdate())
	require.NoError(t, err)

	for name, body := range map[string]string{
		"token":          strings.Replace(string(base), `"listen":"127.0.0.1:1137"`, `"listen":"127.0.0.1:1137","token":"secret"`, 1),
		"token required": strings.Replace(string(base), `"listen":"127.0.0.1:1137"`, `"listen":"127.0.0.1:1137","token_required":true`, 1),
		"transport":      strings.Replace(string(base), `"path":"/mcp"`, `"path":"/mcp","transport":"streamable-http"`, 1),
		"webui":          strings.Replace(string(base), `"log":`, `"webui":{"static_dir":"/tmp/static"},"log":`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/v1/settings", strings.NewReader(body)))
			require.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

func TestSettingsEndpointRejectsUnknownFieldWithoutChangingFile(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)

	update := settingsUpdate()
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/v1/settings", jsonBody(t, update)))
	require.Equal(t, http.StatusOK, response.Code)
	before, err := os.ReadFile(filepath.Join(rt.Config.DataDir, "settings.json"))
	require.NoError(t, err)

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(
		http.MethodPut,
		"/v1/settings",
		strings.NewReader(`{"schema_version":1,"future":true}`),
	))
	require.Equal(t, http.StatusBadRequest, response.Code)
	after, err := os.ReadFile(filepath.Join(rt.Config.DataDir, "settings.json"))
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestRuntimeSettingsEndpointIsRemoved(t *testing.T) {
	server := httpapi.New(testRuntime(t, app.Config{}))
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(method, "/v1/settings/runtime", bytes.NewReader(nil)))
		require.Equal(t, http.StatusNotFound, response.Code)
	}
}

func TestSettingsEndpointUsesActiveStartupAuthentication(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "active-secret"}})
	server := httpapi.New(rt)

	unauthorized := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/v1/settings", nil))
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)

	update := settingsUpdate()
	request := httptest.NewRequest(http.MethodPut, "/v1/settings", jsonBody(t, update))
	request.Header.Set("Authorization", "Bearer active-secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.NotContains(t, response.Body.String(), `"token"`)

	request = httptest.NewRequest(http.MethodGet, "/v1/settings", nil)
	request.Header.Set("Authorization", "Bearer next-start-secret")
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestSettingsEndpointExcludesOverriddenStartupPathsFromRestartRequired(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Listen: "127.0.0.1:3237"}})
	server := httpapi.New(rt)
	update := settingsUpdate()
	update.HTTP.Listen = "127.0.0.1:2237"

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/v1/settings", jsonBody(t, update)))
	require.Equal(t, http.StatusOK, response.Code)

	var snapshot domain.SettingsSnapshot
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &snapshot))
	require.Equal(t, "programmatic", snapshot.Overrides["http.listen"])
	require.Equal(t, "127.0.0.1:2237", snapshot.Settings.HTTP.Listen)
	require.Equal(t, "127.0.0.1:3237", snapshot.Effective.HTTP.Listen)
	require.NotContains(t, snapshot.RestartRequired, "http.listen")
}

func TestSettingsEndpointLimitsRequestBody(t *testing.T) {
	server := httpapi.New(testRuntime(t, app.Config{}))
	body := &countingSettingsBody{}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/v1/settings", body))
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.LessOrEqual(t, body.bytesRead, int64((16<<20)+1))
}

func settingsUpdate() domain.SettingsUpdate {
	value := projectsettings.Default()
	return domain.SettingsUpdate{
		SchemaVersion:  value.SchemaVersion,
		HTTP:           value.HTTP,
		MCP:            value.MCP,
		Log:            value.Log,
		RemoteDefaults: value.RemoteDefaults,
		ProbeDefaults:  value.ProbeDefaults,
		CacheDefaults:  value.CacheDefaults,
		Appearance:     value.Appearance,
		Subscriptions:  value.Subscriptions,
	}
}

func jsonBody(t *testing.T, value any) *bytes.Reader {
	t.Helper()
	body, err := json.Marshal(value)
	require.NoError(t, err)
	return bytes.NewReader(body)
}

type countingSettingsBody struct {
	bytesRead int64
}

func (r *countingSettingsBody) Read(body []byte) (int, error) {
	clear(body)
	r.bytesRead += int64(len(body))
	return len(body), nil
}

func (*countingSettingsBody) Close() error {
	return nil
}

var _ io.ReadCloser = (*countingSettingsBody)(nil)
