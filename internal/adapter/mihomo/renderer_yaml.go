package mihomo

import (
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func marshalProxiesDocument(proxies []map[string]any) ([]byte, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "proxies"},
		proxiesYAMLNode(proxies),
	)
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	return yaml.Marshal(doc)
}

func proxiesYAMLNode(proxies []map[string]any) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, proxy := range proxies {
		seq.Content = append(seq.Content, orderedMapNode(proxy, proxyOrder(proxy)))
	}
	return seq
}

func orderedMapNode(values map[string]any, order []string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	seen := map[string]bool{}
	for _, key := range order {
		value, ok := values[key]
		if !ok || isEmptyYAMLValue(value) {
			continue
		}
		seen[key] = true
		node.Content = append(node.Content, scalarNode(key), valueNode(value))
	}
	extra := make([]string, 0, len(values))
	for key := range values {
		if !seen[key] && !isEmptyYAMLValue(values[key]) {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	for _, key := range extra {
		node.Content = append(node.Content, scalarNode(key), valueNode(values[key]))
	}
	return node
}

func proxyOrder(proxy map[string]any) []string {
	switch proxy["type"] {
	case "ss":
		return []string{"name", "type", "server", "port", "cipher", "password", "plugin", "plugin-opts", "udp", "tfo", "udp-over-tcp", "udp-over-tcp-version", "network", "ws-opts", "tls", "servername", "skip-cert-verify", "client-fingerprint", "fingerprint", "alpn", "ech-opts"}
	case "ssr":
		return []string{"name", "type", "server", "port", "cipher", "password", "protocol", "protocol-param", "obfs", "obfs-param", "udp", "tfo"}
	case "snell":
		return []string{"name", "type", "server", "port", "psk", "version", "reuse", "client-fingerprint", "obfs-opts", "udp", "tfo"}
	case "anytls":
		return []string{"name", "type", "server", "port", "password", "idle-session-check-interval", "idle-session-timeout", "min-idle-session", "sni", "skip-cert-verify", "client-fingerprint", "fingerprint", "alpn"}
	case "vmess":
		return []string{"name", "type", "server", "port", "uuid", "cipher", "alterId", "udp", "tfo", "network", "ws-opts", "grpc-opts", "h2-opts", "http-opts", "tls", "servername", "skip-cert-verify", "client-fingerprint", "fingerprint", "alpn", "ech-opts", "reality-opts", "packet-encoding"}
	case "vless":
		return []string{"name", "type", "server", "port", "uuid", "flow", "encryption", "udp", "tfo", "network", "ws-opts", "grpc-opts", "h2-opts", "http-opts", "xhttp-opts", "tls", "servername", "skip-cert-verify", "client-fingerprint", "fingerprint", "alpn", "ech-opts", "reality-opts", "packet-encoding"}
	case "trojan":
		return []string{"name", "type", "server", "port", "password", "udp", "tfo", "network", "ws-opts", "grpc-opts", "tls", "sni", "skip-cert-verify", "client-fingerprint", "fingerprint", "alpn", "ech-opts", "reality-opts"}
	case "hysteria":
		return []string{"name", "type", "server", "port", "ports", "up", "up-speed", "down", "down-speed", "auth", "auth-str", "obfs", "hop-interval", "fast-open", "sni", "skip-cert-verify", "client-fingerprint", "fingerprint", "alpn", "ech-opts"}
	case "hysteria2":
		return []string{"name", "type", "server", "port", "ports", "password", "up", "down", "obfs", "obfs-password", "hop-interval", "tfo", "sni", "skip-cert-verify", "client-fingerprint", "fingerprint", "alpn", "bbr-profile", "cwnd", "udp-mtu", "realm-opts"}
	case "tuic":
		return []string{"name", "type", "server", "port", "token", "uuid", "password", "congestion-controller", "udp-relay-mode", "reduce-rtt", "udp-over-stream", "udp-over-stream-version", "fast-open", "sni", "skip-cert-verify", "client-fingerprint", "fingerprint", "alpn", "ech-opts"}
	case "mieru":
		return []string{"name", "type", "server", "port", "port-range", "transport", "username", "password", "udp", "multiplexing", "handshake-mode", "traffic-pattern"}
	case "socks5", "http":
		return []string{"name", "type", "server", "port", "username", "password", "headers", "udp", "tfo", "tls", "sni", "skip-cert-verify", "client-fingerprint", "fingerprint"}
	case "wireguard":
		return []string{"name", "type", "server", "port", "udp", "tfo", "ip", "ipv6", "private-key", "public-key", "pre-shared-key", "allowed-ips", "reserved", "peers", "mtu", "workers", "persistent-keepalive"}
	default:
		return []string{"name", "type", "server", "port"}
	}
}

func valueNode(value any) *yaml.Node {
	if m, ok := value.(map[string]any); ok {
		return orderedMapNode(m, nestedOrder(m))
	}
	if m, ok := value.(map[string]string); ok {
		anyMap := make(map[string]any, len(m))
		for key, val := range m {
			anyMap[key] = val
		}
		return orderedMapNode(anyMap, nestedOrder(anyMap))
	}
	if m, ok := value.(map[string][]string); ok {
		anyMap := make(map[string]any, len(m))
		for key, val := range m {
			anyMap[key] = val
		}
		return orderedMapNode(anyMap, nestedOrder(anyMap))
	}
	if list, ok := value.([]map[string]any); ok {
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range list {
			seq.Content = append(seq.Content, orderedMapNode(item, nestedOrder(item)))
		}
		return seq
	}
	var node yaml.Node
	_ = node.Encode(value)
	return &node
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: value}
}

func isEmptyYAMLValue(value any) bool {
	switch t := value.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []string:
		return len(t) == 0
	case []uint8:
		return len(t) == 0
	case []map[string]any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	case map[string]string:
		return len(t) == 0
	case map[string][]string:
		return len(t) == 0
	default:
		return false
	}
}

func nestedOrder(values map[string]any) []string {
	order := []string{
		"path", "headers", "mode", "password", "host", "version", "alpn", "fingerprint", "certificate", "private-key", "skip-cert-verify", "max-early-data", "early-data-header-name",
		"grpc-service-name", "max-connections", "min-streams", "max-streams", "method",
		"enable", "config", "query-server-name",
		"public-key", "short-id",
		"server", "port", "pre-shared-key", "allowed-ips", "reserved",
		"server-url", "token", "realm-id", "stun-servers",
	}
	if _, ok := values["Host"]; ok {
		order = append([]string{"Host"}, order...)
	}
	return order
}

func durationSecondsOrString(value string) any {
	if value == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	if strings.HasSuffix(trimmed, "s") {
		if n, err := strconv.Atoi(strings.TrimSuffix(trimmed, "s")); err == nil {
			return n
		}
	}
	if n, err := strconv.Atoi(trimmed); err == nil {
		return n
	}
	return value
}
