package singbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Name() string {
	return "sing-box"
}

func (p *Parser) Parse(_ context.Context, in []byte) ([]domain.NodeIR, *domain.SourceInfo, error) {
	var doc map[string]any
	decoder := json.NewDecoder(bytes.NewReader(in))
	decoder.UseNumber()
	if err := decoder.Decode(&doc); err != nil {
		return nil, shared.SourceInfo("sing-box", shared.SourceRefs("sing-box")), domain.WrapError(domain.CodeParseFailed, "parse sing-box json", err)
	}
	items := []any{}
	if rawOutbounds, ok := doc["outbounds"]; ok {
		outbounds, ok := rawOutbounds.([]any)
		if !ok {
			return nil, shared.SourceInfo("sing-box", shared.SourceRefs("sing-box")), domain.NewError(domain.CodeParseFailed, "sing-box outbounds must be a list")
		}
		items = append(items, outbounds...)
	}
	if rawEndpoints, ok := doc["endpoints"]; ok {
		endpoints, ok := rawEndpoints.([]any)
		if !ok {
			return nil, shared.SourceInfo("sing-box", shared.SourceRefs("sing-box")), domain.NewError(domain.CodeParseFailed, "sing-box endpoints must be a list")
		}
		items = append(items, endpoints...)
	}
	singleOutbound := false
	if len(items) == 0 && doc["type"] != nil {
		items = []any{doc}
		singleOutbound = true
	}
	info := shared.SourceInfo("sing-box", shared.SourceRefs("sing-box"))
	nodes := make([]domain.NodeIR, 0, len(items))
	for i, item := range items {
		outbound := shared.AnyMapValue(item)
		if outbound == nil {
			err := domain.NewError(domain.CodeParseFailed, fmt.Sprintf("sing-box outbound %d must be an object", i))
			if singleOutbound {
				return nil, info, err
			}
			info.Warnings = append(info.Warnings, skippedSingBoxOutboundWarning(i, item, nil, err))
			continue
		}
		if isNonNodeSingBoxOutboundType(shared.StringValue(outbound["type"])) {
			continue
		}
		node, warnings, err := parseOutbound(outbound, i)
		if err != nil {
			if singleOutbound {
				return nil, info, err
			}
			info.Warnings = append(info.Warnings, skippedSingBoxOutboundWarning(i, item, outbound, err))
			continue
		}
		nodes = append(nodes, node)
		info.Warnings = append(info.Warnings, warnings...)
	}
	if len(nodes) == 0 {
		return nil, info, domain.NewError(domain.CodeParseFailed, "no sing-box outbounds found")
	}
	return nodes, info, nil
}

func parseOutbound(outbound map[string]any, nodeIndex int) (domain.NodeIR, []domain.Warning, error) {
	typ := strings.ToLower(shared.StringValue(outbound["type"]))
	node := domain.NodeIR{
		Name:         firstNonEmpty(shared.StringValue(outbound["tag"]), shared.StringValue(outbound["name"]), typ),
		Type:         singBoxNodeType(typ),
		Server:       shared.StringValue(outbound["server"]),
		SourceFormat: "sing-box",
		Raw:          map[string]json.RawMessage{},
	}
	if node.Type == "" {
		return node, nil, domain.NewError(domain.CodeParseFailed, "unsupported sing-box outbound type")
	}
	if port, err := shared.Uint16Value(outbound["server_port"]); err == nil {
		node.Port = port
	}
	node.Network = shared.StringValue(outbound["network"])
	node.Username = shared.StringValue(outbound["username"])
	node.Password = shared.StringValue(outbound["password"])
	node.UUID = shared.StringValue(outbound["uuid"])
	node.Cipher = firstNonEmpty(shared.StringValue(outbound["method"]), shared.StringValue(outbound["security"]))
	node.Flow = shared.StringValue(outbound["flow"])
	node.PacketEncoding = shared.StringValue(outbound["packet_encoding"])
	node.Plugin = shared.StringValue(outbound["plugin"])
	if opts := shared.StringValue(outbound["plugin_opts"]); opts != "" {
		node.PluginOptions = map[string]any{"raw": opts}
	}
	node.Headers = shared.StringMapValue(outbound["headers"])
	node.Path = shared.StringValue(outbound["path"])
	parseSingBoxDialer(&node, outbound)
	parseSingBoxTLS(&node, outbound)
	parseSingBoxTransport(&node, outbound)
	parseSingBoxMux(&node, outbound)
	parseSingBoxUDPOverTCP(&node, outbound)
	switch node.Type {
	case domain.NodeTypeVMess:
		if alterID, err := shared.IntValue(outbound["alter_id"]); err == nil {
			node.AlterID = alterID
		}
	case domain.NodeTypeHysteria:
		parseSingBoxHysteria(&node, outbound)
	case domain.NodeTypeHysteria2:
		parseSingBoxHysteria2(&node, outbound)
	case domain.NodeTypeTUIC:
		parseSingBoxTUIC(&node, outbound)
	case domain.NodeTypeAnyTLS:
		parseSingBoxAnyTLS(&node, outbound)
	case domain.NodeTypeWireGuard:
		parseSingBoxWireGuard(&node, outbound)
	}
	known := singBoxKnownFields(node.Type)
	shared.AddUnknownRaw(node.Raw, "sing-box.", outbound, known)
	warnings := unknownWarnings(node, node.Raw, "sing-box", nodeIndex, singBoxWarningNodeContext(node, outbound))
	if len(node.Raw) == 0 {
		node.Raw = nil
	}
	return node, warnings, nil
}

