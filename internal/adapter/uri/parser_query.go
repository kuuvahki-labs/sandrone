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
	if node.Type == domain.NodeTypeVLESS && security == "none" {
		return
	}
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
	if insecure := shared.QueryFirst(values, "allowInsecure", "allowinsecure", "allow_insecure", "allow-insecure", "skip-cert-verify", "insecure"); insecure != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.InsecureSkipVerify = shared.BoolValue(insecure)
	}
	if disableSNI := values.Get("disable_sni"); disableSNI != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.DisableSNI = shared.BoolValue(disableSNI)
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

func applyTransportQuery(node *domain.NodeIR, values url.Values) bool {
	typ := shared.QueryFirst(values, "type", "net", "transport")
	if typ == "" {
		return false
	}
	node.Transport = &domain.TransportOptions{
		Type:        typ,
		Path:        shared.QueryFirst(values, "path", "wspath", "wsPath", "ws-path", "obfs-uri"),
		Host:        shared.QueryFirst(values, "host", "authority", "wsHost", "ws-host", "requestHost", "http-host"),
		ServiceName: shared.QueryFirst(values, "serviceName", "service_name"),
	}
	normalizeTransport(node.Transport)
	normalizeWebSocketEarlyData(node.Transport)
	if node.Transport.Type == "tcp" {
		switch {
		case queryValuesEqualFold(values, "headerType", "http"):
			node.Transport.HeaderType = "http"
			node.Transport.Method = values.Get("method")
		case tcpHeaderTypeIsDefault(values):
			node.Transport.Host = ""
			if node.Transport.Path == "/" {
				node.Transport.Path = ""
			}
		}
	}
	if node.Transport.Host != "" {
		node.Transport.Headers = map[string]string{"Host": node.Transport.Host}
	}
	if node.Type == domain.NodeTypeVMess {
		return applyVMessXHTTPExtra(node.Transport, values)
	}
	if node.Type == domain.NodeTypeVLESS {
		return applyXHTTPExtra(node.Transport, values)
	}
	return false
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
	case "splithttp":
		transport.Type = "xhttp"
	}
}

func normalizeWebSocketEarlyData(transport *domain.TransportOptions) {
	if transport.Type != "websocket" {
		return
	}
	path, rawQuery, ok := strings.Cut(transport.Path, "?")
	if !ok {
		return
	}
	rawMaxEarlyData, ok := strings.CutPrefix(rawQuery, "ed=")
	if !ok {
		return
	}
	maxEarlyData, ok := positiveDecimal(rawMaxEarlyData)
	if !ok {
		return
	}
	transport.Path = path
	transport.MaxEarlyData = maxEarlyData
	transport.EarlyDataHeaderName = "Sec-WebSocket-Protocol"
}

func applyWebSocketEarlyDataQuery(transport *domain.TransportOptions, values url.Values, known map[string]bool) {
	if transport == nil || transport.Type != "websocket" {
		return
	}
	rawEarlyData, ok := singleQueryValue(values, "ed")
	if !ok {
		return
	}
	maxEarlyData, ok := positiveDecimal(rawEarlyData)
	if !ok || transport.MaxEarlyData != 0 && transport.MaxEarlyData != maxEarlyData {
		return
	}
	headerName := "Sec-WebSocket-Protocol"
	if rawHeader, exists := values["eh"]; exists {
		if len(rawHeader) != 1 || strings.TrimSpace(rawHeader[0]) == "" {
			return
		}
		headerName = rawHeader[0]
	}
	transport.MaxEarlyData = maxEarlyData
	transport.EarlyDataHeaderName = headerName
	known["ed"] = true
	if _, exists := values["eh"]; exists {
		known["eh"] = true
	}
}

func singleQueryValue(values url.Values, key string) (string, bool) {
	raw, ok := values[key]
	if !ok || len(raw) != 1 {
		return "", false
	}
	return raw[0], true
}

func positiveDecimal(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0
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

func queryValuesAreNoopHeaderType(values url.Values) bool {
	return queryValuesEqualFold(values, "headerType", "none")
}

func queryValuesEqualFold(values url.Values, key, want string) bool {
	raw, ok := values[key]
	if !ok {
		return false
	}
	for _, value := range raw {
		if !strings.EqualFold(strings.TrimSpace(value), want) {
			return false
		}
	}
	return true
}

func preferredQueryString(values url.Values, keys ...string) (string, map[string]bool) {
	known := map[string]bool{}
	selected := ""
	for _, key := range keys {
		raw, ok := singleQueryValue(values, key)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		if selected == "" {
			selected = raw
			known[key] = true
			continue
		}
		if raw == selected {
			known[key] = true
		}
	}
	return selected, known
}

func preferredQueryBool(values url.Values, keys ...string) (bool, map[string]bool) {
	known := map[string]bool{}
	selected := false
	hasSelected := false
	for _, key := range keys {
		raw, ok := singleQueryValue(values, key)
		if !ok {
			continue
		}
		value, ok := strictQueryBool(raw)
		if !ok {
			continue
		}
		if !hasSelected {
			selected = value
			hasSelected = true
			known[key] = true
			continue
		}
		if value == selected {
			known[key] = true
		}
	}
	return selected, known
}

func strictQueryBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func mergeKnownQueryKeys(known map[string]bool, additions map[string]bool) {
	for key := range additions {
		known[key] = true
	}
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
