package node_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/processor/node"
)

func raw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func params(t *testing.T, m map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	for k, v := range m {
		out[k] = raw(t, v)
	}
	return out
}

func makeRegistry() *processor.Registry {
	r := processor.NewRegistry()
	node.Register(r)
	return r
}

func makeProbeRegistry(prober node.ProbeRunner) *processor.Registry {
	r := processor.NewRegistry()
	node.Register(r, prober)
	return r
}

type stubProbeRunner struct {
	requests []domain.ProbeRequest
	result   *domain.ProbeResult
	err      error
}

func (s *stubProbeRunner) Probe(_ context.Context, req domain.ProbeRequest) (*domain.ProbeResult, error) {
	s.requests = append(s.requests, req)
	if s.result != nil {
		for index := range s.result.Results {
			if index < len(req.Input.Nodes) && s.result.Results[index].RuntimeID == "" {
				s.result.Results[index].RuntimeID = domain.NodeRuntimeID(req.Input.Nodes[index])
			}
		}
	}
	return s.result, s.err
}

type unavailableProbeRunner struct {
	stubProbeRunner
}

func (*unavailableProbeRunner) ProbeAvailable() bool { return false }

func buildNode(t *testing.T, r *processor.Registry, typ string, p map[string]any) domain.NodeProcessor {
	t.Helper()
	spec := domain.ProcessorSpec{Type: typ}
	if p != nil {
		spec.Params = params(t, p)
	}
	proc, err := r.BuildNode(spec)
	require.NoError(t, err)
	return proc
}

func TestFilterKeepsByRegex(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "filter", map[string]any{"action": "keep", "field": "name", "match": "regex", "pattern": "^hk-"})
	nodes := []domain.NodeIR{
		{Name: "hk-1"},
		{Name: "us-1"},
		{Name: "hk-2"},
	}
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: nodes})
	require.NoError(t, err)
	require.Len(t, out.Nodes, 2)
	require.Equal(t, "hk-1", out.Nodes[0].Name)
	require.Equal(t, "hk-2", out.Nodes[1].Name)
}

func TestFilterDropsByRegex(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "filter", map[string]any{"action": "drop", "field": "server", "match": "regex", "pattern": "blocked.example"})
	nodes := []domain.NodeIR{
		{Name: "a", Server: "ok.example"},
		{Name: "b", Server: "blocked.example"},
	}
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: nodes})
	require.NoError(t, err)
	require.Len(t, out.Nodes, 1)
	require.Equal(t, "a", out.Nodes[0].Name)
}

func TestFilterKeepsByTypeList(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "filter", map[string]any{"action": "keep", "field": "type", "match": "in", "values": []string{"vmess", "vless"}})
	nodes := []domain.NodeIR{
		{Name: "a", Type: domain.NodeTypeVMess},
		{Name: "b", Type: domain.NodeTypeShadowsocks},
		{Name: "c", Type: domain.NodeTypeVLESS},
	}
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: nodes})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "c"}, []string{out.Nodes[0].Name, out.Nodes[1].Name})
}

func TestFilterDropsByTypeList(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "filter", map[string]any{"action": "drop", "field": "type", "match": "in", "values": []string{"http"}})
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "a", Type: domain.NodeTypeHTTP},
		{Name: "b", Type: domain.NodeTypeSOCKS},
	}})
	require.NoError(t, err)
	require.Len(t, out.Nodes, 1)
	require.Equal(t, "b", out.Nodes[0].Name)
}

func TestFilterRejectsInvalidConfig(t *testing.T) {
	r := makeRegistry()
	cases := []struct {
		name   string
		params map[string]any
	}{
		{name: "missing action", params: map[string]any{"field": "name", "match": "regex", "pattern": ".*"}},
		{name: "invalid action", params: map[string]any{"action": "remove", "field": "name", "match": "regex", "pattern": ".*"}},
		{name: "missing match", params: map[string]any{"action": "keep", "field": "name", "pattern": ".*"}},
		{name: "invalid match", params: map[string]any{"action": "keep", "field": "name", "match": "glob", "pattern": "*"}},
		{name: "missing regex pattern", params: map[string]any{"action": "keep", "field": "name", "match": "regex"}},
		{name: "invalid regex", params: map[string]any{"action": "keep", "field": "name", "match": "regex", "pattern": "["}},
		{name: "missing values", params: map[string]any{"action": "keep", "field": "type", "match": "in"}},
		{name: "unsupported field", params: map[string]any{"action": "keep", "field": "tag", "match": "regex", "pattern": ".*"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.BuildNode(domain.ProcessorSpec{Type: "filter", Params: params(t, tc.params)})
			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
		})
	}
}

