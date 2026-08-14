package mihomo_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestParseMihomoAllProtocols(t *testing.T) {
	input := []byte(`
proxies:
  - name: ss
    type: ss
    server: ss.example.com
    port: 8388
    cipher: aes-128-gcm
    password: secret
  - name: vmess
    type: vmess
    server: vmess.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    cipher: auto
    alterId: 0
    network: ws
    ws-opts:
      path: /ws
      headers:
        Host: cdn.example.com
    tls: true
    servername: vmess.example.com
  - name: vless
    type: vless
    server: vless.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111112
    flow: xtls-rprx-vision
    encryption: none
    tls: true
    reality-opts:
      public-key: public
      short-id: abcd
  - name: trojan
    type: trojan
    server: trojan.example.com
    port: 443
    password: secret
    sni: trojan.example.com
  - name: hysteria
    type: hysteria
    server: hy.example.com
    port: 8443
    up: 20 Mbps
    down: 100 Mbps
    auth-str: secret
    obfs: obfs
    sni: hy.example.com
  - name: hysteria2
    type: hysteria2
    server: hy2.example.com
    port: 8443
    password: secret
    obfs: salamander
    obfs-password: obfs
    bbr-profile: desktop
  - name: tuic
    type: tuic
    server: tuic.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111113
    password: secret
    congestion-controller: bbr
    udp-relay-mode: native
  - name: socks
    type: socks5
    server: socks.example.com
    port: 1080
    username: user
    password: pass
  - name: http
    type: http
    server: http.example.com
    port: 8080
    username: user
    password: pass
    headers:
      X-Test: yes
  - name: wireguard
    type: wireguard
    server: wg.example.com
    port: 51820
    ip: 10.0.0.2/32
    private-key: private
    public-key: public
    pre-shared-key: psk
    allowed-ips:
      - 0.0.0.0/0
    mtu: 1408
`)
	parser := mihomo.NewParser()
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

func TestParseMihomoSSRAndSnell(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: ssr
    type: ssr
    server: ssr.example.com
    port: 8388
    cipher: aes-128-cfb
    password: secret
    protocol: auth_sha1_v4
    protocol-param: param
    obfs: http_simple
    obfs-param: cdn.example.com
    udp: true
  - name: snell
    type: snell
    server: snell.example.com
    port: 44046
    psk: psk-secret
    version: 3
    obfs-opts:
      mode: tls
      host: cdn.example.com
`))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 2)
	require.Equal(t, domain.NodeTypeShadowsocksR, nodes[0].Type)
	require.Equal(t, "secret", nodes[0].Password)
	require.NotNil(t, nodes[0].ShadowsocksR)
	require.Equal(t, "auth_sha1_v4", nodes[0].ShadowsocksR.Protocol)
	require.Equal(t, "param", nodes[0].ShadowsocksR.ProtocolParam)
	require.Equal(t, "http_simple", nodes[0].ShadowsocksR.Obfs)
	require.Equal(t, "cdn.example.com", nodes[0].ShadowsocksR.ObfsParam)
	require.NotNil(t, nodes[0].Dialer)
	require.NotNil(t, nodes[0].Dialer.UDPRelay)
	require.True(t, *nodes[0].Dialer.UDPRelay)
	require.Equal(t, domain.NodeTypeSnell, nodes[1].Type)
	require.Equal(t, "psk-secret", nodes[1].Password)
	require.NotNil(t, nodes[1].Snell)
	require.Equal(t, 3, nodes[1].Snell.Version)
	require.Equal(t, "tls", nodes[1].Snell.Obfs)
	require.Equal(t, "cdn.example.com", nodes[1].Snell.ObfsHost)
}

func TestParseMihomoTransportVariants(t *testing.T) {
	parser := mihomo.NewParser()
	tests := []struct {
		name  string
		yaml  string
		check func(t *testing.T, n domain.NodeIR)
	}{
		{
			name: "grpc",
			yaml: `
proxies:
  - name: grpc-node
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: grpc
    grpc-opts:
      grpc-service-name: mysvc
`,
			check: func(t *testing.T, n domain.NodeIR) {
				require.NotNil(t, n.Transport)
				require.Equal(t, "grpc", n.Transport.Type)
				require.Equal(t, "mysvc", n.Transport.ServiceName)
			},
		},
		{
			name: "h2",
			yaml: `
proxies:
  - name: h2-node
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: h2
    h2-opts:
      host:
        - h2.example.com
      path: /h2
`,
			check: func(t *testing.T, n domain.NodeIR) {
				require.NotNil(t, n.Transport)
				require.Equal(t, "http", n.Transport.Type)
				require.Equal(t, "/h2", n.Transport.Path)
				require.Equal(t, "h2.example.com", n.Transport.Host)
			},
		},
		{
			name: "http-xhttp",
			yaml: `
proxies:
  - name: http-node
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: http
    http-opts:
      method: GET
      path:
        - /api
      headers:
        User-Agent:
          - curl/8
  - name: xhttp-node
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111112
    network: xhttp
    xhttp-opts:
      path: /x
      host: cdn.example.com
`,
			check: func(t *testing.T, n domain.NodeIR) {
				require.NotNil(t, n.Transport)
				if n.Name == "http-node" {
					require.Equal(t, "tcp", n.Transport.Type)
					require.Equal(t, "http", n.Transport.HeaderType)
					require.Equal(t, "GET", n.Transport.Method)
					require.Equal(t, "/api", n.Transport.Path)
					require.Equal(t, "curl/8", n.Transport.Headers["User-Agent"])
				} else {
					require.Equal(t, "xhttp", n.Transport.Type)
					require.Equal(t, "/x", n.Transport.Path)
				}
			},
		},
		{
			name: "mux-udp-over-tcp",
			yaml: `
proxies:
  - name: ss-udp
    type: ss
    server: example.com
    port: 8388
    cipher: aes-128-gcm
    password: secret
    mux: true
    udp-over-tcp: true
    udp-over-tcp-version: 2
`,
			check: func(t *testing.T, n domain.NodeIR) {
				require.NotNil(t, n.Multiplex)
				require.True(t, n.Multiplex.Enabled)
				require.NotNil(t, n.UDPOverTCP)
				require.True(t, n.UDPOverTCP.Enabled)
				require.Equal(t, 2, n.UDPOverTCP.Version)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodes, _, err := parser.Parse(context.Background(), []byte(tc.yaml))
			require.NoError(t, err)
			if tc.name == "http-xhttp" {
				require.Len(t, nodes, 2)
				require.Empty(t, nodes[0].Network)
				require.Empty(t, nodes[1].Network)
				tc.check(t, nodes[0])
				tc.check(t, nodes[1])
				return
			}
			require.Len(t, nodes, 1)
			require.Empty(t, nodes[0].Network)
			tc.check(t, nodes[0])
		})
	}
}

func TestParseMihomoDoesNotPromoteNetworkForProtocolsWithoutTransportField(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: ss-network
    type: ss
    server: example.com
    port: 8388
    cipher: aes-128-gcm
    password: secret
    network: udp
`))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Empty(t, nodes[0].Network)
	require.Nil(t, nodes[0].Transport)
	require.Contains(t, nodes[0].Raw, "mihomo.network")
	require.Len(t, source.Warnings, 1)
	require.Equal(t, "mihomo.network", source.Warnings[0].Field)
}

