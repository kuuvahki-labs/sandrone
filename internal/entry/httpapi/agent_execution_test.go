package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/httpapi"
)

type agentRenderResult struct {
	ContentType string        `json:"content_type"`
	Body        string        `json:"body"`
	Report      domain.Report `json:"report"`
	Cached      bool          `json:"cached"`
}

func TestAgentConvertSupportsProcessorsAndStructuredReport(t *testing.T) {
	rt := testRuntime(t, app.Config{MCP: app.MCPConfig{MaxOutputBytes: 1}})
	body := `{
		"from_format":"uri-list",
		"to_format":"json-nodes",
		"content":"ss://aes-128-gcm:secret@example.com:8388#node-a",
		"parse_processors":[
			{"type":"rename","stage":"nodes","params":{"mode":"prefix","value":"http-"}}
		],
		"meta":{"caller":"skill"}
	}`
	expected, err := rt.Service.Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "uri-list",
		ToFormat:   "json-nodes",
		Content:    []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
		ParseProcessors: []domain.ProcessorSpec{{
			Type:  "rename",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"mode": "prefix", "value": "http-",
			}),
		}},
		Meta: map[string]string{"caller": "skill"},
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	httpapi.New(rt).Handler().ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost, "/v1/convert", strings.NewReader(body),
	))

	require.Equal(t, http.StatusOK, rec.Code)
	var result agentRenderResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Equal(t, expected.ContentType, result.ContentType)
	require.Equal(t, string(expected.Body), result.Body)
	require.Contains(t, result.Body, `"name": "http-node-a"`)
	require.Equal(t, expected.Report.Kind, result.Report.Kind)
	wireExpected := agentWireReport(t, expected.Report)
	require.Equal(t, agentComparableReport(wireExpected), agentComparableReport(result.Report))
}

func TestAgentConvertProcessorParamsAreJSONObject(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	rec := httptest.NewRecorder()
	httpapi.New(rt).Handler().ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/v1/convert",
		strings.NewReader(`{
			"from_format":"uri-list",
			"to_format":"json-nodes",
			"content":"ss://aes-128-gcm:secret@example.com:8388#node-a",
			"parse_processors":[
				{"type":"rename","stage":"nodes","params":{"mode":"prefix","value":"object-"}}
			]
		}`),
	))

	require.Equal(t, http.StatusOK, rec.Code)
	var result agentRenderResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Contains(t, result.Body, `"name": "object-node-a"`)
}

func TestAgentConvertRejectsInvalidInputSourceCount(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	var fetches atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#remote"))
	}))
	defer upstream.Close()

	for _, tc := range []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing",
			body:       `{"from_format":"uri-list","to_format":"json-nodes"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   string(domain.CodeInvalidArgument),
		},
		{
			name:       "explicit empty inline content",
			body:       `{"from_format":"uri-list","to_format":"json-nodes","content":""}`,
			wantStatus: http.StatusInternalServerError,
			wantCode:   string(domain.CodeParseFailed),
		},
		{
			name: "remote and nonempty inline content",
			body: fmt.Sprintf(`{
				"from_format":"uri-list",
				"to_format":"json-nodes",
				"content":"ss://aes-128-gcm:secret@example.com:8388#node-a",
				"remote":{"url":%q}
			}`, upstream.URL),
			wantStatus: http.StatusBadRequest,
			wantCode:   string(domain.CodeInvalidArgument),
		},
		{
			name: "remote and explicitly empty inline content",
			body: fmt.Sprintf(`{
				"from_format":"uri-list",
				"to_format":"json-nodes",
				"content":"",
				"remote":{"url":%q}
			}`, upstream.URL),
			wantStatus: http.StatusBadRequest,
			wantCode:   string(domain.CodeInvalidArgument),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := fetches.Load()
			rec := httptest.NewRecorder()
			httpapi.New(rt).Handler().ServeHTTP(rec, httptest.NewRequest(
				http.MethodPost, "/v1/convert", strings.NewReader(tc.body),
			))
			require.Equal(t, tc.wantStatus, rec.Code)
			require.Contains(t, rec.Body.String(), `"code": "`+tc.wantCode+`"`)
			require.Equal(t, before, fetches.Load())
		})
	}
}

func TestAgentConvertPreservesCompleteWarnings(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	content := "proxies:\n  - name: ss\n    type: ss\n    server: example.com\n    port: 8388\n    cipher: aes-128-gcm\n    password: secret\n    private-thing: value\n"
	expected, err := rt.Service.Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "mihomo",
		ToFormat:   "json-nodes",
		Content:    []byte(content),
	})
	require.NoError(t, err)
	require.NotEmpty(t, expected.Report.Warnings)
	requestBody, err := json.Marshal(map[string]any{
		"from_format": "mihomo",
		"to_format":   "json-nodes",
		"content":     content,
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	httpapi.New(rt).Handler().ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost, "/v1/convert", bytes.NewReader(requestBody),
	))
	require.Equal(t, http.StatusOK, rec.Code)
	var result agentRenderResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Equal(
		t,
		agentComparableReport(agentWireReport(t, expected.Report)),
		agentComparableReport(result.Report),
	)
	require.Equal(t, "value", result.Report.Warnings[0].NodeContext.Raw["private-thing"])
}

func TestAgentExecutionRoutesRequireBearer(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	handler := httpapi.New(rt).Handler()
	for _, path := range []string{
		"/v1/convert",
		"/v1/probe",
		"/v1/subscriptions/demo/render",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(
			http.MethodPost, path, strings.NewReader(`{}`),
		))
		require.Equal(t, http.StatusUnauthorized, rec.Code, path)
	}
}

func TestAgentSubscriptionRenderReturnsBodyAndReport(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "local", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input) {
  var prefix = input.args.source || "";
  input.nodes.forEach(function(node) { node.name = prefix + "-" + node.name; });
  return input;
}`),
			}),
		}},
	}))
	expected, err := rt.Service.RenderSubscription(
		ctx,
		"local",
		"json-nodes",
		domain.RequestInfo{Args: map[string]string{"source": "skill"}},
	)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	httpapi.New(rt).Handler().ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/v1/subscriptions/local/render?arg.source=query",
		strings.NewReader(`{"format":"json-nodes","args":{"source":"skill"}}`),
	))
	require.Equal(t, http.StatusOK, rec.Code)
	var result agentRenderResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Equal(t, expected.ContentType, result.ContentType)
	require.Equal(t, string(expected.Body), result.Body)
	require.Equal(t, expected.Report.Kind, result.Report.Kind)
	wireExpected := agentWireReport(t, expected.Report)
	require.Equal(t, agentComparableReport(wireExpected), agentComparableReport(result.Report))
	require.Contains(t, result.Body, "skill-node-a")
}

