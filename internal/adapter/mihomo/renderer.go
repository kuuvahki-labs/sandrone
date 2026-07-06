package mihomo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) Name() string {
	return "mihomo-proxies"
}

func (r *Renderer) Render(ctx context.Context, nodes []domain.NodeIR, opt domain.RenderOptions) ([]byte, error) {
	out, _, err := r.RenderWithReport(ctx, nodes, opt)
	return out, err
}

func (r *Renderer) RenderWithReport(ctx context.Context, nodes []domain.NodeIR, opt domain.RenderOptions) ([]byte, domain.RenderReport, error) {
	_ = ctx
	_ = opt
	docs := make([]map[string]any, 0, len(nodes))
	report := domain.RenderReport{}
	for _, node := range nodes {
		doc, skipRaw, warnings, err := nodeToMihomo(node)
		if err != nil {
			shared.MergeWarnings(&report, []domain.Warning{shared.RenderNodeSkippedWarning(node, r.Name(), err)})
			continue
		}
		warnings = append(warnings, mihomoStructuredLossWarnings(node)...)
		warnings = append(warnings, shared.RawWarnings(node, skipRaw, r.Name())...)
		shared.MergeWarnings(&report, warnings)
		report.SuccessCount++
		docs = append(docs, doc)
	}
	if len(nodes) > 0 && report.SuccessCount == 0 {
		return nil, report, shared.NoRenderableNodesError(report)
	}
	body, err := marshalProxiesDocument(docs)
	if err != nil {
		return nil, report, err
	}
	return body, report, nil
}

func nodeToMihomo(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	switch node.Type {
	case domain.NodeTypeShadowsocks:
		return renderSS(node)
	case domain.NodeTypeShadowsocksR:
		return renderSSR(node)
	case domain.NodeTypeSnell:
		return renderSnell(node)
	case domain.NodeTypeAnyTLS:
		return renderAnyTLS(node)
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
	case domain.NodeTypeMieru:
		return renderMieru(node)
	case domain.NodeTypeSOCKS:
		return renderSOCKS(node)
	case domain.NodeTypeHTTP:
		return renderHTTP(node)
	case domain.NodeTypeWireGuard:
		return renderWireGuard(node)
	default:
		return nil, nil, nil, domain.WrapError(domain.CodeRenderFailed, "unsupported node type", fmt.Errorf("%s", node.Type))
	}
}

func renderSS(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Cipher == "" || node.Password == "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing ss fields")
	}
	out := baseProxy(node, "ss")
	out["cipher"] = node.Cipher
	out["password"] = node.Password
	plugin, pluginOptions := renderMihomoSSPlugin(node.Plugin, node.PluginOptions)
	if plugin != "" {
		out["plugin"] = plugin
	}
	if len(pluginOptions) > 0 {
		out["plugin-opts"] = pluginOptions
	}
	applyMihomoTransport(out, node)
	applyMihomoTLS(out, node, "servername")
	applyMihomoUDPOverTCP(out, node)
	return out, nil, nil, nil
}

func renderSSR(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Cipher == "" || node.Password == "" || node.ShadowsocksR == nil || node.ShadowsocksR.Protocol == "" || node.ShadowsocksR.Obfs == "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing ssr fields")
	}
	out := baseProxy(node, "ssr")
	out["cipher"] = node.Cipher
	out["password"] = node.Password
	out["protocol"] = node.ShadowsocksR.Protocol
	if node.ShadowsocksR.ProtocolParam != "" {
		out["protocol-param"] = node.ShadowsocksR.ProtocolParam
	}
	out["obfs"] = node.ShadowsocksR.Obfs
	if node.ShadowsocksR.ObfsParam != "" {
		out["obfs-param"] = node.ShadowsocksR.ObfsParam
	}
	return out, nil, nil, nil
}

func renderSnell(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Password == "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing snell fields")
	}
	out := baseProxy(node, "snell")
	out["psk"] = node.Password
	if node.Snell != nil {
		if node.Snell.Version != 0 {
			out["version"] = node.Snell.Version
		}
		if node.Snell.Reuse != nil {
			out["reuse"] = *node.Snell.Reuse
		}
		if node.Snell.ClientFingerprint != "" {
			out["client-fingerprint"] = node.Snell.ClientFingerprint
		}
		if node.Snell.ShadowTLS != nil {
			shadow := node.Snell.ShadowTLS
			opts := map[string]any{
				"mode":     "shadow-tls",
				"password": shadow.Password,
				"host":     shadow.Host,
				"version":  shadow.Version,
			}
			if len(shadow.ALPN) > 0 {
				opts["alpn"] = shadow.ALPN
			}
			if shadow.Fingerprint != "" {
				opts["fingerprint"] = shadow.Fingerprint
			}
			if shadow.Certificate != "" {
				opts["certificate"] = shadow.Certificate
			}
			if shadow.PrivateKey != "" {
				opts["private-key"] = shadow.PrivateKey
			}
			if shadow.InsecureSkipVerify {
				opts["skip-cert-verify"] = true
			}
			out["obfs-opts"] = opts
		} else if node.Snell.Obfs != "" || node.Snell.ObfsHost != "" {
			out["obfs-opts"] = map[string]any{
				"mode": node.Snell.Obfs,
				"host": node.Snell.ObfsHost,
			}
		}
	}
	return out, nil, nil, nil
}

