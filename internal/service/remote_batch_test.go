package service

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func batchTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			http.Error(w, "unavailable", http.StatusNotFound)
		case "/invalid":
			_, _ = w.Write([]byte("not a subscription"))
		case "/invalid-node":
			_, _ = w.Write([]byte(`[{"name":"bad","type":"ss","server":"example.com","port":0,"cipher":"aes-128-gcm","password":"secret"}]`))
		case "/empty":
			_, _ = w.Write([]byte("proxies: []"))
		default:
			_, _ = fmt.Fprintf(w, "ss://aes-128-gcm:secret@example.com:8388#%s", r.URL.Path[1:])
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestRemoteBatchMixedSourcesPreserveOrderAndMergeBeforeProcessors(t *testing.T) {
	server := batchTestServer(t)
	batch := &domain.RemoteBatchInput{Sources: []domain.RemoteInput{
		{URL: server.URL + "/first?token=private#fragment"}, {URL: server.URL + "/missing"},
		{URL: server.URL + "/invalid"}, {URL: server.URL + "/last"},
	}}
	svc := New()
	result, err := svc.Parse(t.Context(), domain.ParseRequest{RemoteBatch: batch})
	require.NoError(t, err)
	require.Equal(t, "partial", result.Report.Status)
	require.Equal(t, []string{"first", "last"}, []string{result.Nodes[0].Name, result.Nodes[1].Name})
	require.Len(t, result.Report.SourceResults, 4)
	for i, source := range result.Report.SourceResults {
		require.Equal(t, i, source.Index)
	}
	require.Equal(t, "ok", result.Report.SourceResults[0].FetchStatus)
	require.Equal(t, "ok", result.Report.SourceResults[0].ParseStatus)
	require.NotEmpty(t, result.Report.SourceResults[0].Format)
	require.Equal(t, 1, result.Report.SourceResults[0].NodeCount)
	require.Equal(t, "failed", result.Report.SourceResults[1].FetchStatus)
	require.Equal(t, "skipped", result.Report.SourceResults[1].ParseStatus)
	require.Equal(t, "failed", result.Report.SourceResults[2].ParseStatus)
	require.NotNil(t, result.Report.SourceResults[1].Error)
	body, err := json.Marshal(result.Report)
	require.NoError(t, err)
	require.NotContains(t, string(body), "private")
	require.NotContains(t, string(body), "fragment")
	result, err = svc.Parse(t.Context(), domain.ParseRequest{RemoteBatch: batch, Processors: []domain.ProcessorSpec{{Type: "dedup", Stage: domain.StageNodes, Params: diagnoseParams(t, map[string]any{"strategy": "connection"})}}})
	require.NoError(t, err)
	require.Len(t, result.Nodes, 1)
	require.Equal(t, "first", result.Nodes[0].Name)
	require.Equal(t, 1, result.Report.SourceResults[3].NodeCount)
}

func TestRemoteBatchFailuresRetainReportsAndRejectConflictingInputs(t *testing.T) {
	server := batchTestServer(t)
	svc := New()
	batch := &domain.RemoteBatchInput{Sources: []domain.RemoteInput{{URL: server.URL + "/missing"}, {URL: server.URL + "/empty"}}}
	parsed, err := svc.Parse(t.Context(), domain.ParseRequest{RemoteBatch: batch})
	require.Error(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, "failed", parsed.Report.Status)
	require.Len(t, parsed.Report.SourceResults, 2)
	rendered, err := svc.Convert(t.Context(), domain.ConvertRequest{RemoteBatch: batch, ToFormat: "json-nodes"})
	require.Error(t, err)
	require.NotNil(t, rendered)
	require.Empty(t, rendered.Body)
	require.Len(t, rendered.Report.SourceResults, 2)
	for _, req := range []domain.ParseRequest{
		{RemoteBatch: batch, Content: []byte{}},
		{RemoteBatch: batch, Remote: &domain.RemoteInput{URL: server.URL}},
		{RemoteBatch: &domain.RemoteBatchInput{}},
		{RemoteBatch: &domain.RemoteBatchInput{Sources: batch.Sources, Concurrency: -1}},
		{RemoteBatch: &domain.RemoteBatchInput{Sources: batch.Sources, Concurrency: 33}},
	} {
		_, err = svc.Parse(t.Context(), req)
		require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "%v", err)
	}
	_, err = svc.ConvertPublic(t.Context(), domain.ConvertRequest{RemoteBatch: batch, ToFormat: "json-nodes"})
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
}

