package shadowrocket_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/jsonnodes"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shadowrocket"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	uriadapter "github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const shadowrocketRevision = "5f1916b5897fc59fb7172aca59ae52050a3532fe"

func TestRendererNameAndRenderWrapper(t *testing.T) {
	t.Parallel()

	renderer := shadowrocket.NewRenderer()
	require.Equal(t, "shadowrocket-proxies", renderer.Name())

	out, err := renderer.Render(context.Background(), supportedNodes()[:1], domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, "[Proxy]\nss = ss, ss.example.com, 8388, password=ss-secret, method=aes-256-gcm, obfs=websocket, plugin=none\n", string(out))
}

func TestRenderAllSupportedProtocolsInCanonicalOrder(t *testing.T) {
	t.Parallel()

	nodes := supportedNodes()
	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{
		Format: "shadowrocket-proxies",
	})

	require.NoError(t, err)
	require.Equal(t, len(nodes), report.SuccessCount)
	require.Empty(t, report.Warnings)
	require.Equal(t, strings.Join([]string{
		"[Proxy]",
		"ss = ss, ss.example.com, 8388, password=ss-secret, method=aes-256-gcm, obfs=websocket, plugin=none",
		"vmess = vmess, vmess.example.com, 443, password=11111111-1111-1111-1111-111111111111, alterId=0, method=auto, obfs=websocket, tfo=1",
		"vless = vless, vless.example.com, 443, password=22222222-2222-2222-2222-222222222222, tls=true, obfs=websocket, peer=vless-sni.example.com",
		"trojan = trojan, trojan.example.com, 443, password=trojan-secret, allowInsecure=1, peer=trojan-sni.example.com",
		"hysteria = hysteria, hy.example.com, 8443, auth=hy-secret, obfsParam=hy-obfs, protocol=udp, udp=1, peer=hy-sni.example.com, alpn=h2, upmbps=100, downmbps=200",
		"hysteria2 = hysteria2, hy2.example.com, 8443, auth=hy2-secret, obfsParam=hy2-obfs, udp=1, peer=hy2-sni.example.com, alpn=h3",
		"tuic = tuic, tuic.example.com, 443, password=tuic-secret, udp=1, user=33333333-3333-3333-3333-333333333333, peer=tuic-sni.example.com, alpn=h2",
		"http = http, http.example.com, 8080, http-user, http-pass",
		"https = https, https.example.com, 8443, https-user, https-pass",
		"socks = socks5, socks.example.com, 1080, socks-user, socks-pass",
		"socks-tls = socks5-tls, socks-tls.example.com, 1081, socks-tls-user, socks-tls-pass, skip-common-name-verify=true",
		"wireguard = wireguard, wg.example.com, 51820, privateKey=private-key, publicKey=public-key, ip=10.0.0.2/32, udp=1, mtu=1350, keepalive=40, reserved=1/2/3",
		"snell = snell, snell.example.com, 44046, password=snell-secret, udp=1, obfs=http, obfs-host=obfs.example.com",
		"",
	}, "\n"), string(out))
}

func TestRenderHysteriaFromOfficialURIKeepsProtocolAndObfsPasswordDistinct(t *testing.T) {
	t.Parallel()

	nodes, _, err := uriadapter.NewParser().Parse(context.Background(), []byte(
		"hysteria://hy.example.com:8443?protocol=wechat-video&auth=hy-secret&peer=hy-sni.example.com&upmbps=100&downmbps=200&obfs=xplus&obfsParam=hy-obfs#hysteria",
	))
	require.NoError(t, err)
	require.Equal(t, "wechat-video", nodes[0].Hysteria.Protocol)
	require.Equal(t, "xplus", nodes[0].Hysteria.Obfs)
	require.Equal(t, "hy-obfs", nodes[0].Hysteria.ObfsPassword)

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Equal(t, "[Proxy]\nhysteria = hysteria, hy.example.com, 8443, auth=hy-secret, obfsParam=hy-obfs, protocol=wechat-video, peer=hy-sni.example.com, upmbps=100, downmbps=200\n", string(out))
}

