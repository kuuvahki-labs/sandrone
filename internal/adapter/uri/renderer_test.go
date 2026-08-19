package uri_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func allURIRenderNodes() []domain.NodeIR {
	return []domain.NodeIR{
		{
			Name:     "ss",
			Type:     domain.NodeTypeShadowsocks,
			Server:   "example.com",
			Port:     8388,
			Cipher:   "aes-128-gcm",
			Password: "secret",
		},
		{
			Name:   "vmess-ws",
			Type:   domain.NodeTypeVMess,
			Server: "example.com",
			Port:   443,
			UUID:   "11111111-1111-1111-1111-111111111111",
			Cipher: "auto",
			Transport: &domain.TransportOptions{
				Type: "websocket",
				Path: "/ws",
				Host: "cdn.example.com",
			},
			TLS: &domain.TLSOptions{Enabled: true, ServerName: "example.com"},
		},
		{
			Name:       "vless",
			Type:       domain.NodeTypeVLESS,
			Server:     "example.com",
			Port:       443,
			UUID:       "11111111-1111-1111-1111-111111111112",
			Encryption: "none",
			TLS:        &domain.TLSOptions{Enabled: true, ServerName: "example.com"},
			Transport:  &domain.TransportOptions{Type: "grpc", ServiceName: "svc"},
		},
		{
			Name:     "trojan",
			Type:     domain.NodeTypeTrojan,
			Server:   "example.com",
			Port:     443,
			Password: "secret",
			TLS:      &domain.TLSOptions{Enabled: true, ServerName: "example.com"},
		},
		{
			Name:   "hysteria",
			Type:   domain.NodeTypeHysteria,
			Server: "example.com",
			Port:   8443,
			TLS:    &domain.TLSOptions{Enabled: true, ServerName: "example.com"},
			Hysteria: &domain.HysteriaOptions{
				AuthString:   "secret",
				Up:           "20Mbps",
				Down:         "100Mbps",
				Obfs:         "xplus",
				ObfsPassword: "obfs",
			},
		},
		{
			Name:     "hysteria2",
			Type:     domain.NodeTypeHysteria2,
			Server:   "example.com",
			Port:     8443,
			Password: "secret",
			Hysteria: &domain.HysteriaOptions{
				Obfs:         "salamander",
				ObfsPassword: "obfs",
			},
			TLS: &domain.TLSOptions{Enabled: true, ServerName: "example.com"},
		},
		{
			Name:     "tuic",
			Type:     domain.NodeTypeTUIC,
			Server:   "example.com",
			Port:     443,
			UUID:     "11111111-1111-1111-1111-111111111113",
			Password: "secret",
			TUIC:     &domain.TUICOptions{CongestionControl: "bbr", UDPRelayMode: "native"},
			TLS:      &domain.TLSOptions{Enabled: true, ServerName: "example.com"},
		},
		{
			Name:     "socks",
			Type:     domain.NodeTypeSOCKS,
			Server:   "example.com",
			Port:     1080,
			Username: "user",
			Password: "pass",
		},
		{
			Name:     "http",
			Type:     domain.NodeTypeHTTP,
			Server:   "example.com",
			Port:     8443,
			Username: "user",
			Password: "pass",
			TLS:      &domain.TLSOptions{Enabled: true},
		},
	}
}

func TestRenderHysteriaURIFromMihomoInfersXPlusModeForObfsPassword(t *testing.T) {
	nodes, _, err := mihomo.NewParser().Parse(context.Background(), []byte(`
proxies:
  - name: hy
    type: hysteria
    server: hy.example.com
    port: 8443
    protocol: faketcp
    auth-str: secret
    obfs: obfs-pass
`))
	require.NoError(t, err)

	out, report, err := uri.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	parsed, err := url.Parse(strings.TrimSpace(string(out)))
	require.NoError(t, err)
	require.Equal(t, "faketcp", parsed.Query().Get("protocol"))
	require.Equal(t, "xplus", parsed.Query().Get("obfs"))
	require.Equal(t, "obfs-pass", parsed.Query().Get("obfsParam"))
}

