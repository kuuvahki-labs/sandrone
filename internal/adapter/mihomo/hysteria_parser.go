package mihomo

import (
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func parseMihomoHysteria(node *domain.NodeIR, proxy map[string]any) {
	node.Hysteria = &domain.HysteriaOptions{
		ServerPorts:  firstStringSlice(proxy["ports"], proxy["server-ports"]),
		Protocol:     firstNonEmpty(shared.StringValue(proxy["protocol"]), shared.StringValue(proxy["obfs-protocol"])),
		Auth:         shared.StringValue(proxy["auth"]),
		AuthString:   shared.StringValue(proxy["auth-str"]),
		ObfsPassword: shared.StringValue(proxy["obfs"]),
		HopInterval:  intString(proxy["hop-interval"]),
	}
	applyMihomoHysteriaRate(node, proxy["up"], proxy["up-speed"], "up", "up-speed", &node.Hysteria.Up, &node.Hysteria.UpMbps)
	applyMihomoHysteriaRate(node, proxy["down"], proxy["down-speed"], "down", "down-speed", &node.Hysteria.Down, &node.Hysteria.DownMbps)
}

func parseMihomoHysteria2(node *domain.NodeIR, proxy map[string]any) {
	hopInterval, hopIntervalMax := shared.ParseMihomoHysteria2HopInterval(shared.StringValue(proxy["hop-interval"]))
	node.Hysteria = &domain.HysteriaOptions{
		ServerPorts:        parseMihomoHysteria2Ports(node, proxy),
		HopInterval:        hopInterval,
		HopIntervalMax:     hopIntervalMax,
		Obfs:               shared.StringValue(proxy["obfs"]),
		ObfsPassword:       shared.StringValue(proxy["obfs-password"]),
		GeckoMinPacketSize: mihomoIntValue(node, proxy, "obfs-min-packet-size"),
		GeckoMaxPacketSize: mihomoIntValue(node, proxy, "obfs-max-packet-size"),
		BBRProfile:         shared.StringValue(proxy["bbr-profile"]),
		UDPMTU:             mihomoIntValue(node, proxy, "udp-mtu"),
		CWND:               mihomoIntValue(node, proxy, "cwnd"),
		HandshakeTimeout:   mihomoHysteria2HandshakeTimeout(node, proxy),
		TLSIdentity:        parseMihomoHysteria2TLSIdentity(proxy),
		QUIC:               parseMihomoHysteria2QUIC(node, proxy),
	}
	applyMihomoHysteriaRate(node, proxy["up"], nil, "up", "", &node.Hysteria.Up, &node.Hysteria.UpMbps)
	applyMihomoHysteriaRate(node, proxy["down"], nil, "down", "", &node.Hysteria.Down, &node.Hysteria.DownMbps)
	if node.TLS == nil {
		node.TLS = &domain.TLSOptions{Enabled: true}
	} else {
		node.TLS.Enabled = true
	}
	if node.Hysteria.TLSIdentity != nil {
		node.TLS.Fingerprint = ""
	}
	realm := shared.AnyMapValue(proxy["realm-opts"])
	if realm != nil {
		node.Hysteria.Realm = &domain.HysteriaRealmOptions{
			Enabled:     shared.BoolValue(realm["enable"]),
			ServerURL:   shared.StringValue(realm["server-url"]),
			Token:       shared.StringValue(realm["token"]),
			RealmID:     shared.StringValue(realm["realm-id"]),
			STUNServers: shared.StringSliceValue(realm["stun-servers"]),
			TLSIdentity: parseMihomoHysteria2TLSIdentity(realm),
		}
		if hasMihomoHysteria2TLS(realm) {
			node.Hysteria.Realm.TLS = &domain.TLSOptions{
				Enabled:            true,
				ServerName:         shared.StringValue(realm["sni"]),
				InsecureSkipVerify: shared.BoolValue(realm["skip-cert-verify"]),
				ALPN:               shared.StringSliceValue(realm["alpn"]),
			}
		}
		preserveNestedMihomoRaw(node, "realm-opts", realm, map[string]bool{
			"enable": true, "server-url": true, "token": true, "realm-id": true, "stun-servers": true,
			"sni": true, "skip-cert-verify": true, "name-cert-verify": true, "fingerprint": true,
			"certificate": true, "private-key": true, "alpn": true,
		})
	} else if value, ok := proxy["realm-opts"]; ok && value != nil {
		shared.AddRaw(node.Raw, "mihomo.realm-opts", value)
	}
}

func parseMihomoHysteria2Ports(node *domain.NodeIR, proxy map[string]any) []string {
	if value, ok := proxy["ports"]; ok && value != nil {
		text, ok := value.(string)
		if !ok {
			shared.AddRaw(node.Raw, "mihomo.ports", value)
			return nil
		}
		parts := strings.Split(text, ",")
		ports := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				ports = append(ports, trimmed)
			}
		}
		return ports
	}
	return shared.StringSliceValue(proxy["server-ports"])
}

