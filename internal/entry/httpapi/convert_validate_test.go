package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
)

func TestInspectAndFilesEndpoints(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)

	inspectReq := httptest.NewRequest(http.MethodGet, "/v1/inspect", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, inspectReq)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"formats"`)
	require.Contains(t, w.Body.String(), `"catalogs"`)
	require.NotContains(t, w.Body.String(), `"capabilities"`)
	require.Less(t, w.Body.Len(), 8<<10)

	fileReq := httptest.NewRequest(http.MethodPost, "/v1/files", bytes.NewBufferString(`{
		"name": "out.yaml",
		"kind": "static",
		"source": {"type": "inline", "content": "hello: true\n"}
	}`))
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, fileReq)
	require.Equal(t, http.StatusCreated, w.Code)

	fileGetReq := httptest.NewRequest(http.MethodGet, "/v1/files/out.yaml", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, fileGetReq)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	require.Contains(t, w.Body.String(), "hello: true")

	fileSpecReq := httptest.NewRequest(http.MethodGet, "/v1/files/out.yaml?mode=spec", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, fileSpecReq)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"name": "out.yaml"`)

	fileJSONReq := httptest.NewRequest(http.MethodGet, "/v1/files/out.yaml?response=json", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, fileJSONReq)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"body": "hello: true`)

	argsFileReq := httptest.NewRequest(http.MethodPost, "/v1/files", bytes.NewBufferString(`{
		"name": "args.txt",
		"kind": "static",
		"source": {"type": "inline", "content": "hello"},
		"processors": [
			{"type": "script", "stage": "file", "params": {"source": {"type": "inline", "content": "function main(input) { input.file.content = input.file.content + ':' + input.args.foo; return input; }"}}}
		]
	}`))
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, argsFileReq)
	require.Equal(t, http.StatusCreated, w.Code)

	argsFileGetReq := httptest.NewRequest(http.MethodGet, "/v1/files/args.txt?response=json&arg.foo=bar", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, argsFileGetReq)
	require.Equal(t, http.StatusOK, w.Code)
	var argsFileResult struct {
		Body string `json:"body"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &argsFileResult))
	require.Equal(t, "hello:bar", argsFileResult.Body)

}

func TestRemovedProbeAndValidateRoutesReturnNotFound(t *testing.T) {
	handler := httpapi.New(testRuntime(t, app.Config{})).Handler()
	for _, path := range []string{"/v1/probe", "/v1/validate"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{}`)))
		require.Equal(t, http.StatusNotFound, recorder.Code, path)
	}
}