func TestRemoteBatchDiagnoseAndIntentionalEmptyOutput(t *testing.T) {
	server := batchTestServer(t)
	svc := New()
	batch := &domain.RemoteBatchInput{Sources: []domain.RemoteInput{{URL: server.URL + "/first"}}}
	drop := []domain.ProcessorSpec{{Type: "filter", Stage: domain.StageNodes, Params: diagnoseParams(t, map[string]any{"action": "drop", "field": "name", "match": "regex", "pattern": ".*"})}}
	rendered, err := svc.Convert(t.Context(), domain.ConvertRequest{RemoteBatch: batch, ToFormat: "json-nodes", ParseProcessors: drop})
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(rendered.Body))
	require.Equal(t, 1, rendered.Report.SourceResults[0].NodeCount)
	diagnostic, err := svc.Diagnose(t.Context(), domain.DiagnoseRequest{Kind: domain.DiagnoseInputURLList, RemoteBatch: batch, Processors: drop})
	require.NoError(t, err)
	require.NotEqual(t, domain.DiagnoseStatusFailed, diagnostic.Status)
	require.Empty(t, diagnostic.Nodes)
	require.Len(t, diagnostic.Report.SourceResults, 1)
	batch.Sources = append(batch.Sources, domain.RemoteInput{URL: server.URL + "/missing"})
	diagnostic, err = svc.Diagnose(t.Context(), domain.DiagnoseRequest{RemoteBatch: batch})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusPartial, diagnostic.Status)
	require.Equal(t, domain.DiagnoseInputURLList, diagnostic.Input.Kind)
	batch.Sources = batch.Sources[1:]
	diagnostic, err = svc.Diagnose(t.Context(), domain.DiagnoseRequest{RemoteBatch: batch})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusFailed, diagnostic.Status)
	require.Len(t, diagnostic.Report.SourceResults, 1)
}

func TestRemoteBatchConcurrencyAndCancellation(t *testing.T) {
	for _, limit := range []int{0, 2} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			want := limit
			if want == 0 {
				want = 4
			}
			started := make(chan struct{}, 8)
			release := make(chan struct{})
			var active, peak atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count := active.Add(1)
				defer active.Add(-1)
				for old := peak.Load(); count > old; old = peak.Load() {
					if peak.CompareAndSwap(old, count) {
						break
					}
				}
				started <- struct{}{}
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#node"))
			}))
			defer server.Close()
			batch := domain.RemoteBatchInput{Concurrency: limit}
			for range 7 {
				batch.Sources = append(batch.Sources, domain.RemoteInput{URL: server.URL})
			}
			done := make(chan error, 1)
			go func() { _, err := New().Parse(t.Context(), domain.ParseRequest{RemoteBatch: &batch}); done <- err }()
			for range want {
				select {
				case <-started:
				case <-time.After(5 * time.Second):
					close(release)
					t.Fatal("workers did not start")
				}
			}
			require.Equal(t, int32(want), peak.Load())
			close(release)
			require.NoError(t, <-done)
			require.LessOrEqual(t, peak.Load(), int32(want))
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err := New().Parse(ctx, domain.ParseRequest{RemoteBatch: &domain.RemoteBatchInput{Sources: []domain.RemoteInput{{URL: "http://example.invalid/sub"}}}})
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, result)
	require.Len(t, result.Report.SourceResults, 1)
}

func TestRemoteBatchValidationFailureRetainsSources(t *testing.T) {
	server := batchTestServer(t)
	result, err := New().Parse(t.Context(), domain.ParseRequest{Format: "json-nodes", RemoteBatch: &domain.RemoteBatchInput{Sources: []domain.RemoteInput{{URL: server.URL + "/invalid-node"}}}})
	require.True(t, domain.IsCode(err, domain.CodeNodeValidationFailed), "%v", err)
	require.NotNil(t, result)
	require.Equal(t, "failed", result.Report.Status)
	require.Len(t, result.Report.SourceResults, 1)
}

