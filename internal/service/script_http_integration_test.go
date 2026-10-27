//go:build probe_singbox || probe_mihomo

package service

import (
	"context"
	"encoding/json/jsontext"
	"fmt"
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

// The requested host has no DNS record. Only this proxy can reach the fixture.
func nodeHTTPTestProxy(t *testing.T, handler http.Handler) (domain.NodeIR, *atomic.Int32) {
	t.Helper()
	origin := httptest.NewServer(handler)
	t.Cleanup(origin.Close)
	count := new(atomic.Int32)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "CONNECT" {
			http.Error(w, "CONNECT required", 405)
			return
		}
		upstream, err := net.Dial("tcp", strings.TrimPrefix(origin.URL, "http://"))
		if err != nil {
			http.Error(w, "upstream", 502)
			return
		}
		downstream, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			_ = upstream.Close()
			return
		}
		defer downstream.Close()
		defer upstream.Close()
		count.Add(1)
		_, _ = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = rw.Flush()
		done := make(chan struct{})
		go func() { _, _ = io.Copy(upstream, rw); _ = upstream.Close(); close(done) }()
		_, _ = io.Copy(downstream, upstream)
		_ = downstream.Close()
		<-done
	}))
	t.Cleanup(proxy.Close)
	host, port, err := net.SplitHostPort(strings.TrimPrefix(proxy.URL, "http://"))
	require.NoError(t, err)
	portNumber, err := strconv.ParseUint(port, 10, 16)
	require.NoError(t, err)
	return domain.NodeIR{Name: "same-name", Type: domain.NodeTypeHTTP, Server: host, Port: uint16(portNumber)}, count
}

func TestNodeHTTPThroughProxyOrderedSameNamesAndFailures(t *testing.T) {
	for _, core := range nodeHTTPCompiledCores {
		t.Run(core, func(t *testing.T) {
			t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
			one, countOne := nodeHTTPTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "forwarded", r.Header.Get("X-Test"))
				w.Header().Set("X-Result", "first")
				_, _ = io.WriteString(w, "first")
			}))
			two, countTwo := nodeHTTPTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "second") }))
			bad := one
			bad.Port = 1
			nodes := []domain.NodeIR{one, two, bad}
			results, err := New().RequestNodeHTTP(context.Background(), nodes, domain.NodeHTTPOptions{Core: core, URL: "http://exit.invalid/", Headers: map[string]string{"X-Test": "forwarded"}})
			require.NoError(t, err)
			require.Len(t, results, 3)
			require.Equal(t, "first", results[0].Body)
			require.Equal(t, "second", results[1].Body)
			require.Equal(t, []string{"first"}, results[0].Headers["X-Result"])
			require.Equal(t, 200, results[0].StatusCode)
			require.NotEmpty(t, results[2].Error)
			for i := range nodes {
				require.Equal(t, i, results[i].Index)
				require.Equal(t, "same-name", nodes[i].Name)
			}
			require.EqualValues(t, 1, countOne.Load())
			require.EqualValues(t, 1, countTwo.Load())
		})
	}
}

func TestNodeHTTPResponseLimitsHeadRedirectAndNoCache(t *testing.T) {
	for _, core := range nodeHTTPCompiledCores {
		t.Run(core, func(t *testing.T) {
			node, count := nodeHTTPTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/redirect" {
					w.Header().Set("Location", "http://must-not-contact.invalid/")
					w.WriteHeader(http.StatusFound)
					return
				}
				_, _ = io.WriteString(w, "12345")
			}))
			svc := New()
			for _, tc := range []struct {
				path, method, body string
				limit              int64
				status             int
				hasError           bool
			}{
				{"/", "GET", "", 4, 200, true},
				{"/", "GET", "12345", 5, 200, false},
				{"/", "HEAD", "", 1, 200, false},
				{"/redirect", "GET", "", 5, 302, false},
			} {
				result, err := svc.RequestNodeHTTP(context.Background(), []domain.NodeIR{node}, domain.NodeHTTPOptions{Core: core, URL: "http://exit.invalid" + tc.path, Method: tc.method, MaxResponseBytes: tc.limit})
				require.NoError(t, err)
				require.Equal(t, tc.status, result[0].StatusCode)
				require.Equal(t, tc.body, result[0].Body)
				require.Equal(t, tc.hasError, result[0].Error != "")
			}
			require.EqualValues(t, 4, count.Load())
		})
	}
}

