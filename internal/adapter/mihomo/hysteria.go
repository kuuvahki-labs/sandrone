package mihomo

import (
	"strconv"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func renderHysteria(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing hysteria fields")
	}
	hy := node.Hysteria
	if hy == nil {
		hy = &domain.HysteriaOptions{}
	}
	if err := shared.ValidateCanonicalHysteriaBandwidth(hy); err != nil {
		return nil, nil, nil, domain.WrapError(domain.CodeRenderFailed, "invalid hysteria bandwidth", err)
	}
	out := baseProxy(node, "hysteria")
	if len(hy.ServerPorts) > 0 {
		out["ports"] = strings.Join(hy.ServerPorts, ",")
	}
	if hy.Protocol != "" {
		out["protocol"] = hy.Protocol
	}
	if hy.UpMbps > 0 {
		out["up"] = strconv.Itoa(hy.UpMbps) + " Mbps"
	} else {
		out["up"] = hy.Up
	}
	if hy.DownMbps > 0 {
		out["down"] = strconv.Itoa(hy.DownMbps) + " Mbps"
	} else {
		out["down"] = hy.Down
	}
	if hy.Auth != "" {
		out["auth"] = hy.Auth
	}
	if hy.AuthString != "" {
		out["auth-str"] = hy.AuthString
	}
	obfsMode, obfsPassword := shared.HysteriaV1Obfs(node)
	if obfsMode != "" && obfsMode != "xplus" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "unsupported hysteria obfuscation mode")
	}
	if obfsMode == "xplus" && obfsPassword == "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "hysteria xplus obfuscation password is required")
	}
	if obfsPassword != "" {
		out["obfs"] = obfsPassword
	}
	if hy.HopInterval != "" {
		out["hop-interval"] = durationSecondsOrString(hy.HopInterval)
	}
	applyMihomoTLS(out, node, "sni")
	return out, nil, nil, nil
}

