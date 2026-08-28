package node

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

type ProbeParams struct {
	Method          string `json:"method,omitempty" jsonschema:"Probe method" enum:"tcp_connect,udp_ntp,url_test"`
	Core            string `json:"core,omitempty" jsonschema:"Optional proxy core selector"`
	URL             string `json:"url,omitempty" jsonschema:"URL used for URL tests"`
	NTPServer       string `json:"ntp_server,omitempty" jsonschema:"NTP server used for UDP NTP probes"`
	ExpectedStatus  string `json:"expected_status,omitempty" jsonschema:"Expected HTTP status expression"`
	TimeoutMS       int    `json:"timeout_ms,omitempty" jsonschema:"Per-attempt timeout in milliseconds" minimum:"0"`
	Attempts        int    `json:"attempts,omitempty" jsonschema:"Number of probe attempts" minimum:"0"`
	Concurrency     int    `json:"concurrency,omitempty" jsonschema:"Maximum concurrent probes" minimum:"0"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds,omitempty" jsonschema:"Successful result cache lifetime in seconds" minimum:"0"`
	FailMode        string `json:"fail_mode,omitempty" jsonschema:"Treatment of nodes that fail probing" enum:"keep,drop,error" default:"keep"`
	Annotate        bool   `json:"annotate,omitempty" jsonschema:"Write probe result fields into node metadata"`
	Sort            string `json:"sort,omitempty" jsonschema:"Optional result ordering" enum:"duration"`
}

type probeProc struct {
	prober ProbeRunner
	params ProbeParams
}

func buildProbe(prober ProbeRunner) processor.NodeBuilder {
	return func(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
		var params ProbeParams
		if err := processor.UnmarshalParams(spec, &params); err != nil {
			return nil, err
		}
		params.FailMode = strings.ToLower(strings.TrimSpace(params.FailMode))
		if params.FailMode == "" {
			params.FailMode = "keep"
		}
		switch params.FailMode {
		case "keep", "drop", "error":
		default:
			return nil, &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   fmt.Sprintf("unsupported probe fail_mode %q", params.FailMode),
				Processor: spec.Type,
			}
		}
		params.Sort = strings.ToLower(strings.TrimSpace(params.Sort))
		switch params.Sort {
		case "", "duration":
		default:
			return nil, &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   fmt.Sprintf("unsupported probe sort %q", params.Sort),
				Processor: spec.Type,
			}
		}
		params.Method = string(probeMethod(params.Method))
		switch domain.ProbeMethod(params.Method) {
		case "", domain.ProbeTCPConnect, domain.ProbeUDPNTP, domain.ProbeURLTest:
		default:
			return nil, &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   fmt.Sprintf("unsupported probe method %q", params.Method),
				Processor: spec.Type,
			}
		}
		if prober == nil {
			return nil, &domain.AppError{
				Code:      domain.CodeProcessorConfigInvalid,
				Message:   "probe processor requires a probe runner",
				Processor: spec.Type,
			}
		}
		return &probeProc{prober: prober, params: params}, nil
	}
}

func (p *probeProc) Name() string { return "probe" }

