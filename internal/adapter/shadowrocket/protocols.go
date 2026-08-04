package shadowrocket

import (
	"strconv"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func renderShadowsocks(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	parts, err := baseParts("ss", node.Server, node.Port)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := validateTLS(node.TLS, tlsFields{}); err != nil {
		return nil, nil, nil, err
	}
	websocket, err := documentedTransport(node.Transport, true)
	if err != nil {
		return nil, nil, nil, err
	}
	if node.Plugin != "" && node.Plugin != "none" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Shadowsocks plugin is not documented for Shadowrocket local proxy output")
	}
	parts, err = appendRequiredField(parts, "password", "password", node.Password)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendRequiredField(parts, "method", "cipher", node.Cipher)
	if err != nil {
		return nil, nil, nil, err
	}
	if websocket {
		parts = append(parts, "obfs=websocket")
	}
	if node.Plugin == "none" {
		parts = append(parts, "plugin=none")
	}
	emitted := emittedFields{
		"cipher": true, "password": true,
	}
	if websocket || isDefaultTCPTransport(node.Transport) {
		emitted["transport.type"] = true
	}
	if node.Plugin == "none" {
		emitted["plugin"] = true
	}
	return parts, emitted, nil, nil
}

func renderVMess(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	parts, err := baseParts("vmess", node.Server, node.Port)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := validateTLS(node.TLS, tlsFields{}); err != nil {
		return nil, nil, nil, err
	}
	websocket, err := documentedTransport(node.Transport, true)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendRequiredField(parts, "password", "uuid", node.UUID)
	if err != nil {
		return nil, nil, nil, err
	}
	parts = append(parts, "alterId="+strconv.Itoa(node.AlterID))
	method := firstNonEmpty(node.Cipher, "auto")
	parts, err = appendRequiredField(parts, "method", "cipher", method)
	if err != nil {
		return nil, nil, nil, err
	}
	if websocket {
		parts = append(parts, "obfs=websocket")
	}
	parts, err = appendOptionalTLS(parts, node.TLS, true, true)
	if err != nil {
		return nil, nil, nil, err
	}
	if node.Dialer != nil && node.Dialer.TFO {
		parts = append(parts, "tfo=1")
	}
	emitted := emittedFields{
		"uuid": true, "alter_id": true, "cipher": true,
	}
	markTransportEmitted(emitted, node.Transport, websocket)
	markTLSEmitted(emitted, node.TLS, true, true, false)
	if node.Dialer != nil && node.Dialer.TFO {
		emitted["dialer.tfo"] = true
	}
	return parts, emitted, nil, nil
}

func renderVLESS(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	if node.Flow != "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "VLESS flow is not documented for Shadowrocket local proxy output")
	}
	if node.Encryption != "" && node.Encryption != "none" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "VLESS encryption is not documented for Shadowrocket local proxy output")
	}
	parts, err := baseParts("vless", node.Server, node.Port)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := validateTLS(node.TLS, tlsFields{enabled: true, serverName: true}); err != nil {
		return nil, nil, nil, err
	}
	websocket, err := documentedTransport(node.Transport, true)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendRequiredField(parts, "password", "uuid", node.UUID)
	if err != nil {
		return nil, nil, nil, err
	}
	if node.TLS != nil && node.TLS.Enabled {
		parts = append(parts, "tls=true")
	}
	if websocket {
		parts = append(parts, "obfs=websocket")
	}
	if node.TLS != nil {
		parts, err = appendOptionalField(parts, "peer", "tls.server_name", node.TLS.ServerName)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	emitted := emittedFields{"uuid": true, "encryption": true}
	markTransportEmitted(emitted, node.Transport, websocket)
	markTLSEmitted(emitted, node.TLS, true, true, false)
	return parts, emitted, nil, nil
}

