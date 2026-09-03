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
	if node.TLS != nil && node.TLS.ClientFingerprint != "" && !singBoxSupportsUTLS(node.Type) {
		warnings = append(warnings, lossyWarning(node, "tls.client_fingerprint", "sing-box does not support uTLS for QUIC-based outbounds"))
	}
	if node.TLS != nil && node.TLS.ECH != nil {
		if node.TLS.ECH.ForceQuery != "" {
			warnings = append(warnings, lossyWarning(node, "tls.ech.force_query", "sing-box ECH options do not expose the URI force-query mode"))
		}
	}
	if node.TLS != nil && node.TLS.Reality != nil {
		if node.TLS.Reality.MLDSA65Verify != "" {
			warnings = append(warnings, lossyWarning(node, "tls.reality.mldsa65_verify", "sing-box Reality options do not expose ML-DSA-65 certificate verification"))
		}
		if node.TLS.Reality.SpiderX != "" {
			warnings = append(warnings, lossyWarning(node, "tls.reality.spider_x", "sing-box Reality options do not expose the SpiderX fallback path"))
		}
	}
	if node.Transport != nil && node.Transport.XHTTP != nil && node.Transport.XHTTP.DownloadSettings != nil {
		tls := node.Transport.XHTTP.DownloadSettings.TLS
		if tls != nil && tls.Reality != nil {
			if tls.Reality.MLDSA65Verify != "" {
				warnings = append(warnings, lossyWarning(node, "transport.xhttp.download_settings.tls.reality.mldsa65_verify", "sing-box Reality options do not expose ML-DSA-65 certificate verification"))
			}
			if tls.Reality.SpiderX != "" {
				warnings = append(warnings, lossyWarning(node, "transport.xhttp.download_settings.tls.reality.spider_x", "sing-box Reality options do not expose the SpiderX fallback path"))
			}
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
	return warnings
}

func isHTTPHeaderTransport(transport *domain.TransportOptions) bool {
	return transport != nil && transport.Type == "tcp" && transport.HeaderType == "http"
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
