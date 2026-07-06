package singbox_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestParseSingBoxAllProtocols(t *testing.T) {
	input := []byte(`{
  "outbounds": [
    {
      "type": "shadowsocks",
      "tag": "ss",
      "server": "ss.example.com",
      "server_port": 8388,
      "method": "aes-128-gcm",
      "password": "secret"
    },
    {
      "type": "vmess",
      "tag": "vmess",
      "server": "vmess.example.com",
      "server_port": 443,
      "uuid": "11111111-1111-1111-1111-111111111111",
      "security": "auto",
      "alter_id": 0,
      "tls": { "enabled": true, "server_name": "vmess.example.com" },
      "transport": { "type": "ws", "path": "/ws", "headers": { "Host": "cdn.example.com" } }
    },
    {
      "type": "vless",
      "tag": "vless",
      "server": "vless.example.com",
      "server_port": 443,
      "uuid": "11111111-1111-1111-1111-111111111112",
      "flow": "xtls-rprx-vision",
      "tls": { "enabled": true, "reality": { "enabled": true, "public_key": "public", "short_id": "abcd" } }
    },
    {
      "type": "trojan",
      "tag": "trojan",
      "server": "trojan.example.com",
      "server_port": 443,
      "password": "secret",
      "tls": { "enabled": true, "server_name": "trojan.example.com" }
    },
    {
      "type": "hysteria",
      "tag": "hysteria",
      "server": "hy.example.com",
      "server_port": 8443,
      "up": "20 Mbps",
      "down": "100 Mbps",
      "auth_str": "secret",
      "obfs": "obfs",
      "tls": { "enabled": true, "server_name": "hy.example.com" }
    },
    {
      "type": "hysteria2",
      "tag": "hysteria2",
      "server": "hy2.example.com",
      "server_port": 8443,
      "password": "secret",
      "obfs": { "type": "salamander", "password": "obfs" },
      "bbr_profile": "desktop"
    },
    {
      "type": "tuic",
      "tag": "tuic",
      "server": "tuic.example.com",
      "server_port": 443,
      "uuid": "11111111-1111-1111-1111-111111111113",
      "password": "secret",
      "congestion_control": "bbr",
      "udp_relay_mode": "native"
    },
    {
      "type": "socks",
      "tag": "socks",
      "server": "socks.example.com",
      "server_port": 1080,
      "username": "user",
      "password": "pass"
    },
    {
      "type": "http",
      "tag": "http",
      "server": "http.example.com",
      "server_port": 8080,
      "username": "user",
      "password": "pass",
      "headers": { "X-Test": "yes" }
    }
  ],
  "endpoints": [
    {
      "type": "wireguard",
      "tag": "wireguard",
      "address": ["10.0.0.2/32"],
      "private_key": "private",
      "peers": [
        {
          "address": "wg.example.com",
          "port": 51820,
          "public_key": "public",
          "pre_shared_key": "psk",
          "allowed_ips": ["0.0.0.0/0"]
        }
      ],
      "mtu": 1408
    }
  ]
}`)
	parser := singbox.NewParser()
	nodes, source, err := parser.Parse(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 10)
	require.Equal(t, domain.NodeTypeShadowsocks, nodes[0].Type)
	require.Equal(t, domain.NodeTypeVMess, nodes[1].Type)
	require.Equal(t, "websocket", nodes[1].Transport.Type)
	require.NotNil(t, nodes[2].TLS.Reality)
	require.Equal(t, domain.NodeTypeWireGuard, nodes[9].Type)
	require.Equal(t, "private", nodes[9].WireGuard.PrivateKey)
	require.Len(t, nodes[9].WireGuard.Peers, 1)
}

func TestParseSingBoxTransportVariants(t *testing.T) {
	parser := singbox.NewParser()
	input := []byte(`{
  "outbounds": [
    {
      "type": "trojan",
      "tag": "grpc-node",
      "server": "example.com",
      "server_port": 443,
      "password": "secret",
      "transport": { "type": "grpc", "service_name": "mysvc" },
      "multiplex": { "enabled": true, "max_connections": 4 }
    },
    {
      "type": "shadowsocks",
      "tag": "ss-udp",
      "server": "example.com",
      "server_port": 8388,
      "method": "aes-128-gcm",
      "password": "secret",
      "udp_over_tcp": { "enabled": true, "version": 2 }
    }
  ]
}`)
	nodes, _, err := parser.Parse(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, "grpc", nodes[0].Transport.Type)
	require.Equal(t, "mysvc", nodes[0].Transport.ServiceName)
	require.NotNil(t, nodes[0].Multiplex)
	require.True(t, nodes[0].Multiplex.Enabled)
	require.NotNil(t, nodes[1].UDPOverTCP)
	require.True(t, nodes[1].UDPOverTCP.Enabled)
	require.Equal(t, 2, nodes[1].UDPOverTCP.Version)
}

