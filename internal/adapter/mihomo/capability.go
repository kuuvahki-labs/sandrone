// Package mihomo adapts Sandrone node IR to and from mihomo configuration formats.
package mihomo

import (
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
)

func (p *Parser) ParseCapabilities() []shared.Capability {
	formats := []string{"mihomo"}
	capabilities := make([]shared.Capability, 0, len(formats))
	for _, format := range formats {
		capabilities = append(capabilities, shared.CapabilityFor(format, shared.DirectionParse, shared.AllNodeTypes(), false))
	}
	return capabilities
}

func (r *Renderer) RenderCapabilities() []shared.Capability {
	return []shared.Capability{
		shared.CapabilityFor(r.Name(), shared.DirectionRender, shared.AllNodeTypes(), false),
	}
}
