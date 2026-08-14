// Package shared provides adapter capabilities, source traces, and shared rendering helpers.
package shared

import "github.com/kuuvahki-labs/sandrone/internal/domain"

type Direction = domain.CapabilityDirection

const (
	DirectionParse  = domain.CapabilityDirectionParse
	DirectionRender = domain.CapabilityDirectionRender
)

type FieldStatus = domain.CapabilityFieldStatus

const (
	FieldStatusSupported = domain.CapabilityFieldStatusSupported
	FieldStatusLossy     = domain.CapabilityFieldStatusLossy
	FieldStatusRawOnly   = domain.CapabilityFieldStatusRawOnly
)

type FieldRef = domain.CapabilityFieldRef

type Capability = domain.FormatCapability

func AllNodeTypes() []domain.NodeType {
	return []domain.NodeType{
		domain.NodeTypeShadowsocks,
		domain.NodeTypeShadowsocksR,
		domain.NodeTypeVMess,
		domain.NodeTypeVLESS,
		domain.NodeTypeTrojan,
		domain.NodeTypeHysteria,
		domain.NodeTypeHysteria2,
		domain.NodeTypeTUIC,
		domain.NodeTypeMieru,
		domain.NodeTypeSOCKS,
		domain.NodeTypeHTTP,
		domain.NodeTypeWireGuard,
		domain.NodeTypeSnell,
		domain.NodeTypeAnyTLS,
	}
}

func URIProfileNodeTypes() []domain.NodeType {
	return []domain.NodeType{
		domain.NodeTypeShadowsocks,
		domain.NodeTypeShadowsocksR,
		domain.NodeTypeVMess,
		domain.NodeTypeVLESS,
		domain.NodeTypeTrojan,
		domain.NodeTypeHysteria,
		domain.NodeTypeHysteria2,
		domain.NodeTypeTUIC,
		domain.NodeTypeMieru,
		domain.NodeTypeSOCKS,
		domain.NodeTypeHTTP,
		domain.NodeTypeAnyTLS,
	}
}

func SingBoxNodeTypes() []domain.NodeType {
	return []domain.NodeType{
		domain.NodeTypeShadowsocks,
		domain.NodeTypeVMess,
		domain.NodeTypeVLESS,
		domain.NodeTypeTrojan,
		domain.NodeTypeHysteria,
		domain.NodeTypeHysteria2,
		domain.NodeTypeTUIC,
		domain.NodeTypeSOCKS,
		domain.NodeTypeHTTP,
		domain.NodeTypeWireGuard,
		domain.NodeTypeAnyTLS,
	}
}

func CapabilityFor(format string, direction Direction, types []domain.NodeType, reversible bool) Capability {
	capability := Capability{
		Format:     format,
		Direction:  direction,
		Types:      append([]domain.NodeType{}, types...),
		Fields:     fieldRefsForStatus(format, direction, types, FieldStatusSupported),
		Lossy:      fieldRefsForStatus(format, direction, types, FieldStatusLossy),
		RawOnly:    fieldRefsForStatus(format, direction, types, FieldStatusRawOnly),
		Reversible: reversible,
	}
	if len(capability.Fields) == 0 {
		capability.Fields = legacyProtocolFieldRefs(format, types)
	}
	return capability
}

func FieldRefs(format string, types []domain.NodeType) []FieldRef {
	fields := fieldRefsForStatus(format, "", types, FieldStatusSupported)
	if len(fields) == 0 {
		return legacyProtocolFieldRefs(format, types)
	}
	return fields
}

func fieldRefsForStatus(format string, direction Direction, types []domain.NodeType, status FieldStatus) []FieldRef {
	out := []FieldRef{}
	for _, nodeType := range types {
		for _, field := range catalogFieldNames(format, direction, nodeType, status) {
			out = append(out, fieldRef(format, nodeType, field, status, catalogNotes(format, status)))
		}
	}
	return out
}

