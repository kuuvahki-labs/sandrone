package uri_test

import (
	"context"
	"encoding/base64"
	"net/url"
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

func TestParseLegacyVMessALPN(t *testing.T) {
	p := uri.NewParser()
	tests := []struct {
		name        string
		alpnJSON    string
		wantALPN    []string
		wantRawALPN bool
		wantWarning bool
	}{
		{
			name:     "comma separated string",
			alpnJSON: `" h2, http/1.1, "`,
			wantALPN: []string{"h2", "http/1.1"},
		},
		{
			name:     "string array",
			alpnJSON: `[" h2 ", "", "http/1.1"]`,
			wantALPN: []string{"h2", "http/1.1"},
		},
		{
			name:     "empty string is consumed",
			alpnJSON: `""`,
		},
		{
			name:     "empty array is consumed",
			alpnJSON: `[]`,
		},
		{
			name:     "whitespace string is consumed",
			alpnJSON: `" , "`,
		},
		{
			name:     "empty array entries are consumed",
			alpnJSON: `[" ", ""]`,
		},
		{
			name:        "mixed array remains raw",
			alpnJSON:    `["h2", 7]`,
			wantRawALPN: true,
			wantWarning: true,
		},
		{
			name:        "null remains raw",
			alpnJSON:    `null`,
			wantRawALPN: true,
			wantWarning: true,
		},
		{
			name:        "object remains raw",
			alpnJSON:    `{}`,
			wantRawALPN: true,
			wantWarning: true,
		},
		{
			name:        "number remains raw",
			alpnJSON:    `7`,
			wantRawALPN: true,
			wantWarning: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := `{"v":"2","ps":"vmess-alpn","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","alpn":` + tc.alpnJSON + `}`
			raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(doc))

			nodes, source, err := p.Parse(context.Background(), []byte(raw))

			require.NoError(t, err)
			require.Len(t, nodes, 1)
			if len(tc.wantALPN) > 0 {
				require.NotNil(t, nodes[0].TLS)
				require.False(t, nodes[0].TLS.Enabled)
				require.Equal(t, tc.wantALPN, nodes[0].TLS.ALPN)
			} else {
				require.Nil(t, nodes[0].TLS)
			}
			if tc.wantRawALPN {
				require.Contains(t, nodes[0].Raw, "vmess.alpn")
			} else {
				require.NotContains(t, nodes[0].Raw, "vmess.alpn")
			}
			if tc.wantWarning {
				require.Contains(t, warningFields(source.Warnings), "vmess.alpn")
			} else {
				require.NotContains(t, warningFields(source.Warnings), "vmess.alpn")
			}
		})
	}
}

func TestParseLegacyVMessWebSocketPathDoesNotDecodeURIEarlyDataConvention(t *testing.T) {
	p := uri.NewParser()
	payload := `{"v":"2","ps":"vmess-ws","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","net":"ws","path":"/do?ed=2048"}`
	raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))

	nodes, _, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.Equal(t, "/do?ed=2048", got.Transport.Path)
	require.Zero(t, got.Transport.MaxEarlyData)
	require.Empty(t, got.Transport.EarlyDataHeaderName)
}

func TestParseVMessAEADURL(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443#vmess-aead"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, "vmess", source.Format)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeVMess, got.Type)
	require.Equal(t, "vmess-aead", got.Name)
	require.Equal(t, "example.com", got.Server)
	require.Equal(t, uint16(443), got.Port)
	require.Equal(t, "11111111-1111-1111-1111-111111111111", got.UUID)
	require.Equal(t, "auto", got.Cipher)
	require.Zero(t, got.AlterID)
	require.Nil(t, got.TLS)
	require.Nil(t, got.Transport)
	require.Empty(t, got.Raw)
}

func TestParseVMessAEADWebSocketTLSURL(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?encryption=zero&security=tls&type=ws&host=cdn.example.com&path=%2Fws&sni=sni.example.com#VMess%20AEAD"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, "VMess AEAD", got.Name)
	require.Equal(t, "zero", got.Cipher)
	require.NotNil(t, got.TLS)
	require.True(t, got.TLS.Enabled)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.NotNil(t, got.Transport)
	require.Equal(t, "websocket", got.Transport.Type)
	require.Equal(t, "cdn.example.com", got.Transport.Host)
	require.Equal(t, "/ws", got.Transport.Path)
	require.Equal(t, map[string]string{"Host": "cdn.example.com"}, got.Transport.Headers)
	require.Empty(t, got.Raw)
}

func TestParseVMessAEADIPv6URL(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@[2001:db8::1]:8443?encryption=auto#ipv6"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "2001:db8::1", nodes[0].Server)
	require.Equal(t, uint16(8443), nodes[0].Port)
}

