package singbox

import (
	"context"
	"fmt"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) Name() string {
	return "sing-box-outbounds"
}

func (r *Renderer) Render(ctx context.Context, nodes []domain.NodeIR, opt domain.RenderOptions) ([]byte, error) {
	out, _, err := r.RenderWithReport(ctx, nodes, opt)
	return out, err
}

func (r *Renderer) RenderWithReport(_ context.Context, nodes []domain.NodeIR, _ domain.RenderOptions) ([]byte, domain.RenderReport, error) {
	outbounds := make([]map[string]any, 0, len(nodes))
	endpoints := make([]map[string]any, 0)
	report := domain.RenderReport{}
	for _, node := range nodes {
		doc, endpoint, skipRaw, warnings, err := nodeToSingBox(node)
		if err != nil {
			shared.MergeWarnings(&report, []domain.Warning{shared.RenderNodeSkippedWarning(node, r.Name(), err)})
			continue
		}
		warnings = append(warnings, singBoxStructuredLossWarnings(node)...)
		warnings = append(warnings, shared.RawWarnings(node, skipRaw, r.Name())...)
		shared.MergeWarnings(&report, warnings)
		report.SuccessCount++
		if endpoint {
			endpoints = append(endpoints, doc)
		} else {
			outbounds = append(outbounds, doc)
		}
	}
	if len(nodes) > 0 && report.SuccessCount == 0 {
		return nil, report, shared.NoRenderableNodesError(report)
	}
	doc := map[string]any{}
	if len(outbounds) > 0 {
		doc["outbounds"] = outbounds
	}
	if len(endpoints) > 0 {
		doc["endpoints"] = endpoints
	}
	body, err := shared.MarshalStableJSON(doc, true)
	return body, report, err
}

func nodeToSingBox(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	switch node.Type {
	case domain.NodeTypeShadowsocks:
		return renderSS(node)
	case domain.NodeTypeVMess:
		return renderVMess(node)
	case domain.NodeTypeVLESS:
		return renderVLESS(node)
	case domain.NodeTypeTrojan:
		return renderTrojan(node)
	case domain.NodeTypeHysteria:
		return renderHysteria(node)
	case domain.NodeTypeHysteria2:
		return renderHysteria2(node)
	case domain.NodeTypeTUIC:
		return renderTUIC(node)
	case domain.NodeTypeAnyTLS:
		return renderAnyTLS(node)
	case domain.NodeTypeSOCKS:
		return renderSOCKS(node)
	case domain.NodeTypeHTTP:
		return renderHTTP(node)
	case domain.NodeTypeWireGuard:
		return renderWireGuard(node)
	default:
		return nil, false, nil, nil, domain.WrapError(domain.CodeRenderFailed, "unsupported node type", fmt.Errorf("%s", node.Type))
	}
}

func renderSS(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Cipher == "" || node.Password == "" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing ss fields")
	}
	out := baseOutbound(node, "shadowsocks")
	out["method"] = node.Cipher
	out["password"] = node.Password
	plugin, pluginOptions := renderSingBoxSSPlugin(node.Plugin, node.PluginOptions)
	if plugin != "" {
		out["plugin"] = plugin
	}
	if pluginOptions != "" {
		out["plugin_opts"] = pluginOptions
	}
	if node.Network != "" {
		out["network"] = node.Network
	}
	applyUDPOverTCP(out, node)
	applyMux(out, node)
	return out, false, nil, nil, nil
}

func renderVMess(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.UUID == "" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing vmess fields")
	}
	out := baseOutbound(node, "vmess")
	out["uuid"] = node.UUID
	out["security"] = firstNonEmptyRender(node.Cipher, "auto")
	if node.AlterID != 0 {
		out["alter_id"] = node.AlterID
	}
	if node.PacketEncoding != "" {
		out["packet_encoding"] = node.PacketEncoding
	}
	applyTLS(out, node)
	applyTransport(out, node)
	applyMux(out, node)
	return out, false, map[string]bool{"vmess.alter_id": true}, nil, nil
}

func renderVLESS(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.UUID == "" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing vless fields")
	}
	out := baseOutbound(node, "vless")
	out["uuid"] = node.UUID
	if node.Flow != "" {
		out["flow"] = node.Flow
	}
	if node.PacketEncoding != "" {
		out["packet_encoding"] = node.PacketEncoding
	}
	applyTLS(out, node)
	applyTransport(out, node)
	applyMux(out, node)
	warnings := []domain.Warning{}
	if node.Encryption != "" && node.Encryption != "none" {
		warnings = append(warnings, lossyWarning(node, "encryption", "sing-box vless outbound schema has no encryption field in the referenced version"))
	}
	return out, false, nil, warnings, nil
}