func TestAgentSubscriptionRenderExposesCacheStatusAndRefresh(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	ttl := 60
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "cached", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content:               "ss://aes-128-gcm:secret@example.com:8388#node-a",
		RenderCacheTTLSeconds: &ttl,
	}))
	handler := httpapi.New(rt).Handler()
	render := func(body string) agentRenderResult {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(
			http.MethodPost,
			"/v1/subscriptions/cached/render",
			strings.NewReader(body),
		))
		require.Equal(t, http.StatusOK, rec.Code)
		var result agentRenderResult
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		return result
	}

	require.False(t, render(`{"format":"uri-list"}`).Cached)
	require.True(t, render(`{"format":"uri-list"}`).Cached)
	require.False(t, render(`{"format":"uri-list","refresh":true}`).Cached)
	require.True(t, render(`{"format":"uri-list"}`).Cached)
}

func TestAgentSubscriptionRenderValidatesPathAndFormat(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	for _, tc := range []struct {
		name string
		path string
		body string
	}{
		{name: "missing name", path: "/v1/subscriptions/%20/render", body: `{"format":"json-nodes"}`},
		{name: "multi segment", path: "/v1/subscriptions/a/b/render", body: `{"format":"json-nodes"}`},
		{name: "percent encoded slash", path: "/v1/subscriptions/a%2Fb/render", body: `{"format":"json-nodes"}`},
		{name: "missing format", path: "/v1/subscriptions/local/render", body: `{}`},
		{name: "blank format", path: "/v1/subscriptions/local/render", body: `{"format":"  "}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			httpapi.New(rt).Handler().ServeHTTP(rec, httptest.NewRequest(
				http.MethodPost, tc.path, strings.NewReader(tc.body),
			))
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), `"code": "invalid_argument"`)
		})
	}
}

func TestAgentExecutionRoutesRejectGET(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	for _, tc := range []struct {
		path       string
		wantStatus int
	}{
		{path: "/v1/convert", wantStatus: http.StatusMethodNotAllowed},
		{path: "/v1/probe", wantStatus: http.StatusMethodNotAllowed},
		{path: "/v1/subscriptions/local/render", wantStatus: http.StatusBadRequest},
	} {
		rec := httptest.NewRecorder()
		httpapi.New(rt).Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		require.Equal(t, tc.wantStatus, rec.Code, tc.path)
		if tc.path == "/v1/subscriptions/local/render" {
			require.Contains(t, rec.Body.String(), "subscription action requires POST")
		}
	}
}

func TestAgentProbeReturnsLiveResultAndCompleteReport(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	httpListener := openAgentProbeListener(t)
	httpRequest := agentProbeRequest(httpListener)
	body, err := json.Marshal(httpRequest)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	httpapi.New(rt).Handler().ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost, "/v1/probe", bytes.NewReader(body),
	))
	require.Equal(t, http.StatusOK, rec.Code)
	var result domain.ProbeResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Len(t, result.Results, 1)
	require.True(t, result.Results[0].Alive)
	require.Equal(t, "probe", result.Report.Kind)
	require.NotNil(t, result.Report.Probe)

	directListener := openAgentProbeListener(t)
	expected, err := rt.Service.Probe(context.Background(), agentProbeRequest(directListener))
	require.NoError(t, err)
	require.Equal(t, expected.Results[0].Alive, result.Results[0].Alive)
	require.Equal(t, expected.Results[0].Method, result.Results[0].Method)
	require.Equal(t, expected.Results[0].Backend, result.Results[0].Backend)
	require.Equal(t, expected.Report.Kind, result.Report.Kind)
	require.Equal(t, expected.Report.Status, result.Report.Status)
	wireExpected := agentWireReport(t, expected.Report)
	require.Equal(t, agentComparableReport(wireExpected), agentComparableReport(result.Report))
}

func TestAgentProbePreservesStructuredServiceErrors(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	node := `{"type":"inline_nodes","nodes":[{"name":"local","server":"127.0.0.1","port":9}]}`
	for _, tc := range []struct {
		name   string
		body   string
		code   string
		status int
	}{
		{
			name:   "invalid method",
			body:   `{"input":` + node + `,"method":"invalid"}`,
			code:   string(domain.CodeInvalidArgument),
			status: http.StatusBadRequest,
		},
		{
			name:   "unavailable core backend",
			body:   `{"input":` + node + `,"method":"url_test","core":"missing"}`,
			code:   string(domain.CodeProbeCoreUnavailable),
			status: http.StatusInternalServerError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			httpapi.New(rt).Handler().ServeHTTP(rec, httptest.NewRequest(
				http.MethodPost, "/v1/probe", strings.NewReader(tc.body),
			))
			require.Equal(t, tc.status, rec.Code)
			require.Contains(t, rec.Body.String(), `"code": "`+tc.code+`"`)
		})
	}
}

func TestAgentExecutionDoesNotMutateResources(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "local", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))
	beforeSubscriptions, err := rt.Service.ListSubscriptions(ctx)
	require.NoError(t, err)
	beforeFiles, err := rt.Service.ListFiles(ctx)
	require.NoError(t, err)
	beforeShares, err := rt.Service.ListShares(ctx)
	require.NoError(t, err)

	listener := openAgentProbeListener(t)
	probeBody, err := json.Marshal(agentProbeRequest(listener))
	require.NoError(t, err)
	handler := httpapi.New(rt).Handler()
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/v1/convert", strings.NewReader(`{
			"from_format":"uri-list",
			"to_format":"json-nodes",
			"content":"ss://aes-128-gcm:secret@example.com:8388#node-a"
		}`)),
		httptest.NewRequest(http.MethodPost, "/v1/probe", bytes.NewReader(probeBody)),
		httptest.NewRequest(http.MethodPost, "/v1/subscriptions/local/render", strings.NewReader(`{"format":"json-nodes"}`)),
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, request)
		require.Equal(t, http.StatusOK, rec.Code, request.URL.Path)
	}

	afterSubscriptions, err := rt.Service.ListSubscriptions(ctx)
	require.NoError(t, err)
	afterFiles, err := rt.Service.ListFiles(ctx)
	require.NoError(t, err)
	afterShares, err := rt.Service.ListShares(ctx)
	require.NoError(t, err)
	require.Equal(t, beforeSubscriptions, afterSubscriptions)
	require.Equal(t, beforeFiles, afterFiles)
	require.Equal(t, beforeShares, afterShares)
}

func openAgentProbeListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	return listener
}

func agentProbeRequest(listener net.Listener) domain.ProbeRequest {
	port := listener.Addr().(*net.TCPAddr).Port
	uri := fmt.Sprintf("ss://aes-128-gcm:secret@127.0.0.1:%s#local", strconv.Itoa(port))
	return domain.ProbeRequest{
		Input: domain.NodeInput{
			Type:    "inline",
			Format:  "uri-list",
			Content: uri,
		},
		Method:      domain.ProbeTCPConnect,
		TimeoutMS:   1000,
		Attempts:    1,
		Concurrency: 1,
	}
}

func agentWireReport(t *testing.T, report domain.Report) domain.Report {
	t.Helper()
	body, err := json.Marshal(report)
	require.NoError(t, err)
	var result domain.Report
	require.NoError(t, json.Unmarshal(body, &result))
	return result
}

func agentComparableReport(report domain.Report) domain.Report {
	report.CreatedAt = time.Time{}
	return report
}