func TestRemoteBatchCancellationSkipsQueuedSources(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		cancel()
		<-r.Context().Done()
	}))
	defer upstream.Close()
	batch := &domain.RemoteBatchInput{Concurrency: 1}
	for range 20 {
		batch.Sources = append(batch.Sources, domain.RemoteInput{URL: upstream.URL})
	}
	result, err := New().Parse(ctx, domain.ParseRequest{RemoteBatch: batch})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, int32(1), calls.Load())
	require.Len(t, result.Report.SourceResults, 20)
	require.Equal(t, "failed", result.Report.SourceResults[0].FetchStatus)
	for _, source := range result.Report.SourceResults[1:] {
		require.Equal(t, "skipped", source.FetchStatus)
		require.Equal(t, domain.CodeRemoteFetchCanceled, source.Error.Code)
	}
}

func TestRemoteBatchFetchTimeoutIsDistinguishable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer upstream.Close()
	result, err := New().Parse(t.Context(), domain.ParseRequest{RemoteBatch: &domain.RemoteBatchInput{Sources: []domain.RemoteInput{{URL: upstream.URL, TimeoutMS: 10}}}})
	require.Error(t, err)
	require.Equal(t, domain.CodeRemoteFetchTimeout, result.Report.SourceResults[0].Error.Code)
}

func TestConvertInputBoundaryRejectsBeforeFetch(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#remote"))
	}))
	defer upstream.Close()
	remote := &domain.RemoteInput{URL: upstream.URL}
	batch := &domain.RemoteBatchInput{Sources: []domain.RemoteInput{*remote}}
	for _, req := range []domain.ConvertRequest{
		{Remote: remote},
		{RemoteBatch: batch},
		{RemoteBatch: batch, ToFormat: "unknown"},
		{ToFormat: "json-nodes"},
		{Remote: remote, Content: []byte{}, ToFormat: "json-nodes"},
		{RemoteBatch: batch, Content: []byte{}, ToFormat: "json-nodes"},
	} {
		_, err := New().Convert(t.Context(), req)
		require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "%v", err)
	}
	require.Zero(t, calls.Load())
}

func TestRemoteBatchAutoRejectsURLDirectoriesWithoutRecursiveFetch(t *testing.T) {
	var childRequests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := "http://" + r.Host + "/child"
		switch r.URL.Path {
		case "/directory":
			_, _ = fmt.Fprint(w, "# sources\n"+body+"\n")
		case "/encoded":
			_, _ = fmt.Fprint(w, base64.StdEncoding.EncodeToString([]byte(body)))
		case "/endpoint":
			_, _ = fmt.Fprint(w, "http://"+r.Host)
		default:
			childRequests.Add(1)
			_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#child"))
		}
	}))
	defer upstream.Close()
	for _, test := range []struct {
		path, format string
		rejected     bool
	}{
		{"/directory", "", true}, {"/encoded", "", true},
		{"/endpoint", "", false}, {"/directory", "uri-list", false},
	} {
		t.Run(test.path+test.format, func(t *testing.T) {
			result, err := New().Parse(t.Context(), domain.ParseRequest{Format: test.format, RemoteBatch: &domain.RemoteBatchInput{Sources: []domain.RemoteInput{{URL: upstream.URL + test.path}}}})
			if test.rejected {
				require.Error(t, err)
				require.Equal(t, "ok", result.Report.SourceResults[0].FetchStatus)
				require.Equal(t, "failed", result.Report.SourceResults[0].ParseStatus)
				require.Equal(t, domain.CodeParseFailed, result.Report.SourceResults[0].Error.Code)
			} else {
				require.NoError(t, err)
				require.Len(t, result.Nodes, 1)
				require.Equal(t, domain.NodeTypeHTTP, result.Nodes[0].Type)
			}
		})
	}
	require.Zero(t, childRequests.Load())
}
