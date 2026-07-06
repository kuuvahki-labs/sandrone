package uri_test

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestParseShadowsocksURI(t *testing.T) {
	p := uri.NewParser()
	nodes, source, err := p.Parse(context.Background(), []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, "ss", source.Format)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeShadowsocks, got.Type)
	require.Equal(t, "node-a", got.Name)
	require.Equal(t, "example.com", got.Server)
	require.Equal(t, uint16(8388), got.Port)
	require.Equal(t, "aes-128-gcm", got.Cipher)
	require.Equal(t, "secret", got.Password)
}

func TestParseShadowsocksBareIPv6HostPort(t *testing.T) {
	p := uri.NewParser()
	nodes, source, err := p.Parse(context.Background(), []byte("ss://YWVzLTEyOC1nY206c2VjcmV0@2a03:4000:38:dff:b49c:beff:fe49:d0ba:8388#ss-v6"))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, "ss", source.Format)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeShadowsocks, got.Type)
	require.Equal(t, "ss-v6", got.Name)
	require.Equal(t, "2a03:4000:38:dff:b49c:beff:fe49:d0ba", got.Server)
	require.Equal(t, uint16(8388), got.Port)
	require.Equal(t, "aes-128-gcm", got.Cipher)
	require.Equal(t, "secret", got.Password)
}

func TestParseVMessURI(t *testing.T) {
	p := uri.NewParser()
	nodes, source, err := p.Parse(context.Background(), []byte("vmess://eyJ2IjoiMiIsInBzIjoidm1lc3Mtd3MtdGxzIiwiYWRkIjoiZXhhbXBsZS5jb20iLCJwb3J0IjoiNDQzIiwiaWQiOiIxMTExMTExMS0xMTExLTExMTEtMTExMS0xMTExMTExMTExMTEiLCJhaWQiOiIwIiwic2N5IjoiYXV0byIsIm5ldCI6IndzIiwidHlwZSI6Im5vbmUiLCJob3N0IjoiY2RuLmV4YW1wbGUuY29tIiwicGF0aCI6Ii93cyIsInRscyI6InRscyIsInNuaSI6ImV4YW1wbGUuY29tIn0="))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, "vmess", source.Format)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeVMess, got.Type)
	require.Equal(t, "vmess-ws-tls", got.Name)
	require.Equal(t, "example.com", got.Server)
	require.Equal(t, uint16(443), got.Port)
	require.Equal(t, "11111111-1111-1111-1111-111111111111", got.UUID)
	require.NotNil(t, got.Transport)
	require.Equal(t, "websocket", got.Transport.Type)
	require.Equal(t, "/ws", got.Transport.Path)
	require.NotNil(t, got.TLS)
	require.True(t, got.TLS.Enabled)
	require.Equal(t, "example.com", got.TLS.ServerName)
}

func TestParseVMessURLSafeBase64Payload(t *testing.T) {
	p := uri.NewParser()
	doc := `{"v":"2","ps":"vmess-url-safe","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"ws","host":"cdn.example.com","path":"/ws","tls":"tls","sni":"sni.example.com"}`
	raw := "vmess://" + base64.RawURLEncoding.EncodeToString([]byte(doc))

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeVMess, got.Type)
	require.Equal(t, "vmess-url-safe", got.Name)
	require.Equal(t, "example.com", got.Server)
	require.Equal(t, uint16(443), got.Port)
	require.Equal(t, "11111111-1111-1111-1111-111111111111", got.UUID)
	require.Equal(t, "auto", got.Cipher)
	require.NotNil(t, got.Transport)
	require.Equal(t, "websocket", got.Transport.Type)
	require.Equal(t, "cdn.example.com", got.Transport.Host)
	require.Equal(t, "/ws", got.Transport.Path)
	require.NotNil(t, got.TLS)
	require.True(t, got.TLS.Enabled)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
}

func TestParseVMessFieldAliases(t *testing.T) {
	p := uri.NewParser()
	doc := `{"v":"2","name":"alias-name","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","alter_id":"7","cipher":"zero","net":"ws","wsHost":"cdn.example.com","wsPath":"/alias","packet-encoding":"xudp"}`
	raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(doc))

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	got := nodes[0]
	require.Equal(t, "alias-name", got.Name)
	require.Equal(t, "zero", got.Cipher)
	require.Equal(t, 7, got.AlterID)
	require.Equal(t, "xudp", got.PacketEncoding)
	require.NotNil(t, got.Transport)
	require.Equal(t, "cdn.example.com", got.Transport.Host)
	require.Equal(t, "/alias", got.Transport.Path)
	require.NotContains(t, got.Raw, "vmess.name")
	require.NotContains(t, got.Raw, "vmess.alter_id")
	require.NotContains(t, got.Raw, "vmess.cipher")
	require.NotContains(t, got.Raw, "vmess.wsHost")
	require.NotContains(t, got.Raw, "vmess.wsPath")
}

func TestParseVMessCompatAliasVariants(t *testing.T) {
	p := uri.NewParser()
	tests := []struct {
		name     string
		doc      string
		wantHost string
		wantPath string
	}{
		{
			name:     "uuid network requestHost wspath serverName",
			doc:      `{"v":"2","name":"alias","add":"example.com","port":"443","uuid":"11111111-1111-1111-1111-111111111111","alterId":"7","security":"auto","network":"ws","requestHost":"cdn.example.com","wspath":"/ws","streamSecurity":"tls","serverName":"sni.example.com"}`,
			wantHost: "cdn.example.com",
			wantPath: "/ws",
		},
		{
			name:     "dash host and path aliases",
			doc:      `{"v":"2","ps":"alias","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","net":"ws","ws-host":"ws.example.com","ws-path":"/dash","tls":"tls","servername":"sni.example.com"}`,
			wantHost: "ws.example.com",
			wantPath: "/dash",
		},
		{
			name:     "http host and obfs uri aliases",
			doc:      `{"v":"2","ps":"alias","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","net":"ws","http-host":"http.example.com","obfs-uri":"/obfs","tls":"tls","sni":"sni.example.com"}`,
			wantHost: "http.example.com",
			wantPath: "/obfs",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(tc.doc))

			nodes, _, err := p.Parse(context.Background(), []byte(raw))
			require.NoError(t, err)
			got := nodes[0]
			require.Equal(t, "11111111-1111-1111-1111-111111111111", got.UUID)
			require.Equal(t, uint16(443), got.Port)
			require.NotNil(t, got.Transport)
			require.Equal(t, "websocket", got.Transport.Type)
			require.Equal(t, tc.wantHost, got.Transport.Host)
			require.Equal(t, tc.wantPath, got.Transport.Path)
			require.NotNil(t, got.TLS)
			require.True(t, got.TLS.Enabled)
			require.Equal(t, "sni.example.com", got.TLS.ServerName)
			require.Empty(t, got.Raw)
		})
	}
}