func renderTrojan(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	parts, err := baseParts("trojan", node.Server, node.Port)
	if err != nil {
		return nil, nil, nil, err
	}
	if _, err := documentedTransport(node.Transport, false); err != nil {
		return nil, nil, nil, err
	}
	if err := validateTLS(node.TLS, tlsFields{enabled: true, serverName: true, insecure: true}); err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendRequiredField(parts, "password", "password", node.Password)
	if err != nil {
		return nil, nil, nil, err
	}
	if node.TLS != nil && node.TLS.InsecureSkipVerify {
		parts = append(parts, "allowInsecure=1")
	}
	if node.TLS != nil {
		parts, err = appendOptionalField(parts, "peer", "tls.server_name", node.TLS.ServerName)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	emitted := emittedFields{"password": true}
	markTransportEmitted(emitted, node.Transport, false)
	markTLSEmitted(emitted, node.TLS, true, true, false)
	return parts, emitted, nil, nil
}

func renderHysteria(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	if node.TLS == nil || !node.TLS.Enabled {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Hysteria TLS is required")
	}
	if node.Hysteria == nil {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Hysteria options are required")
	}
	if len(node.Hysteria.ServerPorts) > 0 {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Hysteria port hopping is not documented for Shadowrocket local proxy output")
	}
	if _, err := documentedTransport(node.Transport, false); err != nil {
		return nil, nil, nil, err
	}
	if err := validateTLS(node.TLS, tlsFields{enabled: true, serverName: true, alpn: true}); err != nil {
		return nil, nil, nil, err
	}
	if node.Hysteria.Auth != "" && node.Hysteria.AuthString != "" && node.Hysteria.Auth != node.Hysteria.AuthString {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Hysteria auth variants cannot both be represented")
	}
	parts, err := baseParts("hysteria", node.Server, node.Port)
	if err != nil {
		return nil, nil, nil, err
	}
	auth := firstNonEmpty(node.Hysteria.Auth, node.Hysteria.AuthString)
	if err := safeScalar("hysteria.auth", auth); err != nil {
		return nil, nil, nil, err
	}
	parts = append(parts, "auth="+auth)
	obfsPassword, err := shadowrocketHysteriaObfsPassword(node)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendOptionalField(parts, "obfsParam", "hysteria.obfs_password", obfsPassword)
	if err != nil {
		return nil, nil, nil, err
	}
	protocol := firstNonEmpty(node.Hysteria.Protocol, "udp")
	if protocol != "udp" && protocol != "wechat-video" && protocol != "faketcp" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Hysteria protocol is not documented for Shadowrocket local proxy output")
	}
	parts, err = appendRequiredField(parts, "protocol", "hysteria.protocol", protocol)
	if err != nil {
		return nil, nil, nil, err
	}
	parts = appendUDP(parts, node)
	parts, err = appendTLSIdentity(parts, node.TLS, true)
	if err != nil {
		return nil, nil, nil, err
	}
	if node.Hysteria.UpMbps != 0 {
		parts = append(parts, "upmbps="+strconv.Itoa(node.Hysteria.UpMbps))
	} else if upMbps, ok := shared.ExactHysteriaMbps(node.Hysteria.Up); ok {
		parts = append(parts, "upmbps="+strconv.Itoa(upMbps))
	}
	if node.Hysteria.DownMbps != 0 {
		parts = append(parts, "downmbps="+strconv.Itoa(node.Hysteria.DownMbps))
	} else if downMbps, ok := shared.ExactHysteriaMbps(node.Hysteria.Down); ok {
		parts = append(parts, "downmbps="+strconv.Itoa(downMbps))
	}
	emitted := emittedFields{
		"hysteria.protocol": true, "hysteria.auth": true, "hysteria.auth_str": true,
		"hysteria.obfs": true, "hysteria.obfs_password": true,
		"hysteria.up_mbps": true, "hysteria.down_mbps": true,
	}
	markTransportEmitted(emitted, node.Transport, false)
	markTLSEmitted(emitted, node.TLS, false, true, true)
	markUDPEmitted(emitted, node)
	return parts, emitted, hysteriaLossWarnings(node, false), nil
}

func shadowrocketHysteriaObfsPassword(node domain.NodeIR) (string, error) {
	mode, password := shared.HysteriaV1Obfs(node)
	if mode != "" && mode != "xplus" {
		return "", domain.NewError(domain.CodeRenderFailed, "Hysteria obfuscation mode is not documented for Shadowrocket local proxy output")
	}
	if mode == "xplus" && password == "" {
		return "", domain.NewError(domain.CodeRenderFailed, "Hysteria xplus obfuscation requires an obfsParam password for Shadowrocket local proxy output")
	}
	return password, nil
}

