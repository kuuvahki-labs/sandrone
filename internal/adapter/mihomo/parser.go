package mihomo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Name() string {
	return "mihomo"
}

func (p *Parser) Parse(_ context.Context, in []byte) ([]domain.NodeIR, *domain.SourceInfo, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(in, &doc); err != nil {
		return nil, shared.SourceInfo("mihomo", shared.SourceRefs("mihomo")), domain.WrapError(domain.CodeParseFailed, "parse mihomo yaml", err)
	}
	var proxies []any
	singleProxy := false
	switch v := doc["proxies"].(type) {
	case []any:
		proxies = v
	case nil:
		if doc["type"] != nil {
			proxies = []any{doc}
			singleProxy = true
		}
	default:
		return nil, shared.SourceInfo("mihomo", shared.SourceRefs("mihomo")), domain.NewError(domain.CodeParseFailed, "mihomo proxies must be a list")
	}
	nodes := make([]domain.NodeIR, 0, len(proxies))
	info := shared.SourceInfo("mihomo", shared.SourceRefs("mihomo"))
	for i, item := range proxies {
		proxy := shared.AnyMapValue(item)
		if proxy == nil {
			err := domain.NewError(domain.CodeParseFailed, fmt.Sprintf("mihomo proxy %d must be an object", i))
			if singleProxy {
				return nil, info, err
			}
			info.Warnings = append(info.Warnings, skippedMihomoProxyWarning(i, item, nil, err))
			continue
		}
		node, warnings, err := parseMihomoProxy(proxy, i)
		if err != nil {
			if singleProxy {
				return nil, info, err
			}
			info.Warnings = append(info.Warnings, skippedMihomoProxyWarning(i, item, proxy, err))
			continue
		}
		nodes = append(nodes, node)
		info.Warnings = append(info.Warnings, warnings...)
	}
	if len(nodes) == 0 {
		return nil, info, domain.NewError(domain.CodeParseFailed, "no mihomo proxies found")
	}
	return nodes, info, nil
}

func parseMihomoProxy(proxy map[string]any, nodeIndex int) (domain.NodeIR, []domain.Warning, error) {
	typ := strings.ToLower(shared.StringValue(proxy["type"]))
	node := domain.NodeIR{
		Name:         shared.StringValue(proxy["name"]),
		Type:         mihomoNodeType(typ),
		Server:       shared.StringValue(proxy["server"]),
		SourceFormat: "mihomo",
		Raw:          map[string]json.RawMessage{},
	}
	if node.Type == "" {
		return node, nil, domain.NewError(domain.CodeParseFailed, "unsupported mihomo proxy type")
	}
	if node.Name == "" {
		node.Name = firstNonEmpty(node.Server, typ)
	}
	if port, err := shared.Uint16Value(proxy["port"]); err == nil {
		node.Port = port
	}
	node.Username = firstNonEmpty(shared.StringValue(proxy["username"]), shared.StringValue(proxy["user"]))
	node.Password = shared.StringValue(proxy["password"])
	node.UUID = shared.StringValue(proxy["uuid"])
	node.Cipher = firstNonEmpty(shared.StringValue(proxy["cipher"]), shared.StringValue(proxy["method"]))
	node.Flow = shared.StringValue(proxy["flow"])
	node.Encryption = shared.StringValue(proxy["encryption"])
	node.PacketEncoding = shared.StringValue(proxy["packet-encoding"])
	if node.PacketEncoding == "" && (node.Type == domain.NodeTypeVMess || node.Type == domain.NodeTypeVLESS) {
		switch {
		case shared.BoolValue(proxy["xudp"]):
			node.PacketEncoding = "xudp"
		case shared.BoolValue(proxy["packet-addr"]):
			node.PacketEncoding = "packetaddr"
		}
	}
	node.Plugin = shared.StringValue(proxy["plugin"])
	node.PluginOptions = shared.AnyMapValue(proxy["plugin-opts"])
	node.Headers = shared.StringMapValue(proxy["headers"])
	parseMihomoDialer(&node, proxy)
	parseMihomoTLS(&node, proxy)
	parseMihomoTransport(&node, proxy)
	parseMihomoMux(&node, proxy)
	parseMihomoUDPOverTCP(&node, proxy)
	switch node.Type {
	case domain.NodeTypeShadowsocksR:
		parseMihomoShadowsocksR(&node, proxy)
	case domain.NodeTypeSnell:
		parseMihomoSnell(&node, proxy)
	case domain.NodeTypeAnyTLS:
		parseMihomoAnyTLS(&node, proxy)
	case domain.NodeTypeVMess:
		if alterID, err := shared.IntValue(proxy["alterId"]); err == nil {
			node.AlterID = alterID
		}
	case domain.NodeTypeHysteria:
		parseMihomoHysteria(&node, proxy)
	case domain.NodeTypeHysteria2:
		parseMihomoHysteria2(&node, proxy)
	case domain.NodeTypeTUIC:
		parseMihomoTUIC(&node, proxy)
	case domain.NodeTypeMieru:
		parseMihomoMieru(&node, proxy)
	case domain.NodeTypeWireGuard:
		parseMihomoWireGuard(&node, proxy)
	}
	known := mihomoKnownFields(node.Type)
	if node.Type == domain.NodeTypeHysteria2 && isExplicitTrue(proxy["udp"]) {
		known["udp"] = true
	}
	shared.AddUnknownRaw(node.Raw, "mihomo.", proxy, known)
	warnings := unknownWarnings(node, node.Raw, "mihomo", nodeIndex, mihomoWarningNodeContext(node, proxy))
	if len(node.Raw) == 0 {
		node.Raw = nil
	}
	return node, warnings, nil
}

