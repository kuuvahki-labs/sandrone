// Package singbox adapts Sandrone node IR to and from sing-box configuration formats.
package singbox

import (
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
)

func (p *Parser) ParseCapabilities() []shared.Capability {
	formats := []string{"sing-box"}
	capabilities := make([]shared.Capability, 0, len(formats))
	for _, format := range formats {
		capabilities = append(capabilities, shared.CapabilityFor(format, shared.DirectionParse, shared.SingBoxNodeTypes(), false))
	}
	return capabilities
}

func (r *Renderer) RenderCapabilities() []shared.Capability {
	return []shared.Capability{
		shared.CapabilityFor("sing-box-outbounds", shared.DirectionRender, shared.SingBoxNodeTypes(), false),
	}
}
