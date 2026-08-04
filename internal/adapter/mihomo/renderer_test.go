package mihomo_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mihomoutils "github.com/metacubex/mihomo/common/utils"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/jsonnodes"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	uriadapter "github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestRenderMihomoRenderWrapper(t *testing.T) {
	r := mihomo.NewRenderer()
	out, err := r.Render(context.Background(), allProtocolNodes()[:1], domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "ss") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRenderMihomoHysteriaBandwidthFromOfficialURIUsesNativeRates(t *testing.T) {
	nodes, _, err := uriadapter.NewParser().Parse(context.Background(), []byte(
		"hysteria://hy.example.com:8443?protocol=wechat-video&auth=secret&upmbps=100&downmbps=200&obfs=xplus&obfsParam=obfs-pass#hy",
	))
	require.NoError(t, err)

	out, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(out, &doc))
	require.Len(t, doc.Proxies, 1)
	require.Equal(t, "wechat-video", doc.Proxies[0]["protocol"])
	require.Equal(t, "obfs-pass", doc.Proxies[0]["obfs"])
	require.Equal(t, "100 Mbps", doc.Proxies[0]["up"])
	require.Equal(t, "200 Mbps", doc.Proxies[0]["down"])
	require.NotContains(t, doc.Proxies[0], "up-speed")
	require.NotContains(t, doc.Proxies[0], "down-speed")
	require.Equal(t, uint64(12_500_000), mihomoutils.StringToBps(doc.Proxies[0]["up"].(string)))
	require.Equal(t, uint64(25_000_000), mihomoutils.StringToBps(doc.Proxies[0]["down"].(string)))
}

func TestRenderMihomoHysteriaBandwidthUsesNativeMbps(t *testing.T) {
	out, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{{
		Name: "hy", Type: domain.NodeTypeHysteria, Server: "example.com", Port: 8443,
		Hysteria: &domain.HysteriaOptions{AuthString: "secret", UpMbps: 55, DownMbps: 100},
	}}, domain.RenderOptions{Format: "mihomo-proxies"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(out, &doc))
	require.Equal(t, "55 Mbps", doc.Proxies[0]["up"])
	require.Equal(t, "100 Mbps", doc.Proxies[0]["down"])
	require.NotContains(t, doc.Proxies[0], "up-speed")
	require.NotContains(t, doc.Proxies[0], "down-speed")
	require.Equal(t, uint64(6_875_000), mihomoutils.StringToBps(doc.Proxies[0]["up"].(string)))
	require.Equal(t, uint64(12_500_000), mihomoutils.StringToBps(doc.Proxies[0]["down"].(string)))
}

func TestRenderMihomoHysteriaBandwidthPreservesSingBoxBps(t *testing.T) {
	nodes, _, err := singbox.NewParser().Parse(context.Background(), []byte(`{
		"outbounds":[{"type":"hysteria","tag":"hy","server":"example.com","server_port":8443,"auth_str":"secret","up":55,"down":100}]
	}`))
	require.NoError(t, err)

	out, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(out, &doc))
	require.Equal(t, "55 Bps", doc.Proxies[0]["up"])
	require.Equal(t, "100 Bps", doc.Proxies[0]["down"])
	require.Equal(t, uint64(55), mihomoutils.StringToBps(doc.Proxies[0]["up"].(string)))
	require.Equal(t, uint64(100), mihomoutils.StringToBps(doc.Proxies[0]["down"].(string)))
}

func TestRenderMihomoHysteriaBandwidthPreservesExplicitUnits(t *testing.T) {
	out, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{{
		Name: "hy", Type: domain.NodeTypeHysteria, Server: "example.com", Port: 8443,
		Hysteria: &domain.HysteriaOptions{AuthString: "secret", Up: "55 Bps", Down: "640 KBps"},
	}}, domain.RenderOptions{Format: "mihomo-proxies"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(out, &doc))
	require.Equal(t, "55 Bps", doc.Proxies[0]["up"])
	require.Equal(t, "640 KBps", doc.Proxies[0]["down"])
}

func TestRenderMihomoHysteriaBandwidthSkipsInvalidNode(t *testing.T) {
	out, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{
		{Name: "valid", Type: domain.NodeTypeHTTP, Server: "valid.example", Port: 8080},
		{
			Name: "invalid", Type: domain.NodeTypeHysteria, Server: "invalid.example", Port: 8443,
			Hysteria: &domain.HysteriaOptions{AuthString: "secret", Up: "55", DownMbps: 100},
		},
	}, domain.RenderOptions{Format: "mihomo-proxies"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Len(t, report.Warnings, 1)
	require.Equal(t, "render_node_skipped", report.Warnings[0].Code)
	require.Equal(t, "invalid", report.Warnings[0].Node)
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(out, &doc))
	require.Len(t, doc.Proxies, 1)
	require.Equal(t, "valid", doc.Proxies[0]["name"])
}

