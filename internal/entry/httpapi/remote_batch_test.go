package httpapi_test

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
)

func TestAgentConvertRemoteBatch(t *testing.T) {
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
	handler := httpapi.New(testRuntime(t, app.Config{})).Handler()
	for _, tc := range []struct {
		name, extra, sources string
		status               int
		results              int
	}{
		{"partial", "", fmt.Sprintf(`[{"url":%q},{"url":%q}]`, upstream.URL+"/ok", upstream.URL+"/bad"), 200, 2},
		{"failed", "", fmt.Sprintf(`[{"url":%q}]`, upstream.URL+"/bad"), 500, 1},
		{"content conflict", `,"content":""`, fmt.Sprintf(`[{"url":%q}]`, upstream.URL), 400, 0},
		{"remote conflict", fmt.Sprintf(`,"remote":{"url":%q}`, upstream.URL), fmt.Sprintf(`[{"url":%q}]`, upstream.URL), 400, 0},
		{"empty", "", `[]`, 400, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := fetches.Load()
			body := fmt.Sprintf(`{"to_format":"json-nodes","remote_batch":{"sources":%s}%s}`, tc.sources, tc.extra)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/convert", strings.NewReader(body)))
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			var out agentRenderResult
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
			require.Len(t, out.Report.SourceResults, tc.results)
			if tc.status == 200 {
				require.Equal(t, "partial", out.Report.Status)
				require.Contains(t, out.Body, "remote")
			}
			if tc.results == 0 {
				require.Equal(t, before, fetches.Load())
			}
			if tc.status == 500 {
				require.Equal(t, "failed", out.Report.Status)
				require.Contains(t, rec.Body.String(), `"error"`)
			}
		})
	}
}
