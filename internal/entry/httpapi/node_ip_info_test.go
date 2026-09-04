package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
)

func TestNodeInspectRouteReturnsOnlyRequestedPrivateIPInformation(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	handler := httpapi.New(rt).Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/nodes/inspect", bytes.NewBufferString(`{
		"node":{"name":"fixture","type":"trojan","server":"198.18.0.1","port":443,"password":"fixture-password"},
		"include":["ip"]
	}`)))

	require.Equal(t, http.StatusOK, recorder.Code)
	var result domain.NodeInspectResult
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
	require.Nil(t, result.URI)
	require.Equal(t, "198.18.0.1", result.IP.IP)
	require.False(t, result.IP.Public)
	require.Nil(t, result.IP.Source)
}

func TestNodeInspectRouteReturnsOnlyRequestedURIInformation(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	handler := httpapi.New(rt).Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/nodes/inspect", bytes.NewBufferString(`{
		"node":{"name":"fixture","type":"trojan","server":"proxy.example.com","port":443,"password":"fixture-password"},
		"include":["uri"]
	}`)))

	require.Equal(t, http.StatusOK, recorder.Code)
	var result domain.NodeInspectResult
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
	require.Nil(t, result.IP)
	require.NotNil(t, result.URI)
	require.Equal(t, "trojan://fixture-password@proxy.example.com:443#fixture", result.URI.Value)
}

func TestNodeInspectRouteMapsStableErrorsAndRequiresAuthentication(t *testing.T) {
	tests := []struct {
		name       string
		cfg        app.Config
		body       string
		auth       string
		wantStatus int
		wantCode   string
	}{
		{name: "invalid include", cfg: app.Config{}, body: `{"node":{},"include":["unknown"]}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_argument"},
		{name: "unauthorized", cfg: app.Config{HTTP: app.HTTPConfig{Token: "secret"}}, body: `{"node":{"server":"198.18.0.1"},"include":["ip"]}`, wantStatus: http.StatusUnauthorized, wantCode: "invalid_argument"},
		{name: "authorized", cfg: app.Config{HTTP: app.HTTPConfig{Token: "secret"}}, body: `{"node":{"server":"198.18.0.1"},"include":["ip"]}`, auth: "Bearer secret", wantStatus: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := httpapi.New(testRuntime(t, test.cfg)).Handler()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/nodes/inspect", bytes.NewBufferString(test.body))
			if test.auth != "" {
				request.Header.Set("Authorization", test.auth)
			}
			handler.ServeHTTP(recorder, request)

			require.Equal(t, test.wantStatus, recorder.Code)
			if test.wantCode != "" {
				var response struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, test.wantCode, response.Error.Code)
			}
		})
	}
}