func TestParseMihomoUnsupportedProtocolTransportFallsBackToTCPAndPreservesRaw(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: trojan-h2
    type: trojan
    server: example.com
    port: 443
    password: secret
    network: h2
    h2-opts:
      path: /ignored
`))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Empty(t, nodes[0].Network)
	require.NotNil(t, nodes[0].Transport)
	require.Equal(t, "tcp", nodes[0].Transport.Type)
	require.Contains(t, nodes[0].Raw, "mihomo.network")
	require.Contains(t, nodes[0].Raw, "mihomo.h2-opts")
	require.Len(t, source.Warnings, 2)
	require.ElementsMatch(t, []string{"mihomo.network", "mihomo.h2-opts"}, []string{
		source.Warnings[0].Field,
		source.Warnings[1].Field,
	})
}

func TestParseMihomoTLSFingerprintSplit(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: tls-split
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    encryption: none
    tls: true
    client-fingerprint: chrome
    fingerprint: sha256:0123
`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].TLS)
	require.Equal(t, "chrome", nodes[0].TLS.ClientFingerprint)
	require.Equal(t, "sha256:0123", nodes[0].TLS.Fingerprint)
}

func TestParseMihomoWebSocketEarlyData(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: ws-early
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: ws
    ws-opts:
      path: /ws
      max-early-data: 2048
      early-data-header-name: Sec-WebSocket-Protocol
`))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].Transport)
	require.Equal(t, 2048, nodes[0].Transport.MaxEarlyData)
	require.Equal(t, "Sec-WebSocket-Protocol", nodes[0].Transport.EarlyDataHeaderName)
}

func TestParseMihomoMetadataAndVLESSCompatibilityFields(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: vless-xudp
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    encryption: none
    country: US
    delay: 123
    udp: true
    xudp: true
  - name: vless-packetaddr
    type: vless
    server: example.org
    port: 443
    uuid: 11111111-1111-1111-1111-111111111112
    encryption: none
    packet-addr: true
`))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 2)
	require.Equal(t, "xudp", nodes[0].PacketEncoding)
	require.NotContains(t, nodes[0].Raw, "mihomo.country")
	require.NotContains(t, nodes[0].Raw, "mihomo.delay")
	require.NotContains(t, nodes[0].Raw, "mihomo.xudp")
	require.Equal(t, "packetaddr", nodes[1].PacketEncoding)
	require.NotContains(t, nodes[1].Raw, "mihomo.packet-addr")
}