func fieldRef(format string, protocol domain.NodeType, field string, status FieldStatus, notes string) FieldRef {
	return FieldRef{
		IRField:   field,
		Protocol:  string(protocol),
		Status:    status,
		SourceRef: SourceRefFor(format, protocol),
		Notes:     notes,
	}
}

func catalogNotes(format string, status FieldStatus) string {
	switch status {
	case FieldStatusLossy:
		return "field is represented in NodeIR but is not emitted by the " + format + " renderer"
	case FieldStatusRawOnly:
		return "field is preserved in NodeIR Raw and is not promoted to a stable IR field"
	default:
		return ""
	}
}

func catalogFieldNames(format string, direction Direction, nodeType domain.NodeType, status FieldStatus) []string {
	canonical := canonicalCapabilityFormat(format)
	switch status {
	case FieldStatusSupported:
		return supportedFieldNames(canonical, nodeType)
	case FieldStatusLossy:
		if direction == DirectionParse {
			return nil
		}
		return lossyFieldNames(canonical, nodeType)
	case FieldStatusRawOnly:
		if direction == DirectionRender {
			return nil
		}
		return rawOnlyFieldNames(canonical, nodeType)
	default:
		return nil
	}
}

func canonicalCapabilityFormat(format string) string {
	switch format {
	case "mihomo", "mihomo-proxies":
		return "mihomo"
	case "sing-box", "sing-box-outbounds":
		return "sing-box"
	case "uri", "uri-list", "base64":
		return "uri-list"
	case "shadowrocket", "shadowrocket-proxies":
		return "shadowrocket"
	default:
		return format
	}
}

func supportedFieldNames(format string, nodeType domain.NodeType) []string {
	switch format {
	case "mihomo":
		return mihomoSupportedFieldNames(nodeType)
	case "sing-box":
		return singBoxSupportedFieldNames(nodeType)
	case "uri-list":
		return uriSupportedFieldNames(nodeType)
	case "json-nodes":
		return append(legacyProtocolFieldNames(nodeType), "dialer.udp_relay", "tags", "meta", "raw", "warnings", "source_format")
	case "shadowrocket":
		return shadowrocketSupportedFieldNames(nodeType)
	default:
		return nil
	}
}

func lossyFieldNames(format string, nodeType domain.NodeType) []string {
	switch format {
	case "mihomo":
		return mihomoLossyFieldNames(nodeType)
	case "sing-box":
		return singBoxLossyFieldNames(nodeType)
	case "uri-list":
		return uriLossyFieldNames(nodeType)
	case "shadowrocket":
		return shadowrocketLossyFieldNames(nodeType)
	default:
		return nil
	}
}

func rawOnlyFieldNames(format string, nodeType domain.NodeType) []string {
	switch format {
	case "mihomo":
		switch nodeType {
		case domain.NodeTypeVLESS:
			return []string{"mihomo.xhttp-opts.x-padding-bytes", "mihomo.grpc-opts.grpc-user-agent", "mihomo.grpc-opts.ping-interval"}
		case domain.NodeTypeVMess, domain.NodeTypeTrojan:
			return []string{"mihomo.grpc-opts.grpc-user-agent", "mihomo.grpc-opts.ping-interval"}
		}
	case "sing-box":
		switch nodeType {
		case domain.NodeTypeHysteria2:
			return []string{"sing-box.realm", "sing-box.bbr_profile", "sing-box.initial_packet_size"}
		case domain.NodeTypeWireGuard:
			return []string{"sing-box.local_address", "sing-box.peer_public_key", "sing-box.allowed_ips"}
		}
	}
	return nil
}

