package singbox

import (
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func baseOutbound(node domain.NodeIR, typ string) map[string]any {
	out := map[string]any{
		"type":        typ,
		"tag":         node.Name,
		"server":      node.Server,
		"server_port": int(node.Port),
	}
	applyDialer(out, node)
	return out
}

func applyDialer(out map[string]any, node domain.NodeIR) {
	if singBoxSupportsNetwork(node.Type) {
		switch {
		case node.Network != "":
			out["network"] = node.Network
		case node.Dialer != nil && node.Dialer.UDPRelay != nil && !*node.Dialer.UDPRelay:
			out["network"] = "tcp"
		}
	}
	if node.Dialer != nil && node.Dialer.TFO {
		out["tcp_fast_open"] = true
	}
}

func singBoxSupportsNetwork(nodeType domain.NodeType) bool {
	switch nodeType {
	case domain.NodeTypeShadowsocks, domain.NodeTypeVMess, domain.NodeTypeVLESS, domain.NodeTypeTrojan, domain.NodeTypeHysteria, domain.NodeTypeHysteria2, domain.NodeTypeTUIC, domain.NodeTypeSOCKS:
		return true
	default:
		return false
	}
}

func applyTLS(out map[string]any, node domain.NodeIR) {
	if node.TLS == nil || !node.TLS.Enabled {
		return
	}
	tls := map[string]any{}
	if node.TLS.Enabled {
		tls["enabled"] = true
	}
	if node.TLS.DisableSNI {
		tls["disable_sni"] = true
	}
	if node.TLS.ServerName != "" {
		tls["server_name"] = node.TLS.ServerName
	}
	if node.TLS.InsecureSkipVerify {
		tls["insecure"] = true
	}
	if len(node.TLS.ALPN) > 0 {
		tls["alpn"] = node.TLS.ALPN
	}
	if node.TLS.ClientFingerprint != "" {
		tls["utls"] = map[string]any{
			"enabled":     true,
			"fingerprint": node.TLS.ClientFingerprint,
		}
	}
	if node.TLS.ECH != nil {
		ech := map[string]any{}
		if node.TLS.ECH.Enabled {
			ech["enabled"] = true
		}
		if len(node.TLS.ECH.Config) > 0 {
			ech["config"] = node.TLS.ECH.Config
		}
		if node.TLS.ECH.QueryServerName != "" {
			ech["query_server_name"] = node.TLS.ECH.QueryServerName
		}
		tls["ech"] = ech
	}
	if node.TLS.Reality != nil {
		tls["reality"] = map[string]any{
			"enabled":    true,
			"public_key": node.TLS.Reality.PublicKey,
			"short_id":   node.TLS.Reality.ShortID,
		}
	}
	out["tls"] = tls
}

func applyTransport(out map[string]any, node domain.NodeIR) error {
	if node.Transport == nil {
		return nil
	}
	if strings.TrimSpace(node.Transport.Type) == "" {
		if transportHasDetails(node.Transport) {
			return domain.NewError(domain.CodeRenderFailed, "sing-box V2Ray transport has connection parameters but no transport type")
		}
		return nil
	}
	if isDefaultTCPTransport(node.Transport) {
		return nil
	}
	if isHTTPHeaderTransport(node.Transport) {
		return domain.NewError(domain.CodeRenderFailed, "sing-box V2Ray transport schema does not support TCP HTTP header obfuscation")
	}
	if !singBoxSupportsTransport(node.Transport.Type) {
		return domain.NewError(
			domain.CodeRenderFailed,
			fmt.Sprintf("sing-box V2Ray transport schema does not support %q transport", node.Transport.Type),
		)
	}
	transport := map[string]any{}
	switch node.Transport.Type {
	case "websocket", "ws":
		if node.Transport.V2RayHTTPUpgrade {
			transport["type"] = "httpupgrade"
		} else {
			transport["type"] = "ws"
		}
		if node.Transport.Path != "" {
			transport["path"] = node.Transport.Path
		}
		headers := node.Transport.Headers
		if node.Transport.V2RayHTTPUpgrade {
			if node.Transport.Host != "" {
				transport["host"] = node.Transport.Host
			} else if headers["Host"] != "" {
				transport["host"] = headers["Host"]
			}
			headers = headersWithoutHost(headers)
		} else if len(headers) == 0 && node.Transport.Host != "" {
			headers = map[string]string{"Host": node.Transport.Host}
		}
		if len(headers) > 0 {
			transport["headers"] = headers
		}
		if !node.Transport.V2RayHTTPUpgrade && node.Transport.MaxEarlyData != 0 {
			transport["max_early_data"] = node.Transport.MaxEarlyData
		}
		if !node.Transport.V2RayHTTPUpgrade && node.Transport.EarlyDataHeaderName != "" {
			transport["early_data_header_name"] = node.Transport.EarlyDataHeaderName
		}
	case "grpc":
		transport["type"] = "grpc"
		if node.Transport.ServiceName != "" {
			transport["service_name"] = node.Transport.ServiceName
		}
	case "http":
		transport["type"] = "http"
		if node.Transport.Host != "" {
			transport["host"] = []string{node.Transport.Host}
		} else if len(node.Transport.Hosts) > 0 {
			transport["host"] = node.Transport.Hosts
		}
		if node.Transport.Path != "" {
			transport["path"] = node.Transport.Path
		}
		if node.Transport.Method != "" {
			transport["method"] = node.Transport.Method
		}
		if len(node.Transport.Headers) > 0 {
			transport["headers"] = node.Transport.Headers
		}
	case "quic":
		transport["type"] = "quic"
	case "httpupgrade":
		transport["type"] = "httpupgrade"
		if node.Transport.Host != "" {
			transport["host"] = node.Transport.Host
		}
		if node.Transport.Path != "" {
			transport["path"] = node.Transport.Path
		}
		if len(node.Transport.Headers) > 0 {
			transport["headers"] = node.Transport.Headers
		}
	}
	out["transport"] = transport
	return nil
}

func headersWithoutHost(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range headers {
		if strings.EqualFold(key, "host") {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func singBoxSupportsTransport(transportType string) bool {
	switch transportType {
	case "websocket", "ws", "http", "grpc", "quic", "httpupgrade":
		return true
	default:
		return false
	}
}

func isDefaultTCPTransport(transport *domain.TransportOptions) bool {
	if transport == nil {
		return false
	}
	typeName := strings.TrimSpace(strings.ToLower(transport.Type))
	if typeName != "tcp" && typeName != "raw" {
		return false
	}
	return !transportHasDetails(transport)
}

func transportHasDetails(transport *domain.TransportOptions) bool {
	return transport != nil && (transport.HeaderType != "" ||
		transport.Method != "" ||
		transport.Path != "" ||
		transport.Host != "" ||
		len(transport.Hosts) > 0 ||
		len(transport.Headers) > 0 ||
		transport.ServiceName != "" ||
		transport.MaxEarlyData != 0 ||
		transport.EarlyDataHeaderName != "" ||
		transport.V2RayHTTPUpgrade ||
		transport.V2RayHTTPUpgradeFastOpen ||
		transport.XHTTP != nil)
}

func applyMux(out map[string]any, node domain.NodeIR) {
	if node.Multiplex == nil {
		return
	}
	mux := map[string]any{
		"enabled": node.Multiplex.Enabled,
	}
	if node.Multiplex.Protocol != "" {
		mux["protocol"] = node.Multiplex.Protocol
	}
	if node.Multiplex.MaxConnections != 0 {
		mux["max_connections"] = node.Multiplex.MaxConnections
	}
	if node.Multiplex.MinStreams != 0 {
		mux["min_streams"] = node.Multiplex.MinStreams
	}
	if node.Multiplex.MaxStreams != 0 {
		mux["max_streams"] = node.Multiplex.MaxStreams
	}
	if node.Multiplex.Padding {
		mux["padding"] = true
	}
	out["multiplex"] = mux
}

func applyUDPOverTCP(out map[string]any, node domain.NodeIR) {
	if node.UDPOverTCP == nil {
		return
	}
	if node.UDPOverTCP.Version == 0 {
		out["udp_over_tcp"] = node.UDPOverTCP.Enabled
		return
	}
	out["udp_over_tcp"] = map[string]any{
		"enabled": node.UDPOverTCP.Enabled,
		"version": node.UDPOverTCP.Version,
	}
}
