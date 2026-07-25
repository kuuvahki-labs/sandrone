package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/agentcatalog"
	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
)

func TestSchemaRoutesPublishServerCatalogs(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	handler := httpapi.New(rt).Handler()

	tests := []struct {
		path   string
		assert func(*testing.T, string)
	}{
		{"/v1/schemas/processors", func(t *testing.T, body string) {
			require.Contains(t, body, `"processors"`)
			require.Contains(t, body, `"rename"`)
		}},
		{"/v1/schemas/processors/nodes/rename", func(t *testing.T, body string) {
			require.Contains(t, body, `"params_schema"`)
			require.Contains(t, body, `"examples"`)
		}},
		{"/v1/schemas/file-kinds/mihomo", func(t *testing.T, body string) {
			require.Contains(t, body, `"kind": "mihomo"`)
			require.Contains(t, body, `"settings_schema"`)
		}},
		{"/v1/schemas/script-api/v1", func(t *testing.T, body string) {
			require.Contains(t, body, `"version": 1`)
			require.Contains(t, body, `"sandbox"`)
			require.Contains(t, body, `"api.ini.override"`)
		}},
		{"/v1/schemas/subscription", func(t *testing.T, body string) {
			require.Contains(t, body, `"name"`)
			require.Contains(t, body, `"processors"`)
		}},
		{"/v1/schemas/file-spec", func(t *testing.T, body string) {
			require.Contains(t, body, `"kind"`)
			require.Contains(t, body, `"config"`)
		}},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, test.path, nil))
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			test.assert(t, rec.Body.String())
		})
	}
}

func TestSchemaRoutesUseV1BearerBoundary(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	handler := httpapi.New(rt).Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet, "/v1/schemas/processors", nil,
	))
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	req := httptest.NewRequest(http.MethodGet, "/v1/schemas/processors", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestSchemaRoutesRejectUnknownCanonicalKeys(t *testing.T) {
	handler := httpapi.New(testRuntime(t, app.Config{})).Handler()
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCode   string
	}{
		{name: "unknown processor stage", method: http.MethodGet, path: "/v1/schemas/processors/output/rename", wantStatus: http.StatusBadRequest, wantCode: "invalid_argument"},
		{name: "unknown node processor", method: http.MethodGet, path: "/v1/schemas/processors/nodes/future", wantStatus: http.StatusBadRequest, wantCode: "invalid_argument"},
		{name: "unknown file processor", method: http.MethodGet, path: "/v1/schemas/processors/file/future", wantStatus: http.StatusBadRequest, wantCode: "invalid_argument"},
		{name: "unknown file kind", method: http.MethodGet, path: "/v1/schemas/file-kinds/future", wantStatus: http.StatusBadRequest, wantCode: "invalid_argument"},
		{name: "unknown script API version", method: http.MethodGet, path: "/v1/schemas/script-api/v2", wantStatus: http.StatusNotFound},
		{name: "processor summary extra segment", method: http.MethodGet, path: "/v1/schemas/processors/extra", wantStatus: http.StatusNotFound},
		{name: "processor detail extra segment", method: http.MethodGet, path: "/v1/schemas/processors/nodes/rename/extra", wantStatus: http.StatusNotFound},
		{name: "file kind extra segment", method: http.MethodGet, path: "/v1/schemas/file-kinds/mihomo/extra", wantStatus: http.StatusNotFound},
		{name: "script API extra segment", method: http.MethodGet, path: "/v1/schemas/script-api/v1/extra", wantStatus: http.StatusNotFound},
		{name: "subscription extra segment", method: http.MethodGet, path: "/v1/schemas/subscription/extra", wantStatus: http.StatusNotFound},
		{name: "file spec extra segment", method: http.MethodGet, path: "/v1/schemas/file-spec/extra", wantStatus: http.StatusNotFound},
		{name: "wrong method", method: http.MethodPost, path: "/v1/schemas/processors", wantStatus: http.StatusMethodNotAllowed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(test.method, test.path, nil))
			require.Equal(t, test.wantStatus, rec.Code)
			if test.wantCode != "" {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				require.Equal(t, test.wantCode, body.Error.Code)
			}
		})
	}
}

func TestSchemaRoutesMatchSharedCatalogDocuments(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	handler := httpapi.New(rt).Handler()

	assertJSONRoute(t, handler, "/v1/schemas/processors",
		agentcatalog.ProcessorSummary(rt.Service.Registry().PublicDescriptors()))
	for _, descriptor := range rt.Service.Registry().PublicDescriptors() {
		document, err := agentcatalog.ProcessorDetail(descriptor)
		require.NoError(t, err)
		assertJSONRoute(t, handler,
			"/v1/schemas/processors/"+string(descriptor.Stage)+"/"+descriptor.Type,
			document,
		)
	}
	for _, capability := range rt.Service.FileKindCapabilities() {
		document, err := agentcatalog.FileKindDetail(capability)
		require.NoError(t, err)
		assertJSONRoute(t, handler,
			"/v1/schemas/file-kinds/"+string(capability.Kind),
			document,
		)
	}
	scriptAPI, err := agentcatalog.ScriptAPI()
	require.NoError(t, err)
	assertJSONRoute(t, handler, "/v1/schemas/script-api/v1", scriptAPI)
	assertJSONRoute(t, handler, "/v1/schemas/subscription", agentcatalog.SubscriptionSchema())
	assertJSONRoute(t, handler, "/v1/schemas/file-spec", agentcatalog.FileSpecSchema(true))
}

func assertJSONRoute(t *testing.T, handler http.Handler, path string, want any) {
	t.Helper()
	expected, err := json.Marshal(want)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code, path)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"), path)
	require.JSONEq(t, string(expected), rec.Body.String(), path)
}