func TestRenderMihomoHysteriaBandwidthSkipsDirectOverBoundMbps(t *testing.T) {
	max := shared.MaxHysteriaMbps()
	if max == int(^uint(0)>>1) {
		t.Skip("max+1 is not representable as int on this platform")
	}
	out, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{
		{
			Name: "over", Type: domain.NodeTypeHysteria, Server: "over.example", Port: 8443,
			Hysteria: &domain.HysteriaOptions{UpMbps: max + 1, DownMbps: max},
		},
		{Name: "valid", Type: domain.NodeTypeHTTP, Server: "valid.example", Port: 8080},
	}, domain.RenderOptions{Format: "mihomo-proxies"})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Len(t, report.Warnings, 1)
	require.Equal(t, "over", report.Warnings[0].Node)
	require.NotContains(t, string(out), "over.example")
}

func TestRenderMihomoHysteriaMigratesLegacyJSONObfsPassword(t *testing.T) {
	nodes, _, err := jsonnodes.NewParser().Parse(context.Background(), []byte(`[{"name":"hy","type":"hysteria","server":"hy.example.com","port":8443,"hysteria":{"auth_str":"secret","obfs":"legacy-password","up_mbps":20,"down_mbps":100}}]`))
	require.NoError(t, err)

	out, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Contains(t, string(out), "obfs: legacy-password")
}

func TestRenderMihomoProxies(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:     "node-a",
			Type:     domain.NodeTypeShadowsocks,
			Server:   "example.com",
			Port:     8388,
			Cipher:   "aes-128-gcm",
			Password: "secret",
		},
		{
			Name:   "vmess-ws-tls",
			Type:   domain.NodeTypeVMess,
			Server: "example.com",
			Port:   443,
			UUID:   "11111111-1111-1111-1111-111111111111",
			Cipher: "auto",
			TLS:    &domain.TLSOptions{Enabled: true, ServerName: "example.com"},
			Transport: &domain.TransportOptions{
				Type: "websocket",
				Path: "/ws",
				Host: "cdn.example.com",
			},
			Raw: map[string]json.RawMessage{
				"vmess.alter_id": json.RawMessage("0"),
				"vmess.v":        json.RawMessage(`"2"`),
			},
		},
	}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if report.SuccessCount != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Field != "vmess.v" {
		t.Fatalf("unexpected warnings: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxies, ok := doc["proxies"].([]any)
	if !ok || len(proxies) != 2 {
		t.Fatalf("unexpected proxies: %#v", doc["proxies"])
	}
	first := proxies[0].(map[string]any)
	if first["type"] != "ss" || first["cipher"] != "aes-128-gcm" {
		t.Fatalf("unexpected first proxy: %#v", first)
	}
	second := proxies[1].(map[string]any)
	if second["type"] != "vmess" || second["network"] != "ws" {
		t.Fatalf("unexpected second proxy: %#v", second)
	}
}

func TestRenderMihomoAllProtocols(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := allProtocolNodes()
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if report.SuccessCount != len(nodes) {
		t.Fatalf("unexpected report: %#v", report)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxies, ok := doc["proxies"].([]any)
	if !ok || len(proxies) != len(nodes) {
		t.Fatalf("unexpected proxies: %#v", doc["proxies"])
	}
	types := map[string]bool{}
	for _, item := range proxies {
		proxy := item.(map[string]any)
		types[proxy["type"].(string)] = true
	}
	for _, typ := range []string{"ss", "vmess", "vless", "trojan", "hysteria", "hysteria2", "tuic", "socks5", "http", "wireguard"} {
		if !types[typ] {
			t.Fatalf("missing type %s in %#v", typ, types)
		}
	}
}

func TestRenderMihomoSkipsBadNodeWithWarning(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:     "ss",
			Type:     domain.NodeTypeShadowsocks,
			Server:   "ss.example.com",
			Port:     8388,
			Cipher:   "aes-128-gcm",
			Password: "secret",
		},
		{
			Name: "bad-vmess",
			Type: domain.NodeTypeVMess,
			UUID: "11111111-1111-1111-1111-111111111111",
		},
	}

	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if report.SuccessCount != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Code != "render_node_skipped" {
		t.Fatalf("expected skipped-node warning: %#v", report.Warnings)
	}
	warning := report.Warnings[0]
	if warning.Node != "bad-vmess" || warning.Target != "mihomo-proxies" {
		t.Fatalf("unexpected skipped warning identity: %#v", warning)
	}
	if warning.NodeContext == nil || warning.NodeContext.Type != domain.NodeTypeVMess {
		t.Fatalf("missing skipped warning context: %#v", warning)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxies := doc["proxies"].([]any)
	if len(proxies) != 1 || proxies[0].(map[string]any)["name"] != "ss" {
		t.Fatalf("unexpected proxies: %#v", proxies)
	}
}

func TestRenderMihomoErrorsWhenAllNodesSkipped(t *testing.T) {
	r := mihomo.NewRenderer()
	_, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{{
		Name: "bad-vmess",
		Type: domain.NodeTypeVMess,
		UUID: "11111111-1111-1111-1111-111111111111",
	}}, domain.RenderOptions{Format: "mihomo-proxies"})

	if err == nil {
		t.Fatal("expected render error")
	}
	if !domain.IsCode(err, domain.CodeRenderFailed) {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.SuccessCount != 0 || len(report.Warnings) != 1 || report.Warnings[0].Code != "render_node_skipped" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestRenderMihomoSSRAndSnell(t *testing.T) {
	r := mihomo.NewRenderer()
	out, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{
		{
			Name:     "ssr",
			Type:     domain.NodeTypeShadowsocksR,
			Server:   "ssr.example.com",
			Port:     8388,
			Cipher:   "aes-128-cfb",
			Password: "secret",
			ShadowsocksR: &domain.ShadowsocksROptions{
				Protocol:      "auth_sha1_v4",
				ProtocolParam: "param",
				Obfs:          "http_simple",
				ObfsParam:     "cdn.example.com",
			},
		},
		{
			Name:     "snell",
			Type:     domain.NodeTypeSnell,
			Server:   "snell.example.com",
			Port:     44046,
			Password: "psk-secret",
			Snell:    &domain.SnellOptions{Version: 3, Obfs: "tls", ObfsHost: "cdn.example.com"},
		},
	}, domain.RenderOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if report.SuccessCount != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}
	body := string(out)
	for _, want := range []string{"type: ssr", "protocol: auth_sha1_v4", "obfs-param: cdn.example.com", "type: snell", "psk: psk-secret", "version: 3"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}

func TestRenderMihomoTLSFingerprintSplit(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:       "client-only",
			Type:       domain.NodeTypeVLESS,
			Server:     "example.com",
			Port:       443,
			UUID:       "11111111-1111-1111-1111-111111111111",
			Encryption: "none",
			TLS:        &domain.TLSOptions{Enabled: true, ClientFingerprint: "chrome"},
		},
		{
			Name:       "cert-only",
			Type:       domain.NodeTypeVLESS,
			Server:     "example.com",
			Port:       443,
			UUID:       "11111111-1111-1111-1111-111111111112",
			Encryption: "none",
			TLS:        &domain.TLSOptions{Enabled: true, Fingerprint: "sha256:0123"},
		},
	}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxies := doc["proxies"].([]any)
	clientOnly := proxies[0].(map[string]any)
	if clientOnly["client-fingerprint"] != "chrome" || clientOnly["fingerprint"] != nil {
		t.Fatalf("unexpected client proxy: %#v", clientOnly)
	}
	certOnly := proxies[1].(map[string]any)
	if certOnly["fingerprint"] != "sha256:0123" || certOnly["client-fingerprint"] != nil {
		t.Fatalf("unexpected certificate proxy: %#v", certOnly)
	}
}

