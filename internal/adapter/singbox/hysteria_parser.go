package singbox

import (
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"strings"

	"github.com/sagernet/sing/common/byteformats"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func parseSingBoxHysteria(node *domain.NodeIR, outbound map[string]any) {
	node.Hysteria = &domain.HysteriaOptions{
		ServerPorts:  shared.StringSliceValue(outbound["server_ports"]),
		HopInterval:  shared.StringValue(outbound["hop_interval"]),
		ObfsPassword: shared.StringValue(outbound["obfs"]),
		AuthString:   shared.StringValue(outbound["auth_str"]),
		Auth:         singBoxHysteriaAuth(node, outbound["auth"]),
	}
	up := singBoxHysteriaRate(node, outbound["up"], outbound["up_mbps"], "up", "up_mbps")
	node.Hysteria.Up, node.Hysteria.UpMbps = up.Text, up.Mbps
	down := singBoxHysteriaRate(node, outbound["down"], outbound["down_mbps"], "down", "down_mbps")
	node.Hysteria.Down, node.Hysteria.DownMbps = down.Text, down.Mbps
}

func singBoxHysteriaAuth(node *domain.NodeIR, value any) string {
	encoded := shared.StringValue(value)
	if encoded == "" {
		return ""
	}
	auth, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		shared.AddRaw(node.Raw, "sing-box.auth", value)
		return ""
	}
	return string(auth)
}

func singBoxHysteriaRate(node *domain.NodeIR, value, fallbackValue any, rawKey, fallbackKey string) shared.HysteriaRate {
	if value == nil || strings.TrimSpace(shared.StringValue(value)) == "" {
		if fallbackValue == nil {
			return shared.HysteriaRate{}
		}
		fallbackMbps, err := shared.NormalizeHysteriaMbps(fallbackValue)
		if err != nil {
			shared.AddRaw(node.Raw, "sing-box."+fallbackKey, fallbackValue)
			return shared.HysteriaRate{}
		}
		return shared.HysteriaRate{Mbps: fallbackMbps}
	}
	implicit := shared.HysteriaImplicitNone
	if _, ok := value.(jsontext.Value); ok {
		implicit = shared.HysteriaImplicitBps
	}
	rate, err := shared.NormalizeHysteriaRate(shared.StringValue(value), implicit)
	if err != nil {
		shared.AddRaw(node.Raw, "sing-box."+rawKey, value)
		return shared.HysteriaRate{}
	}
	if fallbackValue != nil {
		if _, err := shared.NormalizeHysteriaMbps(fallbackValue); err != nil {
			shared.AddRaw(node.Raw, "sing-box."+fallbackKey, fallbackValue)
		}
	}
	return rate
}