func TestParseSingBoxTLSFingerprintSplit(t *testing.T) {
	parser := singbox.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`{
  "outbounds": [{
    "type": "vless",
    "tag": "tls-split",
    "server": "example.com",
    "server_port": 443,
    "uuid": "11111111-1111-1111-1111-111111111111",
    "tls": {
      "enabled": true,
      "utls": { "enabled": true, "fingerprint": "chrome" }
    }
  }]
}`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].TLS)
	require.Equal(t, "chrome", nodes[0].TLS.ClientFingerprint)
	require.Empty(t, nodes[0].TLS.Fingerprint)
}

func TestParseSingBoxTCPFastOpenToDialer(t *testing.T) {
	parser := singbox.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`{
  "outbounds": [{
    "type": "hysteria2",
    "tag": "hy2",
    "server": "example.com",
    "server_port": 8443,
    "password": "secret",
    "tcp_fast_open": true
  }]
}`))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].Dialer)
	require.True(t, nodes[0].Dialer.TFO)
	require.NotContains(t, nodes[0].Raw, "sing-box.tcp_fast_open")
}

func TestParseSingBoxWireGuardEndpointReserved(t *testing.T) {
	parser := singbox.NewParser()
	input := []byte(`{
  "endpoints": [{
    "type": "wireguard",
    "tag": "wg",
    "address": ["10.0.0.2/32"],
    "private_key": "private",
    "peers": [{
      "address": "wg.example.com",
      "port": 51820,
      "public_key": "public",
      "pre_shared_key": "psk",
      "allowed_ips": ["0.0.0.0/0"],
      "reserved": [1, 2, 3]
    }],
    "mtu": 1408
  }]
}`)
	nodes, _, err := parser.Parse(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, []uint8{1, 2, 3}, nodes[0].WireGuard.Peers[0].Reserved)
}

func TestParseSingBoxHTTPTransport(t *testing.T) {
	parser := singbox.NewParser()
	input := []byte(`{
  "outbounds": [
    {
      "type": "vmess",
      "tag": "http",
      "server": "example.com",
      "server_port": 443,
      "uuid": "11111111-1111-1111-1111-111111111111",
      "transport": {
        "type": "http",
        "host": ["h1.example.com"],
        "path": "/api",
        "method": "GET",
        "headers": { "User-Agent": "curl" }
      }
    },
    {
      "type": "vless",
      "tag": "upgrade",
      "server": "example.com",
      "server_port": 443,
      "uuid": "11111111-1111-1111-1111-111111111112",
      "transport": { "type": "httpupgrade", "host": "cdn.example.com", "path": "/up" }
    }
  ]
}`)
	nodes, _, err := parser.Parse(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, "http", nodes[0].Transport.Type)
	require.Equal(t, "GET", nodes[0].Transport.Method)
	require.Equal(t, "httpupgrade", nodes[1].Transport.Type)
}

func TestParseSingBoxUnknownFieldsGoToRaw(t *testing.T) {
	parser := singbox.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`{
  "outbounds": [
    {
      "type": "socks",
      "tag": "socks",
      "server": "example.com",
      "server_port": 1080,
      "password": "secret",
      "private_thing": "value",
      "another_private_thing": 42
    }
  ]
}`))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	require.Contains(t, nodes[0].Raw, "sing-box.private_thing")
	require.Contains(t, nodes[0].Raw, "sing-box.another_private_thing")
	require.NotContains(t, nodes[0].Raw, "sing-box.password")
	require.NotContains(t, nodes[0].Raw, "sing-box.tag")
	require.Len(t, source.Warnings, 2)
	for _, warning := range source.Warnings {
		require.NotNil(t, warning.NodeIndex)
		require.Equal(t, 0, *warning.NodeIndex)
		require.NotNil(t, warning.NodeContext)
		require.Equal(t, "sing-box", warning.NodeContext.Format)
		require.Equal(t, "socks", warning.NodeContext.Name)
		require.Equal(t, domain.NodeTypeSOCKS, warning.NodeContext.Type)
		require.Equal(t, "example.com", warning.NodeContext.Server)
		require.Equal(t, uint16(1080), warning.NodeContext.Port)
		require.Equal(t, "secret", warning.NodeContext.Raw["password"])
		require.Equal(t, "value", warning.NodeContext.Raw["private_thing"])
		require.Equal(t, json.Number("42"), warning.NodeContext.Raw["another_private_thing"])
	}
}