func renderAnyTLS(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Password == "" || node.AnyTLS == nil || node.TLS == nil || !node.TLS.Enabled {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing anytls fields")
	}
	if node.TLS.Reality != nil && node.TLS.Reality.Enabled {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "mihomo anytls does not support reality")
	}
	out := baseProxy(node, "anytls")
	out["password"] = node.Password
	if node.AnyTLS.IdleSessionCheckInterval != "" {
		seconds, err := wholeDurationSeconds(node.AnyTLS.IdleSessionCheckInterval)
		if err != nil {
			return nil, nil, nil, err
		}
		out["idle-session-check-interval"] = seconds
	}
	if node.AnyTLS.IdleSessionTimeout != "" {
		seconds, err := wholeDurationSeconds(node.AnyTLS.IdleSessionTimeout)
		if err != nil {
			return nil, nil, nil, err
		}
		out["idle-session-timeout"] = seconds
	}
	if node.AnyTLS.MinIdleSession != 0 {
		out["min-idle-session"] = node.AnyTLS.MinIdleSession
	}
	applyMihomoTLS(out, node, "sni")
	return out, nil, nil, nil
}

func wholeDurationSeconds(value string) (int64, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 || duration%time.Second != 0 {
		return 0, domain.NewError(domain.CodeRenderFailed, "anytls durations must be positive whole seconds")
	}
	return int64(duration / time.Second), nil
}

func renderMihomoSSPlugin(plugin string, options map[string]any) (string, map[string]any) {
	outOptions := options
	if isSimpleObfsPlugin(plugin) {
		if normalized := mihomoSimpleObfsOptions(options); len(normalized) > 0 {
			return "obfs", normalized
		}
		return "obfs", outOptions
	}
	return plugin, outOptions
}

func isSimpleObfsPlugin(plugin string) bool {
	switch strings.ToLower(strings.TrimSpace(plugin)) {
	case "obfs", "obfs-local", "simple-obfs":
		return true
	default:
		return false
	}
}

func mihomoSimpleObfsOptions(options map[string]any) map[string]any {
	if len(options) == 0 {
		return nil
	}
	if rawValue, ok := options["raw"]; ok {
		return parseSIP002SimpleObfsOptions(fmt.Sprint(rawValue))
	}
	out := map[string]any{}
	if mode, ok := firstNonEmptyOptionString(options, "mode", "obfs"); ok {
		out["mode"] = mode
	}
	if host, ok := firstNonEmptyOptionString(options, "host", "obfs-host"); ok {
		out["host"] = host
	}
	return out
}

func parseSIP002SimpleObfsOptions(raw string) map[string]any {
	out := map[string]any{}
	for _, part := range strings.Split(raw, ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "obfs", "mode":
			if value != "" {
				out["mode"] = value
			}
		case "obfs-host", "host":
			if value != "" {
				out["host"] = value
			}
		}
	}
	return out
}

func firstNonEmptyOptionString(options map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		value, ok := options[key]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			return text, true
		}
	}
	return "", false
}

func renderVMess(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.UUID == "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing vmess fields")
	}
	out := baseProxy(node, "vmess")
	out["uuid"] = node.UUID
	if node.Cipher != "" {
		out["cipher"] = node.Cipher
	}
	out["alterId"] = node.AlterID
	applyMihomoTransport(out, node)
	applyMihomoTLS(out, node, "servername")
	if node.PacketEncoding != "" {
		out["packet-encoding"] = node.PacketEncoding
	}
	return out, map[string]bool{"vmess.alter_id": true}, nil, nil
}

func renderVLESS(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.UUID == "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing vless fields")
	}
	out := baseProxy(node, "vless")
	out["uuid"] = node.UUID
	if node.Flow != "" {
		out["flow"] = node.Flow
	}
	if node.Encryption != "" {
		out["encryption"] = node.Encryption
	}
	if node.PacketEncoding != "" {
		out["packet-encoding"] = node.PacketEncoding
	}
	applyMihomoTransport(out, node)
	applyMihomoTLS(out, node, "servername")
	return out, nil, nil, nil
}

func renderTrojan(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Password == "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing trojan fields")
	}
	out := baseProxy(node, "trojan")
	out["password"] = node.Password
	applyMihomoTransport(out, node)
	applyMihomoTLS(out, node, "sni")
	return out, nil, nil, nil
}