func mihomoNodeType(typ string) domain.NodeType {
	switch typ {
	case "ss":
		return domain.NodeTypeShadowsocks
	case "ssr":
		return domain.NodeTypeShadowsocksR
	case "snell":
		return domain.NodeTypeSnell
	case "anytls":
		return domain.NodeTypeAnyTLS
	case "vmess":
		return domain.NodeTypeVMess
	case "vless":
		return domain.NodeTypeVLESS
	case "trojan":
		return domain.NodeTypeTrojan
	case "hysteria":
		return domain.NodeTypeHysteria
	case "hysteria2", "hy2":
		return domain.NodeTypeHysteria2
	case "tuic":
		return domain.NodeTypeTUIC
	case "mieru":
		return domain.NodeTypeMieru
	case "socks5", "socks":
		return domain.NodeTypeSOCKS
	case "http":
		return domain.NodeTypeHTTP
	case "wireguard":
		return domain.NodeTypeWireGuard
	default:
		return ""
	}
}

func parseMihomoTLS(node *domain.NodeIR, proxy map[string]any) {
	tlsEnabled := shared.BoolValue(proxy["tls"])
	hasTLSFields := tlsEnabled ||
		shared.StringValue(proxy["sni"]) != "" ||
		shared.StringValue(proxy["servername"]) != "" ||
		shared.BoolValue(proxy["skip-cert-verify"]) ||
		len(shared.StringSliceValue(proxy["alpn"])) > 0 ||
		shared.StringValue(proxy["client-fingerprint"]) != "" ||
		shared.StringValue(proxy["fingerprint"]) != "" ||
		shared.AnyMapValue(proxy["reality-opts"]) != nil ||
		shared.AnyMapValue(proxy["ech-opts"]) != nil
	if !hasTLSFields {
		return
	}
	node.TLS = &domain.TLSOptions{
		Enabled:            tlsEnabled || node.Type == domain.NodeTypeTrojan || node.Type == domain.NodeTypeHysteria || node.Type == domain.NodeTypeHysteria2 || node.Type == domain.NodeTypeTUIC || node.Type == domain.NodeTypeAnyTLS,
		ServerName:         firstNonEmpty(shared.StringValue(proxy["servername"]), shared.StringValue(proxy["sni"])),
		InsecureSkipVerify: shared.BoolValue(proxy["skip-cert-verify"]),
		ALPN:               shared.StringSliceValue(proxy["alpn"]),
		ClientFingerprint:  shared.StringValue(proxy["client-fingerprint"]),
		Fingerprint:        shared.StringValue(proxy["fingerprint"]),
	}
	if ech := shared.AnyMapValue(proxy["ech-opts"]); ech != nil {
		node.TLS.ECH = &domain.ECHOptions{
			Enabled:         shared.BoolValue(ech["enable"]),
			QueryServerName: shared.StringValue(ech["query-server-name"]),
		}
		if config := shared.StringValue(ech["config"]); config != "" {
			node.TLS.ECH.Config = []string{config}
		}
	}
	if reality := shared.AnyMapValue(proxy["reality-opts"]); reality != nil {
		node.TLS.Enabled = true
		node.TLS.Reality = &domain.RealityOptions{
			Enabled:   true,
			PublicKey: shared.StringValue(reality["public-key"]),
			ShortID:   shared.StringValue(reality["short-id"]),
		}
	}
}

