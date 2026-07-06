// Package uri parses and renders proxy sharing URI formats.
package uri

import (
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
)

func (p *Parser) ParseCapabilities() []shared.Capability {
	formats := []string{"uri", "uri-list", "base64"}
	capabilities := make([]shared.Capability, 0, len(formats))
	for _, format := range formats {
		capabilities = append(capabilities, shared.CapabilityFor(format, shared.DirectionParse, shared.URIProfileNodeTypes(), false))
	}
	return capabilities
}

func (r *Renderer) RenderCapabilities() []shared.Capability {
	return []shared.Capability{
		shared.CapabilityFor("uri-list", shared.DirectionRender, shared.URIProfileNodeTypes(), false),
	}
}