func isNonNodeSingBoxOutboundType(typ string) bool {
	switch strings.ToLower(typ) {
	case "selector", "direct":
		return true
	default:
		return false
	}
}

func singBoxNodeType(typ string) domain.NodeType {
	switch typ {
	case "shadowsocks":
		return domain.NodeTypeShadowsocks
	case "vmess":
		return domain.NodeTypeVMess
	case "vless":
		return domain.NodeTypeVLESS
	case "trojan":
		return domain.NodeTypeTrojan
	case "hysteria":
		return domain.NodeTypeHysteria
	case "hysteria2":
		return domain.NodeTypeHysteria2
	case "tuic":
		return domain.NodeTypeTUIC
	case "anytls":
		return domain.NodeTypeAnyTLS
	case "socks":
		return domain.NodeTypeSOCKS
	case "http":
		return domain.NodeTypeHTTP
	case "wireguard":
		return domain.NodeTypeWireGuard
	default:
		return ""
	}
}

func parseSingBoxTLS(node *domain.NodeIR, outbound map[string]any) {
	tlsObj := shared.AnyMapValue(outbound["tls"])
	if tlsObj == nil {
		return
	}
	node.TLS = &domain.TLSOptions{
		Enabled:            shared.BoolValue(tlsObj["enabled"]),
		ServerName:         shared.StringValue(tlsObj["server_name"]),
		InsecureSkipVerify: shared.BoolValue(tlsObj["insecure"]),
		ALPN:               shared.StringSliceValue(tlsObj["alpn"]),
		DisableSNI:         shared.BoolValue(tlsObj["disable_sni"]),
	}
	if utls := shared.AnyMapValue(tlsObj["utls"]); utls != nil {
		node.TLS.ClientFingerprint = shared.StringValue(utls["fingerprint"])
	}
	if ech := shared.AnyMapValue(tlsObj["ech"]); ech != nil {
		node.TLS.ECH = &domain.ECHOptions{
			Enabled:         shared.BoolValue(ech["enabled"]),
			Config:          shared.StringSliceValue(ech["config"]),
			QueryServerName: shared.StringValue(ech["query_server_name"]),
		}
	}
	if reality := shared.AnyMapValue(tlsObj["reality"]); reality != nil {
		node.TLS.Enabled = true
		node.TLS.Reality = &domain.RealityOptions{
			Enabled:   shared.BoolValue(reality["enabled"]),
			PublicKey: shared.StringValue(reality["public_key"]),
			ShortID:   shared.StringValue(reality["short_id"]),
		}
		if node.TLS.Reality.PublicKey != "" && !node.TLS.Reality.Enabled {
			node.TLS.Reality.Enabled = true
		}
	}
}

func parseSingBoxAnyTLS(node *domain.NodeIR, outbound map[string]any) {
	node.AnyTLS = &domain.AnyTLSOptions{
		IdleSessionCheckInterval: shared.StringValue(outbound["idle_session_check_interval"]),
		IdleSessionTimeout:       shared.StringValue(outbound["idle_session_timeout"]),
		MinIdleSession:           intValueZero(outbound["min_idle_session"]),
	}
}

func parseSingBoxDialer(node *domain.NodeIR, outbound map[string]any) {
	if shared.BoolValue(outbound["tcp_fast_open"]) {
		node.Dialer = &domain.DialerOptions{TFO: true}
	}
}