func parseMihomoDialer(node *domain.NodeIR, proxy map[string]any) {
	if _, ok := proxy["udp"]; ok && mihomoSupportsUDPRelay(node.Type) {
		if node.Dialer == nil {
			node.Dialer = &domain.DialerOptions{}
		}
		udp := shared.BoolValue(proxy["udp"])
		node.Dialer.UDPRelay = &udp
	}
	fastOpen := node.Type == domain.NodeTypeHysteria || node.Type == domain.NodeTypeTUIC
	if shared.BoolValue(proxy["tfo"]) || (fastOpen && shared.BoolValue(proxy["fast-open"])) {
		if node.Dialer == nil {
			node.Dialer = &domain.DialerOptions{}
		}
		node.Dialer.TFO = true
	}
}

func parseMihomoTransport(node *domain.NodeIR, proxy map[string]any) {
	network := strings.ToLower(shared.StringValue(proxy["network"]))
	if network == "" {
		return
	}
	if !mihomoNodeUsesV2RayTransport(node.Type) {
		shared.AddRaw(node.Raw, "mihomo.network", proxy["network"])
		return
	}
	if !mihomoSupportsSourceTransport(node.Type, network) {
		shared.AddRaw(node.Raw, "mihomo.network", proxy["network"])
		node.Transport = &domain.TransportOptions{Type: "tcp"}
		return
	}
	node.Transport = &domain.TransportOptions{Type: network}
	switch network {
	case "ws":
		node.Transport.Type = "websocket"
		opts := shared.AnyMapValue(proxy["ws-opts"])
		node.Transport.Path = shared.StringValue(opts["path"])
		node.Transport.Headers = shared.StringMapValue(opts["headers"])
		if host := node.Transport.Headers["Host"]; host != "" {
			node.Transport.Host = host
		}
		if maxEarly, err := shared.IntValue(opts["max-early-data"]); err == nil {
			node.Transport.MaxEarlyData = maxEarly
		}
		node.Transport.EarlyDataHeaderName = shared.StringValue(opts["early-data-header-name"])
		node.Transport.V2RayHTTPUpgrade = shared.BoolValue(opts["v2ray-http-upgrade"])
		node.Transport.V2RayHTTPUpgradeFastOpen = shared.BoolValue(opts["v2ray-http-upgrade-fast-open"])
		preserveNestedMihomoRaw(node, "ws-opts", opts, map[string]bool{
			"path": true, "headers": true, "max-early-data": true, "early-data-header-name": true,
			"v2ray-http-upgrade": true, "v2ray-http-upgrade-fast-open": true,
		})
	case "grpc":
		opts := shared.AnyMapValue(proxy["grpc-opts"])
		node.Transport.ServiceName = shared.StringValue(opts["grpc-service-name"])
		known := map[string]bool{
			"grpc-service-name": true, "max-connections": true, "min-streams": true, "max-streams": true,
		}
		if strings.EqualFold(strings.TrimSpace(shared.StringValue(opts["grpc-mode"])), "gun") {
			known["grpc-mode"] = true
		}
		preserveNestedMihomoRaw(node, "grpc-opts", opts, known)
	case "h2":
		opts := shared.AnyMapValue(proxy["h2-opts"])
		node.Transport.Type = "http"
		node.Transport.Hosts = shared.StringSliceValue(opts["host"])
		if len(node.Transport.Hosts) > 0 {
			node.Transport.Host = node.Transport.Hosts[0]
		}
		node.Transport.Path = shared.StringValue(opts["path"])
	case "http":
		opts := shared.AnyMapValue(proxy["http-opts"])
		node.Transport.Type = "tcp"
		node.Transport.HeaderType = "http"
		node.Transport.Method = shared.StringValue(opts["method"])
		paths := shared.StringSliceValue(opts["path"])
		if len(paths) > 0 {
			node.Transport.Path = paths[0]
		}
		if headers := shared.AnyMapValue(opts["headers"]); headers != nil {
			node.Transport.Headers = mapStringListToString(headers)
			if host := node.Transport.Headers["Host"]; host != "" {
				node.Transport.Host = host
			}
		}
	case "xhttp":
		opts := shared.AnyMapValue(proxy["xhttp-opts"])
		node.Transport.Path = shared.StringValue(opts["path"])
		node.Transport.Host = shared.StringValue(opts["host"])
		node.Transport.Headers = shared.StringMapValue(opts["headers"])
		node.Transport.XHTTP = &domain.XHTTPTransportOptions{
			Mode:             shared.StringValue(opts["mode"]),
			ReuseSettings:    parseMihomoXHTTPReuseSettings(shared.AnyMapValue(opts["reuse-settings"])),
			DownloadSettings: parseMihomoXHTTPDownloadSettings(shared.AnyMapValue(opts["download-settings"])),
		}
		known := mihomoKnownXHTTPOptions()
		shared.AddKnownFields(known, "path", "host", "headers")
		preserveNestedMihomoRaw(node, "xhttp-opts", opts, known)
	}
}