func TestRenderMihomoRealityShortIDStaysString(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:       "sid-08",
			Type:       domain.NodeTypeVLESS,
			Server:     "example.com",
			Port:       443,
			UUID:       "11111111-1111-1111-1111-111111111111",
			Encryption: "none",
			TLS:        &domain.TLSOptions{Enabled: true, Reality: &domain.RealityOptions{PublicKey: "public", ShortID: "08"}},
		},
		{
			Name:       "sid-0088",
			Type:       domain.NodeTypeVLESS,
			Server:     "example.org",
			Port:       443,
			UUID:       "11111111-1111-1111-1111-111111111112",
			Encryption: "none",
			TLS:        &domain.TLSOptions{Enabled: true, Reality: &domain.RealityOptions{PublicKey: "public", ShortID: "0088"}},
		},
	}

	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxies := doc["proxies"].([]any)
	for i, want := range []string{"08", "0088"} {
		reality := proxies[i].(map[string]any)["reality-opts"].(map[string]any)
		if reality["short-id"] != want {
			t.Fatalf("short-id parsed as non-string or changed: %#v", reality["short-id"])
		}
	}
}

func TestRenderMihomoWebSocketEarlyData(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:   "ws-early",
		Type:   domain.NodeTypeVMess,
		Server: "example.com",
		Port:   443,
		UUID:   "11111111-1111-1111-1111-111111111111",
		Transport: &domain.TransportOptions{
			Type:                "websocket",
			Path:                "/ws",
			MaxEarlyData:        2048,
			EarlyDataHeaderName: "Sec-WebSocket-Protocol",
		},
	}}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxy := doc["proxies"].([]any)[0].(map[string]any)
	opts := proxy["ws-opts"].(map[string]any)
	if opts["max-early-data"] != 2048 || opts["early-data-header-name"] != "Sec-WebSocket-Protocol" {
		t.Fatalf("unexpected ws opts: %#v", opts)
	}
}

