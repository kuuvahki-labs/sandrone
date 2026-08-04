package uri

import (
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func structuredLossWarnings(node domain.NodeIR, target string) []domain.Warning {
	warnings := []domain.Warning{}
	add := func(field string) {
		warnings = append(warnings, structuredLossWarning(node, field, target))
	}
	if node.Dialer != nil && node.Dialer.TFO {
		add("dialer.tfo")
	}
	if node.Dialer != nil && node.Dialer.UDPRelay != nil && (node.Type != domain.NodeTypeMieru || !*node.Dialer.UDPRelay) {
		add("dialer.udp_relay")
	}
	if node.Multiplex != nil {
		add("multiplex")
	}
	if node.UDPOverTCP != nil {
		add("udp_over_tcp")
	}
	if node.Network != "" {
		add("network")
	}
	switch node.Type {
	case domain.NodeTypeShadowsocks:
		if len(node.PluginOptions) > 0 && node.Plugin == "" {
			add("plugin_options")
		}
		addTLSLossWarnings(node, target, "none", &warnings)
		addTransportLossWarnings(node, target, false, &warnings)
	case domain.NodeTypeShadowsocksR:
		addTLSLossWarnings(node, target, "none", &warnings)
		addTransportLossWarnings(node, target, false, &warnings)
	case domain.NodeTypeVMess:
		if node.PacketEncoding != "" {
			add("packet_encoding")
		}
		addTLSLossWarnings(node, target, "vmess", &warnings)
		addTransportLossWarnings(node, target, true, &warnings)
	case domain.NodeTypeVLESS:
		addTLSLossWarnings(node, target, "query", &warnings)
		addTransportLossWarnings(node, target, true, &warnings)
	case domain.NodeTypeTrojan:
		if node.PacketEncoding != "" {
			add("packet_encoding")
		}
		addTLSLossWarnings(node, target, "query", &warnings)
		addTransportLossWarnings(node, target, true, &warnings)
	case domain.NodeTypeHysteria:
		addHysteriaLossWarnings(node, target, false, &warnings)
		addTLSLossWarnings(node, target, "hysteria", &warnings)
		addTransportLossWarnings(node, target, false, &warnings)
	case domain.NodeTypeHysteria2:
		addHysteriaLossWarnings(node, target, true, &warnings)
		addTLSLossWarnings(node, target, "hysteria2", &warnings)
		addTransportLossWarnings(node, target, false, &warnings)
	case domain.NodeTypeTUIC:
		addTUICLossWarnings(node, target, &warnings)
		addTLSLossWarnings(node, target, "query", &warnings)
		addTransportLossWarnings(node, target, false, &warnings)
	case domain.NodeTypeSOCKS:
		addTLSLossWarnings(node, target, "none", &warnings)
		addTransportLossWarnings(node, target, false, &warnings)
	case domain.NodeTypeHTTP:
		if len(node.Headers) > 0 {
			add("headers")
		}
		if node.Path != "" {
			add("path")
		}
		addTLSLossWarnings(node, target, "http", &warnings)
		addTransportLossWarnings(node, target, false, &warnings)
	}
	return warnings
}

func addTLSLossWarnings(node domain.NodeIR, target, mode string, warnings *[]domain.Warning) {
	tls := node.TLS
	if tls == nil {
		return
	}
	add := func(field string) {
		*warnings = append(*warnings, structuredLossWarning(node, field, target))
	}
	switch mode {
	case "query":
		if tls.DisableSNI {
			add("tls.disable_sni")
		}
	case "hysteria":
		if tls.ClientFingerprint != "" {
			add("tls.client_fingerprint")
		}
		if tls.Fingerprint != "" {
			add("tls.fingerprint")
		}
		if tls.DisableSNI {
			add("tls.disable_sni")
		}
		if tls.ECH != nil {
			add("tls.ech")
		}
		if tls.Reality != nil {
			add("tls.reality")
		}
	case "hysteria2":
		if len(tls.ALPN) > 0 {
			add("tls.alpn")
		}
		if tls.ClientFingerprint != "" {
			add("tls.client_fingerprint")
		}
		if tls.DisableSNI {
			add("tls.disable_sni")
		}
		if tls.ECH != nil {
			add("tls.ech")
		}
		if tls.Reality != nil {
			add("tls.reality")
		}
	case "vmess":
		if tls.InsecureSkipVerify {
			add("tls.insecure_skip_verify")
		}
		if len(tls.ALPN) > 0 {
			add("tls.alpn")
		}
		if tls.DisableSNI {
			add("tls.disable_sni")
		}
		if tls.ECH != nil {
			add("tls.ech")
		}
		if tls.Reality != nil {
			add("tls.reality")
		}
	case "http":
		if tls.ServerName != "" {
			add("tls.server_name")
		}
		if tls.InsecureSkipVerify {
			add("tls.insecure_skip_verify")
		}
		if len(tls.ALPN) > 0 {
			add("tls.alpn")
		}
		if tls.ClientFingerprint != "" {
			add("tls.client_fingerprint")
		}
		if tls.Fingerprint != "" {
			add("tls.fingerprint")
		}
		if tls.DisableSNI {
			add("tls.disable_sni")
		}
		if tls.ECH != nil {
			add("tls.ech")
		}
		if tls.Reality != nil {
			add("tls.reality")
		}
	case "none":
		if tls.Enabled {
			add("tls.enabled")
		}
		if tls.ServerName != "" {
			add("tls.server_name")
		}
		if tls.InsecureSkipVerify {
			add("tls.insecure_skip_verify")
		}
		if len(tls.ALPN) > 0 {
			add("tls.alpn")
		}
		if tls.ClientFingerprint != "" {
			add("tls.client_fingerprint")
		}
		if tls.Fingerprint != "" {
			add("tls.fingerprint")
		}
		if tls.DisableSNI {
			add("tls.disable_sni")
		}
		if tls.ECH != nil {
			add("tls.ech")
		}
		if tls.Reality != nil {
			add("tls.reality")
		}
	}
}

func addTransportLossWarnings(node domain.NodeIR, target string, emitsBasic bool, warnings *[]domain.Warning) {
	transport := node.Transport
	if transport == nil {
		return
	}
	add := func(field string) {
		*warnings = append(*warnings, structuredLossWarning(node, field, target))
	}
	if !emitsBasic {
		if transport.Type != "" {
			add("transport.type")
		}
		if transport.Path != "" {
			add("transport.path")
		}
		if transport.Host != "" {
			add("transport.host")
		}
		if transport.ServiceName != "" {
			add("transport.service_name")
		}
	}
	if transport.Method != "" {
		add("transport.method")
	}
	if len(transport.Hosts) > 0 {
		add("transport.hosts")
	}
	if len(transport.Headers) > 0 {
		add("transport.headers")
	}
	if transport.MaxEarlyData != 0 {
		add("transport.max_early_data")
	}
	if transport.EarlyDataHeaderName != "" {
		add("transport.early_data_header_name")
	}
}

func addHysteriaLossWarnings(node domain.NodeIR, target string, hysteria2 bool, warnings *[]domain.Warning) {
	hy := node.Hysteria
	if hy == nil {
		return
	}
	add := func(field string) {
		*warnings = append(*warnings, structuredLossWarning(node, field, target))
	}
	if len(hy.ServerPorts) > 0 && !hysteria2 {
		add("hysteria.server_ports")
	}
	if hy.HopInterval != "" {
		add("hysteria.hop_interval")
	}
	if hysteria2 {
		if hy.Up != "" {
			add("hysteria.up")
		}
		if hy.Down != "" {
			add("hysteria.down")
		}
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
	} else {
		if hy.Up != "" {
			add("hysteria.up")
		}
		if hy.Down != "" {
			add("hysteria.down")
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
}

func addTUICLossWarnings(node domain.NodeIR, target string, warnings *[]domain.Warning) {
	tuic := node.TUIC
	if tuic == nil {
		return
	}
	add := func(field string) {
		*warnings = append(*warnings, structuredLossWarning(node, field, target))
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
}

func structuredLossWarning(node domain.NodeIR, field, target string) domain.Warning {
	return shared.RenderLossyWarning(node, target, field, "")
}

func unsupportedURIWarning(node domain.NodeIR) domain.Warning {
	return domain.Warning{
		Code:    "uri_profile_unsupported",
		Message: "node type has no supported URI profile",
		Node:    node.Name,
		Field:   string(node.Type),
		Target:  "uri-list",
	}
}