func TestParseVMessHeaderTypeAliasPreservedRaw(t *testing.T) {
	p := uri.NewParser()
	doc := `{"v":"2","ps":"alias","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","net":"tcp","headerType":"http","type":"none"}`
	raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(doc))

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.Equal(t, "tcp", got.Transport.Type)
	require.JSONEq(t, `"http"`, string(got.Raw["vmess.headerType"]))
	require.NotContains(t, got.Raw, "vmess.type")
}

func TestParseVMessPromotesKnownCompatibilityFields(t *testing.T) {
	p := uri.NewParser()
	doc := `{"add":"202.78.162.5","aid":0,"host":"irsoft.sytes.net","id":"2ff97c6d-8557-42a4-b43f-19c77c5959ea","net":"ws","path":"/@forwardv2ray","port":443,"ps":"github.com/freefq - 印度  2","tls":"tls","type":"auto","security":"auto","skip-cert-verify":true,"sni":""}`
	raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(doc))

	nodes, source, err := p.ParseList(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, 0, got.AlterID)
	require.NotNil(t, got.TLS)
	require.True(t, got.TLS.Enabled)
	require.True(t, got.TLS.InsecureSkipVerify)
	require.Empty(t, got.Raw)
}

func TestParseVMessNonDefaultHeaderTypeStaysRaw(t *testing.T) {
	p := uri.NewParser()
	doc := `{"v":"2","ps":"http-header","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","net":"tcp","type":"http"}`
	raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(doc))

	nodes, source, err := p.ParseList(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	require.Len(t, source.Warnings, 1)
	require.Equal(t, "vmess.type", source.Warnings[0].Field)
	require.JSONEq(t, `"http"`, string(nodes[0].Raw["vmess.type"]))
}

func TestParseVMessGRPCPathBecomesServiceName(t *testing.T) {
	p := uri.NewParser()
	doc := `{"ps":"grpc","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","net":"grpc","path":"svc"}`
	raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(doc))

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.Equal(t, "grpc", got.Transport.Type)
	require.Equal(t, "svc", got.Transport.ServiceName)
	require.Empty(t, got.Transport.Path)
}

func TestParseVMessGRPCExplicitServiceName(t *testing.T) {
	p := uri.NewParser()
	doc := `{"ps":"grpc","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","net":"grpc","serviceName":"svc"}`
	raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(doc))

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.Equal(t, "grpc", got.Transport.Type)
	require.Equal(t, "svc", got.Transport.ServiceName)
	require.Empty(t, got.Transport.Path)
	require.NotContains(t, got.Raw, "vmess.serviceName")
}

func TestParseShadowsocksRURI(t *testing.T) {
	p := uri.NewParser()
	password := base64.RawURLEncoding.EncodeToString([]byte("secret"))
	remarks := base64.RawURLEncoding.EncodeToString([]byte("ssr-node"))
	obfsParam := base64.RawURLEncoding.EncodeToString([]byte("cdn.example.com"))
	payload := base64.RawURLEncoding.EncodeToString([]byte("ssr.example.com:8388:auth_sha1_v4:aes-128-cfb:http_simple:" + password + "/?remarks=" + remarks + "&obfsparam=" + obfsParam))

	nodes, source, err := p.Parse(context.Background(), []byte("ssr://"+payload))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, "ssr", source.Format)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeShadowsocksR, got.Type)
	require.Equal(t, "ssr-node", got.Name)
	require.Equal(t, "ssr.example.com", got.Server)
	require.Equal(t, uint16(8388), got.Port)
	require.Equal(t, "aes-128-cfb", got.Cipher)
	require.Equal(t, "secret", got.Password)
	require.NotNil(t, got.ShadowsocksR)
	require.Equal(t, "auth_sha1_v4", got.ShadowsocksR.Protocol)
	require.Equal(t, "http_simple", got.ShadowsocksR.Obfs)
	require.Equal(t, "cdn.example.com", got.ShadowsocksR.ObfsParam)
}

func TestParseShadowsocksCipherAliases(t *testing.T) {
	p := uri.NewParser()
	tests := []struct {
		name       string
		method     string
		wantCipher string
	}{
		{name: "upper chacha", method: "CHACHA20-IETF-POLY1305", wantCipher: "chacha20-ietf-poly1305"},
		{name: "underscore aes", method: "AES_256_GCM", wantCipher: "aes-256-gcm"},
		{name: "legacy aead", method: "AEAD_AES_128_GCM", wantCipher: "aes-128-gcm"},
		{name: "unknown", method: " Unknown_Cipher ", wantCipher: "unknown-cipher"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userinfo := base64.RawURLEncoding.EncodeToString([]byte(tc.method + ":secret"))
			raw := "ss://" + userinfo + "@example.com:8388#ss"
			nodes, _, err := p.Parse(context.Background(), []byte(raw))
			require.NoError(t, err)
			require.Equal(t, tc.wantCipher, nodes[0].Cipher)
		})
	}
}

func TestParseTrojanWebSocketAliases(t *testing.T) {
	p := uri.NewParser()
	raw := "trojan://secret@example.com:443?security=tls&peer=sni.example.com&type=ws&wsHost=cdn.example.com&wsPath=%2Fws&allow_insecure=1#trojan-ws"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	got := nodes[0]
	require.NotNil(t, got.TLS)
	require.True(t, got.TLS.Enabled)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.True(t, got.TLS.InsecureSkipVerify)
	require.NotNil(t, got.Transport)
	require.Equal(t, "websocket", got.Transport.Type)
	require.Equal(t, "cdn.example.com", got.Transport.Host)
	require.Equal(t, "/ws", got.Transport.Path)
	require.Equal(t, map[string]string{"Host": "cdn.example.com"}, got.Transport.Headers)
	require.NotContains(t, got.Raw, "uri.query.peer")
	require.NotContains(t, got.Raw, "uri.query.allow_insecure")
	require.NotContains(t, got.Raw, "uri.query.wsHost")
	require.NotContains(t, got.Raw, "uri.query.wsPath")
}