func parseSingBoxTransport(node *domain.NodeIR, outbound map[string]any) {
	transport := shared.AnyMapValue(outbound["transport"])
	if transport == nil {
		return
	}
	node.Transport = &domain.TransportOptions{
		Type: shared.StringValue(transport["type"]),
	}
	switch node.Transport.Type {
	case "websocket", "ws":
		node.Transport.Type = "websocket"
		node.Transport.Path = shared.StringValue(transport["path"])
		node.Transport.Headers = shared.StringMapValue(transport["headers"])
		if host := node.Transport.Headers["Host"]; host != "" {
			node.Transport.Host = host
		}
		if maxEarly, err := shared.IntValue(transport["max_early_data"]); err == nil {
			node.Transport.MaxEarlyData = maxEarly
		}
		node.Transport.EarlyDataHeaderName = shared.StringValue(transport["early_data_header_name"])
	case "http":
		node.Transport.Hosts = shared.StringSliceValue(transport["host"])
		if len(node.Transport.Hosts) > 0 {
			node.Transport.Host = node.Transport.Hosts[0]
		}
		node.Transport.Path = shared.StringValue(transport["path"])
		node.Transport.Method = shared.StringValue(transport["method"])
		node.Transport.Headers = shared.StringMapValue(transport["headers"])
	case "grpc":
		node.Transport.ServiceName = shared.StringValue(transport["service_name"])
	case "httpupgrade":
		node.Transport.Type = "httpupgrade"
		node.Transport.Host = shared.StringValue(transport["host"])
		node.Transport.Path = shared.StringValue(transport["path"])
		node.Transport.Headers = shared.StringMapValue(transport["headers"])
	case "quic":
		// type is enough for the stable IR at this point.
	}
}

func parseSingBoxMux(node *domain.NodeIR, outbound map[string]any) {
	mux := shared.AnyMapValue(outbound["multiplex"])
	if mux == nil {
		return
	}
	node.Multiplex = &domain.MultiplexOptions{
		Enabled:        shared.BoolValue(mux["enabled"]),
		Protocol:       shared.StringValue(mux["protocol"]),
		MaxConnections: intValueZero(mux["max_connections"]),
		MinStreams:     intValueZero(mux["min_streams"]),
		MaxStreams:     intValueZero(mux["max_streams"]),
		Padding:        shared.BoolValue(mux["padding"]),
	}
}

func parseSingBoxUDPOverTCP(node *domain.NodeIR, outbound map[string]any) {
	value, ok := outbound["udp_over_tcp"]
	if !ok {
		return
	}
	switch t := value.(type) {
	case bool:
		node.UDPOverTCP = &domain.UDPOverTCPOptions{Enabled: t}
	case map[string]any:
		node.UDPOverTCP = &domain.UDPOverTCPOptions{
			Enabled: shared.BoolValue(t["enabled"]),
			Version: intValueZero(t["version"]),
		}
	}
}

func parseSingBoxHysteria(node *domain.NodeIR, outbound map[string]any) {
	node.Hysteria = &domain.HysteriaOptions{
		ServerPorts:  shared.StringSliceValue(outbound["server_ports"]),
		HopInterval:  shared.StringValue(outbound["hop_interval"]),
		Up:           shared.StringValue(outbound["up"]),
		Down:         shared.StringValue(outbound["down"]),
		UpMbps:       intValueZero(outbound["up_mbps"]),
		DownMbps:     intValueZero(outbound["down_mbps"]),
		ObfsPassword: shared.StringValue(outbound["obfs"]),
		AuthString:   shared.StringValue(outbound["auth_str"]),
		Auth:         shared.StringValue(outbound["auth"]),
	}
}

func parseSingBoxHysteria2(node *domain.NodeIR, outbound map[string]any) {
	node.Hysteria = &domain.HysteriaOptions{
		ServerPorts: shared.StringSliceValue(outbound["server_ports"]),
		HopInterval: shared.StringValue(outbound["hop_interval"]),
		UpMbps:      intValueZero(outbound["up_mbps"]),
		DownMbps:    intValueZero(outbound["down_mbps"]),
	}
	if obfs := shared.AnyMapValue(outbound["obfs"]); obfs != nil {
		node.Hysteria.Obfs = shared.StringValue(obfs["type"])
		node.Hysteria.ObfsPassword = shared.StringValue(obfs["password"])
	}
}

func parseSingBoxTUIC(node *domain.NodeIR, outbound map[string]any) {
	node.TUIC = &domain.TUICOptions{
		CongestionControl: shared.StringValue(outbound["congestion_control"]),
		UDPRelayMode:      shared.StringValue(outbound["udp_relay_mode"]),
		ZeroRTTHandshake:  shared.BoolValue(outbound["zero_rtt_handshake"]),
		Heartbeat:         shared.StringValue(outbound["heartbeat"]),
		UDPOverStream:     shared.BoolValue(outbound["udp_over_stream"]),
	}
}