func TestParseVMessAEADRejectsMalformedAuthority(t *testing.T) {
	p := uri.NewParser()
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "missing explicit port",
			raw:  "vmess://11111111-1111-1111-1111-111111111111@example.com#missing-port",
			want: "parse vmess AEAD server",
		},
		{
			name: "password-style userinfo",
			raw:  "vmess://11111111-1111-1111-1111-111111111111:secret@example.com:443#password",
			want: "userinfo must contain only a uuid",
		},
		{
			name: "invalid uuid",
			raw:  "vmess://not-a-uuid@example.com:443",
			want: "invalid vmess uuid",
		},
		{
			name: "non-empty path",
			raw:  "vmess://11111111-1111-1111-1111-111111111111@example.com:443/not-in-profile",
			want: "vmess AEAD URL path is not allowed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := p.Parse(context.Background(), []byte(tc.raw))
			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeParseFailed), "unexpected error: %v", err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestParseVMessAEADRejectsDuplicateQueryKey(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?security=tls&security=reality"

	_, _, err := p.Parse(context.Background(), []byte(raw))

	require.Error(t, err)
	require.Contains(t, err.Error(), `duplicate vmess AEAD query parameter "security"`)
}

func TestParseVMessAEADXHTTPRealityAndUnsupportedRaw(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?encryption=auto&security=reality&pbk=public-key&sid=08&type=xhttp&path=%2Fxhttp&host=cdn.example.com&mode=packet-up&extra=%7B%22xmux%22%3A%7B%22maxConcurrency%22%3A%228-16%22%7D%7D&pqv=mlkem&fm=1#vmess-xhttp"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.TLS)
	require.NotNil(t, got.TLS.Reality)
	require.Equal(t, "public-key", got.TLS.Reality.PublicKey)
	require.Equal(t, "08", got.TLS.Reality.ShortID)
	require.NotNil(t, got.Transport)
	require.Equal(t, "xhttp", got.Transport.Type)
	require.NotNil(t, got.Transport.XHTTP)
	require.Equal(t, "packet-up", got.Transport.XHTTP.Mode)
	require.NotNil(t, got.Transport.XHTTP.ReuseSettings)
	require.Equal(t, "8-16", got.Transport.XHTTP.ReuseSettings.MaxConcurrency)
	require.NotContains(t, got.Raw, "uri.query.mode")
	require.NotContains(t, got.Raw, "uri.query.extra")
	require.JSONEq(t, `"mlkem"`, string(got.Raw["uri.query.pqv"]))
	require.JSONEq(t, `"1"`, string(got.Raw["uri.query.fm"]))
	require.Equal(t, []string{"uri.query.fm", "uri.query.pqv"}, warningFields(source.Warnings))
}

func TestParseVMessAEADPreservesModeInapplicableXHTTPFields(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?type=ws&mode=packet-up&extra=%7B%7D"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.Equal(t, "websocket", got.Transport.Type)
	require.JSONEq(t, `"packet-up"`, string(got.Raw["uri.query.mode"]))
	require.JSONEq(t, `"{}"`, string(got.Raw["uri.query.extra"]))
	require.Equal(t, []string{"uri.query.extra", "uri.query.mode"}, warningFields(source.Warnings))
}

func TestParseVMessAEADPreservesShortIDWithoutReality(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?sid=08"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Nil(t, nodes[0].TLS)
	require.JSONEq(t, `"08"`, string(nodes[0].Raw["uri.query.sid"]))
	require.Equal(t, []string{"uri.query.sid"}, warningFields(source.Warnings))
}

func TestParseVMessAEADAccountsForPromotedWebSocketHostAlias(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?type=ws&wsHost=cdn.example.com"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.Equal(t, "websocket", got.Transport.Type)
	require.Equal(t, "cdn.example.com", got.Transport.Host)
	require.NotContains(t, got.Raw, "uri.query.wsHost")
	require.NotContains(t, warningFields(source.Warnings), "uri.query.wsHost")
}

func TestParseVMessAEADPreservesUnselectedSemanticAlias(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?type=ws&host=selected.example.com&wsHost=unselected.example.com"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.Equal(t, "selected.example.com", got.Transport.Host)
	require.NotContains(t, got.Raw, "uri.query.host")
	require.JSONEq(t, `"unselected.example.com"`, string(got.Raw["uri.query.wsHost"]))
	require.Equal(t, []string{"uri.query.wsHost"}, warningFields(source.Warnings))
}

func TestParseVMessAEADPreservesInvalidXHTTPExtra(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?type=xhttp&mode=packet-up&extra=not-json"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.NotNil(t, got.Transport.XHTTP)
	require.Equal(t, "packet-up", got.Transport.XHTTP.Mode)
	require.JSONEq(t, `"not-json"`, string(got.Raw["uri.query.extra"]))
	require.Equal(t, []string{"uri.query.extra"}, warningFields(source.Warnings))
}