func TestRenderHysteriaFromOfficialURIDefaultsProtocolToUDP(t *testing.T) {
	t.Parallel()

	nodes, _, err := uriadapter.NewParser().Parse(context.Background(), []byte(
		"hysteria://hy.example.com:8443?auth=hy-secret&upmbps=100&downmbps=200#hysteria",
	))
	require.NoError(t, err)

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Equal(t, "[Proxy]\nhysteria = hysteria, hy.example.com, 8443, auth=hy-secret, protocol=udp, upmbps=100, downmbps=200\n", string(out))
}

func TestRenderHysteriaFromOfficialURIAllowsEmptyAuth(t *testing.T) {
	t.Parallel()

	nodes, _, err := uriadapter.NewParser().Parse(context.Background(), []byte(
		"hysteria://hy.example.com:8443?upmbps=100&downmbps=200#hysteria",
	))
	require.NoError(t, err)

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Equal(t, "[Proxy]\nhysteria = hysteria, hy.example.com, 8443, auth=, protocol=udp, upmbps=100, downmbps=200\n", string(out))
}

func TestRenderHysteriaFromMihomoKeepsProtocolAndObfsPassword(t *testing.T) {
	t.Parallel()

	nodes, _, err := mihomo.NewParser().Parse(context.Background(), []byte(`
proxies:
  - name: hysteria
    type: hysteria
    server: hy.example.com
    port: 8443
    protocol: faketcp
    auth-str: hy-secret
    obfs: hy-obfs
    up-speed: 100
    down-speed: 200
    sni: hy-sni.example.com
`))
	require.NoError(t, err)
	require.Empty(t, nodes[0].Hysteria.Obfs)
	require.Equal(t, "hy-obfs", nodes[0].Hysteria.ObfsPassword)
	require.Empty(t, nodes[0].Hysteria.Up)
	require.Equal(t, 100, nodes[0].Hysteria.UpMbps)
	require.Empty(t, nodes[0].Hysteria.Down)
	require.Equal(t, 200, nodes[0].Hysteria.DownMbps)

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Equal(t, "[Proxy]\nhysteria = hysteria, hy.example.com, 8443, auth=hy-secret, obfsParam=hy-obfs, protocol=faketcp, peer=hy-sni.example.com, upmbps=100, downmbps=200\n", string(out))
}

func TestRenderHysteriaFromSingBoxDefaultsProtocolAndMapsObfsPassword(t *testing.T) {
	t.Parallel()

	nodes, _, err := singbox.NewParser().Parse(context.Background(), []byte(`{
  "outbounds": [{
    "type": "hysteria",
    "tag": "hysteria",
    "server": "hy.example.com",
    "server_port": 8443,
    "auth_str": "hy-secret",
    "obfs": "hy-obfs",
    "tls": {"enabled": true, "server_name": "hy-sni.example.com"}
  }]
	}`))
	require.NoError(t, err)
	require.Empty(t, nodes[0].Hysteria.Obfs)
	require.Equal(t, "hy-obfs", nodes[0].Hysteria.ObfsPassword)

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Equal(t, "[Proxy]\nhysteria = hysteria, hy.example.com, 8443, auth=hy-secret, obfsParam=hy-obfs, protocol=udp, peer=hy-sni.example.com\n", string(out))
}

func TestRenderHysteriaMigratesLegacyJSONObfsPassword(t *testing.T) {
	t.Parallel()

	nodes, _, err := jsonnodes.NewParser().Parse(context.Background(), []byte(`[{"name":"hysteria","type":"hysteria","server":"hy.example.com","port":8443,"tls":{"enabled":true},"hysteria":{"auth_str":"hy-secret","obfs":"legacy-password"}}]`))
	require.NoError(t, err)

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Equal(t, "[Proxy]\nhysteria = hysteria, hy.example.com, 8443, auth=hy-secret, obfsParam=legacy-password, protocol=udp\n", string(out))
}