func TestNodeHTTPCancellationAndPerRequestTimeout(t *testing.T) {
	for _, core := range nodeHTTPCompiledCores {
		t.Run(core, func(t *testing.T) {
			node, _ := nodeHTTPTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
			svc := New()
			result, err := svc.RequestNodeHTTP(context.Background(), []domain.NodeIR{node}, domain.NodeHTTPOptions{Core: core, URL: "http://exit.invalid/", TimeoutMS: 20})
			require.NoError(t, err)
			require.Contains(t, result[0].Error, "deadline exceeded")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			_, err = svc.RequestNodeHTTP(ctx, []domain.NodeIR{node, node, node}, domain.NodeHTTPOptions{Core: core, URL: "http://exit.invalid/", Concurrency: 1})
			require.ErrorIs(t, err, context.DeadlineExceeded)
		})
	}
}

func TestScriptHTTPExecutionTimeout(t *testing.T) {
	for _, core := range nodeHTTPCompiledCores {
		t.Run(core, func(t *testing.T) {
			node, _ := nodeHTTPTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
			svc := New()
			proc, err := svc.Registry().BuildNode(domain.ProcessorSpec{Type: "script", Stage: domain.StageNodes, Params: map[string]jsontext.Value{"timeout_ms": jsontext.Value(`30`), "content": jsontext.Value(strconv.Quote(fmt.Sprintf(`function main(input,api){ api.http.request(input.nodes,{url:"http://exit.invalid/",core:%q}); return input; }`, core)))}})
			require.NoError(t, err)
			_, err = proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{node}})
			require.True(t, domain.IsCode(err, domain.CodeScriptTimeout), "%v", err)
		})
	}
}

func TestNodeHTTPDoesNotFallBackToReachableOrigin(t *testing.T) {
	for _, core := range nodeHTTPCompiledCores {
		t.Run(core, func(t *testing.T) {
			var requests atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(http.StatusOK) }))
			defer origin.Close()
			result, err := New().RequestNodeHTTP(context.Background(), []domain.NodeIR{{Name: "unreachable", Type: domain.NodeTypeHTTP, Server: "127.0.0.1", Port: 1}}, domain.NodeHTTPOptions{Core: core, URL: origin.URL})
			require.NoError(t, err)
			require.NotEmpty(t, result[0].Error)
			require.Zero(t, requests.Load())
		})
	}
}

func TestNodeHTTPHonorsConcurrency(t *testing.T) {
	for _, core := range nodeHTTPCompiledCores {
		t.Run(core, func(t *testing.T) {
			var running, maximum atomic.Int32
			node, count := nodeHTTPTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				current := running.Add(1)
				defer running.Add(-1)
				for {
					old := maximum.Load()
					if old >= current || maximum.CompareAndSwap(old, current) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				_, _ = io.WriteString(w, "done")
			}))
			results, err := New().RequestNodeHTTP(context.Background(), []domain.NodeIR{node, node, node, node, node}, domain.NodeHTTPOptions{Core: core, URL: "http://exit.invalid/", Concurrency: 2})
			require.NoError(t, err)
			require.Len(t, results, 5)
			require.EqualValues(t, 5, count.Load())
			require.EqualValues(t, 2, maximum.Load())
		})
	}
}

