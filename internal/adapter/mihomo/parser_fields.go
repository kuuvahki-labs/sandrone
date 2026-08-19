package mihomo

import (
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func mihomoKnownFields(nodeType domain.NodeType) map[string]bool {
	common := shared.KnownFields(
		"name", "type", "server", "port", "username", "user",
		"password", "uuid", "cipher", "method",
		"tls", "sni", "servername", "skip-cert-verify", "fingerprint", "client-fingerprint",
		"alpn", "ech-opts", "reality-opts", "mux", "udp-over-tcp",
		"udp-over-tcp-version", "plugin", "plugin-opts", "headers",
		"flow", "encryption", "packet-encoding", "tfo",
		"country", "delay",
	)
	add := func(keys ...string) {
		shared.AddKnownFields(common, keys...)
	}
	switch nodeType {
	case domain.NodeTypeShadowsocks, domain.NodeTypeSOCKS:
		add("udp")
	case domain.NodeTypeShadowsocksR:
		add("udp", "protocol", "protocol-param", "protocolparam", "obfs", "obfs-param", "obfsparam")
	case domain.NodeTypeSnell:
		add("udp", "psk", "version", "reuse", "client-fingerprint", "obfs-opts")
	case domain.NodeTypeAnyTLS:
		add("idle-session-check-interval", "idle-session-timeout", "min-idle-session")
	case domain.NodeTypeVMess:
		add("alterId", "udp", "xudp", "packet-addr", "network", "ws-opts", "fast-open", "grpc-opts", "h2-opts", "http-opts")
	case domain.NodeTypeVLESS:
		add("udp", "xudp", "packet-addr", "network", "ws-opts", "grpc-opts", "h2-opts", "http-opts", "xhttp-opts")
	case domain.NodeTypeTrojan:
		add("udp", "network", "ws-opts", "grpc-opts")
	case domain.NodeTypeHysteria:
		add("ports", "server-ports", "protocol", "obfs-protocol", "up", "up-speed", "down", "down-speed", "auth", "auth-str", "obfs", "hop-interval", "fast-open")
	case domain.NodeTypeHysteria2:
		add("ports", "server-ports", "hop-interval", "up", "down", "obfs", "obfs-password", "auth", "realm-opts", "bbr-profile", "udp-mtu", "cwnd", "masquerade")
	case domain.NodeTypeTUIC:
		add("token", "congestion-controller", "udp-relay-mode", "reduce-rtt", "heartbeat-interval", "udp-over-stream", "udp-over-stream-version", "fast-open")
	case domain.NodeTypeMieru:
		add("port-range", "transport", "udp", "multiplexing", "handshake-mode", "traffic-pattern")
	case domain.NodeTypeWireGuard:
		add("private-key", "ip", "ipv6", "peers", "public-key", "pre-shared-key", "allowed-ips", "reserved", "mtu", "workers", "persistent-keepalive", "udp")
	}
	return common
}