func TestDedupDefaultsToName(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "dedup", nil)
	nodes := []domain.NodeIR{
		{Name: "a", Type: "ss", Server: "x", Port: 1, Password: "p"},
		{Name: "a", Type: "ss", Server: "y", Port: 2, Password: "other"},
		{Name: "c", Type: "ss", Server: "x", Port: 2, Password: "p"},
	}
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: nodes})
	require.NoError(t, err)
	require.Len(t, out.Nodes, 2)
	require.Equal(t, "a", out.Nodes[0].Name)
	require.Equal(t, "c", out.Nodes[1].Name)
}

func TestDedupConnection(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "dedup", map[string]any{"strategy": "connection"})
	nodes := []domain.NodeIR{
		{Name: "a", Type: "ss", Server: "x", Port: 1, Password: "p"},
		{Name: "b", Type: "ss", Server: "x", Port: 1, Password: "p"},
		{Name: "c", Type: "ss", Server: "x", Port: 2, Password: "p"},
	}
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: nodes})
	require.NoError(t, err)
	require.Len(t, out.Nodes, 2)
	require.Equal(t, "a", out.Nodes[0].Name)
	require.Equal(t, "c", out.Nodes[1].Name)
}

func TestDedupConnectionKeepsDifferentTLSAndIgnoresMetadata(t *testing.T) {
	proc := buildNode(t, makeRegistry(), "dedup", map[string]any{"strategy": "connection"})
	nodes := []domain.NodeIR{
		{Name: "one", Type: domain.NodeTypeVLESS, Server: "x", Port: 443, UUID: "id", TLS: &domain.TLSOptions{Enabled: true, ServerName: "one.example"}},
		{Name: "renamed", Type: domain.NodeTypeVLESS, Server: "x", Port: 443, UUID: "id", TLS: &domain.TLSOptions{Enabled: true, ServerName: "one.example"}, Meta: map[string]string{"probe.alive": "true"}},
		{Name: "other-tls", Type: domain.NodeTypeVLESS, Server: "x", Port: 443, UUID: "id", TLS: &domain.TLSOptions{Enabled: true, ServerName: "two.example"}},
	}
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: nodes})
	require.NoError(t, err)
	require.Equal(t, []string{"one", "other-tls"}, []string{out.Nodes[0].Name, out.Nodes[1].Name})
}

func TestDedupAddsRandomSuffixToDuplicateNames(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "dedup", map[string]any{"strategy": "random_suffix"})
	nodes := []domain.NodeIR{
		{Name: "same", Server: "one.example"},
		{Name: "same", Server: "two.example"},
		{Name: "same", Server: "three.example"},
		{Name: "other", Server: "four.example"},
	}
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: nodes})
	require.NoError(t, err)
	require.Len(t, out.Nodes, len(nodes))
	require.Equal(t, "same", out.Nodes[0].Name)
	require.Regexp(t, `^same-\d{4}$`, out.Nodes[1].Name)
	require.Regexp(t, `^same-\d{4}$`, out.Nodes[2].Name)
	require.NotEqual(t, out.Nodes[1].Name, out.Nodes[2].Name)
	require.Equal(t, "other", out.Nodes[3].Name)
	require.Equal(t, []domain.NodeIR{
		{Name: "same", Server: "one.example"},
		{Name: "same", Server: "two.example"},
		{Name: "same", Server: "three.example"},
		{Name: "other", Server: "four.example"},
	}, nodes)
}