func TestRenderURIAllProtocols(t *testing.T) {
	r := uri.NewRenderer()
	nodes := allURIRenderNodes()
	out, report, err := r.RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, len(nodes), report.SuccessCount)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	require.Len(t, lines, len(nodes))
	require.Contains(t, lines[0], "ss://")
	require.Contains(t, lines[1], "vmess://")
	require.Contains(t, lines[2], "vless://")
	require.Contains(t, lines[3], "trojan://")
	require.Contains(t, lines[4], "hysteria://")
	require.Contains(t, lines[5], "hy2://")
	require.Contains(t, lines[6], "tuic://")
	require.Contains(t, lines[7], "socks5://")
	require.Contains(t, lines[8], "https://")
}

func TestRenderURIMissingFields(t *testing.T) {
	r := uri.NewRenderer()
	tests := []struct {
		name string
		node domain.NodeIR
	}{
		{
			name: "ss",
			node: domain.NodeIR{Type: domain.NodeTypeShadowsocks, Name: "x"},
		},
		{
			name: "vmess",
			node: domain.NodeIR{Type: domain.NodeTypeVMess, Name: "x", Server: "a.com"},
		},
		{
			name: "vless",
			node: domain.NodeIR{Type: domain.NodeTypeVLESS, Name: "x"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.Render(context.Background(), []domain.NodeIR{tc.node}, domain.RenderOptions{})
			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeRenderFailed))
		})
	}
}

func TestRenderURISkipsBadNodeWithWarning(t *testing.T) {
	r := uri.NewRenderer()
	out, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{
		{
			Name:     "trojan",
			Type:     domain.NodeTypeTrojan,
			Server:   "example.com",
			Port:     443,
			Password: "secret",
		},
		{
			Name:     "bad-mieru",
			Type:     domain.NodeTypeMieru,
			Username: "user",
			Password: "pass",
			Mieru:    &domain.MieruOptions{Transport: "tcp"},
		},
	}, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Equal(t, "trojan://secret@example.com:443#trojan", strings.TrimSpace(string(out)))
	require.Len(t, report.Warnings, 1)
	warning := report.Warnings[0]
	require.Equal(t, "render_node_skipped", warning.Code)
	require.Equal(t, "bad-mieru", warning.Node)
	require.Equal(t, "uri-list", warning.Target)
	require.NotNil(t, warning.NodeContext)
	require.Equal(t, "bad-mieru", warning.NodeContext.Name)
	require.Equal(t, domain.NodeTypeMieru, warning.NodeContext.Type)
}

func TestRenderParseShadowsocksRRoundtrip(t *testing.T) {
	p := uri.NewParser()
	r := uri.NewRenderer()
	node := domain.NodeIR{
		Name:     "ssr-node",
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
	}

	rendered, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Contains(t, string(rendered), "ssr://")
	reparsed, _, err := p.Parse(context.Background(), rendered)
	require.NoError(t, err)
	require.Len(t, reparsed, 1)
	require.Equal(t, node.Name, reparsed[0].Name)
	require.Equal(t, node.Server, reparsed[0].Server)
	require.Equal(t, node.Cipher, reparsed[0].Cipher)
	require.Equal(t, node.ShadowsocksR.ProtocolParam, reparsed[0].ShadowsocksR.ProtocolParam)
	require.Equal(t, node.ShadowsocksR.ObfsParam, reparsed[0].ShadowsocksR.ObfsParam)
}

