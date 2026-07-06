package uri

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

var errVMessZeroPort = errors.New("zero vmess port")

func parseVMess(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
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
	if node.TLS.Enabled || node.TLS.ServerName != "" || node.TLS.InsecureSkipVerify || node.TLS.ClientFingerprint != "" || node.TLS.Fingerprint != "" {
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
	shared.AddUnknownRaw(node.Raw, "vmess.", doc, map[string]bool{
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
	})
	return node, source, nil
}

func preserveVMessHeaderTypeRaw(raw map[string]json.RawMessage, doc map[string]any, key string) {
	value := shared.StringValue(doc[key])
	if value == "" || value == "auto" || value == "none" {
		return
	}
	raw["vmess."+key] = shared.RawNumberOrString(value)
}
