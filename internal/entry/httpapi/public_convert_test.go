package httpapi_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
	"github.com/kuuvahki-labs/sandrone/internal/fetcher"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

const publicConvertNode = "ss://aes-128-gcm:secret@example.com:8388#node-a"

func TestPublicConvertReturnsRawContentWithoutAuthentication(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	server := httpapi.New(rt)
	query := url.Values{
		"content":   {publicConvertNode},
		"to_format": {"mihomo-proxies"},
	}
	req := httptest.NewRequest(http.MethodGet, "/convert?"+query.Encode(), nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/yaml", w.Header().Get("Content-Type"))
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	require.Contains(t, w.Body.String(), "node-a")
}

func TestPublicConvertReturnsJSONEnvelope(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)
	query := url.Values{
		"content":     {publicConvertNode},
		"from_format": {"uri-list"},
		"to_format":   {"json-nodes"},
		"response":    {"JSON"},
	}
	req := httptest.NewRequest(http.MethodGet, "/convert?"+query.Encode(), nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var response struct {
		ContentType string           `json:"content_type"`
		Body        string           `json:"body"`
		Warnings    []domain.Warning `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "application/json", response.ContentType)
	require.Empty(t, response.Warnings)
	require.Contains(t, response.Body, "node-a")
}

func TestPublicConvertJSONEnvelopeIncludesWarnings(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)
	content := "proxies:\n  - name: ss\n    type: ss\n    server: example.com\n    port: 8388\n    cipher: aes-128-gcm\n    password: secret\n    private-thing: value\n"
	query := url.Values{
		"content":     {content},
		"from_format": {"mihomo"},
		"to_format":   {"json-nodes"},
		"response":    {"json"},
	}
	req := httptest.NewRequest(http.MethodGet, "/convert?"+query.Encode(), nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var response struct {
		Warnings []domain.Warning `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Warnings, 1)
	require.Equal(t, "parse_unknown_field", response.Warnings[0].Code)
}

func TestPublicConvertSupportsEveryRenderer(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	capabilities, err := rt.Service.ListFormatCapabilities(t.Context())
	require.NoError(t, err)
	formats := []string{}
	for _, capability := range capabilities.Items {
		if capability.Direction == domain.CapabilityDirectionRender {
			formats = append(formats, capability.Format)
		}
	}
	require.NotEmpty(t, formats)
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			server := httpapi.New(rt)
			query := url.Values{
				"content":   {publicConvertNode},
				"to_format": {format},
			}
			req := httptest.NewRequest(http.MethodGet, "/convert?"+query.Encode(), nil)
			w := httptest.NewRecorder()

			server.Handler().ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			require.NotEmpty(t, w.Header().Get("Content-Type"))
			require.NotEmpty(t, w.Body.Bytes())
		})
	}
}

func TestPublicConvertFetchesRemoteSubscription(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(publicConvertNode))
	}))
	defer upstream.Close()

	remoteFetcher := fetcher.New(
		fetcher.WithResolver(httpStaticResolver{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}),
		fetcher.WithDialContext(func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, upstream.Listener.Addr().String())
		}),
	)
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	rt.Service = service.New(service.WithFetcher(remoteFetcher))
	server := httpapi.New(rt)
	query := url.Values{
		"url":         {"http://subscription.example/sub"},
		"from_format": {"uri-list"},
		"to_format":   {"json-nodes"},
		"response":    {"json"},
	}
	req := httptest.NewRequest(http.MethodGet, "/convert?"+query.Encode(), nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "node-a")
}

func TestPublicConvertRejectsPrivateRemoteSubscription(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)
	query := url.Values{
		"url":       {"http://127.0.0.1/sub"},
		"to_format": {"json-nodes"},
	}
	req := httptest.NewRequest(http.MethodGet, "/convert?"+query.Encode(), nil)
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "public")
}

func TestPublicConvertRejectsInvalidQuery(t *testing.T) {
	tooLarge := strings.Repeat("x", (64<<10)+1)
	tests := []struct {
		name  string
		query url.Values
	}{
		{name: "missing input", query: url.Values{"to_format": {"json-nodes"}}},
		{name: "both inputs", query: url.Values{"content": {publicConvertNode}, "url": {"https://example.com/sub"}, "to_format": {"json-nodes"}}},
		{name: "missing target", query: url.Values{"content": {publicConvertNode}}},
		{name: "empty input", query: url.Values{"content": {""}, "to_format": {"json-nodes"}}},
		{name: "unknown parameter", query: url.Values{"content": {publicConvertNode}, "to_format": {"json-nodes"}, "processors": {"[]"}}},
		{name: "duplicate parameter", query: url.Values{"content": {publicConvertNode}, "to_format": {"json-nodes", "uri-list"}}},
		{name: "oversized content", query: url.Values{"content": {tooLarge}, "to_format": {"json-nodes"}}},
		{name: "unsupported input format", query: url.Values{"content": {publicConvertNode}, "from_format": {"unknown"}, "to_format": {"json-nodes"}}},
		{name: "unsupported output format", query: url.Values{"content": {publicConvertNode}, "to_format": {"unknown"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := testRuntime(t, app.Config{})
			server := httpapi.New(rt)
			req := httptest.NewRequest(http.MethodGet, "/convert?"+tt.query.Encode(), nil)
			w := httptest.NewRecorder()

			server.Handler().ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Header().Get("Content-Type"), "application/json")
			require.Contains(t, w.Body.String(), `"error"`)
		})
	}
}

func TestPublicPostConvertIsNotAvailable(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)
	req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader(`{
		"from_format": "uri-list",
		"to_format": "json-nodes",
		"content": "`+publicConvertNode+`"
	}`))
	w := httptest.NewRecorder()

	server.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

type httpStaticResolver struct {
	addresses []net.IPAddr
}

func (r httpStaticResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	return r.addresses, nil
}