func mihomoSupportedFieldNames(nodeType domain.NodeType) []string {
	switch nodeType {
	case domain.NodeTypeShadowsocks:
		return fields(commonNodeFields(), "cipher", "password", "plugin", "plugin_options", "udp_over_tcp", "dialer.udp_relay", "dialer.tfo")
	case domain.NodeTypeShadowsocksR:
		return fields(commonNodeFields(), "cipher", "password", "shadowsocksr.protocol", "shadowsocksr.protocol_param", "shadowsocksr.obfs", "shadowsocksr.obfs_param", "dialer.udp_relay", "dialer.tfo")
	case domain.NodeTypeSnell:
		return fields(commonNodeFields(), "password", "snell.version", "snell.obfs", "snell.obfs_host", "snell.reuse", "snell.client_fingerprint", "snell.shadow_tls", "dialer.udp_relay", "dialer.tfo")
	case domain.NodeTypeAnyTLS:
		return fields(commonNodeFields(), "password", "tls", "anytls.idle_session_check_interval", "anytls.idle_session_timeout", "anytls.min_idle_session", "dialer.tfo")
	case domain.NodeTypeVMess:
		return fields(commonNodeFields(), "uuid", "cipher", "alter_id", "tls", "transport", "packet_encoding", "dialer.udp_relay", "dialer.tfo")
	case domain.NodeTypeVLESS:
		return fields(commonNodeFields(), "uuid", "flow", "encryption", "tls", "reality", "transport", "packet_encoding", "dialer.udp_relay", "dialer.tfo")
	case domain.NodeTypeTrojan:
		return fields(commonNodeFields(), "password", "tls", "reality", "transport", "dialer.udp_relay", "dialer.tfo")
	case domain.NodeTypeHysteria:
		return fields(commonNodeFields(), "tls", "hysteria.protocol", "hysteria.server_ports", "hysteria.hop_interval", "hysteria.up", "hysteria.down", "hysteria.up_mbps", "hysteria.down_mbps", "hysteria.auth", "hysteria.auth_str", "hysteria.obfs", "hysteria.obfs_password", "dialer.tfo")
	case domain.NodeTypeHysteria2:
		return fields(commonNodeFields(), "password", "tls", "hysteria.server_ports", "hysteria.hop_interval", "hysteria.up", "hysteria.down", "hysteria.obfs", "hysteria.obfs_password", "hysteria.realm", "hysteria.bbr_profile", "hysteria.cwnd", "hysteria.udp_mtu", "dialer.tfo")
	case domain.NodeTypeTUIC:
		return fields(commonNodeFields(), "uuid", "password", "token", "tls", "tuic.congestion_control", "tuic.udp_relay_mode", "tuic.reduce_rtt", "tuic.udp_over_stream", "tuic.udp_over_stream_version", "dialer.tfo")
	case domain.NodeTypeMieru:
		return fields(commonNodeFields(), "username", "password", "dialer.udp_relay", "mieru.port_range", "mieru.transport", "mieru.multiplexing", "mieru.handshake_mode", "mieru.traffic_pattern")
	case domain.NodeTypeSOCKS:
		return fields(commonNodeFields(), "username", "password", "tls", "dialer.udp_relay", "dialer.tfo")
	case domain.NodeTypeHTTP:
		return fields(commonNodeFields(), "username", "password", "headers", "tls", "dialer.tfo")
	case domain.NodeTypeWireGuard:
		return fields(commonNodeFields(), "wireguard.private_key", "wireguard.address", "wireguard.ip", "wireguard.ipv6", "wireguard.peers", "wireguard.public_key", "wireguard.pre_shared_key", "wireguard.allowed_ips", "wireguard.mtu", "wireguard.reserved", "wireguard.persistent_keepalive", "dialer.udp_relay")
	default:
		return nil
	}
}

