package iplookup

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIPWhoisClientLooksUpOnlyTheRequestedIP(t *testing.T) {
	var requestPath, requestFields string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		requestFields = r.URL.Query().Get("fields")
		_, _ = w.Write([]byte(`{
			"ip":"8.8.8.8",
			"success":true,
			"country_code":"US",
			"country":"United States",
			"continent_code":"NA",
			"continent":"North America",
			"connection":{"asn":15169,"org":"Google LLC","domain":"google.com"}
		}`))
	}))
	t.Cleanup(server.Close)
	client := testClient(t, server)

	result, err := client.Lookup(t.Context(), netip.MustParseAddr("8.8.8.8"))

	require.NoError(t, err)
	require.Equal(t, "/8.8.8.8", requestPath)
	require.Equal(t, ipwhoisFields, requestFields)
	require.Equal(t, Attribution{
		CountryCode: "US", Country: "United States",
		ContinentCode: "NA", Continent: "North America",
		ASN: "AS15169", ASName: "Google LLC", ASDomain: "google.com",
	}, result)
}

func TestIPWhoisClientRejectsProviderFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"success":false,"message":"Rate limit exceeded"}`},
		{name: "server failure", status: http.StatusInternalServerError, body: `{"success":false}`},
		{name: "application failure", status: http.StatusOK, body: `{"success":false,"message":"Reserved range"}`},
		{name: "malformed JSON", status: http.StatusOK, body: `{`},
		{name: "wrong IP", status: http.StatusOK, body: `{"ip":"1.1.1.1","success":true,"country_code":"US","country":"United States","continent_code":"NA","continent":"North America","connection":{"asn":1}}`},
		{name: "incomplete", status: http.StatusOK, body: `{"ip":"8.8.8.8","success":true}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)

			_, err := testClient(t, server).Lookup(t.Context(), netip.MustParseAddr("8.8.8.8"))

			require.Error(t, err)
			require.NotContains(t, err.Error(), "Rate limit exceeded")
			require.NotContains(t, err.Error(), "Reserved range")
		})
	}
}

func TestIPWhoisClientRejectsOversizedResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", maxResponseBytes+1)))
	}))
	t.Cleanup(server.Close)

	_, err := testClient(t, server).Lookup(t.Context(), netip.MustParseAddr("8.8.8.8"))

	require.EqualError(t, err, "ipwho.is response is too large")
}

func TestIPWhoisClientSanitizesTransportErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := testClient(t, server)
	server.Close()

	_, err := client.Lookup(t.Context(), netip.MustParseAddr("8.8.8.8"))

	require.EqualError(t, err, "ipwho.is request failed")
}

func testClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	endpoint, err := url.Parse(server.URL + "/")
	require.NoError(t, err)
	client := NewIPWhois()
	client.endpoint = endpoint
	client.http = server.Client()
	return client
}
