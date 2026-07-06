package shadowrocket

import (
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func SupportedNodeTypes() []domain.NodeType {
	return []domain.NodeType{
		domain.NodeTypeShadowsocks,
		domain.NodeTypeVMess,
		domain.NodeTypeVLESS,
		domain.NodeTypeTrojan,
		domain.NodeTypeHysteria,
		domain.NodeTypeHysteria2,
		domain.NodeTypeTUIC,
		domain.NodeTypeHTTP,
		domain.NodeTypeSOCKS,
		domain.NodeTypeWireGuard,
		domain.NodeTypeSnell,
	}
}

func (r *Renderer) RenderCapabilities() []shared.Capability {
	return []shared.Capability{
		shared.CapabilityFor(r.Name(), shared.DirectionRender, SupportedNodeTypes(), false),
	}
}