func TestParseTrojanInsecureAliasWithoutPeer(t *testing.T) {
	p := uri.NewParser()
	raw := "trojan://secret@example.com:443?security=tls&allow_insecure=1#trojan"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.NotNil(t, nodes[0].TLS)
	require.True(t, nodes[0].TLS.Enabled)
	require.True(t, nodes[0].TLS.InsecureSkipVerify)
	require.NotContains(t, nodes[0].Raw, "uri.query.allow_insecure")
}

func TestParseTelegramProxyLinks(t *testing.T) {
	p := uri.NewParser()
	tests := []struct {
		name         string
		raw          string
		typ          domain.NodeType
		wantRawExtra bool
	}{
		{name: "tg socks", raw: "tg://socks?server=example.com&port=1080&user=user&pass=pass&extra=value#tg-socks", typ: domain.NodeTypeSOCKS, wantRawExtra: true},
		{name: "tme socks", raw: "https://t.me/socks?server=example.com&port=1080&user=user&pass=pass", typ: domain.NodeTypeSOCKS},
		{name: "tg http", raw: "tg://http?server=example.com&port=8080&user=user&pass=pass", typ: domain.NodeTypeHTTP},
		{name: "tme http", raw: "https://t.me/http?server=example.com&port=8080&user=user&pass=pass", typ: domain.NodeTypeHTTP},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodes, _, err := p.Parse(context.Background(), []byte(tc.raw))
			require.NoError(t, err)
			got := nodes[0]
			require.Equal(t, tc.typ, got.Type)
			require.Equal(t, "example.com", got.Server)
			require.Equal(t, "user", got.Username)
			require.Equal(t, "pass", got.Password)
			if tc.wantRawExtra {
				require.JSONEq(t, `"value"`, string(got.Raw["uri.query.extra"]))
			}
		})
	}
}

func TestParseRejectsTelegramMTProtoProxyLinks(t *testing.T) {
	p := uri.NewParser()
	tests := []string{
		"tg://proxy?server=example.com&port=443&secret=abcdef",
		"https://t.me/proxy?server=example.com&port=443&secret=abcdef",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			_, _, err := p.Parse(context.Background(), []byte(raw))
			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeParseFailed), "unexpected error: %v", err)
			require.ErrorContains(t, err, "MTProto")
		})
	}
}

func TestParseHTTPOnTelegramHostWithoutProxyQuery(t *testing.T) {
	p := uri.NewParser()

	nodes, _, err := p.Parse(context.Background(), []byte("https://t.me/socks#name"))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeHTTP, nodes[0].Type)
	require.Equal(t, "t.me", nodes[0].Server)
	require.Equal(t, uint16(443), nodes[0].Port)
	require.Equal(t, "name", nodes[0].Name)
	require.NotNil(t, nodes[0].TLS)
	require.True(t, nodes[0].TLS.Enabled)
}

func TestParseHysteriaQueryAliases(t *testing.T) {
	p := uri.NewParser()
	raw := "hy://example.com:8443?authString=secret&peer=sni.example.com&allow_insecure=true&hop-interval=30s&obfs-password=obfs#hy"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeHysteria, got.Type)
	require.NotNil(t, got.Hysteria)
	require.Equal(t, "secret", got.Hysteria.AuthString)
	require.Equal(t, "obfs", got.Hysteria.ObfsPassword)
	require.Equal(t, "30s", got.Hysteria.HopInterval)
	require.NotNil(t, got.TLS)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.True(t, got.TLS.InsecureSkipVerify)
	require.NotContains(t, got.Raw, "uri.query.authString")
	require.NotContains(t, got.Raw, "uri.query.obfs-password")
}

func TestParseHysteria2QueryAliases(t *testing.T) {
	p := uri.NewParser()
	raw := "hysteria2://secret@example.com?obfs-type=salamander&obfsParam=obfs&servername=sni.example.com&allow-insecure=1&pcs=pin&hop-interval=30s#hy2"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeHysteria2, got.Type)
	require.Equal(t, uint16(443), got.Port)
	require.NotNil(t, got.Hysteria)
	require.Equal(t, "salamander", got.Hysteria.Obfs)
	require.Equal(t, "obfs", got.Hysteria.ObfsPassword)
	require.Equal(t, "30s", got.Hysteria.HopInterval)
	require.NotNil(t, got.TLS)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.True(t, got.TLS.InsecureSkipVerify)
	require.Equal(t, "pin", got.TLS.Fingerprint)
	require.NotContains(t, got.Raw, "uri.query.obfsParam")
	require.NotContains(t, got.Raw, "uri.query.allow-insecure")
}

func TestParseHysteria2PeerCompatibilityAlias(t *testing.T) {
	p := uri.NewParser()

	tests := []struct {
		name        string
		raw         string
		wantSNI     string
		wantPeerRaw bool
	}{
		{
			name:    "peer only fills server name",
			raw:     "hy2://secret@example.com:443?peer=sni.example.com&insecure=1#hy2",
			wantSNI: "sni.example.com",
		},
		{
			name:    "matching sni and peer does not leave raw",
			raw:     "hy2://secret@example.com:443?sni=sni.example.com&peer=sni.example.com#hy2",
			wantSNI: "sni.example.com",
		},
		{
			name:        "conflicting peer remains raw",
			raw:         "hy2://secret@example.com:443?sni=sni.example.com&peer=other.example.com#hy2",
			wantSNI:     "sni.example.com",
			wantPeerRaw: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodes, _, err := p.Parse(context.Background(), []byte(tc.raw))
			require.NoError(t, err)
			require.Len(t, nodes, 1)
			require.NotNil(t, nodes[0].TLS)
			require.Equal(t, tc.wantSNI, nodes[0].TLS.ServerName)
			if tc.wantPeerRaw {
				require.JSONEq(t, `"other.example.com"`, string(nodes[0].Raw["uri.query.peer"]))
			} else {
				require.NotContains(t, nodes[0].Raw, "uri.query.peer")
			}
		})
	}
}

