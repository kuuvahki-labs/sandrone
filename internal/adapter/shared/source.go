package shared

import (
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const (
	MihomoModule         = "github.com/metacubex/mihomo@v1.19.25"
	SingBoxModule        = "github.com/sagernet/sing-box@v1.13.14"
	ShadowrocketRepo     = "github.com/LOWERTOP/Shadowrocket"
	ShadowrocketRevision = "5f1916b5897fc59fb7172aca59ae52050a3532fe"
)

func SourceRefs(format string) []domain.SourceRef {
	switch format {
	case "ss", "shadowsocks":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "SIP002",
			URL:      "https://shadowsocks.org/doc/sip002.html",
			Revision: "spec",
			Note:     "Shadowsocks URI scheme",
		}}
	case "ssr", "shadowsocksr":
		return []domain.SourceRef{{
			Kind:  "implementation",
			Name:  "ShadowsocksR URI convention",
			Repo:  "github.com/tindy2013/subconverter",
			Path:  "src/parser/subparser.cpp",
			Lines: "928-994",
			Note:  "subconverter-compatible ShadowsocksR URI parser; SSR is not a Shadowsocks SIP standard",
		}}
	case "snell":
		return []domain.SourceRef{{
			Kind:     "implementation",
			Name:     "mihomo snell outbound schema",
			Repo:     MihomoModule,
			Revision: "v1.19.25",
			Path:     "adapter/outbound/snell.go",
			Lines:    "25-34",
			Note:     "proxy tags for Snell fields",
		}}
	case "vmess":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "vmess",
			URL:      "https://raw.githubusercontent.com/v2fly/v2fly-github-io/master/docs/developer/protocols/vmess.md",
			Revision: "spec",
			Note:     "VMess URI is base64 JSON",
		}}
	case "vless":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "VLESS",
			URL:      "https://xtls.github.io/development/protocols/vless.html",
			Revision: "spec",
			Note:     "VLESS protocol and URI profile",
		}}
	case "trojan":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "trojan",
			URL:      "https://raw.githubusercontent.com/trojan-gfw/trojan/master/docs/protocol.md",
			Revision: "spec",
			Note:     "Trojan protocol and common URI profile",
		}}
	case "hysteria":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "Hysteria v1",
			URL:      "https://v1.hysteria.network/docs/protocol/",
			Revision: "spec",
			Note:     "Hysteria v1 protocol",
		}}
	case "hysteria2", "hy2":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "Hysteria 2",
			URL:      "https://raw.githubusercontent.com/apernet/hysteria/master/PROTOCOL.md",
			Revision: "spec",
			Note:     "Hysteria 2 protocol",
		}}
	case "tuic":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "TUIC",
			URL:      "https://raw.githubusercontent.com/tuic-protocol/tuic/master/SPEC.md",
			Revision: "spec",
			Note:     "TUIC protocol",
		}}
	case "mieru":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "Mieru README",
			URL:      "https://github.com/enfein/mieru/blob/main/README.md",
			Revision: "main",
			Note:     "Mieru project and client profile overview",
		}, {
			Kind:     "protocol",
			Name:     "Mieru protocol",
			URL:      "https://github.com/enfein/mieru/blob/main/docs/protocol.md",
			Revision: "main",
			Note:     "Mieru protocol details",
		}, {
			Kind:     "protocol",
			Name:     "Mieru traffic pattern",
			URL:      "https://github.com/enfein/mieru/blob/main/docs/traffic-pattern.md",
			Revision: "main",
			Note:     "Mieru traffic pattern encoding",
		}, mihomoRef("mihomo mieru outbound schema", "/adapter/outbound/mieru.go", "30-42", "proxy tags for Mieru fields")}
	case "socks":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "RFC 1928",
			URL:      "https://www.rfc-editor.org/rfc/rfc1928",
			Revision: "spec",
			Note:     "SOCKS5 protocol",
		}, {
			Kind:     "protocol",
			Name:     "RFC 1929",
			URL:      "https://www.rfc-editor.org/rfc/rfc1929",
			Revision: "spec",
			Note:     "SOCKS username/password authentication",
		}}
	case "http":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "RFC 9110",
			URL:      "https://www.rfc-editor.org/rfc/rfc9110",
			Revision: "spec",
			Note:     "HTTP CONNECT proxy semantics",
		}}
	case "wireguard":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "WireGuard whitepaper",
			URL:      "https://www.wireguard.com/papers/wireguard.pdf",
			Revision: "spec",
			Note:     "WireGuard protocol",
		}}
	case "mihomo", "mihomo-proxies":
		return []domain.SourceRef{{
			Kind:     "implementation",
			Name:     "mihomo outbound adapter schemas",
			Repo:     MihomoModule,
			Revision: "v1.19.25",
			Path:     "adapter/outbound/vmess.go",
			Lines:    "43-100",
			Note:     "proxy tags for VMess and shared transport options",
		}, {
			Kind:     "implementation",
			Name:     "mihomo vless outbound schema",
			Repo:     MihomoModule,
			Revision: "v1.19.25",
			Path:     "adapter/outbound/vless.go",
			Lines:    "50-135",
			Note:     "proxy tags for VLESS, XHTTP, Reality and TLS fields",
		}, {
			Kind:     "implementation",
			Name:     "mihomo hysteria outbound schema",
			Repo:     MihomoModule,
			Revision: "v1.19.25",
			Path:     "adapter/outbound/hysteria.go",
			Lines:    "100-127",
			Note:     "proxy tags for Hysteria v1 fields",
		}, {
			Kind:     "implementation",
			Name:     "mihomo hysteria2 outbound schema",
			Repo:     MihomoModule,
			Revision: "v1.19.25",
			Path:     "adapter/outbound/hysteria2.go",
			Lines:    "45-85",
			Note:     "proxy tags for Hysteria2 fields",
		}, {
			Kind:     "implementation",
			Name:     "mihomo tuic outbound schema",
			Repo:     MihomoModule,
			Revision: "v1.19.25",
			Path:     "adapter/outbound/tuic.go",
			Lines:    "35-68",
			Note:     "proxy tags for TUIC fields",
		}, {
			Kind:     "implementation",
			Name:     "mihomo mieru outbound schema",
			Repo:     MihomoModule,
			Revision: "v1.19.25",
			Path:     "adapter/outbound/mieru.go",
			Lines:    "30-42",
			Note:     "proxy tags for Mieru fields",
		}, {
			Kind:     "implementation",
			Name:     "mihomo wireguard outbound schema",
			Repo:     MihomoModule,
			Revision: "v1.19.25",
			Path:     "adapter/outbound/wireguard.go",
			Lines:    "57-85",
			Note:     "proxy tags for WireGuard fields",
		}}
	case "sing-box", "sing-box-outbounds":
		return []domain.SourceRef{{
			Kind:     "implementation",
			Name:     "sing-box outbound option schemas",
			Repo:     SingBoxModule,
			Revision: "v1.13.14",
			Path:     "option/hysteria.go",
			Lines:    "29-45",
			Note:     "JSON tags for Hysteria outbound fields",
		}, {
			Kind:     "implementation",
			Name:     "sing-box hysteria2 outbound schema",
			Repo:     SingBoxModule,
			Revision: "v1.13.14",
			Path:     "option/hysteria2.go",
			Lines:    "112-124",
			Note:     "JSON tags for Hysteria2 outbound fields",
		}, {
			Kind:     "implementation",
			Name:     "sing-box wireguard endpoint schema",
			Repo:     SingBoxModule,
			Revision: "v1.13.14",
			Path:     "option/wireguard.go",
			Lines:    "9-30",
			Note:     "JSON tags for WireGuard endpoint fields",
		}, {
			Kind:     "implementation",
			Name:     "sing-box outbound TLS schema",
			Repo:     SingBoxModule,
			Revision: "v1.13.14",
			Path:     "option/tls.go",
			Lines:    "97-122, 229-237",
			Note:     "JSON tags for outbound TLS, uTLS, ECH and Reality fields",
		}, {
			Kind:     "implementation",
			Name:     "sing-box v2ray transport schema",
			Repo:     SingBoxModule,
			Revision: "v1.13.14",
			Path:     "option/v2ray_transport.go",
			Lines:    "79-92",
			Note:     "JSON tags for WebSocket early data and gRPC fields",
		}}
	case "shadowrocket", "shadowrocket-proxies":
		return []domain.SourceRef{shadowrocketRef()}
	default:
		return nil
	}
}