func TestRenderHysteriaSkipsUndocumentedProtocolAndObfsModes(t *testing.T) {
	t.Parallel()

	nodes, _, err := uriadapter.NewParser().ParseList(context.Background(), []byte(strings.Join([]string{
		"hysteria://one.example.com:8443?protocol=quic&auth=secret#bad-protocol",
		"hysteria://two.example.com:8443?auth=secret&obfs=salamander&obfsParam=secret#bad-obfs",
		"hysteria://three.example.com:8443?auth=secret&obfs=xplus#missing-obfs-password",
	}, "\n")))
	require.NoError(t, err)
	nodes = append(nodes, domain.NodeIR{Name: "good", Type: domain.NodeTypeHTTP, Server: "good.example.com", Port: 8080})

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, "[Proxy]\ngood = http, good.example.com, 8080\n", string(out))
	require.Equal(t, 1, report.SuccessCount)
	require.Len(t, report.Warnings, 3)
	for _, warning := range report.Warnings {
		require.Equal(t, "render_node_skipped", warning.Code)
	}
}

func TestRenderEmptyInputAndAllSkipped(t *testing.T) {
	t.Parallel()

	renderer := shadowrocket.NewRenderer()
	out, report, err := renderer.RenderWithReport(context.Background(), nil, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, "[Proxy]\n", string(out))
	require.Equal(t, domain.RenderReport{}, report)

	out, report, err = renderer.RenderWithReport(context.Background(), []domain.NodeIR{
		{Name: "ssr", Type: domain.NodeTypeShadowsocksR, Server: "ssr.example.com", Port: 8388},
		{Name: "mieru", Type: domain.NodeTypeMieru, Server: "mieru.example.com", Port: 443},
		{Name: "anytls", Type: domain.NodeTypeAnyTLS, Server: "anytls.example.com", Port: 443},
	}, domain.RenderOptions{})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeRenderFailed))
	require.Nil(t, out)
	require.Zero(t, report.SuccessCount)
	require.Len(t, report.Warnings, 3)
	for _, warning := range report.Warnings {
		require.Equal(t, "render_node_skipped", warning.Code)
		require.Equal(t, "shadowrocket-proxies", warning.Target)
		require.NotNil(t, warning.NodeContext)
		require.Equal(t, "shadowrocket-proxies", warning.NodeContext.Format)
	}
}

func TestRenderNormalizesAndDeduplicatesNamesInRenderedOrder(t *testing.T) {
	t.Parallel()

	nodes := []domain.NodeIR{
		{
			Name: "same", Type: domain.NodeTypeShadowsocks,
			Server: "bad,server", Port: 8388, Cipher: "aes-128-gcm", Password: "secret",
		},
		{
			Name: "  dup,\r\n=  ", Type: domain.NodeTypeHTTP,
			Server: "one.example.com", Port: 8080,
			Headers: map[string]string{"X-Test": "one"},
			Raw: map[string]json.RawMessage{
				"vendor.z": json.RawMessage(`1`),
				"vendor.a": json.RawMessage(`true`),
			},
		},
		{Name: "dup,\n=", Type: domain.NodeTypeHTTP, Server: "two.example.com", Port: 8080},
		{Name: "", Type: domain.NodeTypeHTTP, Server: "three.example.com", Port: 8080},
		{Name: "same", Type: domain.NodeTypeHTTP, Server: "four.example.com", Port: 8080},
		{Name: "same", Type: domain.NodeTypeHTTP, Server: "five.example.com", Port: 8080},
	}

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 5, report.SuccessCount)
	require.Equal(t, strings.Join([]string{
		"[Proxy]",
		"dup， ＝ = http, one.example.com, 8080",
		"dup， ＝ (2) = http, two.example.com, 8080",
		"node-4 = http, three.example.com, 8080",
		"same = http, four.example.com, 8080",
		"same (2) = http, five.example.com, 8080",
		"",
	}, "\n"), string(out))

	require.Equal(t, len(report.Warnings), report.LostFields)
	require.Equal(t, 8, report.LostFields)
	require.Equal(t, []string{
		"ss", "headers", "name", "vendor.a", "vendor.z", "name", "name", "name",
	}, warningFields(report.Warnings))
	for _, warning := range report.Warnings {
		require.Equal(t, "shadowrocket-proxies", warning.Target)
		if warning.Code == "render_lossy_field" && warning.Field == "name" {
			require.Contains(t, warning.Message, "normalized")
		}
	}
}