func mihomoNodeUsesV2RayTransport(nodeType domain.NodeType) bool {
	switch nodeType {
	case domain.NodeTypeVMess, domain.NodeTypeVLESS, domain.NodeTypeTrojan:
		return true
	default:
		return false
	}
}

func mihomoSupportsSourceTransport(nodeType domain.NodeType, transportType string) bool {
	switch nodeType {
	case domain.NodeTypeVMess:
		switch transportType {
		case "tcp", "ws", "grpc", "h2", "http", "mkcp", "mekya":
			return true
		}
	case domain.NodeTypeVLESS:
		switch transportType {
		case "tcp", "ws", "grpc", "h2", "http", "xhttp":
			return true
		}
	case domain.NodeTypeTrojan:
		switch transportType {
		case "tcp", "ws", "grpc":
			return true
		}
	}
	return false
}

func parseMihomoMux(node *domain.NodeIR, proxy map[string]any) {
	if shared.BoolValue(proxy["mux"]) {
		node.Multiplex = &domain.MultiplexOptions{Enabled: true}
	}
	if node.Transport != nil && node.Transport.Type == "grpc" {
		opts := shared.AnyMapValue(proxy["grpc-opts"])
		if max, err := shared.IntValue(opts["max-connections"]); err == nil && max > 0 {
			if node.Multiplex == nil {
				node.Multiplex = &domain.MultiplexOptions{Enabled: true}
			}
			node.Multiplex.MaxConnections = max
		}
		if min, err := shared.IntValue(opts["min-streams"]); err == nil && min > 0 {
			if node.Multiplex == nil {
				node.Multiplex = &domain.MultiplexOptions{Enabled: true}
			}
			node.Multiplex.MinStreams = min
		}
		if max, err := shared.IntValue(opts["max-streams"]); err == nil && max > 0 {
			if node.Multiplex == nil {
				node.Multiplex = &domain.MultiplexOptions{Enabled: true}
			}
			node.Multiplex.MaxStreams = max
		}
	}
}

func parseMihomoUDPOverTCP(node *domain.NodeIR, proxy map[string]any) {
	enabled := shared.BoolValue(proxy["udp-over-tcp"]) || shared.BoolValue(proxy["udp-over-stream"])
	if !enabled {
		return
	}
	node.UDPOverTCP = &domain.UDPOverTCPOptions{Enabled: true}
	if version, err := shared.IntValue(firstNonEmpty(shared.StringValue(proxy["udp-over-tcp-version"]), shared.StringValue(proxy["udp-over-stream-version"]))); err == nil {
		node.UDPOverTCP.Version = version
	}
}

func parseMihomoShadowsocksR(node *domain.NodeIR, proxy map[string]any) {
	node.ShadowsocksR = &domain.ShadowsocksROptions{
		Protocol:      shared.StringValue(proxy["protocol"]),
		ProtocolParam: shared.StringValue(proxy["protocol-param"]),
		Obfs:          shared.StringValue(proxy["obfs"]),
		ObfsParam:     shared.StringValue(proxy["obfs-param"]),
	}
}

