package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
)

func TestFormatCapabilityRoutesPublishIndexAndExactDetail(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	handler := httpapi.New(rt).Handler()

	indexRecorder := httptest.NewRecorder()
	handler.ServeHTTP(indexRecorder, httptest.NewRequest(http.MethodGet, "/v1/capabilities/formats", nil))
	require.Equal(t, http.StatusOK, indexRecorder.Code)
	var index struct {
		Items []struct {
			domain.FormatCapabilitySummary
			Href string `json:"href"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(indexRecorder.Body.Bytes(), &index))
	require.Len(t, index.Items, 12)
	require.Equal(t, "/v1/capabilities/formats/parse/base64", index.Items[0].Href)
	require.NotEmpty(t, index.Items[0].NodeTypes)

	detailRecorder := httptest.NewRecorder()
	handler.ServeHTTP(detailRecorder, httptest.NewRequest(http.MethodGet, "/v1/capabilities/formats/render/mihomo-proxies", nil))
	require.Equal(t, http.StatusOK, detailRecorder.Code)
	var detail domain.FormatCapability
	require.NoError(t, json.Unmarshal(detailRecorder.Body.Bytes(), &detail))
	require.Equal(t, domain.CapabilityDirectionRender, detail.Direction)
	require.Equal(t, "mihomo-proxies", detail.Format)
	require.NotEmpty(t, detail.Fields)

	serviceDetail, err := rt.Service.GetFormatCapability(t.Context(), domain.FormatCapabilityRequest{
		Direction: domain.CapabilityDirectionRender,
		Format:    "mihomo-proxies",
	})
	require.NoError(t, err)
	require.Equal(t, serviceDetail, &detail)
}

func TestFormatCapabilityRoutesRejectUnknownCanonicalKeys(t *testing.T) {
	handler := httpapi.New(testRuntime(t, app.Config{})).Handler()
	for _, path := range []string{
		"/v1/capabilities/formats/future/uri-list",
		"/v1/capabilities/formats/render/future",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusBadRequest, recorder.Code, path)
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		require.Equal(t, "invalid_argument", body.Error.Code)
	}
}
