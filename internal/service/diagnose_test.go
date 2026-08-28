package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
)

func TestDiagnoseDetectsNodeFormats(t *testing.T) {
	uri := "ss://aes-128-gcm:secret@example.com:8388#node-a"
	tests := []struct {
		name    string
		content string
		format  string
	}{
		{name: "uri", content: uri, format: "uri"},
		{name: "uri list", content: uri + "\n" + "ss://aes-128-gcm:secret@example.net:8388#node-b", format: "uri-list"},
		{name: "base64", content: base64.StdEncoding.EncodeToString([]byte(uri)), format: "base64"},
		{name: "mihomo", content: "proxies:\n- {name: node-a, type: ss, server: example.com, port: 8388, cipher: aes-128-gcm, password: secret}\n", format: "mihomo"},
		{name: "sing-box", content: `{"outbounds":[{"tag":"node-a","type":"shadowsocks","server":"example.com","server_port":8388,"method":"aes-128-gcm","password":"secret"}]}`, format: "sing-box"},
		{name: "json nodes", content: `[{"name":"node-a","type":"ss","server":"example.com","port":8388,"cipher":"aes-128-gcm","password":"secret"}]`, format: "json-nodes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := New().Diagnose(context.Background(), domain.DiagnoseRequest{
				Kind: domain.DiagnoseInputAuto, Content: []byte(test.content),
			})
			require.NoError(t, err)
			require.Equal(t, domain.DiagnoseStatusOK, result.Status)
			require.Equal(t, domain.DiagnoseInputNodes, result.Input.Kind)
			require.Equal(t, test.format, result.Input.Format)
			require.NotEmpty(t, result.Nodes)
		})
	}
}

func TestDiagnoseDetectsResourcesAndStructuredFailures(t *testing.T) {
	tests := []struct {
		name    string
		content string
		kind    domain.DiagnoseInputKind
		status  domain.DiagnoseStatus
		code    domain.ErrorCode
	}{
		{
			name: "subscription", kind: domain.DiagnoseInputSubscription, status: domain.DiagnoseStatusOK,
			content: `{"name":"local","type":"local","format":"uri-list","content":"ss://aes-128-gcm:secret@example.com:8388#node-a"}`,
		},
		{
			name: "file", kind: domain.DiagnoseInputFile, status: domain.DiagnoseStatusOK,
			content: `{"name":"example.txt","kind":"static","source":{"type":"inline","content":"hello"}}`,
		},
		{name: "unknown", content: "not an input", status: domain.DiagnoseStatusFailed, code: domain.CodeInputKindUnrecognized},
		{name: "ambiguous nodes", content: "proxies: []\noutbounds: []\n", status: domain.DiagnoseStatusFailed, code: domain.CodeInputKindAmbiguous},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := New().Diagnose(context.Background(), domain.DiagnoseRequest{
				Kind: domain.DiagnoseInputAuto, Content: []byte(test.content),
			})
			require.NoError(t, err)
			require.Equal(t, test.status, result.Status)
			if test.kind != "" {
				require.Equal(t, test.kind, result.Input.Kind)
			}
			if test.code != "" {
				require.NotNil(t, result.Error)
				require.Equal(t, test.code, result.Error.Code)
			}
		})
	}
}

func TestDiagnoseAutoDoesNotTreatResourceJSONAsURI(t *testing.T) {
	tests := []struct {
		name    string
		content string
		kind    domain.DiagnoseInputKind
	}{
		{
			name:    "subscription containing URL",
			content: `{"name":"local","type":"local","format":"uri-list","content":"https://example.com:443"}`,
			kind:    domain.DiagnoseInputSubscription,
		},
		{
			name:    "file containing URL",
			content: `{"name":"example.txt","kind":"static","source":{"type":"inline","content":"https://example.com/value"}}`,
			kind:    domain.DiagnoseInputFile,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := New().Diagnose(context.Background(), domain.DiagnoseRequest{
				Kind: domain.DiagnoseInputAuto, Content: []byte(test.content),
			})
			require.NoError(t, err)
			require.Equal(t, domain.DiagnoseStatusOK, result.Status)
			require.Equal(t, test.kind, result.Input.Kind)
			require.Nil(t, result.Error)
		})
	}
}

