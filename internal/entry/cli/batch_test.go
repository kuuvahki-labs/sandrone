package cli

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

func TestConvertURLListAndProcessors(t *testing.T) {
	dir := t.TempDir()
	processors := filepath.Join(dir, "processors.json")
	require.NoError(t, os.WriteFile(processors, []byte(`[{"type":"dedup","stage":"nodes","params":{"strategy":"connection"}}]`), 0o600))
	rec := &recordingEngine{convertResult: &sandrone.RenderResult{Body: []byte("[]\n")}}
	code, out, stderr := runCLI(t, []string{
		"convert", "--input-urls", "-", "--to", "json-nodes", "--from", "uri-list",
		"--processors", processors, "--user-agent", "batch-test", "--proxy", "http://127.0.0.1:8080", "--remote-timeout", "3s",
	}, "\r\n https://example.com/one?token=x \r\nhttps://example.com/two\n", WithEngineFactory(func(string) engine { return rec }))
	require.Equal(t, 0, code, stderr)
	require.Equal(t, "[]\n", out)
	require.Len(t, rec.convertRequests, 1)
	req := rec.convertRequests[0]
	require.Nil(t, req.Remote)
	require.Empty(t, req.Content)
	require.Equal(t, "uri-list", req.FromFormat)
	require.Len(t, req.RemoteBatch.Sources, 2)
	require.Equal(t, "https://example.com/one?token=x", req.RemoteBatch.Sources[0].URL)
	for _, remote := range req.RemoteBatch.Sources {
		require.Equal(t, "batch-test", remote.UserAgent)
		require.Equal(t, "http://127.0.0.1:8080", remote.Proxy)
		require.Equal(t, 3000, remote.TimeoutMS)
	}
	require.Len(t, req.ParseProcessors, 1)
	require.Equal(t, "dedup", req.ParseProcessors[0].Type)
}

func TestConvertURLListRejectsAmbiguousInputsBeforeExecution(t *testing.T) {
	for _, test := range []struct {
		name  string
		flags []string
		input string
		want  string
	}{
		{"local", []string{"--input", "-"}, "https://example.com/sub", "mutually exclusive"},
		{"remote", []string{"--input-url", "https://example.com/sub"}, "https://example.com/sub", "mutually exclusive"},
		{"stdin", []string{"--processors", "-"}, "https://example.com/sub", "cannot both read from stdin"},
		{"bad line", nil, "https://example.com/sub\n\nss://not-a-subscription", "line 3"},
		{"blank", nil, " \r\n", "at least one"},
		{"credentials", nil, "https://user:password@example.com/sub", "line 1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := &recordingEngine{}
			args := append([]string{"convert", "--input-urls", "-", "--to", "json-nodes"}, test.flags...)
			code, out, stderr := runCLI(t, args, test.input, WithEngineFactory(func(string) engine { return rec }))
			require.Equal(t, 1, code)
			require.Empty(t, out)
			require.Contains(t, stderr, test.want)
			require.Empty(t, rec.convertRequests)
		})
	}
}

func TestConvertURLListFileWithProcessorsOnStdin(t *testing.T) {
	list := filepath.Join(t.TempDir(), "urls.txt")
	require.NoError(t, os.WriteFile(list, []byte("https://example.com/sub\n"), 0o600))
	rec := &recordingEngine{convertResult: &sandrone.RenderResult{Body: []byte("[]")}}
	code, _, stderr := runCLI(t, []string{"convert", "--input-urls", list, "--processors", "-", "--to", "json-nodes"},
		`[{"type":"rename","stage":"nodes","params":{"mode":"prefix","value":"batch-"}}]`, WithEngineFactory(func(string) engine { return rec }))
	require.Equal(t, 0, code, stderr)
	require.Len(t, rec.convertRequests[0].RemoteBatch.Sources, 1)
	require.Len(t, rec.convertRequests[0].ParseProcessors, 1)
}

func TestConvertLocalInputCanUseProcessors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodes.txt")
	require.NoError(t, os.WriteFile(path, []byte("ss://aes-128-gcm:secret@example.com:8388#original"), 0o600))
	code, out, stderr := runCLI(t, []string{"convert", "--input", path, "--from", "uri-list", "--processors", "-", "--to", "json-nodes"},
		`[{"type":"rename","stage":"nodes","params":{"mode":"prefix","value":"processed-"}}]`)
	require.Equal(t, 0, code, stderr)
	var nodes []sandrone.NodeIR
	require.NoError(t, json.Unmarshal([]byte(out), &nodes))
	require.Len(t, nodes, 1)
	require.Equal(t, "processed-original", nodes[0].Name)
}

