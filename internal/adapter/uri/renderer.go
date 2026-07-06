package uri

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) Name() string {
	return "uri-list"
}

func (r *Renderer) Render(ctx context.Context, nodes []domain.NodeIR, opt domain.RenderOptions) ([]byte, error) {
	out, _, err := r.RenderWithReport(ctx, nodes, opt)
	return out, err
}

func (r *Renderer) RenderWithReport(_ context.Context, nodes []domain.NodeIR, _ domain.RenderOptions) ([]byte, domain.RenderReport, error) {
	lines := make([]string, 0, len(nodes))
	report := domain.RenderReport{}
	for _, node := range nodes {
		line, warnings, err := nodeToURI(node)
		if err != nil {
			shared.MergeWarnings(&report, []domain.Warning{shared.RenderNodeSkippedWarning(node, r.Name(), err)})
			continue
		}
		if line == "" {
			warnings = append([]domain.Warning{shared.RenderNodeSkippedWarning(node, r.Name(), nil)}, warnings...)
			shared.MergeWarnings(&report, warnings)
			continue
		}
		warnings = append(warnings, structuredLossWarnings(node, r.Name())...)
		warnings = append(warnings, shared.RawWarnings(node, nil, r.Name())...)
		shared.MergeWarnings(&report, warnings)
		report.SuccessCount++
		lines = append(lines, line)
	}
	out := []byte(strings.Join(lines, "\n"))
	if len(nodes) > 0 && report.SuccessCount == 0 {
		return out, report, shared.NoRenderableNodesError(report)
	}
	return out, report, nil
}

func nodeToURI(node domain.NodeIR) (string, []domain.Warning, error) {
	switch node.Type {
	case domain.NodeTypeShadowsocks:
		return renderSSURI(node)
	case domain.NodeTypeShadowsocksR:
		return renderSSRURI(node)
	case domain.NodeTypeVMess:
		return renderVMessURI(node)
	case domain.NodeTypeVLESS:
		return renderVLESSURI(node)
	case domain.NodeTypeTrojan:
		return renderTrojanURI(node)
	case domain.NodeTypeHysteria:
		return renderHysteriaURI(node)
	case domain.NodeTypeHysteria2:
		return renderHysteria2URI(node)
	case domain.NodeTypeTUIC:
		return renderTUICURI(node)
	case domain.NodeTypeAnyTLS:
		return renderAnyTLSURI(node)
	case domain.NodeTypeMieru:
		return renderMieruURI(node)
	case domain.NodeTypeSOCKS:
		return renderSOCKSURI(node)
	case domain.NodeTypeHTTP:
		return renderHTTPURI(node)
	default:
		return "", []domain.Warning{unsupportedURIWarning(node)}, nil
	}
}

func renderSSURI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Cipher == "" || node.Password == "" {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing ss URI fields")
	}
	credentials := base64.RawURLEncoding.EncodeToString([]byte(node.Cipher + ":" + node.Password))
	u := "ss://" + credentials + "@" + hostPort(node.Server, node.Port)
	query := url.Values{}
	if node.Plugin != "" {
		query.Set("plugin", renderSIP002Plugin(node.Plugin, node.PluginOptions))
	}
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return withFragment(u, node.Name), nil, nil
}

func renderSSRURI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Cipher == "" || node.Password == "" || node.ShadowsocksR == nil || node.ShadowsocksR.Protocol == "" || node.ShadowsocksR.Obfs == "" {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing ssr URI fields")
	}
	password := base64.RawURLEncoding.EncodeToString([]byte(node.Password))
	base := strings.Join([]string{node.Server, strconv.Itoa(int(node.Port)), node.ShadowsocksR.Protocol, node.Cipher, node.ShadowsocksR.Obfs, password}, ":")
	query := url.Values{}
	if node.Name != "" {
		query.Set("remarks", base64.RawURLEncoding.EncodeToString([]byte(node.Name)))
	}
	if node.ShadowsocksR.ProtocolParam != "" {
		query.Set("protoparam", base64.RawURLEncoding.EncodeToString([]byte(node.ShadowsocksR.ProtocolParam)))
	}
	if node.ShadowsocksR.ObfsParam != "" {
		query.Set("obfsparam", base64.RawURLEncoding.EncodeToString([]byte(node.ShadowsocksR.ObfsParam)))
	}
	payload := base
	if len(query) > 0 {
		payload += "/?" + query.Encode()
	}
	return "ssr://" + base64.RawURLEncoding.EncodeToString([]byte(payload)), nil, nil
}