func parseSingBoxHysteria2(node *domain.NodeIR, outbound map[string]any) {
	node.Hysteria = &domain.HysteriaOptions{
		ServerPorts:         shared.StringSliceValue(outbound["server_ports"]),
		HopInterval:         singBoxStringValue(node, outbound, "hop_interval"),
		HopIntervalMax:      singBoxStringValue(node, outbound, "hop_interval_max"),
		UpMbps:              singBoxIntValue(node, outbound, "up_mbps"),
		DownMbps:            singBoxIntValue(node, outbound, "down_mbps"),
		BBRProfile:          singBoxStringValue(node, outbound, "bbr_profile"),
		BrutalDebug:         singBoxBoolValue(node, outbound, "brutal_debug"),
		DisableChromeParrot: singBoxBoolValue(node, outbound, "disable_chrome_parrot"),
	}
	if obfs := shared.AnyMapValue(outbound["obfs"]); obfs != nil {
		node.Hysteria.Obfs = shared.StringValue(obfs["type"])
		node.Hysteria.ObfsPassword = shared.StringValue(obfs["password"])
		node.Hysteria.GeckoMinPacketSize = singBoxNestedIntValue(node, obfs, "obfs", "min_packet_size")
		node.Hysteria.GeckoMaxPacketSize = singBoxNestedIntValue(node, obfs, "obfs", "max_packet_size")
		preserveNestedSingBoxRaw(node, "obfs", obfs, map[string]bool{
			"type": true, "password": true, "min_packet_size": true, "max_packet_size": true,
		})
	} else if value, ok := outbound["obfs"]; ok && value != nil {
		shared.AddRaw(node.Raw, "sing-box.obfs", value)
	}
	node.Hysteria.QUIC = parseSingBoxHysteria2QUIC(node, outbound)
	node.Hysteria.TLSIdentity = parseSingBoxHysteria2TLSIdentity(node, shared.AnyMapValue(outbound["tls"]), "sing-box.tls")
	if realm := shared.AnyMapValue(outbound["realm"]); realm != nil {
		node.Hysteria.Realm = &domain.HysteriaRealmOptions{
			Enabled:     true,
			ServerURL:   shared.StringValue(realm["server_url"]),
			Token:       shared.StringValue(realm["token"]),
			RealmID:     shared.StringValue(realm["realm_id"]),
			STUNServers: shared.StringSliceValue(realm["stun_servers"]),
			IPVersion:   singBoxNestedIntValue(node, realm, "realm", "ip_version"),
		}
		if mapping := shared.AnyMapValue(realm["port_mapping"]); mapping != nil {
			node.Hysteria.Realm.PortMapping = &domain.HysteriaRealmPortMapping{
				Enabled:  singBoxNestedBoolValue(node, mapping, "realm.port_mapping", "enabled"),
				Timeout:  singBoxNestedStringValue(node, mapping, "realm.port_mapping", "timeout"),
				Lifetime: singBoxNestedStringValue(node, mapping, "realm.port_mapping", "lifetime"),
			}
			preserveNestedSingBoxRaw(node, "realm.port_mapping", mapping, map[string]bool{
				"enabled": true, "timeout": true, "lifetime": true,
			})
		} else if value, ok := realm["port_mapping"]; ok && value != nil {
			shared.AddRaw(node.Raw, "sing-box.realm.port_mapping", value)
		}
		parseSingBoxHysteria2RealmHTTPClient(node, shared.AnyMapValue(realm["http_client"]))
		preserveNestedSingBoxRaw(node, "realm", realm, map[string]bool{
			"server_url": true, "token": true, "realm_id": true, "stun_servers": true,
			"ip_version": true, "port_mapping": true, "http_client": true,
		})
	} else if value, ok := outbound["realm"]; ok && value != nil {
		shared.AddRaw(node.Raw, "sing-box.realm", value)
	}
}

func parseSingBoxHysteria2QUIC(node *domain.NodeIR, outbound map[string]any) *domain.HysteriaQUICOptions {
	options := &domain.HysteriaQUICOptions{
		IdleTimeout:             singBoxStringValue(node, outbound, "idle_timeout"),
		KeepAlivePeriod:         singBoxStringValue(node, outbound, "keep_alive_period"),
		StreamReceiveWindow:     singBoxMemoryBytes(node, outbound, "stream_receive_window"),
		ConnectionReceiveWindow: singBoxMemoryBytes(node, outbound, "connection_receive_window"),
		MaxConcurrentStreams:    singBoxIntValue(node, outbound, "max_concurrent_streams"),
		InitialPacketSize:       singBoxIntValue(node, outbound, "initial_packet_size"),
		DisablePathMTUDiscovery: singBoxBoolValue(node, outbound, "disable_path_mtu_discovery"),
	}
	if options.IsZero() {
		return nil
	}
	return options
}

func singBoxMemoryBytes(node *domain.NodeIR, values map[string]any, key string) uint64 {
	value := values[key]
	if value == nil {
		return 0
	}
	body, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	var parsed byteformats.MemoryBytes
	if err := json.Unmarshal(body, &parsed); err != nil {
		shared.AddRaw(node.Raw, "sing-box."+key, value)
		return 0
	}
	return parsed.Value()
}

func singBoxIntValue(node *domain.NodeIR, values map[string]any, key string) int {
	return singBoxNestedIntValue(node, values, "", key)
}

func singBoxNestedIntValue(node *domain.NodeIR, values map[string]any, prefix, key string) int {
	value, ok := values[key]
	if !ok || value == nil {
		return 0
	}
	parsed, err := shared.IntValue(value)
	if err != nil {
		shared.AddRaw(node.Raw, singBoxRawKey(prefix, key), value)
		return 0
	}
	return parsed
}

