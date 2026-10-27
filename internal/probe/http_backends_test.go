//go:build probe_mihomo || probe_singbox

package probe

import (
	"context"
	"encoding/json/v2"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type nodeHTTPObservation struct {
	host    string
	method  string
	headers http.Header
}

// The fixture terminates CONNECT and answers the tunneled HTTP request locally.
// exit.invalid never resolves: a successful response proves the chosen proxy path.
func httpBackendProxy(t *testing.T, reply func(*http.Request) (int, string, http.Header)) (domain.NodeIR, chan nodeHTTPObservation, *atomic.Int32) {
	t.Helper()
	observations := make(chan nodeHTTPObservation, 32)
	count := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = rw.Flush()
		request, err := http.ReadRequest(rw.Reader)
		if err != nil {
			return
		}
		defer request.Body.Close()
		observations <- nodeHTTPObservation{host: request.Host, method: request.Method, headers: request.Header.Clone()}
		count.Add(1)
		status, body, headers := reply(request)
		response := &http.Response{StatusCode: status, ProtoMajor: 1, ProtoMinor: 1, Header: headers, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Close: true, Request: request}
		_ = response.Write(rw)
		_ = rw.Flush()
	}))
	t.Cleanup(server.Close)
	host, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	require.NoError(t, err)
	portNumber, err := strconv.ParseUint(port, 10, 16)
	require.NoError(t, err)
	return domain.NodeIR{Type: domain.NodeTypeHTTP, Server: host, Port: uint16(portNumber)}, observations, count
}

func nodeHTTPBackendPayload(t *testing.T, core string, nodes []domain.NodeIR) Payload {
	t.Helper()
	mappings := make([]map[string]any, len(nodes))
	for i, node := range nodes {
		if core == "mihomo" {
			mappings[i] = map[string]any{"type": "http", "name": node.Name, "server": node.Server, "port": node.Port}
		} else {
			mappings[i] = map[string]any{"type": "http", "tag": node.Name, "server": node.Server, "server_port": node.Port}
		}
	}
	key, format := "outbounds", "sing-box-outbounds"
	if core == "mihomo" {
		key, format = "proxies", "mihomo-proxies"
	}
	body, err := json.Marshal(map[string]any{key: mappings})
	require.NoError(t, err)
	return Payload{Core: core, Format: format, Body: body}
}

func forNodeHTTPCores(t *testing.T, run func(*testing.T, string)) {
	t.Helper()
	for _, core := range []string{"sing-box", "mihomo"} {
		t.Run(core, func(t *testing.T) {
			if !NodeHTTPAvailable(core) {
				t.Skip("core not built")
			}
			run(t, core)
		})
	}
}

func backendHTTPOptions(core string) domain.NodeHTTPOptions {
	return domain.NodeHTTPOptions{Core: core, URL: "http://exit.invalid/trace", Method: http.MethodGet, TimeoutMS: 1000, Concurrency: 4, MaxResponseBytes: 65536}
}

func TestNodeHTTPBackendProxyRequestsAndPartialFailure(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	forNodeHTTPCores(t, func(t *testing.T, core string) {
		one, observations, count := httpBackendProxy(t, func(*http.Request) (int, string, http.Header) { return 200, "first", http.Header{"X-Result": {"ok"}} })
		two, _, _ := httpBackendProxy(t, func(*http.Request) (int, string, http.Header) { return 201, "second", nil })
		one.Name = "one"
		two.Name = "two"
		bad := domain.NodeIR{Name: "bad", Type: domain.NodeTypeHTTP, Server: "127.0.0.1", Port: 1}
		nodes := []domain.NodeIR{one, bad, two}
		opts := backendHTTPOptions(core)
		opts.Headers = map[string]string{"X-Test": "forwarded"}
		results, err := RequestNodeHTTP(context.Background(), nodes, opts, nodeHTTPBackendPayload(t, core, nodes))
		require.NoError(t, err)
		require.Len(t, results, 3)
		require.Equal(t, "first", results[0].Body)
		require.Equal(t, 200, results[0].StatusCode)
		require.Equal(t, []string{"ok"}, results[0].Headers["X-Result"])
		require.NotEmpty(t, results[1].Error)
		require.Equal(t, "second", results[2].Body)
		require.Equal(t, 201, results[2].StatusCode)
		for i, result := range results {
			require.Equal(t, i, result.Index)
		}
		require.EqualValues(t, 1, count.Load())
		observation := <-observations
		require.Equal(t, "exit.invalid", observation.host)
		require.Equal(t, "GET", observation.method)
		require.Equal(t, "forwarded", observation.headers.Get("X-Test"))
	})
}

func TestNodeHTTPBackendHeadRedirectAndLimit(t *testing.T) {
	forNodeHTTPCores(t, func(t *testing.T, core string) {
		node, _, count := httpBackendProxy(t, func(r *http.Request) (int, string, http.Header) {
			if r.URL.Path == "/redirect" {
				return 302, "", http.Header{"Location": {"http://must-not-contact.invalid/"}}
			}
			return 200, "12345", nil
		})
		node.Name = "node"
		nodes := []domain.NodeIR{node}
		payload := nodeHTTPBackendPayload(t, core, nodes)
		for _, tc := range []struct {
			method, path, body string
			limit              int64
			status             int
			hasError           bool
		}{
			{"GET", "/trace", "", 4, 200, true},
			{"GET", "/trace", "12345", 5, 200, false},
			{"HEAD", "/trace", "", 1, 200, false},
			{"GET", "/redirect", "", 1, 302, false},
		} {
			opts := backendHTTPOptions(core)
			opts.Method = tc.method
			opts.URL = "http://exit.invalid" + tc.path
			opts.MaxResponseBytes = tc.limit
			results, err := RequestNodeHTTP(context.Background(), nodes, opts, payload)
			require.NoError(t, err)
			require.Equal(t, tc.status, results[0].StatusCode)
			require.Equal(t, tc.body, results[0].Body)
			require.Equal(t, tc.hasError, results[0].Error != "")
		}
		require.EqualValues(t, 4, count.Load())
	})
}

func TestNodeHTTPBackendCancellationAndTimeout(t *testing.T) {
	forNodeHTTPCores(t, func(t *testing.T, core string) {
		release := make(chan struct{})
		defer close(release)
		node, _, _ := httpBackendProxy(t, func(*http.Request) (int, string, http.Header) { <-release; return 200, "done", nil })
		node.Name = "node"
		nodes := []domain.NodeIR{node}
		payload := nodeHTTPBackendPayload(t, core, nodes)
		opts := backendHTTPOptions(core)
		opts.TimeoutMS = 20
		results, err := RequestNodeHTTP(context.Background(), nodes, opts, payload)
		require.NoError(t, err)
		require.Contains(t, results[0].Error, "deadline exceeded")
		opts.TimeoutMS = 5000
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		_, err = RequestNodeHTTP(ctx, nodes, opts, payload)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestNodeHTTPBackendNeverFallsBackDirect(t *testing.T) {
	forNodeHTTPCores(t, func(t *testing.T, core string) {
		var hits atomic.Int32
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
		defer target.Close()
		nodes := []domain.NodeIR{{Name: "bad", Type: domain.NodeTypeHTTP, Server: "127.0.0.1", Port: 1}}
		opts := backendHTTPOptions(core)
		opts.URL = target.URL
		result, err := RequestNodeHTTP(context.Background(), nodes, opts, nodeHTTPBackendPayload(t, core, nodes))
		require.NoError(t, err)
		require.NotEmpty(t, result[0].Error)
		require.Zero(t, hits.Load())
	})
}
