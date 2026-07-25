package uri

import (
	"context"
	"encoding/base64"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type Base64Renderer struct {
	uriList *Renderer
}

func NewBase64Renderer(uriList *Renderer) *Base64Renderer {
	return &Base64Renderer{uriList: uriList}
}

func (r *Base64Renderer) Name() string {
	return "base64"
}

func (r *Base64Renderer) Render(ctx context.Context, nodes []domain.NodeIR, opt domain.RenderOptions) ([]byte, error) {
	out, _, err := r.RenderWithReport(ctx, nodes, opt)
	return out, err
}

func (r *Base64Renderer) RenderWithReport(
	ctx context.Context,
	nodes []domain.NodeIR,
	opt domain.RenderOptions,
) ([]byte, domain.RenderReport, error) {
	plain, report, err := r.uriList.RenderWithReport(ctx, nodes, opt)
	if err != nil {
		return nil, report, err
	}
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(plain)))
	base64.StdEncoding.Encode(encoded, plain)
	return encoded, report, nil
}

func (r *Base64Renderer) RenderCapabilities() []shared.Capability {
	return []shared.Capability{
		shared.CapabilityFor(r.Name(), shared.DirectionRender, shared.URIProfileNodeTypes(), false),
	}
}
