package jsonnodes

import (
	"context"
	"encoding/json/v2"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/jsonvalue"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Name() string {
	return "json-nodes"
}

func (p *Parser) Parse(_ context.Context, in []byte) ([]domain.NodeIR, *domain.SourceInfo, error) {
	var nodes []domain.NodeIR
	if err := json.Unmarshal(in, &nodes, jsonvalue.PreserveNumbers); err == nil {
		normalizeLegacyHysteriaBandwidth(nodes)
		return nodes, &domain.SourceInfo{Format: "json-nodes"}, nil
	}
	var doc struct {
		Nodes []domain.NodeIR `json:"nodes"`
	}
	if err := json.Unmarshal(in, &doc, jsonvalue.PreserveNumbers); err != nil {
		return nil, &domain.SourceInfo{Format: "json-nodes"}, domain.WrapError(domain.CodeParseFailed, "parse json nodes", err)
	}
	normalizeLegacyHysteriaBandwidth(doc.Nodes)
	return doc.Nodes, &domain.SourceInfo{Format: "json-nodes"}, nil
}

func normalizeLegacyHysteriaBandwidth(nodes []domain.NodeIR) {
	for i := range nodes {
		if nodes[i].SourceFormat == "" {
			nodes[i].SourceFormat = "json-nodes"
		}
		nodes[i].Warnings = append(nodes[i].Warnings, shared.NormalizeLegacyHysteriaBandwidth(&nodes[i])...)
	}
}

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) Name() string {
	return "json-nodes"
}

func (r *Renderer) Render(ctx context.Context, nodes []domain.NodeIR, opt domain.RenderOptions) ([]byte, error) {
	out, _, err := r.RenderWithReport(ctx, nodes, opt)
	return out, err
}

func (r *Renderer) RenderWithReport(_ context.Context, nodes []domain.NodeIR, _ domain.RenderOptions) ([]byte, domain.RenderReport, error) {
	body, err := shared.MarshalStableJSON(nodes, true)
	return body, domain.RenderReport{SuccessCount: len(nodes)}, err
}
