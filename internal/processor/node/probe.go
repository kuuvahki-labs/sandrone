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
	Layer           string `json:"layer,omitempty"`
	Method          string `json:"method,omitempty"`
	Core            string `json:"core,omitempty"`
	URL             string `json:"url,omitempty"`
	NTPServer       string `json:"ntp_server,omitempty"`
	ExpectedStatus  string `json:"expected_status,omitempty"`
	TimeoutMS       int    `json:"timeout_ms,omitempty"`
	Attempts        int    `json:"attempts,omitempty"`
	Concurrency     int    `json:"concurrency,omitempty"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds,omitempty"`
	FailMode        string `json:"fail_mode,omitempty"`
	Annotate        bool   `json:"annotate,omitempty"`
	Sort            string `json:"sort,omitempty"`
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
	layer := domain.ProbeLayer(p.params.Layer)
	method := domain.ProbeMethod(p.params.Method)
	core := strings.TrimSpace(p.params.Core)
	url := strings.TrimSpace(p.params.URL)
	req := domain.ProbeRequest{
		Input: domain.NodeInput{
			Name:  in.Context.InputName,
			Type:  "inline_nodes",
			Nodes: append([]domain.NodeIR{}, in.Nodes...),
			Meta:  cloneStringMap(in.Context.Meta),
		},
		Method:          method,
		Layer:           layer,
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
	if len(result.Results) != len(in.Nodes) {
		return domain.NodeProcessOutput{}, &domain.AppError{
			Code:      domain.CodeNodeProcessorFailed,
			Message:   fmt.Sprintf("probe result count %d does not match node count %d", len(result.Results), len(in.Nodes)),
			Processor: "probe",
		}
	}
	items := make([]probeItem, 0, len(in.Nodes))
	for i, node := range in.Nodes {
		probeResult := domain.NodeProbeResult{}
		if i < len(result.Results) {
			probeResult = result.Results[i]
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
	out["probe.layer"] = result.Layer
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