func TestPreviewNodeNamesUseTheSameSkipAndDeduplicationOrderAsRendering(t *testing.T) {
	t.Parallel()

	nodes := []domain.NodeIR{
		{Name: "dup", Type: domain.NodeTypeShadowsocks, Server: "bad,server", Port: 8388, Cipher: "aes-128-gcm", Password: "secret"},
		{Name: "dup", Type: domain.NodeTypeHTTP, Server: "one.example.com", Port: 8080},
		{Name: "dup", Type: domain.NodeTypeHTTP, Server: "two.example.com", Port: 8080},
		{Name: "", Type: domain.NodeTypeHTTP, Server: "three.example.com", Port: 8080},
	}

	realized := shadowrocket.PreviewNodeNames(nodes)

	require.Equal(t, []string{"", "dup", "dup (2)", "node-4"}, realized)
}

func TestRenderNormalizesLeadingINIControlCharacters(t *testing.T) {
	t.Parallel()

	nodes := []domain.NodeIR{
		{Name: "#comment", Type: domain.NodeTypeHTTP, Server: "one.example.com", Port: 8080},
		{Name: ";comment", Type: domain.NodeTypeHTTP, Server: "two.example.com", Port: 8080},
		{Name: "[section]", Type: domain.NodeTypeHTTP, Server: "three.example.com", Port: 8080},
	}

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, strings.Join([]string{
		"[Proxy]",
		"＃comment = http, one.example.com, 8080",
		"；comment = http, two.example.com, 8080",
		"［section] = http, three.example.com, 8080",
		"",
	}, "\n"), string(out))
	require.Equal(t, 3, report.SuccessCount)
	require.Equal(t, []string{"name", "name", "name"}, warningFields(report.Warnings))
}

func TestRenderNormalizesNamesThatConflictWithBuiltInPolicies(t *testing.T) {
	t.Parallel()

	nodes := []domain.NodeIR{
		{Name: "DIRECT", Type: domain.NodeTypeHTTP, Server: "one.example.com", Port: 8080},
		{Name: "direct", Type: domain.NodeTypeHTTP, Server: "two.example.com", Port: 8080},
		{Name: "ReJeCt", Type: domain.NodeTypeHTTP, Server: "three.example.com", Port: 8080},
		{Name: "PROXY", Type: domain.NodeTypeHTTP, Server: "four.example.com", Port: 8080},
		{Name: "Proxy", Type: domain.NodeTypeHTTP, Server: "five.example.com", Port: 8080},
		{Name: "TAILSCALE", Type: domain.NodeTypeHTTP, Server: "six.example.com", Port: 8080},
		{Name: "REJECT-DROP", Type: domain.NodeTypeHTTP, Server: "seven.example.com", Port: 8080},
	}

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, strings.Join([]string{
		"[Proxy]",
		"DIRECT (Node) = http, one.example.com, 8080",
		"direct (Node) = http, two.example.com, 8080",
		"ReJeCt (Node) = http, three.example.com, 8080",
		"PROXY (Node) = http, four.example.com, 8080",
		"Proxy = http, five.example.com, 8080",
		"TAILSCALE (Node) = http, six.example.com, 8080",
		"REJECT-DROP (Node) = http, seven.example.com, 8080",
		"",
	}, "\n"), string(out))
	require.Equal(t, []string{
		"DIRECT (Node)", "direct (Node)", "ReJeCt (Node)", "PROXY (Node)",
		"Proxy", "TAILSCALE (Node)", "REJECT-DROP (Node)",
	}, shadowrocket.PreviewNodeNames(nodes))
	require.Equal(t, []string{"name", "name", "name", "name", "name", "name"}, warningFields(report.Warnings))
}

func TestRenderRejectsRequiredScalarDelimitersWithoutLeakingValues(t *testing.T) {
	t.Parallel()

	nodes := []domain.NodeIR{
		{
			Name: "bad-server", Type: domain.NodeTypeShadowsocks,
			Server: "bad,server.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "secret",
		},
		{
			Name: "bad-password", Type: domain.NodeTypeTrojan,
			Server: "trojan.example.com", Port: 443, Password: "must-not\nleak",
		},
		{
			Name: "bad-utf8", Type: domain.NodeTypeHTTP,
			Server: string([]byte{'b', 'a', 'd', 0xff}), Port: 8080,
		},
		{Name: "good", Type: domain.NodeTypeHTTP, Server: "good.example.com", Port: 8080},
	}

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, "[Proxy]\ngood = http, good.example.com, 8080\n", string(out))
	require.Equal(t, 1, report.SuccessCount)
	require.Len(t, report.Warnings, 3)
	for _, warning := range report.Warnings {
		require.Equal(t, "render_node_skipped", warning.Code)
		require.NotContains(t, warning.Message, "must-not")
		require.NotContains(t, warning.Message, "bad,server")
	}
}

