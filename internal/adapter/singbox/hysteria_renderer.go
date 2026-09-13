package singbox

import (
	"encoding/json/v2"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/sagernet/sing/common/json/badoption"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func renderHysteria(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.TLS == nil || !node.TLS.Enabled {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing hysteria fields")
	}
	hy := node.Hysteria
	if hy == nil {
		hy = &domain.HysteriaOptions{}
	}
	if err := shared.ValidateCanonicalHysteriaBandwidth(hy); err != nil {
		return nil, false, nil, nil, domain.WrapError(domain.CodeRenderFailed, "invalid hysteria bandwidth", err)
	}
	if err := validateSingBoxHopInterval(hy.HopInterval); err != nil {
		return nil, false, nil, nil, domain.WrapError(domain.CodeRenderFailed, "invalid sing-box hysteria hop_interval", err)
	}
	up, err := singBoxRepresentableHysteriaRate(hy.Up)
	if err != nil {
		return nil, false, nil, nil, domain.WrapError(domain.CodeRenderFailed, "hysteria upload bandwidth is not representable by sing-box", err)
	}
	down, err := singBoxRepresentableHysteriaRate(hy.Down)
	if err != nil {
		return nil, false, nil, nil, domain.WrapError(domain.CodeRenderFailed, "hysteria download bandwidth is not representable by sing-box", err)
	}
	out := baseOutbound(node, "hysteria")
	if len(hy.ServerPorts) > 0 {
		out["server_ports"] = singBoxHysteriaServerPorts(hy.ServerPorts)
	}
	if hy.HopInterval != "" {
		out["hop_interval"] = hy.HopInterval
	}
	if up != "" {
		out["up"] = up
	}
	if down != "" {
		out["down"] = down
	}
	upMbps := hy.UpMbps
	if upMbps == 0 {
		upMbps, _ = shared.ExactHysteriaMbps(hy.Up)
	}
	if upMbps != 0 {
		out["up_mbps"] = upMbps
	}
	downMbps := hy.DownMbps
	if downMbps == 0 {
		downMbps, _ = shared.ExactHysteriaMbps(hy.Down)
	}
	if downMbps != 0 {
		out["down_mbps"] = downMbps
	}
	obfsMode, obfsPassword := shared.HysteriaV1Obfs(node)
	if obfsMode != "" && obfsMode != "xplus" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "unsupported hysteria obfuscation mode")
	}
	if obfsMode == "xplus" && obfsPassword == "" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "hysteria xplus obfuscation password is required")
	}
	if obfsPassword != "" {
		out["obfs"] = obfsPassword
	}
	if hy.Auth != "" {
		out["auth"] = []byte(hy.Auth)
	}
	if hy.AuthString != "" {
		out["auth_str"] = hy.AuthString
	}
	applyTLS(out, node)
	warnings := []domain.Warning{}
	if hy.QUIC != nil && !hy.QUIC.IsZero() {
		warnings = append(warnings, lossyWarning(node, "hysteria.quic", "sing-box v1.14.0 hysteria outbound schema has no QUIC tuning fields represented by NodeIR"))
	}
	return out, false, nil, warnings, nil
}

func singBoxRepresentableHysteriaRate(rate string) (string, error) {
	valueText, isBitsPerSecond := strings.CutSuffix(rate, " bps")
	if !isBitsPerSecond {
		return rate, nil
	}
	value, err := strconv.ParseUint(valueText, 10, 64)
	if err != nil {
		return "", err
	}
	if value%8 != 0 {
		return "", fmt.Errorf("%q cannot be converted losslessly to bytes per second", rate)
	}
	return strconv.FormatUint(value/8, 10) + " Bps", nil
}

func validateSingBoxHopInterval(value string) error {
	if value == "" {
		return nil
	}
	var duration badoption.Duration
	return duration.UnmarshalJSON([]byte(strconv.Quote(value)))
}

func singBoxHysteriaServerPorts(values []string) []string {
	ports := make([]string, len(values))
	for index, value := range values {
		if strings.Contains(value, ":") {
			ports[index] = value
			continue
		}
		start, end, ranged := strings.Cut(value, "-")
		if !ranged {
			end = start
		}
		ports[index] = start + ":" + end
	}
	return ports
}

