package service

import (
	"context"
	"fmt"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

// subscriptionExecutionRequest is the complete semantic input for producing a
// saved subscription's canonical node set. Entrypoints adapt their protocol
// request into this type before executing the subscription pipeline.
type subscriptionExecutionRequest struct {
	Name    string
	Request domain.RequestInfo
	Refresh bool
}

// subscriptionExecutionResult keeps the normalized input and final processed
// node sets together so preview and other consumers cannot execute divergent
// copies of the subscription pipeline.
type subscriptionExecutionResult struct {
	Before *domain.NodeSet
	After  *domain.NodeSet

	snapshotCacheStatus string
}

func (r *subscriptionExecutionResult) clone() *subscriptionExecutionResult {
	if r == nil {
		return nil
	}
	out := &subscriptionExecutionResult{snapshotCacheStatus: r.snapshotCacheStatus}
	if r.Before != nil {
		out.Before = r.Before.Clone()
	}
	if r.After != nil {
		out.After = r.After.Clone()
	}
	return out
}

// subscriptionExecutionState is shared by declarative and script-induced
// subscription recursion. The stack is resource-based, while memo entries are
// keyed by the complete execution variant.
type subscriptionExecutionState struct {
	stack     map[string]bool
	memo      map[string]*subscriptionExecutionResult
	fileState *fileResolveState
}

func newSubscriptionExecutionState() *subscriptionExecutionState {
	return &subscriptionExecutionState{
		stack: map[string]bool{},
		memo:  map[string]*subscriptionExecutionResult{},
		fileState: &fileResolveState{
			stack: map[string]bool{},
			memo:  map[string]*domain.FileResult{},
		},
	}
}

type subscriptionExecutionContextKey struct{}

type subscriptionExecutionContext struct {
	request      subscriptionExecutionRequest
	state        *subscriptionExecutionState
	dependencies *[]domain.ResourceRef
}

func withSubscriptionExecutionContext(
	ctx context.Context,
	req subscriptionExecutionRequest,
	state *subscriptionExecutionState,
	dependencies *[]domain.ResourceRef,
) context.Context {
	value := &subscriptionExecutionContext{request: req, state: state, dependencies: dependencies}
	return context.WithValue(ctx, subscriptionExecutionContextKey{}, value)
}

func subscriptionExecutionContextFrom(ctx context.Context) (*subscriptionExecutionContext, bool) {
	value, ok := ctx.Value(subscriptionExecutionContextKey{}).(*subscriptionExecutionContext)
	return value, ok && value != nil && value.state != nil && value.dependencies != nil
}

func (c *subscriptionExecutionContext) addDependency(ref domain.ResourceRef) {
	if c == nil || c.dependencies == nil {
		return
	}
	*c.dependencies = appendResourceRef(*c.dependencies, ref)
}

func subscriptionExecutionMemoKey(sub domain.Subscription, req subscriptionExecutionRequest) (string, error) {
	return cacheIdentity(struct {
		SubscriptionName string             `json:"subscription_name"`
		InputName        string             `json:"input_name,omitempty"`
		Request          domain.RequestInfo `json:"request,omitempty"`
	}{
		SubscriptionName: sub.Name, InputName: req.Name, Request: req.Request,
	})
}

func (s *Service) executeSubscription(
	ctx context.Context,
	sub domain.Subscription,
	req subscriptionExecutionRequest,
	state *subscriptionExecutionState,
) (*subscriptionExecutionResult, error) {
	if state == nil {
		state = newSubscriptionExecutionState()
	}
	if req.Refresh {
		ctx = withCacheReadBypass(ctx)
	}
	normalized, err := normalizeSubscription(sub)
	if err != nil {
		return nil, err
	}
	sub = normalized
	scopeName := firstNonEmptyString(sub.Name, req.Name, "inline")
	ctx = processor.WithTraceScope(ctx, "subscription:"+scopeName)

	memoKey := ""
	if sub.Name != "" {
		if state.stack[sub.Name] {
			return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("subscription dependency cycle at %q", sub.Name))
		}
		memoKey, err = subscriptionExecutionMemoKey(sub, req)
		if err != nil {
			return nil, err
		}
		if cached, ok := state.memo[memoKey]; ok {
			return cached.clone(), nil
		}
		state.stack[sub.Name] = true
		defer delete(state.stack, sub.Name)
	}

	snapshotStatus := snapshotCacheStatusDisabled
	snapshotTTLSeconds := s.subscriptionSnapshotTTLSeconds(sub.SnapshotTTLSeconds)
	snapshotEntryID := ""
	_, cacheOwned := ownedCacheKey(ctx, cacheKeyPrefixSubscriptionSnapshot)
	if sub.Name != "" && snapshotTTLSeconds > 0 && s.cache != nil && cacheOwned {
		snapshotEntryID, err = s.subscriptionSnapshotCacheEntryID(sub, req)
		if err != nil {
			return nil, err
		}
		if cacheReadBypass(ctx) {
			snapshotStatus = snapshotCacheStatusBypass
		} else {
			snapshotStatus = snapshotCacheStatusMiss
			if cached := s.readSubscriptionSnapshotCache(ctx, snapshotEntryID, time.Duration(snapshotTTLSeconds)*time.Second); cached != nil {
				if memoKey != "" {
					state.memo[memoKey] = cached.clone()
				}
				return cached, nil
			}
		}
	}

	dynamicDependencies := []domain.ResourceRef{}
	ctx = withSubscriptionExecutionContext(ctx, req, state, &dynamicDependencies)
	base, err := s.subscriptionBaseNodes(ctx, sub, req, state)
	if err != nil {
		return nil, err
	}
	validatedBase, baseValidationWarnings, err := validateNodeBatch(base.Nodes, nodevalidation.StageNormalized, "")
	if err != nil {
		return nil, err
	}
	base.Nodes = validatedBase.Nodes
	base.Warnings = append(base.Warnings, baseValidationWarnings...)
	before := &domain.NodeSet{
		Nodes:        append([]domain.NodeIR{}, base.Nodes...),
		Dependencies: append([]domain.ResourceRef{}, base.Dependencies...),
		Sources:      append([]domain.SourceInfo{}, base.Sources...),
		Warnings:     append([]domain.Warning{}, base.Warnings...),
		Traffic:      domain.CloneSubscriptionTrafficItems(base.Traffic),
		Meta:         cloneStringMap(base.Meta),
	}
	processed, err := s.registry.RunNodes(ctx, sub.Processors, domain.NodeProcessInput{
		Target: "",
		Nodes:  append([]domain.NodeIR{}, before.Nodes...),
		Context: domain.NodeContext{
			InputName:    firstNonEmptyString(req.Name, sub.Name),
			Dependencies: append([]domain.ResourceRef{}, before.Dependencies...),
			Sources:      append([]domain.SourceInfo{}, before.Sources...),
			Meta:         cloneStringMap(before.Meta),
		},
		Request: req.Request,
	})
	if err != nil {
		return nil, err
	}
	validatedProcessed, processedValidationWarnings, err := validateNodeBatch(processed.Nodes, nodevalidation.StageProcessed, "")
	if err != nil {
		return nil, err
	}
	processed.Nodes = validatedProcessed.Nodes
	processed.Warnings = append(processed.Warnings, processedValidationWarnings...)
	afterDependencies := append([]domain.ResourceRef{}, before.Dependencies...)
	for _, dependency := range dynamicDependencies {
		afterDependencies = appendResourceRef(afterDependencies, dependency)
	}
	after := &domain.NodeSet{
		Nodes:        append([]domain.NodeIR{}, processed.Nodes...),
		Dependencies: afterDependencies,
		Sources:      append([]domain.SourceInfo{}, before.Sources...),
		Warnings:     append(append([]domain.Warning{}, before.Warnings...), processed.Warnings...),
		Traffic:      domain.CloneSubscriptionTrafficItems(before.Traffic),
		Meta:         cloneStringMap(before.Meta),
	}
	result := &subscriptionExecutionResult{
		Before: before, After: after,
		snapshotCacheStatus: snapshotStatus,
	}
	if memoKey != "" {
		state.memo[memoKey] = result.clone()
	}
	s.writeSubscriptionSnapshotCache(ctx, sub.Name, snapshotEntryID, snapshotTTLSeconds, result)
	return result, nil
}