func TestRenderSkipsUndocumentedConnectionCriticalFeatures(t *testing.T) {
	t.Parallel()

	nodes := []domain.NodeIR{
		{
			Name: "custom-plugin", Type: domain.NodeTypeShadowsocks,
			Server: "ss.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "secret", Plugin: "custom-plugin",
		},
		{
			Name: "grpc", Type: domain.NodeTypeVLESS,
			Server: "vless.example.com", Port: 443, UUID: "44444444-4444-4444-4444-444444444444",
			Transport: &domain.TransportOptions{Type: "grpc", ServiceName: "critical"},
		},
		{
			Name: "reality", Type: domain.NodeTypeVLESS,
			Server: "vless.example.com", Port: 443, UUID: "55555555-5555-5555-5555-555555555555",
			TLS: &domain.TLSOptions{Enabled: true, Reality: &domain.RealityOptions{
				Enabled: true, PublicKey: "public", ShortID: "08",
			}},
		},
		{Name: "good", Type: domain.NodeTypeHTTP, Server: "good.example.com", Port: 8080},
	}

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, "[Proxy]\ngood = http, good.example.com, 8080\n", string(out))
	require.Equal(t, 1, report.SuccessCount)
	require.Len(t, report.Warnings, 3)
	for _, warning := range report.Warnings {
		require.Equal(t, "render_node_skipped", warning.Code)
	}
}

func TestRenderSkipsTLSFeaturesNotDocumentedForVMessOrVLESS(t *testing.T) {
	t.Parallel()

	nodes := []domain.NodeIR{
		{
			Name: "vmess-tls", Type: domain.NodeTypeVMess,
			Server: "vmess.example.com", Port: 443, UUID: "11111111-1111-1111-1111-111111111111",
			TLS: &domain.TLSOptions{Enabled: true, ServerName: "vmess-sni.example.com"},
		},
		{
			Name: "vless-insecure", Type: domain.NodeTypeVLESS,
			Server: "vless.example.com", Port: 443, UUID: "22222222-2222-2222-2222-222222222222",
			TLS: &domain.TLSOptions{Enabled: true, ServerName: "vless-sni.example.com", InsecureSkipVerify: true},
		},
		{
			Name: "vless-documented", Type: domain.NodeTypeVLESS,
			Server: "documented.example.com", Port: 443, UUID: "33333333-3333-3333-3333-333333333333",
			TLS: &domain.TLSOptions{Enabled: true, ServerName: "documented-sni.example.com"},
		},
	}

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, "[Proxy]\nvless-documented = vless, documented.example.com, 443, password=33333333-3333-3333-3333-333333333333, tls=true, peer=documented-sni.example.com\n", string(out))
	require.Equal(t, 1, report.SuccessCount)
	require.Len(t, report.Warnings, 2)
	require.Equal(t, []string{"vmess-tls", "vless-insecure"}, []string{report.Warnings[0].Node, report.Warnings[1].Node})
	for _, warning := range report.Warnings {
		require.Equal(t, "render_node_skipped", warning.Code)
	}
}

func TestCapabilitiesDoNotClaimUndocumentedTLSFields(t *testing.T) {
	t.Parallel()

	capability := shadowrocket.NewRenderer().RenderCapabilities()[0]
	supported := map[string]bool{}
	for _, field := range capability.Fields {
		supported[field.Protocol+"\x00"+field.IRField] = true
	}
	lossy := map[string]bool{}
	for _, field := range capability.Lossy {
		lossy[field.Protocol+"\x00"+field.IRField] = true
	}
	for _, field := range []string{"tls.enabled", "tls.server_name", "tls.insecure_skip_verify", "tls.alpn"} {
		require.False(t, supported[string(domain.NodeTypeVMess)+"\x00"+field], field)
		require.True(t, lossy[string(domain.NodeTypeVMess)+"\x00"+field], field)
	}
	for _, field := range []string{"tls.insecure_skip_verify", "tls.alpn"} {
		require.False(t, supported[string(domain.NodeTypeVLESS)+"\x00"+field], field)
		require.True(t, lossy[string(domain.NodeTypeVLESS)+"\x00"+field], field)
	}
}

