package singbox_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/jsonnodes"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	uriadapter "github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestRenderSingBoxTransportAndWireGuard(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:     "grpc",
			Type:     domain.NodeTypeTrojan,
			Server:   "example.com",
			Port:     443,
			Password: "secret",
			Transport: &domain.TransportOptions{
				Type:        "grpc",
				ServiceName: "mysvc",
			},
			Multiplex: &domain.MultiplexOptions{Enabled: true, MaxConnections: 4},
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
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if report.SuccessCount != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	outbounds := doc["outbounds"].([]any)
	if outbounds[0].(map[string]any)["transport"] == nil {
		t.Fatalf("missing transport: %#v", outbounds[0])
	}
	endpoints := doc["endpoints"].([]any)
	if endpoints[0].(map[string]any)["type"] != "wireguard" {
		t.Fatalf("unexpected endpoint: %#v", endpoints[0])
	}
}

func TestRenderSingBoxHysteriaFromOfficialURIUsesCanonicalObfsPassword(t *testing.T) {
	nodes, _, err := uriadapter.NewParser().Parse(context.Background(), []byte(
		"hysteria://hy.example.com:8443?protocol=wechat-video&auth=secret&upmbps=100&downmbps=200&obfs=xplus&obfsParam=obfs-pass#hy",
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Field != "hysteria.protocol" {
		t.Fatalf("expected protocol lossy warning: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	outbounds := doc["outbounds"].([]any)
	outbound := outbounds[0].(map[string]any)
	if outbound["obfs"] != "obfs-pass" {
		t.Fatalf("unexpected obfs password: %#v", outbound)
	}
	if _, ok := outbound["protocol"]; ok {
		t.Fatalf("unexpected protocol field: %#v", outbound)
	}
}

func TestRenderSingBoxHysteriaMigratesLegacyJSONObfsPassword(t *testing.T) {
	nodes, _, err := jsonnodes.NewParser().Parse(context.Background(), []byte(`[{"name":"hy","type":"hysteria","server":"hy.example.com","port":8443,"hysteria":{"auth_str":"secret","obfs":"legacy-password"}}]`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if report.SuccessCount != 1 || !strings.Contains(string(out), `"obfs": "legacy-password"`) {
		t.Fatalf("unexpected output/report: %s %#v", out, report)
	}
}

func TestRenderSingBoxDialerTFO(t *testing.T) {
	r := singbox.NewRenderer()
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
			Name:   "wg",
			Type:   domain.NodeTypeWireGuard,
			Server: "wg.example.com",
			Port:   51820,
			Dialer: &domain.DialerOptions{TFO: true},
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
			},
		},
	}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if report.SuccessCount != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	outbounds := doc["outbounds"].([]any)
	if outbounds[0].(map[string]any)["tcp_fast_open"] != true {
		t.Fatalf("unexpected hysteria2 outbound: %#v", outbounds[0])
	}
	endpoint := doc["endpoints"].([]any)[0].(map[string]any)
	if endpoint["tcp_fast_open"] != true {
		t.Fatalf("unexpected wireguard endpoint: %#v", endpoint)
	}
}

func TestRenderSingBoxUDPRelayMapsDefaultsAndDisable(t *testing.T) {
	r := singbox.NewRenderer()
	udpOn := true
	udpOff := false
	out, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{
		{
			Name:     "ss-on",
			Type:     domain.NodeTypeShadowsocks,
			Server:   "example.com",
			Port:     8388,
			Cipher:   "aes-128-gcm",
			Password: "secret",
			Dialer:   &domain.DialerOptions{UDPRelay: &udpOn},
		},
		{
			Name:     "ss-off",
			Type:     domain.NodeTypeShadowsocks,
			Server:   "example.org",
			Port:     8388,
			Cipher:   "aes-128-gcm",
			Password: "secret",
			Dialer:   &domain.DialerOptions{UDPRelay: &udpOff},
		},
	}, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unexpected udp relay warning: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	outbounds := doc["outbounds"].([]any)
	udpOnOutbound := outbounds[0].(map[string]any)
	if udpOnOutbound["network"] != nil {
		t.Fatalf("udp relay true must not be rendered as sing-box network: %#v", udpOnOutbound)
	}
	udpOffOutbound := outbounds[1].(map[string]any)
	if udpOffOutbound["network"] != "tcp" {
		t.Fatalf("udp relay false should disable UDP with network tcp: %#v", udpOffOutbound)
	}
}

func TestRenderSingBoxSSPluginAndUDPOverTCP(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:          "ss",
		Type:          domain.NodeTypeShadowsocks,
		Server:        "example.com",
		Port:          8388,
		Cipher:        "aes-128-gcm",
		Password:      "secret",
		Plugin:        "obfs-local",
		PluginOptions: map[string]any{"raw": "mode=tls"},
		UDPOverTCP:    &domain.UDPOverTCPOptions{Enabled: true, Version: 2},
		Multiplex:     &domain.MultiplexOptions{Enabled: true, MaxStreams: 8, Padding: true},
	}}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if report.SuccessCount != 1 {
		t.Fatalf("report: %#v", report)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	ob := doc["outbounds"].([]any)[0].(map[string]any)
	if ob["plugin"] != "obfs-local" {
		t.Fatalf("unexpected plugin: %#v", ob["plugin"])
	}
	if ob["plugin_opts"] != "obfs=tls" {
		t.Fatalf("unexpected plugin_opts: %#v", ob["plugin_opts"])
	}
	udp := ob["udp_over_tcp"].(map[string]any)
	if udp["version"] != float64(2) {
		t.Fatalf("unexpected udp_over_tcp: %#v", udp)
	}
}

func TestRenderSingBoxSSSIP002SimpleObfsPlugin(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:          "ss-plugin",
		Type:          domain.NodeTypeShadowsocks,
		Server:        "example.com",
		Port:          8388,
		Cipher:        "aes-256-gcm",
		Password:      "p@ss",
		Plugin:        "obfs",
		PluginOptions: map[string]any{"raw": "obfs=http;obfs-host=cdn.example.com"},
	}}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	ob := doc["outbounds"].([]any)[0].(map[string]any)
	if ob["plugin"] != "obfs-local" {
		t.Fatalf("unexpected plugin: %#v", ob)
	}
	if ob["plugin_opts"] != "obfs=http;obfs-host=cdn.example.com" {
		t.Fatalf("unexpected plugin_opts: %#v", ob)
	}
}