func TestParseMihomoWebSocketHTTPUpgradeIsKnown(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: ws-upgrade
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    encryption: none
    network: ws
    ws-opts:
      path: /upgrade
      headers:
        Host: cdn.example.com
      v2ray-http-upgrade: true
      v2ray-http-upgrade-fast-open: true
`))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].Transport)
	require.Equal(t, "websocket", nodes[0].Transport.Type)
	require.NotContains(t, nodes[0].Raw, "mihomo.ws-opts.v2ray-http-upgrade")
	require.NotContains(t, nodes[0].Raw, "mihomo.ws-opts.v2ray-http-upgrade-fast-open")
}

func TestParseMihomoNestedUnsupportedFieldsGoToRaw(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: xhttp-node
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: xhttp
    xhttp-opts:
      path: /x
      host: cdn.example.com
      mode: packet-up
      x-padding-bytes: 100-200
      x-padding-method: random
      session-key: session
  - name: grpc-node
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111112
    network: grpc
    grpc-opts:
      grpc-service-name: svc
      grpc-user-agent: curl
      ping-interval: 30
`))
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.NotContains(t, nodes[0].Raw, "mihomo.xhttp-opts.mode")
	require.Contains(t, nodes[0].Raw, "mihomo.xhttp-opts.x-padding-bytes")
	require.Contains(t, nodes[0].Raw, "mihomo.xhttp-opts.x-padding-method")
	require.Contains(t, nodes[0].Raw, "mihomo.xhttp-opts.session-key")
	require.Contains(t, nodes[1].Raw, "mihomo.grpc-opts.grpc-user-agent")
	require.Contains(t, nodes[1].Raw, "mihomo.grpc-opts.ping-interval")
	require.Len(t, source.Warnings, 5)
}