func TestRenderMihomoWireGuardMultiPeers(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:   "wg",
		Type:   domain.NodeTypeWireGuard,
		Server: "wg.example.com",
		Port:   51820,
		WireGuard: &domain.WireGuardOptions{
			PrivateKey:          "private",
			Address:             []string{"10.0.0.2/32", "fd00::2/128"},
			MTU:                 1408,
			Workers:             2,
			PersistentKeepalive: 25,
			Peers: []domain.WireGuardPeer{
				{Server: "p1.example.com", Port: 51820, PublicKey: "pub1", AllowedIPs: []string{"0.0.0.0/0"}},
				{Server: "p2.example.com", Port: 51821, PublicKey: "pub2", PreSharedKey: "psk", Reserved: []uint8{1, 2, 3}},
			},
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	body := string(out)
	for _, want := range []string{"peers:", "pub1", "pub2", "persistent-keepalive", "reserved"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
}

func TestRenderMihomoLossWarningsCoveredByCapability(t *testing.T) {
	r := mihomo.NewRenderer()
	_, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{
		{
			Name:   "vmess",
			Type:   domain.NodeTypeVMess,
			Server: "example.com",
			Port:   443,
			UUID:   "11111111-1111-1111-1111-111111111111",
			Multiplex: &domain.MultiplexOptions{
				Enabled:    true,
				MaxStreams: 8,
			},
		},
		{
			Name:     "hy2",
			Type:     domain.NodeTypeHysteria2,
			Server:   "example.com",
			Port:     8443,
			Password: "secret",
			Hysteria: &domain.HysteriaOptions{
				UpMbps:   20,
				DownMbps: 100,
			},
		},
		{
			Name:   "http",
			Type:   domain.NodeTypeHTTP,
			Server: "example.com",
			Port:   8080,
			Path:   "/proxy",
		},
	}, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	lossy := capabilityLossyFields(mihomo.NewRenderer().RenderCapabilities()[0])
	for _, warning := range report.Warnings {
		if warning.Code != "render_lossy_field" {
			continue
		}
		if !lossy[warning.Field] {
			t.Fatalf("warning field %q not declared in capability lossy list: %#v", warning.Field, report.Warnings)
		}
	}
}

func TestRenderMihomoSSSIP002SimpleObfsPlugin(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:          "ss-plugin",
		Type:          domain.NodeTypeShadowsocks,
		Server:        "example.com",
		Port:          8388,
		Cipher:        "aes-256-gcm",
		Password:      "p@ss",
		Plugin:        "obfs-local",
		PluginOptions: map[string]any{"raw": "obfs=http;obfs-host=cdn.example.com"},
	}}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxy := doc["proxies"].([]any)[0].(map[string]any)
	if proxy["plugin"] != "obfs" {
		t.Fatalf("unexpected plugin: %#v", proxy)
	}
	pluginOpts := proxy["plugin-opts"].(map[string]any)
	if pluginOpts["mode"] != "http" || pluginOpts["host"] != "cdn.example.com" || pluginOpts["raw"] != nil {
		t.Fatalf("unexpected plugin-opts: %#v", pluginOpts)
	}
}

func TestRenderMihomoSSPluginAndHTTPTransport(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:          "ss-http",
		Type:          domain.NodeTypeShadowsocks,
		Server:        "example.com",
		Port:          8388,
		Cipher:        "aes-128-gcm",
		Password:      "secret",
		Plugin:        "obfs-local",
		PluginOptions: map[string]any{"mode": "tls"},
		Transport: &domain.TransportOptions{
			Type:    "http",
			Method:  "GET",
			Path:    "/api",
			Headers: map[string]string{"User-Agent": "curl"},
		},
		UDPOverTCP: &domain.UDPOverTCPOptions{Enabled: true, Version: 2},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	body := string(out)
	for _, want := range []string{"plugin:", "udp-over-tcp-version"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body)
		}
	}
	for _, unwanted := range []string{"h2-opts", "/api"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("unexpected unsupported transport field %q in %s", unwanted, body)
		}
	}
}

func TestRenderMihomoOmitsDefaultTCPTransport(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:   "vmess-tcp",
		Type:   domain.NodeTypeVMess,
		Server: "example.com",
		Port:   443,
		UUID:   "11111111-1111-1111-1111-111111111111",
		Transport: &domain.TransportOptions{
			Type: "tcp",
		},
	}}

	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", report.Warnings)
	}
	body := string(out)
	if strings.Contains(body, "network: tcp") {
		t.Fatalf("unexpected default tcp network: %s", body)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxy := doc["proxies"].([]any)[0].(map[string]any)
	if proxy["network"] != nil {
		t.Fatalf("unexpected default tcp network: %#v", proxy)
	}
}

func TestRenderMihomoSkipsUnsupportedTrojanXHTTP(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:     "trojan-xhttp",
		Type:     domain.NodeTypeTrojan,
		Server:   "example.com",
		Port:     443,
		Password: "secret",
		TLS:      &domain.TLSOptions{Enabled: true, Reality: &domain.RealityOptions{PublicKey: "public", ShortID: "08"}},
		Transport: &domain.TransportOptions{
			Type: "xhttp",
			Path: "/xhttp",
			Host: "cdn.example.com",
		},
	}}

	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Field != "transport.type" {
		t.Fatalf("expected transport.type warning: %#v", report.Warnings)
	}
	body := string(out)
	for _, unwanted := range []string{"network: xhttp", "xhttp-opts"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("unexpected unsupported xhttp output %q in %s", unwanted, body)
		}
	}
	if !strings.Contains(body, "reality-opts") {
		t.Fatalf("expected supported reality opts: %s", body)
	}
}