func SourceRefFor(format string, protocol domain.NodeType) domain.SourceRef {
	if ref, ok := adapterSourceRefs[format][protocol]; ok {
		return ref
	}
	if ref, ok := adapterSourceRefs[canonicalSourceFormat(format)][protocol]; ok {
		return ref
	}
	refs := SourceRefs(string(protocol))
	if len(refs) == 0 && protocol == domain.NodeTypeShadowsocks {
		refs = SourceRefs("ss")
	}
	if len(refs) == 0 {
		refs = SourceRefs(format)
	}
	if len(refs) == 0 {
		return domain.SourceRef{}
	}
	return refs[0]
}

func canonicalSourceFormat(format string) string {
	switch format {
	case "mihomo-proxies":
		return "mihomo"
	case "sing-box-outbounds":
		return "sing-box"
	case "shadowrocket-proxies":
		return "shadowrocket"
	case "uri", "uri-list", "base64":
		return "uri-list"
	default:
		return format
	}
}

var adapterSourceRefs = map[string]map[domain.NodeType]domain.SourceRef{
	"mihomo": {
		domain.NodeTypeShadowsocks:  mihomoRef("mihomo shadowsocks outbound schema", "/adapter/outbound/shadowsocks.go", "41-53", "proxy tags for Shadowsocks fields"),
		domain.NodeTypeShadowsocksR: mihomoRef("mihomo shadowsocksr outbound schema", "/adapter/outbound/shadowsocksr.go", "28-40", "proxy tags for ShadowsocksR fields"),
		domain.NodeTypeSnell:        mihomoRef("mihomo snell outbound schema", "/adapter/outbound/snell.go", "25-34", "proxy tags for Snell fields"),
		domain.NodeTypeVMess:        mihomoRef("mihomo vmess outbound schema", "/adapter/outbound/vmess.go", "43-100", "proxy tags for VMess and shared transport fields"),
		domain.NodeTypeVLESS:        mihomoRef("mihomo vless outbound schema", "/adapter/outbound/vless.go", "50-134", "proxy tags for VLESS, XHTTP, Reality and TLS fields"),
		domain.NodeTypeTrojan:       mihomoRef("mihomo trojan outbound schema", "/adapter/outbound/trojan.go", "39-57", "proxy tags for Trojan, transport and TLS fields"),
		domain.NodeTypeHysteria:     mihomoRef("mihomo hysteria outbound schema", "/adapter/outbound/hysteria.go", "100-126", "proxy tags for Hysteria v1 fields"),
		domain.NodeTypeHysteria2:    mihomoRef("mihomo hysteria2 outbound schema", "/adapter/outbound/hysteria2.go", "39-84", "proxy tags for Hysteria2 fields"),
		domain.NodeTypeTUIC:         mihomoRef("mihomo tuic outbound schema", "/adapter/outbound/tuic.go", "34-68", "proxy tags for TUIC fields"),
		domain.NodeTypeMieru:        mihomoRef("mihomo mieru outbound schema", "/adapter/outbound/mieru.go", "30-42", "proxy tags for Mieru fields"),
		domain.NodeTypeSOCKS:        mihomoRef("mihomo socks5 outbound schema", "/adapter/outbound/socks5.go", "30-42", "proxy tags for SOCKS fields"),
		domain.NodeTypeHTTP:         mihomoRef("mihomo http outbound schema", "/adapter/outbound/http.go", "28-41", "proxy tags for HTTP proxy fields"),
		domain.NodeTypeWireGuard:    mihomoRef("mihomo wireguard outbound schema", "/adapter/outbound/wireguard.go", "57-76", "proxy tags for WireGuard fields"),
	},
	"sing-box": {
		domain.NodeTypeShadowsocks: singBoxRef("sing-box shadowsocks outbound schema", "/option/shadowsocks.go", "25-34", "JSON tags for Shadowsocks outbound fields"),
		domain.NodeTypeVMess:       singBoxRef("sing-box vmess outbound schema", "/option/vmess.go", "17-29", "JSON tags for VMess outbound fields"),
		domain.NodeTypeVLESS:       singBoxRef("sing-box vless outbound schema", "/option/vless.go", "17-26", "JSON tags for VLESS outbound fields"),
		domain.NodeTypeTrojan:      singBoxRef("sing-box trojan outbound schema", "/option/trojan.go", "18-25", "JSON tags for Trojan outbound fields"),
		domain.NodeTypeHysteria:    singBoxRef("sing-box hysteria outbound schema", "/option/hysteria.go", "29-44", "JSON tags for Hysteria outbound fields"),
		domain.NodeTypeHysteria2:   singBoxRef("sing-box hysteria2 outbound schema", "/option/hysteria2.go", "112-123", "JSON tags for Hysteria2 outbound fields"),
		domain.NodeTypeTUIC:        singBoxRef("sing-box tuic outbound schema", "/option/tuic.go", "21-31", "JSON tags for TUIC outbound fields"),
		domain.NodeTypeSOCKS:       singBoxRef("sing-box socks outbound schema", "/option/simple.go", "22-29", "JSON tags for SOCKS outbound fields"),
		domain.NodeTypeHTTP:        singBoxRef("sing-box http outbound schema", "/option/simple.go", "32-39", "JSON tags for HTTP outbound fields"),
		domain.NodeTypeWireGuard:   singBoxRef("sing-box wireguard endpoint schema", "/option/wireguard.go", "9-29", "JSON tags for WireGuard endpoint fields"),
	},
	"shadowrocket": {
		domain.NodeTypeShadowsocks: shadowrocketRef(),
		domain.NodeTypeVMess:       shadowrocketRef(),
		domain.NodeTypeVLESS:       shadowrocketRef(),
		domain.NodeTypeTrojan:      shadowrocketRef(),
		domain.NodeTypeHysteria:    shadowrocketRef(),
		domain.NodeTypeHysteria2:   shadowrocketRef(),
		domain.NodeTypeTUIC:        shadowrocketRef(),
		domain.NodeTypeHTTP:        shadowrocketRef(),
		domain.NodeTypeSOCKS:       shadowrocketRef(),
		domain.NodeTypeWireGuard:   shadowrocketRef(),
		domain.NodeTypeSnell:       shadowrocketRef(),
	},
}