func TestParseMihomoTFOToDialer(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: hy2
    type: hysteria2
    server: example.com
    port: 8443
    password: secret
    tfo: true
  - name: hy
    type: hysteria
    server: example.com
    port: 8443
    auth-str: secret
    fast-open: true
`))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 2)
	require.NotNil(t, nodes[0].Dialer)
	require.True(t, nodes[0].Dialer.TFO)
	require.NotContains(t, nodes[0].Raw, "mihomo.tfo")
	require.NotNil(t, nodes[1].Dialer)
	require.True(t, nodes[1].Dialer.TFO)
	require.NotContains(t, nodes[1].Raw, "mihomo.fast-open")
}

func TestParseMihomoUDPRelayToDialer(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: ss-off
    type: ss
    server: example.com
    port: 8388
    cipher: aes-128-gcm
    password: secret
    udp: false
  - name: vmess-on
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    udp: true
`))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 2)
	require.NotNil(t, nodes[0].Dialer)
	require.NotNil(t, nodes[0].Dialer.UDPRelay)
	require.False(t, *nodes[0].Dialer.UDPRelay)
	require.NotContains(t, nodes[0].Raw, "mihomo.udp")
	require.NotNil(t, nodes[1].Dialer)
	require.NotNil(t, nodes[1].Dialer.UDPRelay)
	require.True(t, *nodes[1].Dialer.UDPRelay)
	require.NotContains(t, nodes[1].Raw, "mihomo.udp")
}

func TestParseMihomoHysteria2FastOpenStaysRaw(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: hy2
    type: hysteria2
    server: example.com
    port: 8443
    password: secret
    fast-open: true
`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Nil(t, nodes[0].Dialer)
	require.Contains(t, nodes[0].Raw, "mihomo.fast-open")
	require.Len(t, source.Warnings, 1)
}

func TestParseMihomoHysteria2MasqueradeIsIgnored(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: hy2
    type: hysteria2
    server: example.com
    port: 8443
    password: secret
    masquerade: https://bing.com
`))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 1)
	require.NotContains(t, nodes[0].Raw, "mihomo.masquerade")
}

func TestParseMihomoWireGuardPeersList(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: wg
    type: wireguard
    server: wg.example.com
    port: 51820
    private-key: private
    peers:
      - server: peer.example.com
        port: 51821
        public-key: public
        pre-shared-key: psk
        allowed-ips:
          - 0.0.0.0/0
        reserved: [1, 2, 3]
`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Len(t, nodes[0].WireGuard.Peers, 1)
	require.Equal(t, []uint8{1, 2, 3}, nodes[0].WireGuard.Peers[0].Reserved)
}

func TestParseMihomoGrpcMuxMaxConnections(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: grpc-mux
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: grpc
    grpc-opts:
      grpc-service-name: svc
      max-connections: 8
`))
	require.NoError(t, err)
	require.NotNil(t, nodes[0].Multiplex)
	require.Equal(t, 8, nodes[0].Multiplex.MaxConnections)
}