func TestRenderMihomoVLESSXHTTPOptions(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:       "vless-xhttp",
		Type:       domain.NodeTypeVLESS,
		Server:     "example.com",
		Port:       443,
		UUID:       "11111111-1111-1111-1111-111111111111",
		Encryption: "none",
		Transport: &domain.TransportOptions{
			Type:    "xhttp",
			Path:    "/xhttp",
			Host:    "cdn.example.com",
			Headers: map[string]string{"User-Agent": "curl"},
			XHTTP: &domain.XHTTPTransportOptions{
				Mode: "packet-up",
				ReuseSettings: &domain.XHTTPReuseSettings{
					MaxConcurrency: "2-4",
					MaxConnections: "1",
				},
			},
		},
	}}

	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	require.NoError(t, err)
	require.Empty(t, report.Warnings)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out, &doc))
	proxy := doc["proxies"].([]any)[0].(map[string]any)
	require.Equal(t, "xhttp", proxy["network"])
	opts := proxy["xhttp-opts"].(map[string]any)
	require.Equal(t, "/xhttp", opts["path"])
	require.Equal(t, "cdn.example.com", opts["host"])
	require.Equal(t, "curl", opts["headers"].(map[string]any)["User-Agent"])
	require.Equal(t, "packet-up", opts["mode"])
	require.Equal(t, "2-4", opts["reuse-settings"].(map[string]any)["max-concurrency"])
	require.Equal(t, "1", opts["reuse-settings"].(map[string]any)["max-connections"])
}

func TestRenderMihomoWebSocketHTTPUpgradeOptions(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:       "vless-upgrade",
		Type:       domain.NodeTypeVLESS,
		Server:     "example.com",
		Port:       443,
		UUID:       "11111111-1111-1111-1111-111111111111",
		Encryption: "none",
		Transport: &domain.TransportOptions{
			Type:                     "websocket",
			Path:                     "/upgrade",
			Host:                     "cdn.example.com",
			V2RayHTTPUpgrade:         true,
			V2RayHTTPUpgradeFastOpen: true,
		},
	}}

	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	require.NoError(t, err)
	require.Empty(t, report.Warnings)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out, &doc))
	proxy := doc["proxies"].([]any)[0].(map[string]any)
	require.Equal(t, "ws", proxy["network"])
	opts := proxy["ws-opts"].(map[string]any)
	require.Equal(t, true, opts["v2ray-http-upgrade"])
	require.Equal(t, true, opts["v2ray-http-upgrade-fast-open"])
}

