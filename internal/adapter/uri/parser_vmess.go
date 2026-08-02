package uri

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

var errVMessZeroPort = errors.New("zero vmess port")
var vmessAEADUUIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func parseVMess(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	payload := strings.TrimPrefix(raw, "vmess://")
	if strings.Contains(payload, "@") {
		return parseVMessAEAD(raw)
	}
	return parseLegacyVMess(raw)
}

func parseVMessAEAD(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeVMess, SourceFormat: "uri"}
	source := shared.SourceInfo("vmess", shared.SourceRefs("vmess-aead"))
	u, err := url.Parse(raw)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse vmess AEAD URI", err)
	}
	if u.Opaque != "" || u.Path != "" || u.RawPath != "" {
		return node, source, domain.NewError(domain.CodeParseFailed, "vmess AEAD URL path is not allowed")
	}
	if u.User == nil || u.User.Username() == "" {
		return node, source, domain.NewError(domain.CodeParseFailed, "missing vmess uuid")
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		return node, source, domain.NewError(domain.CodeParseFailed, "vmess AEAD userinfo must contain only a uuid")
	}
	if !vmessAEADUUIDPattern.MatchString(u.User.Username()) {
		return node, source, domain.NewError(domain.CodeParseFailed, "invalid vmess uuid")
	}
	host, port, err := shared.ParseURLHostPort(u, "")
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse vmess AEAD server", err)
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse vmess AEAD query", err)
	}
	for _, key := range sortedQueryKeys(values) {
		if len(values[key]) > 1 {
			return node, source, domain.NewError(
				domain.CodeParseFailed,
				fmt.Sprintf("duplicate vmess AEAD query parameter %q", key),
			)
		}
	}

	node.Name = shared.DecodeName(u.Fragment, host)
	node.Server = host
	node.Port = port
	node.UUID = u.User.Username()
	node.Cipher = firstNonEmpty(values.Get("encryption"), "auto")
	node.PacketEncoding = shared.QueryFirst(values, "packetEncoding", "packet-encoding")
	applyTLSQuery(&node, values)
	xhttpExtraComplete := applyTransportQuery(&node, values)
	node.Raw = map[string]json.RawMessage{}
	known := vmessAEADKnownQueryFields(node, values, xhttpExtraComplete)
	preserveURIQuery(&node, values, known)
	return node, source, nil
}

func vmessAEADKnownQueryFields(node domain.NodeIR, values url.Values, xhttpExtraComplete bool) map[string]bool {
	known := map[string]bool{}
	if values.Get("encryption") != "" {
		known["encryption"] = true
	}
	if key, value := vmessAEADFirstQueryField(values, "packetEncoding", "packet-encoding"); value != "" && node.PacketEncoding == value {
		known[key] = true
	}
	if tls := node.TLS; tls != nil {
		if key, value := vmessAEADFirstQueryField(values, "security", "tls"); vmessAEADTLSModeIsTyped(value) && tls.Enabled {
			known[key] = true
		}
		if key, value := vmessAEADFirstQueryField(values, "sni", "servername", "serverName"); value != "" && tls.ServerName == value {
			known[key] = true
		}
		if key, value := vmessAEADFirstQueryField(values, "allowInsecure", "allow_insecure", "allow-insecure", "skip-cert-verify", "insecure"); vmessAEADBoolQueryIsValid(value) {
			known[key] = true
		}
		if value := values.Get("disable_sni"); vmessAEADBoolQueryIsValid(value) {
			known["disable_sni"] = true
		}
		if values.Get("alpn") != "" && len(tls.ALPN) > 0 {
			known["alpn"] = true
		}
		if value := values.Get("fp"); value != "" && tls.ClientFingerprint == value {
			known["fp"] = true
		}
		if key, value := vmessAEADFirstQueryField(values, "fingerprint", "pinSHA256", "pcs"); value != "" && tls.Fingerprint == value {
			known[key] = true
		}
		if reality := tls.Reality; reality != nil {
			if key, value := vmessAEADFirstQueryField(values, "pbk", "public-key"); value != "" && reality.PublicKey == value {
				known[key] = true
			}
			if key, value := vmessAEADFirstQueryField(values, "sid", "short-id"); value != "" && reality.ShortID == value {
				known[key] = true
			}
		}
		if tls.ECH != nil {
			if values.Get("ech") != "" {
				known["ech"] = true
			}
			if values.Get("echForceQuery") != "" {
				known["echForceQuery"] = true
			}
		}
	}
	if transport := node.Transport; transport != nil {
		if key, value := vmessAEADFirstQueryField(values, "type", "net", "transport"); value != "" {
			known[key] = true
		}
		if key, value := vmessAEADFirstQueryField(values, "path", "wspath", "wsPath", "ws-path", "obfs-uri"); value != "" && transport.Path == value {
			known[key] = true
		}
		if key, value := vmessAEADFirstQueryField(values, "host", "authority", "wsHost", "ws-host", "requestHost", "http-host"); value != "" && (transport.Host == value || transport.Headers["Host"] == value) {
			known[key] = true
		}
		if key, value := vmessAEADFirstQueryField(values, "serviceName", "service_name"); value != "" && transport.ServiceName == value {
			known[key] = true
		}
		if transport.XHTTP != nil {
			if value := values.Get("mode"); value != "" && transport.XHTTP.Mode == value {
				known["mode"] = true
			}
			if xhttpExtraComplete && values.Get("extra") != "" {
				known["extra"] = true
			}
		}
	}
	return known
}