func mihomoRef(name, path, lines, note string) domain.SourceRef {
	return domain.SourceRef{
		Kind:     "implementation",
		Name:     name,
		Repo:     MihomoModule,
		Revision: "v1.19.25",
		Path:     strings.TrimPrefix(path, "/"),
		Lines:    lines,
		Note:     note,
	}
}

func singBoxRef(name, path, lines, note string) domain.SourceRef {
	return domain.SourceRef{
		Kind:     "implementation",
		Name:     name,
		Repo:     SingBoxModule,
		Revision: "v1.13.14",
		Path:     strings.TrimPrefix(path, "/"),
		Lines:    lines,
		Note:     note,
	}
}

func shadowrocketRef() domain.SourceRef {
	return domain.SourceRef{
		Kind:     "implementation",
		Name:     "Shadowrocket local node configuration examples",
		Repo:     ShadowrocketRepo,
		Revision: ShadowrocketRevision,
		Path:     "README.md",
		Lines:    "1222-1271",
		Note:     "documented field names and value forms for local proxy lines",
	}
}

func SourceInfo(format string, refs ...[]domain.SourceRef) *domain.SourceInfo {
	info := &domain.SourceInfo{Format: format}
	for _, set := range refs {
		info.SourceRefs = append(info.SourceRefs, set...)
	}
	return info
}
