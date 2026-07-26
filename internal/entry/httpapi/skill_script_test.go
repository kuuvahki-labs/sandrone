package httpapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func sandroneAPIScript(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../../../skills/sandrone/scripts/sandrone-api.sh")
	require.NoError(t, err)
	return path
}

func runSandroneAPIScript(
	t *testing.T,
	env []string,
	stdin io.Reader,
	args ...string,
) (string, string, error) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{sandroneAPIScript(t)}, args...)...)
	cmd.Env = sandroneAPITestEnv(env)
	cmd.Stdin = stdin
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func sandroneAPITestEnv(overrides []string) []string {
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key == "SANDRONE_URL" || key == "SANDRONE_TOKEN" {
			continue
		}
		env = append(env, entry)
	}
	return append(env, overrides...)
}

func TestSandroneAPIScriptSendsBearerAndStdinBody(t *testing.T) {
	var gotMethod, gotAuth, gotContentType, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	stdout, stderr, err := runSandroneAPIScript(
		t,
		[]string{
			"SANDRONE_URL=" + server.URL,
			"SANDRONE_TOKEN=secret-value",
		},
		strings.NewReader(`{"name":"demo"}`),
		"POST",
		"/v1/files",
		"-",
	)

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.JSONEq(t, `{"ok":true}`, stdout)
	require.Equal(t, "POST", gotMethod)
	require.Equal(t, "Bearer secret-value", gotAuth)
	require.Equal(t, "application/json", gotContentType)
	require.JSONEq(t, `{"name":"demo"}`, gotBody)
}

func TestSandroneAPIScriptGETWithoutTokenOrBody(t *testing.T) {
	t.Setenv("SANDRONE_TOKEN", "host-environment-token")

	var gotMethod, gotPath, gotAuth, gotContentType, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.RequestURI()
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte("ready"))
	}))
	defer server.Close()

	stdout, stderr, err := runSandroneAPIScript(
		t,
		[]string{"SANDRONE_URL=" + server.URL + "///"},
		nil,
		"GET",
		"/v1/status?verbose=true",
	)

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, "ready", stdout)
	require.Equal(t, "GET", gotMethod)
	require.Equal(t, "/v1/status?verbose=true", gotPath)
	require.Empty(t, gotAuth)
	require.Empty(t, gotContentType)
	require.Empty(t, gotBody)
}

func TestSandroneAPIScriptReadsBodyFromNamedFile(t *testing.T) {
	var gotContentType, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	bodyPath := filepath.Join(t.TempDir(), "body with spaces.json")
	require.NoError(t, os.WriteFile(bodyPath, []byte(`{"enabled":false}`), 0o600))

	stdout, stderr, err := runSandroneAPIScript(
		t,
		[]string{"SANDRONE_URL=" + server.URL},
		nil,
		"PUT",
		"/v1/settings",
		bodyPath,
	)

	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
	require.Equal(t, "application/json", gotContentType)
	require.JSONEq(t, `{"enabled":false}`, gotBody)
}