func TestRenderWireGuardRequiresExactlyOneEffectivePeer(t *testing.T) {
	t.Parallel()

	base := domain.NodeIR{
		Type: domain.NodeTypeWireGuard,
		WireGuard: &domain.WireGuardOptions{
			PrivateKey: "private", Address: []string{"10.0.0.2/32"},
		},
	}
	zero := base
	zero.Name = "zero"
	one := base
	one.Name = "one"
	one.WireGuard = &domain.WireGuardOptions{
		PrivateKey: "private", Address: []string{"10.0.0.2/32"},
		Peers: []domain.WireGuardPeer{{Server: "wg.example.com", Port: 51820, PublicKey: "public"}},
	}
	many := base
	many.Name = "many"
	many.WireGuard = &domain.WireGuardOptions{
		PrivateKey: "private", Address: []string{"10.0.0.2/32"},
		Peers: []domain.WireGuardPeer{
			{Server: "one.example.com", Port: 51820, PublicKey: "one"},
			{Server: "two.example.com", Port: 51820, PublicKey: "two"},
		},
	}

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{zero, one, many}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, "[Proxy]\none = wireguard, wg.example.com, 51820, privateKey=private, publicKey=public, ip=10.0.0.2/32\n", string(out))
	require.Equal(t, 1, report.SuccessCount)
	require.Len(t, report.Warnings, 2)
	require.Equal(t, "zero", report.Warnings[0].Node)
	require.Equal(t, "many", report.Warnings[1].Node)
}

func TestRenderWireGuardWarnsWhenPeerOverridesConflictingTopLevelOptions(t *testing.T) {
	t.Parallel()

	udp := true
	node := domain.NodeIR{
		Name: "wireguard", Type: domain.NodeTypeWireGuard,
		Dialer: &domain.DialerOptions{UDPRelay: &udp},
		WireGuard: &domain.WireGuardOptions{
			PrivateKey: "private", Address: []string{"10.0.0.2/32"},
			PersistentKeepalive: 25, Reserved: []uint8{4, 5, 6},
			Peers: []domain.WireGuardPeer{{
				Server: "wg.example.com", Port: 51820, PublicKey: "public",
				PersistentKeepalive: 40, Reserved: []uint8{1, 2, 3},
			}},
		},
	}

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, "[Proxy]\nwireguard = wireguard, wg.example.com, 51820, privateKey=private, publicKey=public, ip=10.0.0.2/32, udp=1, keepalive=40, reserved=1/2/3\n", string(out))
	require.Equal(t, 1, report.SuccessCount)
	require.Equal(t, []string{"wireguard.persistent_keepalive", "wireguard.reserved"}, warningFields(report.Warnings))
	for _, warning := range report.Warnings {
		require.Equal(t, "render_lossy_field", warning.Code)
	}
}

func TestRenderSnellAcceptsOnlyV2OrUnspecifiedVersion(t *testing.T) {
	t.Parallel()

	nodes := []domain.NodeIR{
		{Name: "unspecified", Type: domain.NodeTypeSnell, Server: "one.example.com", Port: 44046, Password: "one"},
		{Name: "v2", Type: domain.NodeTypeSnell, Server: "two.example.com", Port: 44046, Password: "two", Snell: &domain.SnellOptions{Version: 2}},
		{Name: "v3", Type: domain.NodeTypeSnell, Server: "three.example.com", Port: 44046, Password: "three", Snell: &domain.SnellOptions{Version: 3}},
	}

	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, strings.Join([]string{
		"[Proxy]",
		"unspecified = snell, one.example.com, 44046, password=one",
		"v2 = snell, two.example.com, 44046, password=two",
		"",
	}, "\n"), string(out))
	require.Equal(t, 2, report.SuccessCount)
	require.Len(t, report.Warnings, 1)
	require.Equal(t, "render_node_skipped", report.Warnings[0].Code)
	require.Equal(t, "v3", report.Warnings[0].Node)
}