func TestRenderSingBoxVLESSLossyEncryption(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:       "vless",
		Type:       domain.NodeTypeVLESS,
		Server:     "example.com",
		Port:       443,
		UUID:       "11111111-1111-1111-1111-111111111111",
		Encryption: "custom",
	}}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) == 0 {
		t.Fatalf("expected lossy warning: %#v", report)
	}
	if !strings.Contains(string(out), "vless") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRenderSingBoxLossWarningsCoveredByCapability(t *testing.T) {
	r := singbox.NewRenderer()
	_, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{
		{
			Name:       "vless",
			Type:       domain.NodeTypeVLESS,
			Server:     "example.com",
			Port:       443,
			UUID:       "11111111-1111-1111-1111-111111111111",
			Encryption: "custom",
			TLS:        &domain.TLSOptions{Enabled: true, Fingerprint: "cert-pin"},
		},
		{
			Name:     "tuic",
			Type:     domain.NodeTypeTUIC,
			Server:   "example.com",
			Port:     443,
			Token:    "legacy-token",
			Password: "secret",
			TUIC: &domain.TUICOptions{
				ReduceRTT:            true,
				UDPOverStreamVersion: 2,
			},
		},
	}, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	lossy := capabilityLossyFields(singbox.NewRenderer().RenderCapabilities()[0])
	for _, warning := range report.Warnings {
		if warning.Code != "render_lossy_field" {
			continue
		}
		if !lossy[warning.Field] {
			t.Fatalf("warning field %q not declared in capability lossy list: %#v", warning.Field, report.Warnings)
		}
	}
}

func TestRenderSingBoxTLSFingerprintSplit(t *testing.T) {
	r := singbox.NewRenderer()
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
			TLS:        &domain.TLSOptions{Enabled: true, Fingerprint: "cert-pin"},
		},
	}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Field != "tls.fingerprint" {
		t.Fatalf("expected certificate fingerprint warning: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	outbounds := doc["outbounds"].([]any)
	clientTLS := outbounds[0].(map[string]any)["tls"].(map[string]any)
	utls := clientTLS["utls"].(map[string]any)
	if utls["fingerprint"] != "chrome" {
		t.Fatalf("unexpected utls: %#v", utls)
	}
	certTLS := outbounds[1].(map[string]any)["tls"].(map[string]any)
	if certTLS["utls"] != nil {
		t.Fatalf("unexpected utls for certificate fingerprint: %#v", certTLS)
	}
	if certTLS["certificate_public_key_sha256"] != nil {
		t.Fatalf("unexpected certificate pinning render: %#v", certTLS)
	}
}

func TestRenderSingBoxHysteriaQUIC(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:   "hy",
		Type:   domain.NodeTypeHysteria,
		Server: "example.com",
		Port:   8443,
		Hysteria: &domain.HysteriaOptions{
			AuthString: "secret",
			QUIC:       map[string]any{"init_stream_receive_window": 8388608},
		},
	}}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(out), "init_stream_receive_window") {
		t.Fatalf("unexpected quic field: %s", out)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Field != "hysteria.quic" {
		t.Fatalf("expected quic lossy warning: %#v", report.Warnings)
	}
}