func TestRenderURIUnsupportedType(t *testing.T) {
	r := uri.NewRenderer()
	out, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{{
		Name:   "wg",
		Type:   domain.NodeTypeWireGuard,
		Server: "example.com",
		Port:   51820,
	}}, domain.RenderOptions{})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeRenderFailed), "unexpected error: %v", err)
	require.Empty(t, strings.TrimSpace(string(out)))
	require.Equal(t, 0, report.SuccessCount)
	require.NotEmpty(t, report.Warnings)
	require.Equal(t, "render_node_skipped", report.Warnings[0].Code)
}

func TestRenderURIStructuredLossWarnings(t *testing.T) {
	r := uri.NewRenderer()
	out, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{{
		Name:     "tuic-loss",
		Type:     domain.NodeTypeTUIC,
		Server:   "example.com",
		Port:     443,
		UUID:     "11111111-1111-1111-1111-111111111111",
		Password: "secret",
		TUIC: &domain.TUICOptions{
			ReduceRTT:            true,
			Heartbeat:            "10s",
			UDPOverStream:        true,
			UDPOverStreamVersion: 2,
		},
	}}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Contains(t, string(out), "tuic://")
	require.Equal(t, 1, report.SuccessCount)
	require.Equal(t, 4, report.LostFields)
	require.ElementsMatch(t, []string{
		"tuic.reduce_rtt",
		"tuic.heartbeat",
		"tuic.udp_over_stream",
		"tuic.udp_over_stream_version",
	}, warningFields(report.Warnings))
	lossy := capabilityLossyFields(uri.NewRenderer().RenderCapabilities()[0])
	for _, warning := range report.Warnings {
		require.True(t, lossy[warning.Field], "warning field %q not declared in capability lossy list", warning.Field)
	}
}

func TestRenderURIParseRoundtrip(t *testing.T) {
	p := uri.NewParser()
	r := uri.NewRenderer()
	raw := "ss://aes-128-gcm:secret@example.com:8388#node-a"
	parsed, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	rendered, err := r.Render(context.Background(), parsed, domain.RenderOptions{})
	require.NoError(t, err)
	reparsed, _, err := p.Parse(context.Background(), rendered)
	require.NoError(t, err)
	require.Equal(t, parsed[0].Cipher, reparsed[0].Cipher)
	require.Equal(t, parsed[0].Server, reparsed[0].Server)
	require.Equal(t, parsed[0].Port, reparsed[0].Port)
}