func parseSingBoxWireGuard(node *domain.NodeIR, outbound map[string]any) {
	node.WireGuard = &domain.WireGuardOptions{
		PrivateKey: shared.StringValue(outbound["private_key"]),
		Address:    shared.StringSliceValue(outbound["address"]),
		MTU:        intValueZero(outbound["mtu"]),
		Workers:    intValueZero(outbound["workers"]),
	}
	peerItems, _ := outbound["peers"].([]any)
	for _, item := range peerItems {
		peerMap := shared.AnyMapValue(item)
		if peerMap == nil {
			continue
		}
		port, _ := shared.Uint16Value(peerMap["port"])
		peer := domain.WireGuardPeer{
			Server:              firstNonEmpty(shared.StringValue(peerMap["address"]), shared.StringValue(peerMap["server"])),
			Port:                port,
			PublicKey:           shared.StringValue(peerMap["public_key"]),
			PreSharedKey:        shared.StringValue(peerMap["pre_shared_key"]),
			AllowedIPs:          shared.StringSliceValue(peerMap["allowed_ips"]),
			Reserved:            uint8SliceValue(peerMap["reserved"]),
			PersistentKeepalive: intValueZero(peerMap["persistent_keepalive_interval"]),
		}
		node.WireGuard.Peers = append(node.WireGuard.Peers, peer)
	}
	if node.Server == "" && len(node.WireGuard.Peers) > 0 {
		node.Server = node.WireGuard.Peers[0].Server
		node.Port = node.WireGuard.Peers[0].Port
	}
}

func singBoxKnownFields(nodeType domain.NodeType) map[string]bool {
	if nodeType == domain.NodeTypeWireGuard {
		return shared.KnownFields(
			"type", "tag", "system", "name", "mtu",
			"address", "private_key", "listen_port", "peers",
			"udp_timeout", "workers", "tcp_fast_open",
		)
	}
	common := shared.KnownFields(
		"type", "tag", "server", "server_port", "network",
		"username", "password", "uuid", "method", "security",
		"flow", "packet_encoding", "plugin", "plugin_opts", "tls",
		"transport", "multiplex", "udp_over_tcp", "headers", "path",
		"tcp_fast_open",
	)
	add := func(keys ...string) {
		shared.AddKnownFields(common, keys...)
	}
	switch nodeType {
	case domain.NodeTypeVMess:
		add("alter_id", "global_padding", "authenticated_length")
	case domain.NodeTypeHysteria:
		add("server_ports", "hop_interval", "up", "down", "up_mbps", "down_mbps", "obfs", "auth", "auth_str",
			"recv_window_conn", "recv_window", "disable_mtu_discovery")
	case domain.NodeTypeHysteria2:
		add("server_ports", "hop_interval", "up_mbps", "down_mbps", "obfs", "brutal_debug")
	case domain.NodeTypeTUIC:
		add("congestion_control", "udp_relay_mode", "udp_over_stream", "zero_rtt_handshake", "heartbeat",
			"initial_packet_size", "disable_path_mtu_discovery")
	case domain.NodeTypeAnyTLS:
		add("idle_session_check_interval", "idle_session_timeout", "min_idle_session")
	}
	return common
}

func unknownWarnings(node domain.NodeIR, raw map[string]json.RawMessage, source string, nodeIndex int, nodeContext domain.WarningNodeContext) []domain.Warning {
	index := nodeIndex
	return shared.ParseUnknownWarningsWithContext(node, raw, source, &index, &nodeContext)
}

func skippedSingBoxOutboundWarning(nodeIndex int, item any, outbound map[string]any, err error) domain.Warning {
	index := nodeIndex
	context := skippedSingBoxOutboundContext(item, outbound)
	return domain.Warning{
		Code:        "parse_outbound_skipped",
		Message:     err.Error(),
		Source:      "sing-box",
		NodeIndex:   &index,
		NodeContext: &context,
	}
}

func skippedSingBoxOutboundContext(item any, outbound map[string]any) domain.WarningNodeContext {
	if outbound == nil {
		return domain.WarningNodeContext{
			Format: "sing-box",
			Raw:    map[string]any{"value": item},
		}
	}
	port, _ := shared.Uint16Value(outbound["server_port"])
	return domain.WarningNodeContext{
		Format: "sing-box",
		Name:   firstNonEmpty(shared.StringValue(outbound["tag"]), shared.StringValue(outbound["name"])),
		Type:   singBoxNodeType(strings.ToLower(shared.StringValue(outbound["type"]))),
		Server: shared.StringValue(outbound["server"]),
		Port:   port,
		Raw:    outbound,
	}
}

func singBoxWarningNodeContext(node domain.NodeIR, outbound map[string]any) domain.WarningNodeContext {
	return domain.WarningNodeContext{
		Format: "sing-box",
		Name:   node.Name,
		Type:   node.Type,
		Server: node.Server,
		Port:   node.Port,
		Raw:    outbound,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func intValueZero(v any) int {
	n, err := shared.IntValue(v)
	if err != nil {
		return 0
	}
	return n
}

func uint8SliceValue(v any) []uint8 {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]uint8, 0, len(items))
	for _, item := range items {
		n, err := shared.IntValue(item)
		if err == nil && n >= 0 && n <= 255 {
			out = append(out, uint8(n))
		}
	}
	return out
}