func TestParseVLESSURIFingerprintIsClientFingerprint(t *testing.T) {
	p := uri.NewParser()
	nodes, _, err := p.Parse(context.Background(), []byte("vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&fp=chrome&sni=example.com#vless"))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].TLS)
	require.Equal(t, "chrome", nodes[0].TLS.ClientFingerprint)
	require.Empty(t, nodes[0].TLS.Fingerprint)
}

func TestParseRejectsUnknownScheme(t *testing.T) {
	p := uri.NewParser()
	_, _, err := p.Parse(context.Background(), []byte("wireguard://foo"))
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeParseFailed), "unexpected error: %v", err)
}

func TestParseRejectsInvalidURI(t *testing.T) {
	p := uri.NewParser()
	_, _, err := p.Parse(context.Background(), []byte("ss://broken"))
	require.ErrorContains(t, err, "parse_failed")
}

func TestParseRejectsInvalidVMessEncoding(t *testing.T) {
	p := uri.NewParser()
	_, _, err := p.Parse(context.Background(), []byte("vmess://not-base64"))
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeParseFailed), "unexpected error: %v", err)
}

func TestParseRejectsMissingVMessField(t *testing.T) {
	p := uri.NewParser()
	_, _, err := p.Parse(context.Background(), []byte("vmess://eyJwb3J0IjoiNDQzIiwiaWQiOiIxMTExMTExMS0xMTExLTExMTEtMTExMS0xMTExMTExMTExMTEifQ=="))
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeParseFailed), "unexpected error: %v", err)
}

func TestParseAdditionalURIProtocols(t *testing.T) {
	p := uri.NewParser()
	tests := []struct {
		name string
		raw  string
		typ  domain.NodeType
	}{
		{
			name: "vless",
			raw:  "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&sni=example.com&type=ws&host=cdn.example.com&path=%2Fws#vless-ws",
			typ:  domain.NodeTypeVLESS,
		},
		{
			name: "trojan",
			raw:  "trojan://secret@example.com:443?security=tls&sni=example.com&type=grpc&serviceName=svc#trojan-grpc",
			typ:  domain.NodeTypeTrojan,
		},
		{
			name: "hysteria",
			raw:  "hysteria://example.com:8443?auth_str=secret&up=20Mbps&down=100Mbps&obfs=obfs&sni=example.com#hy",
			typ:  domain.NodeTypeHysteria,
		},
		{
			name: "hysteria2",
			raw:  "hy2://secret@example.com:8443?obfs=salamander&obfs-password=obfs&sni=example.com#hy2",
			typ:  domain.NodeTypeHysteria2,
		},
		{
			name: "tuic",
			raw:  "tuic://11111111-1111-1111-1111-111111111111:secret@example.com:443?congestion_control=bbr&udp_relay_mode=native&sni=example.com#tuic",
			typ:  domain.NodeTypeTUIC,
		},
		{
			name: "socks",
			raw:  "socks5://user:pass@example.com:1080#socks",
			typ:  domain.NodeTypeSOCKS,
		},
		{
			name: "http",
			raw:  "https://user:pass@example.com:8443#http",
			typ:  domain.NodeTypeHTTP,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodes, source, err := p.Parse(context.Background(), []byte(tc.raw))
			require.NoError(t, err)
			require.NotNil(t, source)
			require.Len(t, nodes, 1)
			require.Equal(t, tc.typ, nodes[0].Type)
			require.Equal(t, "example.com", nodes[0].Server)
		})
	}
}

func TestParseBase64Subscription(t *testing.T) {
	p := uri.NewParser()
	input := "ss://aes-128-gcm:secret@example.com:8388#node-a\nsocks5://user:pass@example.com:1080#socks"
	encoded := "c3M6Ly9hZXMtMTI4LWdjbTpzZWNyZXRAZXhhbXBsZS5jb206ODM4OCNub2RlLWEKc29ja3M1Oi8vdXNlcjpwYXNzQGV4YW1wbGUuY29tOjEwODAjc29ja3M="
	require.NotEqual(t, input, encoded)
	nodes, source, err := p.ParseList(context.Background(), []byte(encoded))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 2)
	require.Equal(t, domain.NodeTypeShadowsocks, nodes[0].Type)
	require.Equal(t, domain.NodeTypeSOCKS, nodes[1].Type)
}

func TestParseListAcceptsURIJSONAndYAMLLines(t *testing.T) {
	p := uri.NewParser()
	input := strings.Join([]string{
		"ss://aes-128-gcm:secret@example.com:8388#node-a",
		`{"name":"json-socks","type":"socks","server":"json.example.com","port":1080}`,
		`{name: yaml-http, type: http, server: yaml.example.com, port: 8080}`,
		`{"nodes":[{"name":"json-vless","type":"vless","server":"vless.example.com","port":443,"uuid":"11111111-1111-1111-1111-111111111111"}]}`,
		`{nodes: [{name: yaml-trojan, type: trojan, server: trojan.example.com, port: 443, password: secret}]}`,
	}, "\n")

	nodes, source, err := p.ParseList(context.Background(), []byte(input))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, "uri-list", source.Format)
	require.Len(t, nodes, 5)
	require.Equal(t, []domain.NodeType{
		domain.NodeTypeShadowsocks,
		domain.NodeTypeSOCKS,
		domain.NodeTypeHTTP,
		domain.NodeTypeVLESS,
		domain.NodeTypeTrojan,
	}, []domain.NodeType{nodes[0].Type, nodes[1].Type, nodes[2].Type, nodes[3].Type, nodes[4].Type})
	require.Equal(t, "json-nodes", nodes[1].SourceFormat)
	require.Equal(t, "yaml-node", nodes[2].SourceFormat)
}

