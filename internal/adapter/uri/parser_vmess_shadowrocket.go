package uri

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"uuid"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func parseVMess1(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	return parseVMess("vmess://" + strings.TrimPrefix(raw, "vmess1://"))
}

func parseVLESSCompat(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	if node, source, matched, err := parseShadowrocketVLESS(raw); matched {
		return node, source, err
	}
	node, source, err := parseVLESS(raw)
	if err == nil {
		node.PacketEncoding = shared.NormalizePacketEncoding(node.PacketEncoding)
	}
	return node, source, err
}

func parseShadowrocketVMess(raw string) (domain.NodeIR, *domain.SourceInfo, bool, error) {
	node := domain.NodeIR{Type: domain.NodeTypeVMess, SourceFormat: "uri"}
	source := shared.SourceInfo("vmess", shared.SourceRefs("vmess-aead"))
	payload, rawQuery, fragment := splitShadowrocketURI(raw, "vmess")
	cipher, id, host, port, matched, err := parseShadowrocketAuthority(payload, "")
	if !matched {
		return node, source, false, nil
	}
	if err != nil {
		return node, source, true, domain.WrapError(domain.CodeParseFailed, "parse Shadowrocket vmess authority", err)
	}
	values, err := parseShadowrocketQuery(rawQuery)
	if err != nil {
		return node, source, true, err
	}
	node.Name = shadowrocketName(values, fragment, host)
	node.Server = host
	node.Port = port
	node.UUID = id
	node.Cipher = cipher
	node.PacketEncoding = shared.NormalizePacketEncoding(shared.QueryFirst(values, "packetEncoding", "packet-encoding"))
	known := map[string]bool{}
	if err := applyShadowrocketTLS(&node, values, known); err != nil {
		return node, source, true, err
	}
	applyShadowrocketTransport(&node, values, known)
	if err := markShadowrocketCommonKnown(&node, values, known); err != nil {
		return node, source, true, err
	}
	node.Raw = map[string]json.RawMessage{}
	preserveURIQuery(&node, values, known)
	return node, source, true, nil
}

func parseShadowrocketVLESS(raw string) (domain.NodeIR, *domain.SourceInfo, bool, error) {
	node := domain.NodeIR{Type: domain.NodeTypeVLESS, SourceFormat: "uri"}
	source := shared.SourceInfo("vless", shared.SourceRefs("vless"))
	payload, rawQuery, fragment := splitShadowrocketURI(raw, "vless")
	_, id, host, port, matched, err := parseShadowrocketAuthority(payload, "none")
	if !matched {
		return node, source, false, nil
	}
	if err != nil {
		return node, source, true, domain.WrapError(domain.CodeParseFailed, "parse Shadowrocket vless authority", err)
	}
	values, err := parseShadowrocketQuery(rawQuery)
	if err != nil {
		return node, source, true, err
	}
	node.Name = shadowrocketName(values, fragment, host)
	node.Server = host
	node.Port = port
	node.UUID = id
	node.Encryption = "none"
	node.PacketEncoding = shared.NormalizePacketEncoding(shared.QueryFirst(values, "packetEncoding", "packet-encoding"))
	known := map[string]bool{}
	if err := applyShadowrocketTLS(&node, values, known); err != nil {
		return node, source, true, err
	}
	applyShadowrocketTransport(&node, values, known)
	if xtls, ok := singleQueryValue(values, "xtls"); ok && xtls == "2" {
		node.Flow = "xtls-rprx-vision"
		known["xtls"] = true
	}
	if err := markShadowrocketCommonKnown(&node, values, known); err != nil {
		return node, source, true, err
	}
	node.Raw = map[string]json.RawMessage{}
	preserveURIQuery(&node, values, known)
	return node, source, true, nil
}

func splitShadowrocketURI(raw, scheme string) (string, string, string) {
	prefix := scheme + "://"
	rest, ok := strings.CutPrefix(raw, prefix)
	if !ok {
		return "", "", ""
	}
	rest, fragment, _ := strings.Cut(rest, "#")
	payload, rawQuery, _ := strings.Cut(rest, "?")
	return payload, rawQuery, fragment
}

func parseShadowrocketQuery(rawQuery string) (url.Values, error) {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, domain.WrapError(domain.CodeParseFailed, "parse Shadowrocket URI query", err)
	}
	for key, items := range values {
		if len(items) != 1 {
			return nil, domain.NewError(domain.CodeParseFailed, fmt.Sprintf("duplicate Shadowrocket query parameter %q", key))
		}
	}
	return values, nil
}