func renderHysteria2(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.TLS == nil || !node.TLS.Enabled {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing hysteria2 fields")
	}
	hy := node.Hysteria
	if hy == nil {
		hy = &domain.HysteriaOptions{}
	}
	realmEnabled := hy.Realm != nil && hy.Realm.Enabled
	if !realmEnabled && (node.Server == "" || node.Port == 0 && len(hy.ServerPorts) == 0) {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing hysteria2 fields")
	}
	if realmEnabled && len(hy.ServerPorts) > 0 {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "hysteria2 Realm and server_ports are mutually exclusive")
	}
	upMbps, err := singBoxHysteria2Mbps("up", hy.Up, hy.UpMbps)
	if err != nil {
		return nil, false, nil, nil, err
	}
	downMbps, err := singBoxHysteria2Mbps("down", hy.Down, hy.DownMbps)
	if err != nil {
		return nil, false, nil, nil, err
	}
	out := map[string]any{"type": "hysteria2", "tag": node.Name}
	if !realmEnabled {
		out["server"] = node.Server
		if node.Port != 0 {
			out["server_port"] = int(node.Port)
		}
	}
	applyDialer(out, node)
	if node.Password != "" {
		out["password"] = node.Password
	}
	if err := validateSingBoxHopInterval(hy.HopInterval); err != nil {
		return nil, false, nil, nil, domain.WrapError(domain.CodeRenderFailed, "invalid sing-box hysteria2 hop_interval", err)
	}
	if err := validateSingBoxHopInterval(hy.HopIntervalMax); err != nil {
		return nil, false, nil, nil, domain.WrapError(domain.CodeRenderFailed, "invalid sing-box hysteria2 hop_interval_max", err)
	}
	if len(hy.ServerPorts) > 0 {
		out["server_ports"] = singBoxHysteriaServerPorts(hy.ServerPorts)
	}
	if hy.HopInterval != "" {
		out["hop_interval"] = hy.HopInterval
	}
	if hy.HopIntervalMax != "" {
		out["hop_interval_max"] = hy.HopIntervalMax
	}
	if upMbps != 0 {
		out["up_mbps"] = upMbps
	}
	if downMbps != 0 {
		out["down_mbps"] = downMbps
	}
	if hy.Obfs != "" || hy.ObfsPassword != "" {
		obfs := map[string]any{
			"type":     hy.Obfs,
			"password": hy.ObfsPassword,
		}
		if hy.GeckoMinPacketSize != 0 {
			obfs["min_packet_size"] = hy.GeckoMinPacketSize
		}
		if hy.GeckoMaxPacketSize != 0 {
			obfs["max_packet_size"] = hy.GeckoMaxPacketSize
		}
		out["obfs"] = obfs
	}
	if hy.BBRProfile != "" {
		out["bbr_profile"] = hy.BBRProfile
	}
	if hy.BrutalDebug {
		out["brutal_debug"] = true
	}
	if hy.DisableChromeParrot {
		out["disable_chrome_parrot"] = true
	}
	applySingBoxHysteria2QUIC(out, hy.QUIC)
	if realmEnabled {
		realm, err := renderSingBoxHysteria2Realm(hy.Realm)
		if err != nil {
			return nil, false, nil, nil, err
		}
		if rawHTTPClient, ok := node.Raw["sing-box.realm.http_client"]; ok {
			realm["http_client"] = rawHTTPClient
		}
		out["realm"] = realm
	}
	applyTLS(out, node)
	if realmEnabled && node.Server != "" && node.TLS.ServerName == "" {
		tlsOut := shared.AnyMapValue(out["tls"])
		if tlsOut == nil {
			tlsOut = map[string]any{"enabled": true}
		}
		tlsOut["server_name"] = node.Server
		out["tls"] = tlsOut
	}
	if err := applySingBoxHysteria2TLSIdentity(out, node, hy.TLSIdentity); err != nil {
		return nil, false, nil, nil, err
	}
	skipRaw := map[string]bool{}
	if rawIdentity, ok := node.Raw["sing-box.tls.client_identity"]; ok {
		identity := shared.AnyMapValue(out["tls"])
		if identity == nil {
			identity = map[string]any{"enabled": true}
		}
		var fields map[string]any
		if err := json.Unmarshal(rawIdentity, &fields); err != nil {
			return nil, false, nil, nil, domain.WrapError(domain.CodeRenderFailed, "restore sing-box Hysteria2 TLS client identity", err)
		}
		for key, value := range fields {
			identity[key] = value
		}
		out["tls"] = identity
		skipRaw["sing-box.tls.client_identity"] = true
	}
	if _, ok := node.Raw["sing-box.realm.http_client"]; ok {
		skipRaw["sing-box.realm.http_client"] = true
	}
	warnings := []domain.Warning{}
	if hy.Up != "" && upMbps == 0 {
		warnings = append(warnings, lossyWarning(node, "hysteria.up", "sing-box hysteria2 uses up_mbps instead of string up"))
	}
	if hy.Down != "" && downMbps == 0 {
		warnings = append(warnings, lossyWarning(node, "hysteria.down", "sing-box hysteria2 uses down_mbps instead of string down"))
	}
	if hy.CWND != 0 {
		warnings = append(warnings, lossyWarning(node, "hysteria.cwnd", "sing-box Hysteria2 outbound schema has no cwnd field"))
	}
	if hy.UDPMTU != 0 {
		warnings = append(warnings, lossyWarning(node, "hysteria.udp_mtu", "sing-box Hysteria2 outbound schema has no udp_mtu field"))
	}
	if hy.HandshakeTimeout != "" {
		warnings = append(warnings, lossyWarning(node, "hysteria.handshake_timeout", "sing-box Hysteria2 handshake timeout is not equivalent to Mihomo handshake-timeout"))
	}
	if hy.QUIC != nil && len(hy.QUIC.Unknown) > 0 {
		warnings = append(warnings, lossyWarning(node, "hysteria.quic", "unknown canonical QUIC fields are not passed through to sing-box"))
	}
	if hy.QUIC != nil && !mihomoWindowPairMapsToSingBox(hy.QUIC.InitialStreamReceiveWindow, hy.QUIC.MaxStreamReceiveWindow, hy.QUIC.StreamReceiveWindow) {
		warnings = append(warnings, lossyWarning(node, "hysteria.quic.stream_receive_window", "Mihomo initial and maximum stream receive windows are not equal"))
	}
	if hy.QUIC != nil && !mihomoWindowPairMapsToSingBox(hy.QUIC.InitialConnectionReceiveWindow, hy.QUIC.MaxConnectionReceiveWindow, hy.QUIC.ConnectionReceiveWindow) {
		warnings = append(warnings, lossyWarning(node, "hysteria.quic.connection_receive_window", "Mihomo initial and maximum connection receive windows are not equal"))
	}
	return out, false, skipRaw, warnings, nil
}