func TestRenderParseVMessGRPCRoundtrip(t *testing.T) {
	p := uri.NewParser()
	r := uri.NewRenderer()
	node := domain.NodeIR{
		Name:   "vmess-grpc",
		Type:   domain.NodeTypeVMess,
		Server: "example.com",
		Port:   443,
		UUID:   "11111111-1111-1111-1111-111111111111",
		Cipher: "auto",
		Transport: &domain.TransportOptions{
			Type:        "grpc",
			ServiceName: "svc",
		},
	}

	rendered, err := r.Render(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	reparsed, _, err := p.Parse(context.Background(), rendered)
	require.NoError(t, err)
	require.NotNil(t, reparsed[0].Transport)
	require.Equal(t, "grpc", reparsed[0].Transport.Type)
	require.Equal(t, "svc", reparsed[0].Transport.ServiceName)
	require.Empty(t, reparsed[0].Transport.Path)
}

func TestRenderParsedVMessAEADStillUsesLegacyBase64JSON(t *testing.T) {
	p := uri.NewParser()
	nodes, _, err := p.Parse(context.Background(), []byte(
		"vmess://11111111-1111-1111-1111-111111111111@example.com:443?encryption=zero#vmess-aead",
	))
	require.NoError(t, err)

	body, _, err := uri.NewRenderer().RenderWithReport(
		context.Background(), nodes, domain.RenderOptions{},
	)
	require.NoError(t, err)
	legacyJSON := `{"add":"example.com","aid":"0","id":"11111111-1111-1111-1111-111111111111","net":"tcp","port":"443","ps":"vmess-aead","scy":"zero","type":"none","v":"2"}`
	require.Equal(t, "vmess://"+base64.StdEncoding.EncodeToString([]byte(legacyJSON)), string(body))
}

func TestRenderURIVLESSXHTTPRealityRoundtrip(t *testing.T) {
	p := uri.NewParser()
	r := uri.NewRenderer()
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=reality&pbk=public-key&sid=08&type=xhttp&path=%2Fxhttp&host=cdn.example.com&mode=packet-up&packet-encoding=xudp&udp=true&sni=sni.example.com&fp=chrome&alpn=h2,http/1.1#vless-xhttp"

	parsed, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	rendered, report, err := r.RenderWithReport(context.Background(), parsed, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.NotContains(t, warningFields(report.Warnings), "uri.query.mode")
	require.Contains(t, warningFields(report.Warnings), "uri.query.udp")
	require.NotContains(t, warningFields(report.Warnings), "packet_encoding")

	reparsed, _, err := p.Parse(context.Background(), rendered)
	require.NoError(t, err)
	got := reparsed[0]
	require.Equal(t, domain.NodeTypeVLESS, got.Type)
	require.Equal(t, "xudp", got.PacketEncoding)
	require.NotNil(t, got.TLS)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.Equal(t, "chrome", got.TLS.ClientFingerprint)
	require.Equal(t, []string{"h2", "http/1.1"}, got.TLS.ALPN)
	require.NotNil(t, got.TLS.Reality)
	require.Equal(t, "public-key", got.TLS.Reality.PublicKey)
	require.Equal(t, "08", got.TLS.Reality.ShortID)
	require.NotNil(t, got.Transport)
	require.Equal(t, "xhttp", got.Transport.Type)
	require.Equal(t, "/xhttp", got.Transport.Path)
	require.Equal(t, "cdn.example.com", got.Transport.Host)
	require.NotNil(t, got.Transport.XHTTP)
	require.Equal(t, "packet-up", got.Transport.XHTTP.Mode)
}

func TestParsedSplitHTTPRendersAsCanonicalMihomoXHTTP(t *testing.T) {
	parsed, source, err := uri.NewParser().Parse(context.Background(), []byte(
		"vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&type=splithttp&path=%2Fupload&host=cdn.example.com&mode=packet-up#split-http",
	))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)

	body, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), parsed, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Contains(t, string(body), "network: xhttp")
	require.Contains(t, string(body), "xhttp-opts:")
	require.Contains(t, string(body), "mode: packet-up")
	require.Contains(t, string(body), "path: /upload")
}

func TestParsedTrojanPresenceFlagsRenderToMihomoAndSingBox(t *testing.T) {
	parsed, source, err := uri.NewParser().Parse(context.Background(), []byte(
		"trojan://secret@example.com:443?udp&tfo&ws&wspath=%2Fws#trojan",
	))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)

	mihomoBody, mihomoReport, err := mihomo.NewRenderer().RenderWithReport(context.Background(), parsed, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, mihomoReport.SuccessCount)
	require.Contains(t, string(mihomoBody), "network: ws")
	require.Contains(t, string(mihomoBody), "udp: true")
	require.Contains(t, string(mihomoBody), "tfo: true")
	require.Contains(t, string(mihomoBody), "path: /ws")

	singBoxBody, singBoxReport, err := singbox.NewRenderer().RenderWithReport(context.Background(), parsed, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, singBoxReport.SuccessCount)
	var document struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(singBoxBody, &document))
	require.Len(t, document.Outbounds, 1)
	require.Equal(t, true, document.Outbounds[0]["tcp_fast_open"])
	transport, ok := document.Outbounds[0]["transport"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ws", transport["type"])
	require.Equal(t, "/ws", transport["path"])
}

