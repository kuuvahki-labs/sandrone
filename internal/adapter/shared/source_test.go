package shared_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestSourceRefs(t *testing.T) {
	tests := []struct {
		format string
		name   string
		kind   string
		count  int
	}{
		{format: "ss", name: "SIP002", kind: "protocol", count: 1},
		{format: "shadowsocks", name: "SIP002", kind: "protocol", count: 1},
		{format: "vmess", name: "vmess", kind: "protocol", count: 1},
		{format: "vmess-aead", name: "VMessAEAD / VLESS sharing link", kind: "protocol", count: 1},
		{format: "vless", name: "VLESS", kind: "protocol", count: 1},
		{format: "trojan", name: "trojan", kind: "protocol", count: 1},
		{format: "hysteria", name: "Hysteria v1", kind: "protocol", count: 1},
		{format: "hysteria2", name: "Hysteria 2", kind: "protocol", count: 1},
		{format: "hy2", name: "Hysteria 2", kind: "protocol", count: 1},
		{format: "tuic", name: "TUIC", kind: "protocol", count: 1},
		{format: "mieru", name: "Mieru README", kind: "protocol", count: 4},
		{format: "socks", name: "RFC 1928", kind: "protocol", count: 2},
		{format: "http", name: "RFC 9110", kind: "protocol", count: 1},
		{format: "wireguard", name: "WireGuard whitepaper", kind: "protocol", count: 1},
		{format: "mihomo", name: "mihomo outbound adapter schemas", kind: "implementation", count: 7},
		{format: "sing-box", name: "sing-box outbound option schemas", kind: "implementation", count: 5},
	}

	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			refs := shared.SourceRefs(tc.format)
			require.Len(t, refs, tc.count)
			require.Equal(t, tc.name, refs[0].Name)
			require.Equal(t, tc.kind, refs[0].Kind)
		})
	}

	require.Nil(t, shared.SourceRefs("unknown"))
}

func TestSourceInfoCombinesRefs(t *testing.T) {
	info := shared.SourceInfo("uri-list",
		[]domain.SourceRef{{Name: "one"}},
		[]domain.SourceRef{{Name: "two"}, {Name: "three"}},
	)

	require.Equal(t, "uri-list", info.Format)
	require.Equal(t, []domain.SourceRef{{Name: "one"}, {Name: "two"}, {Name: "three"}}, info.SourceRefs)
}

func TestSourceRefForTargetSchema(t *testing.T) {
	mihomo := shared.SourceRefFor("mihomo-proxies", domain.NodeTypeShadowsocks)
	require.Equal(t, "mihomo shadowsocks outbound schema", mihomo.Name)
	require.Equal(t, "adapter/outbound/shadowsocks.go", mihomo.Path)
	require.Equal(t, "41-53", mihomo.Lines)

	singBox := shared.SourceRefFor("sing-box-outbounds", domain.NodeTypeWireGuard)
	require.Equal(t, "sing-box wireguard endpoint schema", singBox.Name)
	require.Equal(t, "option/wireguard.go", singBox.Path)
	require.Equal(t, "9-29", singBox.Lines)
	require.Equal(t, "github.com/sagernet/sing-box@v1.13.14", singBox.Repo)
	require.Equal(t, "v1.13.14", singBox.Revision)

	uri := shared.SourceRefFor("uri-list", domain.NodeTypeTrojan)
	require.Equal(t, "trojan", uri.Name)
	require.Equal(t, "protocol", uri.Kind)

	mieru := shared.SourceRefFor("mihomo-proxies", domain.NodeTypeMieru)
	require.Equal(t, "mihomo mieru outbound schema", mieru.Name)
	require.Equal(t, "adapter/outbound/mieru.go", mieru.Path)
	require.Equal(t, "30-42", mieru.Lines)
}