func TestParseStrictListAcceptsURIOnlyLines(t *testing.T) {
	p := uri.NewParser()
	input := strings.Join([]string{
		"ss://aes-128-gcm:secret@example.com:8388#node-a",
		`{"name":"json-socks","type":"socks","server":"json.example.com","port":1080}`,
		"socks5://user:pass@example.com:1080#socks",
	}, "\n")

	nodes, source, err := p.ParseStrictList(context.Background(), []byte(input))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, "uri-list", source.Format)
	require.Len(t, nodes, 2)
	require.Equal(t, []domain.NodeType{domain.NodeTypeShadowsocks, domain.NodeTypeSOCKS}, []domain.NodeType{nodes[0].Type, nodes[1].Type})
	require.Len(t, source.Warnings, 1)
	require.Equal(t, "parse_line_failed", source.Warnings[0].Code)
	require.Contains(t, source.Warnings[0].Message, "JSON/YAML node lines are not allowed")
}

func TestParseStrictListRejectsOnlyJSONAndYAMLLines(t *testing.T) {
	p := uri.NewParser()
	input := strings.Join([]string{
		`{"name":"json-socks","type":"socks","server":"json.example.com","port":1080}`,
		`{name: yaml-http, type: http, server: yaml.example.com, port: 8080}`,
	}, "\n")

	_, source, err := p.ParseStrictList(context.Background(), []byte(input))

	require.Error(t, err)
	require.ErrorContains(t, err, "no nodes found")
	require.NotNil(t, source)
	require.Len(t, source.Warnings, 2)
	require.Equal(t, "parse_line_failed", source.Warnings[0].Code)
	require.Contains(t, source.Warnings[0].Message, "JSON/YAML node lines are not allowed")
}

func TestParseBase64SubscriptionWithJSONAndYAMLLines(t *testing.T) {
	p := uri.NewParser()
	input := strings.Join([]string{
		`{"name":"json-socks","type":"socks","server":"json.example.com","port":1080}`,
		`{name: yaml-http, type: http, server: yaml.example.com, port: 8080}`,
	}, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(input))

	nodes, source, err := p.ParseList(context.Background(), []byte(encoded))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 2)
	require.Equal(t, domain.NodeTypeSOCKS, nodes[0].Type)
	require.Equal(t, domain.NodeTypeHTTP, nodes[1].Type)
}

func TestParseListSkipsVMessZeroPortPlaceholder(t *testing.T) {
	p := uri.NewParser()
	placeholderDoc := `{"v":"2","ps":"更新于:02-07 04:00 -by BuLink.xyz- 以下节点不计流量","add":"使用前记得更新订阅","port":"0","id":"6a3bcc08-9c77-4c02-844b-4a694c4f2fea","aid":"0","net":"tcp","type":"none","host":"","path":"","tls":""}`
	placeholderLine := "vmess://" + base64.StdEncoding.EncodeToString([]byte(placeholderDoc))
	input := strings.Join([]string{
		"ss://aes-128-gcm:secret@example.com:8388#node-a",
		placeholderLine,
		"socks5://user:pass@example.com:1080#socks",
	}, "\n")

	nodes, source, err := p.ParseList(context.Background(), []byte(input))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 2)
	require.Equal(t, domain.NodeTypeShadowsocks, nodes[0].Type)
	require.Equal(t, domain.NodeTypeSOCKS, nodes[1].Type)
	require.Len(t, source.Warnings, 1)
	warning := source.Warnings[0]
	require.Equal(t, "parse_line_skipped", warning.Code)
	require.Equal(t, "port", warning.Field)
	require.Equal(t, "uri-list", warning.Source)
	require.Contains(t, warning.Message, "zero port")
	require.NotNil(t, warning.NodeContext)
	require.Equal(t, "uri-list", warning.NodeContext.Format)
	require.Equal(t, domain.NodeTypeVMess, warning.NodeContext.Type)
	require.Equal(t, placeholderLine, warning.NodeContext.RawLine)
	require.Equal(t, 2, warning.NodeContext.Line)
}

func TestParseListLineErrorsBecomeWarningsWhenNodesRemain(t *testing.T) {
	p := uri.NewParser()
	input := strings.Join([]string{
		"ss://aes-128-gcm:secret@example.com:8388#node-a",
		"not a node",
		"socks5://user:pass@example.com:1080#socks",
	}, "\n")

	nodes, source, err := p.ParseList(context.Background(), []byte(input))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 2)
	require.Equal(t, domain.NodeTypeShadowsocks, nodes[0].Type)
	require.Equal(t, domain.NodeTypeSOCKS, nodes[1].Type)
	require.Len(t, source.Warnings, 1)
	warning := source.Warnings[0]
	require.Equal(t, "parse_line_failed", warning.Code)
	require.Equal(t, "uri-list", warning.Source)
	require.Contains(t, warning.Message, `line "not a node"`)
	require.Contains(t, warning.Message, "URI:")
	require.Contains(t, warning.Message, "JSON:")
	require.Contains(t, warning.Message, "YAML:")
	require.NotNil(t, warning.NodeContext)
	require.Equal(t, "uri-list", warning.NodeContext.Format)
	require.Equal(t, "not a node", warning.NodeContext.RawLine)
	require.Equal(t, 2, warning.NodeContext.Line)
}

func TestParseListRejectsOnlyBadLines(t *testing.T) {
	p := uri.NewParser()
	_, source, err := p.ParseList(context.Background(), []byte("# comment\n\nnot a node"))
	require.Error(t, err)
	require.ErrorContains(t, err, "no nodes found")
	require.True(t, domain.IsCode(err, domain.CodeParseFailed), "unexpected error: %v", err)
	require.NotNil(t, source)
	require.Len(t, source.Warnings, 1)
	require.Equal(t, "parse_line_failed", source.Warnings[0].Code)
}

func TestParseRawBase64Subscription(t *testing.T) {
	p := uri.NewParser()
	input := "ss://aes-128-gcm:secret@example.com:8388#node-a"
	encoded := base64.RawStdEncoding.EncodeToString([]byte(input))

	nodes, source, err := p.ParseList(context.Background(), []byte(encoded))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeShadowsocks, nodes[0].Type)
}

func TestParseListSkipsCommentsAndReportsLineNumber(t *testing.T) {
	p := uri.NewParser()
	nodes, source, err := p.ParseList(context.Background(), []byte(`# comment

ss://aes-128-gcm:secret@example.com:8388#node-a
wireguard://unsupported
`))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	require.Len(t, source.Warnings, 1)
	warning := source.Warnings[0]
	require.Equal(t, "parse_line_failed", warning.Code)
	require.NotNil(t, warning.NodeContext)
	require.Equal(t, "uri-list", warning.NodeContext.Format)
	require.Equal(t, "wireguard://unsupported", warning.NodeContext.RawLine)
	require.Equal(t, 4, warning.NodeContext.Line)
}