func singBoxSupportedFieldNames(nodeType domain.NodeType) []string {
	switch nodeType {
	case domain.NodeTypeShadowsocks:
		return fields(commonNodeFields(), "cipher", "password", "plugin", "plugin_options", "network", "udp_over_tcp", "multiplex", "dialer.tfo")
	case domain.NodeTypeVMess:
		return fields(commonNodeFields(), "uuid", "cipher", "alter_id", "tls", "transport", "packet_encoding", "multiplex", "network", "dialer.tfo")
	case domain.NodeTypeVLESS:
		return fields(commonNodeFields(), "uuid", "flow", "tls", "reality", "transport", "packet_encoding", "multiplex", "network", "dialer.tfo")
	case domain.NodeTypeTrojan:
		return fields(commonNodeFields(), "password", "tls", "transport", "multiplex", "network", "dialer.tfo")
	case domain.NodeTypeHysteria:
		return fields(commonNodeFields(), "tls", "hysteria.server_ports", "hysteria.hop_interval", "hysteria.up", "hysteria.down", "hysteria.up_mbps", "hysteria.down_mbps", "hysteria.auth", "hysteria.auth_str", "hysteria.obfs", "hysteria.obfs_password", "network", "dialer.tfo")
	case domain.NodeTypeHysteria2:
		return fields(commonNodeFields(), "password", "tls", "hysteria.server_ports", "hysteria.hop_interval", "hysteria.up_mbps", "hysteria.down_mbps", "hysteria.obfs", "hysteria.obfs_password", "network", "dialer.tfo")
	case domain.NodeTypeTUIC:
		return fields(commonNodeFields(), "uuid", "password", "tls", "tuic.congestion_control", "tuic.udp_relay_mode", "tuic.zero_rtt_handshake", "tuic.heartbeat", "tuic.udp_over_stream", "network", "dialer.tfo")
	case domain.NodeTypeSOCKS:
		return fields(commonNodeFields(), "username", "password", "network", "udp_over_tcp", "dialer.tfo")
	case domain.NodeTypeHTTP:
		return fields(commonNodeFields(), "username", "password", "path", "headers", "tls", "dialer.tfo")
	case domain.NodeTypeWireGuard:
		return fields([]string{"name", "type"}, "wireguard.private_key", "wireguard.address", "wireguard.peers", "wireguard.public_key", "wireguard.pre_shared_key", "wireguard.allowed_ips", "wireguard.mtu", "wireguard.reserved", "wireguard.persistent_keepalive", "wireguard.workers")
	case domain.NodeTypeAnyTLS:
		return fields(commonNodeFields(), "password", "tls", "anytls.idle_session_check_interval", "anytls.idle_session_timeout", "anytls.min_idle_session", "dialer.tfo")
	default:
		return nil
	}
}

func uriSupportedFieldNames(nodeType domain.NodeType) []string {
	switch nodeType {
	case domain.NodeTypeShadowsocks:
		return fields(commonNodeFields(), "cipher", "password", "plugin")
	case domain.NodeTypeShadowsocksR:
		return fields(commonNodeFields(), "cipher", "password", "shadowsocksr.protocol", "shadowsocksr.protocol_param", "shadowsocksr.obfs", "shadowsocksr.obfs_param")
	case domain.NodeTypeVMess:
		return fields(commonNodeFields(), "uuid", "cipher", "alter_id", "tls", "transport")
	case domain.NodeTypeVLESS:
		return fields(commonNodeFields(), "uuid", "flow", "encryption", "tls", "reality", "transport", "packet_encoding")
	case domain.NodeTypeTrojan:
		return fields(commonNodeFields(), "password", "tls", "transport")
	case domain.NodeTypeHysteria:
		return fields(commonNodeFields(), "tls", "hysteria.protocol", "hysteria.auth", "hysteria.auth_str", "hysteria.obfs", "hysteria.obfs_password", "hysteria.up_mbps", "hysteria.down_mbps")
	case domain.NodeTypeHysteria2:
		return fields(commonNodeFields(), "password", "tls", "hysteria.server_ports", "hysteria.obfs", "hysteria.obfs_password")
	case domain.NodeTypeTUIC:
		return fields(commonNodeFields(), "uuid", "password", "token", "tls", "tuic.congestion_control", "tuic.udp_relay_mode", "tuic.zero_rtt_handshake")
	case domain.NodeTypeMieru:
		return fields(commonNodeFields(), "username", "password", "dialer.udp_relay", "mieru.port_range", "mieru.transport", "mieru.multiplexing", "mieru.handshake_mode", "mieru.traffic_pattern")
	case domain.NodeTypeSOCKS:
		return fields(commonNodeFields(), "username", "password")
	case domain.NodeTypeHTTP:
		return fields(commonNodeFields(), "username", "password", "tls.enabled")
	case domain.NodeTypeAnyTLS:
		return fields(commonNodeFields(), "password", "tls", "anytls.idle_session_check_interval", "anytls.idle_session_timeout", "anytls.min_idle_session")
	default:
		return nil
	}
}