func TestRenderMihomoHysteriaHopInterval(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:   "hy",
		Type:   domain.NodeTypeHysteria,
		Server: "example.com",
		Port:   8443,
		Hysteria: &domain.HysteriaOptions{
			HopInterval: "30s",
			AuthString:  "secret",
			UpMbps:      20,
			DownMbps:    100,
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "hop-interval: 30") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRenderMihomoHysteriaBandwidthUsesNativeMbpsLegacyCase(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:   "hy",
		Type:   domain.NodeTypeHysteria,
		Server: "example.com",
		Port:   8443,
		Hysteria: &domain.HysteriaOptions{
			Auth:     "secret",
			UpMbps:   20,
			DownMbps: 100,
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxy := doc["proxies"].([]any)[0].(map[string]any)
	if proxy["up"] != "20 Mbps" || proxy["down"] != "100 Mbps" {
		t.Fatalf("unexpected hysteria speeds: %#v", proxy)
	}
	if _, exists := proxy["up-speed"]; exists {
		t.Fatalf("unexpected compatibility upload speed: %#v", proxy)
	}
	if _, exists := proxy["down-speed"]; exists {
		t.Fatalf("unexpected compatibility download speed: %#v", proxy)
	}
}

func TestRenderMihomoTUICOptions(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:     "tuic",
		Type:     domain.NodeTypeTUIC,
		Server:   "example.com",
		Port:     443,
		UUID:     "11111111-1111-1111-1111-111111111111",
		Password: "secret",
		TUIC: &domain.TUICOptions{
			CongestionControl:    "bbr",
			ReduceRTT:            true,
			UDPOverStream:        true,
			UDPOverStreamVersion: 2,
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	body := string(out)
	for _, want := range []string{"reduce-rtt", "udp-over-stream", "congestion-controller"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
}

func TestRenderMihomoHysteria2Realm(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:     "hy2",
		Type:     domain.NodeTypeHysteria2,
		Server:   "example.com",
		Port:     8443,
		Password: "secret",
		Hysteria: &domain.HysteriaOptions{
			UpMbps:   20,
			DownMbps: 100,
			CWND:     32,
			UDPMTU:   1200,
			Realm: &domain.HysteriaRealmOptions{
				Enabled:     true,
				ServerURL:   "https://realm.example.com",
				Token:       "tok",
				RealmID:     "id",
				STUNServers: []string{"stun.example.com"},
			},
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "realm-opts") {
		t.Fatalf("missing realm-opts: %s", out)
	}
}

func TestRenderMihomoHTTPTransport(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:   "http-net",
		Type:   domain.NodeTypeVMess,
		Server: "example.com",
		Port:   443,
		UUID:   "11111111-1111-1111-1111-111111111111",
		Transport: &domain.TransportOptions{
			Type:   "http",
			Method: "GET",
			Path:   "/api",
			Hosts:  []string{"h1.example.com", "h2.example.com"},
		},
		UDPOverTCP: &domain.UDPOverTCPOptions{Enabled: true, Version: 1},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxy := doc["proxies"].([]any)[0].(map[string]any)
	if proxy["network"] != "h2" {
		t.Fatalf("unexpected proxy: %#v", proxy)
	}
}

func TestRenderMihomoTransportAndWireGuard(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:   "grpc",
			Type:   domain.NodeTypeVMess,
			Server: "example.com",
			Port:   443,
			UUID:   "11111111-1111-1111-1111-111111111111",
			Transport: &domain.TransportOptions{
				Type:        "grpc",
				ServiceName: "mysvc",
			},
			Multiplex:  &domain.MultiplexOptions{Enabled: true, MaxConnections: 4},
			UDPOverTCP: &domain.UDPOverTCPOptions{Enabled: true, Version: 2},
		},
		{
			Name:   "wg",
			Type:   domain.NodeTypeWireGuard,
			Server: "wg.example.com",
			Port:   51820,
			WireGuard: &domain.WireGuardOptions{
				PrivateKey: "private",
				Address:    []string{"10.0.0.2/32"},
				Peers: []domain.WireGuardPeer{{
					Server:       "wg.example.com",
					Port:         51820,
					PublicKey:    "public",
					PreSharedKey: "psk",
					AllowedIPs:   []string{"0.0.0.0/0"},
				}},
				MTU: 1408,
			},
		},
	}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if report.SuccessCount != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxies := doc["proxies"].([]any)
	grpcProxy := proxies[0].(map[string]any)
	if grpcProxy["network"] != "grpc" {
		t.Fatalf("unexpected grpc proxy: %#v", grpcProxy)
	}
	wgProxy := proxies[1].(map[string]any)
	if wgProxy["type"] != "wireguard" {
		t.Fatalf("unexpected wireguard proxy: %#v", wgProxy)
	}
}

func TestRenderMihomoTLSFullOptions(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:       "vless",
		Type:       domain.NodeTypeVLESS,
		Server:     "example.com",
		Port:       443,
		UUID:       "11111111-1111-1111-1111-111111111111",
		Encryption: "none",
		TLS: &domain.TLSOptions{
			Enabled:            true,
			ServerName:         "sni.example.com",
			InsecureSkipVerify: true,
			ALPN:               []string{"h2", "http/1.1"},
			ClientFingerprint:  "chrome",
			Fingerprint:        "sha256:abcd",
			ECH: &domain.ECHOptions{
				Enabled:         true,
				Config:          []string{"ech-config", "ignored-second-config"},
				QueryServerName: "ech.example.com",
			},
			Reality: &domain.RealityOptions{PublicKey: "public", ShortID: "abcd"},
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxy := doc["proxies"].([]any)[0].(map[string]any)
	if proxy["tls"] != true || proxy["servername"] != "sni.example.com" || proxy["skip-cert-verify"] != true {
		t.Fatalf("unexpected tls proxy: %#v", proxy)
	}
	if proxy["client-fingerprint"] != "chrome" || proxy["fingerprint"] != "sha256:abcd" {
		t.Fatalf("unexpected fingerprint fields: %#v", proxy)
	}
	ech := proxy["ech-opts"].(map[string]any)
	if ech["enable"] != true || ech["config"] != "ech-config" || ech["query-server-name"] != "ech.example.com" {
		t.Fatalf("unexpected ech opts: %#v", ech)
	}
	reality := proxy["reality-opts"].(map[string]any)
	if reality["public-key"] != "public" || reality["short-id"] != "abcd" {
		t.Fatalf("unexpected reality opts: %#v", reality)
	}
}

func TestRenderMihomoHTTPUpgradeHeadersAsStringLists(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:   "vmess-httpupgrade",
		Type:   domain.NodeTypeVMess,
		Server: "example.com",
		Port:   443,
		UUID:   "11111111-1111-1111-1111-111111111111",
		Transport: &domain.TransportOptions{
			Type:    "httpupgrade",
			Method:  "GET",
			Path:    "/upgrade",
			Headers: map[string]string{"User-Agent": "curl"},
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxy := doc["proxies"].([]any)[0].(map[string]any)
	if proxy["network"] != "http" {
		t.Fatalf("unexpected network: %#v", proxy)
	}
	opts := proxy["http-opts"].(map[string]any)
	if opts["method"] != "GET" || opts["path"].([]any)[0] != "/upgrade" {
		t.Fatalf("unexpected http opts: %#v", opts)
	}
	headers := opts["headers"].(map[string]any)
	if headers["User-Agent"].([]any)[0] != "curl" {
		t.Fatalf("unexpected headers: %#v", headers)
	}
}

func TestRenderMihomoHysteriaHopIntervalVariants(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:   "numeric",
			Type:   domain.NodeTypeHysteria,
			Server: "example.com",
			Port:   8443,
			Hysteria: &domain.HysteriaOptions{
				AuthString:  "secret",
				HopInterval: " 45 ",
				UpMbps:      20,
				DownMbps:    100,
			},
		},
		{
			Name:   "string",
			Type:   domain.NodeTypeHysteria,
			Server: "example.com",
			Port:   8443,
			Hysteria: &domain.HysteriaOptions{
				AuthString:  "secret",
				HopInterval: "fast",
				UpMbps:      20,
				DownMbps:    100,
			},
		},
	}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxies := doc["proxies"].([]any)
	if proxies[0].(map[string]any)["hop-interval"] != 45 {
		t.Fatalf("unexpected numeric hop interval: %#v", proxies[0])
	}
	if proxies[1].(map[string]any)["hop-interval"] != "fast" {
		t.Fatalf("unexpected string hop interval: %#v", proxies[1])
	}
}

func TestRenderMihomoReportsCanonicalNetworkAsLossy(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:     "ss",
		Type:     domain.NodeTypeShadowsocks,
		Server:   "example.com",
		Port:     8388,
		Cipher:   "aes-128-gcm",
		Password: "secret",
		Network:  "udp",
	}}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Field != "network" {
		t.Fatalf("expected network loss warning: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxy := doc["proxies"].([]any)[0].(map[string]any)
	if proxy["network"] != nil {
		t.Fatalf("canonical network must not be rendered as mihomo transport: %#v", proxy)
	}
}

func TestRenderMihomoExtraKeysAreSorted(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:     "http",
		Type:     domain.NodeTypeHTTP,
		Server:   "example.com",
		Port:     8443,
		Username: "user",
		Password: "pass",
		TLS: &domain.TLSOptions{
			Enabled: true,
			ALPN:    []string{"h2"},
			ECH: &domain.ECHOptions{
				Enabled: true,
				Config:  []string{"ech-config"},
			},
			Reality: &domain.RealityOptions{PublicKey: "public", ShortID: "sid"},
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	body := string(out)
	alpn := strings.Index(body, "alpn:")
	ech := strings.Index(body, "ech-opts:")
	reality := strings.Index(body, "reality-opts:")
	if alpn < 0 || ech < 0 || reality < 0 || !(alpn < ech && ech < reality) {
		t.Fatalf("extra keys are not sorted in output: %s", body)
	}
}

func TestRenderMihomoDialerTFO(t *testing.T) {
	r := mihomo.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:     "hy2",
			Type:     domain.NodeTypeHysteria2,
			Server:   "example.com",
			Port:     8443,
			Password: "secret",
			Dialer:   &domain.DialerOptions{TFO: true},
		},
		{
			Name:   "hy",
			Type:   domain.NodeTypeHysteria,
			Server: "example.com",
			Port:   8443,
			Hysteria: &domain.HysteriaOptions{
				AuthString: "secret",
				UpMbps:     20,
				DownMbps:   100,
			},
			Dialer: &domain.DialerOptions{TFO: true},
		},
	}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxies := doc["proxies"].([]any)
	hy2 := proxies[0].(map[string]any)
	if hy2["tfo"] != true || hy2["fast-open"] != nil {
		t.Fatalf("unexpected hysteria2 proxy: %#v", hy2)
	}
	hy := proxies[1].(map[string]any)
	if hy["fast-open"] != true || hy["tfo"] != nil {
		t.Fatalf("unexpected hysteria proxy: %#v", hy)
	}
}

func TestRenderMihomoQuickSettingsFields(t *testing.T) {
	r := mihomo.NewRenderer()
	udpOff := false
	udpOn := true
	nodes := []domain.NodeIR{
		{
			Name:     "ss",
			Type:     domain.NodeTypeShadowsocks,
			Server:   "example.com",
			Port:     8388,
			Cipher:   "aes-128-gcm",
			Password: "secret",
			Dialer:   &domain.DialerOptions{UDPRelay: &udpOff, TFO: true},
		},
		{
			Name:    "vmess",
			Type:    domain.NodeTypeVMess,
			Server:  "example.com",
			Port:    443,
			UUID:    "11111111-1111-1111-1111-111111111111",
			AlterID: 0,
			Dialer:  &domain.DialerOptions{UDPRelay: &udpOn},
			TLS:     &domain.TLSOptions{Enabled: true, InsecureSkipVerify: true},
		},
	}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	proxies := doc["proxies"].([]any)
	ss := proxies[0].(map[string]any)
	if value, ok := ss["udp"]; !ok || value != false {
		t.Fatalf("expected explicit udp false: %#v", ss)
	}
	if ss["tfo"] != true {
		t.Fatalf("expected tfo true: %#v", ss)
	}
	vmess := proxies[1].(map[string]any)
	if vmess["udp"] != true || vmess["alterId"] != 0 || vmess["skip-cert-verify"] != true {
		t.Fatalf("unexpected vmess proxy: %#v", vmess)
	}
}

func TestRenderMihomoUDPRelayLossyForUnsupportedProtocol(t *testing.T) {
	r := mihomo.NewRenderer()
	udp := true
	_, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{{
		Name:   "http",
		Type:   domain.NodeTypeHTTP,
		Server: "example.com",
		Port:   8080,
		Dialer: &domain.DialerOptions{UDPRelay: &udp},
	}}, domain.RenderOptions{Format: "mihomo-proxies"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Field != "dialer.udp_relay" {
		t.Fatalf("expected udp relay warning: %#v", report.Warnings)
	}
}

func allProtocolNodes() []domain.NodeIR {
	return []domain.NodeIR{
		{
			Name:     "ss",
			Type:     domain.NodeTypeShadowsocks,
			Server:   "ss.example.com",
			Port:     8388,
			Cipher:   "aes-128-gcm",
			Password: "secret",
		},
		{
			Name:    "vmess",
			Type:    domain.NodeTypeVMess,
			Server:  "vmess.example.com",
			Port:    443,
			UUID:    "11111111-1111-1111-1111-111111111111",
			Cipher:  "auto",
			AlterID: 0,
			TLS:     &domain.TLSOptions{Enabled: true, ServerName: "vmess.example.com"},
			Transport: &domain.TransportOptions{
				Type: "websocket",
				Path: "/ws",
				Host: "cdn.example.com",
			},
		},
		{
			Name:       "vless",
			Type:       domain.NodeTypeVLESS,
			Server:     "vless.example.com",
			Port:       443,
			UUID:       "11111111-1111-1111-1111-111111111112",
			Flow:       "xtls-rprx-vision",
			Encryption: "none",
			TLS:        &domain.TLSOptions{Enabled: true, Reality: &domain.RealityOptions{Enabled: true, PublicKey: "public", ShortID: "abcd"}},
		},
		{
			Name:     "trojan",
			Type:     domain.NodeTypeTrojan,
			Server:   "trojan.example.com",
			Port:     443,
			Password: "secret",
			TLS:      &domain.TLSOptions{Enabled: true, ServerName: "trojan.example.com"},
		},
		{
			Name:   "hysteria",
			Type:   domain.NodeTypeHysteria,
			Server: "hy.example.com",
			Port:   8443,
			TLS:    &domain.TLSOptions{Enabled: true, ServerName: "hy.example.com"},
			Hysteria: &domain.HysteriaOptions{
				UpMbps:       20,
				DownMbps:     100,
				AuthString:   "secret",
				Obfs:         "xplus",
				ObfsPassword: "obfs",
			},
		},
		{
			Name:     "hysteria2",
			Type:     domain.NodeTypeHysteria2,
			Server:   "hy2.example.com",
			Port:     8443,
			Password: "secret",
			Hysteria: &domain.HysteriaOptions{
				Obfs:         "salamander",
				ObfsPassword: "obfs",
				BBRProfile:   "desktop",
			},
		},
		{
			Name:     "tuic",
			Type:     domain.NodeTypeTUIC,
			Server:   "tuic.example.com",
			Port:     443,
			UUID:     "11111111-1111-1111-1111-111111111113",
			Password: "secret",
			TUIC:     &domain.TUICOptions{CongestionControl: "bbr", UDPRelayMode: "native"},
			TLS:      &domain.TLSOptions{Enabled: true, ServerName: "tuic.example.com"},
		},
		{
			Name:     "socks",
			Type:     domain.NodeTypeSOCKS,
			Server:   "socks.example.com",
			Port:     1080,
			Username: "user",
			Password: "pass",
		},
		{
			Name:     "http",
			Type:     domain.NodeTypeHTTP,
			Server:   "http.example.com",
			Port:     8080,
			Username: "user",
			Password: "pass",
			Headers:  map[string]string{"X-Test": "yes"},
		},
		{
			Name:   "wireguard",
			Type:   domain.NodeTypeWireGuard,
			Server: "wg.example.com",
			Port:   51820,
			WireGuard: &domain.WireGuardOptions{
				PrivateKey: "private",
				Address:    []string{"10.0.0.2/32"},
				Peers: []domain.WireGuardPeer{{
					Server:       "wg.example.com",
					Port:         51820,
					PublicKey:    "public",
					PreSharedKey: "psk",
					AllowedIPs:   []string{"0.0.0.0/0"},
				}},
				MTU: 1408,
			},
		},
	}
}

func capabilityLossyFields(capability shared.Capability) map[string]bool {
	out := map[string]bool{}
	for _, field := range capability.Lossy {
		out[field.IRField] = true
	}
	return out
}