func parseMihomoSnell(node *domain.NodeIR, proxy map[string]any) {
	node.Password = firstNonEmpty(shared.StringValue(proxy["psk"]), node.Password)
	node.Snell = &domain.SnellOptions{
		Version:           intValueZero(proxy["version"]),
		ClientFingerprint: shared.StringValue(proxy["client-fingerprint"]),
	}
	if _, ok := proxy["reuse"]; ok {
		reuse := shared.BoolValue(proxy["reuse"])
		node.Snell.Reuse = &reuse
	}
	if opts := shared.AnyMapValue(proxy["obfs-opts"]); opts != nil {
		mode := shared.StringValue(opts["mode"])
		if mode == "shadow-tls" {
			node.Snell.ShadowTLS = &domain.ShadowTLSOptions{
				Password:           shared.StringValue(opts["password"]),
				Host:               shared.StringValue(opts["host"]),
				Version:            intValueZero(opts["version"]),
				ALPN:               shared.StringSliceValue(opts["alpn"]),
				Fingerprint:        shared.StringValue(opts["fingerprint"]),
				Certificate:        shared.StringValue(opts["certificate"]),
				PrivateKey:         shared.StringValue(opts["private-key"]),
				InsecureSkipVerify: shared.BoolValue(opts["skip-cert-verify"]),
			}
		} else {
			node.Snell.Obfs = mode
			node.Snell.ObfsHost = shared.StringValue(opts["host"])
		}
		preserveNestedMihomoRaw(node, "obfs-opts", opts, map[string]bool{
			"mode": true, "password": true, "host": true, "version": true, "alpn": true,
			"fingerprint": true, "certificate": true, "private-key": true, "skip-cert-verify": true,
		})
	}
}

func parseMihomoAnyTLS(node *domain.NodeIR, proxy map[string]any) {
	node.AnyTLS = &domain.AnyTLSOptions{
		IdleSessionCheckInterval: mihomoDurationString(proxy["idle-session-check-interval"]),
		IdleSessionTimeout:       mihomoDurationString(proxy["idle-session-timeout"]),
		MinIdleSession:           intValueZero(proxy["min-idle-session"]),
	}
}

func mihomoDurationString(value any) string {
	if seconds, err := shared.IntValue(value); err == nil && seconds > 0 {
		return (time.Duration(seconds) * time.Second).String()
	}
	text := strings.TrimSpace(shared.StringValue(value))
	if duration, err := time.ParseDuration(text); err == nil && duration > 0 {
		return duration.String()
	}
	return text
}

func parseMihomoHysteria(node *domain.NodeIR, proxy map[string]any) {
	node.Hysteria = &domain.HysteriaOptions{
		ServerPorts:  firstStringSlice(proxy["ports"], proxy["server-ports"]),
		Protocol:     firstNonEmpty(shared.StringValue(proxy["protocol"]), shared.StringValue(proxy["obfs-protocol"])),
		Auth:         shared.StringValue(proxy["auth"]),
		AuthString:   shared.StringValue(proxy["auth-str"]),
		ObfsPassword: shared.StringValue(proxy["obfs"]),
		HopInterval:  intString(proxy["hop-interval"]),
	}
	applyMihomoHysteriaRate(node, proxy["up"], proxy["up-speed"], "up", "up-speed", &node.Hysteria.Up, &node.Hysteria.UpMbps)
	applyMihomoHysteriaRate(node, proxy["down"], proxy["down-speed"], "down", "down-speed", &node.Hysteria.Down, &node.Hysteria.DownMbps)
}

func parseMihomoHysteria2(node *domain.NodeIR, proxy map[string]any) {
	node.Hysteria = &domain.HysteriaOptions{
		ServerPorts:  firstStringSlice(proxy["ports"], proxy["server-ports"]),
		HopInterval:  shared.StringValue(proxy["hop-interval"]),
		Up:           shared.StringValue(proxy["up"]),
		Down:         shared.StringValue(proxy["down"]),
		Obfs:         shared.StringValue(proxy["obfs"]),
		ObfsPassword: shared.StringValue(proxy["obfs-password"]),
		BBRProfile:   shared.StringValue(proxy["bbr-profile"]),
		UDPMTU:       intValueZero(proxy["udp-mtu"]),
		CWND:         intValueZero(proxy["cwnd"]),
	}
	realm := shared.AnyMapValue(proxy["realm-opts"])
	if realm != nil {
		node.Hysteria.Realm = &domain.HysteriaRealmOptions{
			Enabled:     shared.BoolValue(realm["enable"]),
			ServerURL:   shared.StringValue(realm["server-url"]),
			Token:       shared.StringValue(realm["token"]),
			RealmID:     shared.StringValue(realm["realm-id"]),
			STUNServers: shared.StringSliceValue(realm["stun-servers"]),
		}
		preserveNestedMihomoRaw(node, "realm-opts", realm, map[string]bool{
			"enable": true, "server-url": true, "token": true, "realm-id": true, "stun-servers": true,
		})
	}
}

