package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) resolveNodeInput(ctx context.Context, input domain.NodeInput, req domain.FileRequest) (*domain.NodeSet, error) {
	return s.resolveNodeInputWithSubscriptionState(ctx, input, req, newSubscriptionResolveState())
}

func (s *Service) resolveNodeInputWithSubscriptionState(ctx context.Context, input domain.NodeInput, req domain.FileRequest, subscriptionState *subscriptionResolveState) (*domain.NodeSet, error) {
	switch strings.ToLower(input.Type) {
	case "inline_nodes":
		nodes, warnings := normalizeInlineNodes(input.Nodes)
		return &domain.NodeSet{
			Nodes:    nodes,
			Warnings: warnings,
			Meta:     cloneStringMap(input.Meta),
		}, nil
	case "inline":
		if len(input.Nodes) > 0 {
			nodes, warnings := normalizeInlineNodes(input.Nodes)
			return &domain.NodeSet{
				Nodes:    nodes,
				Warnings: warnings,
				Meta:     cloneStringMap(input.Meta),
			}, nil
		}
		if input.Format == "" {
			return nil, domain.NewError(domain.CodeInvalidArgument, "inline node input format is required")
		}
		return s.parseNodeInputContent(ctx, input, []byte(input.Content))
	case "local":
		if s.store == nil {
			return nil, storeUnavailable()
		}
		key := input.Path
		if key == "" {
			key = input.Ref.Name
		}
		if key == "" {
			return nil, missingNodeInputError(input, "local node input path is required", nil)
		}
		body, err := s.store.Read(ctx, key)
		if err != nil {
			return nil, nodeInputReadError(input, err)
		}
		return s.parseNodeInputContent(ctx, input, body)
	case "remote":
		return s.resolveRemoteNodeInput(ctx, input)
	case "ref":
		switch strings.ToLower(input.Ref.Kind) {
		case "subscription":
			return s.resolveSubscriptionNodeInput(ctx, input, req, subscriptionState)
		default:
			return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported node ref kind %q", input.Ref.Kind))
		}
	case "subscription":
		input.Ref.Kind = "subscription"
		return s.resolveSubscriptionNodeInput(ctx, input, req, subscriptionState)
	default:
		return nil, domain.NewError(domain.CodeNotImplemented, fmt.Sprintf("node input type %q not implemented", input.Type))
	}
}

func normalizeInlineNodes(nodes []domain.NodeIR) ([]domain.NodeIR, []domain.Warning) {
	out := make([]domain.NodeIR, len(nodes))
	copy(out, nodes)
	warnings := []domain.Warning{}
	for i := range out {
		if out[i].Hysteria != nil {
			hysteria := *out[i].Hysteria
			out[i].Hysteria = &hysteria
		}
		if out[i].Raw != nil {
			raw := make(map[string]json.RawMessage, len(out[i].Raw))
			for key, value := range out[i].Raw {
				raw[key] = append(json.RawMessage(nil), value...)
			}
			out[i].Raw = raw
		}
		warnings = append(warnings, shared.NormalizeLegacyHysteriaBandwidth(&out[i])...)
	}
	return out, warnings
}

func (s *Service) resolveSubscriptionNodeInput(ctx context.Context, input domain.NodeInput, req domain.FileRequest, subscriptionState *subscriptionResolveState) (*domain.NodeSet, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	name := refNameNode(input)
	if name == "" {
		return nil, missingNodeInputError(input, "subscription node input ref is required", nil)
	}
	sub, err := s.metaStore.GetSubscription(ctx, name)
	if err != nil {
		return nil, nodeInputReadError(input, err)
	}
	childReq := req
	childReq.Name = input.Name
	nodeSet, err := s.materializeSubscription(ctx, sub, childReq, subscriptionState)
	if err != nil {
		return nil, err
	}
	out := cloneNodeSet(nodeSet)
	out.Dependencies = append([]domain.ResourceRef{{Kind: "subscription", Name: name}}, out.Dependencies...)
	out.Meta = mergeStringMaps(out.Meta, input.Meta)
	return out, nil
}

func (s *Service) parseNodeInputContent(ctx context.Context, input domain.NodeInput, content []byte) (*domain.NodeSet, error) {
	return s.parseNodeInputContentWithSource(ctx, input, content, false, nil)
}

func (s *Service) parseNodeInputContentWithSource(ctx context.Context, input domain.NodeInput, content []byte, allowAuto bool, sourceRef *domain.SourceRef) (*domain.NodeSet, error) {
	parsed, err := s.parseNodeContent(ctx, input.Format, content, allowAuto, sourceRef)
	if err != nil {
		return nil, err
	}
	report := domain.Report{}
	if parsed.Source != nil {
		report.SourceRefs = append(report.SourceRefs, parsed.Source.SourceRefs...)
		report.Warnings = append(report.Warnings, parsed.Source.Warnings...)
	}
	for _, n := range parsed.Nodes {
		report.Warnings = append(report.Warnings, n.Warnings...)
	}
	return &domain.NodeSet{
		Nodes:    append([]domain.NodeIR{}, parsed.Nodes...),
		Sources:  sourcesSlice(parsed.Source),
		Warnings: append([]domain.Warning{}, report.Warnings...),
		Meta:     cloneStringMap(input.Meta),
	}, nil
}

func (s *Service) resolveRemoteNodeInput(ctx context.Context, input domain.NodeInput) (*domain.NodeSet, error) {
	if input.URL == "" {
		return nil, missingNodeInputError(input, "remote node input url is required", nil)
	}
	remote := domain.RemoteInput{
		URL:             input.URL,
		UserAgent:       input.UserAgent,
		Proxy:           input.Proxy,
		TimeoutMS:       input.TimeoutMS,
		CacheTTLSeconds: input.CacheTTLSeconds,
	}
	result, err := s.fetchRemoteCached(ctx, remote)
	if err != nil {
		return nil, nodeInputReadError(input, err)
	}
	nodeSet, err := s.parseNodeInputContentWithSource(ctx, input, result.Body, true, &result.SourceRef)
	if err != nil {
		return nil, err
	}
	return nodeSet, nil
}
