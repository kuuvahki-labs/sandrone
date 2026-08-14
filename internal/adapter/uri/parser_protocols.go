package uri

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func parseVLESS(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeVLESS, SourceFormat: "uri"}
	source := shared.SourceInfo("vless", shared.SourceRefs("vless"))
	u, err := url.Parse(raw)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse vless URI", err)
	}
	host, port, err := shared.ParseURLHostPort(u, "443")
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse vless server", err)
	}
	node.Name = shared.DecodeName(u.Fragment, host)
	node.Server = host
	node.Port = port
	node.UUID = u.User.Username()
	if node.UUID == "" {
		return node, source, domain.NewError(domain.CodeParseFailed, "missing vless uuid")
	}
	values := u.Query()
	node.Flow = values.Get("flow")
	node.Encryption = firstNonEmpty(values.Get("encryption"), "none")
	node.PacketEncoding = shared.QueryFirst(values, "packetEncoding", "packet-encoding")
	applyTLSQuery(&node, values)
	applyTransportQuery(&node, values)
	node.Raw = map[string]json.RawMessage{}
	known := map[string]bool{
		"flow": true, "encryption": true, "packetEncoding": true, "packet-encoding": true,
		"security": true, "tls": true, "sni": true, "servername": true, "serverName": true, "fp": true, "fingerprint": true, "pinSHA256": true, "pcs": true, "alpn": true,
		"allowInsecure": true, "allowinsecure": true, "allow_insecure": true, "allow-insecure": true, "skip-cert-verify": true, "insecure": true, "disable_sni": true,
		"pbk": true, "public-key": true, "sid": true, "short-id": true,
		"ech": true, "echForceQuery": true,
		"type": true, "net": true, "transport": true, "host": true, "authority": true, "path": true, "wspath": true, "wsPath": true, "ws-path": true, "obfs-uri": true, "serviceName": true, "service_name": true,
	}
	applyWebSocketEarlyDataQuery(node.Transport, values, known)
	if node.Transport != nil && node.Transport.Type == "xhttp" {
		known["mode"] = true
		known["extra"] = true
	}
	if queryValuesAreNoopHeaderType(values, "headerType") {
		known["headerType"] = true
	} else if node.Transport != nil && node.Transport.HeaderType == "http" {
		known["headerType"] = true
		if values.Get("method") != "" {
			known["method"] = true
		}
	}
	if node.Transport != nil && node.Transport.Type == "grpc" && queryValuesEqualFold(values, "mode", "gun") {
		known["mode"] = true
	}
	if node.Transport != nil && node.Transport.Type == "tcp" && queryValuesEqualFold(values, "quicSecurity", "none") {
		known["quicSecurity"] = true
	}
	if queryValuesAreEmpty(values, "pqv") {
		known["pqv"] = true
	}
	preserveURIQuery(&node, values, known)
	return node, source, nil
}

func parseTrojan(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeTrojan, SourceFormat: "uri"}
	source := shared.SourceInfo("trojan", shared.SourceRefs("trojan"))
	u, err := url.Parse(raw)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse trojan URI", err)
	}
	host, port, err := shared.ParseURLHostPort(u, "443")
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse trojan server", err)
	}
	password, _ := url.QueryUnescape(u.User.Username())
	if password == "" {
		return node, source, domain.NewError(domain.CodeParseFailed, "missing trojan password")
	}
	node.Name = shared.DecodeName(u.Fragment, host)
	node.Server = host
	node.Port = port
	node.Password = password
	values := u.Query()
	applyTLSQuery(&node, values)
	if peer := values.Get("peer"); peer != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.ServerName = peer
	}
	if node.TLS == nil {
		node.TLS = &domain.TLSOptions{Enabled: true}
	} else {
		node.TLS.Enabled = true
	}
	applyTransportQuery(&node, values)
	node.Raw = map[string]json.RawMessage{}
	known := map[string]bool{
		"security": true, "tls": true, "sni": true, "servername": true, "serverName": true, "peer": true,
		"fp": true, "fingerprint": true, "pinSHA256": true, "pcs": true, "alpn": true,
		"allowInsecure": true, "allowinsecure": true, "allow_insecure": true, "allow-insecure": true, "skip-cert-verify": true, "insecure": true, "disable_sni": true,
		"pbk": true, "public-key": true, "sid": true, "short-id": true,
		"type": true, "net": true, "transport": true, "host": true, "authority": true,
		"path": true, "wspath": true, "obfs-uri": true, "serviceName": true, "service_name": true,
		"wsHost": true, "ws-host": true, "wsPath": true, "ws-path": true,
	}
	if queryValuesAreNoopHeaderType(values, "headerType") {
		known["headerType"] = true
	} else if node.Transport != nil && node.Transport.HeaderType == "http" {
		known["headerType"] = true
		if values.Get("method") != "" {
			known["method"] = true
		}
	}
	if node.Transport != nil && node.Transport.Type == "grpc" && queryValuesEqualFold(values, "mode", "gun") {
		known["mode"] = true
	}
	applyWebSocketEarlyDataQuery(node.Transport, values, known)
	preserveURIQuery(&node, values, known)
	return node, source, nil
}

