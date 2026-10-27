package service

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
)

// Each core's build-tagged test file records the expected compiled capability.
// Availability assertions must not derive their expectation from the tested API.
var nodeHTTPCompiledCores []string

func TestNodeHTTPOptionsValidation(t *testing.T) {
	defaults, err := normalizeNodeHTTPOptions(domain.NodeHTTPOptions{URL: "https://example.com/"})
	require.NoError(t, err)
	require.Equal(t, "GET", defaults.Method)
	require.Equal(t, "sing-box", defaults.Core)
	require.Equal(t, 5000, defaults.TimeoutMS)
	require.Equal(t, 4, defaults.Concurrency)
	require.EqualValues(t, 65536, defaults.MaxResponseBytes)
	for _, core := range []string{"sing-box", "mihomo"} {
		normalized, err := normalizeNodeHTTPOptions(domain.NodeHTTPOptions{URL: "https://example.com/", Core: core})
		require.NoError(t, err)
		require.Equal(t, core, normalized.Core)
	}
	for name, opts := range map[string]domain.NodeHTTPOptions{
		"port":         {URL: "https://example.com:99999/"},
		"zero port":    {URL: "https://example.com:0/"},
		"url":          {URL: "file:///tmp/target"},
		"credentials":  {URL: "https://user:pass@example.com/"},
		"fragment":     {URL: "https://example.com/#fragment"},
		"core":         {URL: "https://example.com/", Core: "unknown"},
		"method":       {URL: "https://example.com/", Method: "POST"},
		"timeout":      {URL: "https://example.com/", TimeoutMS: -1},
		"concurrency":  {URL: "https://example.com/", Concurrency: -1},
		"limit":        {URL: "https://example.com/", MaxResponseBytes: -1},
		"header name":  {URL: "https://example.com/", Headers: map[string]string{"bad header": "value"}},
		"header value": {URL: "https://example.com/", Headers: map[string]string{"X-Test": "value\r\ninjection"}},
	} {
		t.Run(name, func(t *testing.T) { _, err := normalizeNodeHTTPOptions(opts); require.Error(t, err) })
	}
}

func TestNodeHTTPDisabled(t *testing.T) {
	svc := New(WithProbeEngine(probe.NewDisabled()))
	for _, core := range []string{"", "sing-box", "mihomo"} {
		t.Run(core, func(t *testing.T) {
			_, err := svc.RequestNodeHTTP(context.Background(), nil, domain.NodeHTTPOptions{URL: "https://example.com/", Core: core})
			require.True(t, domain.IsCode(err, domain.CodeProbeBackendUnavailable), "%v", err)
		})
	}
}

func TestNodeHTTPAvailabilityByCore(t *testing.T) {
	for _, core := range []string{"", "sing-box", "mihomo"} {
		t.Run(core, func(t *testing.T) {
			selected := core
			if selected == "" {
				selected = "sing-box"
			}
			available := slices.Contains(nodeHTTPCompiledCores, selected)
			if core != "" {
				require.Equal(t, available, probe.NodeHTTPAvailable(core))
			}
			svc := New()
			result, err := svc.RequestNodeHTTP(t.Context(), nil, domain.NodeHTTPOptions{URL: "https://example.com/", Core: core})
			if available {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Empty(t, result)
				return
			}
			require.True(t, domain.IsCode(err, domain.CodeProbeBackendUnavailable), "%v", err)
			for _, nodes := range [][]domain.NodeIR{
				{{Name: "n", Type: domain.NodeTypeHTTP, Server: "127.0.0.1", Port: 80}},
				{{Name: "n", Type: domain.NodeType("unsupported"), Server: "127.0.0.1", Port: 80}},
			} {
				_, err = svc.RequestNodeHTTP(t.Context(), nodes, domain.NodeHTTPOptions{URL: "https://example.com/", Core: core})
				require.True(t, domain.IsCode(err, domain.CodeProbeBackendUnavailable), "%v", err)
			}
		})
	}
}
