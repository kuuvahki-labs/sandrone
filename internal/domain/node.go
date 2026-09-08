package domain

import (
	"encoding/json/jsontext"
)

type NodeType string

const (
	NodeTypeShadowsocks  NodeType = "ss"
	NodeTypeShadowsocksR NodeType = "ssr"
	NodeTypeVMess        NodeType = "vmess"
	NodeTypeVLESS        NodeType = "vless"
	NodeTypeTrojan       NodeType = "trojan"
	NodeTypeHysteria     NodeType = "hysteria"
	NodeTypeHysteria2    NodeType = "hysteria2"
	NodeTypeTUIC         NodeType = "tuic"
	NodeTypeMieru        NodeType = "mieru"
	NodeTypeSOCKS        NodeType = "socks"
	NodeTypeHTTP         NodeType = "http"
	NodeTypeWireGuard    NodeType = "wireguard"
	NodeTypeSnell        NodeType = "snell"
	NodeTypeAnyTLS       NodeType = "anytls"
)

type NodeIR struct {
	runtimeID      string
	Name           string                    `json:"name" yaml:"name"`
	Type           NodeType                  `json:"type" yaml:"type"`
	Server         string                    `json:"server" yaml:"server"`
	Port           uint16                    `json:"port" yaml:"port"`
	Network        string                    `json:"network,omitempty" yaml:"network,omitempty"`
	Username       string                    `json:"username,omitempty" yaml:"username,omitempty"`
	Password       string                    `json:"password,omitempty" yaml:"password,omitempty"`
	UUID           string                    `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Cipher         string                    `json:"cipher,omitempty" yaml:"cipher,omitempty"`
	AlterID        int                       `json:"alter_id,omitzero" yaml:"alter_id,omitempty"`
	Flow           string                    `json:"flow,omitempty" yaml:"flow,omitempty"`
	Encryption     string                    `json:"encryption,omitempty" yaml:"encryption,omitempty"`
	Token          string                    `json:"token,omitempty" yaml:"token,omitempty"`
	PacketEncoding string                    `json:"packet_encoding,omitempty" yaml:"packet_encoding,omitempty"`
	Plugin         string                    `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	PluginOptions  map[string]any            `json:"plugin_options,omitempty" yaml:"plugin_options,omitempty"`
	ShadowsocksR   *ShadowsocksROptions      `json:"shadowsocksr,omitzero" yaml:"shadowsocksr,omitempty"`
	Snell          *SnellOptions             `json:"snell,omitzero" yaml:"snell,omitempty"`
	AnyTLS         *AnyTLSOptions            `json:"anytls,omitzero" yaml:"anytls,omitempty"`
	Headers        map[string]string         `json:"headers,omitempty" yaml:"headers,omitempty"`
	Path           string                    `json:"path,omitempty" yaml:"path,omitempty"`
	TLS            *TLSOptions               `json:"tls,omitzero" yaml:"tls,omitempty"`
	Dialer         *DialerOptions            `json:"dialer,omitzero" yaml:"dialer,omitempty"`
	Transport      *TransportOptions         `json:"transport,omitzero" yaml:"transport,omitempty"`
	Multiplex      *MultiplexOptions         `json:"multiplex,omitzero" yaml:"multiplex,omitempty"`
	UDPOverTCP     *UDPOverTCPOptions        `json:"udp_over_tcp,omitzero" yaml:"udp_over_tcp,omitempty"`
	Hysteria       *HysteriaOptions          `json:"hysteria,omitzero" yaml:"hysteria,omitempty"`
	TUIC           *TUICOptions              `json:"tuic,omitzero" yaml:"tuic,omitempty"`
	Mieru          *MieruOptions             `json:"mieru,omitzero" yaml:"mieru,omitempty"`
	WireGuard      *WireGuardOptions         `json:"wireguard,omitzero" yaml:"wireguard,omitempty"`
	Tags           []string                  `json:"tags,omitempty" yaml:"tags,omitempty"`
	Meta           map[string]string         `json:"meta,omitempty" yaml:"meta,omitempty"`
	Raw            map[string]jsontext.Value `json:"raw,omitempty" yaml:"raw,omitempty"`
	Lossy          bool                      `json:"lossy,omitzero" yaml:"lossy,omitempty"`
	Warnings       []Warning                 `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	SourceFormat   string                    `json:"source_format,omitempty" yaml:"source_format,omitempty"`
}