func parseHysteria(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeHysteria, SourceFormat: "uri"}
	source := shared.SourceInfo("hysteria", shared.SourceRefs("hysteria"))
	u, err := url.Parse(raw)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse hysteria URI", err)
	}
	host, port, err := shared.ParseURLHostPort(u, "")
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse hysteria server", err)
	}
	node.Name = shared.DecodeName(u.Fragment, host)
	node.Server = host
	node.Port = port
	values := u.Query()
	node.Hysteria = &domain.HysteriaOptions{
		Protocol:     values.Get("protocol"),
		Auth:         values.Get("auth"),
		AuthString:   shared.QueryFirst(values, "auth_str", "auth-str", "authString"),
		Obfs:         values.Get("obfs"),
		ObfsPassword: shared.QueryFirst(values, "obfsParam", "obfs-param", "obfs-password", "obfs_password"),
		HopInterval:  shared.QueryFirst(values, "hop_interval", "hop-interval"),
	}
	knownRates := shared.NormalizeURIHysteriaBandwidth(&node, source, values)
	applyTLSQuery(&node, values)
	applyHysteriaParseQueryTLS(&node, values)
	if node.TLS == nil {
		node.TLS = &domain.TLSOptions{Enabled: true}
	} else {
		node.TLS.Enabled = true
	}
	node.Raw = map[string]json.RawMessage{}
	known := map[string]bool{
		"protocol": true,
		"auth":     true, "auth_str": true, "auth-str": true, "authString": true,
		"obfs": true, "obfsParam": true, "obfs-param": true, "obfs-password": true, "obfs_password": true,
		"hop_interval": true, "hop-interval": true,
		"security": true, "tls": true, "sni": true, "servername": true, "serverName": true, "alpn": true,
		"peer": true, "insecure": true, "allowInsecure": true, "allowinsecure": true, "allow_insecure": true, "allow-insecure": true, "skip-cert-verify": true, "disable_sni": true,
		"fp": true, "fingerprint": true, "pinSHA256": true, "pcs": true,
	}
	for key, value := range knownRates {
		known[key] = value
	}
	preserveURIQuery(&node, values, known)
	return node, source, nil
}

func parseHysteria2(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeHysteria2, SourceFormat: "uri"}
	source := shared.SourceInfo("hysteria2", shared.SourceRefs("hysteria2"))
	authority, queryStr, fragment, err := splitURI(raw, "hy2", "hysteria2")
	if err != nil {
		return node, source, err
	}
	userInfo, hostPart := splitUserInfo(authority)
	host, port, serverPorts, err := parseHysteria2Authority(hostPart)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse hysteria2 server", err)
	}
	password, _ := url.QueryUnescape(userInfo)
	node.Server = host
	node.Port = port
	node.Password = password
	values, err := url.ParseQuery(queryStr)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse hysteria2 query", err)
	}
	node.Name = shared.DecodeName(fragment, host)
	node.Hysteria = &domain.HysteriaOptions{
		Obfs:         shared.QueryFirst(values, "obfs", "obfs-type"),
		ObfsPassword: shared.QueryFirst(values, "obfs-password", "obfs_password", "obfsParam", "obfs-param"),
		HopInterval:  shared.QueryFirst(values, "hop_interval", "hop-interval"),
		ServerPorts:  serverPorts,
	}
	applyTLSQuery(&node, values)
	applyHysteria2ParseQueryTLS(&node, values)
	if node.TLS == nil {
		node.TLS = &domain.TLSOptions{Enabled: true}
	} else {
		node.TLS.Enabled = true
	}
	node.Raw = map[string]json.RawMessage{}
	known := map[string]bool{
		"obfs": true, "obfs-type": true, "obfs-password": true, "obfs_password": true, "obfsParam": true, "obfs-param": true,
		"hop_interval": true, "hop-interval": true,
		"security": true, "tls": true, "sni": true, "servername": true, "serverName": true, "alpn": true,
		"allowInsecure": true, "allowinsecure": true, "allow_insecure": true, "allow-insecure": true, "skip-cert-verify": true, "insecure": true, "disable_sni": true,
		"fp": true, "fingerprint": true, "pinSHA256": true, "pcs": true,
	}
	if peer := values.Get("peer"); peer != "" && node.TLS != nil && node.TLS.ServerName == peer {
		known["peer"] = true
	}
	preserveURIQuery(&node, values, known)
	return node, source, nil
}

