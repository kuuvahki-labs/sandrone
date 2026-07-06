// Package jsonnodes encodes and decodes Sandrone node IR as JSON.
package jsonnodes

import "github.com/kuuvahki-labs/sandrone/internal/adapter/shared"

func (p *Parser) ParseCapabilities() []shared.Capability {
	return []shared.Capability{
		shared.CapabilityFor("json-nodes", shared.DirectionParse, shared.AllNodeTypes(), true),
	}
}

func (r *Renderer) RenderCapabilities() []shared.Capability {
	return []shared.Capability{
		shared.CapabilityFor("json-nodes", shared.DirectionRender, shared.AllNodeTypes(), true),
	}
}