func TestParseMihomoHysteriaNumericSpeeds(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: hy
    type: hysteria
    server: example.com
    port: 8443
    up-speed: 20
    down-speed: 100
    auth-str: secret
`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].Hysteria)
	require.Empty(t, nodes[0].Hysteria.Up)
	require.Equal(t, 20, nodes[0].Hysteria.UpMbps)
	require.Empty(t, nodes[0].Hysteria.Down)
	require.Equal(t, 100, nodes[0].Hysteria.DownMbps)
}

func TestParseMihomoNormalizesHysteriaBandwidth(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`
proxies:
  - {name: bare, type: hysteria, server: bare.example, port: 443, up: "11", down: "55", auth-str: secret}
  - {name: units, type: hysteria, server: units.example, port: 443, up: "640 KBps", down: "1 Gbps", auth-str: secret}
  - {name: compat, type: hysteria, server: compat.example, port: 443, up: "11 Mbps", down: "55 Mbps", up-speed: 20, down-speed: 100, auth-str: secret}
`))
	require.NoError(t, err)
	require.Len(t, nodes, 3)

	require.Empty(t, nodes[0].Hysteria.Up)
	require.Equal(t, 11, nodes[0].Hysteria.UpMbps)
	require.Empty(t, nodes[0].Hysteria.Down)
	require.Equal(t, 55, nodes[0].Hysteria.DownMbps)

	require.Equal(t, "640 KBps", nodes[1].Hysteria.Up)
	require.Zero(t, nodes[1].Hysteria.UpMbps)
	require.Empty(t, nodes[1].Hysteria.Down)
	require.Equal(t, 1000, nodes[1].Hysteria.DownMbps)

	require.Empty(t, nodes[2].Hysteria.Up)
	require.Equal(t, 20, nodes[2].Hysteria.UpMbps)
	require.Empty(t, nodes[2].Hysteria.Down)
	require.Equal(t, 100, nodes[2].Hysteria.DownMbps)
}

func TestParseMihomoPreservesInvalidHysteriaBandwidthAsRaw(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - {name: invalid, type: hysteria, server: invalid.example, port: 443, up: fast, auth-str: secret}
  - {name: negative-compat, type: hysteria, server: negative.example, port: 443, up: "11", up-speed: -1, auth-str: secret}
  - {name: zero-compat, type: hysteria, server: zero.example, port: 443, up: "12", down: "34", up-speed: 0, down-speed: 0, auth-str: secret}
`))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 3)

	require.Empty(t, nodes[0].Hysteria.Up)
	require.Zero(t, nodes[0].Hysteria.UpMbps)
	require.JSONEq(t, `"fast"`, string(nodes[0].Raw["mihomo.up"]))

	require.Empty(t, nodes[1].Hysteria.Up)
	require.Equal(t, 11, nodes[1].Hysteria.UpMbps)
	require.JSONEq(t, `-1`, string(nodes[1].Raw["mihomo.up-speed"]))

	require.Empty(t, nodes[2].Hysteria.Up)
	require.Equal(t, 12, nodes[2].Hysteria.UpMbps)
	require.Empty(t, nodes[2].Hysteria.Down)
	require.Equal(t, 34, nodes[2].Hysteria.DownMbps)
	require.NotContains(t, nodes[2].Raw, "mihomo.up-speed")
	require.NotContains(t, nodes[2].Raw, "mihomo.down-speed")

	require.Condition(t, func() bool {
		for _, warning := range source.Warnings {
			if warning.Code == "parse_unknown_field" {
				return true
			}
		}
		return false
	})
}

func TestParseMihomoChecksHysteriaCompatibilityMbpsBound(t *testing.T) {
	max := shared.MaxHysteriaMbps()
	if max == int(^uint(0)>>1) {
		t.Skip("max+1 is not representable as int on this platform")
	}
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(fmt.Sprintf(`
proxies:
  - {name: max, type: hysteria, server: max.example, port: 443, up-speed: %d, down-speed: %d}
  - {name: over, type: hysteria, server: over.example, port: 443, up-speed: %d, down-speed: %d}
`, max, max, max+1, max)))

	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, &domain.HysteriaOptions{UpMbps: max, DownMbps: max}, nodes[0].Hysteria)
	require.Zero(t, nodes[1].Hysteria.UpMbps)
	require.Equal(t, max, nodes[1].Hysteria.DownMbps)
	require.JSONEq(t, fmt.Sprint(max+1), string(nodes[1].Raw["mihomo.up-speed"]))
	require.Contains(t, warningFieldNames(source.Warnings), "mihomo.up-speed")
}

func TestParseMihomoValidatesShadowedNativeHysteriaBandwidth(t *testing.T) {
	max := shared.MaxHysteriaMbps()
	if max == int(^uint(0)>>1) {
		t.Skip("max+1 is not representable as int on this platform")
	}
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(fmt.Sprintf(`
proxies:
  - {name: invalid-native, type: hysteria, server: invalid-native.example, port: 443, up: fast, up-speed: 20, down-speed: 100}
  - {name: fractional-compat, type: hysteria, server: fractional.example, port: 443, up: "11", up-speed: 1.5, down-speed: 100}
  - {name: zero-compat, type: hysteria, server: zero.example, port: 443, up: "12", up-speed: 0, down-speed: 100}
  - {name: over-compat, type: hysteria, server: over.example, port: 443, up: "13", up-speed: %d, down-speed: 100}
`, max+1)))

	require.NoError(t, err)
	require.Len(t, nodes, 4)
	require.Equal(t, 20, nodes[0].Hysteria.UpMbps)
	require.JSONEq(t, `"fast"`, string(nodes[0].Raw["mihomo.up"]))
	require.Equal(t, 11, nodes[1].Hysteria.UpMbps)
	require.JSONEq(t, `1.5`, string(nodes[1].Raw["mihomo.up-speed"]))
	require.Equal(t, 12, nodes[2].Hysteria.UpMbps)
	require.NotContains(t, nodes[2].Raw, "mihomo.up-speed")
	require.Equal(t, 13, nodes[3].Hysteria.UpMbps)
	require.JSONEq(t, fmt.Sprint(max+1), string(nodes[3].Raw["mihomo.up-speed"]))
	require.ElementsMatch(t, []string{"mihomo.up", "mihomo.up-speed", "mihomo.up-speed"}, warningFieldNames(source.Warnings))
}