func TestDiagnoseURLListMapsBatchAndProcessors(t *testing.T) {
	rec := &recordingEngine{}
	code, _, stderr := runCLI(t, []string{"diagnose", "input", "-", "--kind", "url-list", "--user-agent", "diagnose-list", "--format", "uri-list"},
		"https://example.com/a\nhttps://example.com/b", WithEngineFactory(func(string) engine { return rec }))
	require.Equal(t, 0, code, stderr)
	require.Len(t, rec.diagnoseRequests, 1)
	req := rec.diagnoseRequests[0]
	require.Equal(t, sandrone.DiagnoseInputURLList, req.Kind)
	require.Empty(t, req.Content)
	require.Equal(t, "uri-list", req.Format)
	require.Len(t, req.RemoteBatch.Sources, 2)
	require.Equal(t, "diagnose-list", req.RemoteBatch.Sources[1].UserAgent)
}

func TestConvertBatchProcessesMergedNodesAndWritesSourceReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/one":
			_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#first"))
		case "/two":
			_, _ = w.Write([]byte(`{"proxies":[{"name":"duplicate","type":"ss","server":"example.com","port":8388,"cipher":"aes-128-gcm","password":"secret"},{"name":"second","type":"ss","server":"other.example.com","port":8388,"cipher":"aes-128-gcm","password":"secret"}]}`))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	processors := filepath.Join(dir, "processors.json")
	require.NoError(t, os.WriteFile(processors, []byte(`[{"type":"dedup","stage":"nodes","params":{"strategy":"connection"}}]`), 0o600))
	reportPath := filepath.Join(dir, "report.json")
	list := server.URL + "/one\n" + server.URL + "/two\n" + server.URL + "/missing?token=hidden\n"
	code, out, stderr := runCLI(t, []string{"--data-dir", filepath.Join(dir, "data"), "convert", "--input-urls", "-", "--processors", processors, "--to", "json-nodes", "--report-output", reportPath}, list)
	require.Equal(t, 0, code, stderr)
	var nodes []sandrone.NodeIR
	require.NoError(t, json.Unmarshal([]byte(out), &nodes))
	require.Len(t, nodes, 2)
	require.Equal(t, "first", nodes[0].Name)
	require.Equal(t, "second", nodes[1].Name)
	body, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	var report sandrone.Report
	require.NoError(t, json.Unmarshal(body, &report))
	require.Equal(t, "partial", report.Status)
	require.Len(t, report.SourceResults, 3)
	require.Equal(t, 2, report.SourceResults[1].NodeCount)
	require.Equal(t, "failed", report.SourceResults[2].FetchStatus)
	require.NotContains(t, string(body), "hidden")
}

func TestConvertBatchFailurePreservesOldOutputAndWritesReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	dir := t.TempDir()
	output, reportPath := filepath.Join(dir, "nodes.json"), filepath.Join(dir, "report.json")
	require.NoError(t, os.WriteFile(output, []byte("old output"), 0o600))
	code, out, stderr := runCLI(t, []string{"--data-dir", filepath.Join(dir, "data"), "convert", "--input-urls", "-", "--to", "json-nodes", "--output", output, "--report-output", reportPath}, server.URL)
	require.Equal(t, 1, code, stderr)
	require.Empty(t, out)
	body, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Equal(t, "old output", string(body))
	body, err = os.ReadFile(reportPath)
	require.NoError(t, err)
	var report sandrone.Report
	require.NoError(t, json.Unmarshal(body, &report))
	require.Equal(t, "failed", report.Status)
	require.Len(t, report.SourceResults, 1)
	require.NotNil(t, report.SourceResults[0].Error)
}

func TestAtomicOutputReplacesFileWithoutTruncatingPreviousInode(t *testing.T) {
	dir := t.TempDir()
	output, oldLink := filepath.Join(dir, "nodes.json"), filepath.Join(dir, "old.json")
	require.NoError(t, os.WriteFile(output, []byte("old"), 0o600))
	require.NoError(t, os.Link(output, oldLink))
	require.NoError(t, writeAtomicOutput(output, []byte("new"), 0o644))
	body, err := os.ReadFile(oldLink)
	require.NoError(t, err)
	require.Equal(t, "old", string(body))
	body, err = os.ReadFile(output)
	require.NoError(t, err)
	require.Equal(t, "new", string(body))
	info, err := os.Stat(output)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, file := range files {
		require.False(t, strings.HasPrefix(file.Name(), ".sandrone-output-"))
	}
}