func shadowrocketSupportedFieldNames(nodeType domain.NodeType) []string {
	common := commonNodeFields()
	switch nodeType {
	case domain.NodeTypeShadowsocks:
		return fields(common, "cipher", "password", "plugin", "transport.type")
	case domain.NodeTypeVMess:
		return fields(common, "uuid", "cipher", "alter_id", "transport.type", "dialer.tfo")
	case domain.NodeTypeVLESS:
		return fields(common, "uuid", "encryption", "tls.enabled", "tls.server_name", "transport.type")
	case domain.NodeTypeTrojan:
		return fields(common, "password", "tls.enabled", "tls.server_name", "tls.insecure_skip_verify", "transport.type")
	case domain.NodeTypeHysteria:
		return fields(common, "hysteria.protocol", "tls.enabled", "tls.server_name", "tls.alpn", "hysteria.auth", "hysteria.auth_str", "hysteria.obfs", "hysteria.obfs_password", "hysteria.up_mbps", "hysteria.down_mbps", "dialer.udp_relay")
	case domain.NodeTypeHysteria2:
		return fields(common, "password", "tls.enabled", "tls.server_name", "tls.alpn", "hysteria.obfs", "hysteria.obfs_password", "dialer.udp_relay")
	case domain.NodeTypeTUIC:
		return fields(common, "uuid", "password", "tls.enabled", "tls.server_name", "tls.alpn", "dialer.udp_relay")
	case domain.NodeTypeHTTP:
		return fields(common, "username", "password", "tls.enabled")
	case domain.NodeTypeSOCKS:
		return fields(common, "username", "password", "tls.enabled", "tls.insecure_skip_verify")
	case domain.NodeTypeWireGuard:
		return fields(common, "wireguard.private_key", "wireguard.address", "wireguard.peers", "wireguard.public_key", "wireguard.mtu", "wireguard.reserved", "wireguard.persistent_keepalive", "dialer.udp_relay")
	case domain.NodeTypeSnell:
		return fields(common, "password", "snell.version", "snell.obfs", "snell.obfs_host", "snell.reuse", "dialer.udp_relay")
	default:
		return nil
	}
}

func mihomoLossyFieldNames(nodeType domain.NodeType) []string {
	common := []string{"network"}
	tlsECHAdvanced := []string{"tls.ech.dns", "tls.ech.force_query"}
	switch nodeType {
	case domain.NodeTypeShadowsocks:
		return fields(common, "transport.type")
	case domain.NodeTypeVMess, domain.NodeTypeVLESS:
		return fieldGroups(common, tlsECHAdvanced, []string{"multiplex", "transport.type"})
	case domain.NodeTypeTrojan:
		return fieldGroups(common, tlsECHAdvanced, []string{"multiplex", "transport.type", "transport.header_type"})
	case domain.NodeTypeHysteria:
		return fields(common, "dialer.udp_relay", "hysteria.quic", "transport.type")
	case domain.NodeTypeHysteria2:
		return fields(common, "dialer.udp_relay", "hysteria.up_mbps", "hysteria.down_mbps", "hysteria.quic", "transport.type")
	case domain.NodeTypeTUIC:
		return fields(common, "dialer.udp_relay", "tuic.zero_rtt_handshake", "tuic.heartbeat", "transport.type")
	case domain.NodeTypeSOCKS:
		return fields(common, "udp_over_tcp", "transport.type")
	case domain.NodeTypeHTTP:
		return fields(common, "dialer.udp_relay", "path", "transport.type")
	case domain.NodeTypeWireGuard:
		return fields(common, "wireguard.peers.persistent_keepalive", "transport.type")
	case domain.NodeTypeAnyTLS:
		return fieldGroups(common, tlsECHAdvanced, []string{"tls.reality"})
	default:
		return fields(common, tlsECHAdvanced...)
	}
}