func renderVMessURI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.UUID == "" {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing vmess URI fields")
	}
	doc := map[string]any{
		"v":    "2",
		"ps":   node.Name,
		"add":  node.Server,
		"port": fmt.Sprint(node.Port),
		"id":   node.UUID,
		"aid":  fmt.Sprint(node.AlterID),
		"scy":  firstNonEmptyURI(node.Cipher, "auto"),
		"net":  "tcp",
		"type": "none",
	}
	if node.Transport != nil {
		switch node.Transport.Type {
		case "websocket", "ws":
			doc["net"] = "ws"
			doc["host"] = firstNonEmptyURI(node.Transport.Host, node.Transport.Headers["Host"])
			doc["path"] = node.Transport.Path
		case "grpc":
			doc["net"] = "grpc"
			doc["path"] = node.Transport.ServiceName
		default:
			doc["net"] = node.Transport.Type
			doc["path"] = node.Transport.Path
			doc["host"] = node.Transport.Host
		}
	}
	if node.TLS != nil && node.TLS.Enabled {
		doc["tls"] = "tls"
		if node.TLS.ServerName != "" {
			doc["sni"] = node.TLS.ServerName
		}
		if node.TLS.ClientFingerprint != "" {
			doc["fp"] = node.TLS.ClientFingerprint
		}
		if node.TLS.Fingerprint != "" {
			doc["pcs"] = node.TLS.Fingerprint
		}
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return "", nil, err
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(body), nil, nil
}

func renderVLESSURI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.UUID == "" {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing vless URI fields")
	}
	u := &url.URL{
		Scheme:   "vless",
		User:     url.User(node.UUID),
		Host:     hostPort(node.Server, node.Port),
		Fragment: node.Name,
	}
	q := u.Query()
	if node.Flow != "" {
		q.Set("flow", node.Flow)
	}
	if node.Encryption != "" {
		q.Set("encryption", node.Encryption)
	} else {
		q.Set("encryption", "none")
	}
	if node.PacketEncoding != "" {
		q.Set("packet-encoding", node.PacketEncoding)
	}
	applyURIQueryTLS(q, node)
	applyURIQueryTransport(q, node)
	u.RawQuery = q.Encode()
	return u.String(), nil, nil
}

func renderTrojanURI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Password == "" {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing trojan URI fields")
	}
	u := &url.URL{
		Scheme:   "trojan",
		User:     url.User(node.Password),
		Host:     hostPort(node.Server, node.Port),
		Fragment: node.Name,
	}
	q := u.Query()
	applyURIQueryTLS(q, node)
	applyURIQueryTransport(q, node)
	u.RawQuery = q.Encode()
	return u.String(), nil, nil
}

func renderAnyTLSURI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 || node.Password == "" || node.AnyTLS == nil || node.TLS == nil || !node.TLS.Enabled {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing anytls URI fields")
	}
	u := &url.URL{
		Scheme:   "anytls",
		User:     url.User(node.Password),
		Host:     hostPort(node.Server, node.Port),
		Fragment: node.Name,
	}
	q := u.Query()
	if node.AnyTLS.IdleSessionCheckInterval != "" {
		q.Set("idle-session-check-interval", node.AnyTLS.IdleSessionCheckInterval)
	}
	if node.AnyTLS.IdleSessionTimeout != "" {
		q.Set("idle-session-timeout", node.AnyTLS.IdleSessionTimeout)
	}
	if node.AnyTLS.MinIdleSession != 0 {
		q.Set("min-idle-session", strconv.Itoa(node.AnyTLS.MinIdleSession))
	}
	applyURIQueryTLS(q, node)
	u.RawQuery = q.Encode()
	return u.String(), nil, nil
}