func TestParseListSkipsMieruCountMismatchWithWarning(t *testing.T) {
	p := uri.NewParser()
	badMieru := "mierus://user:pass@example.com?port=1234&port=5678&protocol=tcp#bad"
	input := strings.Join([]string{
		"ss://aes-128-gcm:secret@example.com:8388#node-a",
		badMieru,
		"socks5://user:pass@example.com:1080#socks",
	}, "\n")

	nodes, source, err := p.ParseList(context.Background(), []byte(input))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 2)
	require.Equal(t, domain.NodeTypeShadowsocks, nodes[0].Type)
	require.Equal(t, domain.NodeTypeSOCKS, nodes[1].Type)
	require.Len(t, source.Warnings, 1)
	warning := source.Warnings[0]
	require.Equal(t, "parse_line_failed", warning.Code)
	require.Equal(t, "uri-list", warning.Source)
	require.Contains(t, warning.Message, "mieru port and protocol counts must match")
	require.NotNil(t, warning.NodeContext)
	require.Equal(t, "uri-list", warning.NodeContext.Format)
	require.Equal(t, badMieru, warning.NodeContext.RawLine)
	require.Equal(t, 2, warning.NodeContext.Line)
}

func TestParseListUnknownFieldWarningsIncludeLineContext(t *testing.T) {
	p := uri.NewParser()
	rawLine := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&ech=ech-config&unknown-param=value#vless"
	input := strings.Join([]string{
		"# comment",
		"",
		rawLine,
	}, "\n")

	nodes, source, err := p.ParseList(context.Background(), []byte(input))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotNil(t, source)
	require.Len(t, source.Warnings, 1)
	require.Equal(t, []string{"uri.query.unknown-param"}, warningFields(source.Warnings))
	for _, warning := range source.Warnings {
		require.NotNil(t, warning.NodeIndex)
		require.Equal(t, 0, *warning.NodeIndex)
		require.NotNil(t, warning.NodeContext)
		require.Equal(t, "uri-list", warning.NodeContext.Format)
		require.Equal(t, "vless", warning.NodeContext.Name)
		require.Equal(t, domain.NodeTypeVLESS, warning.NodeContext.Type)
		require.Equal(t, "example.com", warning.NodeContext.Server)
		require.Equal(t, uint16(443), warning.NodeContext.Port)
		require.Equal(t, rawLine, warning.NodeContext.RawLine)
		require.Equal(t, 3, warning.NodeContext.Line)
	}
}

func TestParseListRejectsNoNodes(t *testing.T) {
	p := uri.NewParser()
	_, _, err := p.ParseList(context.Background(), []byte("# only comments"))
	require.Error(t, err)
	require.ErrorContains(t, err, "no nodes found")
}

func TestParseShadowsocksBase64UserinfoAndQuery(t *testing.T) {
	p := uri.NewParser()
	credentials := base64.RawURLEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:secret"))
	raw := "ss://" + credentials + "@example.com:8388?plugin=v2ray-plugin&plugin-opts=mode%3Dwebsocket%3Bhost%3Dcdn.example.com&extra=value#ss"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, "chacha20-ietf-poly1305", got.Cipher)
	require.Equal(t, "secret", got.Password)
	require.Equal(t, "v2ray-plugin", got.Plugin)
	require.Equal(t, map[string]any{"raw": "mode=websocket;host=cdn.example.com"}, got.PluginOptions)
	require.JSONEq(t, `"value"`, string(got.Raw["uri.query.extra"]))
}

func TestParseShadowsocksLegacyBase64Payload(t *testing.T) {
	p := uri.NewParser()
	payload := base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:secret@example.com:19629"))
	raw := "ss://" + payload + "#legacy-ss"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeShadowsocks, got.Type)
	require.Equal(t, "legacy-ss", got.Name)
	require.Equal(t, "example.com", got.Server)
	require.Equal(t, uint16(19629), got.Port)
	require.Equal(t, "aes-256-gcm", got.Cipher)
	require.Equal(t, "secret", got.Password)
}

func TestParseBase64SubscriptionWithLegacyShadowsocksPayload(t *testing.T) {
	p := uri.NewParser()
	payload := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:secret@example.com:19629"))
	input := "ss://" + payload + "#legacy-ss\nvmess://eyJ2IjoiMiIsInBzIjoidm1lc3MiLCJhZGQiOiJ2LmV4YW1wbGUuY29tIiwicG9ydCI6IjQ0MyIsImlkIjoiMTExMTExMTEtMTExMS0xMTExLTExMTEtMTExMTExMTExMTExIiwiYWlkIjoiMCIsIm5ldCI6InRjcCIsInR5cGUiOiJub25lIn0="
	encoded := base64.StdEncoding.EncodeToString([]byte(input))

	nodes, source, err := p.ParseList(context.Background(), []byte(encoded))
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, nodes, 2)
	require.Equal(t, domain.NodeTypeShadowsocks, nodes[0].Type)
	require.Equal(t, "example.com", nodes[0].Server)
	require.Equal(t, uint16(19629), nodes[0].Port)
	require.Equal(t, domain.NodeTypeVMess, nodes[1].Type)
}

func TestParseShadowsocksSIP002RawUserinfoPluginAndSlash(t *testing.T) {
	p := uri.NewParser()
	raw := "ss://aes-256-gcm:p%40ss@example.com:8388/?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dcdn.example.com#ss"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, "aes-256-gcm", got.Cipher)
	require.Equal(t, "p@ss", got.Password)
	require.Equal(t, "obfs-local", got.Plugin)
	require.Equal(t, map[string]any{"raw": "obfs=http;obfs-host=cdn.example.com"}, got.PluginOptions)
}

