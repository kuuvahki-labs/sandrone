package mcpapi_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

func TestConvertRemoteBatchReportsAndSchema(t *testing.T) {
	var fetches atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		if r.URL.Path == "/bad" {
			http.Error(w, "unavailable", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#remote"))
	}))
	defer upstream.Close()
	session := connect(t, t.Context(), mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()
	for _, tc := range []struct {
		name    string
		sources []map[string]any
		extra   map[string]any
		failed  bool
		count   int
	}{
		{"partial", []map[string]any{{"url": upstream.URL}, {"url": upstream.URL + "/bad"}}, nil, false, 2},
		{"failed", []map[string]any{{"url": upstream.URL + "/bad"}}, nil, true, 1},
		{"content conflict", []map[string]any{{"url": upstream.URL}}, map[string]any{"content": "", "from_format": "uri-list"}, true, 0},
		{"remote conflict", []map[string]any{{"url": upstream.URL}}, map[string]any{"remote": map[string]any{"url": upstream.URL}}, true, 0},
		{"empty", []map[string]any{}, nil, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := fetches.Load()
			args := map[string]any{"to_format": "json-nodes", "remote_batch": map[string]any{"sources": tc.sources}}
			for key, value := range tc.extra {
				args[key] = value
			}
			result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "sandrone_convert", Arguments: args})
			require.NoError(t, err)
			require.Equal(t, tc.failed, result.IsError)
			var out struct {
				Report domain.Report `json:"report"`
				Body   string        `json:"body"`
			}
			decodeStructuredContent(t, result, &out)
			require.Len(t, out.Report.SourceResults, tc.count)
			if !tc.failed {
				require.Equal(t, "partial", out.Report.Status)
				require.Contains(t, out.Body, "remote")
			}
			if tc.count == 0 {
				require.Equal(t, before, fetches.Load())
			}
			if tc.count == 1 {
				require.Equal(t, "failed", out.Report.Status)
			}
		})
	}
}