func TestParsedLegacyVMessCombinedHostPathRendersAcrossTargets(t *testing.T) {
	doc := `{"v":"1","ps":"legacy-v1","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","net":"ws","host":"cdn.example.com;/socket"}`
	raw := "vmess://" + base64.RawStdEncoding.EncodeToString([]byte(doc))
	parsed, source, err := uri.NewParser().Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)

	mihomoBody, mihomoReport, err := mihomo.NewRenderer().RenderWithReport(context.Background(), parsed, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, mihomoReport.SuccessCount)
	require.Contains(t, string(mihomoBody), "Host: cdn.example.com")
	require.Contains(t, string(mihomoBody), "path: /socket")

	singBoxBody, singBoxReport, err := singbox.NewRenderer().RenderWithReport(context.Background(), parsed, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, singBoxReport.SuccessCount)
	require.Contains(t, string(singBoxBody), `"path": "/socket"`)
	require.Contains(t, string(singBoxBody), `"Host": "cdn.example.com"`)
}

func TestRenderURIRawQueryWarnings(t *testing.T) {
	p := uri.NewParser()
	r := uri.NewRenderer()
	parsed, _, err := p.Parse(context.Background(), []byte("vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&ech=ech-config&unknown-param=value#vless"))
	require.NoError(t, err)

	_, report, err := r.RenderWithReport(context.Background(), parsed, domain.RenderOptions{})
	require.NoError(t, err)
	require.NotContains(t, warningFields(report.Warnings), "uri.query.ech")
	require.Contains(t, warningFields(report.Warnings), "uri.query.unknown-param")
}

func TestRenderURIRenderDelegatesToReport(t *testing.T) {
	r := uri.NewRenderer()
	out, err := r.Render(context.Background(), allURIRenderNodes()[:1], domain.RenderOptions{})
	require.NoError(t, err)
	require.Contains(t, string(out), "ss://")
}

func TestRenderURIVLESSWithReality(t *testing.T) {
	r := uri.NewRenderer()
	out, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{{
		Name:       "vless",
		Type:       domain.NodeTypeVLESS,
		Server:     "example.com",
		Port:       443,
		UUID:       "11111111-1111-1111-1111-111111111111",
		Encryption: "none",
		TLS: &domain.TLSOptions{
			Reality: &domain.RealityOptions{PublicKey: "pk", ShortID: "ab"},
		},
		Transport: &domain.TransportOptions{Type: "http", Path: "/h2", Host: "cdn.example.com"},
	}}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Contains(t, string(out), "security=reality")
	require.Contains(t, string(out), "type=h2")
}

func TestRenderURIVMessTCPHTTPHeader(t *testing.T) {
	r := uri.NewRenderer()
	node := domain.NodeIR{
		Name:   "vmess-http-header",
		Type:   domain.NodeTypeVMess,
		Server: "example.com",
		Port:   443,
		UUID:   "11111111-1111-1111-1111-111111111111",
		Transport: &domain.TransportOptions{
			Type:       "tcp",
			HeaderType: "http",
			Method:     "GET",
			Path:       "/api",
			Host:       "cdn.example.com",
			Headers:    map[string]string{"Host": "cdn.example.com"},
		},
	}

	out, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Empty(t, report.Warnings)

	parsed, _, err := uri.NewParser().Parse(context.Background(), out)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	require.Equal(t, node.Transport, parsed[0].Transport)
}

func TestRenderURIReportsCanonicalNetworkAsLossyForV2RayProtocols(t *testing.T) {
	nodes := []domain.NodeIR{
		{
			Name: "vmess", Type: domain.NodeTypeVMess, Server: "example.com", Port: 443,
			UUID: "11111111-1111-1111-1111-111111111111", Network: "tcp",
		},
		{
			Name: "vless", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
			UUID: "11111111-1111-1111-1111-111111111112", Encryption: "none", Network: "udp",
		},
		{
			Name: "trojan", Type: domain.NodeTypeTrojan, Server: "example.com", Port: 443,
			Password: "secret", Network: "tcp", TLS: &domain.TLSOptions{Enabled: true},
		},
	}

	out, report, err := uri.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Len(t, strings.Split(string(out), "\n"), 3)
	require.Len(t, report.Warnings, 3)
	for _, warning := range report.Warnings {
		require.Equal(t, "network", warning.Field)
		require.Equal(t, "render_lossy_field", warning.Code)
	}
}