func parseMihomoHysteria2TLSIdentity(values map[string]any) *domain.Hysteria2TLSIdentity {
	identity := &domain.Hysteria2TLSIdentity{
		CertificateName:   shared.StringValue(values["name-cert-verify"]),
		Certificate:       shared.StringValue(values["certificate"]),
		PrivateKey:        shared.StringValue(values["private-key"]),
		MihomoFingerprint: shared.StringValue(values["fingerprint"]),
	}
	if identity.CertificateName == "" && identity.Certificate == "" && identity.PrivateKey == "" && identity.MihomoFingerprint == "" {
		return nil
	}
	return identity
}

func hasMihomoHysteria2TLS(values map[string]any) bool {
	return shared.StringValue(values["sni"]) != "" ||
		shared.BoolValue(values["skip-cert-verify"]) ||
		len(shared.StringSliceValue(values["alpn"])) > 0 ||
		parseMihomoHysteria2TLSIdentity(values) != nil
}

func parseMihomoHysteria2QUIC(node *domain.NodeIR, proxy map[string]any) *domain.HysteriaQUICOptions {
	options := &domain.HysteriaQUICOptions{}
	options.InitialStreamReceiveWindow = mihomoUint64Value(node, proxy, "initial-stream-receive-window")
	options.MaxStreamReceiveWindow = mihomoUint64Value(node, proxy, "max-stream-receive-window")
	options.InitialConnectionReceiveWindow = mihomoUint64Value(node, proxy, "initial-connection-receive-window")
	options.MaxConnectionReceiveWindow = mihomoUint64Value(node, proxy, "max-connection-receive-window")
	if options.IsZero() {
		return nil
	}
	return options
}

func mihomoIntValue(node *domain.NodeIR, values map[string]any, key string) int {
	value, ok := values[key]
	if !ok || value == nil {
		return 0
	}
	parsed, err := shared.IntValue(value)
	if err != nil {
		shared.AddRaw(node.Raw, "mihomo."+key, value)
		return 0
	}
	return parsed
}

func mihomoUint64Value(node *domain.NodeIR, values map[string]any, key string) uint64 {
	value, ok := values[key]
	if !ok || value == nil {
		return 0
	}
	parsed, err := shared.Uint64Value(value)
	if err != nil {
		shared.AddRaw(node.Raw, "mihomo."+key, value)
		return 0
	}
	return parsed
}

func mihomoHysteria2HandshakeTimeout(node *domain.NodeIR, proxy map[string]any) string {
	value, ok := proxy["handshake-timeout"]
	if !ok || value == nil {
		return ""
	}
	seconds, err := shared.IntValue(value)
	if err != nil || seconds <= 0 || int64(seconds) > (1<<63-1)/int64(time.Second) {
		shared.AddRaw(node.Raw, "mihomo.handshake-timeout", value)
		return ""
	}
	return (time.Duration(seconds) * time.Second).String()
}
