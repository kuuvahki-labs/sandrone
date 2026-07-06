package httpapi_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
)

func TestValidateInspectAndFilesEndpoints(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)

	validateReq := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewBufferString(`{
		"format": "uri-list",
		"content": "ss://aes-128-gcm:secret@example.com:8388#node-a"
	}`))
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, validateReq)
	require.Equal(t, http.StatusOK, w.Code)
	var validateResult domain.ValidateResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &validateResult))
	require.True(t, validateResult.OK)

	inspectReq := httptest.NewRequest(http.MethodGet, "/v1/inspect", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, inspectReq)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "capabilities")

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

func TestValidateEndpointAcceptsRemoteInput(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	server := httpapi.New(rt)
	sub := base64.StdEncoding.EncodeToString([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sub))
	}))
	defer subServer.Close()

	validateReq := httptest.NewRequest(http.MethodPost, "/v1/validate", bytes.NewBufferString(`{
		"remote": {"url": "`+subServer.URL+`"}
	}`))
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, validateReq)

	require.Equal(t, http.StatusOK, w.Code)
	var result domain.ValidateResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.True(t, result.OK)
}