func singBoxLossyFieldNames(nodeType domain.NodeType) []string {
	common := []string{"dialer.udp_relay", "tls.ech.dns", "tls.ech.force_query"}
	tlsFingerprint := fields(common, "tls.fingerprint")
	switch nodeType {
	case domain.NodeTypeVLESS:
		return fields(tlsFingerprint, "encryption", "transport.type", "transport.header_type")
	case domain.NodeTypeVMess, domain.NodeTypeTrojan:
		return fields(tlsFingerprint, "transport.type", "transport.header_type")
	case domain.NodeTypeHysteria:
		return fields(tlsFingerprint, "hysteria.protocol", "hysteria.quic")
	case domain.NodeTypeHysteria2:
		return fields(tlsFingerprint, "hysteria.up", "hysteria.down", "hysteria.bbr_profile", "hysteria.realm", "hysteria.cwnd", "hysteria.udp_mtu", "hysteria.quic")
	case domain.NodeTypeTUIC:
		return fields(tlsFingerprint, "token", "tuic.reduce_rtt", "tuic.udp_over_stream_version")
	case domain.NodeTypeSOCKS:
		return fields(common, "tls")
	case domain.NodeTypeHTTP:
		return fields(tlsFingerprint, "multiplex")
	case domain.NodeTypeShadowsocks, domain.NodeTypeWireGuard:
		return common
	default:
		return nil
	}
}

func uriLossyFieldNames(nodeType domain.NodeType) []string {
	common := []string{"dialer.tfo", "dialer.udp_relay", "multiplex", "udp_over_tcp", "network"}
	switch nodeType {
	case domain.NodeTypeShadowsocks:
		return fieldGroups(common, []string{"plugin_options"}, uriTLSLossFields("none"), uriTransportLossFields(false))
	case domain.NodeTypeVMess:
		return fieldGroups(common, []string{"packet_encoding"}, uriTLSLossFields("vmess"), uriTransportLossFields(true))
	case domain.NodeTypeVLESS:
		return fieldGroups(common, uriTLSLossFields("query"), uriTransportLossFields(true))
	case domain.NodeTypeTrojan:
		return fieldGroups(common, []string{"packet_encoding"}, uriTLSLossFields("query"), uriTransportLossFields(true))
	case domain.NodeTypeHysteria:
		return fieldGroups(common,
			[]string{"hysteria.server_ports", "hysteria.hop_interval", "hysteria.up", "hysteria.down", "hysteria.realm", "hysteria.bbr_profile", "hysteria.cwnd", "hysteria.udp_mtu", "hysteria.quic"},
			uriTLSLossFields("hysteria"),
			uriTransportLossFields(false),
		)
	case domain.NodeTypeHysteria2:
		return fieldGroups(common,
			[]string{"hysteria.hop_interval", "hysteria.up", "hysteria.down", "hysteria.up_mbps", "hysteria.down_mbps", "hysteria.auth", "hysteria.auth_str", "hysteria.realm", "hysteria.bbr_profile", "hysteria.cwnd", "hysteria.udp_mtu", "hysteria.quic"},
			uriTLSLossFields("hysteria2"),
			uriTransportLossFields(false),
		)
	case domain.NodeTypeTUIC:
		return fieldGroups(common, []string{"tuic.reduce_rtt", "tuic.heartbeat", "tuic.udp_over_stream", "tuic.udp_over_stream_version"}, uriTLSLossFields("query"), uriTransportLossFields(false))
	case domain.NodeTypeMieru:
		return common
	case domain.NodeTypeSOCKS:
		return fieldGroups(common, uriTLSLossFields("none"), uriTransportLossFields(false))
	case domain.NodeTypeHTTP:
		return fieldGroups(common, []string{"headers", "path"}, uriTLSLossFields("http"), uriTransportLossFields(false))
	default:
		return nil
	}
}