func TestDedupInvalidStrategy(t *testing.T) {
	r := makeRegistry()
	_, err := r.BuildNode(domain.ProcessorSpec{
		Type:   "dedup",
		Params: params(t, map[string]any{"strategy": "unknown"}),
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}

func TestDedupFieldsRequiresList(t *testing.T) {
	r := makeRegistry()
	_, err := r.BuildNode(domain.ProcessorSpec{
		Type:   "dedup",
		Params: params(t, map[string]any{"strategy": "fields"}),
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}

func TestDedupByFields(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "dedup", map[string]any{"strategy": "fields", "fields": []string{"server"}})
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "a", Type: domain.NodeTypeShadowsocks, Server: "same", Port: 1},
		{Name: "b", Type: domain.NodeTypeVMess, Server: "same", Port: 2},
		{Name: "c", Type: domain.NodeTypeHTTP, Server: "other", Port: 3},
	}})
	require.NoError(t, err)
	require.Len(t, out.Nodes, 2)
}

func TestDedupByWideFields(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "dedup", map[string]any{
		"strategy": "fields",
		"fields":   []string{"name", "type", "port", "uuid", "password", "username", "cipher", "unknown"},
	})
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{
			Name:     "a",
			Type:     domain.NodeTypeShadowsocks,
			Port:     8388,
			UUID:     "id",
			Password: "secret",
			Username: "user",
			Cipher:   "aes-128-gcm",
			Server:   "one.example",
		},
		{
			Name:     "a",
			Type:     domain.NodeTypeShadowsocks,
			Port:     8388,
			UUID:     "id",
			Password: "secret",
			Username: "user",
			Cipher:   "aes-128-gcm",
			Server:   "two.example",
		},
		{
			Name:     "a",
			Type:     domain.NodeTypeShadowsocks,
			Port:     8389,
			UUID:     "id",
			Password: "secret",
			Username: "user",
			Cipher:   "aes-128-gcm",
			Server:   "three.example",
		},
	}})
	require.NoError(t, err)
	require.Len(t, out.Nodes, 2)
	require.Equal(t, "one.example", out.Nodes[0].Server)
	require.Equal(t, "three.example", out.Nodes[1].Server)
}

func TestProbeProcessorAnnotatesAndSorts(t *testing.T) {
	checkedAt := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	runner := &stubProbeRunner{result: &domain.ProbeResult{
		Results: []domain.NodeProbeResult{
			{NodeName: "slow", Method: "tcp_connect", Target: "slow.example:443", Alive: true, DurationMS: 90, CheckedAt: checkedAt},
			{NodeName: "dead", Method: "tcp_connect", Target: "dead.example:443", Alive: false, CheckedAt: checkedAt, ErrorCode: "probe_tcp_failed"},
			{NodeName: "fast", Method: "tcp_connect", Target: "fast.example:443", Alive: true, DurationMS: 10, CheckedAt: checkedAt},
		},
		Report: domain.Report{Warnings: []domain.Warning{{Code: "probe_tcp_failed"}}},
	}}
	r := makeProbeRegistry(runner)
	proc := buildNode(t, r, "probe", map[string]any{
		"method":      "tcp_connect",
		"timeout_ms":  123,
		"concurrency": 2,
		"annotate":    true,
		"sort":        "duration",
	})

	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{
		Context: domain.NodeContext{InputName: "nodes"},
		Nodes: []domain.NodeIR{
			{Name: "slow", Server: "slow.example", Port: 443, Meta: map[string]string{"keep": "yes"}},
			{Name: "dead", Server: "dead.example", Port: 443},
			{Name: "fast", Server: "fast.example", Port: 443},
		},
	})

	require.NoError(t, err)
	require.Len(t, runner.requests, 1)
	require.Equal(t, domain.ProbeMethod("tcp_connect"), runner.requests[0].Method)
	require.Equal(t, 123, runner.requests[0].TimeoutMS)
	require.Equal(t, 2, runner.requests[0].Concurrency)
	require.Len(t, out.Nodes, 3)
	require.Equal(t, []string{"fast", "slow", "dead"}, []string{out.Nodes[0].Name, out.Nodes[1].Name, out.Nodes[2].Name})
	require.Equal(t, "true", out.Nodes[0].Meta["probe.alive"])
	require.Equal(t, "10", out.Nodes[0].Meta["probe.duration_ms"])
	require.Equal(t, "yes", out.Nodes[1].Meta["keep"])
	require.Equal(t, "false", out.Nodes[2].Meta["probe.alive"])
	require.Equal(t, "probe_tcp_failed", out.Nodes[2].Meta["probe.error_code"])
	require.Equal(t, "probe_tcp_failed", out.Warnings[0].Code)
}