func renderHysteriaURI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing hysteria URI fields")
	}
	u := &url.URL{Scheme: "hysteria", Host: hostPort(node.Server, node.Port), Fragment: node.Name}
	q := u.Query()
	if node.Hysteria != nil {
		if node.Hysteria.Protocol != "" {
			q.Set("protocol", node.Hysteria.Protocol)
		}
		if auth := firstNonEmptyURI(node.Hysteria.Auth, node.Hysteria.AuthString); auth != "" {
			q.Set("auth", auth)
		}
		obfsMode, obfsPassword := shared.HysteriaV1Obfs(node)
		if obfsMode == "" && obfsPassword != "" {
			obfsMode = "xplus"
		}
		if obfsMode != "" {
			q.Set("obfs", obfsMode)
		}
		if obfsPassword != "" {
			q.Set("obfsParam", obfsPassword)
		}
		if node.Hysteria.UpMbps != 0 {
			q.Set("upmbps", strconv.Itoa(node.Hysteria.UpMbps))
		}
		if node.Hysteria.DownMbps != 0 {
			q.Set("downmbps", strconv.Itoa(node.Hysteria.DownMbps))
		}
	}
	applyHysteriaURIQueryTLS(q, node)
	u.RawQuery = q.Encode()
	return u.String(), nil, nil
}

func renderHysteria2URI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing hysteria2 URI fields")
	}
	u := &url.URL{Scheme: "hy2", Host: hysteria2URIHost(node), Fragment: node.Name}
	if node.Password != "" {
		u.User = url.User(node.Password)
	}
	q := u.Query()
	if node.Hysteria != nil {
		if node.Hysteria.Obfs != "" {
			q.Set("obfs", node.Hysteria.Obfs)
		}
		if node.Hysteria.ObfsPassword != "" {
			q.Set("obfs-password", node.Hysteria.ObfsPassword)
		}
	}
	applyHysteria2URIQueryTLS(q, node)
	u.RawQuery = q.Encode()
	return u.String(), nil, nil
}

func renderTUICURI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing tuic URI fields")
	}
	u := &url.URL{Scheme: "tuic", Host: hostPort(node.Server, node.Port), Fragment: node.Name}
	if node.UUID != "" {
		if node.Password != "" {
			u.User = url.UserPassword(node.UUID, node.Password)
		} else {
			u.User = url.User(node.UUID)
		}
	}
	q := u.Query()
	if node.Token != "" {
		q.Set("token", node.Token)
	}
	if node.TUIC != nil {
		if node.TUIC.CongestionControl != "" {
			q.Set("congestion_control", node.TUIC.CongestionControl)
		}
		if node.TUIC.UDPRelayMode != "" {
			q.Set("udp_relay_mode", node.TUIC.UDPRelayMode)
		}
		if node.TUIC.ZeroRTTHandshake {
			q.Set("zero_rtt_handshake", "true")
		}
	}
	applyURIQueryTLS(q, node)
	u.RawQuery = q.Encode()
	return u.String(), nil, nil
}

func renderMieruURI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Username == "" || node.Password == "" || node.Mieru == nil || node.Mieru.Transport == "" || (node.Port == 0 && node.Mieru.PortRange == "") {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing mieru URI fields")
	}
	portSpec := node.Mieru.PortRange
	if node.Port != 0 {
		portSpec = strconv.Itoa(int(node.Port))
	}
	u := &url.URL{
		Scheme: "mierus",
		User:   url.UserPassword(node.Username, node.Password),
		Host:   hostOnly(node.Server),
	}
	q := u.Query()
	q.Set("port", portSpec)
	q.Set("protocol", node.Mieru.Transport)
	if profile := mieruProfileName(node.Name, portSpec, node.Mieru.Transport); profile != "" {
		q.Set("profile", profile)
	}
	if node.Mieru.Multiplexing != "" {
		q.Set("multiplexing", node.Mieru.Multiplexing)
	}
	if node.Mieru.HandshakeMode != "" {
		q.Set("handshake-mode", node.Mieru.HandshakeMode)
	}
	if node.Mieru.TrafficPattern != "" {
		q.Set("traffic-pattern", node.Mieru.TrafficPattern)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil, nil
}

func renderSOCKSURI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing socks URI fields")
	}
	u := &url.URL{Scheme: "socks5", Host: hostPort(node.Server, node.Port), Fragment: node.Name}
	if node.Username != "" {
		if node.Password != "" {
			u.User = url.UserPassword(node.Username, node.Password)
		} else {
			u.User = url.User(node.Username)
		}
	}
	return u.String(), nil, nil
}

