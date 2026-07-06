package shadowrocket

import (
	"slices"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func structuredLossWarnings(node domain.NodeIR, emitted emittedFields) []domain.Warning {
	warnings := []domain.Warning{}
	add := func(field string) {
		if emitted[field] {
			return
		}
		warnings = append(warnings, protocolLossWarning(node, field))
	}

	if node.Type == domain.NodeTypeHTTP {
		if len(node.Headers) > 0 {
			add("headers")
		}
		if node.Path != "" {
			add("path")
		}
	}
	if node.Network != "" {
		add("network")
	}
	if len(node.PluginOptions) > 0 {
		add("plugin_options")
	}
	if node.PacketEncoding != "" {
		add("packet_encoding")
	}
	if node.Flow != "" {
		add("flow")
	}
	if node.Encryption != "" {
		add("encryption")
	}
	if node.Dialer != nil {
		if node.Dialer.Network != "" {
			add("dialer.network")
		}
		if node.Dialer.TFO {
			add("dialer.tfo")
		}
		if node.Dialer.UDPRelay != nil {
			add("dialer.udp_relay")
		}
	}
	if node.Multiplex != nil {
		add("multiplex")
	}
	if node.UDPOverTCP != nil {
		add("udp_over_tcp")
	}
	if node.Transport != nil && node.Transport.Type != "" {
		add("transport.type")
	}
	if node.TLS != nil {
		if node.TLS.Enabled {
			add("tls.enabled")
		}
		if node.TLS.ServerName != "" {
			add("tls.server_name")
		}
		if node.TLS.InsecureSkipVerify {
			add("tls.insecure_skip_verify")
		}
		if len(node.TLS.ALPN) > 0 {
			add("tls.alpn")
		}
		if node.TLS.ClientFingerprint != "" {
			add("tls.client_fingerprint")
		}
		if node.TLS.Fingerprint != "" {
			add("tls.fingerprint")
		}
	}
	if node.Type == domain.NodeTypeSnell && node.Snell != nil && node.Snell.ClientFingerprint != "" {
		add("snell.client_fingerprint")
	}
	return warnings
}

func hysteriaLossWarnings(node domain.NodeIR, hysteria2 bool) []domain.Warning {
	if node.Hysteria == nil {
		return nil
	}
	hy := node.Hysteria
	warnings := []domain.Warning{}
	add := func(field string) {
		warnings = append(warnings, protocolLossWarning(node, field))
	}
	if hy.HopInterval != "" {
		add("hysteria.hop_interval")
	}
	if hy.Up != "" {
		add("hysteria.up")
	}
	if hy.Down != "" {
		add("hysteria.down")
	}
	if hysteria2 {
		if hy.UpMbps != 0 {
			add("hysteria.up_mbps")
		}
		if hy.DownMbps != 0 {
			add("hysteria.down_mbps")
		}
		if hy.Auth != "" {
			add("hysteria.auth")
		}
		if hy.AuthString != "" {
			add("hysteria.auth_str")
		}
	}
	if hy.Realm != nil {
		add("hysteria.realm")
	}
	if hy.BBRProfile != "" {
		add("hysteria.bbr_profile")
	}
	if hy.CWND != 0 {
		add("hysteria.cwnd")
	}
	if hy.UDPMTU != 0 {
		add("hysteria.udp_mtu")
	}
	if len(hy.QUIC) > 0 {
		add("hysteria.quic")
	}
	return warnings
}

func tuicLossWarnings(node domain.NodeIR) []domain.Warning {
	if node.TUIC == nil {
		return nil
	}
	tuic := node.TUIC
	warnings := []domain.Warning{}
	add := func(field string) {
		warnings = append(warnings, protocolLossWarning(node, field))
	}
	if tuic.CongestionControl != "" {
		add("tuic.congestion_control")
	}
	if tuic.UDPRelayMode != "" {
		add("tuic.udp_relay_mode")
	}
	if tuic.ZeroRTTHandshake {
		add("tuic.zero_rtt_handshake")
	}
	if tuic.ReduceRTT {
		add("tuic.reduce_rtt")
	}
	if tuic.Heartbeat != "" {
		add("tuic.heartbeat")
	}
	if tuic.UDPOverStream {
		add("tuic.udp_over_stream")
	}
	if tuic.UDPOverStreamVersion != 0 {
		add("tuic.udp_over_stream_version")
	}
	return warnings
}

func wireGuardLossWarnings(node domain.NodeIR) []domain.Warning {
	if node.WireGuard == nil || len(node.WireGuard.Peers) != 1 {
		return nil
	}
	wg := node.WireGuard
	peer := wg.Peers[0]
	warnings := []domain.Warning{}
	add := func(field string) {
		warnings = append(warnings, protocolLossWarning(node, field))
	}
	addresses := map[string]bool{}
	for _, address := range wg.Address {
		if address != "" {
			addresses[address] = true
		}
	}
	if wg.IP != "" {
		addresses[wg.IP] = true
	}
	if wg.IPv6 != "" {
		addresses[wg.IPv6] = true
	}
	if len(addresses) > 1 {
		add("wireguard.address")
	}
	if wg.PersistentKeepalive != 0 && peer.PersistentKeepalive != 0 && wg.PersistentKeepalive != peer.PersistentKeepalive {
		add("wireguard.persistent_keepalive")
	}
	if len(wg.Reserved) > 0 && len(peer.Reserved) > 0 && !slices.Equal(wg.Reserved, peer.Reserved) {
		add("wireguard.reserved")
	}
	if len(peer.AllowedIPs) > 0 {
		add("wireguard.allowed_ips")
	}
	if wg.Workers != 0 {
		add("wireguard.workers")
	}
	return warnings
}