func splitURI(raw string, schemes ...string) (authority string, query string, fragment string, err error) {
	scheme, body, ok := strings.Cut(raw, "://")
	if !ok {
		return "", "", "", domain.NewError(domain.CodeParseFailed, "missing URI scheme")
	}
	matched := false
	for _, allowed := range schemes {
		if strings.EqualFold(scheme, allowed) {
			matched = true
			break
		}
	}
	if !matched {
		return "", "", "", domain.NewError(domain.CodeParseFailed, "unsupported URI scheme")
	}
	body, fragment, _ = strings.Cut(body, "#")
	body, query, _ = strings.Cut(body, "?")
	authority, _, _ = strings.Cut(body, "/")
	if authority == "" {
		return "", "", "", domain.NewError(domain.CodeParseFailed, "missing URI authority")
	}
	return authority, query, fragment, nil
}

func splitUserInfo(authority string) (string, string) {
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return "", authority
	}
	return authority[:at], authority[at+1:]
}

func parseHysteria2Authority(authority string) (string, uint16, []string, error) {
	host, portSpec, err := splitHysteria2Authority(authority)
	if err != nil {
		return "", 0, nil, err
	}
	if host == "" {
		return "", 0, nil, fmt.Errorf("missing host")
	}
	if portSpec == "" {
		return host, 443, nil, nil
	}
	ports := splitList(portSpec)
	firstPort := ports[0]
	if start, _, ok := strings.Cut(firstPort, "-"); ok {
		firstPort = start
	}
	port, err := strconv.Atoi(firstPort)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, nil, fmt.Errorf("invalid port")
	}
	if len(ports) == 1 && ports[0] == strconv.Itoa(port) {
		return host, uint16(port), nil, nil
	}
	return host, uint16(port), ports, nil
}

func splitHysteria2Authority(authority string) (string, string, error) {
	if strings.HasPrefix(authority, "[") {
		end := strings.Index(authority, "]")
		if end < 0 {
			return "", "", fmt.Errorf("invalid ipv6 host")
		}
		host := authority[1:end]
		if len(authority) == end+1 {
			return host, "", nil
		}
		if authority[end+1] != ':' {
			return "", "", fmt.Errorf("invalid host port separator")
		}
		return host, authority[end+2:], nil
	}
	if host, portSpec, ok := shared.SplitBareIPv6HostPort(authority); ok {
		return host, portSpec, nil
	}
	if host, portSpec, ok := strings.Cut(authority, ":"); ok {
		return host, portSpec, nil
	}
	return authority, "", nil
}

func parseTUIC(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeTUIC, SourceFormat: "uri"}
	source := shared.SourceInfo("tuic", shared.SourceRefs("tuic"))
	u, err := url.Parse(raw)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse tuic URI", err)
	}
	host, port, err := shared.ParseURLHostPort(u, "443")
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse tuic server", err)
	}
	node.Name = shared.DecodeName(u.Fragment, host)
	node.Server = host
	node.Port = port
	node.UUID = u.User.Username()
	if password, ok := u.User.Password(); ok {
		node.Password, _ = url.QueryUnescape(password)
	}
	values := u.Query()
	node.Token = values.Get("token")
	node.TUIC = &domain.TUICOptions{
		CongestionControl: shared.QueryFirst(values, "congestion_control", "congestion-controller"),
		UDPRelayMode:      shared.QueryFirst(values, "udp_relay_mode", "udp-relay-mode"),
		ZeroRTTHandshake:  shared.BoolValue(values.Get("zero_rtt_handshake")),
		ReduceRTT:         shared.BoolValue(values.Get("reduce_rtt")),
		Heartbeat:         values.Get("heartbeat"),
	}
	applyTLSQuery(&node, values)
	if node.TLS == nil {
		node.TLS = &domain.TLSOptions{Enabled: true}
	} else {
		node.TLS.Enabled = true
	}
	node.Raw = map[string]json.RawMessage{}
	preserveURIQuery(&node, values, map[string]bool{
		"token": true, "congestion_control": true, "congestion-controller": true,
		"udp_relay_mode": true, "udp-relay-mode": true, "zero_rtt_handshake": true,
		"reduce_rtt": true, "heartbeat": true, "security": true, "sni": true,
		"alpn": true, "allowInsecure": true, "allowinsecure": true, "allow_insecure": true, "allow-insecure": true,
		"skip-cert-verify": true, "insecure": true, "disable_sni": true,
		"fp": true, "fingerprint": true, "pinSHA256": true, "pcs": true,
	})
	return node, source, nil
}