func TestParseShadowsocksWholeBase64SIP002Payload(t *testing.T) {
	p := uri.NewParser()
	payload := base64.RawURLEncoding.EncodeToString([]byte("AEAD_AES_256_GCM:secret@example.com:8388"))

	nodes, _, err := p.Parse(context.Background(), []byte("ss://"+payload+"#ss"))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeShadowsocks, got.Type)
	require.Equal(t, "aes-256-gcm", got.Cipher)
	require.Equal(t, "secret", got.Password)
	require.Equal(t, "example.com", got.Server)
	require.Equal(t, uint16(8388), got.Port)
}

func TestParseShadowsocksWholeBase64SIP002PayloadWithDecodedQuery(t *testing.T) {
	p := uri.NewParser()
	decoded := "AEAD_AES_256_GCM:secret@example.com:8388/?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dcdn.example.com&extra=value"
	payload := base64.RawURLEncoding.EncodeToString([]byte(decoded))

	nodes, _, err := p.Parse(context.Background(), []byte("ss://"+payload+"#ss"))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeShadowsocks, got.Type)
	require.Equal(t, "aes-256-gcm", got.Cipher)
	require.Equal(t, "secret", got.Password)
	require.Equal(t, "example.com", got.Server)
	require.Equal(t, uint16(8388), got.Port)
	require.Equal(t, "obfs-local", got.Plugin)
	require.Equal(t, map[string]any{"raw": "obfs=http;obfs-host=cdn.example.com"}, got.PluginOptions)
	require.JSONEq(t, `"value"`, string(got.Raw["uri.query.extra"]))
}

func TestParseShadowsocksPluginQueryKeepsSIP002ArgumentOverPluginOptsAlias(t *testing.T) {
	p := uri.NewParser()
	raw := "ss://aes-256-gcm:secret@example.com:8388?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dcdn.example.com&plugin-opts=ignored#ss"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "obfs-local", nodes[0].Plugin)
	require.Equal(t, map[string]any{"raw": "obfs=http;obfs-host=cdn.example.com"}, nodes[0].PluginOptions)
}

func TestParseHysteriaOfficialURIFields(t *testing.T) {
	p := uri.NewParser()
	raw := "hysteria://example.com:8443?auth=secret&peer=sni.example.com&insecure=1&upmbps=20&downmbps=100&obfs=xplus&obfsParam=obfs-pass&protocol=udp#hy"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeHysteria, got.Type)
	require.NotNil(t, got.TLS)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.True(t, got.TLS.InsecureSkipVerify)
	require.NotNil(t, got.Hysteria)
	require.Equal(t, 20, got.Hysteria.UpMbps)
	require.Equal(t, 100, got.Hysteria.DownMbps)
	require.Equal(t, "udp", got.Hysteria.Protocol)
	require.Equal(t, "xplus", got.Hysteria.Obfs)
	require.Equal(t, "obfs-pass", got.Hysteria.ObfsPassword)
	require.NotContains(t, got.Raw, "uri.query.protocol")
}

func TestParseHysteria2OfficialURIFields(t *testing.T) {
	p := uri.NewParser()
	nodes, _, err := p.Parse(context.Background(), []byte("hy2://secret@example.com?sni=sni.example.com&insecure=1&pinSHA256=pin&obfs=salamander&obfs-password=obfs#hy2"))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, uint16(443), got.Port)
	require.Equal(t, "secret", got.Password)
	require.NotNil(t, got.TLS)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.True(t, got.TLS.InsecureSkipVerify)
	require.Equal(t, "pin", got.TLS.Fingerprint)
	require.Empty(t, got.TLS.ClientFingerprint)
	require.Equal(t, "salamander", got.Hysteria.Obfs)
	require.Equal(t, "obfs", got.Hysteria.ObfsPassword)
}

func TestParseHysteria2MultiPortHost(t *testing.T) {
	p := uri.NewParser()
	nodes, _, err := p.Parse(context.Background(), []byte("hy2://secret@example.com:443,8443-9000?sni=sni.example.com#hy2"))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "example.com", nodes[0].Server)
	require.Equal(t, uint16(443), nodes[0].Port)
	require.Equal(t, []string{"443", "8443-9000"}, nodes[0].Hysteria.ServerPorts)
}

func TestParseHysteria2BareIPv6HostPort(t *testing.T) {
	p := uri.NewParser()
	nodes, _, err := p.Parse(context.Background(), []byte("hysteria2://secret@2a03:4000:38:dff:b49c:beff:fe49:d0ba:56245?sni=sni.example.com&insecure=1#hy2"))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, "2a03:4000:38:dff:b49c:beff:fe49:d0ba", got.Server)
	require.Equal(t, uint16(56245), got.Port)
	require.Equal(t, "secret", got.Password)
	require.NotNil(t, got.TLS)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.True(t, got.TLS.InsecureSkipVerify)
}

func TestParseURIQueryFingerprintFields(t *testing.T) {
	p := uri.NewParser()
	tests := []struct {
		name         string
		query        string
		rawKey       string
		wantClientFP string
		wantCertPin  string
	}{
		{name: "fp", query: "fp=chrome", rawKey: "uri.query.fp", wantClientFP: "chrome"},
		{name: "fingerprint", query: "fingerprint=sha256:abcd", rawKey: "uri.query.fingerprint", wantCertPin: "sha256:abcd"},
		{name: "pinSHA256", query: "pinSHA256=pin", rawKey: "uri.query.pinSHA256", wantCertPin: "pin"},
		{name: "pcs", query: "pcs=pcs", rawKey: "uri.query.pcs", wantCertPin: "pcs"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls&encryption=none&" + tc.query
			nodes, _, err := p.Parse(context.Background(), []byte(raw))
			require.NoError(t, err)
			require.NotNil(t, nodes[0].TLS)
			require.Equal(t, tc.wantClientFP, nodes[0].TLS.ClientFingerprint)
			require.Equal(t, tc.wantCertPin, nodes[0].TLS.Fingerprint)
			require.NotContains(t, nodes[0].Raw, tc.rawKey)
		})
	}
}

