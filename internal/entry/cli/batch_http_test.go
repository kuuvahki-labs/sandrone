//go:build probe_singbox

package cli

import (
	"encoding/json/v2"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

func TestBatchConvertThroughProxyWithExitPipeline(t *testing.T) {
	var requests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect || r.Host != "exit.invalid:80" {
			http.Error(w, "unexpected proxy request", http.StatusBadRequest)
			return
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = rw.Flush()
		req, err := http.ReadRequest(rw.Reader)
		if err != nil {
			return
		}
		defer req.Body.Close()
		if req.Method != http.MethodGet || req.URL.Path != "/trace" {
			return
		}
		requests.Add(1)
		body := "ip=8.8.8.8\nloc=US\n"
		_, _ = fmt.Fprintf(rw, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
		_ = rw.Flush()
	}))
	defer proxy.Close()
	host, port, err := net.SplitHostPort(strings.TrimPrefix(proxy.URL, "http://"))
	require.NoError(t, err)
	portValue, err := strconv.Atoi(port)
	require.NoError(t, err)
	nodes := []map[string]any{{"name": "original", "type": "http", "server": host, "port": portValue}}
	nodeBody, err := json.Marshal(nodes)
	require.NoError(t, err)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(nodeBody) }))
	defer source.Close()
	script, err := os.ReadFile("../../../examples/scripts/exit-info.js")
	require.NoError(t, err)
	processors, err := json.Marshal([]map[string]any{
		{"type": "dedup", "stage": "nodes", "params": map[string]any{"strategy": "connection"}},
		{"type": "script", "stage": "nodes", "params": map[string]any{
			"timeout_ms": 10000,
			"args":       map[string]any{"url": "http://exit.invalid/trace"},
			"source":     map[string]any{"type": "inline", "content": string(script)},
		}},
	})
	require.NoError(t, err)
	dir := t.TempDir()
	processorsPath, outputPath, reportPath := filepath.Join(dir, "pipeline.json"), filepath.Join(dir, "nodes.json"), filepath.Join(dir, "report.json")
	require.NoError(t, os.WriteFile(processorsPath, processors, 0o600))
	code, stdout, stderr := runCLI(t, []string{
		"--data-dir", filepath.Join(dir, "data"), "convert", "--input-urls", "-", "--from", "json-nodes",
		"--processors", processorsPath, "--to", "json-nodes", "--output", outputPath, "--report-output", reportPath,
	}, source.URL+"\n"+source.URL+"\n")
	require.Equal(t, 0, code, stderr)
	require.Empty(t, stdout)
	body, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	var result []sandrone.NodeIR
	require.NoError(t, json.Unmarshal(body, &result))
	require.Len(t, result, 1)
	require.Equal(t, "US", result[0].Meta["exit.country"])
	require.Equal(t, "8.8.8.8", result[0].Meta["exit.ip"])
	require.NotEqual(t, "original", result[0].Name)
	require.EqualValues(t, 1, requests.Load(), "cross-source deduplication must precede the single GET")
	body, err = os.ReadFile(reportPath)
	require.NoError(t, err)
	var report sandrone.Report
	require.NoError(t, json.Unmarshal(body, &report))
	require.Len(t, report.SourceResults, 2)
	require.Equal(t, 1, report.SourceResults[0].NodeCount)
	require.Equal(t, 1, report.SourceResults[1].NodeCount)
}