func TestParseVMessAEADPreservesNonObjectXHTTPExtra(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?type=xhttp&mode=packet-up&extra=null"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.NotNil(t, got.Transport.XHTTP)
	require.Equal(t, "packet-up", got.Transport.XHTTP.Mode)
	require.JSONEq(t, `"null"`, string(got.Raw["uri.query.extra"]))
	require.Equal(t, []string{"uri.query.extra"}, warningFields(source.Warnings))
}

func TestParseVMessAEADPreservesPartiallyRepresentableXHTTPExtra(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?type=xhttp&mode=packet-up&extra=%7B%22xmux%22%3A%7B%22maxConcurrency%22%3A%228-16%22%2C%22futureOption%22%3A1%7D%7D"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.NotNil(t, got.Transport.XHTTP)
	require.Equal(t, "packet-up", got.Transport.XHTTP.Mode)
	require.NotNil(t, got.Transport.XHTTP.ReuseSettings)
	require.Equal(t, "8-16", got.Transport.XHTTP.ReuseSettings.MaxConcurrency)
	require.JSONEq(t, `"{\"xmux\":{\"maxConcurrency\":\"8-16\",\"futureOption\":1}}"`, string(got.Raw["uri.query.extra"]))
	require.Equal(t, []string{"uri.query.extra"}, warningFields(source.Warnings))
}