func singBoxStringValue(node *domain.NodeIR, values map[string]any, key string) string {
	return singBoxNestedStringValue(node, values, "", key)
}

func singBoxNestedStringValue(node *domain.NodeIR, values map[string]any, prefix, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	parsed, ok := value.(string)
	if !ok {
		shared.AddRaw(node.Raw, singBoxRawKey(prefix, key), value)
		return ""
	}
	return parsed
}

func singBoxBoolValue(node *domain.NodeIR, values map[string]any, key string) bool {
	return singBoxNestedBoolValue(node, values, "", key)
}

func singBoxNestedBoolValue(node *domain.NodeIR, values map[string]any, prefix, key string) bool {
	value, ok := values[key]
	if !ok || value == nil {
		return false
	}
	parsed, ok := value.(bool)
	if !ok {
		shared.AddRaw(node.Raw, singBoxRawKey(prefix, key), value)
		return false
	}
	return parsed
}

func singBoxRawKey(prefix, key string) string {
	if prefix == "" {
		return "sing-box." + key
	}
	return "sing-box." + prefix + "." + key
}

func preserveNestedSingBoxRaw(node *domain.NodeIR, parent string, values map[string]any, known map[string]bool) {
	for key, value := range values {
		if !known[key] {
			shared.AddRaw(node.Raw, singBoxRawKey(parent, key), value)
		}
	}
}

func parseSingBoxHysteria2TLSIdentity(node *domain.NodeIR, tls map[string]any, rawKey string) *domain.Hysteria2TLSIdentity {
	if tls == nil {
		return nil
	}
	certificates := shared.StringSliceValue(tls["client_certificate"])
	keys := shared.StringSliceValue(tls["client_key"])
	if len(certificates) > 1 || len(keys) > 1 {
		shared.AddRaw(node.Raw, rawKey+".client_identity", map[string]any{
			"client_certificate": tls["client_certificate"],
			"client_key":         tls["client_key"],
		})
		return nil
	}
	identity := &domain.Hysteria2TLSIdentity{}
	identity.CertificatePublicKeySHA256 = shared.StringSliceValue(tls["certificate_public_key_sha256"])
	if len(certificates) == 1 {
		identity.Certificate = certificates[0]
	}
	if len(keys) == 1 {
		identity.PrivateKey = keys[0]
	}
	if identity.Certificate == "" && identity.PrivateKey == "" && len(identity.CertificatePublicKeySHA256) == 0 {
		return nil
	}
	return identity
}

func parseSingBoxHysteria2RealmHTTPClient(node *domain.NodeIR, client map[string]any) {
	if client == nil || node.Hysteria == nil || node.Hysteria.Realm == nil {
		return
	}
	unsupported := false
	for key := range client {
		if key != "tls" {
			unsupported = true
			break
		}
	}
	tls := shared.AnyMapValue(client["tls"])
	if tls != nil {
		for key := range tls {
			switch key {
			case "enabled", "server_name", "insecure", "alpn", "client_certificate", "client_key", "certificate_public_key_sha256":
			default:
				unsupported = true
			}
		}
		node.Hysteria.Realm.TLS = &domain.TLSOptions{
			Enabled:            shared.BoolValue(tls["enabled"]),
			ServerName:         shared.StringValue(tls["server_name"]),
			InsecureSkipVerify: shared.BoolValue(tls["insecure"]),
			ALPN:               shared.StringSliceValue(tls["alpn"]),
		}
		node.Hysteria.Realm.TLSIdentity = parseSingBoxHysteria2TLSIdentity(node, tls, "sing-box.realm.http_client.tls")
		if _, criticalIdentity := node.Raw["sing-box.realm.http_client.tls.client_identity"]; criticalIdentity {
			delete(node.Raw, "sing-box.realm.http_client.tls.client_identity")
			unsupported = true
		}
	}
	if unsupported {
		shared.AddRaw(node.Raw, "sing-box.realm.http_client", client)
	}
}
