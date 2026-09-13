package singbox

import (
	"context"
	"fmt"
	"strings"

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
	for index, node := range nodes {
		doc, endpoint, skipRaw, warnings, err := nodeToSingBox(node)
		if err != nil {
			warning := shared.RenderNodeSkippedWarning(node, r.Name(), err)
			warning.NodeIndex = &index
			shared.MergeWarnings(&report, []domain.Warning{warning})
			continue
		}
		warnings = append(warnings, singBoxStructuredLossWarnings(node)...)
		warnings = append(warnings, shared.RawWarnings(node, skipRaw, r.Name())...)
		for warningIndex := range warnings {
			warnings[warningIndex].NodeIndex = &index
		}
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
	if field := singBoxUnsupportedECHDNSField(node); field != "" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, field+" requires a custom ECH DNS transport that sing-box cannot express")
	}
	if node.Network != "" && node.Network != "tcp" && node.Network != "udp" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "network must be tcp or udp")
	}
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

func singBoxUnsupportedECHDNSField(node domain.NodeIR) string {
	if node.TLS != nil && node.TLS.ECH != nil && node.TLS.ECH.DNS != "" {
		return "tls.ech.dns"
	}
	if node.Transport != nil && node.Transport.XHTTP != nil && node.Transport.XHTTP.DownloadSettings != nil {
		tls := node.Transport.XHTTP.DownloadSettings.TLS
		if tls != nil && tls.ECH != nil && tls.ECH.DNS != "" {
			return "transport.xhttp.download_settings.tls.ech.dns"
		}
	}
	return ""
}

func renderSS(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Cipher == "" || node.Password == "" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing ss fields")
	}
	out := baseOutbound(node, "shadowsocks")
	out["method"] = node.Cipher
	out["password"] = node.Password
	plugin, pluginOptions, err := renderSingBoxSSPlugin(node.Plugin, node.PluginOptions)
	if err != nil {
		return nil, false, nil, nil, err
	}
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
	if err := applyPacketEncoding(out, node.PacketEncoding); err != nil {
		return nil, false, nil, nil, err
	}
	applyTLS(out, node)
	if err := applyTransport(out, node); err != nil {
		return nil, false, nil, nil, err
	}
	applyMux(out, node)
	return out, false, map[string]bool{"vmess.alter_id": true}, nil, nil
}

func renderVLESS(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.UUID == "" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing vless fields")
	}
	if node.Encryption != "" && node.Encryption != "none" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "sing-box vless outbound schema does not support non-default encryption")
	}
	out := baseOutbound(node, "vless")
	out["uuid"] = node.UUID
	if node.Flow != "" {
		out["flow"] = node.Flow
	}
	if err := applyPacketEncoding(out, node.PacketEncoding); err != nil {
		return nil, false, nil, nil, err
	}
	applyTLS(out, node)
	if err := applyTransport(out, node); err != nil {
		return nil, false, nil, nil, err
	}
	applyMux(out, node)
	return out, false, nil, nil, nil
}

func applyPacketEncoding(out map[string]any, value string) error {
	switch normalized := strings.ToLower(strings.TrimSpace(value)); normalized {
	case "":
		return nil
	case "none":
		// sing-box represents an explicitly disabled VLESS packet encoding as an
		// empty string. Omitting the field is not equivalent: VLESS defaults to
		// xudp when packet_encoding is absent.
		out["packet_encoding"] = ""
		return nil
	case "packetaddr", "xudp":
		out["packet_encoding"] = normalized
		return nil
	default:
		return domain.NewError(
			domain.CodeRenderFailed,
			fmt.Sprintf("unsupported sing-box packet_encoding %q", value),
		)
	}
}

func renderTrojan(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Password == "" {
		return nil, false, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing trojan fields")
	}
	out := baseOutbound(node, "trojan")
	out["password"] = node.Password
	applyTLS(out, node)
	if err := applyTransport(out, node); err != nil {
		return nil, false, nil, nil, err
	}
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

func renderTUIC(node domain.NodeIR) (map[string]any, bool, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.TLS == nil || !node.TLS.Enabled {
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