func renderHysteria2(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing hysteria2 fields")
	}
	hy := node.Hysteria
	if hy == nil {
		hy = &domain.HysteriaOptions{}
	}
	if node.Port == 0 && len(hy.ServerPorts) == 0 {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing hysteria2 fields")
	}
	if hy.Realm != nil && hy.Realm.Enabled && len(hy.ServerPorts) > 0 {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "hysteria2 Realm and port hopping are mutually exclusive")
	}
	out := baseProxy(node, "hysteria2")
	warnings := []domain.Warning{}
	if node.Port == 0 {
		delete(out, "port")
	}
	if node.Password != "" {
		out["password"] = node.Password
	}
	if len(hy.ServerPorts) > 0 {
		out["ports"] = strings.Join(hy.ServerPorts, ",")
	}
	if len(hy.ServerPorts) > 0 && hy.HopInterval != "" {
		hopInterval, err := shared.FormatMihomoHysteria2HopInterval(hy.HopInterval, hy.HopIntervalMax)
		if err != nil {
			return nil, nil, nil, domain.WrapError(domain.CodeRenderFailed, "invalid mihomo hysteria2 hop interval", err)
		}
		out["hop-interval"] = hopInterval
	} else if len(hy.ServerPorts) == 0 && (hy.HopInterval != "" || hy.HopIntervalMax != "") {
		warnings = append(warnings, shared.RenderLossyWarning(node, "mihomo-proxies", "hysteria.hop_interval", "Mihomo ignores Hysteria2 hop intervals when port hopping is not configured"))
	}
	if hy.Up != "" {
		out["up"] = hy.Up
	} else if hy.UpMbps != 0 {
		out["up"] = strconv.Itoa(hy.UpMbps) + " Mbps"
	}
	if hy.Down != "" {
		out["down"] = hy.Down
	} else if hy.DownMbps != 0 {
		out["down"] = strconv.Itoa(hy.DownMbps) + " Mbps"
	}
	if hy.Obfs != "" {
		out["obfs"] = hy.Obfs
	}
	if hy.ObfsPassword != "" {
		out["obfs-password"] = hy.ObfsPassword
	}
	if hy.GeckoMinPacketSize != 0 {
		out["obfs-min-packet-size"] = hy.GeckoMinPacketSize
	}
	if hy.GeckoMaxPacketSize != 0 {
		out["obfs-max-packet-size"] = hy.GeckoMaxPacketSize
	}
	if hy.BBRProfile != "" {
		out["bbr-profile"] = hy.BBRProfile
	}
	if hy.CWND != 0 {
		out["cwnd"] = hy.CWND
	}
	if hy.UDPMTU != 0 {
		out["udp-mtu"] = hy.UDPMTU
	}
	if hy.HandshakeTimeout != "" {
		duration, err := time.ParseDuration(hy.HandshakeTimeout)
		if err != nil || duration <= 0 || duration%time.Second != 0 {
			return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "invalid mihomo hysteria2 handshake timeout")
		}
		out["handshake-timeout"] = int64(duration / time.Second)
	}
	if hy.TLSIdentity != nil {
		if node.TLS != nil && node.TLS.Fingerprint != "" && node.TLS.Fingerprint != hy.TLSIdentity.MihomoFingerprint {
			return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "conflicting Hysteria2 Mihomo certificate fingerprints")
		}
		if len(hy.TLSIdentity.CertificatePublicKeySHA256) > 0 {
			return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "sing-box TLS public-key pin cannot be represented as a Mihomo DER fingerprint")
		}
		applyMihomoHysteria2TLSIdentity(out, hy.TLSIdentity)
	}
	applyMihomoHysteria2QUIC(out, hy.QUIC)
	if hy.Realm != nil && hy.Realm.Enabled {
		if _, ok := node.Raw["sing-box.realm.http_client"]; ok {
			return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "sing-box Realm HTTP client contains connection-critical options not represented by Mihomo")
		}
		if hy.Realm.IPVersion != 0 || hy.Realm.PortMapping != nil && hy.Realm.PortMapping.Enabled {
			return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Mihomo Realm cannot represent IP version or port mapping")
		}
		realm := map[string]any{
			"enable":       hy.Realm.Enabled,
			"server-url":   hy.Realm.ServerURL,
			"token":        hy.Realm.Token,
			"realm-id":     hy.Realm.RealmID,
			"stun-servers": hy.Realm.STUNServers,
		}
		if hy.Realm.TLS != nil {
			if hy.Realm.TLS.Fingerprint != "" && hy.Realm.TLSIdentity != nil && hy.Realm.TLS.Fingerprint != hy.Realm.TLSIdentity.MihomoFingerprint {
				return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "conflicting Hysteria2 Realm Mihomo certificate fingerprints")
			}
			if hy.Realm.TLS.ServerName != "" {
				realm["sni"] = hy.Realm.TLS.ServerName
			}
			if hy.Realm.TLS.InsecureSkipVerify {
				realm["skip-cert-verify"] = true
			}
			if len(hy.Realm.TLS.ALPN) > 0 {
				realm["alpn"] = hy.Realm.TLS.ALPN
			}
			if hy.Realm.TLS.Fingerprint != "" {
				realm["fingerprint"] = hy.Realm.TLS.Fingerprint
			}
		}
		if identity := hy.Realm.TLSIdentity; identity != nil {
			if len(identity.CertificatePublicKeySHA256) > 0 {
				return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "sing-box Realm TLS public-key pin cannot be represented by Mihomo")
			}
			applyMihomoHysteria2TLSIdentity(realm, identity)
		}
		out["realm-opts"] = realm
	}
	if _, ok := node.Raw["sing-box.tls.client_identity"]; ok {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "sing-box TLS client identity contains connection-critical options not represented by Mihomo")
	}
	applyMihomoTLS(out, node, "sni")
	return out, nil, warnings, nil
}

func applyMihomoHysteria2TLSIdentity(out map[string]any, identity *domain.Hysteria2TLSIdentity) {
	if identity.CertificateName != "" {
		out["name-cert-verify"] = identity.CertificateName
	}
	if identity.Certificate != "" {
		out["certificate"] = identity.Certificate
	}
	if identity.PrivateKey != "" {
		out["private-key"] = identity.PrivateKey
	}
	if identity.MihomoFingerprint != "" {
		out["fingerprint"] = identity.MihomoFingerprint
	}
}

func applyMihomoHysteria2QUIC(out map[string]any, options *domain.HysteriaQUICOptions) {
	if options == nil {
		return
	}
	initialStream, maxStream := options.InitialStreamReceiveWindow, options.MaxStreamReceiveWindow
	if initialStream == 0 && maxStream == 0 && options.StreamReceiveWindow != 0 {
		initialStream, maxStream = options.StreamReceiveWindow, options.StreamReceiveWindow
	}
	initialConnection, maxConnection := options.InitialConnectionReceiveWindow, options.MaxConnectionReceiveWindow
	if initialConnection == 0 && maxConnection == 0 && options.ConnectionReceiveWindow != 0 {
		initialConnection, maxConnection = options.ConnectionReceiveWindow, options.ConnectionReceiveWindow
	}
	if initialStream != 0 {
		out["initial-stream-receive-window"] = initialStream
	}
	if maxStream != 0 {
		out["max-stream-receive-window"] = maxStream
	}
	if initialConnection != 0 {
		out["initial-connection-receive-window"] = initialConnection
	}
	if maxConnection != 0 {
		out["max-connection-receive-window"] = maxConnection
	}
}