func TestRenderWarningsAreDeclaredByCapability(t *testing.T) {
	t.Parallel()

	node := domain.NodeIR{
		Name: "lossy", Type: domain.NodeTypeHTTP, Server: "http.example.com", Port: 8080,
		Network: "tcp", Path: "/proxy", Headers: map[string]string{"X-Test": "one"},
		Dialer:     &domain.DialerOptions{Network: "tcp", TFO: true},
		Multiplex:  &domain.MultiplexOptions{Enabled: true},
		UDPOverTCP: &domain.UDPOverTCPOptions{Enabled: true},
	}

	_, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, report.Warnings)

	capability := shadowrocket.NewRenderer().RenderCapabilities()[0]
	lossy := map[string]bool{}
	for _, field := range capability.Lossy {
		lossy[field.IRField] = true
	}
	for _, warning := range report.Warnings {
		if warning.Code == "render_lossy_field" {
			require.Truef(t, lossy[warning.Field], "warning field %q not declared in capability", warning.Field)
		}
	}
}

func TestRenderCapabilitiesAreRenderOnlyAndTraceFixedSource(t *testing.T) {
	t.Parallel()

	capabilities := shadowrocket.NewRenderer().RenderCapabilities()
	require.Len(t, capabilities, 1)
	capability := capabilities[0]
	require.Equal(t, "shadowrocket-proxies", capability.Format)
	require.Equal(t, shared.DirectionRender, capability.Direction)
	require.False(t, capability.Reversible)
	require.Equal(t, []domain.NodeType{
		domain.NodeTypeShadowsocks,
		domain.NodeTypeVMess,
		domain.NodeTypeVLESS,
		domain.NodeTypeTrojan,
		domain.NodeTypeHysteria,
		domain.NodeTypeHysteria2,
		domain.NodeTypeTUIC,
		domain.NodeTypeHTTP,
		domain.NodeTypeSOCKS,
		domain.NodeTypeWireGuard,
		domain.NodeTypeSnell,
	}, capability.Types)
	require.NotEmpty(t, capability.Fields)
	require.NotEmpty(t, capability.Lossy)
	supported := map[string]bool{}
	for _, field := range capability.Fields {
		supported[field.Protocol+"\x00"+field.IRField] = true
	}
	lossy := map[string]bool{}
	for _, field := range capability.Lossy {
		lossy[field.Protocol+"\x00"+field.IRField] = true
		require.Falsef(t, supported[field.Protocol+"\x00"+field.IRField], "%s.%s declared as both supported and lossy", field.Protocol, field.IRField)
	}
	for _, field := range append(append([]shared.FieldRef{}, capability.Fields...), capability.Lossy...) {
		require.Equal(t, shadowrocketRevision, field.SourceRef.Revision)
		require.Equal(t, "README.md", field.SourceRef.Path)
		require.Equal(t, "1222-1271", field.SourceRef.Lines)
	}
	require.True(t, supported[string(domain.NodeTypeHysteria)+"\x00hysteria.protocol"])
	require.False(t, supported[string(domain.NodeTypeHysteria)+"\x00network"])
	require.True(t, lossy[string(domain.NodeTypeHysteria)+"\x00network"])

	refs := shared.SourceRefs("shadowrocket-proxies")
	require.Len(t, refs, 1)
	require.Equal(t, shadowrocketRevision, refs[0].Revision)
	require.Equal(t, "github.com/LOWERTOP/Shadowrocket", refs[0].Repo)
}