func renderHysteria2(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	if node.TLS == nil || !node.TLS.Enabled {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Hysteria2 TLS is required")
	}
	if node.Hysteria != nil && len(node.Hysteria.ServerPorts) > 0 {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Hysteria2 port hopping is not documented for Shadowrocket local proxy output")
	}
	if _, err := documentedTransport(node.Transport, false); err != nil {
		return nil, nil, nil, err
	}
	if err := validateTLS(node.TLS, tlsFields{enabled: true, serverName: true, alpn: true}); err != nil {
		return nil, nil, nil, err
	}
	parts, err := baseParts("hysteria2", node.Server, node.Port)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendRequiredField(parts, "auth", "password", node.Password)
	if err != nil {
		return nil, nil, nil, err
	}
	emitted := emittedFields{"password": true}
	if node.Hysteria != nil {
		if node.Hysteria.Obfs != "" && node.Hysteria.Obfs != "salamander" {
			return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Hysteria2 obfuscation mode is not documented for Shadowrocket local proxy output")
		}
		parts, err = appendOptionalField(parts, "obfsParam", "hysteria.obfs_password", node.Hysteria.ObfsPassword)
		if err != nil {
			return nil, nil, nil, err
		}
		emitted["hysteria.obfs"] = true
		emitted["hysteria.obfs_password"] = true
	}
	parts = appendUDP(parts, node)
	parts, err = appendTLSIdentity(parts, node.TLS, true)
	if err != nil {
		return nil, nil, nil, err
	}
	markTransportEmitted(emitted, node.Transport, false)
	markTLSEmitted(emitted, node.TLS, false, true, true)
	markUDPEmitted(emitted, node)
	return parts, emitted, hysteriaLossWarnings(node, true), nil
}

func renderTUIC(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	if node.Token != "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "TUIC token credentials are not documented for Shadowrocket local proxy output")
	}
	if _, err := documentedTransport(node.Transport, false); err != nil {
		return nil, nil, nil, err
	}
	if err := validateTLS(node.TLS, tlsFields{enabled: true, serverName: true, alpn: true}); err != nil {
		return nil, nil, nil, err
	}
	parts, err := baseParts("tuic", node.Server, node.Port)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendRequiredField(parts, "password", "password", node.Password)
	if err != nil {
		return nil, nil, nil, err
	}
	parts = appendUDP(parts, node)
	parts, err = appendRequiredField(parts, "user", "uuid", node.UUID)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendTLSIdentity(parts, node.TLS, true)
	if err != nil {
		return nil, nil, nil, err
	}
	emitted := emittedFields{"uuid": true, "password": true}
	markTransportEmitted(emitted, node.Transport, false)
	markTLSEmitted(emitted, node.TLS, false, true, true)
	markUDPEmitted(emitted, node)
	return parts, emitted, tuicLossWarnings(node), nil
}

func renderHTTP(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	protocol := "http"
	tlsEnabled := node.TLS != nil && node.TLS.Enabled
	if tlsEnabled {
		protocol = "https"
	}
	if err := validateTLS(node.TLS, tlsFields{enabled: true}); err != nil {
		return nil, nil, nil, err
	}
	if _, err := documentedTransport(node.Transport, false); err != nil {
		return nil, nil, nil, err
	}
	parts, err := baseParts(protocol, node.Server, node.Port)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendCredentials(parts, node.Username, node.Password)
	if err != nil {
		return nil, nil, nil, err
	}
	emitted := emittedFields{}
	if node.Username != "" || node.Password != "" {
		emitted["username"] = true
		emitted["password"] = true
	}
	markTLSEmitted(emitted, node.TLS, false, false, false)
	markTransportEmitted(emitted, node.Transport, false)
	return parts, emitted, nil, nil
}

func renderSOCKS(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	protocol := "socks5"
	tlsEnabled := node.TLS != nil && node.TLS.Enabled
	if tlsEnabled {
		protocol = "socks5-tls"
	}
	if err := validateTLS(node.TLS, tlsFields{enabled: true, insecure: true}); err != nil {
		return nil, nil, nil, err
	}
	if _, err := documentedTransport(node.Transport, false); err != nil {
		return nil, nil, nil, err
	}
	parts, err := baseParts(protocol, node.Server, node.Port)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendCredentials(parts, node.Username, node.Password)
	if err != nil {
		return nil, nil, nil, err
	}
	if tlsEnabled && node.TLS.InsecureSkipVerify {
		parts = append(parts, "skip-common-name-verify=true")
	}
	emitted := emittedFields{}
	if node.Username != "" || node.Password != "" {
		emitted["username"] = true
		emitted["password"] = true
	}
	markTLSEmitted(emitted, node.TLS, true, false, false)
	markTransportEmitted(emitted, node.Transport, false)
	return parts, emitted, nil, nil
}