func parseMieru(raw string) ([]domain.NodeIR, *domain.SourceInfo, error) {
	source := shared.SourceInfo("mieru", shared.SourceRefs("mieru"))
	u, err := url.Parse(raw)
	if err != nil {
		return nil, source, domain.WrapError(domain.CodeParseFailed, "parse mieru URI", err)
	}
	server := u.Hostname()
	if server == "" {
		return nil, source, domain.NewError(domain.CodeParseFailed, "missing mieru server")
	}
	values := u.Query()
	ports := values["port"]
	protocols := values["protocol"]
	if len(ports) == 0 {
		return nil, source, domain.NewError(domain.CodeParseFailed, "missing mieru port")
	}
	if len(ports) != len(protocols) {
		return nil, source, domain.NewError(domain.CodeParseFailed, "mieru port and protocol counts must match")
	}
	baseName := shared.DecodeName(u.Fragment, "")
	if baseName == "node" {
		baseName = values.Get("profile")
	}
	if baseName == "" || baseName == "node" {
		baseName = server
	}
	username := u.User.Username()
	password, _ := u.User.Password()
	multiplexing := values.Get("multiplexing")
	handshakeMode := values.Get("handshake-mode")
	trafficPattern := values.Get("traffic-pattern")
	udp := true
	nodes := make([]domain.NodeIR, 0, len(ports))
	for i, portSpec := range ports {
		protocol := protocols[i]
		if portSpec == "" || protocol == "" {
			return nil, source, domain.NewError(domain.CodeParseFailed, "missing mieru port or protocol")
		}
		node := domain.NodeIR{
			Name:         fmt.Sprintf("%s:%s/%s", baseName, portSpec, protocol),
			Type:         domain.NodeTypeMieru,
			Server:       server,
			Username:     username,
			Password:     password,
			SourceFormat: "uri",
			Dialer:       &domain.DialerOptions{UDPRelay: &udp},
			Mieru: &domain.MieruOptions{
				Transport:      protocol,
				Multiplexing:   multiplexing,
				HandshakeMode:  handshakeMode,
				TrafficPattern: trafficPattern,
			},
		}
		if strings.Contains(portSpec, "-") {
			node.Mieru.PortRange = portSpec
		} else {
			port, err := strconv.Atoi(portSpec)
			if err != nil || port <= 0 || port > 65535 {
				return nil, source, domain.NewError(domain.CodeParseFailed, "invalid mieru port")
			}
			node.Port = uint16(port)
		}
		nodes = append(nodes, node)
	}
	return nodes, source, nil
}

func parseSOCKS(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeSOCKS, SourceFormat: "uri"}
	source := shared.SourceInfo("socks", shared.SourceRefs("socks"))
	u, err := url.Parse(raw)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse socks URI", err)
	}
	host, port, err := shared.ParseURLHostPort(u, "1080")
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse socks server", err)
	}
	node.Name = shared.DecodeName(u.Fragment, host)
	node.Server = host
	node.Port = port
	node.Username = u.User.Username()
	if password, ok := u.User.Password(); ok {
		node.Password, _ = url.QueryUnescape(password)
	}
	node.Raw = map[string]json.RawMessage{}
	values := u.Query()
	preserveURIQuery(&node, values, map[string]bool{})
	return node, source, nil
}