func TestDiagnoseRejectsYAMLResourceDefinitions(t *testing.T) {
	for _, content := range []string{
		"name: local\ntype: local\nformat: uri-list\ncontent: ss://example\n",
		"name: client\nkind: static\nsource:\n  type: inline\n  content: body\n",
	} {
		result, err := New().Diagnose(context.Background(), domain.DiagnoseRequest{
			Kind: domain.DiagnoseInputAuto, Content: []byte(content),
		})
		require.NoError(t, err)
		require.Equal(t, domain.DiagnoseStatusFailed, result.Status)
		require.Equal(t, domain.CodeInputKindUnrecognized, result.Error.Code)
	}

	for _, kind := range []domain.DiagnoseInputKind{domain.DiagnoseInputSubscription, domain.DiagnoseInputFile} {
		result, err := New().Diagnose(context.Background(), domain.DiagnoseRequest{
			Kind: kind, Content: []byte("name: resource\n"),
		})
		require.NoError(t, err)
		require.Equal(t, domain.DiagnoseStatusFailed, result.Status)
		require.Equal(t, domain.CodeParseFailed, result.Error.Code)
	}
}

func TestDiagnoseDecodesJSONResourceRawFields(t *testing.T) {
	t.Run("subscription processor params", func(t *testing.T) {
		result, err := New().Diagnose(context.Background(), domain.DiagnoseRequest{
			Kind: domain.DiagnoseInputAuto,
			Content: []byte(`{
  "name":"local-sub",
  "type":"local",
  "format":"uri-list",
  "content":"ss://aes-128-gcm:secret@example.com:8388#node-a",
  "processors":[{"type":"rename","stage":"nodes","params":{"mode":"prefix","value":"json-"}}]
}`),
		})
		require.NoError(t, err)
		require.Equal(t, domain.DiagnoseStatusOK, result.Status)
		require.Equal(t, domain.DiagnoseInputSubscription, result.Input.Kind)
		require.Len(t, result.Nodes, 1)
		require.Equal(t, "json-node-a", result.Nodes[0].Name)
		require.Len(t, result.Stages, 2)
		require.Equal(t, "rename", result.Stages[1].Type)
	})

	t.Run("file processor params", func(t *testing.T) {
		result, err := New().Diagnose(context.Background(), domain.DiagnoseRequest{
			Kind: domain.DiagnoseInputAuto,
			Content: []byte(`{
  "name":"client.yaml",
  "kind":"static",
  "source":{"type":"inline","content":"value: before\n"},
  "processors":[{"type":"merge","stage":"file","params":{"mode":"yaml_override","content":"value: after\n"}}]
}`),
		})
		require.NoError(t, err)
		require.Equal(t, domain.DiagnoseStatusOK, result.Status)
		require.Equal(t, domain.DiagnoseInputFile, result.Input.Kind)
		require.NotNil(t, result.File)
		require.Contains(t, string(result.File.Content), "value: after")
		require.Len(t, result.Stages, 2)
		require.Equal(t, "merge", result.Stages[1].Type)
	})

	t.Run("typed file settings", func(t *testing.T) {
		spec, err := decodeFileSpecDefinition([]byte(`{"name":"client.yaml","kind":"mihomo","config":{"settings":{"group_preset":"basic"}}}`))
		require.NoError(t, err)
		require.NotNil(t, spec.Config)
		require.JSONEq(t, `{"group_preset":"basic"}`, string(spec.Config.Settings))
	})
}

func TestDiagnoseExplicitFormatDoesNotFallback(t *testing.T) {
	result, err := New().Diagnose(context.Background(), domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputAuto, Format: "json-nodes",
		Content: []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
	})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusFailed, result.Status)
	require.Equal(t, domain.DiagnoseInputNodes, result.Input.Kind)
	require.Equal(t, "json-nodes", result.Input.Format)
	require.Equal(t, domain.CodeParseFailed, result.Error.Code)
}