func TestProbeProcessorSkipsDuplicateNodeNamesWithOneWarning(t *testing.T) {
	runner := &stubProbeRunner{}
	proc := buildNode(t, makeProbeRegistry(runner), "probe", map[string]any{
		"annotate":  true,
		"fail_mode": "error",
		"sort":      "duration",
	})
	nodes := []domain.NodeIR{
		{Name: "same", Server: "one.example", Port: 443, Meta: map[string]string{"probe.alive": "stale"}},
		{Name: "same", Server: "two.example", Port: 443},
		{Name: "same", Server: "three.example", Port: 443},
		{Name: "other", Server: "four.example", Port: 443},
		{Name: "other", Server: "five.example", Port: 443},
		{Name: "unique", Server: "six.example", Port: 443},
	}

	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: nodes})

	require.NoError(t, err)
	require.Empty(t, runner.requests)
	require.Equal(t, nodes, out.Nodes)
	require.Len(t, out.Warnings, 1)
	require.Equal(t, domain.Warning{
		Code:    "probe_skipped_duplicate_node_names",
		Message: "probe skipped because duplicate node names were detected: groups=2 affected_nodes=5",
		Source:  "probe",
	}, out.Warnings[0])
}

func TestProbeProcessorSkipsUnavailableBackendAndContinuesChain(t *testing.T) {
	for _, failMode := range []string{"keep", "drop", "error"} {
		t.Run(failMode, func(t *testing.T) {
			runner := &unavailableProbeRunner{}
			r := makeProbeRegistry(runner)
			specs := []domain.ProcessorSpec{
				{Type: "rename", Params: params(t, map[string]any{"mode": "prefix", "value": "before-"})},
				{Type: "probe", Params: params(t, map[string]any{"annotate": true, "fail_mode": failMode, "sort": "duration"})},
				{Type: "rename", Params: params(t, map[string]any{"mode": "suffix", "value": "-after"})},
			}

			out, err := r.RunNodes(t.Context(), specs, domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "node"}}})

			require.NoError(t, err)
			require.Empty(t, runner.requests)
			require.Equal(t, "before-node-after", out.Nodes[0].Name)
			require.Equal(t, []domain.Warning{{
				Code:    "probe_skipped_backend_unavailable",
				Message: "probe processor skipped because no probe backend is available",
				Source:  "probe",
			}}, out.Warnings)
		})
	}
}

func TestProbeProcessorRejectsUnsupportedMethod(t *testing.T) {
	r := makeProbeRegistry(&stubProbeRunner{})
	_, err := r.BuildNode(domain.ProcessorSpec{
		Type:   "probe",
		Params: params(t, map[string]any{"method": "future"}),
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}

func TestProbeProcessorDropsInvalidTargetAndRetainsWarnings(t *testing.T) {
	runner := &stubProbeRunner{result: &domain.ProbeResult{
		Results: []domain.NodeProbeResult{
			{NodeName: "invalid-hysteria", Method: "url_test", Alive: false, ErrorCode: "probe_invalid_target"},
			{NodeName: "valid-http", Method: "url_test", Alive: true},
		},
		Report: domain.Report{Warnings: []domain.Warning{
			{Code: "parse_unknown_field"},
			{Code: "render_node_skipped"},
		}},
	}}
	proc := buildNode(t, makeProbeRegistry(runner), "probe", map[string]any{"fail_mode": "drop"})

	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "invalid-hysteria", Server: "invalid.example", Port: 443},
		{Name: "valid-http", Server: "valid.example", Port: 443},
	}})

	require.NoError(t, err)
	require.Len(t, out.Nodes, 1)
	require.Equal(t, "valid-http", out.Nodes[0].Name)
	require.Equal(t, []domain.Warning{
		{Code: "parse_unknown_field"},
		{Code: "render_node_skipped"},
	}, out.Warnings)
}