func (p *probeProc) ApplyNodes(ctx context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	duplicateGroups, affectedNodes := duplicateNodeNameCounts(in.Nodes)
	if duplicateGroups > 0 {
		return domain.NodeProcessOutput{
			Nodes: append([]domain.NodeIR{}, in.Nodes...),
			Warnings: []domain.Warning{{
				Code:    "probe_skipped_duplicate_node_names",
				Message: fmt.Sprintf("probe skipped because duplicate node names were detected: groups=%d affected_nodes=%d", duplicateGroups, affectedNodes),
				Source:  "probe",
			}},
		}, nil
	}
	nodes := append([]domain.NodeIR{}, in.Nodes...)
	domain.AssignNodeRuntimeIDs(nodes)
	method := domain.ProbeMethod(p.params.Method)
	core := strings.TrimSpace(p.params.Core)
	url := strings.TrimSpace(p.params.URL)
	req := domain.ProbeRequest{
		Input: domain.NodeInput{
			Name:  in.Context.InputName,
			Type:  "inline_nodes",
			Nodes: nodes,
			Meta:  cloneStringMap(in.Context.Meta),
		},
		Method:          method,
		Core:            core,
		URL:             url,
		NTPServer:       p.params.NTPServer,
		ExpectedStatus:  p.params.ExpectedStatus,
		TimeoutMS:       p.params.TimeoutMS,
		Attempts:        p.params.Attempts,
		Concurrency:     p.params.Concurrency,
		CacheTTLSeconds: p.params.CacheTTLSeconds,
		Meta:            cloneStringMap(in.Request.Meta),
	}
	result, err := p.prober.Probe(ctx, req)
	if err != nil {
		return domain.NodeProcessOutput{}, err
	}
	if result == nil {
		return domain.NodeProcessOutput{}, &domain.AppError{
			Code:      domain.CodeNodeProcessorFailed,
			Message:   "probe runner returned nil result",
			Processor: "probe",
		}
	}
	if len(result.Results) != len(nodes) {
		return domain.NodeProcessOutput{}, &domain.AppError{
			Code:      domain.CodeNodeProcessorFailed,
			Message:   fmt.Sprintf("probe result count %d does not match node count %d", len(result.Results), len(nodes)),
			Processor: "probe",
		}
	}
	resultsByRuntimeID := make(map[string]domain.NodeProbeResult, len(result.Results))
	for _, probeResult := range result.Results {
		if probeResult.RuntimeID == "" {
			return domain.NodeProcessOutput{}, &domain.AppError{Code: domain.CodeNodeProcessorFailed, Message: "probe result is missing runtime_id", Processor: "probe"}
		}
		if _, exists := resultsByRuntimeID[probeResult.RuntimeID]; exists {
			return domain.NodeProcessOutput{}, &domain.AppError{Code: domain.CodeNodeProcessorFailed, Message: fmt.Sprintf("duplicate probe result runtime_id %q", probeResult.RuntimeID), Processor: "probe"}
		}
		resultsByRuntimeID[probeResult.RuntimeID] = probeResult
	}
	items := make([]probeItem, 0, len(nodes))
	for i, node := range nodes {
		probeResult, ok := resultsByRuntimeID[domain.NodeRuntimeID(node)]
		if !ok {
			return domain.NodeProcessOutput{}, &domain.AppError{Code: domain.CodeNodeProcessorFailed, Message: fmt.Sprintf("probe result missing for node %q", node.Name), Processor: "probe"}
		}
		if p.params.FailMode == "error" && !probeResult.Alive {
			return domain.NodeProcessOutput{}, &domain.AppError{
				Code:      domain.CodeNodeProcessorFailed,
				Message:   fmt.Sprintf("probe failed for node %q", node.Name),
				Processor: "probe",
				Cause:     fmt.Errorf("%s: %s", probeResult.ErrorCode, probeResult.Error),
			}
		}
		if p.params.FailMode == "drop" && !probeResult.Alive {
			continue
		}
		outNode := node
		if p.params.Annotate {
			outNode.Meta = annotateProbeMeta(outNode.Meta, probeResult)
		} else if outNode.Meta != nil {
			outNode.Meta = cloneStringMap(outNode.Meta)
		}
		items = append(items, probeItem{
			node:   outNode,
			result: probeResult,
			index:  i,
		})
	}
	if p.params.Sort == "duration" {
		sort.SliceStable(items, func(i, j int) bool {
			left := items[i].result
			right := items[j].result
			if left.Alive != right.Alive {
				return left.Alive
			}
			if left.Alive && right.Alive && left.DurationMS != right.DurationMS {
				return left.DurationMS < right.DurationMS
			}
			return items[i].index < items[j].index
		})
	}
	out := make([]domain.NodeIR, len(items))
	for i, item := range items {
		out[i] = item.node
	}
	return domain.NodeProcessOutput{
		Nodes:    out,
		Warnings: append([]domain.Warning{}, result.Report.Warnings...),
	}, nil
}

func duplicateNodeNameCounts(nodes []domain.NodeIR) (groups int, affected int) {
	counts := make(map[string]int, len(nodes))
	for _, node := range nodes {
		counts[node.Name]++
	}
	for _, count := range counts {
		if count < 2 {
			continue
		}
		groups++
		affected += count
	}
	return groups, affected
}

type probeItem struct {
	node   domain.NodeIR
	result domain.NodeProbeResult
	index  int
}

func annotateProbeMeta(meta map[string]string, result domain.NodeProbeResult) map[string]string {
	out := cloneStringMap(meta)
	if out == nil {
		out = map[string]string{}
	}
	for key := range out {
		if strings.HasPrefix(key, "probe.") {
			delete(out, key)
		}
	}
	out["probe.method"] = result.Method
	if result.Core != "" {
		out["probe.core"] = result.Core
	}
	if result.Target != "" {
		out["probe.target"] = result.Target
	}
	out["probe.alive"] = strconv.FormatBool(result.Alive)
	if result.DurationMS > 0 {
		out["probe.duration_ms"] = strconv.Itoa(result.DurationMS)
	}
	if !result.CheckedAt.IsZero() {
		out["probe.checked_at"] = result.CheckedAt.Format(time.RFC3339Nano)
	}
	if result.ErrorCode != "" {
		out["probe.error_code"] = result.ErrorCode
	}
	return out
}

func probeMethod(value string) domain.ProbeMethod {
	return domain.ProbeMethod(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "-", "_"))
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
