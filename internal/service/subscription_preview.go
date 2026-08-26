package service

import (
	"context"
	"reflect"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shadowrocket"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const (
	subscriptionPreviewStatusAdded     = "added"
	subscriptionPreviewStatusModified  = "modified"
	subscriptionPreviewStatusRemoved   = "removed"
	subscriptionPreviewStatusUnchanged = "unchanged"
)

func (s *Service) PreviewSubscription(ctx context.Context, name string, args ...map[string]string) (*domain.SubscriptionPreviewResult, error) {
	return s.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{
		Name:    name,
		Request: domain.RequestInfo{Args: optionalRequestArgs(args...)},
	})
}

func (s *Service) PreviewSubscriptionRequest(ctx context.Context, req domain.SubscriptionPreviewRequest) (*domain.SubscriptionPreviewResult, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	if req.Refresh {
		ctx = withCacheReadBypass(ctx)
	}
	sub, err := s.metaStore.GetSubscription(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	ctx = withSubscriptionCacheScope(ctx, sub.Name)
	before, after, err := s.subscriptionPreviewNodes(ctx, sub, domain.FileRequest{
		Request: domain.RequestInfo{Args: req.Request.Args, Meta: sub.Meta},
		Meta:    sub.Meta,
	}, newSubscriptionResolveState())
	if err != nil {
		return nil, err
	}
	report := domain.Report{
		Dependencies: append([]domain.ResourceRef{}, after.Dependencies...),
		Warnings:     append([]domain.Warning{}, after.Warnings...),
	}
	for _, source := range after.Sources {
		report.SourceRefs = append(report.SourceRefs, source.SourceRefs...)
	}
	report = s.prepareReport("subscription_preview", report)

	nodes, counts := diffPreviewNodes(before.Nodes, after.Nodes)
	attachPreviewTargetNames(nodes, "shadowrocket", shadowrocket.PreviewNodeNames(after.Nodes))
	return &domain.SubscriptionPreviewResult{
		SubscriptionName: sub.Name,
		Type:             sub.Type,
		Format:           sub.Format,
		BeforeCount:      len(before.Nodes),
		AfterCount:       len(after.Nodes),
		StatusCounts:     counts,
		Nodes:            nodes,
		Report:           report,
	}, nil
}

func attachPreviewTargetNames(diffs []domain.SubscriptionPreviewNodeDiff, target string, names []string) {
	afterIndex := 0
	for index := range diffs {
		if diffs[index].After == nil {
			continue
		}
		if afterIndex >= len(names) {
			return
		}
		diffs[index].TargetNames = map[string]string{target: names[afterIndex]}
		afterIndex++
	}
}

func diffPreviewNodes(before, after []domain.NodeIR) ([]domain.SubscriptionPreviewNodeDiff, map[string]int) {
	counts := map[string]int{
		subscriptionPreviewStatusAdded:     0,
		subscriptionPreviewStatusModified:  0,
		subscriptionPreviewStatusRemoved:   0,
		subscriptionPreviewStatusUnchanged: 0,
	}
	ops := previewDiffOps(before, after)
	diffs := make([]domain.SubscriptionPreviewNodeDiff, 0, len(ops))
	for _, op := range ops {
		switch op.Kind {
		case previewDiffOpMatch:
			beforeNode := before[op.BeforeIndex]
			afterNode := after[op.AfterIndex]
			status := subscriptionPreviewStatusUnchanged
			if !previewNodeNativeEqual(beforeNode, afterNode) {
				status = subscriptionPreviewStatusModified
			}
			counts[status]++
			diffs = append(diffs, domain.SubscriptionPreviewNodeDiff{
				RuntimeID: domain.NodeRuntimeID(beforeNode),
				Status:    status,
				Before:    clonePreviewNode(beforeNode),
				After:     clonePreviewNode(afterNode),
			})
		case previewDiffOpRemove:
			counts[subscriptionPreviewStatusRemoved]++
			diffs = append(diffs, domain.SubscriptionPreviewNodeDiff{
				RuntimeID: domain.NodeRuntimeID(before[op.BeforeIndex]),
				Status:    subscriptionPreviewStatusRemoved,
				Before:    clonePreviewNode(before[op.BeforeIndex]),
			})
		case previewDiffOpAdd:
			counts[subscriptionPreviewStatusAdded]++
			diffs = append(diffs, domain.SubscriptionPreviewNodeDiff{
				RuntimeID: domain.NodeRuntimeID(after[op.AfterIndex]),
				Status:    subscriptionPreviewStatusAdded,
				After:     clonePreviewNode(after[op.AfterIndex]),
			})
		}
	}
	return diffs, counts
}

func previewDiffOps(before, after []domain.NodeIR) []previewDiffStep {
	beforeToAfter, afterToBefore := matchPreviewNodes(before, after)
	steps := make([]previewDiffStep, 0, max(len(before), len(after)))
	removedEmitted := make([]bool, len(before))
	emitRemovedBefore := func(limit int) {
		for beforeIndex := 0; beforeIndex < limit; beforeIndex++ {
			if beforeToAfter[beforeIndex] != -1 || removedEmitted[beforeIndex] {
				continue
			}
			steps = append(steps, previewDiffStep{Kind: previewDiffOpRemove, BeforeIndex: beforeIndex})
			removedEmitted[beforeIndex] = true
		}
	}
	for afterIndex := range after {
		beforeIndex := afterToBefore[afterIndex]
		if beforeIndex == -1 {
			steps = append(steps, previewDiffStep{Kind: previewDiffOpAdd, AfterIndex: afterIndex})
			continue
		}
		emitRemovedBefore(beforeIndex)
		steps = append(steps, previewDiffStep{Kind: previewDiffOpMatch, BeforeIndex: beforeIndex, AfterIndex: afterIndex})
	}
	emitRemovedBefore(len(before))
	return steps
}

func matchPreviewNodes(before, after []domain.NodeIR) ([]int, []int) {
	beforeToAfter := make([]int, len(before))
	for index := range beforeToAfter {
		beforeToAfter[index] = -1
	}
	afterToBefore := make([]int, len(after))
	for index := range afterToBefore {
		afterToBefore[index] = -1
	}
	beforeByRuntimeID := make(map[string]int, len(before))
	for beforeIndex, node := range before {
		runtimeID := domain.NodeRuntimeID(node)
		if runtimeID != "" {
			beforeByRuntimeID[runtimeID] = beforeIndex
		}
	}
	for afterIndex, node := range after {
		runtimeID := domain.NodeRuntimeID(node)
		beforeIndex, ok := beforeByRuntimeID[runtimeID]
		if !ok || runtimeID == "" || beforeToAfter[beforeIndex] != -1 {
			continue
		}
		beforeToAfter[beforeIndex] = afterIndex
		afterToBefore[afterIndex] = beforeIndex
	}
	return beforeToAfter, afterToBefore
}

func previewNodeNativeEqual(left, right domain.NodeIR) bool {
	return reflect.DeepEqual(previewNodeNativeFields(left), previewNodeNativeFields(right))
}

func previewNodeNativeFields(node domain.NodeIR) domain.NodeIR {
	domain.ClearNodeRuntimeID(&node)
	node.Tags = nil
	node.Meta = nil
	node.Lossy = false
	node.Warnings = nil
	node.SourceFormat = ""
	node.Hysteria = previewNativeHysteria(node.Hysteria)
	return node
}

func previewNativeHysteria(hysteria *domain.HysteriaOptions) *domain.HysteriaOptions {
	if hysteria == nil || reflect.DeepEqual(*hysteria, domain.HysteriaOptions{}) {
		return nil
	}
	return hysteria
}

type previewDiffOp byte

const (
	previewDiffOpRemove previewDiffOp = iota + 1
	previewDiffOpAdd
	previewDiffOpMatch
)

type previewDiffStep struct {
	Kind        previewDiffOp
	BeforeIndex int
	AfterIndex  int
}

func clonePreviewNode(node domain.NodeIR) *domain.NodeIR {
	cloned := node
	domain.ClearNodeRuntimeID(&cloned)
	return &cloned
}