func TestProbeProcessorLeavesRuntimeDefaultsToRunner(t *testing.T) {
	runner := &stubProbeRunner{result: &domain.ProbeResult{
		Results: []domain.NodeProbeResult{{NodeName: "proxy", Method: "url_test", Alive: true}},
	}}
	proc := buildNode(t, makeProbeRegistry(runner), "probe", map[string]any{
		"method": "url_test",
	})

	_, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{
		Name:   "proxy",
		Server: "proxy.example",
		Port:   443,
	}}})

	require.NoError(t, err)
	require.Len(t, runner.requests, 1)
	require.Empty(t, runner.requests[0].Core)
	require.Empty(t, runner.requests[0].URL)
}

func TestProbeProcessorErrorOnFailed(t *testing.T) {
	runner := &stubProbeRunner{result: &domain.ProbeResult{
		Results: []domain.NodeProbeResult{
			{NodeName: "dead", Method: "tcp_connect", Alive: false, ErrorCode: "probe_tcp_failed", Error: "connect failed"},
		},
	}}
	proc := buildNode(t, makeProbeRegistry(runner), "probe", map[string]any{"fail_mode": "error"})

	_, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "dead", Server: "dead.example", Port: 443},
	}})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeNodeProcessorFailed))
}

func TestProbeProcessorErrorsWhenResultCountMismatches(t *testing.T) {
	runner := &stubProbeRunner{result: &domain.ProbeResult{
		Results: []domain.NodeProbeResult{
			{NodeName: "only-one", Method: "tcp_connect", Alive: true},
		},
	}}
	proc := buildNode(t, makeProbeRegistry(runner), "probe", map[string]any{})

	_, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "a", Server: "a.example", Port: 443},
		{Name: "b", Server: "b.example", Port: 443},
	}})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeNodeProcessorFailed))
	require.Contains(t, err.Error(), "probe result count")
}

func TestDedupByName(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "dedup", map[string]any{"strategy": "name"})
	nodes := []domain.NodeIR{
		{Name: "a"},
		{Name: "a"},
		{Name: "b"},
	}
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: nodes})
	require.NoError(t, err)
	require.Len(t, out.Nodes, 2)
}

func TestRenameCleansAndReplacesWithFlatConfig(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "rename", map[string]any{
		"strip":       []string{"[HK]", "测试"},
		"trim":        true,
		"mode":        "replace",
		"pattern":     "^0+",
		"replacement": "",
	})
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "[HK]测试 001"}}})
	require.NoError(t, err)
	require.Equal(t, "1", out.Nodes[0].Name)
}

func TestRenameAddsPrefixWithFlatConfig(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "rename", map[string]any{"mode": "prefix", "value": "[HK]"})
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "hk-a"}}})
	require.NoError(t, err)
	require.Equal(t, "[HK]hk-a", out.Nodes[0].Name)
}

func TestRenameTemplateUsesCurrentName(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "rename", map[string]any{
		"trim":  true,
		"mode":  "template",
		"value": "{type}|{server}|{name}|{source_format}",
	})
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{
		Name:         " hk-1 ",
		Type:         domain.NodeTypeVMess,
		Server:       "example.com",
		SourceFormat: "uri-list",
	}}})
	require.NoError(t, err)
	require.Equal(t, "vmess|example.com|hk-1|uri-list", out.Nodes[0].Name)
}

func TestRenameRejectsInvalidConfig(t *testing.T) {
	r := makeRegistry()
	cases := []struct {
		name   string
		params map[string]any
	}{
		{name: "missing action", params: map[string]any{}},
		{name: "strip without fragments", params: map[string]any{"strip": []string{}}},
		{name: "unknown mode", params: map[string]any{"mode": "unknown"}},
		{name: "replace without pattern", params: map[string]any{"mode": "replace", "replacement": "x"}},
		{name: "replace invalid regex", params: map[string]any{"mode": "replace", "pattern": "[", "replacement": "x"}},
		{name: "prefix without value", params: map[string]any{"mode": "prefix"}},
		{name: "suffix without value", params: map[string]any{"mode": "suffix"}},
		{name: "template without value", params: map[string]any{"mode": "template"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.BuildNode(domain.ProcessorSpec{Type: "rename", Params: params(t, tc.params)})
			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
		})
	}
}