func shadowrocketLossyFieldNames(nodeType domain.NodeType) []string {
	common := []string{
		"dialer.network", "multiplex", "udp_over_tcp",
		"tls.client_fingerprint", "tls.fingerprint", "tls.certificate", "tls.private_key",
		"tls.disable_sni", "tls.ech", "tls.reality",
		"transport.header_type", "transport.method", "transport.path", "transport.host", "transport.hosts",
		"transport.headers", "transport.service_name", "transport.max_early_data",
		"transport.early_data_header_name", "transport.xhttp",
	}
	switch nodeType {
	case domain.NodeTypeShadowsocks:
		return fields(common, "dialer.tfo", "dialer.udp_relay", "network", "plugin_options", "tls.enabled", "tls.server_name", "tls.insecure_skip_verify", "tls.alpn")
	case domain.NodeTypeVMess:
		return fields(common, "dialer.udp_relay", "network", "packet_encoding", "tls.enabled", "tls.server_name", "tls.insecure_skip_verify", "tls.alpn")
	case domain.NodeTypeVLESS:
		return fields(common, "dialer.tfo", "dialer.udp_relay", "network", "flow", "packet_encoding", "tls.insecure_skip_verify", "tls.alpn")
	case domain.NodeTypeTrojan:
		return fields(common, "dialer.tfo", "dialer.udp_relay", "network", "packet_encoding", "tls.alpn")
	case domain.NodeTypeHysteria:
		return fields(common, "dialer.tfo", "network", "hysteria.server_ports", "hysteria.hop_interval", "hysteria.up", "hysteria.down", "hysteria.realm", "hysteria.bbr_profile", "hysteria.cwnd", "hysteria.udp_mtu", "hysteria.quic", "tls.insecure_skip_verify")
	case domain.NodeTypeHysteria2:
		return fields(common, "dialer.tfo", "network", "hysteria.server_ports", "hysteria.hop_interval", "hysteria.up", "hysteria.down", "hysteria.up_mbps", "hysteria.down_mbps", "hysteria.auth", "hysteria.auth_str", "hysteria.realm", "hysteria.bbr_profile", "hysteria.cwnd", "hysteria.udp_mtu", "hysteria.quic", "tls.insecure_skip_verify")
	case domain.NodeTypeTUIC:
		return fields(common, "dialer.tfo", "network", "token", "tuic.congestion_control", "tuic.udp_relay_mode", "tuic.zero_rtt_handshake", "tuic.reduce_rtt", "tuic.heartbeat", "tuic.udp_over_stream", "tuic.udp_over_stream_version", "tls.insecure_skip_verify")
	case domain.NodeTypeHTTP:
		return fields(common, "dialer.tfo", "dialer.udp_relay", "network", "headers", "path", "tls.server_name", "tls.insecure_skip_verify", "tls.alpn")
	case domain.NodeTypeSOCKS:
		return fields(common, "dialer.tfo", "dialer.udp_relay", "network", "tls.server_name", "tls.alpn")
	case domain.NodeTypeWireGuard:
		return fields(common, "dialer.tfo", "network", "wireguard.pre_shared_key", "wireguard.allowed_ips", "wireguard.workers", "wireguard.peers.persistent_keepalive")
	case domain.NodeTypeSnell:
		return fields(common, "dialer.tfo", "network", "snell.client_fingerprint", "snell.shadow_tls")
	default:
		return nil
	}
}

