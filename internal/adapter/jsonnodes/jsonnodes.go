package jsonnodes

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Name() string {
	return "json-nodes"
}

func (p *Parser) Parse(_ context.Context, in []byte) ([]domain.NodeIR, *domain.SourceInfo, error) {
	decoder := json.NewDecoder(bytes.NewReader(in))
	decoder.UseNumber()
	var nodes []domain.NodeIR
	if err := decoder.Decode(&nodes); err == nil {
		for i := range nodes {
			if nodes[i].SourceFormat == "" {
				nodes[i].SourceFormat = "json-nodes"
			}
		}
		return nodes, &domain.SourceInfo{Format: "json-nodes"}, nil
	}
	decoder = json.NewDecoder(bytes.NewReader(in))
	decoder.UseNumber()
	var doc struct {
		Nodes []domain.NodeIR `json:"nodes"`
	}
	if err := decoder.Decode(&doc); err != nil {
		return nil, &domain.SourceInfo{Format: "json-nodes"}, domain.WrapError(domain.CodeParseFailed, "parse json nodes", err)
	}
	for i := range doc.Nodes {
		if doc.Nodes[i].SourceFormat == "" {
			doc.Nodes[i].SourceFormat = "json-nodes"
		}
	}
	return doc.Nodes, &domain.SourceInfo{Format: "json-nodes"}, nil
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