func warningFieldNames(warnings []domain.Warning) []string {
	fields := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		fields = append(fields, warning.Field)
	}
	return fields
}

func TestParseMihomoRejectsInvalidYAML(t *testing.T) {
	parser := mihomo.NewParser()
	_, _, err := parser.Parse(context.Background(), []byte("proxies: [\n"))
	require.Error(t, err)
}

func TestParseMihomoUnknownFieldsGoToRaw(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: ss
    type: ss
    server: example.com
    port: 8388
    cipher: aes-128-gcm
    password: secret
    country: DE
    delay: 123
    private-thing: value
    another-private-thing: 42
`))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	require.Contains(t, nodes[0].Raw, "mihomo.private-thing")
	require.Contains(t, nodes[0].Raw, "mihomo.another-private-thing")
	require.NotContains(t, nodes[0].Raw, "mihomo.country")
	require.NotContains(t, nodes[0].Raw, "mihomo.delay")
	require.NotContains(t, nodes[0].Raw, "mihomo.password")
	require.NotContains(t, nodes[0].Raw, "mihomo.name")
	require.Len(t, source.Warnings, 2)
	for _, warning := range source.Warnings {
		require.NotNil(t, warning.NodeIndex)
		require.Equal(t, 0, *warning.NodeIndex)
		require.NotNil(t, warning.NodeContext)
		require.Equal(t, "mihomo", warning.NodeContext.Format)
		require.Equal(t, "ss", warning.NodeContext.Name)
		require.Equal(t, domain.NodeTypeShadowsocks, warning.NodeContext.Type)
		require.Equal(t, "example.com", warning.NodeContext.Server)
		require.Equal(t, uint16(8388), warning.NodeContext.Port)
		require.Equal(t, "secret", warning.NodeContext.Raw["password"])
		require.Equal(t, "value", warning.NodeContext.Raw["private-thing"])
		require.Equal(t, 42, warning.NodeContext.Raw["another-private-thing"])
	}
}

func TestParseMihomoHysteria2UDPCompatibilityBoundary(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: udp-true
    type: hysteria2
    server: true.example.com
    port: 443
    password: secret
    udp: true
  - name: udp-false
    type: hysteria2
    server: false.example.com
    port: 443
    password: secret
    udp: false
  - name: udp-invalid
    type: hysteria2
    server: invalid.example.com
    port: 443
    password: secret
    udp: enabled
`))

	require.NoError(t, err)
	require.Len(t, nodes, 3)
	require.NotContains(t, nodes[0].Raw, "mihomo.udp")
	require.Contains(t, nodes[1].Raw, "mihomo.udp")
	require.Contains(t, nodes[2].Raw, "mihomo.udp")
	require.Len(t, source.Warnings, 2)
	require.Equal(t, "mihomo.udp", source.Warnings[0].Field)
	require.Equal(t, "udp-false", source.Warnings[0].Node)
	require.Equal(t, "mihomo.udp", source.Warnings[1].Field)
	require.Equal(t, "udp-invalid", source.Warnings[1].Node)
}