func TestParseVMessAEADXHTTPDownloadPreservesSemanticallyIncompleteExtra(t *testing.T) {
	tests := []struct {
		name         string
		extra        string
		wantRaw      string
		requireTyped func(*testing.T, domain.NodeIR)
	}{
		{
			name:    "future network cannot disappear through the fixed renderer network",
			extra:   `{"downloadSettings":{"address":"download.example.com","network":"future","xhttpSettings":{"path":"/download"}}}`,
			wantRaw: `"{\"downloadSettings\":{\"address\":\"download.example.com\",\"network\":\"future\",\"xhttpSettings\":{\"path\":\"/download\"}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				require.NotNil(t, download.Server)
				require.Equal(t, "download.example.com", *download.Server)
				require.NotNil(t, download.Path)
				require.Equal(t, "/download", *download.Path)
			},
		},
		{
			name:    "missing network cannot inherit the fixed renderer network",
			extra:   `{"downloadSettings":{"security":"none"}}`,
			wantRaw: `"{\"downloadSettings\":{\"security\":\"none\"}}"`,
		},
		{
			name:    "null download settings cannot be treated as absent",
			extra:   `{"downloadSettings":null}`,
			wantRaw: `"{\"downloadSettings\":null}"`,
		},
		{
			name:    "future security cannot be treated as no security",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"future"}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"future\"}}"`,
		},
		{
			name:    "null security cannot be treated as absent security",
			extra:   `{"downloadSettings":{"network":"xhttp","security":null}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":null}}"`,
		},
		{
			name:    "tls security cannot consume reality settings",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"tls","realitySettings":{"publicKey":"public-key","shortId":"08"}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"tls\",\"realitySettings\":{\"publicKey\":\"public-key\",\"shortId\":\"08\"}}}"`,
		},
		{
			name:    "nested download settings cannot disappear after xmux promotion",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"none","xhttpSettings":{"extra":{"xmux":{"maxConnections":"4"},"downloadSettings":{"network":"xhttp","security":"none"}}}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"none\",\"xhttpSettings\":{\"extra\":{\"xmux\":{\"maxConnections\":\"4\"},\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"none\"}}}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				require.NotNil(t, download.ReuseSettings)
				require.Equal(t, "4", download.ReuseSettings.MaxConnections)
			},
		},
		{
			name:    "nested null download settings cannot be treated as absent",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"none","xhttpSettings":{"extra":{"downloadSettings":null}}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"none\",\"xhttpSettings\":{\"extra\":{\"downloadSettings\":null}}}}"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, source := parseVMessAEADXHTTPDownloadExtra(t, tc.extra)
			if tc.requireTyped != nil {
				tc.requireTyped(t, got)
			}
			require.Equal(t, tc.wantRaw, string(got.Raw["uri.query.extra"]))
			require.Equal(t, []string{"uri.query.extra"}, warningFields(source.Warnings))
			require.Len(t, source.Warnings, 1)
			require.Equal(t, "parse_unknown_field", source.Warnings[0].Code)
			require.Equal(t, "field preserved in NodeIR Raw", source.Warnings[0].Message)
			require.Equal(t, "uri.query.extra", source.Warnings[0].Field)
			require.Equal(t, "uri", source.Warnings[0].Source)
		})
	}
}

func TestParseVMessAEADXHTTPDownloadConsumesSupportedSettings(t *testing.T) {
	tests := []struct {
		name         string
		extra        string
		requireTyped func(*testing.T, domain.NodeIR)
	}{
		{
			name:  "exact xhttp network with no security",
			extra: `{"downloadSettings":{"address":"download.example.com","network":"xhttp","security":"none","xhttpSettings":{"path":"/download"}}}`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				require.NotNil(t, download.Server)
				require.Equal(t, "download.example.com", *download.Server)
				require.NotNil(t, download.Path)
				require.Equal(t, "/download", *download.Path)
				require.Nil(t, download.TLS)
			},
		},
		{
			name:  "exact tls security with tls settings",
			extra: `{"downloadSettings":{"network":"xhttp","security":"tls","tlsSettings":{"serverName":"download.example.com","allowInsecure":true,"alpn":["h2"],"fingerprint":"chrome"}}}`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				require.NotNil(t, download.TLS)
				require.True(t, download.TLS.Enabled)
				require.Equal(t, "download.example.com", download.TLS.ServerName)
				require.True(t, download.TLS.InsecureSkipVerify)
				require.Equal(t, []string{"h2"}, download.TLS.ALPN)
				require.Equal(t, "chrome", download.TLS.ClientFingerprint)
				require.Nil(t, download.TLS.Reality)
			},
		},
		{
			name:  "exact reality security with reality settings",
			extra: `{"downloadSettings":{"network":"xhttp","security":"reality","realitySettings":{"publicKey":"public-key","shortId":"08"}}}`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				require.NotNil(t, download.TLS)
				require.True(t, download.TLS.Enabled)
				require.NotNil(t, download.TLS.Reality)
				require.True(t, download.TLS.Reality.Enabled)
				require.Equal(t, "public-key", download.TLS.Reality.PublicKey)
				require.Equal(t, "08", download.TLS.Reality.ShortID)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, source := parseVMessAEADXHTTPDownloadExtra(t, tc.extra)
			tc.requireTyped(t, got)
			require.Empty(t, got.Raw)
			require.Empty(t, source.Warnings)
		})
	}
}

func TestParseVMessAEADXHTTPExplicitNullPreservesRawAndPromotesSiblings(t *testing.T) {
	tests := []struct {
		name         string
		extra        string
		wantRaw      string
		requireTyped func(*testing.T, domain.NodeIR)
	}{
		{
			name:    "top level null xmux does not hide a valid download sibling",
			extra:   `{"xmux":null,"downloadSettings":{"address":"download.example.com","network":"xhttp","security":"none"}}`,
			wantRaw: `"{\"xmux\":null,\"downloadSettings\":{\"address\":\"download.example.com\",\"network\":\"xhttp\",\"security\":\"none\"}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				require.Nil(t, got.Transport.XHTTP.ReuseSettings)
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				require.NotNil(t, download.Server)
				require.Equal(t, "download.example.com", *download.Server)
			},
		},
		{
			name:    "null reuse string and integer leaves do not hide valid leaves",
			extra:   `{"xmux":{"maxConcurrency":null,"maxConnections":"2","cMaxReuseTimes":"64","hKeepAlivePeriod":null}}`,
			wantRaw: `"{\"xmux\":{\"maxConcurrency\":null,\"maxConnections\":\"2\",\"cMaxReuseTimes\":\"64\",\"hKeepAlivePeriod\":null}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				reuse := got.Transport.XHTTP.ReuseSettings
				require.NotNil(t, reuse)
				require.Empty(t, reuse.MaxConcurrency)
				require.Equal(t, "2", reuse.MaxConnections)
				require.Equal(t, "64", reuse.CMaxReuseTimes)
				require.Zero(t, reuse.HKeepAlivePeriod)
			},
		},
		{
			name:    "null download address and port do not hide a valid path",
			extra:   `{"downloadSettings":{"address":null,"port":null,"network":"xhttp","security":"none","xhttpSettings":{"path":"/download"}}}`,
			wantRaw: `"{\"downloadSettings\":{\"address\":null,\"port\":null,\"network\":\"xhttp\",\"security\":\"none\",\"xhttpSettings\":{\"path\":\"/download\"}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				require.Nil(t, download.Server)
				require.Nil(t, download.Port)
				require.NotNil(t, download.Path)
				require.Equal(t, "/download", *download.Path)
			},
		},
		{
			name:    "null xhttp path does not hide host and nested reuse siblings",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"none","xhttpSettings":{"path":null,"host":"cdn.example.com","extra":{"xmux":{"maxConnections":"4"}}}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"none\",\"xhttpSettings\":{\"path\":null,\"host\":\"cdn.example.com\",\"extra\":{\"xmux\":{\"maxConnections\":\"4\"}}}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				require.Nil(t, download.Path)
				require.NotNil(t, download.Host)
				require.Equal(t, "cdn.example.com", *download.Host)
				require.NotNil(t, download.ReuseSettings)
				require.Equal(t, "4", download.ReuseSettings.MaxConnections)
			},
		},
		{
			name:    "null xhttp host and extra do not hide a valid path sibling",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"none","xhttpSettings":{"path":"/download","host":null,"extra":null}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"none\",\"xhttpSettings\":{\"path\":\"/download\",\"host\":null,\"extra\":null}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				require.NotNil(t, download.Path)
				require.Equal(t, "/download", *download.Path)
				require.Nil(t, download.Host)
				require.Nil(t, download.ReuseSettings)
			},
		},
		{
			name:    "null tls scalar slice and ech leaves do not hide valid tls siblings",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"tls","tlsSettings":{"serverName":null,"allowInsecure":null,"alpn":null,"fingerprint":"chrome","echConfigList":null,"echQuery":"query.example","echDNS":null,"echForceQuery":"full"}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"tls\",\"tlsSettings\":{\"serverName\":null,\"allowInsecure\":null,\"alpn\":null,\"fingerprint\":\"chrome\",\"echConfigList\":null,\"echQuery\":\"query.example\",\"echDNS\":null,\"echForceQuery\":\"full\"}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				tls := got.Transport.XHTTP.DownloadSettings.TLS
				require.NotNil(t, tls)
				require.Empty(t, tls.ServerName)
				require.False(t, tls.InsecureSkipVerify)
				require.Nil(t, tls.ALPN)
				require.Equal(t, "chrome", tls.ClientFingerprint)
				require.NotNil(t, tls.ECH)
				require.Nil(t, tls.ECH.Config)
				require.Equal(t, "query.example", tls.ECH.QueryServerName)
				require.Empty(t, tls.ECH.DNS)
				require.Equal(t, "full", tls.ECH.ForceQuery)
			},
		},
		{
			name:    "null array element is not promoted as an empty string",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"tls","tlsSettings":{"serverName":"download.example.com","alpn":["h2",null]}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"tls\",\"tlsSettings\":{\"serverName\":\"download.example.com\",\"alpn\":[\"h2\",null]}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				tls := got.Transport.XHTTP.DownloadSettings.TLS
				require.NotNil(t, tls)
				require.Equal(t, "download.example.com", tls.ServerName)
				require.Nil(t, tls.ALPN)
			},
		},
		{
			name:    "null reality public key does not hide a valid short id sibling",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"reality","realitySettings":{"publicKey":null,"shortId":"08"}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"reality\",\"realitySettings\":{\"publicKey\":null,\"shortId\":\"08\"}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				reality := got.Transport.XHTTP.DownloadSettings.TLS.Reality
				require.NotNil(t, reality)
				require.Empty(t, reality.PublicKey)
				require.Equal(t, "08", reality.ShortID)
			},
		},
		{
			name:    "null reality short id does not hide a valid public key sibling",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"reality","realitySettings":{"publicKey":"public-key","shortId":null}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"reality\",\"realitySettings\":{\"publicKey\":\"public-key\",\"shortId\":null}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				reality := got.Transport.XHTTP.DownloadSettings.TLS.Reality
				require.NotNil(t, reality)
				require.Equal(t, "public-key", reality.PublicKey)
				require.Empty(t, reality.ShortID)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := requireVMessAEADXHTTPExtraPreserved(t, tc.extra, tc.wantRaw)
			tc.requireTyped(t, got)
		})
	}
}