func renderTrojan(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Password == "" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing trojan fields")
	}
	out := baseOutbound(node, "trojan")
	out["password"] = node.Password
	applyTLS(out, node)
	applyTransport(out, node)
	applyMux(out, node)
	return out, false, nil, nil, nil
}

func renderAnyTLS(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Password == "" || node.AnyTLS == nil || node.TLS == nil || !node.TLS.Enabled {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing anytls fields")
	}
	out := baseOutbound(node, "anytls")
	out["password"] = node.Password
	if node.AnyTLS.IdleSessionCheckInterval != "" {
		out["idle_session_check_interval"] = node.AnyTLS.IdleSessionCheckInterval
	}
	if node.AnyTLS.IdleSessionTimeout != "" {
		out["idle_session_timeout"] = node.AnyTLS.IdleSessionTimeout
	}
	if node.AnyTLS.MinIdleSession != 0 {
		out["min_idle_session"] = node.AnyTLS.MinIdleSession
	}
	applyTLS(out, node)
	return out, false, nil, nil, nil
}

func renderHysteria(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing hysteria fields")
	}
	out := baseOutbound(node, "hysteria")
	hy := node.Hysteria
	if hy == nil {
		hy = &domain.HysteriaOptions{}
	}
	if len(hy.ServerPorts) > 0 {
		out["server_ports"] = hy.ServerPorts
	}
	if hy.HopInterval != "" {
		out["hop_interval"] = hy.HopInterval
	}
	if hy.Up != "" {
		out["up"] = hy.Up
	}
	if hy.Down != "" {
		out["down"] = hy.Down
	}
	if hy.UpMbps != 0 {
		out["up_mbps"] = hy.UpMbps
	}
	if hy.DownMbps != 0 {
		out["down_mbps"] = hy.DownMbps
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
		out["auth"] = hy.Auth
	}
	if hy.AuthString != "" {
		out["auth_str"] = hy.AuthString
	}
	applyTLS(out, node)
	warnings := []domain.Warning{}
	if len(hy.QUIC) > 0 {
		warnings = append(warnings, lossyWarning(node, "hysteria.quic", "sing-box v1.13.14 hysteria outbound schema has no QUIC tuning fields represented by NodeIR"))
	}
	return out, false, nil, warnings, nil
}

func renderHysteria2(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing hysteria2 fields")
	}
	out := baseOutbound(node, "hysteria2")
	if node.Password != "" {
		out["password"] = node.Password
	}
	hy := node.Hysteria
	if hy == nil {
		hy = &domain.HysteriaOptions{}
	}
	if len(hy.ServerPorts) > 0 {
		out["server_ports"] = hy.ServerPorts
	}
	if hy.HopInterval != "" {
		out["hop_interval"] = hy.HopInterval
	}
	if hy.UpMbps != 0 {
		out["up_mbps"] = hy.UpMbps
	}
	if hy.DownMbps != 0 {
		out["down_mbps"] = hy.DownMbps
	}
	if hy.Obfs != "" || hy.ObfsPassword != "" {
		out["obfs"] = map[string]any{
			"type":     hy.Obfs,
			"password": hy.ObfsPassword,
		}
	}
	applyTLS(out, node)
	warnings := []domain.Warning{}
	if hy.Up != "" {
		warnings = append(warnings, lossyWarning(node, "hysteria.up", "sing-box hysteria2 uses up_mbps instead of string up"))
	}
	if hy.Down != "" {
		warnings = append(warnings, lossyWarning(node, "hysteria.down", "sing-box hysteria2 uses down_mbps instead of string down"))
	}
	if hy.BBRProfile != "" {
		warnings = append(warnings, lossyWarning(node, "hysteria.bbr_profile", "sing-box v1.13.14 hysteria2 outbound schema has no bbr_profile field"))
	}
	if hy.Realm != nil {
		warnings = append(warnings, lossyWarning(node, "hysteria.realm", "sing-box v1.13.14 hysteria2 outbound schema has no realm field"))
	}
	if hy.CWND != 0 {
		warnings = append(warnings, lossyWarning(node, "hysteria.cwnd", "sing-box v1.13.14 hysteria2 outbound schema has no cwnd field"))
	}
	if hy.UDPMTU != 0 {
		warnings = append(warnings, lossyWarning(node, "hysteria.udp_mtu", "sing-box v1.13.14 hysteria2 outbound schema has no udp_mtu field"))
	}
	if len(hy.QUIC) > 0 {
		warnings = append(warnings, lossyWarning(node, "hysteria.quic", "sing-box v1.13.14 hysteria2 outbound schema has no QUIC tuning fields represented by NodeIR"))
	}
	return out, false, nil, warnings, nil
}