func TestParseMihomoGRPCModeCompatibilityBoundary(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: grpc-gun
    type: vless
    server: gun.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: grpc
    grpc-opts:
      grpc-service-name: service
      grpc-mode: gun
  - name: grpc-multi
    type: vless
    server: multi.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111112
    network: grpc
    grpc-opts:
      grpc-service-name: service
      grpc-mode: multi
    dialer-proxy: upstream
`))

	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.NotContains(t, nodes[0].Raw, "mihomo.grpc-opts.grpc-mode")
	require.Contains(t, nodes[1].Raw, "mihomo.grpc-opts.grpc-mode")
	require.Contains(t, nodes[1].Raw, "mihomo.dialer-proxy")
	require.Len(t, source.Warnings, 2)
	require.Equal(t, "mihomo.dialer-proxy", source.Warnings[0].Field)
	require.Equal(t, "mihomo.grpc-opts.grpc-mode", source.Warnings[1].Field)
}

func TestParseMihomoSingleProxyDocumentAndFallbackName(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
type: ss
server: example.com
port: 8388
cipher: aes-128-gcm
password: secret
plugin: obfs-local
plugin-opts:
  mode: tls
user: user-a
`))
	require.NoError(t, err)
	require.Equal(t, "mihomo", parser.Name())
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	require.Equal(t, "example.com", nodes[0].Name)
	require.Equal(t, "user-a", nodes[0].Username)
	require.Equal(t, "obfs-local", nodes[0].Plugin)
	require.Equal(t, map[string]any{"mode": "tls"}, nodes[0].PluginOptions)
}

func TestParseMihomoTLSFullOptions(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: tls-full
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    encryption: none
    tls: true
    sni: sni.example.com
    skip-cert-verify: true
    alpn:
      - h2
      - http/1.1
    client-fingerprint: chrome
    fingerprint: sha256:abcd
    ech-opts:
      enable: true
      config: ech-config
      query-server-name: ech.example.com
    reality-opts:
      public-key: public
      short-id: abcd
`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	tls := nodes[0].TLS
	require.NotNil(t, tls)
	require.True(t, tls.Enabled)
	require.Equal(t, "sni.example.com", tls.ServerName)
	require.True(t, tls.InsecureSkipVerify)
	require.Equal(t, []string{"h2", "http/1.1"}, tls.ALPN)
	require.Equal(t, "chrome", tls.ClientFingerprint)
	require.Equal(t, "sha256:abcd", tls.Fingerprint)
	require.NotNil(t, tls.ECH)
	require.True(t, tls.ECH.Enabled)
	require.Equal(t, []string{"ech-config"}, tls.ECH.Config)
	require.Equal(t, "ech.example.com", tls.ECH.QueryServerName)
	require.NotNil(t, tls.Reality)
	require.Equal(t, "public", tls.Reality.PublicKey)
	require.Equal(t, "abcd", tls.Reality.ShortID)
}

func TestParseMihomoHysteria2RealmAndPorts(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: hy2
    type: hy2
    server: example.com
    port: 8443
    password: secret
    auth: legacy-auth
    server-ports:
      - "8443"
      - "9443"
    hop-interval: 30s
    up: 20 Mbps
    down: 100 Mbps
    obfs: salamander
    obfs-password: obfs-pass
    bbr-profile: desktop
    udp-mtu: 1200
    cwnd: 32
    realm-opts:
      enable: true
      server-url: https://realm.example.com
      token: token
      realm-id: realm-id
      stun-servers:
        - stun.example.com
`))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeHysteria2, nodes[0].Type)
	require.NotContains(t, nodes[0].Raw, "mihomo.auth")
	hy := nodes[0].Hysteria
	require.NotNil(t, hy)
	require.Empty(t, hy.Auth)
	require.Equal(t, []string{"8443", "9443"}, hy.ServerPorts)
	require.Equal(t, "30s", hy.HopInterval)
	require.Equal(t, "20 Mbps", hy.Up)
	require.Equal(t, "100 Mbps", hy.Down)
	require.Equal(t, "salamander", hy.Obfs)
	require.Equal(t, "obfs-pass", hy.ObfsPassword)
	require.Equal(t, "desktop", hy.BBRProfile)
	require.Equal(t, 1200, hy.UDPMTU)
	require.Equal(t, 32, hy.CWND)
	require.NotNil(t, hy.Realm)
	require.True(t, hy.Realm.Enabled)
	require.Equal(t, "realm-id", hy.Realm.RealmID)
	require.Equal(t, []string{"stun.example.com"}, hy.Realm.STUNServers)
}