func parseTelegramProxy(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return emptyTelegramNode(), telegramSource("tg"), domain.WrapError(domain.CodeParseFailed, "parse telegram proxy URI", err)
	}
	kind := strings.TrimPrefix(u.Host+u.Path, "/")
	if kind == "" {
		kind = strings.TrimPrefix(u.Opaque, "/")
	}
	return parseTelegramProxyURL(u, kind)
}

func parseTelegramProxyURL(u *url.URL, kind string) (domain.NodeIR, *domain.SourceInfo, error) {
	values := u.Query()
	server := shared.QueryFirst(values, "server", "host")
	if server == "" {
		return telegramProxyParseError("missing telegram proxy server")
	}
	port, err := parseTelegramProxyPort(kind, values.Get("port"))
	if err != nil {
		return telegramProxyParseError(err.Error())
	}
	nodeType, format, err := telegramProxyType(kind)
	if err != nil {
		return telegramProxyParseError(err.Error())
	}
	node := domain.NodeIR{
		Type:         nodeType,
		SourceFormat: "uri",
		Name:         shared.DecodeName(u.Fragment, server),
		Server:       server,
		Port:         port,
		Username:     shared.QueryFirst(values, "user", "username"),
		Password:     shared.QueryFirst(values, "pass", "password"),
		Raw:          map[string]json.RawMessage{},
	}
	preserveURIQuery(&node, values, map[string]bool{"server": true, "host": true, "port": true, "user": true, "username": true, "pass": true, "password": true})
	return node, telegramSource(format), nil
}

func parseTelegramProxyPort(kind string, rawPort string) (uint16, error) {
	portStr := rawPort
	if portStr == "" {
		portStr = "80"
		if isTelegramSocksKind(kind) {
			portStr = "1080"
		}
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid telegram proxy port")
	}
	return uint16(port), nil
}

func telegramProxyType(kind string) (domain.NodeType, string, error) {
	if isTelegramSocksKind(kind) {
		return domain.NodeTypeSOCKS, "socks", nil
	}
	if strings.EqualFold(kind, "http") {
		return domain.NodeTypeHTTP, "http", nil
	}
	if strings.EqualFold(kind, "proxy") {
		return "", "", fmt.Errorf("unsupported telegram MTProto proxy")
	}
	return "", "", fmt.Errorf("unsupported telegram proxy type")
}

func isTelegramSocksKind(kind string) bool {
	return strings.EqualFold(kind, "socks") || strings.EqualFold(kind, "socks5")
}

func telegramProxyParseError(message string) (domain.NodeIR, *domain.SourceInfo, error) {
	return emptyTelegramNode(), telegramSource("tg"), domain.NewError(domain.CodeParseFailed, message)
}

func emptyTelegramNode() domain.NodeIR {
	return domain.NodeIR{SourceFormat: "uri"}
}

func telegramSource(format string) *domain.SourceInfo {
	return shared.SourceInfo(format, shared.SourceRefs("tg"))
}

func parseHTTP(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeHTTP, SourceFormat: "uri"}
	source := shared.SourceInfo("http", shared.SourceRefs("http"))
	u, err := url.Parse(raw)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse http URI", err)
	}
	if isTelegramProxyWebURL(u) {
		return parseTelegramProxyURL(u, strings.TrimPrefix(u.Path, "/"))
	}
	defaultPort := "80"
	if strings.EqualFold(u.Scheme, "https") {
		defaultPort = "443"
	}
	host, port, err := shared.ParseURLHostPort(u, defaultPort)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse http server", err)
	}
	node.Name = shared.DecodeName(u.Fragment, host)
	node.Server = host
	node.Port = port
	node.Username = u.User.Username()
	if password, ok := u.User.Password(); ok {
		node.Password, _ = url.QueryUnescape(password)
	}
	if strings.EqualFold(u.Scheme, "https") {
		node.TLS = &domain.TLSOptions{Enabled: true}
	}
	values := u.Query()
	applyTLSQuery(&node, values)
	node.Raw = map[string]json.RawMessage{}
	preserveURIQuery(&node, values, map[string]bool{
		"security": true, "tls": true, "sni": true, "servername": true, "serverName": true,
		"allowInsecure": true, "allowinsecure": true, "allow_insecure": true, "allow-insecure": true, "skip-cert-verify": true, "insecure": true, "disable_sni": true,
		"fp": true, "fingerprint": true, "pinSHA256": true, "pcs": true,
	})
	return node, source, nil
}
