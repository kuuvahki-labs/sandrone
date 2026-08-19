package uri_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	uriadapter "github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

const compatUUID = "11111111-1111-1111-1111-111111111111"

func TestParseShadowrocketVMessAndRenderAcrossTargets(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte("auto:" + compatUUID + "@shadowrocket-vmess.example.com:443"))
	raw := "vmess://" + payload + "?remarks=Shadowrocket%20VMess&obfs=websocket&path=%2Fshadow&obfsParam=ws.shadow.example.com&tls=1&peer=sni.shadow.example.com&allowInsecure=1&fp=safari&alpn=h2"

	nodes, source, err := uriadapter.NewParser().Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 1)
	node := nodes[0]
	require.Equal(t, domain.NodeTypeVMess, node.Type)
	require.Equal(t, "Shadowrocket VMess", node.Name)
	require.Equal(t, "shadowrocket-vmess.example.com", node.Server)
	require.Equal(t, uint16(443), node.Port)
	require.Equal(t, compatUUID, node.UUID)
	require.Equal(t, "auto", node.Cipher)
	require.Equal(t, &domain.TLSOptions{
		Enabled:            true,
		ServerName:         "sni.shadow.example.com",
		InsecureSkipVerify: true,
		ALPN:               []string{"h2"},
		ClientFingerprint:  "safari",
	}, node.TLS)
	require.Equal(t, &domain.TransportOptions{
		Type:    "websocket",
		Path:    "/shadow",
		Host:    "ws.shadow.example.com",
		Headers: map[string]string{"Host": "ws.shadow.example.com"},
	}, node.Transport)
	require.Empty(t, node.Raw)

	assertCompatRendersAcrossTargets(t, nodes, "shadowrocket-vmess.example.com", "/shadow")
}

func TestParseShadowrocketVLESSAndRenderAcrossTargets(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte("none:" + compatUUID + "@shadowrocket-vless.example.com:443"))
	raw := "vless://" + payload + "?remarks=Shadowrocket%20VLESS&tls=1&obfs=websocket&obfsParam=ws.shadow.example.com&path=%2Fshadow&xtls=2"

	nodes, source, err := uriadapter.NewParser().Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 1)
	node := nodes[0]
	require.Equal(t, domain.NodeTypeVLESS, node.Type)
	require.Equal(t, "Shadowrocket VLESS", node.Name)
	require.Equal(t, "shadowrocket-vless.example.com", node.Server)
	require.Equal(t, uint16(443), node.Port)
	require.Equal(t, compatUUID, node.UUID)
	require.Equal(t, "none", node.Encryption)
	require.Equal(t, "xtls-rprx-vision", node.Flow)
	require.Equal(t, &domain.TLSOptions{Enabled: true}, node.TLS)
	require.Equal(t, &domain.TransportOptions{
		Type:    "websocket",
		Path:    "/shadow",
		Host:    "ws.shadow.example.com",
		Headers: map[string]string{"Host": "ws.shadow.example.com"},
	}, node.Transport)
	require.Empty(t, node.Raw)

	assertCompatServiceNormalizesAndRendersVLESS(t, raw, "shadowrocket-vless.example.com", "/shadow")
}

func assertCompatServiceNormalizesAndRendersVLESS(t *testing.T, raw, server, path string) {
	t.Helper()
	svc := service.New()

	mihomoResult, err := svc.Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "uri", ToFormat: "mihomo-proxies", Content: []byte(raw),
	})
	require.NoError(t, err)
	require.Contains(t, warningCodes(mihomoResult.Report.Warnings), "node_normalized_incompatible_flow")
	var mihomoDoc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(mihomoResult.Body, &mihomoDoc))
	require.Len(t, mihomoDoc.Proxies, 1)
	require.Equal(t, server, mihomoDoc.Proxies[0]["server"])
	require.NotContains(t, mihomoDoc.Proxies[0], "flow")
	require.Equal(t, path, mihomoDoc.Proxies[0]["ws-opts"].(map[string]any)["path"])

	singBoxResult, err := svc.Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "uri", ToFormat: "sing-box-outbounds", Content: []byte(raw),
	})
	require.NoError(t, err)
	require.Contains(t, warningCodes(singBoxResult.Report.Warnings), "node_normalized_incompatible_flow")
	var singBoxDoc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(singBoxResult.Body, &singBoxDoc))
	require.Len(t, singBoxDoc.Outbounds, 1)
	require.Equal(t, server, singBoxDoc.Outbounds[0]["server"])
	require.NotContains(t, singBoxDoc.Outbounds[0], "flow")
	require.Equal(t, path, singBoxDoc.Outbounds[0]["transport"].(map[string]any)["path"])
}

func warningCodes(warnings []domain.Warning) []string {
	codes := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		codes = append(codes, warning.Code)
	}
	return codes
}

func TestVMess1IsOnlyASchemeAlias(t *testing.T) {
	doc := `{"v":"2","ps":"vmess1","add":"vmess1.example.com","port":"443","id":"` + compatUUID + `","aid":"0","net":"ws","path":"/alias","host":"cdn.example.com","tls":"tls"}`
	raw := "vmess1://" + base64.RawStdEncoding.EncodeToString([]byte(doc))

	nodes, source, err := uriadapter.NewParser().Parse(context.Background(), []byte(raw))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeVMess, nodes[0].Type)
	require.Equal(t, "vmess1.example.com", nodes[0].Server)
	require.Equal(t, "/alias", nodes[0].Transport.Path)
}