func parseMihomoTUIC(node *domain.NodeIR, proxy map[string]any) {
	node.Token = shared.StringValue(proxy["token"])
	node.TUIC = &domain.TUICOptions{
		CongestionControl:    shared.StringValue(proxy["congestion-controller"]),
		UDPRelayMode:         shared.StringValue(proxy["udp-relay-mode"]),
		ReduceRTT:            shared.BoolValue(proxy["reduce-rtt"]),
		Heartbeat:            intString(proxy["heartbeat-interval"]),
		UDPOverStream:        shared.BoolValue(proxy["udp-over-stream"]),
		UDPOverStreamVersion: intValueZero(proxy["udp-over-stream-version"]),
	}
}

func parseMihomoMieru(node *domain.NodeIR, proxy map[string]any) {
	node.Mieru = &domain.MieruOptions{
		PortRange:      shared.StringValue(proxy["port-range"]),
		Transport:      shared.StringValue(proxy["transport"]),
		Multiplexing:   shared.StringValue(proxy["multiplexing"]),
		HandshakeMode:  shared.StringValue(proxy["handshake-mode"]),
		TrafficPattern: shared.StringValue(proxy["traffic-pattern"]),
	}
}

func parseMihomoWireGuard(node *domain.NodeIR, proxy map[string]any) {
	node.WireGuard = &domain.WireGuardOptions{
		PrivateKey:          shared.StringValue(proxy["private-key"]),
		IP:                  shared.StringValue(proxy["ip"]),
		IPv6:                shared.StringValue(proxy["ipv6"]),
		MTU:                 intValueZero(proxy["mtu"]),
		Workers:             intValueZero(proxy["workers"]),
		PersistentKeepalive: intValueZero(proxy["persistent-keepalive"]),
	}
	if node.WireGuard.IP != "" {
		node.WireGuard.Address = append(node.WireGuard.Address, node.WireGuard.IP)
	}
	if node.WireGuard.IPv6 != "" {
		node.WireGuard.Address = append(node.WireGuard.Address, node.WireGuard.IPv6)
	}
	peers := shared.StringValue(proxy["peers"])
	_ = peers
	peerItems, _ := proxy["peers"].([]any)
	if len(peerItems) == 0 && shared.StringValue(proxy["public-key"]) != "" {
		node.WireGuard.Peers = []domain.WireGuardPeer{{
			Server:       node.Server,
			Port:         node.Port,
			PublicKey:    shared.StringValue(proxy["public-key"]),
			PreSharedKey: shared.StringValue(proxy["pre-shared-key"]),
			AllowedIPs:   shared.StringSliceValue(proxy["allowed-ips"]),
			Reserved:     uint8SliceValue(proxy["reserved"]),
		}}
		return
	}
	for _, item := range peerItems {
		peerMap := shared.AnyMapValue(item)
		if peerMap == nil {
			continue
		}
		port, _ := shared.Uint16Value(peerMap["port"])
		node.WireGuard.Peers = append(node.WireGuard.Peers, domain.WireGuardPeer{
			Server:       shared.StringValue(peerMap["server"]),
			Port:         port,
			PublicKey:    shared.StringValue(peerMap["public-key"]),
			PreSharedKey: shared.StringValue(peerMap["pre-shared-key"]),
			AllowedIPs:   shared.StringSliceValue(peerMap["allowed-ips"]),
			Reserved:     uint8SliceValue(peerMap["reserved"]),
		})
	}
}

func isExplicitTrue(value any) bool {
	enabled, ok := value.(bool)
	return ok && enabled
}

func preserveNestedMihomoRaw(node *domain.NodeIR, parent string, opts map[string]any, known map[string]bool) {
	if len(opts) == 0 {
		return
	}
	if node.Raw == nil {
		node.Raw = map[string]json.RawMessage{}
	}
	for _, key := range sortedMapKeys(opts) {
		if known[key] {
			continue
		}
		shared.AddRaw(node.Raw, "mihomo."+parent+"."+key, opts[key])
	}
}