func TestRenderSingBoxHysteriaWarnsWhenProtocolCannotBeRepresented(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:   "hy",
		Type:   domain.NodeTypeHysteria,
		Server: "example.com",
		Port:   8443,
		Hysteria: &domain.HysteriaOptions{
			Protocol:   "wechat-video",
			AuthString: "secret",
		},
	}}

	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})

	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(out), "wechat-video") {
		t.Fatalf("unexpected hysteria protocol field: %s", out)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Field != "hysteria.protocol" {
		t.Fatalf("expected protocol lossy warning: %#v", report.Warnings)
	}
}

func TestRenderSingBoxWireGuardReserved(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{{
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
				Reserved:     []uint8{1, 2, 3},
			}},
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	ep := doc["endpoints"].([]any)[0].(map[string]any)
	peers := ep["peers"].([]any)
	if peers[0].(map[string]any)["reserved"] == nil {
		t.Fatalf("missing reserved: %#v", peers[0])
	}
}

func TestRenderSingBoxRenderWrapper(t *testing.T) {
	r := singbox.NewRenderer()
	out, err := r.Render(context.Background(), allProtocolNodes()[:1], domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("empty output")
	}
}

func TestRenderSingBoxHTTPTransport(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:   "http",
		Type:   domain.NodeTypeVMess,
		Server: "example.com",
		Port:   443,
		UUID:   "11111111-1111-1111-1111-111111111111",
		Transport: &domain.TransportOptions{
			Type:    "http",
			Method:  "GET",
			Path:    "/api",
			Hosts:   []string{"h1.example.com"},
			Headers: map[string]string{"User-Agent": "curl"},
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	ob := doc["outbounds"].([]any)[0].(map[string]any)
	if ob["transport"] == nil {
		t.Fatalf("missing transport: %#v", ob)
	}
}

func TestRenderSingBoxAllProtocols(t *testing.T) {
	r := singbox.NewRenderer()
	out, report, err := r.RenderWithReport(context.Background(), allProtocolNodes(), domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if report.SuccessCount != 10 {
		t.Fatalf("unexpected report: %#v", report)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(doc["outbounds"].([]any)) != 9 {
		t.Fatalf("unexpected outbounds: %#v", doc["outbounds"])
	}
	if len(doc["endpoints"].([]any)) != 1 {
		t.Fatalf("unexpected endpoints: %#v", doc["endpoints"])
	}
}

func TestRenderSingBoxSkipsBadNodeWithWarning(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:   "vless",
			Type:   domain.NodeTypeVLESS,
			Server: "vless.example.com",
			Port:   443,
			UUID:   "11111111-1111-1111-1111-111111111111",
		},
		{
			Name:  "bad-mieru",
			Type:  domain.NodeTypeMieru,
			Mieru: &domain.MieruOptions{Transport: "tcp"},
		},
	}

	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
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
	if warning.Node != "bad-mieru" || warning.Target != "sing-box-outbounds" {
		t.Fatalf("unexpected skipped warning identity: %#v", warning)
	}
	if warning.NodeContext == nil || warning.NodeContext.Type != domain.NodeTypeMieru {
		t.Fatalf("missing skipped warning context: %#v", warning)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	outbounds := doc["outbounds"].([]any)
	if len(outbounds) != 1 || outbounds[0].(map[string]any)["tag"] != "vless" {
		t.Fatalf("unexpected outbounds: %#v", outbounds)
	}
}

func TestRenderSingBoxTLSFullOptions(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:   "vless",
		Type:   domain.NodeTypeVLESS,
		Server: "example.com",
		Port:   443,
		UUID:   "11111111-1111-1111-1111-111111111111",
		TLS: &domain.TLSOptions{
			Enabled:            true,
			DisableSNI:         true,
			ServerName:         "sni.example.com",
			InsecureSkipVerify: true,
			ALPN:               []string{"h2", "http/1.1"},
			ClientFingerprint:  "chrome",
			ECH: &domain.ECHOptions{
				Enabled:         true,
				Config:          []string{"ech-config"},
				QueryServerName: "ech.example.com",
			},
			Reality: &domain.RealityOptions{PublicKey: "public", ShortID: "abcd"},
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	tls := doc["outbounds"].([]any)[0].(map[string]any)["tls"].(map[string]any)
	if tls["enabled"] != true || tls["disable_sni"] != true || tls["server_name"] != "sni.example.com" || tls["insecure"] != true {
		t.Fatalf("unexpected tls: %#v", tls)
	}
	if tls["alpn"].([]any)[0] != "h2" {
		t.Fatalf("unexpected alpn: %#v", tls["alpn"])
	}
	if tls["utls"].(map[string]any)["fingerprint"] != "chrome" {
		t.Fatalf("unexpected utls: %#v", tls["utls"])
	}
	ech := tls["ech"].(map[string]any)
	if ech["enabled"] != true || ech["query_server_name"] != "ech.example.com" {
		t.Fatalf("unexpected ech: %#v", ech)
	}
	reality := tls["reality"].(map[string]any)
	if reality["public_key"] != "public" || reality["short_id"] != "abcd" {
		t.Fatalf("unexpected reality: %#v", reality)
	}
}

func TestRenderSingBoxTransportVariantsAndPluginOptions(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:          "ss",
			Type:          domain.NodeTypeShadowsocks,
			Server:        "example.com",
			Port:          8388,
			Cipher:        "aes-128-gcm",
			Password:      "secret",
			Plugin:        "simple-obfs",
			PluginOptions: map[string]any{"mode": "tls", "host": "cdn.example.com"},
			UDPOverTCP:    &domain.UDPOverTCPOptions{Enabled: true},
		},
		{
			Name:   "ws",
			Type:   domain.NodeTypeVMess,
			Server: "example.com",
			Port:   443,
			UUID:   "11111111-1111-1111-1111-111111111111",
			Transport: &domain.TransportOptions{
				Type:                "websocket",
				Path:                "/ws",
				Host:                "cdn.example.com",
				MaxEarlyData:        2048,
				EarlyDataHeaderName: "Sec-WebSocket-Protocol",
			},
		},
		{
			Name:   "ws-upgrade",
			Type:   domain.NodeTypeVLESS,
			Server: "example.com",
			Port:   443,
			UUID:   "11111111-1111-1111-1111-111111111114",
			Transport: &domain.TransportOptions{
				Type:                     "websocket",
				Path:                     "/upgrade",
				Host:                     "upgrade.example.com",
				Headers:                  map[string]string{"Host": "upgrade.example.com", "X-Test": "yes"},
				V2RayHTTPUpgrade:         true,
				V2RayHTTPUpgradeFastOpen: true,
				MaxEarlyData:             2048,
				EarlyDataHeaderName:      "Sec-WebSocket-Protocol",
			},
		},
		{
			Name:   "httpupgrade",
			Type:   domain.NodeTypeVLESS,
			Server: "example.com",
			Port:   443,
			UUID:   "11111111-1111-1111-1111-111111111112",
			Transport: &domain.TransportOptions{
				Type:    "httpupgrade",
				Path:    "/upgrade",
				Host:    "upgrade.example.com",
				Headers: map[string]string{"X-Test": "yes"},
			},
		},
		{
			Name:   "tcp",
			Type:   domain.NodeTypeVMess,
			Server: "example.com",
			Port:   443,
			UUID:   "11111111-1111-1111-1111-111111111113",
			Transport: &domain.TransportOptions{
				Type: "tcp",
			},
		},
		{
			Name:      "custom",
			Type:      domain.NodeTypeTrojan,
			Server:    "example.com",
			Port:      443,
			Password:  "secret",
			Transport: &domain.TransportOptions{Type: "splithttp"},
		},
	}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Field != "transport.type" {
		t.Fatalf("expected unsupported transport warning: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	outbounds := doc["outbounds"].([]any)
	ss := outbounds[0].(map[string]any)
	if ss["plugin"] != "obfs-local" || ss["plugin_opts"] != "obfs=tls;obfs-host=cdn.example.com" || ss["udp_over_tcp"] != true {
		t.Fatalf("unexpected ss outbound: %#v", ss)
	}
	ws := outbounds[1].(map[string]any)["transport"].(map[string]any)
	if ws["type"] != "ws" || ws["max_early_data"] != float64(2048) || ws["early_data_header_name"] != "Sec-WebSocket-Protocol" {
		t.Fatalf("unexpected websocket transport: %#v", ws)
	}
	if ws["headers"].(map[string]any)["Host"] != "cdn.example.com" {
		t.Fatalf("unexpected websocket headers: %#v", ws["headers"])
	}
	wsUpgrade := outbounds[2].(map[string]any)["transport"].(map[string]any)
	if wsUpgrade["type"] != "httpupgrade" || wsUpgrade["host"] != "upgrade.example.com" || wsUpgrade["path"] != "/upgrade" {
		t.Fatalf("unexpected websocket upgrade transport: %#v", wsUpgrade)
	}
	wsUpgradeHeaders := wsUpgrade["headers"].(map[string]any)
	if wsUpgradeHeaders["Host"] != nil || wsUpgradeHeaders["X-Test"] != "yes" {
		t.Fatalf("unexpected websocket upgrade headers: %#v", wsUpgradeHeaders)
	}
	if wsUpgrade["max_early_data"] != nil || wsUpgrade["early_data_header_name"] != nil {
		t.Fatalf("httpupgrade transport should not include websocket early data: %#v", wsUpgrade)
	}
	httpupgrade := outbounds[3].(map[string]any)["transport"].(map[string]any)
	if httpupgrade["type"] != "httpupgrade" || httpupgrade["host"] != "upgrade.example.com" || httpupgrade["path"] != "/upgrade" {
		t.Fatalf("unexpected httpupgrade transport: %#v", httpupgrade)
	}
	tcp := outbounds[4].(map[string]any)
	if tcp["transport"] != nil {
		t.Fatalf("unexpected default tcp transport: %#v", tcp)
	}
	custom := outbounds[5].(map[string]any)
	if custom["transport"] != nil {
		t.Fatalf("unexpected unsupported custom transport: %#v", custom)
	}
}

func TestRenderSingBoxWireGuardAddressAndPeerFallbacks(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{{
		Name: "wg",
		Type: domain.NodeTypeWireGuard,
		WireGuard: &domain.WireGuardOptions{
			PrivateKey:          "private",
			IP:                  "10.0.0.2/32",
			IPv6:                "fd00::2/128",
			Reserved:            []uint8{1, 2, 3},
			PersistentKeepalive: 25,
			Peers: []domain.WireGuardPeer{{
				Server:    "wg.example.com",
				Port:      51820,
				PublicKey: "public",
			}},
		},
	}}
	out, _, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	endpoint := doc["endpoints"].([]any)[0].(map[string]any)
	addresses := endpoint["address"].([]any)
	if addresses[0] != "10.0.0.2/32" || addresses[1] != "fd00::2/128" {
		t.Fatalf("unexpected addresses: %#v", addresses)
	}
	peer := endpoint["peers"].([]any)[0].(map[string]any)
	if peer["persistent_keepalive_interval"] != float64(25) || peer["reserved"] == nil {
		t.Fatalf("unexpected peer fallback fields: %#v", peer)
	}
}

func TestRenderSingBoxHysteria2RealmAndLossyRates(t *testing.T) {
	r := singbox.NewRenderer()
	nodes := []domain.NodeIR{{
		Name:     "hy2",
		Type:     domain.NodeTypeHysteria2,
		Server:   "example.com",
		Port:     8443,
		Password: "secret",
		Hysteria: &domain.HysteriaOptions{
			Up:   "20 Mbps",
			Down: "100 Mbps",
			Realm: &domain.HysteriaRealmOptions{
				ServerURL:   "https://realm.example.com",
				Token:       "token",
				RealmID:     "realm-id",
				STUNServers: []string{"stun.example.com"},
			},
		},
	}}
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{Format: "sing-box-outbounds"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(report.Warnings) != 3 {
		t.Fatalf("expected lossy rate and realm warnings: %#v", report.Warnings)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	if doc["outbounds"].([]any)[0].(map[string]any)["realm"] != nil {
		t.Fatalf("unexpected realm render: %#v", doc["outbounds"])
	}
}

func capabilityLossyFields(capability shared.Capability) map[string]bool {
	out := map[string]bool{}
	for _, field := range capability.Lossy {
		out[field.IRField] = true
	}
	return out
}
