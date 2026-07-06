package httpapi_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestRuleSetCatalogEndpointReturnsRequestedTarget(t *testing.T) {
	server := newRuleSetCatalogServer(t, app.Config{}, testRuleSetCatalogGzip(t))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/rule-set-catalog?target=mihomo", nil))
	require.Equal(t, http.StatusOK, response.Code)
	var result service.RuleSetCatalogResult
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Equal(t, []service.RuleSetCatalogItem{
		{Name: "geosite-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs", RuleKind: "domain"},
		{Name: "geoip-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/cn.mrs", RuleKind: "ip"},
	}, result.Items)
	require.NotContains(t, response.Body.String(), "reference_type")

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/rule-set-catalog?target=sing-box", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Equal(t, []service.RuleSetCatalogItem{
		{Name: "geosite-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.srs", RuleKind: "domain"},
		{Name: "geoip-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/cn.srs", RuleKind: "ip"},
	}, result.Items)
	require.NotContains(t, response.Body.String(), "reference_type")

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/rule-set-catalog?target=shadowrocket", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Equal(t, []service.RuleSetCatalogItem{
		{Name: "Apple/Apple_Domain", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Apple/Apple_Domain.list", RuleKind: "domain", ReferenceType: "DOMAIN-SET"},
		{Name: "Global/Global", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Global/Global.list", RuleKind: "mixed", ReferenceType: "RULE-SET"},
	}, result.Items)
	require.Contains(t, response.Body.String(), `"reference_type": "DOMAIN-SET"`)
}

func TestRuleSetCatalogEndpointRequiresSupportedTarget(t *testing.T) {
	server := newRuleSetCatalogServer(t, app.Config{}, testRuleSetCatalogGzip(t))
	for _, requestURL := range []string{
		"/v1/rule-set-catalog",
		"/v1/rule-set-catalog?target=clash",
	} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, requestURL, nil))
		require.Equal(t, http.StatusBadRequest, response.Code, requestURL)
		var body struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		require.Equal(t, "invalid_argument", body.Error.Code)
		require.Equal(t, "rule-set catalog target must be mihomo, sing-box, or shadowrocket", body.Error.Message)
	}
}

func TestRuleSetCatalogEndpointIsUnavailableForMissingOrDamagedSnapshot(t *testing.T) {
	for name, snapshot := range map[string][]byte{"missing": nil, "damaged": []byte("not gzip")} {
		t.Run(name, func(t *testing.T) {
			server := newRuleSetCatalogServer(t, app.Config{}, snapshot)
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/rule-set-catalog?target=mihomo", nil))
			require.Equal(t, http.StatusServiceUnavailable, response.Code)
			require.Contains(t, response.Body.String(), "rule-set catalog is unavailable")
		})
	}
}

func TestRuleSetCatalogEndpointRequiresBearerAuth(t *testing.T) {
	server := newRuleSetCatalogServer(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}}, testRuleSetCatalogGzip(t))
	request := httptest.NewRequest(http.MethodGet, "/v1/rule-set-catalog?target=mihomo", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)

	request = httptest.NewRequest(http.MethodGet, "/v1/rule-set-catalog?target=mihomo", nil)
	request.Header.Set("Authorization", "Bearer secret")
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
}

func TestRuleSetCatalogPreviewAndRefreshRoutesDoNotExist(t *testing.T) {
	server := newRuleSetCatalogServer(t, app.Config{}, testRuleSetCatalogGzip(t))
	for _, requestURL := range []string{"/v1/rule-set-catalog/preview", "/v1/rule-set-catalog/refresh"} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, requestURL, nil))
		require.Equal(t, http.StatusNotFound, response.Code, requestURL)
	}
}

func newRuleSetCatalogServer(t *testing.T, cfg app.Config, snapshot []byte) *httpapi.Server {
	t.Helper()
	runtime := testRuntime(t, cfg)
	runtime.Service = service.New(service.WithRuleSetCatalogSnapshot(snapshot))
	return httpapi.New(runtime)
}

func testRuleSetCatalogGzip(t *testing.T) []byte {
	t.Helper()
	catalog := struct {
		Mihomo       []service.RuleSetCatalogItem `json:"mihomo"`
		SingBox      []service.RuleSetCatalogItem `json:"sing-box"`
		Shadowrocket []service.RuleSetCatalogItem `json:"shadowrocket"`
	}{
		Mihomo: []service.RuleSetCatalogItem{
			{Name: "geosite-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs", RuleKind: "domain"},
			{Name: "geoip-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/cn.mrs", RuleKind: "ip"},
		},
		SingBox: []service.RuleSetCatalogItem{
			{Name: "geosite-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.srs", RuleKind: "domain"},
			{Name: "geoip-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/cn.srs", RuleKind: "ip"},
		},
		Shadowrocket: []service.RuleSetCatalogItem{
			{Name: "Apple/Apple_Domain", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Apple/Apple_Domain.list", RuleKind: "domain", ReferenceType: "DOMAIN-SET"},
			{Name: "Global/Global", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Global/Global.list", RuleKind: "mixed", ReferenceType: "RULE-SET"},
		},
	}
	decoded, err := json.Marshal(catalog)
	require.NoError(t, err)
	var body bytes.Buffer
	writer := gzip.NewWriter(&body)
	_, err = writer.Write(decoded)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return body.Bytes()
}