func vmessAEADFirstQueryField(values url.Values, keys ...string) (string, string) {
	for _, key := range keys {
		if value := values.Get(key); value != "" {
			return key, value
		}
	}
	return "", ""
}

func vmessAEADTLSModeIsTyped(value string) bool {
	switch strings.ToLower(value) {
	case "tls", "reality", "true":
		return true
	default:
		return false
	}
}

func vmessAEADBoolQueryIsValid(value string) bool {
	switch strings.ToLower(value) {
	case "true", "1", "yes", "y", "on", "false", "0", "no", "n", "off":
		return true
	default:
		return false
	}
}

func parseLegacyVMess(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeVMess, SourceFormat: "uri"}
	source := shared.SourceInfo("vmess", shared.SourceRefs("vmess"))
	payload := strings.TrimPrefix(raw, "vmess://")
	decoded, ok := decodeBase64Bytes(payload)
	if !ok {
		return node, source, domain.NewError(domain.CodeParseFailed, "decode vmess payload")
	}
	var doc map[string]any
	if err := json.Unmarshal(decoded, &doc); err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "unmarshal vmess json", err)
	}
	node.Name = firstNonEmpty(shared.StringValue(doc["ps"]), shared.StringValue(doc["name"]), shared.StringValue(doc["remarks"]), shared.StringValue(doc["add"]), "vmess")
	node.Server = shared.StringValue(doc["add"])
	if node.Server == "" {
		return node, source, domain.NewError(domain.CodeParseFailed, "missing vmess server")
	}
	portValue := shared.StringValue(doc["port"])
	port, err := shared.Uint16Value(doc["port"])
	if err != nil || port == 0 {
		if err == nil && portValue == "0" {
			return node, source, domain.WrapError(domain.CodeParseFailed, "invalid vmess port", errVMessZeroPort)
		}
		return node, source, domain.NewError(domain.CodeParseFailed, "invalid vmess port")
	}
	node.Port = port
	node.UUID = firstNonEmpty(shared.StringValue(doc["id"]), shared.StringValue(doc["uuid"]))
	if node.UUID == "" {
		return node, source, domain.NewError(domain.CodeParseFailed, "missing vmess uuid")
	}
	node.Cipher = firstNonEmpty(shared.StringValue(doc["scy"]), shared.StringValue(doc["security"]), shared.StringValue(doc["cipher"]), "auto")
	if alterID, err := shared.IntValue(firstNonEmpty(shared.StringValue(doc["aid"]), shared.StringValue(doc["alterId"]), shared.StringValue(doc["alter_id"]))); err == nil {
		node.AlterID = alterID
	}
	node.PacketEncoding = shared.StringValue(doc["packetEncoding"])
	if node.PacketEncoding == "" {
		node.PacketEncoding = shared.StringValue(doc["packet-encoding"])
	}
	node.TLS = &domain.TLSOptions{}
	if tlsVal := firstNonEmpty(shared.StringValue(doc["tls"]), shared.StringValue(doc["streamSecurity"])); tlsVal != "" && tlsVal != "none" {
		node.TLS.Enabled = true
	}
	if sni := firstNonEmpty(shared.StringValue(doc["sni"]), shared.StringValue(doc["servername"]), shared.StringValue(doc["serverName"])); sni != "" {
		node.TLS.ServerName = sni
	}
	if insecure := firstNonEmpty(
		shared.StringValue(doc["allowInsecure"]),
		shared.StringValue(doc["allow_insecure"]),
		shared.StringValue(doc["allow-insecure"]),
		shared.StringValue(doc["skip-cert-verify"]),
		shared.StringValue(doc["insecure"]),
	); insecure != "" {
		node.TLS.InsecureSkipVerify = shared.BoolValue(insecure)
	}
	if fp := shared.StringValue(doc["fp"]); fp != "" {
		node.TLS.ClientFingerprint = fp
	}
	if pcs := firstNonEmpty(shared.StringValue(doc["pcs"]), shared.StringValue(doc["fingerprint"]), shared.StringValue(doc["pinSHA256"])); pcs != "" {
		node.TLS.Fingerprint = pcs
	}
	alpn, alpnKnown := parseLegacyVMessALPN(doc["alpn"])
	if alpnKnown {
		node.TLS.ALPN = alpn
	}
	if node.TLS.Enabled || node.TLS.ServerName != "" || node.TLS.InsecureSkipVerify || len(node.TLS.ALPN) > 0 || node.TLS.ClientFingerprint != "" || node.TLS.Fingerprint != "" {
		// keep populated
	} else {
		node.TLS = nil
	}
	network := firstNonEmpty(shared.StringValue(doc["net"]), shared.StringValue(doc["network"]))
	host := firstNonEmpty(
		shared.StringValue(doc["host"]),
		shared.StringValue(doc["wsHost"]),
		shared.StringValue(doc["requestHost"]),
		shared.StringValue(doc["ws-host"]),
		shared.StringValue(doc["http-host"]),
	)
	path := firstNonEmpty(
		shared.StringValue(doc["path"]),
		shared.StringValue(doc["wsPath"]),
		shared.StringValue(doc["wspath"]),
		shared.StringValue(doc["ws-path"]),
		shared.StringValue(doc["obfs-uri"]),
	)
	serviceName := firstNonEmpty(shared.StringValue(doc["serviceName"]), shared.StringValue(doc["service_name"]))
	if host != "" || path != "" || serviceName != "" || network != "" {
		node.Transport = &domain.TransportOptions{
			Type:        network,
			Host:        host,
			Path:        path,
			ServiceName: serviceName,
		}
		normalizeTransport(node.Transport)
		if node.Transport.Type == "grpc" && node.Transport.ServiceName == "" && node.Transport.Path != "" {
			node.Transport.ServiceName = node.Transport.Path
			node.Transport.Path = ""
		}
	}
	node.Raw = map[string]json.RawMessage{}
	preserveVMessHeaderTypeRaw(node.Raw, doc, "type")
	preserveVMessHeaderTypeRaw(node.Raw, doc, "headerType")
	knownFields := map[string]bool{
		"v": true, "ps": true, "name": true, "remarks": true, "add": true, "port": true,
		"id": true, "uuid": true, "aid": true, "alterId": true, "alter_id": true,
		"scy": true, "security": true, "cipher": true,
		"net": true, "network": true, "type": true, "headerType": true,
		"host": true, "wsHost": true, "requestHost": true, "ws-host": true, "http-host": true,
		"path": true, "wsPath": true, "wspath": true, "ws-path": true, "obfs-uri": true,
		"tls": true, "streamSecurity": true, "sni": true, "servername": true, "serverName": true,
		"allowInsecure": true, "allow_insecure": true, "allow-insecure": true, "skip-cert-verify": true, "insecure": true,
		"fp": true, "fingerprint": true, "pinSHA256": true, "pcs": true,
		"serviceName": true, "service_name": true,
		"packetEncoding": true, "packet-encoding": true,
	}
	if alpnKnown {
		knownFields["alpn"] = true
	}
	shared.AddUnknownRaw(node.Raw, "vmess.", doc, knownFields)
	return node, source, nil
}

func parseLegacyVMessALPN(value any) ([]string, bool) {
	var values []string
	switch typed := value.(type) {
	case string:
		values = []string{typed}
	case []any:
		values = make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			values = append(values, text)
		}
	default:
		return nil, false
	}
	alpn := make([]string, 0, len(values))
	for _, value := range values {
		alpn = append(alpn, splitList(value)...)
	}
	return alpn, true
}

func preserveVMessHeaderTypeRaw(raw map[string]json.RawMessage, doc map[string]any, key string) {
	value := shared.StringValue(doc[key])
	if value == "" || value == "auto" || value == "none" {
		return
	}
	raw["vmess."+key] = shared.RawNumberOrString(value)
}