func TestSandroneAPIScriptRejectsInvalidInputs(t *testing.T) {
	t.Setenv("SANDRONE_URL", "http://host-environment.invalid")
	t.Setenv("SANDRONE_TOKEN", "host-environment-token")

	tests := []struct {
		name           string
		env            []string
		args           []string
		wantDiagnostic string
	}{
		{name: "missing URL", args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_URL must be an absolute HTTP(S) URL"},
		{name: "relative URL", env: []string{"SANDRONE_URL=localhost:8080"}, args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_URL must be an absolute HTTP(S) URL"},
		{name: "non HTTP URL", env: []string{"SANDRONE_URL=ftp://example.com"}, args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_URL must be an absolute HTTP(S) URL"},
		{name: "empty HTTP authority", env: []string{"SANDRONE_URL=http://"}, args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_URL must be an absolute HTTP(S) URL"},
		{name: "empty HTTPS authority", env: []string{"SANDRONE_URL=https://"}, args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_URL must be an absolute HTTP(S) URL"},
		{name: "slash before authority", env: []string{"SANDRONE_URL=http:///base"}, args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_URL must be an absolute HTTP(S) URL"},
		{name: "query before authority", env: []string{"SANDRONE_URL=http://?base"}, args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_URL must be an absolute HTTP(S) URL"},
		{name: "fragment before authority", env: []string{"SANDRONE_URL=http://#base"}, args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_URL must be an absolute HTTP(S) URL"},
		{name: "relative path", env: []string{"SANDRONE_URL=http://example.com"}, args: []string{"GET", "v1/status"}, wantDiagnostic: "PATH must begin with /"},
		{name: "unsupported method", env: []string{"SANDRONE_URL=http://example.com"}, args: []string{"PATCH", "/v1/status"}, wantDiagnostic: "unsupported HTTP method"},
		{name: "URL carriage return", env: []string{"SANDRONE_URL=http://example.com\rspoofed"}, args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_URL must not contain CR or LF"},
		{name: "URL line feed", env: []string{"SANDRONE_URL=http://example.com\nspoofed"}, args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_URL must not contain CR or LF"},
		{name: "token carriage return", env: []string{"SANDRONE_URL=http://example.com", "SANDRONE_TOKEN=secret\rspoofed"}, args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_TOKEN must not contain CR or LF"},
		{name: "token line feed", env: []string{"SANDRONE_URL=http://example.com", "SANDRONE_TOKEN=secret\nspoofed"}, args: []string{"GET", "/v1/status"}, wantDiagnostic: "SANDRONE_TOKEN must not contain CR or LF"},
		{name: "path carriage return", env: []string{"SANDRONE_URL=http://example.com"}, args: []string{"GET", "/v1/status\rspoofed"}, wantDiagnostic: "PATH must not contain CR or LF"},
		{name: "path line feed", env: []string{"SANDRONE_URL=http://example.com"}, args: []string{"GET", "/v1/status\nspoofed"}, wantDiagnostic: "PATH must not contain CR or LF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			curlCalled := filepath.Join(tempDir, "curl-called")
			require.NoError(t, os.WriteFile(
				filepath.Join(tempDir, "curl"),
				[]byte("#!/bin/sh\n: > \"$FAKE_CURL_CALLED\"\nexit 99\n"),
				0o755,
			))
			env := append(
				append([]string(nil), tt.env...),
				"FAKE_CURL_CALLED="+curlCalled,
				"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
			)

			stdout, stderr, err := runSandroneAPIScript(t, env, nil, tt.args...)
			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr)
			require.Equal(t, 64, exitErr.ExitCode())
			require.Empty(t, stdout)
			require.Contains(t, stderr, tt.wantDiagnostic)
			_, statErr := os.Stat(curlCalled)
			require.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
}

func TestSandroneAPIScriptPreservesHTTPErrorBody(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantCode string
	}{
		{
			name:     "structured unauthorized",
			status:   http.StatusUnauthorized,
			body:     `{"error":{"code":"unauthorized","message":"invalid token"}}`,
			wantCode: "401",
		},
		{
			name:     "plain internal error",
			status:   http.StatusInternalServerError,
			body:     "backend unavailable",
			wantCode: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			stdout, stderr, err := runSandroneAPIScript(
				t,
				[]string{"SANDRONE_URL=" + server.URL},
				nil,
				"GET",
				"/v1/status",
			)

			require.Error(t, err)
			require.Equal(t, tt.body, stdout)
			require.Contains(t, stderr, tt.wantCode)
		})
	}
}

func TestSandroneAPIScriptReportsConnectionFailure(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tempDir, "curl"),
		[]byte("#!/bin/sh\nexit 7\n"),
		0o755,
	))

	stdout, stderr, err := runSandroneAPIScript(
		t,
		[]string{
			"SANDRONE_URL=http://example.com",
			"PATH=" + tempDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		},
		nil,
		"GET",
		"/v1/status",
	)

	require.Error(t, err)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "transport failure (curl exit 7)")
}

func TestSandroneAPIScriptNeverPrintsToken(t *testing.T) {
	const token = "do-not-leak-this-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"unauthorized"}}`))
	}))
	defer server.Close()

	stdout, stderr, err := runSandroneAPIScript(
		t,
		[]string{
			"SANDRONE_URL=" + server.URL,
			"SANDRONE_TOKEN=" + token,
		},
		nil,
		"GET",
		"/v1/status",
	)

	require.Error(t, err)
	require.NotContains(t, stdout, token)
	require.NotContains(t, stderr, token)
}

func TestSandroneAPIScriptKeepsTokenOutOfCurlArgv(t *testing.T) {
	const token = "argv-secret-value"
	tempDir := t.TempDir()
	argvPath := filepath.Join(tempDir, "argv")
	fakeCurl := filepath.Join(tempDir, "curl")
	require.NoError(t, os.WriteFile(fakeCurl, []byte(`#!/bin/sh
set -eu
: > "$FAKE_CURL_ARGV"
output=
previous=
for argument do
  printf '%s\n' "$argument" >> "$FAKE_CURL_ARGV"
  if [ "$previous" = "--output" ]; then
    output=$argument
  fi
  previous=$argument
done
printf '%s' '{"ok":true}' > "$output"
printf '%s' '200'
`), 0o755))

	stdout, stderr, err := runSandroneAPIScript(
		t,
		[]string{
			"SANDRONE_URL=http://example.com",
			"SANDRONE_TOKEN=" + token,
			"FAKE_CURL_ARGV=" + argvPath,
			"PATH=" + tempDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		},
		nil,
		"GET",
		"/v1/status",
	)

	require.NoError(t, err)
	require.Empty(t, stderr)
	require.JSONEq(t, `{"ok":true}`, stdout)
	argv, err := os.ReadFile(argvPath)
	require.NoError(t, err)
	require.NotContains(t, string(argv), token)
}

func TestSandroneAPIScriptDoesNotRetryAmbiguousPOST(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		hijacker, ok := w.(http.Hijacker)
		require.True(t, ok)
		conn, _, err := hijacker.Hijack()
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	}))
	defer server.Close()

	stdout, stderr, err := runSandroneAPIScript(
		t,
		[]string{"SANDRONE_URL=" + server.URL},
		strings.NewReader(`{"mutation":true}`),
		"POST",
		"/v1/files",
		"-",
	)

	require.Error(t, err)
	require.Empty(t, stdout)
	require.NotEmpty(t, stderr)
	require.EqualValues(t, 1, requests.Load())
}