func renderTUIC(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing tuic fields")
	}
	out := baseProxy(node, "tuic")
	if node.Token != "" {
		out["token"] = node.Token
	}
	if node.UUID != "" {
		out["uuid"] = node.UUID
	}
	if node.Password != "" {
		out["password"] = node.Password
	}
	if node.TUIC != nil {
		if node.TUIC.CongestionControl != "" {
			out["congestion-controller"] = node.TUIC.CongestionControl
		}
		if node.TUIC.UDPRelayMode != "" {
			out["udp-relay-mode"] = node.TUIC.UDPRelayMode
		}
		if node.TUIC.ReduceRTT {
			out["reduce-rtt"] = true
		}
		if node.TUIC.UDPOverStream {
			out["udp-over-stream"] = true
		}
		if node.TUIC.UDPOverStreamVersion != 0 {
			out["udp-over-stream-version"] = node.TUIC.UDPOverStreamVersion
		}
	}
	applyMihomoTLS(out, node, "sni")
	return out, nil, nil, nil
}

func renderMieru(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Username == "" || node.Password == "" || node.Mieru == nil || node.Mieru.Transport == "" || (node.Port == 0 && node.Mieru.PortRange == "") {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing mieru fields")
	}
	out := map[string]any{
		"name":      node.Name,
		"type":      "mieru",
		"server":    node.Server,
		"transport": node.Mieru.Transport,
		"username":  node.Username,
		"password":  node.Password,
	}
	if node.Port != 0 {
		out["port"] = int(node.Port)
	} else {
		out["port-range"] = node.Mieru.PortRange
	}
	applyMihomoDialer(out, node)
	if node.Mieru.Multiplexing != "" {
		out["multiplexing"] = node.Mieru.Multiplexing
	}
	if node.Mieru.HandshakeMode != "" {
		out["handshake-mode"] = node.Mieru.HandshakeMode
	}
	if node.Mieru.TrafficPattern != "" {
		out["traffic-pattern"] = node.Mieru.TrafficPattern
	}
	return out, nil, nil, nil
}

func renderSOCKS(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing socks fields")
	}
	out := baseProxy(node, "socks5")
	if node.Username != "" {
		out["username"] = node.Username
	}
	if node.Password != "" {
		out["password"] = node.Password
	}
	applyMihomoTLS(out, node, "")
	return out, nil, nil, nil
}

func renderHTTP(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing http fields")
	}
	out := baseProxy(node, "http")
	if node.Username != "" {
		out["username"] = node.Username
	}
	if node.Password != "" {
		out["password"] = node.Password
	}
	if len(node.Headers) > 0 {
		out["headers"] = node.Headers
	}
	applyMihomoTLS(out, node, "sni")
	return out, nil, nil, nil
}

func renderWireGuard(node domain.NodeIR) (map[string]any, map[string]bool, []domain.Warning, error) {
	if node.WireGuard == nil || node.WireGuard.PrivateKey == "" {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing wireguard fields")
	}
	out := baseProxy(node, "wireguard")
	wg := node.WireGuard
	out["private-key"] = wg.PrivateKey
	if wg.IP != "" {
		out["ip"] = wg.IP
	} else if len(wg.Address) > 0 {
		out["ip"] = wg.Address[0]
	}
	if wg.IPv6 != "" {
		out["ipv6"] = wg.IPv6
	} else if len(wg.Address) > 1 {
		out["ipv6"] = wg.Address[1]
	}
	if wg.MTU != 0 {
		out["mtu"] = wg.MTU
	}
	if wg.Workers != 0 {
		out["workers"] = wg.Workers
	}
	if wg.PersistentKeepalive != 0 {
		out["persistent-keepalive"] = wg.PersistentKeepalive
	}
	if len(wg.Peers) == 1 {
		peer := wg.Peers[0]
		if node.Server == "" && peer.Server != "" {
			out["server"] = peer.Server
		}
		if node.Port == 0 && peer.Port != 0 {
			out["port"] = int(peer.Port)
		}
		out["public-key"] = peer.PublicKey
		if peer.PreSharedKey != "" {
			out["pre-shared-key"] = peer.PreSharedKey
		}
		if len(peer.AllowedIPs) > 0 {
			out["allowed-ips"] = peer.AllowedIPs
		}
		if len(peer.Reserved) > 0 {
			out["reserved"] = peer.Reserved
		}
	} else if len(wg.Peers) > 1 {
		peers := make([]map[string]any, 0, len(wg.Peers))
		for _, peer := range wg.Peers {
			item := map[string]any{}
			if peer.Server != "" {
				item["server"] = peer.Server
			}
			if peer.Port != 0 {
				item["port"] = int(peer.Port)
			}
			item["public-key"] = peer.PublicKey
			if peer.PreSharedKey != "" {
				item["pre-shared-key"] = peer.PreSharedKey
			}
			if len(peer.AllowedIPs) > 0 {
				item["allowed-ips"] = peer.AllowedIPs
			}
			if len(peer.Reserved) > 0 {
				item["reserved"] = peer.Reserved
			}
			peers = append(peers, item)
		}
		out["peers"] = peers
	}
	return out, nil, nil, nil
}
