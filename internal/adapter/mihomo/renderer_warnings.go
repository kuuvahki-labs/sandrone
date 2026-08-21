package mihomo

import (
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func mihomoStructuredLossWarnings(node domain.NodeIR) []domain.Warning {
	warnings := []domain.Warning{}
	add := func(field, message string) {
		warnings = append(warnings, shared.RenderLossyWarning(node, "mihomo-proxies", field, message))
	}
	if node.Network != "" {
		add("network", "mihomo network is a transport selector and cannot represent the canonical tcp/udp protocol selector")
	}
	if node.Dialer != nil && node.Dialer.UDPRelay != nil && !mihomoSupportsUDPRelay(node.Type) {
		add("dialer.udp_relay", "mihomo proxy schema for "+string(node.Type)+" does not expose a udp relay field")
	}
	if isHTTPHeaderTransport(node.Transport) && !mihomoSupportsHTTPHeaderTransport(node.Type, node.Transport) {
		add("transport.header_type", "mihomo proxy schema for "+string(node.Type)+" does not support TCP HTTP header obfuscation")
	} else if node.Transport != nil && node.Transport.Type != "" && !mihomoSupportsTransport(node.Type, node.Transport.Type) && !isDefaultTCPTransport(node.Transport) && !mihomoSupportsHTTPHeaderTransport(node.Type, node.Transport) {
		add("transport.type", "mihomo proxy schema for "+string(node.Type)+" does not support "+node.Transport.Type+" transport")
	}
	if node.TLS != nil && node.TLS.ECH != nil {
		if node.TLS.ECH.ForceQuery != "" {
			add("tls.ech.force_query", "mihomo ECH options do not expose a force-query mode")
		}
	}
	switch node.Type {
	case domain.NodeTypeVMess, domain.NodeTypeVLESS, domain.NodeTypeTrojan:
		if node.Multiplex != nil {
			add("multiplex", "mihomo proxy schema has no general multiplex field; only selected grpc options can be emitted")
		}
	case domain.NodeTypeHysteria:
		if node.Hysteria != nil && len(node.Hysteria.QUIC) > 0 {
			add("hysteria.quic", "mihomo hysteria proxy schema has no stable QUIC tuning fields represented by NodeIR")
		}
	case domain.NodeTypeHysteria2:
		if node.Hysteria != nil {
			if node.Hysteria.UpMbps != 0 {
				add("hysteria.up_mbps", "mihomo hysteria2 proxy schema uses string up/down fields, not up_mbps")
			}
			if node.Hysteria.DownMbps != 0 {
				add("hysteria.down_mbps", "mihomo hysteria2 proxy schema uses string up/down fields, not down_mbps")
			}
			if len(node.Hysteria.QUIC) > 0 {
				add("hysteria.quic", "mihomo hysteria2 proxy schema has no stable QUIC tuning fields represented by NodeIR")
			}
		}
	case domain.NodeTypeTUIC:
		if node.TUIC != nil {
			if node.TUIC.ZeroRTTHandshake {
				add("tuic.zero_rtt_handshake", "mihomo tuic proxy schema has no zero_rtt_handshake field")
			}
			if node.TUIC.Heartbeat != "" {
				add("tuic.heartbeat", "mihomo tuic proxy schema uses heartbeat-interval, which is not emitted from NodeIR heartbeat")
			}
		}
	case domain.NodeTypeSOCKS:
		if node.UDPOverTCP != nil {
			add("udp_over_tcp", "mihomo socks5 proxy schema has no udp-over-tcp field")
		}
	case domain.NodeTypeHTTP:
		if node.Path != "" {
			add("path", "mihomo http proxy schema has no path field")
		}
	case domain.NodeTypeWireGuard:
		if node.WireGuard != nil {
			for _, peer := range node.WireGuard.Peers {
				if peer.PersistentKeepalive != 0 {
					add("wireguard.peers.persistent_keepalive", "mihomo wireguard peer schema has no persistent-keepalive field")
					break
				}
			}
		}
	}
	return warnings
}
