package processor

import (
	"context"
	"errors"
	"sync"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type traceContextKey struct{}
type traceScopeContextKey struct{}
type traceStepContextKey struct{}

// TraceRecorder records diagnostic-only execution details. It is inert unless
// explicitly attached to a context by Service.Diagnose.
type TraceRecorder struct {
	mu     sync.Mutex
	stages []domain.DiagnoseStage
}

func NewTraceRecorder() *TraceRecorder { return &TraceRecorder{} }

func WithTrace(ctx context.Context, recorder *TraceRecorder) context.Context {
	if recorder == nil {
		return ctx
	}
	return context.WithValue(ctx, traceContextKey{}, recorder)
}

func WithTraceScope(ctx context.Context, scope string) context.Context {
	if scope == "" {
		return ctx
	}
	return context.WithValue(ctx, traceScopeContextKey{}, scope)
}

func traceFromContext(ctx context.Context) *TraceRecorder {
	recorder, _ := ctx.Value(traceContextKey{}).(*TraceRecorder)
	return recorder
}

func traceScope(ctx context.Context) string {
	scope, _ := ctx.Value(traceScopeContextKey{}).(string)
	if scope == "" {
		return "input"
	}
	return scope
}

func (r *TraceRecorder) begin(ctx context.Context, spec domain.ProcessorSpec, stage domain.Stage, inputCount int) (context.Context, int) {
	r.mu.Lock()
	id := len(r.stages)
	r.stages = append(r.stages, domain.DiagnoseStage{
		Index: id + 1, Scope: traceScope(ctx), Kind: "processor",
		Type: spec.Type, Name: spec.Name, Stage: stage, InputCount: inputCount,
	})
	r.mu.Unlock()
	return context.WithValue(ctx, traceStepContextKey{}, id), id
}

func (r *TraceRecorder) finish(id, outputCount int, warnings []domain.Warning, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id < 0 || id >= len(r.stages) {
		return
	}
	r.stages[id].OutputCount = outputCount
	r.stages[id].Warnings = append([]domain.Warning{}, warnings...)
	if err != nil {
		r.stages[id].Error = appError(err)
	}
}

func RecordProbe(ctx context.Context, result *domain.ProbeResult) {
	if result == nil {
		return
	}
	recorder := traceFromContext(ctx)
	id, ok := ctx.Value(traceStepContextKey{}).(int)
	if recorder == nil || !ok {
		return
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if id < 0 || id >= len(recorder.stages) {
		return
	}
	cloned := *result
	cloned.Results = append([]domain.NodeProbeResult{}, result.Results...)
	recorder.stages[id].Probes = append(recorder.stages[id].Probes, cloned)
}

func (r *TraceRecorder) Snapshot() []domain.DiagnoseStage {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.DiagnoseStage, len(r.stages))
	copy(out, r.stages)
	for i := range out {
		out[i].Warnings = append([]domain.Warning{}, r.stages[i].Warnings...)
		out[i].Probes = append([]domain.ProbeResult{}, r.stages[i].Probes...)
	}
	return out
}

func appError(err error) *domain.AppError {
	var existing *domain.AppError
	if errors.As(err, &existing) {
		cloned := *existing
		cloned.Cause = nil
		return &cloned
	}
	return &domain.AppError{Code: domain.CodeInvalidArgument, Message: err.Error()}
}