func TestParseVMessAEADXHTTPStringContainingNullIsConsumed(t *testing.T) {
	got, source := parseVMessAEADXHTTPDownloadExtra(t, `{"xmux":{"maxConcurrency":"null"}}`)

	require.NotNil(t, got.Transport.XHTTP.ReuseSettings)
	require.Equal(t, "null", got.Transport.XHTTP.ReuseSettings.MaxConcurrency)
	require.Empty(t, got.Raw)
	require.Empty(t, source.Warnings)
}

func TestParseVMessAEADXHTTPCaseInsensitiveWireFieldsRemainTyped(t *testing.T) {
	extra := `{"XMUX":{"MAXCONCURRENCY":"8-16"},"DOWNLOADSETTINGS":{"ADDRESS":"download.example.com","NETWORK":"xhttp","SECURITY":"none","XHTTPSETTINGS":{"PATH":"/download"}}}`

	got, source := parseVMessAEADXHTTPDownloadExtra(t, extra)

	require.NotNil(t, got.Transport.XHTTP.ReuseSettings)
	require.Equal(t, "8-16", got.Transport.XHTTP.ReuseSettings.MaxConcurrency)
	download := got.Transport.XHTTP.DownloadSettings
	require.NotNil(t, download)
	require.NotNil(t, download.Server)
	require.Equal(t, "download.example.com", *download.Server)
	require.NotNil(t, download.Path)
	require.Equal(t, "/download", *download.Path)
	require.Empty(t, got.Raw)
	require.Empty(t, source.Warnings)
}

func TestParseVMessAEADXHTTPCaseVariantsKeepWholeStructDecodeOrder(t *testing.T) {
	extra := `{"xmux":{"maxConcurrency":"first"},"XMUX":{"MAXCONCURRENCY":"last"}}`

	got, source := parseVMessAEADXHTTPDownloadExtra(t, extra)

	require.NotNil(t, got.Transport.XHTTP.ReuseSettings)
	require.Equal(t, "last", got.Transport.XHTTP.ReuseSettings.MaxConcurrency)
	require.Empty(t, got.Raw)
	require.Empty(t, source.Warnings)
}