func singBoxHysteria2Mbps(direction, text string, mbps int) (int, error) {
	if text != "" && mbps != 0 {
		return 0, domain.NewError(domain.CodeRenderFailed, "hysteria2 "+direction+" rate must populate only one canonical field")
	}
	if mbps != 0 {
		value, err := shared.NormalizeHysteriaMbps(mbps)
		if err != nil {
			return 0, domain.WrapError(domain.CodeRenderFailed, "invalid hysteria2 "+direction+" Mbps rate", err)
		}
		return value, nil
	}
	value, _ := shared.ExactHysteriaMbps(text)
	return value, nil
}

func applySingBoxHysteria2QUIC(out map[string]any, options *domain.HysteriaQUICOptions) {
	if options == nil {
		return
	}
	if options.IdleTimeout != "" {
		out["idle_timeout"] = options.IdleTimeout
	}
	if options.KeepAlivePeriod != "" {
		out["keep_alive_period"] = options.KeepAlivePeriod
	}
	streamWindow := options.StreamReceiveWindow
	if streamWindow == 0 && options.InitialStreamReceiveWindow != 0 && options.InitialStreamReceiveWindow == options.MaxStreamReceiveWindow {
		streamWindow = options.InitialStreamReceiveWindow
	}
	if streamWindow != 0 {
		out["stream_receive_window"] = streamWindow
	}
	connectionWindow := options.ConnectionReceiveWindow
	if connectionWindow == 0 && options.InitialConnectionReceiveWindow != 0 && options.InitialConnectionReceiveWindow == options.MaxConnectionReceiveWindow {
		connectionWindow = options.InitialConnectionReceiveWindow
	}
	if connectionWindow != 0 {
		out["connection_receive_window"] = connectionWindow
	}
	if options.MaxConcurrentStreams != 0 {
		out["max_concurrent_streams"] = options.MaxConcurrentStreams
	}
	if options.InitialPacketSize != 0 {
		out["initial_packet_size"] = options.InitialPacketSize
	}
	if options.DisablePathMTUDiscovery {
		out["disable_path_mtu_discovery"] = true
	}
}

func mihomoWindowPairMapsToSingBox(initial, maximum, canonical uint64) bool {
	if initial == 0 && maximum == 0 {
		return true
	}
	return initial != 0 && initial == maximum && (canonical == 0 || canonical == initial)
}