func TestDiagnosePartialKeepsValidNodesAndIssues(t *testing.T) {
	result, err := New().Diagnose(context.Background(), domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputNodes, Format: "json-nodes",
		Content: []byte(`[
  {"name":"valid","type":"ss","server":"example.com","port":8388,"cipher":"aes-128-gcm","password":"secret"},
  {"name":"invalid","type":"ss","server":"example.com","port":0,"cipher":"aes-128-gcm","password":"secret"}
]`),
	})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusPartial, result.Status)
	require.Len(t, result.Nodes, 1)
	require.NotEmpty(t, result.Issues)
	require.NotEmpty(t, result.Warnings)
	require.Equal(t, 1, result.Counts.Valid)
	require.Equal(t, 1, result.Counts.Invalid)
}

func TestDiagnoseRemoteDetectsJSONNodesWithoutResourceFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"remote","type":"ss","server":"example.com","port":8388,"cipher":"aes-128-gcm","password":"secret"}]`))
	}))
	defer server.Close()

	result, err := New().Diagnose(context.Background(), domain.DiagnoseRequest{
		Kind:   domain.DiagnoseInputNodes,
		Remote: &domain.RemoteInput{URL: server.URL},
	})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusOK, result.Status)
	require.Equal(t, "json-nodes", result.Input.Format)
	require.Len(t, result.Nodes, 1)
	require.Equal(t, "remote", result.Nodes[0].Name)
}

func TestDiagnoseProcessorTracePreservesOrderCountsAndFailure(t *testing.T) {
	svc := New()
	content := []byte("ss://aes-128-gcm:secret@example.com:8388#node-a\nss://aes-128-gcm:secret@example.net:8388#node-b")
	result, err := svc.Diagnose(context.Background(), domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputNodes, Format: "uri-list", Content: content,
		Processors: []domain.ProcessorSpec{
			{Type: "rename", Stage: domain.StageNodes, Params: diagnoseParams(t, map[string]any{"mode": "prefix", "value": "x-"})},
			{Type: "filter", Stage: domain.StageNodes, Params: diagnoseParams(t, map[string]any{"action": "keep", "field": "server", "match": "in", "values": []string{"example.com"}})},
		},
	})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusPartial, result.Status)
	require.Len(t, result.Stages, 3)
	require.Equal(t, "rename", result.Stages[1].Type)
	require.Equal(t, 2, result.Stages[1].InputCount)
	require.Equal(t, 2, result.Stages[1].OutputCount)
	require.Equal(t, "filter", result.Stages[2].Type)
	require.Equal(t, 2, result.Stages[2].InputCount)
	require.Equal(t, 1, result.Stages[2].OutputCount)
	require.Equal(t, "x-node-a", result.Nodes[0].Name)

	failed, err := svc.Diagnose(context.Background(), domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputNodes, Format: "uri-list", Content: content,
		Processors: []domain.ProcessorSpec{{Type: "missing", Stage: domain.StageNodes}},
	})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusFailed, failed.Status)
	require.Len(t, failed.Stages, 2)
	require.NotNil(t, failed.Stages[1].Error)
	require.Equal(t, domain.CodeProcessorUnknown, failed.Stages[1].Error.Code)
}

type diagnoseProbeEngine struct{}

func (diagnoseProbeEngine) Probe(_ context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, _ ...probe.Payload) (*domain.ProbeResult, error) {
	results := make([]domain.NodeProbeResult, len(nodes))
	for i, node := range nodes {
		results[i] = domain.NodeProbeResult{RuntimeID: domain.NodeRuntimeID(node), NodeName: node.Name, Method: string(req.Method), Alive: i != 0}
	}
	return &domain.ProbeResult{Results: results, Report: domain.Report{Probe: &domain.ProbeReport{FailureCount: 1}}}, nil
}

func TestDiagnoseTraceKeepsProbeResultsWithoutAnnotation(t *testing.T) {
	svc := New(WithProbeEngine(diagnoseProbeEngine{}))
	result, err := svc.Diagnose(context.Background(), domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputNodes, Format: "uri-list",
		Content: []byte("ss://aes-128-gcm:secret@example.com:8388#node-a\nss://aes-128-gcm:secret@example.net:8388#node-b"),
		Processors: []domain.ProcessorSpec{{
			Type: "probe", Stage: domain.StageNodes,
			Params: diagnoseParams(t, map[string]any{"method": "tcp_connect", "annotate": false, "fail_mode": "keep"}),
		}},
	})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusPartial, result.Status)
	require.Len(t, result.Stages, 2)
	require.Len(t, result.Stages[1].Probes, 1)
	require.Len(t, result.Stages[1].Probes[0].Results, 2)
	require.False(t, result.Stages[1].Probes[0].Results[0].Alive)
	for _, node := range result.Nodes {
		for key := range node.Meta {
			require.NotContains(t, key, "probe.")
		}
	}

	annotated, err := svc.Diagnose(context.Background(), domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputNodes, Format: "uri-list",
		Content: []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
		Processors: []domain.ProcessorSpec{{
			Type: "probe", Stage: domain.StageNodes,
			Params: diagnoseParams(t, map[string]any{"method": "tcp_connect", "annotate": true, "fail_mode": "keep"}),
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "false", annotated.Nodes[0].Meta["probe.alive"])
	require.Len(t, annotated.Stages[1].Probes, 1)

	for _, failMode := range []string{"drop", "error"} {
		t.Run(failMode, func(t *testing.T) {
			failed, err := svc.Diagnose(context.Background(), domain.DiagnoseRequest{
				Kind: domain.DiagnoseInputNodes, Format: "uri-list",
				Content: []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
				Processors: []domain.ProcessorSpec{{
					Type: "probe", Stage: domain.StageNodes,
					Params: diagnoseParams(t, map[string]any{"method": "tcp_connect", "fail_mode": failMode}),
				}},
			})
			require.NoError(t, err)
			require.Equal(t, domain.DiagnoseStatusFailed, failed.Status)
			require.Len(t, failed.Stages[1].Probes, 1)
			if failMode == "drop" {
				require.Equal(t, 0, failed.Stages[1].OutputCount)
			} else {
				require.NotNil(t, failed.Stages[1].Error)
			}
		})
	}
}

func TestDiagnoseStoredSubscriptionTracesNestedScopes(t *testing.T) {
	svc := New(WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(context.Background(), domain.Subscription{
		Name: "child", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type: "rename", Stage: domain.StageNodes,
			Params: diagnoseParams(t, map[string]any{"mode": "prefix", "value": "child-"}),
		}},
	}))
	require.NoError(t, svc.PutSubscription(context.Background(), domain.Subscription{
		Name: "parent", Type: domain.SubscriptionTypeCollection,
		Inputs: []domain.NodeInput{{Name: "child-input", Type: "subscription", Ref: domain.ResourceRef{Kind: "subscription", Name: "child"}, Required: true}},
		Processors: []domain.ProcessorSpec{{
			Type: "sort", Stage: domain.StageNodes,
			Params: diagnoseParams(t, map[string]any{"by": "+name"}),
		}},
	}))

	result, err := svc.Diagnose(context.Background(), domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputSubscription, SubscriptionName: "parent",
	})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusOK, result.Status)
	require.Len(t, result.Stages, 3)
	require.Equal(t, "subscription:child", result.Stages[1].Scope)
	require.Equal(t, "rename", result.Stages[1].Type)
	require.Equal(t, "subscription:parent", result.Stages[2].Scope)
	require.Equal(t, "sort", result.Stages[2].Type)
	require.Equal(t, "child-node-a", result.Nodes[0].Name)
	require.Contains(t, result.Dependencies, domain.ResourceRef{Kind: "subscription", Name: "child"})
}

func TestDiagnoseStoredSubscriptionBypassesRemoteCache(t *testing.T) {
	body := "ss://aes-128-gcm:secret@example.com:8388#before"
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	ctx := context.Background()
	snapshotTTL := 3600
	svc := New(WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name: "remote", Type: domain.SubscriptionTypeRemote, Format: "uri-list",
		Remote:             &domain.RemoteInput{URL: server.URL, CacheTTLSeconds: 3600},
		SnapshotTTLSeconds: &snapshotTTL,
	}))
	preview, err := svc.PreviewSubscription(ctx, "remote")
	require.NoError(t, err)
	require.Equal(t, "before", preview.Nodes[0].After.Name)

	body = "ss://aes-128-gcm:secret@example.com:8388#after"
	result, err := svc.Diagnose(ctx, domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputSubscription, SubscriptionName: "remote",
	})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusOK, result.Status)
	require.Len(t, result.Nodes, 1)
	require.Equal(t, "after", result.Nodes[0].Name)
	require.Equal(t, 2, calls)

	body = "ss://aes-128-gcm:secret@example.com:8388#newer"
	reused, err := svc.Diagnose(ctx, domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputSubscription, SubscriptionName: "remote",
		CacheMode: domain.DiagnoseCacheModeReuse,
	})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusOK, reused.Status)
	require.Equal(t, "after", reused.Nodes[0].Name)
	require.Equal(t, 2, calls)
}

func TestDiagnoseStoredFileTracesSubscriptionAndFileProcessors(t *testing.T) {
	svc := New(WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(context.Background(), domain.Subscription{
		Name: "provider", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type: "rename", Stage: domain.StageNodes,
			Params: diagnoseParams(t, map[string]any{"mode": "prefix", "value": "file-"}),
		}},
	}))
	require.NoError(t, svc.PutFile(context.Background(), domain.FileSpec{
		Name: "client.yaml", Kind: domain.FileKindMihomo,
		Config: &domain.FileConfig{Subscriptions: []string{"provider"}},
		Processors: []domain.ProcessorSpec{{
			Type: "merge", Stage: domain.StageFile,
			Params: diagnoseParams(t, map[string]any{"mode": "yaml_override", "content": "mode: global"}),
		}},
	}))

	result, err := svc.Diagnose(context.Background(), domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputFile,
		File: &domain.FileRequest{Name: "client.yaml"},
	})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusOK, result.Status)
	require.NotNil(t, result.File)
	require.Len(t, result.Stages, 3)
	require.Equal(t, "subscription:provider", result.Stages[1].Scope)
	require.Equal(t, "rename", result.Stages[1].Type)
	require.Equal(t, "file:client.yaml", result.Stages[2].Scope)
	require.Equal(t, "merge", result.Stages[2].Type)
	require.Contains(t, string(result.File.Content), "mode: global")
}

func TestDiagnoseTraceKeepsMultipleScriptProbeCalls(t *testing.T) {
	svc := New(WithProbeEngine(diagnoseProbeEngine{}))
	result, err := svc.Diagnose(context.Background(), domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputNodes, Format: "uri-list",
		Content: []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
		Processors: []domain.ProcessorSpec{{
			Type: "script", Stage: domain.StageNodes,
			Params: diagnoseParams(t, map[string]any{
				"source": map[string]any{
					"type": "inline",
					"content": `function main(input) {
  api.probe(input.nodes, {method: "tcp_connect"});
  api.probe(input.nodes, {method: "tcp_connect"});
  return input;
}`,
				},
			}),
		}},
	})
	require.NoError(t, err)
	require.Equal(t, domain.DiagnoseStatusPartial, result.Status)
	require.Len(t, result.Stages, 2)
	require.Equal(t, "script", result.Stages[1].Type)
	require.Len(t, result.Stages[1].Probes, 2)
}

func TestDiagnoseTransientTraceDoesNotUsePersistentProbeCache(t *testing.T) {
	svc := New(WithFS(afero.NewMemMapFs()), WithProbeEngine(diagnoseProbeEngine{}))
	req := domain.DiagnoseRequest{
		Kind: domain.DiagnoseInputNodes, Format: "uri-list",
		Content: []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
		Processors: []domain.ProcessorSpec{{
			Type: "probe", Stage: domain.StageNodes,
			Params: diagnoseParams(t, map[string]any{
				"method": "tcp_connect", "cache_ttl_seconds": 60,
			}),
		}},
	}
	first, err := svc.Diagnose(context.Background(), req)
	require.NoError(t, err)
	require.False(t, first.Stages[1].Probes[0].Results[0].CacheHit)

	second, err := svc.Diagnose(context.Background(), req)
	require.NoError(t, err)
	require.False(t, second.Stages[1].Probes[0].Results[0].CacheHit)
}

func diagnoseParams(t *testing.T, values map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		body, err := json.Marshal(value)
		require.NoError(t, err)
		out[key] = body
	}
	return out
}