func TestNodeHTTPUnsupportedNodeKeepsPerNodeResult(t *testing.T) {
	for _, core := range nodeHTTPCompiledCores {
		t.Run(core, func(t *testing.T) {
			node, _ := nodeHTTPTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") }))
			unsupported := domain.NodeIR{Name: "unsupported", Type: domain.NodeType("unsupported"), Server: "example.com", Port: 80}
			svc := New()
			for _, nodes := range [][]domain.NodeIR{{node, unsupported}, {unsupported}} {
				result, err := svc.RequestNodeHTTP(context.Background(), nodes, domain.NodeHTTPOptions{Core: core, URL: "http://exit.invalid/"})
				require.NoError(t, err)
				require.Len(t, result, len(nodes))
				require.NotEmpty(t, result[len(result)-1].Error)
				if len(nodes) > 1 {
					require.Equal(t, "ok", result[0].Body)
				}
			}
		})
	}
}

// Recording the selected renderer makes the core assertion meaningful even when
// both HTTP backends can reach the same proxy fixture in a dual-core build.
type nodeHTTPRecordingRenderer struct {
	Renderer
	calls *atomic.Int32
}

func (r nodeHTTPRecordingRenderer) Render(ctx context.Context, nodes []domain.NodeIR, options domain.RenderOptions) ([]byte, error) {
	r.calls.Add(1)
	return r.Renderer.Render(ctx, nodes, options)
}

func (r nodeHTTPRecordingRenderer) RenderWithReport(ctx context.Context, nodes []domain.NodeIR, options domain.RenderOptions) ([]byte, domain.RenderReport, error) {
	r.calls.Add(1)
	if renderer, ok := r.Renderer.(reportingRenderer); ok {
		return renderer.RenderWithReport(ctx, nodes, options)
	}
	body, err := r.Renderer.Render(ctx, nodes, options)
	return body, domain.RenderReport{}, err
}

func TestScriptHTTPSelectsRequestedCore(t *testing.T) {
	cores := append([]string{}, nodeHTTPCompiledCores...)
	for _, core := range nodeHTTPCompiledCores {
		if core == "sing-box" {
			cores = append(cores, "")
			break
		}
	}
	for _, core := range cores {
		name := core
		if name == "" {
			name = "default-sing-box"
		}
		t.Run(name, func(t *testing.T) {
			node, requests := nodeHTTPTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "GET", r.Method)
				require.Equal(t, "/script", r.URL.Path)
				_, _ = io.WriteString(w, "script-proxy-response")
			}))
			svc := New()
			singBoxCalls, mihomoCalls := new(atomic.Int32), new(atomic.Int32)
			svc.renderers["sing-box-outbounds"] = nodeHTTPRecordingRenderer{Renderer: svc.renderers["sing-box-outbounds"], calls: singBoxCalls}
			svc.renderers["mihomo-proxies"] = nodeHTTPRecordingRenderer{Renderer: svc.renderers["mihomo-proxies"], calls: mihomoCalls}
			options := `{url:"http://exit.invalid/script"}`
			if core != "" {
				options = fmt.Sprintf(`{url:"http://exit.invalid/script",core:%q}`, core)
			}
			source := fmt.Sprintf(`function main(input,api) {
    const result=api.http.request(input.nodes,%s);
    if(result[0].error) throw new Error(result[0].error);
    input.nodes[0].name=result[0].body;
    return input;
   }`, options)
			proc, err := svc.Registry().BuildNode(domain.ProcessorSpec{Type: "script", Stage: domain.StageNodes, Params: map[string]jsontext.Value{"content": jsontext.Value(strconv.Quote(source))}})
			require.NoError(t, err)
			result, err := proc.ApplyNodes(t.Context(), domain.NodeProcessInput{Nodes: []domain.NodeIR{node}})
			require.NoError(t, err)
			require.Len(t, result.Nodes, 1)
			require.Equal(t, "script-proxy-response", result.Nodes[0].Name)
			require.EqualValues(t, 1, requests.Load())
			if core == "mihomo" {
				require.EqualValues(t, 1, mihomoCalls.Load())
				require.Zero(t, singBoxCalls.Load())
			} else {
				require.EqualValues(t, 1, singBoxCalls.Load())
				require.Zero(t, mihomoCalls.Load())
			}
		})
	}
}