func TestPacketEncodingPacketNormalizesWithoutRegressingVMessDialects(t *testing.T) {
	parser := uriadapter.NewParser()

	t.Run("vmess aead", func(t *testing.T) {
		raw := "vmess://" + compatUUID + "@aead.example.com:443?encryption=auto&type=ws&path=%2Fws&packetEncoding=packet#aead"
		nodes, _, err := parser.Parse(context.Background(), []byte(raw))
		require.NoError(t, err)
		require.Equal(t, "packetaddr", nodes[0].PacketEncoding)
		assertCompatRendersAcrossTargets(t, nodes, "aead.example.com", "/ws")
	})

	t.Run("vless", func(t *testing.T) {
		raw := "vless://" + compatUUID + "@vless.example.com:443?encryption=none&type=ws&path=%2Fws&packetEncoding=packet#vless"
		nodes, _, err := parser.Parse(context.Background(), []byte(raw))
		require.NoError(t, err)
		require.Equal(t, "packetaddr", nodes[0].PacketEncoding)
		assertCompatRendersAcrossTargets(t, nodes, "vless.example.com", "/ws")
	})

	t.Run("base64 json", func(t *testing.T) {
		doc := `{"v":"2","ps":"json","add":"json.example.com","port":"443","id":"` + compatUUID + `","aid":"0","net":"ws","path":"/json","packetEncoding":"packet"}`
		raw := "vmess://" + base64.RawStdEncoding.EncodeToString([]byte(doc))
		nodes, _, err := parser.Parse(context.Background(), []byte(raw))
		require.NoError(t, err)
		require.Equal(t, "json.example.com", nodes[0].Server)
		require.Equal(t, "packetaddr", nodes[0].PacketEncoding)
	})
}

func TestShadowrocketAuthorityRecognitionFailsClosed(t *testing.T) {
	parser := uriadapter.NewParser()
	for _, decoded := range []string{
		"auto:not-a-uuid@example.com:443",
		"auto:" + compatUUID + ":extra@example.com:443",
		"none:" + compatUUID + "@example.com:443@attacker.example:443",
	} {
		raw := "vmess://" + base64.RawURLEncoding.EncodeToString([]byte(decoded)) + "?tls=1"
		_, _, err := parser.Parse(context.Background(), []byte(raw))
		require.Error(t, err, decoded)
	}

	wrongVLESSCipher := "vless://" + base64.RawURLEncoding.EncodeToString([]byte("auto:"+compatUUID+"@example.com:443")) + "?tls=1"
	_, _, err := parser.Parse(context.Background(), []byte(wrongVLESSCipher))
	require.Error(t, err)
}

func TestShadowrocketVMessAlterIDAliasesDoNotSilentlyConflict(t *testing.T) {
	parser := uriadapter.NewParser()
	payload := base64.RawURLEncoding.EncodeToString([]byte("auto:" + compatUUID + "@example.com:443"))

	nodes, _, err := parser.Parse(context.Background(), []byte("vmess://"+payload+"?aid=2&alterId=2"))
	require.NoError(t, err)
	require.Equal(t, 2, nodes[0].AlterID)
	require.NotContains(t, nodes[0].Raw, "uri.query.aid")
	require.NotContains(t, nodes[0].Raw, "uri.query.alterId")

	_, _, err = parser.Parse(context.Background(), []byte("vmess://"+payload+"?aid=1&alterId=2"))
	require.Error(t, err)
	require.ErrorContains(t, err, "conflicting Shadowrocket alterId aliases")
}

func TestLegacyVMessSemicolonCompatibilityIsVersionScopedAndFailClosed(t *testing.T) {
	parser := uriadapter.NewParser()
	encode := func(doc string) string {
		return "vmess://" + base64.RawStdEncoding.EncodeToString([]byte(doc))
	}

	for _, host := range []string{"cdn.example.com;socket", "cdn.example.com;/one;/two"} {
		doc := `{"v":"1","add":"example.com","port":"443","id":"` + compatUUID + `","net":"ws","host":"` + host + `"}`
		_, _, err := parser.Parse(context.Background(), []byte(encode(doc)))
		require.Error(t, err, host)
	}

	v2 := `{"v":"2","add":"example.com","port":"443","id":"` + compatUUID + `","net":"ws","host":"cdn.example.com;/not-legacy"}`
	nodes, _, err := parser.Parse(context.Background(), []byte(encode(v2)))
	require.NoError(t, err)
	require.Equal(t, "cdn.example.com;/not-legacy", nodes[0].Transport.Host)
}

func assertCompatRendersAcrossTargets(t *testing.T, nodes []domain.NodeIR, server, path string) {
	t.Helper()
	mihomoBody, mihomoReport, err := mihomo.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, mihomoReport.SuccessCount)
	var mihomoDoc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(mihomoBody, &mihomoDoc))
	require.Len(t, mihomoDoc.Proxies, 1)
	require.Equal(t, server, mihomoDoc.Proxies[0]["server"])
	wsOptions, ok := mihomoDoc.Proxies[0]["ws-opts"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, path, wsOptions["path"])

	singBoxBody, singBoxReport, err := singbox.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, singBoxReport.SuccessCount)
	var singBoxDoc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(singBoxBody, &singBoxDoc))
	require.Len(t, singBoxDoc.Outbounds, 1)
	require.Equal(t, server, singBoxDoc.Outbounds[0]["server"])
	transport, ok := singBoxDoc.Outbounds[0]["transport"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, path, transport["path"])
}