func TestParseSingBoxSingleOutboundDocument(t *testing.T) {
	parser := singbox.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`{
  "type": "shadowsocks",
  "tag": "single",
  "server": "example.com",
  "server_port": 8388,
  "method": "aes-128-gcm",
  "password": "secret",
  "plugin": "obfs-local",
  "plugin_opts": "mode=tls",
  "network": "udp"
}`))
	require.NoError(t, err)
	require.Equal(t, "sing-box", parser.Name())
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeShadowsocks, nodes[0].Type)
	require.Equal(t, "single", nodes[0].Name)
	require.Equal(t, "obfs-local", nodes[0].Plugin)
	require.Equal(t, map[string]any{"raw": "mode=tls"}, nodes[0].PluginOptions)
	require.Equal(t, "udp", nodes[0].Network)
}

func TestParseSingBoxTLSFullOptions(t *testing.T) {
	parser := singbox.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`{
  "outbounds": [{
    "type": "vless",
    "tag": "tls-full",
    "server": "example.com",
    "server_port": 443,
    "uuid": "11111111-1111-1111-1111-111111111111",
    "tls": {
      "enabled": true,
      "server_name": "sni.example.com",
      "insecure": true,
      "disable_sni": true,
      "alpn": ["h2", "http/1.1"],
      "utls": { "fingerprint": "chrome" },
      "ech": {
        "enabled": true,
        "config": ["ech-config"],
        "query_server_name": "ech.example.com"
      },
      "reality": {
        "public_key": "public",
        "short_id": "abcd"
      }
    }
  }]
}`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	tls := nodes[0].TLS
	require.NotNil(t, tls)
	require.True(t, tls.Enabled)
	require.Equal(t, "sni.example.com", tls.ServerName)
	require.True(t, tls.InsecureSkipVerify)
	require.True(t, tls.DisableSNI)
	require.Equal(t, []string{"h2", "http/1.1"}, tls.ALPN)
	require.Equal(t, "chrome", tls.ClientFingerprint)
	require.NotNil(t, tls.ECH)
	require.True(t, tls.ECH.Enabled)
	require.Equal(t, []string{"ech-config"}, tls.ECH.Config)
	require.Equal(t, "ech.example.com", tls.ECH.QueryServerName)
	require.NotNil(t, tls.Reality)
	require.True(t, tls.Reality.Enabled)
	require.Equal(t, "public", tls.Reality.PublicKey)
}

func TestParseSingBoxWireGuardDeprecatedCompatibilityFieldsStayRaw(t *testing.T) {
	parser := singbox.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`{
  "endpoints": [{
    "type": "wireguard",
    "tag": "wg",
    "server_port": 51820,
    "local_address": ["10.0.0.2/32", "fd00::2/128"],
    "private_key": "private",
    "peer_public_key": "public",
    "pre_shared_key": "psk",
    "allowed_ips": ["0.0.0.0/0"],
    "reserved": [1, 2, 300, -1],
    "workers": 2
  }]
}`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Empty(t, got.WireGuard.Address)
	require.Equal(t, 2, got.WireGuard.Workers)
	require.Empty(t, got.WireGuard.Peers)
	require.Contains(t, got.Raw, "sing-box.local_address")
	require.Contains(t, got.Raw, "sing-box.peer_public_key")
	require.Contains(t, got.Raw, "sing-box.pre_shared_key")
	require.Contains(t, got.Raw, "sing-box.allowed_ips")
	require.NotEmpty(t, source.Warnings)
}

func TestParseSingBoxHysteria2RealmAndQUIC(t *testing.T) {
	parser := singbox.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`{
  "outbounds": [{
    "type": "hysteria2",
    "tag": "hy2",
    "server": "example.com",
    "server_port": 8443,
    "password": "secret",
    "server_ports": ["8443", "9443"],
    "hop_interval": "30s",
    "up_mbps": 20,
    "down_mbps": 100,
    "obfs": { "type": "salamander", "password": "obfs-pass" },
    "bbr_profile": "desktop",
    "realm": {
      "server_url": "https://realm.example.com",
      "token": "token",
      "realm_id": "realm-id",
      "stun_servers": ["stun.example.com"]
    },
    "initial_packet_size": 1200,
    "idle_timeout": "30s"
  }]
}`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	hy := nodes[0].Hysteria
	require.NotNil(t, hy)
	require.Equal(t, []string{"8443", "9443"}, hy.ServerPorts)
	require.Equal(t, "30s", hy.HopInterval)
	require.Equal(t, 20, hy.UpMbps)
	require.Equal(t, 100, hy.DownMbps)
	require.Equal(t, "salamander", hy.Obfs)
	require.Equal(t, "obfs-pass", hy.ObfsPassword)
	require.Empty(t, hy.BBRProfile)
	require.Nil(t, hy.Realm)
	require.Nil(t, hy.QUIC)
	require.Contains(t, nodes[0].Raw, "sing-box.bbr_profile")
	require.Contains(t, nodes[0].Raw, "sing-box.realm")
	require.Contains(t, nodes[0].Raw, "sing-box.initial_packet_size")
	require.Contains(t, nodes[0].Raw, "sing-box.idle_timeout")
	require.Len(t, source.Warnings, 4)
}

