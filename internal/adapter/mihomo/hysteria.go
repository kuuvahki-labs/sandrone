package mihomo

import (
	"strconv"
	"strings"

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
	if node.Server == "" || node.Port == 0 {
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "missing hysteria2 fields")
	}
	out := baseProxy(node, "hysteria2")
	if node.Password != "" {
		out["password"] = node.Password
	}
	hy := node.Hysteria
	if hy == nil {
		hy = &domain.HysteriaOptions{}
	}
	if len(hy.ServerPorts) > 0 {
		out["ports"] = strings.Join(hy.ServerPorts, ",")
	}
	if hy.HopInterval != "" {
		out["hop-interval"] = hy.HopInterval
	}
	if hy.Up != "" {
		out["up"] = hy.Up
	}
	if hy.Down != "" {
		out["down"] = hy.Down
	}
	if hy.Obfs != "" {
		out["obfs"] = hy.Obfs
	}
	if hy.ObfsPassword != "" {
		out["obfs-password"] = hy.ObfsPassword
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
	if hy.Realm != nil {
		out["realm-opts"] = map[string]any{
			"enable":       hy.Realm.Enabled,
			"server-url":   hy.Realm.ServerURL,
			"token":        hy.Realm.Token,
			"realm-id":     hy.Realm.RealmID,
			"stun-servers": hy.Realm.STUNServers,
		}
	}
	applyMihomoTLS(out, node, "sni")
	return out, nil, nil, nil
}