func TestSortNameAsc(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "sort", map[string]any{"by": "+name"})
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "b"}, {Name: "a"}, {Name: "c"},
	}})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, []string{out.Nodes[0].Name, out.Nodes[1].Name, out.Nodes[2].Name})
}

func TestSortDefaultAndRejectsInvalidKeys(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "sort", nil)
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "b"}, {Name: "a"}}})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, []string{out.Nodes[0].Name, out.Nodes[1].Name})

	_, err = r.BuildNode(domain.ProcessorSpec{Type: "sort", Params: params(t, map[string]any{"by": ","})})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))

	_, err = r.BuildNode(domain.ProcessorSpec{Type: "sort", Params: params(t, map[string]any{"by": "+"})})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}

func TestSortTypeThenName(t *testing.T) {
	r := makeRegistry()
	proc := buildNode(t, r, "sort", map[string]any{"by": "type,-name"})
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "a", Type: "ss"},
		{Name: "b", Type: "ss"},
		{Name: "c", Type: "vmess"},
	}})
	require.NoError(t, err)
	require.Equal(t, "b", out.Nodes[0].Name)
	require.Equal(t, "a", out.Nodes[1].Name)
	require.Equal(t, "c", out.Nodes[2].Name)
}

func TestQuickSettingsDefaultPreservesValues(t *testing.T) {
	r := makeRegistry()
	udp := true
	proc := buildNode(t, r, "quick_settings", map[string]any{
		"udp":            "default",
		"tfo":            "default",
		"allow_insecure": "default",
		"vmess_aead":     "default",
	})
	nodes := []domain.NodeIR{{
		Name:    "vmess",
		Type:    domain.NodeTypeVMess,
		AlterID: 7,
		Dialer:  &domain.DialerOptions{TFO: true, UDPRelay: &udp},
		TLS:     &domain.TLSOptions{Enabled: true, InsecureSkipVerify: true},
	}}

	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: nodes})

	require.NoError(t, err)
	require.Equal(t, nodes, out.Nodes)
	require.Empty(t, out.Warnings)
}

func TestQuickSettingsUDPAndTFO(t *testing.T) {
	r := makeRegistry()
	enable := buildNode(t, r, "quick_settings", map[string]any{
		"udp": "enabled",
		"tfo": "enabled",
	})
	enabled, err := enable.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{
		Name: "ss",
		Type: domain.NodeTypeShadowsocks,
	}}})
	require.NoError(t, err)
	require.NotNil(t, enabled.Nodes[0].Dialer)
	require.NotNil(t, enabled.Nodes[0].Dialer.UDPRelay)
	require.True(t, *enabled.Nodes[0].Dialer.UDPRelay)
	require.True(t, enabled.Nodes[0].Dialer.TFO)

	existingUDP := true
	disable := buildNode(t, r, "quick_settings", map[string]any{
		"udp": "disabled",
		"tfo": "disabled",
	})
	disabled, err := disable.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{
		Name:   "ss",
		Type:   domain.NodeTypeShadowsocks,
		Dialer: &domain.DialerOptions{TFO: true, UDPRelay: &existingUDP},
	}}})
	require.NoError(t, err)
	require.NotNil(t, disabled.Nodes[0].Dialer.UDPRelay)
	require.False(t, *disabled.Nodes[0].Dialer.UDPRelay)
	require.False(t, disabled.Nodes[0].Dialer.TFO)
	require.True(t, existingUDP, "processor must not mutate the input dialer pointer")
}