func TestParseMihomoTUICAndUDPOverStreamAlias(t *testing.T) {
	parser := mihomo.NewParser()
	nodes, _, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: tuic
    type: tuic
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    password: secret
    token: legacy-token
    congestion-controller: bbr
    udp-relay-mode: native
    reduce-rtt: true
    heartbeat-interval: 10
    udp-over-stream: true
    udp-over-stream-version: 2
`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "legacy-token", nodes[0].Token)
	require.NotNil(t, nodes[0].TUIC)
	require.Equal(t, "bbr", nodes[0].TUIC.CongestionControl)
	require.Equal(t, "native", nodes[0].TUIC.UDPRelayMode)
	require.True(t, nodes[0].TUIC.ReduceRTT)
	require.Equal(t, "10", nodes[0].TUIC.Heartbeat)
	require.True(t, nodes[0].TUIC.UDPOverStream)
	require.Equal(t, 2, nodes[0].TUIC.UDPOverStreamVersion)
	require.NotNil(t, nodes[0].UDPOverTCP)
	require.True(t, nodes[0].UDPOverTCP.Enabled)
	require.Equal(t, 2, nodes[0].UDPOverTCP.Version)
}

func TestParseMihomoSkipsBadProxiesWithWarnings(t *testing.T) {
	parser := mihomo.NewParser()

	nodes, source, err := parser.Parse(context.Background(), []byte(`
proxies:
  - name: good
    type: ss
    server: ss.example.com
    port: 8388
    cipher: aes-128-gcm
    password: secret
  - bad
  - name: direct
    type: direct
    server: direct.example.com
    port: 1
`))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	require.Equal(t, "good", nodes[0].Name)
	require.Len(t, source.Warnings, 2)
	first := source.Warnings[0]
	require.Equal(t, "parse_proxy_skipped", first.Code)
	require.Equal(t, "mihomo", first.Source)
	require.NotNil(t, first.NodeIndex)
	require.Equal(t, 1, *first.NodeIndex)
	require.NotNil(t, first.NodeContext)
	require.Equal(t, "mihomo", first.NodeContext.Format)
	require.NotNil(t, first.NodeContext.Raw)

	second := source.Warnings[1]
	require.Equal(t, "parse_proxy_skipped", second.Code)
	require.Equal(t, "mihomo", second.Source)
	require.NotNil(t, second.NodeIndex)
	require.Equal(t, 2, *second.NodeIndex)
	require.NotNil(t, second.NodeContext)
	require.Equal(t, "direct", second.NodeContext.Name)
	require.Equal(t, "direct.example.com", second.NodeContext.Server)
	require.Equal(t, uint16(1), second.NodeContext.Port)
	require.Equal(t, "direct", second.NodeContext.Raw["type"])
}

func TestParseMihomoRejectsInvalidInputs(t *testing.T) {
	parser := mihomo.NewParser()

	_, _, err := parser.Parse(context.Background(), []byte(`proxies: bad`))
	require.Error(t, err)
	require.ErrorContains(t, err, "mihomo proxies must be a list")

	_, _, err = parser.Parse(context.Background(), []byte(`
proxies:
  - bad
`))
	require.Error(t, err)
	require.ErrorContains(t, err, "no mihomo proxies found")

	_, _, err = parser.Parse(context.Background(), []byte(`
proxies:
  - name: direct
    type: direct
`))
	require.Error(t, err)
	require.ErrorContains(t, err, "no mihomo proxies found")

	_, _, err = parser.Parse(context.Background(), []byte(`{type: direct, name: direct}`))
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported mihomo proxy type")
}
