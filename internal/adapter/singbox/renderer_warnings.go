package singbox

import (
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func singBoxStructuredLossWarnings(node domain.NodeIR) []domain.Warning {
	warnings := []domain.Warning{}
	if node.Dialer != nil && node.Dialer.UDPRelay != nil && !*node.Dialer.UDPRelay && !singBoxSupportsNetwork(node.Type) {
		warnings = append(warnings, lossyWarning(node, "dialer.udp_relay", "sing-box outbound network is a protocol selector, not a UDP relay enable flag"))
	}
	if node.TLS != nil && node.TLS.Fingerprint != "" {
		warnings = append(warnings, lossyWarning(node, "tls.fingerprint", "TLS certificate fingerprint is not rendered to sing-box utls.fingerprint"))
	}
	if node.TLS != nil && node.TLS.ECH != nil {
		if node.TLS.ECH.DNS != "" {
			warnings = append(warnings, lossyWarning(node, "tls.ech.dns", "sing-box ECH options do not expose the URI DNS transport field"))
		}
		if node.TLS.ECH.ForceQuery != "" {
			warnings = append(warnings, lossyWarning(node, "tls.ech.force_query", "sing-box ECH options do not expose the URI force-query mode"))
		}
	}
	if node.Type == domain.NodeTypeSOCKS && node.TLS != nil {
		warnings = append(warnings, lossyWarning(node, "tls", "sing-box socks outbound schema has no tls field"))
	}
	if node.Type == domain.NodeTypeHTTP && node.Multiplex != nil {
		warnings = append(warnings, lossyWarning(node, "multiplex", "sing-box http outbound schema has no multiplex field"))
	}
	if node.Type == domain.NodeTypeHysteria && node.Hysteria != nil && node.Hysteria.Protocol != "" {
		warnings = append(warnings, lossyWarning(node, "hysteria.protocol", "sing-box v1.13.14 hysteria outbound schema has no protocol selector"))
	}
	if singBoxNodeUsesV2RayTransport(node.Type) && node.Transport != nil && node.Transport.Type != "" && !singBoxSupportsTransport(node.Transport.Type) && !isDefaultTCPTransport(node.Transport) {
		warnings = append(warnings, lossyWarning(node, "transport.type", "sing-box v1.13.14 V2Ray transport schema does not support "+node.Transport.Type))
	}
	return warnings
}

func singBoxNodeUsesV2RayTransport(nodeType domain.NodeType) bool {
	switch nodeType {
	case domain.NodeTypeVMess, domain.NodeTypeVLESS, domain.NodeTypeTrojan:
		return true
	default:
		return false
	}
}

func lossyWarning(node domain.NodeIR, field, message string) domain.Warning {
	return shared.RenderLossyWarning(node, "sing-box-outbounds", field, message)
}

func firstNonEmptyRender(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