func TestQuickSettingsAllowInsecure(t *testing.T) {
	r := makeRegistry()
	enable := buildNode(t, r, "quick_settings", map[string]any{"allow_insecure": "enabled"})
	enabled, err := enable.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{
		Name: "vless",
		Type: domain.NodeTypeVLESS,
	}}})
	require.NoError(t, err)
	require.NotNil(t, enabled.Nodes[0].TLS)
	require.True(t, enabled.Nodes[0].TLS.Enabled)
	require.True(t, enabled.Nodes[0].TLS.InsecureSkipVerify)

	disable := buildNode(t, r, "quick_settings", map[string]any{"allow_insecure": "disabled"})
	disabled, err := disable.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{
		Name: "vless",
		Type: domain.NodeTypeVLESS,
		TLS:  &domain.TLSOptions{Enabled: true, InsecureSkipVerify: true, ServerName: "sni.example"},
	}}})
	require.NoError(t, err)
	require.True(t, disabled.Nodes[0].TLS.Enabled)
	require.False(t, disabled.Nodes[0].TLS.InsecureSkipVerify)
	require.Equal(t, "sni.example", disabled.Nodes[0].TLS.ServerName)
}

func TestQuickSettingsVMessAEAD(t *testing.T) {
	r := makeRegistry()
	enable := buildNode(t, r, "quick_settings", map[string]any{"vmess_aead": "enabled"})
	enabled, err := enable.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "vmess", Type: domain.NodeTypeVMess, AlterID: 8},
		{Name: "ss", Type: domain.NodeTypeShadowsocks, AlterID: 8},
	}})
	require.NoError(t, err)
	require.Zero(t, enabled.Nodes[0].AlterID)
	require.Equal(t, 8, enabled.Nodes[1].AlterID)

	disable := buildNode(t, r, "quick_settings", map[string]any{"vmess_aead": "disabled"})
	disabled, err := disable.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "legacy", Type: domain.NodeTypeVMess, AlterID: 4},
		{Name: "aead", Type: domain.NodeTypeVMess},
		{Name: "ss", Type: domain.NodeTypeShadowsocks},
	}})
	require.NoError(t, err)
	require.Equal(t, 4, disabled.Nodes[0].AlterID)
	require.Zero(t, disabled.Nodes[1].AlterID)
	require.Len(t, disabled.Warnings, 1)
	require.Equal(t, "quick_settings_vmess_aead_legacy_unavailable", disabled.Warnings[0].Code)
	require.Equal(t, "aead", disabled.Warnings[0].Node)
}

func TestQuickSettingsSnellReuseFollowsProtocolVersions(t *testing.T) {
	t.Parallel()

	r := processor.NewRegistry()
	node.Register(r, nil)
	enable := buildNode(t, r, "quick_settings", map[string]any{"reuse": "enabled"})
	output, err := enable.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "v1", Type: domain.NodeTypeSnell, Snell: &domain.SnellOptions{Version: 1}},
		{Name: "v2", Type: domain.NodeTypeSnell, Snell: &domain.SnellOptions{Version: 2}},
		{Name: "v4", Type: domain.NodeTypeSnell, Snell: &domain.SnellOptions{Version: 4}},
	}})
	require.NoError(t, err)
	require.Nil(t, output.Nodes[0].Snell.Reuse)
	require.NotNil(t, output.Nodes[1].Snell.Reuse)
	require.True(t, *output.Nodes[1].Snell.Reuse)
	require.NotNil(t, output.Nodes[2].Snell.Reuse)
	require.True(t, *output.Nodes[2].Snell.Reuse)
	require.Len(t, output.Warnings, 1)
	require.Equal(t, "quick_settings_snell_reuse_unavailable", output.Warnings[0].Code)

	disable := buildNode(t, r, "quick_settings", map[string]any{"reuse": "disabled"})
	output, err = disable.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{
		{Name: "v2", Type: domain.NodeTypeSnell, Snell: &domain.SnellOptions{Version: 2}},
		{Name: "v5", Type: domain.NodeTypeSnell, Snell: &domain.SnellOptions{Version: 5}},
	}})
	require.NoError(t, err)
	require.NotNil(t, output.Nodes[0].Snell.Reuse)
	require.True(t, *output.Nodes[0].Snell.Reuse)
	require.NotNil(t, output.Nodes[1].Snell.Reuse)
	require.False(t, *output.Nodes[1].Snell.Reuse)
	require.Len(t, output.Warnings, 1)
}

func TestQuickSettingsRejectsInvalidValue(t *testing.T) {
	r := makeRegistry()
	_, err := r.BuildNode(domain.ProcessorSpec{Type: "quick_settings", Params: params(t, map[string]any{"udp": "yes"})})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid))
}