func uriTLSLossFields(mode string) []string {
	switch mode {
	case "query":
		return []string{"tls.disable_sni"}
	case "hysteria":
		return []string{"tls.client_fingerprint", "tls.fingerprint", "tls.disable_sni", "tls.ech", "tls.reality"}
	case "hysteria2":
		return []string{"tls.alpn", "tls.client_fingerprint", "tls.disable_sni", "tls.ech", "tls.reality"}
	case "vmess":
		return []string{"tls.insecure_skip_verify", "tls.alpn", "tls.disable_sni", "tls.ech", "tls.reality"}
	case "http":
		return []string{"tls.server_name", "tls.insecure_skip_verify", "tls.alpn", "tls.client_fingerprint", "tls.fingerprint", "tls.disable_sni", "tls.ech", "tls.reality"}
	case "none":
		return []string{"tls.enabled", "tls.server_name", "tls.insecure_skip_verify", "tls.alpn", "tls.client_fingerprint", "tls.fingerprint", "tls.disable_sni", "tls.ech", "tls.reality"}
	default:
		return nil
	}
}

func uriTransportLossFields(emitsBasic bool) []string {
	fields := []string{}
	if !emitsBasic {
		fields = append(fields, "transport.type", "transport.path", "transport.host", "transport.service_name")
	}
	return append(fields, "transport.method", "transport.hosts", "transport.headers", "transport.max_early_data", "transport.early_data_header_name")
}

func legacyProtocolFieldRefs(format string, types []domain.NodeType) []FieldRef {
	fields := []FieldRef{}
	for _, nodeType := range types {
		for _, name := range legacyProtocolFieldNames(nodeType) {
			fields = append(fields, fieldRef(format, nodeType, name, FieldStatusSupported, ""))
		}
	}
	return fields
}

func legacyProtocolFieldNames(nodeType domain.NodeType) []string {
	names := []string{"name", "type", "server", "port"}
	switch nodeType {
	case domain.NodeTypeShadowsocks:
		names = append(names, "cipher", "password", "plugin", "udp_over_tcp")
	case domain.NodeTypeShadowsocksR:
		names = append(names, "cipher", "password", "shadowsocksr.protocol", "shadowsocksr.protocol_param", "shadowsocksr.obfs", "shadowsocksr.obfs_param")
	case domain.NodeTypeSnell:
		names = append(names, "password", "snell.version", "snell.obfs", "snell.obfs_host")
	case domain.NodeTypeVMess:
		names = append(names, "uuid", "cipher", "alter_id", "tls", "transport", "packet_encoding", "multiplex")
	case domain.NodeTypeVLESS:
		names = append(names, "uuid", "flow", "encryption", "tls", "reality", "transport", "packet_encoding", "multiplex")
	case domain.NodeTypeTrojan:
		names = append(names, "password", "tls", "transport", "multiplex")
	case domain.NodeTypeHysteria:
		names = append(names, "protocol", "auth", "auth_str", "obfs", "up", "down", "server_ports", "tls", "quic", "hop_interval")
	case domain.NodeTypeHysteria2:
		names = append(names, "password", "obfs", "up_mbps", "down_mbps", "server_ports", "tls", "quic", "realm", "bbr_profile")
	case domain.NodeTypeTUIC:
		names = append(names, "uuid", "password", "token", "congestion_control", "udp_relay_mode", "zero_rtt", "tls", "quic")
	case domain.NodeTypeMieru:
		names = append(names, "username", "password", "mieru.port_range", "mieru.transport", "mieru.multiplexing", "mieru.handshake_mode", "mieru.traffic_pattern")
	case domain.NodeTypeSOCKS:
		names = append(names, "username", "password", "network", "udp_over_tcp")
	case domain.NodeTypeHTTP:
		names = append(names, "username", "password", "tls", "headers", "path")
	case domain.NodeTypeWireGuard:
		names = append(names, "private_key", "address", "peers", "public_key", "pre_shared_key", "allowed_ips", "mtu", "reserved", "persistent_keepalive")
	}
	return names
}

func commonNodeFields() []string {
	return []string{"name", "type", "server", "port"}
}

func fields(base []string, extra ...string) []string {
	out := append([]string{}, base...)
	out = append(out, extra...)
	return out
}

func fieldGroups(base []string, groups ...[]string) []string {
	out := append([]string{}, base...)
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}