func TestParseVMessAEADXHTTPTypeErrorsPreserveRawAndPromoteSiblings(t *testing.T) {
	tests := []struct {
		name         string
		extra        string
		wantRaw      string
		requireTyped func(*testing.T, domain.NodeIR)
	}{
		{
			name:    "invalid download port does not block top level xmux",
			extra:   `{"xmux":{"maxConcurrency":"8-16"},"downloadSettings":{"network":"xhttp","security":"none","port":"bad"}}`,
			wantRaw: `"{\"xmux\":{\"maxConcurrency\":\"8-16\"},\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"none\",\"port\":\"bad\"}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				require.NotNil(t, got.Transport.XHTTP.ReuseSettings)
				require.Equal(t, "8-16", got.Transport.XHTTP.ReuseSettings.MaxConcurrency)
			},
		},
		{
			name:    "invalid reuse leaf does not block a valid reuse leaf",
			extra:   `{"xmux":{"maxConcurrency":"8-16","maxConnections":1}}`,
			wantRaw: `"{\"xmux\":{\"maxConcurrency\":\"8-16\",\"maxConnections\":1}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				require.NotNil(t, got.Transport.XHTTP.ReuseSettings)
				require.Equal(t, "8-16", got.Transport.XHTTP.ReuseSettings.MaxConcurrency)
				require.Empty(t, got.Transport.XHTTP.ReuseSettings.MaxConnections)
			},
		},
		{
			name:    "invalid download leaf does not block valid address and path",
			extra:   `{"downloadSettings":{"address":"download.example.com","port":"bad","network":"xhttp","security":"none","xhttpSettings":{"path":"/download"}}}`,
			wantRaw: `"{\"downloadSettings\":{\"address\":\"download.example.com\",\"port\":\"bad\",\"network\":\"xhttp\",\"security\":\"none\",\"xhttpSettings\":{\"path\":\"/download\"}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				require.NotNil(t, download.Server)
				require.Equal(t, "download.example.com", *download.Server)
				require.Nil(t, download.Port)
				require.NotNil(t, download.Path)
				require.Equal(t, "/download", *download.Path)
			},
		},
		{
			name:    "invalid tls bool does not block valid tls string siblings",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"tls","tlsSettings":{"serverName":"download.example.com","allowInsecure":"bad","fingerprint":"chrome"}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"tls\",\"tlsSettings\":{\"serverName\":\"download.example.com\",\"allowInsecure\":\"bad\",\"fingerprint\":\"chrome\"}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				tls := download.TLS
				require.NotNil(t, tls)
				require.Equal(t, "download.example.com", tls.ServerName)
				require.False(t, tls.InsecureSkipVerify)
				require.Equal(t, "chrome", tls.ClientFingerprint)
			},
		},
		{
			name:    "invalid reality leaf does not block a valid reality sibling",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"reality","realitySettings":{"publicKey":1,"shortId":"08"}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"reality\",\"realitySettings\":{\"publicKey\":1,\"shortId\":\"08\"}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				require.NotNil(t, download.TLS)
				reality := download.TLS.Reality
				require.NotNil(t, reality)
				require.Empty(t, reality.PublicKey)
				require.Equal(t, "08", reality.ShortID)
			},
		},
		{
			name:    "invalid ech leaf does not block valid tls and ech siblings",
			extra:   `{"downloadSettings":{"network":"xhttp","security":"tls","tlsSettings":{"serverName":"download.example.com","echConfigList":"bad","echQuery":"query.example"}}}`,
			wantRaw: `"{\"downloadSettings\":{\"network\":\"xhttp\",\"security\":\"tls\",\"tlsSettings\":{\"serverName\":\"download.example.com\",\"echConfigList\":\"bad\",\"echQuery\":\"query.example\"}}}"`,
			requireTyped: func(t *testing.T, got domain.NodeIR) {
				t.Helper()
				download := got.Transport.XHTTP.DownloadSettings
				require.NotNil(t, download)
				tls := download.TLS
				require.NotNil(t, tls)
				require.Equal(t, "download.example.com", tls.ServerName)
				require.NotNil(t, tls.ECH)
				require.Nil(t, tls.ECH.Config)
				require.Equal(t, "query.example", tls.ECH.QueryServerName)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := requireVMessAEADXHTTPExtraPreserved(t, tc.extra, tc.wantRaw)
			tc.requireTyped(t, got)
		})
	}
}

func TestVLESSXHTTPInvalidTypedExtraRetainsAllOrNothingPromotion(t *testing.T) {
	extra := `{"xmux":{"maxConcurrency":"8-16"},"downloadSettings":{"network":"xhttp","security":"none","port":"bad"}}`
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&type=xhttp&extra=" + url.QueryEscape(extra)

	nodes, source, err := uri.NewParser().Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].Transport)
	require.NotNil(t, nodes[0].Transport.XHTTP)
	require.Nil(t, nodes[0].Transport.XHTTP.ReuseSettings)
	require.Nil(t, nodes[0].Transport.XHTTP.DownloadSettings)
	require.Empty(t, nodes[0].Raw)
	require.Empty(t, source.Warnings)
}

func parseVMessAEADXHTTPDownloadExtra(t *testing.T, extra string) (domain.NodeIR, *domain.SourceInfo) {
	t.Helper()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?type=xhttp&extra=" + url.QueryEscape(extra)
	nodes, source, err := uri.NewParser().Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotNil(t, source)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.Equal(t, "xhttp", got.Transport.Type)
	require.NotNil(t, got.Transport.XHTTP)
	return got, source
}

func requireVMessAEADXHTTPExtraPreserved(t *testing.T, extra, wantRaw string) domain.NodeIR {
	t.Helper()
	got, source := parseVMessAEADXHTTPDownloadExtra(t, extra)
	require.Equal(t, wantRaw, string(got.Raw["uri.query.extra"]))
	require.Equal(t, []string{"uri.query.extra"}, warningFields(source.Warnings))
	require.Len(t, source.Warnings, 1)
	require.Equal(t, "parse_unknown_field", source.Warnings[0].Code)
	require.Equal(t, "field preserved in NodeIR Raw", source.Warnings[0].Message)
	require.Equal(t, "uri.query.extra", source.Warnings[0].Field)
	require.Equal(t, "uri", source.Warnings[0].Source)
	return got
}

func TestParseVMessAEADFromBase64URIList(t *testing.T) {
	p := uri.NewParser()
	legacyDoc := `{"add":"legacy.example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","ps":"legacy"}`
	legacyURI := "vmess://" + base64.StdEncoding.EncodeToString([]byte(legacyDoc))
	aeadURI := "vmess://22222222-2222-2222-2222-222222222222@example.com:8443?unsupported=value#wrapped-aead"
	wrapped := base64.StdEncoding.EncodeToString([]byte(legacyURI + "\n" + aeadURI + "\n"))

	nodes, source, err := p.ParseList(context.Background(), []byte(wrapped))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, "uri-list", source.Format)
	require.Len(t, nodes, 2)
	require.Equal(t, domain.NodeTypeVMess, nodes[0].Type)
	require.Equal(t, "legacy", nodes[0].Name)
	require.Equal(t, "legacy.example.com", nodes[0].Server)
	require.Equal(t, uint16(443), nodes[0].Port)
	require.Equal(t, domain.NodeTypeVMess, nodes[1].Type)
	require.Equal(t, "wrapped-aead", nodes[1].Name)
	require.Equal(t, "example.com", nodes[1].Server)
	require.Equal(t, uint16(8443), nodes[1].Port)
	require.Len(t, source.SourceRefs, 2)
	require.Equal(t, "vmess", source.SourceRefs[0].Name)
	require.Equal(t, "VMessAEAD / VLESS sharing link", source.SourceRefs[1].Name)
	require.Equal(t, "https://github.com/XTLS/Xray-core/discussions/716", source.SourceRefs[1].URL)
	require.Len(t, source.Warnings, 1)
	warning := source.Warnings[0]
	require.Equal(t, "parse_unknown_field", warning.Code)
	require.Equal(t, "uri.query.unsupported", warning.Field)
	require.Equal(t, "uri-list", warning.Source)
	require.NotNil(t, warning.NodeIndex)
	require.Equal(t, 1, *warning.NodeIndex)
	require.NotNil(t, warning.NodeContext)
	require.Equal(t, "uri-list", warning.NodeContext.Format)
	require.Equal(t, "wrapped-aead", warning.NodeContext.Name)
	require.Equal(t, domain.NodeTypeVMess, warning.NodeContext.Type)
	require.Equal(t, aeadURI, warning.NodeContext.RawLine)
	require.Equal(t, 2, warning.NodeContext.Line)
}

func TestParseVMessUsesProfileSpecificSourceReference(t *testing.T) {
	p := uri.NewParser()
	aead, aeadSource, err := p.Parse(context.Background(), []byte(
		"vmess://11111111-1111-1111-1111-111111111111@example.com:443",
	))
	require.NoError(t, err)
	require.Len(t, aead, 1)
	require.Len(t, aeadSource.SourceRefs, 1)
	require.Equal(t, "VMessAEAD / VLESS sharing link", aeadSource.SourceRefs[0].Name)
	require.Equal(t, "https://github.com/XTLS/Xray-core/discussions/716", aeadSource.SourceRefs[0].URL)

	doc := `{"add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111"}`
	legacy, legacySource, err := p.Parse(context.Background(), []byte(
		"vmess://"+base64.StdEncoding.EncodeToString([]byte(doc)),
	))
	require.NoError(t, err)
	require.Len(t, legacy, 1)
	require.Len(t, legacySource.SourceRefs, 1)
	require.Equal(t, "vmess", legacySource.SourceRefs[0].Name)
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

func TestParseTUICTLSCompatibilityAliases(t *testing.T) {
	p := uri.NewParser()
	for _, alias := range []string{
		"allowInsecure",
		"allow_insecure",
		"allow-insecure",
		"skip-cert-verify",
		"insecure",
	} {
		t.Run(alias, func(t *testing.T) {
			raw := "tuic://11111111-1111-1111-1111-111111111111:secret@example.com:443?security=tls&" + alias + "=1#tuic"

			nodes, source, err := p.Parse(context.Background(), []byte(raw))

			require.NoError(t, err)
			require.Len(t, nodes, 1)
			require.NotNil(t, nodes[0].TLS)
			require.True(t, nodes[0].TLS.InsecureSkipVerify)
			require.NotContains(t, nodes[0].Raw, "uri.query."+alias)
			require.Empty(t, source.Warnings)
		})
	}
}

func TestParseTUICEnablesMandatoryTLSWhenOptionsCreateTLS(t *testing.T) {
	p := uri.NewParser()
	raw := "tuic://11111111-1111-1111-1111-111111111111:secret@example.com:443?sni=sni.example.com&alpn=h3&allow_insecure=1#tuic"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotNil(t, nodes[0].TLS)
	require.True(t, nodes[0].TLS.Enabled)
	require.Equal(t, "sni.example.com", nodes[0].TLS.ServerName)
	require.Equal(t, []string{"h3"}, nodes[0].TLS.ALPN)
	require.True(t, nodes[0].TLS.InsecureSkipVerify)
	require.Empty(t, source.Warnings)
}

func TestParseTUICDisableSNIExplicitBoolean(t *testing.T) {
	p := uri.NewParser()
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "1", want: true},
		{value: "true", want: true},
		{value: "0", want: false},
		{value: "false", want: false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			raw := "tuic://11111111-1111-1111-1111-111111111111:secret@example.com:443?disable_sni=" + tc.value + "#tuic"

			nodes, source, err := p.Parse(context.Background(), []byte(raw))

			require.NoError(t, err)
			require.Len(t, nodes, 1)
			require.NotNil(t, nodes[0].TLS)
			require.Equal(t, tc.want, nodes[0].TLS.DisableSNI)
			require.NotContains(t, nodes[0].Raw, "uri.query.disable_sni")
			require.Empty(t, source.Warnings)
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

func TestParseVLESSWebSocketEarlyDataPath(t *testing.T) {
	p := uri.NewParser()
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&type=ws&path=%2Fdo%3Fed%3D2048#vless"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.Transport)
	require.Equal(t, "websocket", got.Transport.Type)
	require.Equal(t, "/do", got.Transport.Path)
	require.Equal(t, 2048, got.Transport.MaxEarlyData)
	require.Equal(t, "Sec-WebSocket-Protocol", got.Transport.EarlyDataHeaderName)
}

func TestParseVLESSWebSocketNonCanonicalEarlyDataPathStaysLiteral(t *testing.T) {
	p := uri.NewParser()
	tests := []string{
		"/do?ed=",
		"/do?ed=0",
		"/do?ed=-1",
		"/do?ed=invalid",
		"/do?ed=2048&",
		"/do?ed=2048&other=value",
		"/do?ed=2048&ed=1024",
		"/do?%65d=2048",
		"/do?ed=%2B1",
	}
	for _, path := range tests {
		t.Run(url.QueryEscape(path), func(t *testing.T) {
			raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&type=ws&path=" + url.QueryEscape(path)

			nodes, _, err := p.Parse(context.Background(), []byte(raw))

			require.NoError(t, err)
			require.Len(t, nodes, 1)
			got := nodes[0]
			require.NotNil(t, got.Transport)
			require.Equal(t, path, got.Transport.Path)
			require.Zero(t, got.Transport.MaxEarlyData)
			require.Empty(t, got.Transport.EarlyDataHeaderName)
		})
	}
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

func TestParseVLESSTCPQUICSecurityCompatibilityBoundary(t *testing.T) {
	p := uri.NewParser()

	nodes, source, err := p.Parse(context.Background(), []byte(
		"vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&type=tcp&quicSecurity=none&mode=multi&spx=%2F#vless",
	))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.NotContains(t, nodes[0].Raw, "uri.query.quicSecurity")
	require.Contains(t, nodes[0].Raw, "uri.query.mode")
	require.Contains(t, nodes[0].Raw, "uri.query.spx")
	require.Equal(t, []string{"uri.query.mode", "uri.query.spx"}, warningFields(source.Warnings))

	nodes, source, err = p.Parse(context.Background(), []byte(
		"vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&type=tcp&quicSecurity=aes-128-gcm#vless",
	))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.JSONEq(t, `"aes-128-gcm"`, string(nodes[0].Raw["uri.query.quicSecurity"]))
	require.Equal(t, []string{"uri.query.quicSecurity"}, warningFields(source.Warnings))
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