func TestRenderURIWithPlugin(t *testing.T) {
	r := uri.NewRenderer()
	out, err := r.Render(context.Background(), []domain.NodeIR{{
		Name:     "ss-plugin",
		Type:     domain.NodeTypeShadowsocks,
		Server:   "example.com",
		Port:     8388,
		Cipher:   "aes-128-gcm",
		Password: "secret",
		Plugin:   "obfs-local",
	}}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Contains(t, string(out), "plugin=obfs-local")
}

func TestRenderURIShadowsocksPluginUsesSIP002Argument(t *testing.T) {
	r := uri.NewRenderer()
	out, err := r.Render(context.Background(), []domain.NodeIR{{
		Name:          "ss-plugin",
		Type:          domain.NodeTypeShadowsocks,
		Server:        "example.com",
		Port:          8388,
		Cipher:        "aes-128-gcm",
		Password:      "secret",
		Plugin:        "obfs-local",
		PluginOptions: map[string]any{"raw": "obfs=http;obfs-host=cdn.example.com"},
	}}, domain.RenderOptions{})
	require.NoError(t, err)
	parsed, err := url.Parse(strings.TrimSpace(string(out)))
	require.NoError(t, err)
	require.Equal(t, "obfs-local;obfs=http;obfs-host=cdn.example.com", parsed.Query().Get("plugin"))
}

func TestRenderURIHysteriaOfficialFields(t *testing.T) {
	r := uri.NewRenderer()
	out, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{{
		Name:   "hy",
		Type:   domain.NodeTypeHysteria,
		Server: "example.com",
		Port:   8443,
		TLS: &domain.TLSOptions{
			ServerName:         "sni.example.com",
			InsecureSkipVerify: true,
			ALPN:               []string{"hysteria"},
		},
		Hysteria: &domain.HysteriaOptions{
			Auth:         "secret",
			Obfs:         "xplus",
			ObfsPassword: "obfs-pass",
			UpMbps:       20,
			DownMbps:     100,
		},
	}}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	parsed, err := url.Parse(strings.TrimSpace(string(out)))
	require.NoError(t, err)
	query := parsed.Query()
	require.Equal(t, "secret", query.Get("auth"))
	require.Equal(t, "sni.example.com", query.Get("peer"))
	require.Equal(t, "1", query.Get("insecure"))
	require.Equal(t, "20", query.Get("upmbps"))
	require.Equal(t, "100", query.Get("downmbps"))
	require.Equal(t, "xplus", query.Get("obfs"))
	require.Equal(t, "obfs-pass", query.Get("obfsParam"))
	require.Empty(t, query.Get("up"))
}

func TestRenderURIHysteriaBandwidthConvertsExactExplicitRates(t *testing.T) {
	out, report, err := uri.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{{
		Name: "hy", Type: domain.NodeTypeHysteria, Server: "example.com", Port: 8443,
		Hysteria: &domain.HysteriaOptions{Up: "125 KBps", Down: "250 KBps"},
	}}, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	parsed, err := url.Parse(strings.TrimSpace(string(out)))
	require.NoError(t, err)
	require.Equal(t, "1", parsed.Query().Get("upmbps"))
	require.Equal(t, "2", parsed.Query().Get("downmbps"))
	for _, warning := range report.Warnings {
		require.NotContains(t, []string{"hysteria.up", "hysteria.down"}, warning.Field)
	}
}

func TestRenderURIHysteriaCrossFormatBandwidth(t *testing.T) {
	t.Run("Mihomo bare rates remain Mbps", func(t *testing.T) {
		nodes, _, err := mihomo.NewParser().Parse(context.Background(), []byte(`
proxies:
  - {name: hy, type: hysteria, server: example.com, port: 8443, up: "55", down: "100", auth-str: secret}
`))
		require.NoError(t, err)

		query, report := renderHysteriaURIQuery(t, nodes)
		require.Equal(t, "55", query.Get("upmbps"))
		require.Equal(t, "100", query.Get("downmbps"))
		require.Empty(t, hysteriaBandwidthWarningFields(report))
	})

	t.Run("sing-box numeric Bps omits inexact direction", func(t *testing.T) {
		nodes, _, err := singbox.NewParser().Parse(context.Background(), []byte(`{
			"outbounds":[{"type":"hysteria","tag":"hy","server":"example.com","server_port":8443,"auth_str":"secret","up":55,"down":125000}]
		}`))
		require.NoError(t, err)

		query, report := renderHysteriaURIQuery(t, nodes)
		require.Empty(t, query.Get("upmbps"))
		require.Equal(t, "1", query.Get("downmbps"))
		require.Equal(t, []string{"hysteria.up"}, hysteriaBandwidthWarningFields(report))
	})

	t.Run("URI Mbps remains exact", func(t *testing.T) {
		nodes, _, err := uri.NewParser().Parse(context.Background(), []byte(
			"hysteria://example.com:8443?auth_str=secret&up=55&down=100#hy",
		))
		require.NoError(t, err)

		query, report := renderHysteriaURIQuery(t, nodes)
		require.Equal(t, "55", query.Get("upmbps"))
		require.Equal(t, "100", query.Get("downmbps"))
		require.Empty(t, hysteriaBandwidthWarningFields(report))
	})
}

func TestRenderURIHysteriaBandwidthLossDoesNotDropOtherFields(t *testing.T) {
	query, report := renderHysteriaURIQuery(t, []domain.NodeIR{{
		Name: "hy", Type: domain.NodeTypeHysteria, Server: "example.com", Port: 8443,
		TLS: &domain.TLSOptions{ServerName: "sni.example.com"},
		Hysteria: &domain.HysteriaOptions{
			Protocol: "wechat-video", AuthString: "secret", Up: "55 Bps", DownMbps: 2,
		},
	}})

	require.Equal(t, "secret", query.Get("auth"))
	require.Equal(t, "wechat-video", query.Get("protocol"))
	require.Equal(t, "sni.example.com", query.Get("peer"))
	require.Empty(t, query.Get("upmbps"))
	require.Equal(t, "2", query.Get("downmbps"))
	require.Equal(t, []string{"hysteria.up"}, hysteriaBandwidthWarningFields(report))
}

func renderHysteriaURIQuery(t *testing.T, nodes []domain.NodeIR) (url.Values, domain.RenderReport) {
	t.Helper()
	out, report, err := uri.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	parsed, err := url.Parse(strings.TrimSpace(string(out)))
	require.NoError(t, err)
	return parsed.Query(), report
}

func hysteriaBandwidthWarningFields(report domain.RenderReport) []string {
	fields := []string{}
	for _, warning := range report.Warnings {
		if warning.Code == "render_lossy_field" && (warning.Field == "hysteria.up" || warning.Field == "hysteria.down") {
			fields = append(fields, warning.Field)
		}
	}
	return fields
}

func TestRenderURIHysteria2OfficialFields(t *testing.T) {
	r := uri.NewRenderer()
	out, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{{
		Name:     "hy2",
		Type:     domain.NodeTypeHysteria2,
		Server:   "example.com",
		Port:     443,
		Password: "secret",
		TLS: &domain.TLSOptions{
			ServerName:         "sni.example.com",
			InsecureSkipVerify: true,
			Fingerprint:        "pin",
		},
		Hysteria: &domain.HysteriaOptions{
			ServerPorts:  []string{"443", "8443-9000"},
			Obfs:         "salamander",
			ObfsPassword: "obfs",
		},
	}}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	body := strings.TrimSpace(string(out))
	require.Contains(t, body, "example.com:443,8443-9000")
	_, queryStr, _ := strings.Cut(body, "?")
	queryStr, _, _ = strings.Cut(queryStr, "#")
	query, err := url.ParseQuery(queryStr)
	require.NoError(t, err)
	require.Equal(t, "sni.example.com", query.Get("sni"))
	require.Equal(t, "1", query.Get("insecure"))
	require.Equal(t, "pin", query.Get("pinSHA256"))
	require.Equal(t, "salamander", query.Get("obfs"))
	require.Equal(t, "obfs", query.Get("obfs-password"))
	require.Empty(t, query.Get("fp"))
}

func TestRenderURIVLESSQueryTLSAndTransportDetails(t *testing.T) {
	r := uri.NewRenderer()
	out, report, err := r.RenderWithReport(context.Background(), []domain.NodeIR{{
		Name:       "vless detail",
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
		},
		Transport: &domain.TransportOptions{
			Type: "websocket",
			Path: "/ws",
			Host: "cdn.example.com",
		},
	}}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)

	parsed, err := url.Parse(strings.TrimSpace(string(out)))
	require.NoError(t, err)
	query := parsed.Query()
	require.Equal(t, "tls", query.Get("security"))
	require.Equal(t, "sni.example.com", query.Get("sni"))
	require.Equal(t, "true", query.Get("allowInsecure"))
	require.Equal(t, "h2,http/1.1", query.Get("alpn"))
	require.Equal(t, "chrome", query.Get("fp"))
	require.Equal(t, "ws", query.Get("type"))
	require.Equal(t, "/ws", query.Get("path"))
	require.Equal(t, "cdn.example.com", query.Get("host"))
	require.Equal(t, "vless detail", parsed.Fragment)
}

