package mihomo_test

import (
	"context"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestParseMihomoWireGuardFlatDefaults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		ip, ipv6  string
		addresses []string
		allowed   []string
	}{
		{name: "bare IPv4", ip: "10.0.0.2", addresses: []string{"10.0.0.2/32"}, allowed: []string{"0.0.0.0/0"}},
		{name: "bare IPv6", ipv6: "fd00::2", addresses: []string{"fd00::2/128"}, allowed: []string{"::/0"}},
		{name: "dual stack", ip: "10.0.0.2/24", ipv6: "fd00::2/64", addresses: []string{"10.0.0.2/24", "fd00::2/64"}, allowed: []string{"0.0.0.0/0", "::/0"}},
		{name: "invalid retained", ip: "not-an-ip", addresses: []string{"not-an-ip"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input, err := json.Marshal(map[string]any{
				"name": "wg", "type": "wireguard", "server": "wg.example", "port": 51820,
				"private-key": "private", "public-key": "public", "ip": tc.ip, "ipv6": tc.ipv6,
				"reserved": []int{1, 2, 3}, "persistent-keepalive": 25,
			})
			require.NoError(t, err)
			nodes, source, err := mihomo.NewParser().Parse(context.Background(), input)
			require.NoError(t, err)
			require.Empty(t, source.Warnings)
			require.Len(t, nodes, 1)
			wg := nodes[0].WireGuard
			require.Equal(t, tc.addresses, wg.Address)
			require.Equal(t, tc.allowed, wg.Peers[0].AllowedIPs)
			require.Equal(t, []uint8{1, 2, 3}, wg.Peers[0].Reserved)
			require.Equal(t, 25, wg.PersistentKeepalive)
			if tc.ip != "" {
				require.Equal(t, tc.addresses[0], wg.IP)
			}
			if tc.ipv6 != "" {
				require.Equal(t, tc.addresses[len(tc.addresses)-1], wg.IPv6)
			}
		})
	}
}

func TestParseMihomoWireGuardExplicitRoutesAndIgnoredTopLevelField(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		peer    bool
		allowed []string
		want    []string
	}{
		{name: "flat ignored restriction", allowed: []string{"10.0.0.0/8"}, want: []string{"0.0.0.0/0"}},
		{name: "explicit missing", peer: true},
		{name: "explicit restriction", peer: true, allowed: []string{"10.0.0.0/8"}, want: []string{"10.0.0.0/8"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			proxy := map[string]any{"name": "wg", "type": "wireguard", "server": "wg.example", "port": 51820, "private-key": "private", "ip": "10.0.0.2"}
			if tc.peer {
				proxy["peers"] = []map[string]any{{"server": "peer.example", "port": 51821, "public-key": "public", "allowed-ips": tc.allowed}}
			} else {
				proxy["public-key"] = "public"
				proxy["allowed-ips"] = tc.allowed
			}
			input, err := json.Marshal(proxy)
			require.NoError(t, err)
			nodes, source, err := mihomo.NewParser().Parse(context.Background(), input)
			require.NoError(t, err)
			require.ElementsMatch(t, tc.want, nodes[0].WireGuard.Peers[0].AllowedIPs)
			if tc.peer {
				require.Empty(t, source.Warnings)
			} else {
				require.JSONEq(t, `["10.0.0.0/8"]`, string(nodes[0].Raw["mihomo.allowed-ips"]))
				require.Len(t, source.Warnings, 1)
				require.Equal(t, "parse_unknown_field", source.Warnings[0].Code)
			}
		})
	}
}

func TestRenderMihomoWireGuardSinglePeerPreservesRoutes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		addresses []string
		allowed   []string
		flat      bool
	}{
		{name: "IPv4 default", addresses: []string{"10.0.0.2/32"}, allowed: []string{"0.0.0.0/0"}, flat: true},
		{name: "IPv6 default", addresses: []string{"fd00::2/128"}, allowed: []string{"::/0"}, flat: true},
		{name: "dual default reversed", addresses: []string{"10.0.0.2/32", "fd00::2/128"}, allowed: []string{"::/0", "0.0.0.0/0"}, flat: true},
		{name: "restricted", addresses: []string{"10.0.0.2/32"}, allowed: []string{"10.0.0.0/8"}},
		{name: "dual IPv4 only", addresses: []string{"10.0.0.2/32", "fd00::2/128"}, allowed: []string{"0.0.0.0/0"}},
		{name: "missing stays missing", addresses: []string{"10.0.0.2/32"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			node := domain.NodeIR{Name: "wg", Type: domain.NodeTypeWireGuard, Server: "outer.example", Port: 51820,
				WireGuard: &domain.WireGuardOptions{PrivateKey: "private", Address: tc.addresses, PersistentKeepalive: 25,
					Peers: []domain.WireGuardPeer{{Server: "peer.example", Port: 51821, PublicKey: "public", AllowedIPs: tc.allowed, Reserved: []uint8{1, 2, 3}}}},
			}
			body, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
			require.NoError(t, err)
			require.Empty(t, report.Warnings)
			var doc struct {
				Proxies []map[string]any `yaml:"proxies"`
			}
			require.NoError(t, yaml.Unmarshal(body, &doc))
			proxy := doc.Proxies[0]
			require.NotContains(t, proxy, "allowed-ips")
			if tc.flat {
				require.NotContains(t, proxy, "peers")
				require.Equal(t, "peer.example", proxy["server"])
				require.Equal(t, 51821, proxy["port"])
			} else {
				require.Contains(t, proxy, "peers")
				require.NotContains(t, proxy, "public-key")
			}
			reparsed, source, err := mihomo.NewParser().Parse(context.Background(), body)
			require.NoError(t, err)
			require.Empty(t, source.Warnings)
			require.ElementsMatch(t, tc.allowed, reparsed[0].WireGuard.Peers[0].AllowedIPs)
			require.Equal(t, "peer.example", reparsed[0].WireGuard.Peers[0].Server)
			require.Equal(t, uint16(51821), reparsed[0].WireGuard.Peers[0].Port)
			require.Equal(t, []uint8{1, 2, 3}, reparsed[0].WireGuard.Peers[0].Reserved)
			require.Equal(t, 25, reparsed[0].WireGuard.PersistentKeepalive)
		})
	}
}