func TestParseVLESSQueryTLSRealityTransportAndRaw(t *testing.T) {
	p := uri.NewParser()
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=reality&sni=sni.example.com&allowInsecure=1&alpn=h2,%20http/1.1&public-key=pk&short-id=sid&type=h2&host=cdn.example.com&path=%2Fh2&mode=packet-up&extra=value#vless"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.TLS)
	require.True(t, got.TLS.Enabled)
	require.True(t, got.TLS.InsecureSkipVerify)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.Equal(t, []string{"h2", "http/1.1"}, got.TLS.ALPN)
	require.NotNil(t, got.TLS.Reality)
	require.Equal(t, "pk", got.TLS.Reality.PublicKey)
	require.Equal(t, "sid", got.TLS.Reality.ShortID)
	require.NotNil(t, got.Transport)
	require.Equal(t, "http", got.Transport.Type)
	require.Equal(t, "/h2", got.Transport.Path)
	require.Equal(t, "cdn.example.com", got.Transport.Host)
	require.Equal(t, map[string]string{"Host": "cdn.example.com"}, got.Transport.Headers)
	require.JSONEq(t, `"packet-up"`, string(got.Raw["uri.query.mode"]))
	require.JSONEq(t, `"value"`, string(got.Raw["uri.query.extra"]))
}

func TestParseVLESSDefaultTCPCompatibilityQueryIsSilent(t *testing.T) {
	p := uri.NewParser()
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=reality&pbk=pk&sid=sid&type=tcp&host=cdn.example.com&headerType=none#vless"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.Equal(t, "tcp", got.Transport.Type)
	require.Empty(t, got.Transport.Host)
	require.Empty(t, got.Transport.Headers)
	require.NotContains(t, got.Raw, "uri.query.headerType")
	require.Empty(t, source.Warnings)
}

func TestParseVLESSEmptyPQVQueryIsSilent(t *testing.T) {
	p := uri.NewParser()
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=reality&pbk=pk&sid=sid&pqv=#vless"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotContains(t, nodes[0].Raw, "uri.query.pqv")
	require.Empty(t, source.Warnings)
}

func TestParseVLESSNonEmptyPQVQueryStaysRaw(t *testing.T) {
	p := uri.NewParser()
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=reality&pbk=pk&sid=sid&pqv=mlkem#vless"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.JSONEq(t, `"mlkem"`, string(nodes[0].Raw["uri.query.pqv"]))
	require.Len(t, source.Warnings, 1)
	require.Equal(t, "uri.query.pqv", source.Warnings[0].Field)
}

func TestParseVLESSXHTTPRealityURI(t *testing.T) {
	p := uri.NewParser()
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=reality&pbk=public-key&sid=08&type=xhttp&path=%2Fxhttp&host=cdn.example.com&mode=packet-up&packet-encoding=xudp&udp=true&sni=sni.example.com&fp=chrome&alpn=h2,http/1.1#vless-xhttp"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeVLESS, got.Type)
	require.Equal(t, "xudp", got.PacketEncoding)
	require.NotNil(t, got.TLS)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.Equal(t, "chrome", got.TLS.ClientFingerprint)
	require.Empty(t, got.TLS.Fingerprint)
	require.Equal(t, []string{"h2", "http/1.1"}, got.TLS.ALPN)
	require.NotNil(t, got.TLS.Reality)
	require.Equal(t, "public-key", got.TLS.Reality.PublicKey)
	require.Equal(t, "08", got.TLS.Reality.ShortID)
	require.NotNil(t, got.Transport)
	require.Equal(t, "xhttp", got.Transport.Type)
	require.Equal(t, "/xhttp", got.Transport.Path)
	require.Equal(t, "cdn.example.com", got.Transport.Host)
	require.Equal(t, map[string]string{"Host": "cdn.example.com"}, got.Transport.Headers)
	require.NotNil(t, got.Transport.XHTTP)
	require.Equal(t, "packet-up", got.Transport.XHTTP.Mode)
	require.NotContains(t, got.Raw, "uri.query.mode")
	require.JSONEq(t, `"true"`, string(got.Raw["uri.query.udp"]))
}

func TestParseTrojanXHTTPRealityURI(t *testing.T) {
	p := uri.NewParser()
	raw := "trojan://secret@example.com:443?security=reality&public-key=public-key&short-id=0088&type=xhttp&path=%2Fxhttp&authority=authority.example.com&mode=stream-one&sni=sni.example.com&fp=firefox#trojan-xhttp"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeTrojan, got.Type)
	require.NotNil(t, got.TLS)
	require.True(t, got.TLS.Enabled)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.Equal(t, "firefox", got.TLS.ClientFingerprint)
	require.Empty(t, got.TLS.Fingerprint)
	require.NotNil(t, got.TLS.Reality)
	require.Equal(t, "public-key", got.TLS.Reality.PublicKey)
	require.Equal(t, "0088", got.TLS.Reality.ShortID)
	require.NotNil(t, got.Transport)
	require.Equal(t, "xhttp", got.Transport.Type)
	require.Equal(t, "/xhttp", got.Transport.Path)
	require.Equal(t, "authority.example.com", got.Transport.Host)
	require.JSONEq(t, `"stream-one"`, string(got.Raw["uri.query.mode"]))
}

func TestParseURIQueryECHIsTypedAndUnknownStaysRaw(t *testing.T) {
	p := uri.NewParser()
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&ech=ech-config&unknown-param=value#vless"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].TLS)
	require.Equal(t, &domain.ECHOptions{Enabled: true, Config: []string{"ech-config"}}, nodes[0].TLS.ECH)
	require.NotContains(t, nodes[0].Raw, "uri.query.ech")
	require.JSONEq(t, `"value"`, string(nodes[0].Raw["uri.query.unknown-param"]))
	require.NotNil(t, source)
	require.Len(t, source.Warnings, 1)
	require.Equal(t, []string{"uri.query.unknown-param"}, warningFields(source.Warnings))
	for _, warning := range source.Warnings {
		require.NotNil(t, warning.NodeIndex)
		require.Equal(t, 0, *warning.NodeIndex)
		require.NotNil(t, warning.NodeContext)
		require.Equal(t, "uri", warning.NodeContext.Format)
		require.Equal(t, "vless", warning.NodeContext.Name)
		require.Equal(t, domain.NodeTypeVLESS, warning.NodeContext.Type)
		require.Equal(t, "example.com", warning.NodeContext.Server)
		require.Equal(t, uint16(443), warning.NodeContext.Port)
		require.Equal(t, raw, warning.NodeContext.RawLine)
		require.Zero(t, warning.NodeContext.Line)
	}
}