func renderHTTPURI(node domain.NodeIR) (string, []domain.Warning, error) {
	if node.Server == "" || node.Port == 0 {
		return "", nil, domain.NewError(domain.CodeRenderFailed, "missing http URI fields")
	}
	scheme := "http"
	if node.TLS != nil && node.TLS.Enabled {
		scheme = "https"
	}
	u := &url.URL{Scheme: scheme, Host: hostPort(node.Server, node.Port), Fragment: node.Name}
	if node.Username != "" {
		if node.Password != "" {
			u.User = url.UserPassword(node.Username, node.Password)
		} else {
			u.User = url.User(node.Username)
		}
	}
	return u.String(), nil, nil
}

func applyURIQueryTLS(q url.Values, node domain.NodeIR) {
	if node.TLS == nil {
		return
	}
	if node.TLS.Reality != nil {
		q.Set("security", "reality")
		if node.TLS.Reality.PublicKey != "" {
			q.Set("pbk", node.TLS.Reality.PublicKey)
		}
		if node.TLS.Reality.ShortID != "" {
			q.Set("sid", node.TLS.Reality.ShortID)
		}
	} else if node.TLS.Enabled {
		q.Set("security", "tls")
	}
	if node.TLS.ServerName != "" {
		q.Set("sni", node.TLS.ServerName)
	}
	if node.TLS.InsecureSkipVerify {
		q.Set("allowInsecure", "true")
	}
	if len(node.TLS.ALPN) > 0 {
		q.Set("alpn", strings.Join(node.TLS.ALPN, ","))
	}
	if node.TLS.ClientFingerprint != "" {
		q.Set("fp", node.TLS.ClientFingerprint)
	}
	if node.TLS.Fingerprint != "" {
		q.Set("fingerprint", node.TLS.Fingerprint)
	}
	if node.TLS.ECH != nil {
		if ech := renderECHQuery(node.TLS.ECH); ech != "" {
			q.Set("ech", ech)
		}
		if node.TLS.ECH.ForceQuery != "" {
			q.Set("echForceQuery", node.TLS.ECH.ForceQuery)
		}
	}
}

func applyHysteriaURIQueryTLS(q url.Values, node domain.NodeIR) {
	if node.TLS == nil {
		return
	}
	if node.TLS.ServerName != "" {
		q.Set("peer", node.TLS.ServerName)
	}
	if node.TLS.InsecureSkipVerify {
		q.Set("insecure", "1")
	}
	if len(node.TLS.ALPN) > 0 {
		q.Set("alpn", strings.Join(node.TLS.ALPN, ","))
	}
}

func applyHysteria2URIQueryTLS(q url.Values, node domain.NodeIR) {
	if node.TLS == nil {
		return
	}
	if node.TLS.ServerName != "" {
		q.Set("sni", node.TLS.ServerName)
	}
	if node.TLS.InsecureSkipVerify {
		q.Set("insecure", "1")
	}
	if node.TLS.Fingerprint != "" {
		q.Set("pinSHA256", node.TLS.Fingerprint)
	}
}

func applyURIQueryTransport(q url.Values, node domain.NodeIR) {
	if node.Transport == nil || node.Transport.Type == "" {
		return
	}
	switch node.Transport.Type {
	case "websocket", "ws":
		q.Set("type", "ws")
		if node.Transport.Path != "" {
			q.Set("path", node.Transport.Path)
		}
		if node.Transport.Host != "" {
			q.Set("host", node.Transport.Host)
		}
	case "grpc":
		q.Set("type", "grpc")
		if node.Transport.ServiceName != "" {
			q.Set("serviceName", node.Transport.ServiceName)
		}
	case "http":
		q.Set("type", "h2")
		if node.Transport.Path != "" {
			q.Set("path", node.Transport.Path)
		}
		if node.Transport.Host != "" {
			q.Set("host", node.Transport.Host)
		}
	case "xhttp":
		q.Set("type", "xhttp")
		if node.Transport.Path != "" {
			q.Set("path", node.Transport.Path)
		}
		if node.Transport.Host != "" {
			q.Set("host", node.Transport.Host)
		}
		if node.Transport.XHTTP != nil {
			if node.Transport.XHTTP.Mode != "" {
				q.Set("mode", node.Transport.XHTTP.Mode)
			}
			if extra := renderVLESSXHTTPExtra(node.Transport); extra != "" {
				q.Set("extra", extra)
			}
		}
	default:
		q.Set("type", node.Transport.Type)
	}
}