func TestParseSingBoxTUICAndBoolUDPOverTCP(t *testing.T) {
	parser := singbox.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`{
  "outbounds": [{
    "type": "tuic",
    "tag": "tuic",
    "server": "example.com",
    "server_port": 443,
    "uuid": "11111111-1111-1111-1111-111111111111",
    "password": "secret",
    "congestion_control": "bbr",
    "udp_relay_mode": "native",
    "zero_rtt_handshake": true,
    "heartbeat": "10s",
    "udp_over_stream": true,
    "udp_over_tcp": true
  }]
}`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].TUIC)
	require.Equal(t, "bbr", nodes[0].TUIC.CongestionControl)
	require.Equal(t, "native", nodes[0].TUIC.UDPRelayMode)
	require.True(t, nodes[0].TUIC.ZeroRTTHandshake)
	require.True(t, nodes[0].TUIC.UDPOverStream)
	require.Equal(t, "10s", nodes[0].TUIC.Heartbeat)
	require.NotNil(t, nodes[0].UDPOverTCP)
	require.True(t, nodes[0].UDPOverTCP.Enabled)
}

func TestParseSingBoxSkipsExpectedNonNodeOutboundsWithoutWarnings(t *testing.T) {
	parser := singbox.NewParser()

	nodes, source, err := parser.Parse(context.Background(), []byte(`{
  "outbounds": [
    {
      "type": "selector",
      "tag": "select",
      "outbounds": ["good", "direct"]
    },
    {
      "type": "vless",
      "tag": "good",
      "server": "vless.example.com",
      "server_port": 443,
      "uuid": "11111111-1111-1111-1111-111111111111"
    },
    {
      "type": "direct",
      "tag": "direct"
    }
  ]
}`))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	require.Equal(t, "good", nodes[0].Name)
	require.Empty(t, source.Warnings)
}

func TestParseSingBoxSkipsBadOutboundsWithWarnings(t *testing.T) {
	parser := singbox.NewParser()

	nodes, source, err := parser.Parse(context.Background(), []byte(`{
  "outbounds": [
    {
      "type": "vless",
      "tag": "good",
      "server": "vless.example.com",
      "server_port": 443,
      "uuid": "11111111-1111-1111-1111-111111111111"
    },
    "bad",
    {
      "type": "mieru",
      "tag": "mieru",
      "server": "mieru.example.com",
      "server_port": 8964
    }
  ]
}`))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	require.Equal(t, "good", nodes[0].Name)
	require.Len(t, source.Warnings, 2)
	first := source.Warnings[0]
	require.Equal(t, "parse_outbound_skipped", first.Code)
	require.Equal(t, "sing-box", first.Source)
	require.NotNil(t, first.NodeIndex)
	require.Equal(t, 1, *first.NodeIndex)
	require.NotNil(t, first.NodeContext)
	require.Equal(t, "sing-box", first.NodeContext.Format)
	require.NotNil(t, first.NodeContext.Raw)

	second := source.Warnings[1]
	require.Equal(t, "parse_outbound_skipped", second.Code)
	require.Equal(t, "sing-box", second.Source)
	require.NotNil(t, second.NodeIndex)
	require.Equal(t, 2, *second.NodeIndex)
	require.NotNil(t, second.NodeContext)
	require.Equal(t, "mieru", second.NodeContext.Name)
	require.Equal(t, "mieru.example.com", second.NodeContext.Server)
	require.Equal(t, uint16(8964), second.NodeContext.Port)
	require.Equal(t, "mieru", second.NodeContext.Raw["type"])
}

func TestParseSingBoxRejectsInvalidInputs(t *testing.T) {
	parser := singbox.NewParser()

	_, _, err := parser.Parse(context.Background(), []byte(`{`))
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeParseFailed))

	_, _, err = parser.Parse(context.Background(), []byte(`{"outbounds":[]}`))
	require.Error(t, err)
	require.ErrorContains(t, err, "no sing-box outbounds found")

	_, _, err = parser.Parse(context.Background(), []byte(`{"outbounds":["bad"]}`))
	require.Error(t, err)
	require.ErrorContains(t, err, "no sing-box outbounds found")

	_, _, err = parser.Parse(context.Background(), []byte(`{"outbounds":[{"type":"mieru","tag":"mieru"}]}`))
	require.Error(t, err)
	require.ErrorContains(t, err, "no sing-box outbounds found")

	_, _, err = parser.Parse(context.Background(), []byte(`{"type":"direct","tag":"direct"}`))
	require.Error(t, err)
	require.ErrorContains(t, err, "no sing-box outbounds found")

	_, _, err = parser.Parse(context.Background(), []byte(`{"type":"mieru","tag":"mieru"}`))
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported sing-box outbound type")
}
