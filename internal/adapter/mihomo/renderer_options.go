package mihomo

import (
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func baseProxy(node domain.NodeIR, typ string) map[string]any {
	out := map[string]any{
		"name":   node.Name,
		"type":   typ,
		"server": node.Server,
		"port":   int(node.Port),
	}
	applyMihomoDialer(out, node)
	return out
}

func applyMihomoDialer(out map[string]any, node domain.NodeIR) {
	if node.Dialer == nil {
		return
	}
	if node.Dialer.UDPRelay != nil && mihomoSupportsUDPRelay(node.Type) {
		out["udp"] = *node.Dialer.UDPRelay
	}
	if !node.Dialer.TFO {
		return
	}
	switch node.Type {
	case domain.NodeTypeHysteria, domain.NodeTypeTUIC:
		out["fast-open"] = true
	default:
		out["tfo"] = true
	}
}

func mihomoSupportsUDPRelay(nodeType domain.NodeType) bool {
	switch nodeType {
	case domain.NodeTypeShadowsocks, domain.NodeTypeShadowsocksR, domain.NodeTypeSnell, domain.NodeTypeVMess, domain.NodeTypeVLESS, domain.NodeTypeTrojan, domain.NodeTypeMieru, domain.NodeTypeSOCKS, domain.NodeTypeWireGuard:
		return true
	default:
		return false
	}
}

func applyMihomoTLS(out map[string]any, node domain.NodeIR, sniKey string) {
	if node.TLS == nil {
		return
	}
	if node.TLS.Enabled {
		out["tls"] = true
	}
	if sniKey != "" && node.TLS.ServerName != "" {
		out[sniKey] = node.TLS.ServerName
	}
	if node.TLS.InsecureSkipVerify {
		out["skip-cert-verify"] = true
	}
	if len(node.TLS.ALPN) > 0 {
		out["alpn"] = node.TLS.ALPN
	}
	if node.TLS.ClientFingerprint != "" {
		out["client-fingerprint"] = node.TLS.ClientFingerprint
	}
	if node.TLS.Fingerprint != "" {
		out["fingerprint"] = node.TLS.Fingerprint
	}
	if node.TLS.ECH != nil {
		ech := map[string]any{}
		if node.TLS.ECH.Enabled {
			ech["enable"] = true
		}
		if len(node.TLS.ECH.Config) > 0 {
			ech["config"] = node.TLS.ECH.Config[0]
		}
		if node.TLS.ECH.QueryServerName != "" {
			ech["query-server-name"] = node.TLS.ECH.QueryServerName
		}
		out["ech-opts"] = ech
	}
	if node.TLS.Reality != nil {
		reality := map[string]any{}
		if node.TLS.Reality.PublicKey != "" {
			reality["public-key"] = node.TLS.Reality.PublicKey
		}
		if node.TLS.Reality.ShortID != "" {
			reality["short-id"] = node.TLS.Reality.ShortID
		}
		out["reality-opts"] = reality
	}
}

func applyMihomoTransport(out map[string]any, node domain.NodeIR) {
	if node.Transport == nil || node.Transport.Type == "" {
		return
	}
	if mihomoSupportsHTTPHeaderTransport(node.Type, node.Transport) {
		out["network"] = "http"
		path := node.Transport.Path
		if path == "" {
			path = "/"
		}
		opts := map[string]any{"path": []string{path}}
		if node.Transport.Method != "" {
			opts["method"] = node.Transport.Method
		}
		headers := node.Transport.Headers
		if len(headers) == 0 && node.Transport.Host != "" {
			headers = map[string]string{"Host": node.Transport.Host}
		}
		if len(headers) > 0 {
			opts["headers"] = mapStringToStringList(headers)
		}
		out["http-opts"] = opts
		return
	}
	if !mihomoSupportsTransport(node.Type, node.Transport.Type) {
		return
	}
	switch node.Transport.Type {
	case "websocket", "ws":
		out["network"] = "ws"
		opts := map[string]any{}
		if node.Transport.Path != "" {
			opts["path"] = node.Transport.Path
		}
		headers := node.Transport.Headers
		if len(headers) == 0 && node.Transport.Host != "" {
			headers = map[string]string{"Host": node.Transport.Host}
		}
		if len(headers) > 0 {
			opts["headers"] = headers
		}
		if node.Transport.MaxEarlyData != 0 {
			opts["max-early-data"] = node.Transport.MaxEarlyData
		}
		if node.Transport.EarlyDataHeaderName != "" {
			opts["early-data-header-name"] = node.Transport.EarlyDataHeaderName
		}
		if node.Transport.V2RayHTTPUpgrade {
			opts["v2ray-http-upgrade"] = true
		}
		if node.Transport.V2RayHTTPUpgradeFastOpen {
			opts["v2ray-http-upgrade-fast-open"] = true
		}
		out["ws-opts"] = opts
	case "grpc":
		out["network"] = "grpc"
		opts := map[string]any{}
		if node.Transport.ServiceName != "" {
			opts["grpc-service-name"] = node.Transport.ServiceName
		}
		if node.Multiplex != nil {
			if node.Multiplex.MaxConnections != 0 {
				opts["max-connections"] = node.Multiplex.MaxConnections
			}
			if node.Multiplex.MinStreams != 0 {
				opts["min-streams"] = node.Multiplex.MinStreams
			}
			if node.Multiplex.MaxStreams != 0 {
				opts["max-streams"] = node.Multiplex.MaxStreams
			}
		}
		out["grpc-opts"] = opts
	case "http":
		out["network"] = "h2"
		opts := map[string]any{}
		if node.Transport.Host != "" {
			opts["host"] = []string{node.Transport.Host}
		} else if len(node.Transport.Hosts) > 0 {
			opts["host"] = node.Transport.Hosts
		}
		if node.Transport.Path != "" {
			opts["path"] = node.Transport.Path
		}
		out["h2-opts"] = opts
	case "httpupgrade":
		out["network"] = "http"
		opts := map[string]any{}
		if node.Transport.Path != "" {
			opts["path"] = []string{node.Transport.Path}
		}
		if node.Transport.Method != "" {
			opts["method"] = node.Transport.Method
		}
		if len(node.Transport.Headers) > 0 {
			opts["headers"] = mapStringToStringList(node.Transport.Headers)
		}
		out["http-opts"] = opts
	case "xhttp":
		out["network"] = "xhttp"
		opts := map[string]any{}
		if node.Transport.XHTTP != nil {
			if node.Transport.XHTTP.Mode != "" {
				opts["mode"] = node.Transport.XHTTP.Mode
			}
			if reuse := renderMihomoXHTTPReuseSettings(node.Transport.XHTTP.ReuseSettings); reuse != nil {
				opts["reuse-settings"] = reuse
			}
			if download := renderMihomoXHTTPDownloadSettings(node.Transport.XHTTP.DownloadSettings); download != nil {
				opts["download-settings"] = download
			}
		}
		if node.Transport.Path != "" {
			opts["path"] = node.Transport.Path
		}
		if node.Transport.Host != "" {
			opts["host"] = node.Transport.Host
		}
		if len(node.Transport.Headers) > 0 {
			opts["headers"] = node.Transport.Headers
		}
		out["xhttp-opts"] = opts
	default:
		out["network"] = node.Transport.Type
	}
}

func mihomoSupportsTransport(nodeType domain.NodeType, transportType string) bool {
	switch nodeType {
	case domain.NodeTypeVMess:
		switch transportType {
		case "websocket", "ws", "grpc", "http", "httpupgrade":
			return true
		}
	case domain.NodeTypeVLESS:
		switch transportType {
		case "websocket", "ws", "grpc", "http", "httpupgrade", "xhttp":
			return true
		}
	case domain.NodeTypeTrojan:
		switch transportType {
		case "websocket", "ws", "grpc":
			return true
		}
	}
	return false
}

func isDefaultTCPTransport(transport *domain.TransportOptions) bool {
	if transport == nil || strings.TrimSpace(strings.ToLower(transport.Type)) != "tcp" {
		return false
	}
	return transport.HeaderType == "" &&
		transport.Method == "" &&
		transport.Path == "" &&
		transport.Host == "" &&
		len(transport.Hosts) == 0 &&
		len(transport.Headers) == 0 &&
		transport.ServiceName == "" &&
		transport.MaxEarlyData == 0 &&
		transport.EarlyDataHeaderName == "" &&
		!transport.V2RayHTTPUpgrade &&
		!transport.V2RayHTTPUpgradeFastOpen &&
		transport.XHTTP == nil
}

func mihomoSupportsHTTPHeaderTransport(nodeType domain.NodeType, transport *domain.TransportOptions) bool {
	return (nodeType == domain.NodeTypeVMess || nodeType == domain.NodeTypeVLESS) && isHTTPHeaderTransport(transport)
}

func isHTTPHeaderTransport(transport *domain.TransportOptions) bool {
	return transport != nil && strings.EqualFold(strings.TrimSpace(transport.Type), "tcp") &&
		strings.EqualFold(strings.TrimSpace(transport.HeaderType), "http")
}

func renderMihomoXHTTPReuseSettings(settings *domain.XHTTPReuseSettings) map[string]any {
	if settings == nil {
		return nil
	}
	out := map[string]any{}
	if settings.MaxConcurrency != "" {
		out["max-concurrency"] = settings.MaxConcurrency
	}
	if settings.MaxConnections != "" {
		out["max-connections"] = settings.MaxConnections
	}
	if settings.CMaxReuseTimes != "" {
		out["c-max-reuse-times"] = settings.CMaxReuseTimes
	}
	if settings.HMaxRequestTimes != "" {
		out["h-max-request-times"] = settings.HMaxRequestTimes
	}
	if settings.HMaxReusableSecs != "" {
		out["h-max-reusable-secs"] = settings.HMaxReusableSecs
	}
	if settings.HKeepAlivePeriod != 0 {
		out["h-keep-alive-period"] = settings.HKeepAlivePeriod
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func renderMihomoXHTTPDownloadSettings(settings *domain.XHTTPDownloadSettings) map[string]any {
	if settings == nil {
		return nil
	}
	out := map[string]any{}
	if settings.Server != nil {
		out["server"] = *settings.Server
	}
	if settings.Port != nil {
		out["port"] = int(*settings.Port)
	}
	if settings.Path != nil {
		out["path"] = *settings.Path
	}
	if settings.Host != nil {
		out["host"] = *settings.Host
	}
	if settings.Headers != nil {
		out["headers"] = *settings.Headers
	}
	if reuse := renderMihomoXHTTPReuseSettings(settings.ReuseSettings); reuse != nil {
		out["reuse-settings"] = reuse
	}
	applyMihomoDownloadTLS(out, settings.TLS)
	return out
}

func applyMihomoDownloadTLS(out map[string]any, tls *domain.TLSOptions) {
	if tls == nil {
		return
	}
	out["tls"] = tls.Enabled
	if tls.ServerName != "" {
		out["servername"] = tls.ServerName
	}
	if tls.InsecureSkipVerify {
		out["skip-cert-verify"] = true
	}
	if len(tls.ALPN) > 0 {
		out["alpn"] = tls.ALPN
	}
	if tls.ClientFingerprint != "" {
		out["client-fingerprint"] = tls.ClientFingerprint
	}
	if tls.Fingerprint != "" {
		out["fingerprint"] = tls.Fingerprint
	}
	if tls.Certificate != "" {
		out["certificate"] = tls.Certificate
	}
	if tls.PrivateKey != "" {
		out["private-key"] = tls.PrivateKey
	}
	if tls.ECH != nil {
		ech := map[string]any{"enable": tls.ECH.Enabled}
		if len(tls.ECH.Config) > 0 {
			ech["config"] = tls.ECH.Config[0]
		}
		if tls.ECH.QueryServerName != "" {
			ech["query-server-name"] = tls.ECH.QueryServerName
		}
		out["ech-opts"] = ech
	}
	if tls.Reality != nil {
		out["reality-opts"] = map[string]any{
			"public-key": tls.Reality.PublicKey,
			"short-id":   tls.Reality.ShortID,
		}
	}
}

func applyMihomoUDPOverTCP(out map[string]any, node domain.NodeIR) {
	if node.UDPOverTCP == nil || !node.UDPOverTCP.Enabled {
		return
	}
	out["udp-over-tcp"] = true
	if node.UDPOverTCP.Version != 0 {
		out["udp-over-tcp-version"] = node.UDPOverTCP.Version
	}
}

func mapStringToStringList(in map[string]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for key, value := range in {
		out[key] = []string{value}
	}
	return out
}