func TestRenderURIUsesTransportAliases(t *testing.T) {
	r := uri.NewRenderer()
	tests := []struct {
		name     string
		node     domain.NodeIR
		wantType string
	}{
		{
			name: "grpc",
			node: domain.NodeIR{
				Name:      "grpc",
				Type:      domain.NodeTypeTrojan,
				Server:    "example.com",
				Port:      443,
				Password:  "secret",
				Transport: &domain.TransportOptions{Type: "grpc", ServiceName: "svc"},
			},
			wantType: "grpc",
		},
		{
			name: "http",
			node: domain.NodeIR{
				Name:      "http",
				Type:      domain.NodeTypeVLESS,
				Server:    "example.com",
				Port:      443,
				UUID:      "11111111-1111-1111-1111-111111111111",
				Transport: &domain.TransportOptions{Type: "http", Path: "/h2", Host: "h2.example.com"},
			},
			wantType: "h2",
		},
		{
			name: "custom",
			node: domain.NodeIR{
				Name:      "custom",
				Type:      domain.NodeTypeVLESS,
				Server:    "example.com",
				Port:      443,
				UUID:      "11111111-1111-1111-1111-111111111112",
				Transport: &domain.TransportOptions{Type: "splithttp"},
			},
			wantType: "splithttp",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := r.Render(context.Background(), []domain.NodeIR{tc.node}, domain.RenderOptions{})
			require.NoError(t, err)
			parsed, err := url.Parse(strings.TrimSpace(string(out)))
			require.NoError(t, err)
			require.Equal(t, tc.wantType, parsed.Query().Get("type"))
		})
	}
}

func warningFields(warnings []domain.Warning) []string {
	fields := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		fields = append(fields, warning.Field)
	}
	return fields
}

func capabilityLossyFields(capability shared.Capability) map[string]bool {
	out := map[string]bool{}
	for _, field := range capability.Lossy {
		out[field.IRField] = true
	}
	return out
}