func parseShadowrocketAuthority(payload, requiredCipher string) (string, string, string, uint16, bool, error) {
	decoded, ok := decodeBase64String(payload)
	if !ok || strings.HasPrefix(strings.TrimSpace(decoded), "{") {
		return "", "", "", 0, false, nil
	}
	userinfo, authority, hasAuthority := strings.Cut(decoded, "@")
	cipher, id, hasCredentials := strings.Cut(userinfo, ":")
	if !hasAuthority || !hasCredentials {
		return "", "", "", 0, false, nil
	}
	if strings.Contains(authority, "@") || strings.Contains(id, ":") {
		return "", "", "", 0, true, domain.NewError(domain.CodeParseFailed, "invalid encoded credentials")
	}
	if !shadowrocketCipherToken(cipher) || requiredCipher != "" && cipher != requiredCipher {
		return "", "", "", 0, true, domain.NewError(domain.CodeParseFailed, "unexpected encoded cipher")
	}
	if _, err := uuid.Parse(id); err != nil {
		return "", "", "", 0, true, domain.NewError(domain.CodeParseFailed, "invalid encoded uuid")
	}
	u, err := url.Parse("compat://" + authority)
	if err != nil || u.User != nil || u.Opaque != "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", "", "", 0, true, domain.NewError(domain.CodeParseFailed, "invalid encoded server authority")
	}
	host, port, err := shared.ParseURLHostPort(u, "")
	if err != nil {
		return "", "", "", 0, true, err
	}
	return cipher, id, host, port, true, nil
}

func shadowrocketCipherToken(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func shadowrocketName(values url.Values, fragment, fallback string) string {
	if remarks := values.Get("remarks"); remarks != "" {
		return remarks
	}
	return shared.DecodeName(fragment, fallback)
}

func applyShadowrocketTLS(node *domain.NodeIR, values url.Values, known map[string]bool) error {
	if raw, ok := singleQueryValue(values, "tls"); ok {
		switch raw {
		case "1":
			node.TLS = &domain.TLSOptions{Enabled: true}
			known["tls"] = true
		case "0":
			known["tls"] = true
		default:
			return domain.NewError(domain.CodeParseFailed, "Shadowrocket tls must be 0 or 1")
		}
	}
	if peer := values.Get("peer"); peer != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.ServerName = peer
		known["peer"] = true
	}
	if insecure := shared.QueryFirst(values, "allowInsecure", "allowinsecure"); insecure != "" {
		if !vmessAEADBoolQueryIsValid(insecure) {
			return domain.NewError(domain.CodeParseFailed, "invalid Shadowrocket allowInsecure value")
		}
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.InsecureSkipVerify = shared.BoolValue(insecure)
		if values.Get("allowInsecure") != "" {
			known["allowInsecure"] = true
		} else {
			known["allowinsecure"] = true
		}
	}
	if fp := values.Get("fp"); fp != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.ClientFingerprint = fp
		known["fp"] = true
	}
	if alpn := values.Get("alpn"); alpn != "" {
		if node.TLS == nil {
			node.TLS = &domain.TLSOptions{}
		}
		node.TLS.ALPN = splitList(alpn)
		known["alpn"] = true
	}
	return nil
}

func applyShadowrocketTransport(node *domain.NodeIR, values url.Values, known map[string]bool) {
	obfs := strings.ToLower(strings.TrimSpace(values.Get("obfs")))
	if obfs != "websocket" && obfs != "ws" {
		return
	}
	transport := &domain.TransportOptions{
		Type: "websocket",
		Path: values.Get("path"),
		Host: values.Get("obfsParam"),
	}
	if transport.Host != "" {
		transport.Headers = map[string]string{"Host": transport.Host}
		known["obfsParam"] = true
	}
	if transport.Path != "" {
		known["path"] = true
	}
	node.Transport = transport
	known["obfs"] = true
}

func markShadowrocketCommonKnown(node *domain.NodeIR, values url.Values, known map[string]bool) error {
	if values.Get("remarks") != "" {
		known["remarks"] = true
	}
	if key, value := vmessAEADFirstQueryField(values, "packetEncoding", "packet-encoding"); value != "" && node.PacketEncoding == shared.NormalizePacketEncoding(value) {
		known[key] = true
	}
	if node.Type == domain.NodeTypeVMess {
		alterIDSet := false
		for _, key := range []string{"aid", "alterId"} {
			if raw, ok := singleQueryValue(values, key); ok {
				value, err := strconv.Atoi(raw)
				if err != nil || value < 0 {
					return domain.NewError(domain.CodeParseFailed, "invalid Shadowrocket alterId")
				}
				if alterIDSet && node.AlterID != value {
					return domain.NewError(domain.CodeParseFailed, "conflicting Shadowrocket alterId aliases")
				}
				node.AlterID = value
				alterIDSet = true
				known[key] = true
			}
		}
	}
	return nil
}