func supportedNodes() []domain.NodeIR {
	udp := true
	return []domain.NodeIR{
		{
			Name: "ss", Type: domain.NodeTypeShadowsocks, Server: "ss.example.com", Port: 8388,
			Cipher: "aes-256-gcm", Password: "ss-secret", Plugin: "none",
			Transport: &domain.TransportOptions{Type: "websocket"},
		},
		{
			Name: "vmess", Type: domain.NodeTypeVMess, Server: "vmess.example.com", Port: 443,
			UUID:      "11111111-1111-1111-1111-111111111111",
			Transport: &domain.TransportOptions{Type: "websocket"},
			Dialer:    &domain.DialerOptions{TFO: true},
		},
		{
			Name: "vless", Type: domain.NodeTypeVLESS, Server: "vless.example.com", Port: 443,
			UUID: "22222222-2222-2222-2222-222222222222", Encryption: "none",
			TLS:       &domain.TLSOptions{Enabled: true, ServerName: "vless-sni.example.com"},
			Transport: &domain.TransportOptions{Type: "ws"},
		},
		{
			Name: "trojan", Type: domain.NodeTypeTrojan, Server: "trojan.example.com", Port: 443,
			Password: "trojan-secret",
			TLS:      &domain.TLSOptions{Enabled: true, ServerName: "trojan-sni.example.com", InsecureSkipVerify: true},
		},
		{
			Name: "hysteria", Type: domain.NodeTypeHysteria, Server: "hy.example.com", Port: 8443,
			TLS:      &domain.TLSOptions{Enabled: true, ServerName: "hy-sni.example.com", ALPN: []string{"h2"}},
			Dialer:   &domain.DialerOptions{UDPRelay: &udp},
			Hysteria: &domain.HysteriaOptions{Protocol: "udp", AuthString: "hy-secret", ObfsPassword: "hy-obfs", UpMbps: 100, DownMbps: 200},
		},
		{
			Name: "hysteria2", Type: domain.NodeTypeHysteria2, Server: "hy2.example.com", Port: 8443,
			Password: "hy2-secret",
			TLS:      &domain.TLSOptions{Enabled: true, ServerName: "hy2-sni.example.com", ALPN: []string{"h3"}},
			Dialer:   &domain.DialerOptions{UDPRelay: &udp},
			Hysteria: &domain.HysteriaOptions{Obfs: "salamander", ObfsPassword: "hy2-obfs"},
		},
		{
			Name: "tuic", Type: domain.NodeTypeTUIC, Server: "tuic.example.com", Port: 443,
			UUID: "33333333-3333-3333-3333-333333333333", Password: "tuic-secret",
			TLS:    &domain.TLSOptions{Enabled: true, ServerName: "tuic-sni.example.com", ALPN: []string{"h2"}},
			Dialer: &domain.DialerOptions{UDPRelay: &udp},
		},
		{Name: "http", Type: domain.NodeTypeHTTP, Server: "http.example.com", Port: 8080, Username: "http-user", Password: "http-pass"},
		{Name: "https", Type: domain.NodeTypeHTTP, Server: "https.example.com", Port: 8443, Username: "https-user", Password: "https-pass", TLS: &domain.TLSOptions{Enabled: true}},
		{Name: "socks", Type: domain.NodeTypeSOCKS, Server: "socks.example.com", Port: 1080, Username: "socks-user", Password: "socks-pass"},
		{Name: "socks-tls", Type: domain.NodeTypeSOCKS, Server: "socks-tls.example.com", Port: 1081, Username: "socks-tls-user", Password: "socks-tls-pass", TLS: &domain.TLSOptions{Enabled: true, InsecureSkipVerify: true}},
		{
			Name: "wireguard", Type: domain.NodeTypeWireGuard,
			Dialer: &domain.DialerOptions{UDPRelay: &udp},
			WireGuard: &domain.WireGuardOptions{
				PrivateKey: "private-key", IP: "10.0.0.2/32", Address: []string{"10.0.0.2/32"}, MTU: 1350,
				Peers: []domain.WireGuardPeer{{
					Server: "wg.example.com", Port: 51820, PublicKey: "public-key",
					Reserved: []uint8{1, 2, 3}, PersistentKeepalive: 40,
				}},
			},
		},
		{
			Name: "snell", Type: domain.NodeTypeSnell, Server: "snell.example.com", Port: 44046,
			Password: "snell-secret", Dialer: &domain.DialerOptions{UDPRelay: &udp},
			Snell: &domain.SnellOptions{Obfs: "http", ObfsHost: "obfs.example.com"},
		},
	}
}

func warningFields(warnings []domain.Warning) []string {
	fields := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		fields = append(fields, warning.Field)
	}
	return fields
}