func renderTUIC(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing tuic fields")
	}
	out := baseOutbound(node, "tuic")
	if node.UUID != "" {
		out["uuid"] = node.UUID
	}
	if node.Password != "" {
		out["password"] = node.Password
	}
	if node.TUIC != nil {
		if node.TUIC.CongestionControl != "" {
			out["congestion_control"] = node.TUIC.CongestionControl
		}
		if node.TUIC.UDPRelayMode != "" {
			out["udp_relay_mode"] = node.TUIC.UDPRelayMode
		}
		if node.TUIC.ZeroRTTHandshake {
			out["zero_rtt_handshake"] = true
		}
		if node.TUIC.Heartbeat != "" {
			out["heartbeat"] = node.TUIC.Heartbeat
		}
		if node.TUIC.UDPOverStream {
			out["udp_over_stream"] = true
		}
	}
	applyTLS(out, node)
	warnings := []domain.Warning{}
	if node.Token != "" {
		warnings = append(warnings, lossyWarning(node, "token", "sing-box tuic outbound schema uses uuid/password, not token"))
	}
	if node.TUIC != nil {
		if node.TUIC.ReduceRTT {
			warnings = append(warnings, lossyWarning(node, "tuic.reduce_rtt", "sing-box tuic outbound schema has no reduce_rtt field"))
		}
		if node.TUIC.UDPOverStreamVersion != 0 {
			warnings = append(warnings, lossyWarning(node, "tuic.udp_over_stream_version", "sing-box tuic outbound schema has no udp_over_stream_version field"))
		}
	}
	return out, false, nil, warnings, nil
}

func renderSOCKS(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing socks fields")
	}
	out := baseOutbound(node, "socks")
	if node.Username != "" {
		out["username"] = node.Username
	}
	if node.Password != "" {
		out["password"] = node.Password
	}
	if node.Network != "" {
		out["network"] = node.Network
	}
	applyUDPOverTCP(out, node)
	return out, false, nil, nil, nil
}

func renderHTTP(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing http fields")
	}
	out := baseOutbound(node, "http")
	if node.Username != "" {
		out["username"] = node.Username
	}
	if node.Password != "" {
		out["password"] = node.Password
	}
	if node.Path != "" {
		out["path"] = node.Path
	}
	if len(node.Headers) > 0 {
		out["headers"] = node.Headers
	}
	applyTLS(out, node)
	return out, false, nil, nil, nil
}

func renderWireGuard(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.WireGuard == nil || node.WireGuard.PrivateKey == "" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing wireguard fields")
	}
	wg := node.WireGuard
	out := map[string]any{
		"type":        "wireguard",
		"tag":         node.Name,
		"address":     wg.Address,
		"private_key": wg.PrivateKey,
	}
	if len(wg.Address) == 0 {
		address := []string{}
		if wg.IP != "" {
			address = append(address, wg.IP)
		}
		if wg.IPv6 != "" {
			address = append(address, wg.IPv6)
		}
		out["address"] = address
	}
	if wg.MTU != 0 {
		out["mtu"] = wg.MTU
	}
	if wg.Workers != 0 {
		out["workers"] = wg.Workers
	}
	applyDialer(out, node)
	if len(wg.Peers) > 0 {
		peers := make([]map[string]any, 0, len(wg.Peers))
		for _, peer := range wg.Peers {
			item := map[string]any{
				"public_key": peer.PublicKey,
			}
			if peer.Server != "" {
				item["address"] = peer.Server
			}
			if peer.Port != 0 {
				item["port"] = int(peer.Port)
			}
			if peer.PreSharedKey != "" {
				item["pre_shared_key"] = peer.PreSharedKey
			}
			if len(peer.AllowedIPs) > 0 {
				item["allowed_ips"] = peer.AllowedIPs
			}
			if peer.PersistentKeepalive != 0 {
				item["persistent_keepalive_interval"] = peer.PersistentKeepalive
			} else if wg.PersistentKeepalive != 0 {
				item["persistent_keepalive_interval"] = wg.PersistentKeepalive
			}
			if len(peer.Reserved) > 0 {
				item["reserved"] = peer.Reserved
			} else if len(wg.Reserved) > 0 {
				item["reserved"] = wg.Reserved
			}
			peers = append(peers, item)
		}
		out["peers"] = peers
	}
	return out, true, nil, nil, nil
}
