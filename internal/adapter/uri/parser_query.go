package uri

import (
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func applyTLSQuery(node *domain.NodeIR, values url.Values) {
	security := strings.ToLower(shared.QueryFirst(values, "security", "tls"))
	if security == "tls" || security == "reality" || security == "true" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.Enabled = true
	}
	if sni := shared.QueryFirst(values, "sni", "servername", "serverName"); sni != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.ServerName = sni
	}
	if insecure := shared.QueryFirst(values, "allowInsecure", "allow_insecure", "allow-insecure", "skip-cert-verify", "insecure"); insecure != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.InsecureSkipVerify = shared.BoolValue(insecure)
	}
	if alpn := values.Get("alpn"); alpn != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.ALPN = splitList(alpn)
	}
	if fp := values.Get("fp"); fp != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.ClientFingerprint = fp
	}
	if fingerprint := shared.QueryFirst(values, "fingerprint", "pinSHA256", "pcs"); fingerprint != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.Fingerprint = fingerprint
	}
	if security == "reality" || values.Get("pbk") != "" || values.Get("public-key") != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{Enabled: true}
		}
		node.TLS.Enabled = true
		node.TLS.Reality = &domain.RealityOptions{
			Enabled:   true,
			PublicKey: shared.QueryFirst(values, "pbk", "public-key"),
			ShortID:   shared.QueryFirst(values, "sid", "short-id"),
		}
	}
	if ech := parseECHQuery(values.Get("ech"), values.Get("echForceQuery")); ech != nil {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{Enabled: true}
		}
		node.TLS.Enabled = true
		node.TLS.ECH = ech
	}
}

func applyHysteriaParseQueryTLS(node *domain.NodeIR, values url.Values) {
	if peer := values.Get("peer"); peer != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.ServerName = peer
	}
}

func applyHysteria2ParseQueryTLS(node *domain.NodeIR, values url.Values) {
	if peer := values.Get("peer"); peer != "" && (node.TLS == nil || node.TLS.ServerName == "") {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.ServerName = peer
	}
	if pin := values.Get("pinSHA256"); pin != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.Fingerprint = pin
	}
}

func applyTransportQuery(node *domain.NodeIR, values url.Values) {
	typ := shared.QueryFirst(values, "type", "net", "transport")
	if typ == "" {
		return
	}
	node.Transport = &domain.TransportOptions{
		Type:        typ,
		Path:        shared.QueryFirst(values, "path", "wspath", "wsPath", "ws-path", "obfs-uri"),
		Host:        shared.QueryFirst(values, "host", "authority", "wsHost", "ws-host", "requestHost", "http-host"),
		ServiceName: shared.QueryFirst(values, "serviceName", "service_name"),
	}
	normalizeTransport(node.Transport)
	if node.Transport.Type == "tcp" && tcpHeaderTypeIsDefault(values) {
		node.Transport.Host = ""
	}
	if node.Transport.Host != "" {
		node.Transport.Headers = map[string]string{"Host": node.Transport.Host}
	}
	if node.Type == domain.NodeTypeVLESS {
		applyVLESSXHTTPExtra(node.Transport, values)
	}
}

func isTelegramProxyWebURL(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	if host != "t.me" && host != "telegram.me" {
		return false
	}
	path := strings.TrimPrefix(u.Path, "/")
	if path != "socks" && path != "http" && path != "proxy" {
		return false
	}
	values := u.Query()
	return shared.QueryFirst(values, "server", "host") != "" || values.Get("port") != "" || values.Get("secret") != ""
}

func normalizeTransport(transport *domain.TransportOptions) {
	if transport == nil {
		return
	}
	switch strings.ToLower(transport.Type) {
	case "ws":
		transport.Type = "websocket"
	case "h2":
		transport.Type = "http"
	}
}

func preserveURIQuery(node *domain.NodeIR, values url.Values, known map[string]bool) {
	for _, key := range sortedQueryKeys(values) {
		if known[key] {
			continue
		}
		if node.Raw == nil {
			node.Raw = map[string]json.RawMessage{}
		}
		node.Raw["uri.query."+key] = json.RawMessage(strconv.Quote(values.Get(key)))
	}
}

func sortedQueryKeys(values url.Values) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func tcpHeaderTypeIsDefault(values url.Values) bool {
	raw, ok := values["headerType"]
	if !ok {
		return true
	}
	for _, value := range raw {
		if strings.ToLower(strings.TrimSpace(value)) != "none" {
			return false
		}
	}
	return true
}

func queryValuesAreEmpty(values url.Values, key string) bool {
	raw, ok := values[key]
	if !ok {
		return false
	}
	for _, value := range raw {
		if value != "" {
			return false
		}
	}
	return true
}

func queryValuesAreNoopHeaderType(values url.Values, key string) bool {
	raw, ok := values[key]
	if !ok {
		return false
	}
	for _, value := range raw {
		if strings.ToLower(strings.TrimSpace(value)) != "none" {
			return false
		}
	}
	return true
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