func renderWireGuard(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	if node.WireGuard == nil {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "WireGuard options are required")
	}
	if len(node.WireGuard.Peers) != 1 {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "WireGuard requires exactly one effective peer")
	}
	if _, err := documentedTransport(node.Transport, false); err != nil {
		return nil, nil, nil, err
	}
	if err := validateTLS(node.TLS, tlsFields{}); err != nil {
		return nil, nil, nil, err
	}
	wg := node.WireGuard
	peer := wg.Peers[0]
	if peer.PreSharedKey != "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "WireGuard pre-shared keys are not documented for Shadowrocket local proxy output")
	}
	server := firstNonEmpty(peer.Server, node.Server)
	port := peer.Port
	if port == 0 {
		port = node.Port
	}
	address := firstNonEmpty(wg.IP, firstString(wg.Address), wg.IPv6)
	parts, err := baseParts("wireguard", server, port)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendRequiredField(parts, "privateKey", "wireguard.private_key", wg.PrivateKey)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendRequiredField(parts, "publicKey", "wireguard.peers.public_key", peer.PublicKey)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendRequiredField(parts, "ip", "wireguard.address", address)
	if err != nil {
		return nil, nil, nil, err
	}
	parts = appendUDP(parts, node)
	if wg.MTU != 0 {
		parts = append(parts, "mtu="+strconv.Itoa(wg.MTU))
	}
	keepalive := peer.PersistentKeepalive
	if keepalive == 0 {
		keepalive = wg.PersistentKeepalive
	}
	if keepalive != 0 {
		parts = append(parts, "keepalive="+strconv.Itoa(keepalive))
	}
	reserved := peer.Reserved
	if len(reserved) == 0 {
		reserved = wg.Reserved
	}
	if len(reserved) > 0 {
		parts = append(parts, "reserved="+joinReserved(reserved))
	}
	emitted := emittedFields{
		"wireguard.private_key": true, "wireguard.address": true,
		"wireguard.peers": true, "wireguard.public_key": true,
		"wireguard.mtu": true, "wireguard.persistent_keepalive": true,
		"wireguard.reserved": true,
	}
	markTransportEmitted(emitted, node.Transport, false)
	markUDPEmitted(emitted, node)
	return parts, emitted, wireGuardLossWarnings(node), nil
}

func renderSnell(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	if _, err := documentedTransport(node.Transport, false); err != nil {
		return nil, nil, nil, err
	}
	if err := validateTLS(node.TLS, tlsFields{}); err != nil {
		return nil, nil, nil, err
	}
	if node.Snell != nil {
		if node.Snell.Version != 0 && node.Snell.Version != 2 {
			return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "only Snell v2 is documented for Shadowrocket local proxy output")
		}
		if node.Snell.ShadowTLS != nil {
			return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Snell ShadowTLS is not documented for Shadowrocket local proxy output")
		}
		if node.Snell.Obfs != "" && node.Snell.Obfs != "http" {
			return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Snell obfuscation mode is not documented for Shadowrocket local proxy output")
		}
		if node.Snell.ObfsHost != "" && node.Snell.Obfs == "" {
			return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Snell obfuscation host requires HTTP obfuscation")
		}
		if node.Snell.Reuse != nil && !*node.Snell.Reuse {
			return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "Snell v2 requires reuse")
		}
	}
	parts, err := baseParts("snell", node.Server, node.Port)
	if err != nil {
		return nil, nil, nil, err
	}
	parts, err = appendRequiredField(parts, "password", "password", node.Password)
	if err != nil {
		return nil, nil, nil, err
	}
	parts = appendUDP(parts, node)
	emitted := emittedFields{"password": true, "snell.version": true, "snell.reuse": true}
	if node.Snell != nil {
		parts, err = appendOptionalField(parts, "obfs", "snell.obfs", node.Snell.Obfs)
		if err != nil {
			return nil, nil, nil, err
		}
		parts, err = appendOptionalField(parts, "obfs-host", "snell.obfs_host", node.Snell.ObfsHost)
		if err != nil {
			return nil, nil, nil, err
		}
		emitted["snell.obfs"] = true
		emitted["snell.obfs_host"] = true
	}
	markTransportEmitted(emitted, node.Transport, false)
	markUDPEmitted(emitted, node)
	return parts, emitted, nil, nil
}