func applySingBoxHysteria2TLSIdentity(out map[string]any, node domain.NodeIR, identity *domain.Hysteria2TLSIdentity) error {
	tls := node.TLS
	if identity == nil {
		if tls != nil && tls.Fingerprint != "" {
			return domain.NewError(domain.CodeRenderFailed, "Mihomo TLS fingerprint cannot be represented as a sing-box public-key pin")
		}
		return nil
	}
	if identity.CertificateName != "" {
		effectiveServerName := node.Server
		if tls != nil && tls.ServerName != "" {
			effectiveServerName = tls.ServerName
		}
		if !strings.EqualFold(identity.CertificateName, effectiveServerName) {
			return domain.NewError(domain.CodeRenderFailed, "Mihomo certificate verification name differs from the effective sing-box TLS server name")
		}
	}
	if identity.MihomoFingerprint != "" || tls != nil && tls.Fingerprint != "" {
		return domain.NewError(domain.CodeRenderFailed, "Mihomo certificate verification identity cannot be represented by sing-box")
	}
	tlsOut := shared.AnyMapValue(out["tls"])
	if tlsOut == nil {
		tlsOut = map[string]any{"enabled": true}
	}
	if identity.Certificate != "" {
		tlsOut["client_certificate"] = identity.Certificate
	}
	if identity.PrivateKey != "" {
		tlsOut["client_key"] = identity.PrivateKey
	}
	if len(identity.CertificatePublicKeySHA256) > 0 {
		tlsOut["certificate_public_key_sha256"] = identity.CertificatePublicKeySHA256
	}
	out["tls"] = tlsOut
	return nil
}

func renderSingBoxHysteria2Realm(realm *domain.HysteriaRealmOptions) (map[string]any, error) {
	out := map[string]any{
		"server_url":   realm.ServerURL,
		"realm_id":     realm.RealmID,
		"stun_servers": realm.STUNServers,
	}
	if realm.Token != "" {
		out["token"] = realm.Token
	}
	if realm.IPVersion != 0 {
		out["ip_version"] = realm.IPVersion
	}
	if mapping := realm.PortMapping; mapping != nil {
		value := map[string]any{}
		if mapping.Enabled {
			value["enabled"] = true
		}
		if mapping.Timeout != "" {
			value["timeout"] = mapping.Timeout
		}
		if mapping.Lifetime != "" {
			value["lifetime"] = mapping.Lifetime
		}
		out["port_mapping"] = value
	}
	if realm.TLS != nil || realm.TLSIdentity != nil {
		client := map[string]any{}
		tls := map[string]any{}
		if realm.TLS != nil {
			if realm.TLS.Fingerprint != "" {
				return nil, domain.NewError(domain.CodeRenderFailed, "Mihomo Realm TLS fingerprint cannot be represented as a sing-box public-key pin")
			}
			if realm.TLS.Enabled {
				tls["enabled"] = true
			}
			if realm.TLS.ServerName != "" {
				tls["server_name"] = realm.TLS.ServerName
			}
			if realm.TLS.InsecureSkipVerify {
				tls["insecure"] = true
			}
			if len(realm.TLS.ALPN) > 0 {
				tls["alpn"] = realm.TLS.ALPN
			}
		}
		if identity := realm.TLSIdentity; identity != nil {
			if identity.CertificateName != "" {
				effectiveServerName := ""
				if serverURL, err := url.Parse(realm.ServerURL); err == nil {
					effectiveServerName = serverURL.Hostname()
				}
				if realm.TLS != nil && realm.TLS.ServerName != "" {
					effectiveServerName = realm.TLS.ServerName
				}
				if !strings.EqualFold(identity.CertificateName, effectiveServerName) {
					return nil, domain.NewError(domain.CodeRenderFailed, "Mihomo Realm certificate verification name differs from the effective sing-box TLS server name")
				}
			}
			if identity.MihomoFingerprint != "" {
				return nil, domain.NewError(domain.CodeRenderFailed, "Mihomo Realm certificate verification identity cannot be represented by sing-box")
			}
			if identity.Certificate != "" {
				tls["client_certificate"] = identity.Certificate
			}
			if identity.PrivateKey != "" {
				tls["client_key"] = identity.PrivateKey
			}
			if len(identity.CertificatePublicKeySHA256) > 0 {
				tls["certificate_public_key_sha256"] = identity.CertificatePublicKeySHA256
			}
		}
		client["tls"] = tls
		out["http_client"] = client
	}
	return out, nil
}
