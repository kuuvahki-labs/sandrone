package service_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceParseURISubscription(t *testing.T) {
	svc := service.New()
	result, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format: "uri-list",
		Content: []byte(strings.Join([]string{
			"ss://aes-128-gcm:secret@example.com:8388#node-a",
			"socks5://user:pass@example.com:1080#socks",
		}, "\n")),
	})
	require.NoError(t, err)
	require.Len(t, result.Nodes, 2)
}

func TestServiceParseAggregatesURIListFallbackHysteriaWarningsOnce(t *testing.T) {
	svc := service.New()
	content := `{"name":"fallback-hy","type":"hysteria","server":"fallback.example","port":8443,"tls":{"enabled":true},"hysteria":{"up":"55","down":"100"}}`

	result, err := svc.Parse(context.Background(), domain.ParseRequest{Format: "uri-list", Content: []byte(content)})

	require.NoError(t, err)
	require.Len(t, result.Nodes, 1)
	require.Equal(t, &domain.HysteriaOptions{UpMbps: 55, DownMbps: 100}, result.Nodes[0].Hysteria)
	require.Len(t, result.Nodes[0].Warnings, 2)
	require.Equal(t, []string{"parse_implicit_bandwidth_unit", "parse_implicit_bandwidth_unit"}, warningCodes(result.Report.Warnings))
}

func TestServiceParseAndRenderRegistry(t *testing.T) {
	svc := service.New()
	parsed, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format: "uri-list",
		Content: []byte(strings.Join([]string{
			"ss://aes-128-gcm:secret@example.com:8388#node-a",
			"socks5://user:pass@example.com:1080#socks",
		}, "\n")),
	})
	require.NoError(t, err)
	require.Len(t, parsed.Nodes, 2)

	for _, format := range []string{"base64", "mihomo-proxies", "shadowrocket-proxies", "sing-box-outbounds", "json-nodes", "uri-list"} {
		t.Run(format, func(t *testing.T) {
			rendered, err := svc.Render(context.Background(), domain.RenderRequest{
				Format: format,
				Nodes:  parsed.Nodes,
			})
			require.NoError(t, err)
			require.NotEmpty(t, rendered.Body)
		})
	}
}

func TestServiceCanonicalizesVMessAndVLESSUserIDsAcrossEntryPointsAndTargets(t *testing.T) {
	svc := service.New()
	rawNodes := []domain.NodeIR{
		{Name: "mapped-vmess", Type: domain.NodeTypeVMess, Server: "vmess.example", Port: 443, UUID: "123456", Cipher: "auto"},
		{Name: "mapped-vless", Type: domain.NodeTypeVLESS, Server: "vless.example", Port: 443, UUID: "a9dk23bz0", Encryption: "none"},
	}
	input, err := json.Marshal(rawNodes)
	require.NoError(t, err)

	parsed, err := svc.Parse(context.Background(), domain.ParseRequest{Format: "json-nodes", Content: input})
	require.NoError(t, err)
	require.Empty(t, parsed.Report.Warnings)
	require.Equal(t, "f8598425-92f2-5508-a071-4fc67f9040ac", parsed.Nodes[0].UUID)
	require.Equal(t, "c91481b6-fc0f-5d9e-b166-5ddf07b9c3c5", parsed.Nodes[1].UUID)

	validated, err := svc.ValidateNodes(context.Background(), domain.ParseRequest{Format: "json-nodes", Content: input})
	require.NoError(t, err)
	require.True(t, validated.OK)
	require.Equal(t, 2, validated.Counts.Valid)
	require.Empty(t, validated.Issues)
	require.Empty(t, validated.Report.Warnings)

	for _, format := range []string{"json-nodes", "mihomo-proxies", "sing-box-outbounds", "shadowrocket-proxies"} {
		t.Run(format, func(t *testing.T) {
			rendered, err := svc.Render(context.Background(), domain.RenderRequest{Format: format, Nodes: rawNodes})
			require.NoError(t, err)
			require.Empty(t, rendered.Report.Warnings)
			require.Contains(t, string(rendered.Body), "f8598425-92f2-5508-a071-4fc67f9040ac")
			require.Contains(t, string(rendered.Body), "c91481b6-fc0f-5d9e-b166-5ddf07b9c3c5")
		})
	}

	uriList, err := svc.Render(context.Background(), domain.RenderRequest{Format: "uri-list", Nodes: rawNodes})
	require.NoError(t, err)
	require.Empty(t, uriList.Report.Warnings)
	roundTrip, err := svc.Parse(context.Background(), domain.ParseRequest{Format: "uri-list", Content: uriList.Body})
	require.NoError(t, err)
	require.Equal(t, "f8598425-92f2-5508-a071-4fc67f9040ac", roundTrip.Nodes[0].UUID)
	require.Equal(t, "c91481b6-fc0f-5d9e-b166-5ddf07b9c3c5", roundTrip.Nodes[1].UUID)
	require.Equal(t, "123456", rawNodes[0].UUID)
	require.Equal(t, "a9dk23bz0", rawNodes[1].UUID)
}

func TestServiceDefaultsRealityClientFingerprintAcrossEntryPointsAndTargets(t *testing.T) {
	svc := service.New()
	const raw = "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=reality&pbk=public-key&sid=01&type=tcp#reality"

	parsed, err := svc.Parse(context.Background(), domain.ParseRequest{Format: "uri-list", Content: []byte(raw)})
	require.NoError(t, err)
	require.Len(t, parsed.Nodes, 1)
	require.Equal(t, "chrome", parsed.Nodes[0].TLS.ClientFingerprint)

	validated, err := svc.ValidateNodes(context.Background(), domain.ParseRequest{Format: "uri-list", Content: []byte(raw)})
	require.NoError(t, err)
	require.True(t, validated.OK)

	rawNode := domain.NodeIR{
		Name: "reality", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none",
		TLS: &domain.TLSOptions{
			Enabled: true, ServerName: "example.com",
			Reality: &domain.RealityOptions{Enabled: true, PublicKey: "public-key", ShortID: "01"},
		},
	}

	jsonResult, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "json-nodes",
		Nodes:  []domain.NodeIR{rawNode},
	})
	require.NoError(t, err)
	var jsonNodes []domain.NodeIR
	require.NoError(t, json.Unmarshal(jsonResult.Body, &jsonNodes))
	require.Equal(t, "chrome", jsonNodes[0].TLS.ClientFingerprint)

	mihomoResult, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "mihomo-proxies",
		Nodes:  []domain.NodeIR{rawNode},
	})
	require.NoError(t, err)
	require.Contains(t, string(mihomoResult.Body), "client-fingerprint: chrome")

	singBoxResult, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "sing-box-outbounds",
		Nodes:  []domain.NodeIR{rawNode},
	})
	require.NoError(t, err)
	var singBoxDoc struct {
		Outbounds []struct {
			TLS struct {
				UTLS map[string]any `json:"utls"`
			} `json:"tls"`
		} `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(singBoxResult.Body, &singBoxDoc))
	require.Equal(t, true, singBoxDoc.Outbounds[0].TLS.UTLS["enabled"])
	require.Equal(t, "chrome", singBoxDoc.Outbounds[0].TLS.UTLS["fingerprint"])

	uriResult, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "uri-list",
		Nodes:  []domain.NodeIR{rawNode},
	})
	require.NoError(t, err)
	require.Contains(t, string(uriResult.Body), "fp=chrome")
	require.Empty(t, rawNode.TLS.ClientFingerprint)
}

func TestServiceDropsUnsupportedTLSClientFingerprintAcrossInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		format  string
		content string
	}{
		{
			name:   "uri list",
			format: "uri-list",
			content: strings.Join([]string{
				"vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=tls&fp=chrome#valid",
				"vless://22222222-2222-2222-2222-222222222222@example.com:443?encryption=none&security=tls&fp=unsafe#unsupported",
			}, "\n"),
		},
		{
			name:   "mihomo",
			format: "mihomo",
			content: `proxies:
  - {name: valid, type: vless, server: example.com, port: 443, uuid: 11111111-1111-1111-1111-111111111111, tls: true, client-fingerprint: chrome}
  - {name: unsupported, type: vless, server: example.com, port: 443, uuid: 22222222-2222-2222-2222-222222222222, tls: true, client-fingerprint: unsafe}`,
		},
		{
			name:   "sing-box",
			format: "sing-box",
			content: `{"outbounds":[
  {"tag":"valid","type":"vless","server":"example.com","server_port":443,"uuid":"11111111-1111-1111-1111-111111111111","tls":{"enabled":true,"utls":{"enabled":true,"fingerprint":"chrome"}}},
  {"tag":"unsupported","type":"vless","server":"example.com","server_port":443,"uuid":"22222222-2222-2222-2222-222222222222","tls":{"enabled":true,"utls":{"enabled":true,"fingerprint":"unsafe"}}}
]}`,
		},
		{
			name:   "json nodes",
			format: "json-nodes",
			content: `[
  {"name":"valid","type":"vless","server":"example.com","port":443,"uuid":"11111111-1111-1111-1111-111111111111","tls":{"enabled":true,"client_fingerprint":"chrome"}},
  {"name":"unsupported","type":"vless","server":"example.com","port":443,"uuid":"22222222-2222-2222-2222-222222222222","tls":{"enabled":true,"client_fingerprint":"unsafe"}}
]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := service.New().Parse(context.Background(), domain.ParseRequest{
				Format: tc.format, Content: []byte(tc.content),
			})

			require.NoError(t, err)
			require.Len(t, result.Nodes, 1)
			require.Equal(t, "valid", result.Nodes[0].Name)
			require.Len(t, result.Report.Warnings, 1)
			require.Equal(t, "node_validation_dropped", result.Report.Warnings[0].Code)
			require.Equal(t, "tls.client_fingerprint", result.Report.Warnings[0].Field)
		})
	}
}

func TestServicePreservesPortableTLSClientFingerprintAcrossTargets(t *testing.T) {
	t.Parallel()

	node := domain.NodeIR{
		Name: "vless", Type: domain.NodeTypeVLESS, Server: "example.com", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none",
		TLS: &domain.TLSOptions{Enabled: true, ClientFingerprint: "randomized"},
	}
	tests := []struct {
		format string
		want   string
	}{
		{format: "json-nodes", want: `"client_fingerprint": "randomized"`},
		{format: "mihomo-proxies", want: "client-fingerprint: randomized"},
		{format: "sing-box-outbounds", want: `"fingerprint": "randomized"`},
		{format: "uri-list", want: "fp=randomized"},
	}

	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			t.Parallel()
			result, err := service.New().Render(context.Background(), domain.RenderRequest{
				Format: tc.format, Nodes: []domain.NodeIR{node},
			})
			require.NoError(t, err)
			require.Contains(t, string(result.Body), tc.want)
		})
	}
}

func TestServicePreservesDisabledPacketEncodingAcrossTargets(t *testing.T) {
	svc := service.New()
	content := []byte("vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&packetEncoding=none#disabled-packet-encoding")

	parsed, err := svc.Parse(context.Background(), domain.ParseRequest{Format: "uri-list", Content: content})
	require.NoError(t, err)
	require.Len(t, parsed.Nodes, 1)
	require.Equal(t, "none", parsed.Nodes[0].PacketEncoding)

	singBoxResult, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "sing-box-outbounds",
		Nodes:  parsed.Nodes,
	})
	require.NoError(t, err)
	require.Empty(t, singBoxResult.Report.Warnings)
	var singBoxDoc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(singBoxResult.Body, &singBoxDoc))
	require.Len(t, singBoxDoc.Outbounds, 1)
	packetEncoding, exists := singBoxDoc.Outbounds[0]["packet_encoding"]
	require.True(t, exists)
	require.Equal(t, "", packetEncoding)

	mihomoResult, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "mihomo-proxies",
		Nodes:  parsed.Nodes,
	})
	require.NoError(t, err)
	require.Empty(t, mihomoResult.Report.Warnings)
	require.Contains(t, string(mihomoResult.Body), "packet-encoding: none")
}

func TestServicePreservesURIWebSocketEarlyDataAcrossTargets(t *testing.T) {
	svc := service.New()
	content := []byte("vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&type=ws&path=%2Fws&ed=2560&eh=Sec-WebSocket-Protocol#early-data")

	parsed, err := svc.Parse(context.Background(), domain.ParseRequest{Format: "uri-list", Content: content})

	require.NoError(t, err)
	require.Len(t, parsed.Nodes, 1)
	require.Empty(t, parsed.Report.Warnings)
	require.Equal(t, 2560, parsed.Nodes[0].Transport.MaxEarlyData)
	require.Equal(t, "Sec-WebSocket-Protocol", parsed.Nodes[0].Transport.EarlyDataHeaderName)

	tests := []struct {
		format   string
		contains []string
	}{
		{
			format: "mihomo-proxies",
			contains: []string{
				"max-early-data: 2560",
				"early-data-header-name: Sec-WebSocket-Protocol",
			},
		},
		{
			format: "sing-box-outbounds",
			contains: []string{
				`"max_early_data": 2560`,
				`"early_data_header_name": "Sec-WebSocket-Protocol"`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			rendered, err := svc.Render(context.Background(), domain.RenderRequest{Format: tc.format, Nodes: parsed.Nodes})

			require.NoError(t, err)
			require.Empty(t, rendered.Report.Warnings)
			for _, want := range tc.contains {
				require.Contains(t, string(rendered.Body), want)
			}
		})
	}
}

func TestServicePreservesVMessTCPHTTPHeaderAndRejectsUnsupportedTarget(t *testing.T) {
	svc := service.New()
	doc := `{"v":"2","ps":"payload-name","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","net":"tcp","type":"http","method":"GET","host":"cdn.example.com","path":"/api"}`
	content := []byte("vmess://" + base64.StdEncoding.EncodeToString([]byte(doc)) + "#fragment-name")

	parsed, err := svc.Parse(context.Background(), domain.ParseRequest{Format: "uri-list", Content: content})

	require.NoError(t, err)
	require.Empty(t, parsed.Report.Warnings)
	require.Len(t, parsed.Nodes, 1)
	require.Equal(t, "fragment-name", parsed.Nodes[0].Name)
	require.Equal(t, "tcp", parsed.Nodes[0].Transport.Type)
	require.Equal(t, "http", parsed.Nodes[0].Transport.HeaderType)

	mihomoResult, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "mihomo-proxies",
		Nodes:  parsed.Nodes,
	})
	require.NoError(t, err)
	require.Empty(t, mihomoResult.Report.Warnings)
	require.Contains(t, string(mihomoResult.Body), "network: http")
	require.Contains(t, string(mihomoResult.Body), "http-opts:")

	singBoxResult, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "sing-box-outbounds",
		Nodes:  parsed.Nodes,
	})
	require.Error(t, err)
	require.Nil(t, singBoxResult)
	require.True(t, domain.IsCode(err, domain.CodeRenderFailed), "unexpected error: %v", err)
	require.Contains(t, err.Error(), "TCP HTTP header obfuscation")
}

func TestServiceConvertRunsParseAndRender(t *testing.T) {
	targets := []string{}
	svc := service.New(service.WithProcessor(func(r *processor.Registry) {
		r.RegisterNode("record_target", func(domain.ProcessorSpec) (domain.NodeProcessor, error) {
			return targetRecorder{targets: &targets}, nil
		})
	}))

	result, err := svc.Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "uri-list",
		ToFormat:   "json-nodes",
		Content: []byte(strings.Join([]string{
			"ss://aes-128-gcm:secret@example.com:8388#hk-a",
			"ss://aes-128-gcm:secret@example.com:8388#us-a",
		}, "\n")),
		ParseProcessors: []domain.ProcessorSpec{
			{Type: "filter", Params: params(t, map[string]any{"action": "keep", "field": "name", "match": "regex", "pattern": "^hk-"})},
			{Type: "record_target"},
		},
		RenderProcessors: []domain.ProcessorSpec{
			{Type: "record_target"},
			{Type: "rename", Params: params(t, map[string]any{"mode": "prefix", "value": "[X]"})},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "application/json", result.ContentType)
	require.Equal(t, "convert", result.Report.Kind)
	require.Equal(t, []string{"json-nodes", "json-nodes"}, targets)
	require.Contains(t, string(result.Body), "[X]hk-a")
	require.NotContains(t, string(result.Body), "us-a")
	require.NotEmpty(t, result.Report.SourceRefs)
}

func TestServiceConvertVMessWithoutCipherUsesClientDefaults(t *testing.T) {
	svc := service.New()
	content := []byte(`[{
  "name": "vmess-default",
  "type": "vmess",
  "server": "vmess.example.com",
  "port": 443,
  "uuid": "11111111-1111-1111-1111-111111111111"
}]`)

	for _, format := range []string{
		"mihomo-proxies",
		"sing-box-outbounds",
		"shadowrocket-proxies",
		"uri-list",
		"base64",
	} {
		t.Run(format, func(t *testing.T) {
			result, err := svc.Convert(context.Background(), domain.ConvertRequest{
				FromFormat: "json-nodes",
				ToFormat:   format,
				Content:    content,
			})
			require.NoError(t, err)

			switch format {
			case "mihomo-proxies":
				require.Contains(t, string(result.Body), "cipher: auto")
			case "sing-box-outbounds":
				var doc struct {
					Outbounds []map[string]any `json:"outbounds"`
				}
				require.NoError(t, json.Unmarshal(result.Body, &doc))
				require.Len(t, doc.Outbounds, 1)
				require.Equal(t, "auto", doc.Outbounds[0]["security"])
			case "shadowrocket-proxies":
				require.Contains(t, string(result.Body), "method=auto")
			case "uri-list", "base64":
				parsed, err := svc.Parse(context.Background(), domain.ParseRequest{
					Format:  format,
					Content: result.Body,
				})
				require.NoError(t, err)
				require.Len(t, parsed.Nodes, 1)
				require.Equal(t, "auto", parsed.Nodes[0].Cipher)
			}
		})
	}
}

func TestServiceConvertVLESSSecurityNoneToSingBoxOmitsTLS(t *testing.T) {
	svc := service.New()

	result, err := svc.Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "uri-list",
		ToFormat:   "sing-box-outbounds",
		Content: []byte(
			"vless://00000000-0000-0000-0000-000000000000@example.com:443" +
				"?security=none&type=tcp&sni=ignored.example&allowInsecure=1#plain-vless",
		),
	})

	require.NoError(t, err)
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(result.Body, &doc))
	require.Len(t, doc.Outbounds, 1)
	require.Equal(t, "vless", doc.Outbounds[0]["type"])
	require.NotContains(t, doc.Outbounds[0], "tls")
}

func TestServiceConvertPreservesMihomoParserWarningContext(t *testing.T) {
	svc := service.New()

	result, err := svc.Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "mihomo",
		ToFormat:   "json-nodes",
		Content: []byte(`
proxies:
  - name: ss
    type: ss
    server: example.com
    port: 8388
    cipher: aes-128-gcm
    password: secret
    private-thing: value
`),
	})

	require.NoError(t, err)
	require.Equal(t, "convert", result.Report.Kind)
	require.Len(t, result.Report.Warnings, 1)
	warning := result.Report.Warnings[0]
	require.Equal(t, "parse_unknown_field", warning.Code)
	require.Equal(t, "mihomo.private-thing", warning.Field)
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
}

func TestServiceConvertMihomoWebSocketToSingBoxKeepsTransportSeparateFromNetwork(t *testing.T) {
	svc := service.New()

	result, err := svc.Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "mihomo",
		ToFormat:   "sing-box-outbounds",
		Content: []byte(`
proxies:
  - name: trojan-ws
    type: trojan
    server: example.com
    port: 443
    password: secret
    network: ws
    ws-opts:
      path: /ws
    tls: true
`),
	})

	require.NoError(t, err)
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(result.Body, &doc))
	require.Len(t, doc.Outbounds, 1)
	require.NotContains(t, doc.Outbounds[0], "network")
	require.Equal(t, "ws", doc.Outbounds[0]["transport"].(map[string]any)["type"])
}

func TestServiceConvertRemoteAutoDetectsBase64Subscription(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "sandrone-test", r.UserAgent())
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	svc := service.New()
	result, err := svc.Convert(context.Background(), domain.ConvertRequest{
		ToFormat: "json-nodes",
		Remote: &domain.RemoteInput{
			URL:       server.URL,
			UserAgent: "sandrone-test",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "application/json", result.ContentType)
	require.Contains(t, string(result.Body), "remote-node")
	require.Equal(t, "convert", result.Report.Kind)
	require.NotEmpty(t, result.Report.SourceRefs)
	require.Equal(t, "remote", result.Report.SourceRefs[0].Kind)
	require.Equal(t, server.URL, result.Report.SourceRefs[0].URL)
	require.Contains(t, result.Report.SourceRefs[0].Note, "status=200")
	require.Contains(t, result.Report.SourceRefs[0].Note, "sha256=")
}

func TestServiceConvertPreservesExplicitLegacyLookingUserAgent(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "sandrone/0", r.UserAgent())
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	svc := service.New()
	_, err := svc.Convert(context.Background(), domain.ConvertRequest{
		ToFormat: "json-nodes",
		Remote: &domain.RemoteInput{
			URL:       server.URL,
			UserAgent: "sandrone/0",
		},
	})

	require.NoError(t, err)
}

func TestServiceConvertResolvesEmptyUserAgentBeforeRemoteCacheLookup(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, buildinfo.UserAgent(), r.UserAgent())
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	putProjectSettings(t, svc, context.Background(), func(update *domain.SettingsUpdate) {
		update.RemoteDefaults.UserAgent = ""
		update.CacheDefaults.RemoteFetchTTLSeconds = 60
	})

	for range 2 {
		_, err := svc.Convert(context.Background(), domain.ConvertRequest{
			ToFormat: "json-nodes",
			Remote:   &domain.RemoteInput{URL: server.URL},
		})
		require.NoError(t, err)
	}

	require.Equal(t, 1, calls)
}

func TestServiceConvertRemoteInputUsesRuntimeDefaultsAndLocalOverride(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	userAgents := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgents = append(userAgents, r.UserAgent())
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	putProjectSettings(t, svc, context.Background(), func(update *domain.SettingsUpdate) {
		update.RemoteDefaults = domain.RemoteDefaults{
			UserAgent: "Sandrone Global",
			TimeoutMS: 8000,
		}
	})

	_, err := svc.Convert(context.Background(), domain.ConvertRequest{
		ToFormat: "json-nodes",
		Remote:   &domain.RemoteInput{URL: server.URL},
	})
	require.NoError(t, err)
	_, err = svc.Convert(context.Background(), domain.ConvertRequest{
		ToFormat: "json-nodes",
		Remote: &domain.RemoteInput{
			URL:       server.URL,
			UserAgent: "Sandrone Local",
		},
	})
	require.NoError(t, err)

	require.Equal(t, []string{"Sandrone Global", "Sandrone Local"}, userAgents)
}

func TestServiceConvertRemoteInputUsesExplicitCacheTTL(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-1"))
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	first, err := svc.Convert(context.Background(), domain.ConvertRequest{
		ToFormat: "json-nodes",
		Remote:   &domain.RemoteInput{URL: server.URL, CacheTTLSeconds: 60},
	})
	require.NoError(t, err)
	body = base64.StdEncoding.EncodeToString([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-2"))
	second, err := svc.Convert(context.Background(), domain.ConvertRequest{
		ToFormat: "json-nodes",
		Remote:   &domain.RemoteInput{URL: server.URL, CacheTTLSeconds: 60},
	})
	require.NoError(t, err)

	require.Equal(t, 1, calls)
	require.Contains(t, string(first.Body), "remote-1")
	require.Contains(t, string(second.Body), "remote-1")
	require.Contains(t, second.Report.SourceRefs[0].Note, "cache_hit=true")
}

func TestServiceRemoteFetchCacheUsesRuntimeDefaultAndRequestIdentity(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte("ss://aes-128-gcm:secret@example.com:8388#remote"))
	userAgents := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgents = append(userAgents, r.UserAgent())
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	putProjectSettings(t, svc, context.Background(), func(update *domain.SettingsUpdate) {
		update.RemoteDefaults = domain.RemoteDefaults{
			UserAgent: "Sandrone Global",
			TimeoutMS: 8000,
		}
		update.CacheDefaults = domain.CacheDefaults{
			RemoteFetchTTLSeconds:         60,
			SubscriptionTrafficTTLSeconds: 60,
		}
	})

	for i := 0; i < 2; i++ {
		_, err := svc.Convert(context.Background(), domain.ConvertRequest{
			ToFormat: "json-nodes",
			Remote:   &domain.RemoteInput{URL: server.URL},
		})
		require.NoError(t, err)
	}
	_, err := svc.Convert(context.Background(), domain.ConvertRequest{
		ToFormat: "json-nodes",
		Remote:   &domain.RemoteInput{URL: server.URL, UserAgent: "Sandrone Local"},
	})
	require.NoError(t, err)

	require.Equal(t, []string{"Sandrone Global", "Sandrone Local"}, userAgents)
}

func TestServiceParseRemoteAutoDetectsURIList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Join([]string{
			"ss://aes-128-gcm:secret@example.com:8388#remote-ss",
			"socks5://user:pass@example.com:1080#remote-socks",
		}, "\n")))
	}))
	defer server.Close()

	svc := service.New()
	result, err := svc.Parse(context.Background(), domain.ParseRequest{
		Remote: &domain.RemoteInput{URL: server.URL},
	})

	require.NoError(t, err)
	require.NotNil(t, result.Source)
	require.Equal(t, "uri-list", result.Source.Format)
	require.Len(t, result.Nodes, 2)
	require.Equal(t, domain.NodeTypeShadowsocks, result.Nodes[0].Type)
	require.Equal(t, domain.NodeTypeSOCKS, result.Nodes[1].Type)
}

func TestServiceParseRemoteAutoDetectPrefersSingBoxDocument(t *testing.T) {
	body := []byte(`{
  "log": {
    "level": "info"
  },
  "outbounds": [
    {
      "type": "vless",
      "tag": "real-vless",
      "server": "example.com",
      "server_port": 443,
      "uuid": "11111111-1111-1111-1111-111111111111"
    },
    { "type": "direct", "tag": "direct" }
  ]
}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := service.New()
	result, err := svc.Parse(context.Background(), domain.ParseRequest{
		Remote: &domain.RemoteInput{URL: server.URL},
	})

	require.NoError(t, err)
	require.NotNil(t, result.Source)
	require.Equal(t, "sing-box", result.Source.Format)
	require.Len(t, result.Nodes, 1)
	require.Equal(t, domain.NodeTypeVLESS, result.Nodes[0].Type)
	require.Equal(t, "real-vless", result.Nodes[0].Name)
}

func TestServiceParseRemoteAutoDetectPrefersMihomoDocument(t *testing.T) {
	body := []byte(`
proxies:
  - name: real-ss
    type: ss
    server: example.com
    port: 8388
    cipher: aes-128-gcm
    password: secret
`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := service.New()
	result, err := svc.Parse(context.Background(), domain.ParseRequest{
		Remote: &domain.RemoteInput{URL: server.URL},
	})

	require.NoError(t, err)
	require.NotNil(t, result.Source)
	require.Equal(t, "mihomo", result.Source.Format)
	require.Len(t, result.Nodes, 1)
	require.Equal(t, domain.NodeTypeShadowsocks, result.Nodes[0].Type)
	require.Equal(t, "real-ss", result.Nodes[0].Name)
	require.Equal(t, "example.com", result.Nodes[0].Server)
}

func TestServiceParseRemoteExplicitFormatDoesNotFallback(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	svc := service.New()
	_, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format: "mihomo",
		Remote: &domain.RemoteInput{URL: server.URL},
	})

	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "could not detect subscription format"))
}

func TestServiceParseRemoteExplicitFormatPreservesParserFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	}))
	defer server.Close()

	svc := service.New()
	result, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format: "URI-LIST",
		Remote: &domain.RemoteInput{URL: server.URL},
	})

	require.NoError(t, err)
	require.NotNil(t, result.Source)
	require.Equal(t, "uri-list", result.Source.Format)
	require.NotEmpty(t, result.Source.SourceRefs)
	require.Equal(t, "remote", result.Source.SourceRefs[0].Kind)
	require.Equal(t, server.URL, result.Source.SourceRefs[0].URL)
}

func TestServiceParseRemoteAutoUnwrapsBoundedSubscriptionEnvelopes(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantFormat string
		wantName   string
	}{
		{
			name: "folded base64 uri list",
			body: func() string {
				encoded := base64.StdEncoding.EncodeToString([]byte("ss://aes-128-gcm:secret@example.com:8388#folded"))
				return encoded[:20] + "\r\n" + encoded[20:]
			}(),
			wantFormat: "uri-list",
			wantName:   "folded",
		},
		{
			name:       "base64 mihomo document",
			body:       base64.StdEncoding.EncodeToString([]byte("proxies:\n  - name: wrapped-mihomo\n    type: ss\n    server: example.com\n    port: 8388\n    cipher: aes-128-gcm\n    password: secret\n")),
			wantFormat: "mihomo",
			wantName:   "wrapped-mihomo",
		},
		{
			name:       "raw url base64 sing-box document",
			body:       base64.RawURLEncoding.EncodeToString([]byte(`{"outbounds":[{"type":"socks","tag":"wrapped-sing-box","server":"example.com","server_port":1080}]}`)),
			wantFormat: "sing-box",
			wantName:   "wrapped-sing-box",
		},
		{
			name:       "whole body percent encoded",
			body:       url.PathEscape("ss://aes-128-gcm:secret@example.com:8388#percent"),
			wantFormat: "uri-list",
			wantName:   "percent",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			result, err := service.New().Parse(context.Background(), domain.ParseRequest{
				Remote: &domain.RemoteInput{URL: server.URL},
			})

			require.NoError(t, err)
			require.NotNil(t, result.Source)
			require.Equal(t, tc.wantFormat, result.Source.Format)
			require.Len(t, result.Nodes, 1)
			require.Equal(t, tc.wantName, result.Nodes[0].Name)
		})
	}
}

func TestServiceParseRemoteAutoDoesNotRecursivelyDecodeOrExposeBody(t *testing.T) {
	uri := "ss://aes-128-gcm:credential-must-not-leak@example.com:8388#private"
	doubleWrapped := base64.StdEncoding.EncodeToString([]byte(base64.StdEncoding.EncodeToString([]byte(uri))))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(doubleWrapped))
	}))
	defer server.Close()

	_, err := service.New().Parse(context.Background(), domain.ParseRequest{
		Remote: &domain.RemoteInput{URL: server.URL},
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
	require.NotContains(t, err.Error(), doubleWrapped)
	require.NotContains(t, err.Error(), "credential-must-not-leak")
}

func TestServiceParseRemoteAutoRejectsNonCanonicalOrTrailingBase64(t *testing.T) {
	uri := "ss://aes-128-gcm:secret@example.com:8388#strict"
	canonical := base64.StdEncoding.EncodeToString([]byte(uri))
	padding := strings.IndexByte(canonical, '=')
	require.Positive(t, padding)
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	last := strings.IndexByte(alphabet, canonical[padding-1])
	require.NotEqual(t, -1, last)
	nonCanonical := canonical[:padding-1] + string(alphabet[last+1]) + canonical[padding:]

	for _, body := range []string{nonCanonical, canonical + "!junk"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))

		_, err := service.New().Parse(context.Background(), domain.ParseRequest{
			Remote: &domain.RemoteInput{URL: server.URL},
		})
		server.Close()

		require.Error(t, err)
		require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
	}
}

func TestServiceParseRemoteExplicitFormatDoesNotUnwrapStructuredEnvelope(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte("proxies:\n  - name: wrapped-mihomo\n    type: ss\n    server: example.com\n    port: 8388\n    cipher: aes-128-gcm\n    password: secret\n"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	_, err := service.New().Parse(context.Background(), domain.ParseRequest{
		Format: "mihomo",
		Remote: &domain.RemoteInput{URL: server.URL},
	})

	require.Error(t, err)
}

func TestServiceParseRemoteAutoDetectAggregatesCandidateErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not a subscription"))
	}))
	defer server.Close()

	svc := service.New()
	_, err := svc.Parse(context.Background(), domain.ParseRequest{
		Remote: &domain.RemoteInput{URL: server.URL},
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
	require.Contains(t, err.Error(), "could not detect subscription format")
	require.Contains(t, err.Error(), "mihomo:")
	require.Contains(t, err.Error(), "sing-box:")
}

func TestServiceParseRejectsContentAndRemoteTogether(t *testing.T) {
	svc := service.New()
	_, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format:  "uri-list",
		Content: []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
		Remote:  &domain.RemoteInput{URL: "https://example.com/sub"},
	})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
	require.Contains(t, err.Error(), "content and remote input are mutually exclusive")
}
func TestServiceConvertRejectsUnsupportedFormats(t *testing.T) {
	svc := service.New()

	_, err := svc.Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "bad",
		ToFormat:   "json-nodes",
		Content:    []byte("x"),
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
	require.Contains(t, err.Error(), "unsupported parse format")

	_, err = svc.Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "uri-list",
		ToFormat:   "bad",
		Content:    []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
	})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
	require.Contains(t, err.Error(), "unsupported render format")
}
func TestServiceParsePlatformFormats(t *testing.T) {
	svc := service.New()
	mihomoParsed, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format: "mihomo",
		Content: []byte(`
proxies:
  - name: vless
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
`),
	})
	require.NoError(t, err)
	require.Len(t, mihomoParsed.Nodes, 1)
	require.Equal(t, domain.NodeTypeVLESS, mihomoParsed.Nodes[0].Type)

	singBoxParsed, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format: "sing-box",
		Content: []byte(`{
  "outbounds": [
    {
      "type": "tuic",
      "tag": "tuic",
      "server": "example.com",
      "server_port": 443,
      "uuid": "11111111-1111-1111-1111-111111111111",
      "password": "secret",
      "tls": {"enabled": true}
    }
  ]
}`),
	})
	require.NoError(t, err)
	require.Len(t, singBoxParsed.Nodes, 1)
	require.Equal(t, domain.NodeTypeTUIC, singBoxParsed.Nodes[0].Type)
}

func TestServiceParseNormalizesIncompatibleVLESSVisionFlow(t *testing.T) {
	svc := service.New()
	tests := []struct {
		name    string
		format  string
		content string
	}{
		{
			name:   "uri",
			format: "uri",
			content: "vless://11111111-1111-1111-1111-111111111111@example.com:443" +
				"?encryption=none&security=tls&flow=xtls-rprx-vision&type=ws&path=%2Fws#vless",
		},
		{
			name:   "mihomo",
			format: "mihomo",
			content: `
proxies:
  - name: vless
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    flow: xtls-rprx-vision
    network: ws
    ws-opts:
      path: /ws
`,
		},
		{
			name:   "sing-box",
			format: "sing-box",
			content: `{
  "outbounds": [{
    "type": "vless",
    "tag": "vless",
    "server": "example.com",
    "server_port": 443,
    "uuid": "11111111-1111-1111-1111-111111111111",
    "flow": "xtls-rprx-vision",
    "transport": {"type": "ws", "path": "/ws"}
  }]
}`,
		},
		{
			name:   "json nodes",
			format: "json-nodes",
			content: `[{
  "name": "vless",
  "type": "vless",
  "server": "example.com",
  "port": 443,
  "uuid": "11111111-1111-1111-1111-111111111111",
  "flow": "xtls-rprx-vision",
  "transport": {"type": "websocket", "path": "/ws"}
}]`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.Parse(context.Background(), domain.ParseRequest{
				Format:  tc.format,
				Content: []byte(tc.content),
			})

			require.NoError(t, err)
			require.Len(t, result.Nodes, 1)
			require.Empty(t, result.Nodes[0].Flow)
			require.Len(t, result.Report.Warnings, 1)
			warning := result.Report.Warnings[0]
			require.Equal(t, "node_normalized_incompatible_flow", warning.Code)
			require.Equal(t, "flow", warning.Field)
			require.Equal(t, "normalized", warning.Source)
			require.Equal(t, "vless", warning.Node)
			require.NotNil(t, warning.NodeIndex)
			require.Zero(t, *warning.NodeIndex)
			require.NotContains(t, warning.Message, "11111111-1111-1111-1111-111111111111")
		})
	}
}

func TestServiceParseDropsUnsupportedVLESSFlowWithoutDowngrade(t *testing.T) {
	result, err := service.New().Parse(context.Background(), domain.ParseRequest{
		Format: "uri-list",
		Content: []byte(strings.Join([]string{
			"vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none#valid",
			"vless://22222222-2222-2222-2222-222222222222@example.com:443?encryption=none&flow=xtls-rprx-vision-udp443#unsupported",
		}, "\n")),
	})

	require.NoError(t, err)
	require.Len(t, result.Nodes, 1)
	require.Equal(t, "valid", result.Nodes[0].Name)
	require.Len(t, result.Report.Warnings, 1)
	require.Equal(t, "node_validation_dropped", result.Report.Warnings[0].Code)
	require.Equal(t, "flow", result.Report.Warnings[0].Field)
}

func TestServiceParseKeepsVLESSVisionFlowForStreamTransports(t *testing.T) {
	svc := service.New()
	for _, transport := range []string{"", "tcp", "raw"} {
		name := transport
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			transportJSON := ""
			if transport != "" {
				transportJSON = `,"transport":{"type":"` + transport + `"}`
			}
			result, err := svc.Parse(context.Background(), domain.ParseRequest{
				Format: "json-nodes",
				Content: []byte(`[{
  "name": "vless",
  "type": "vless",
  "server": "example.com",
  "port": 443,
  "uuid": "11111111-1111-1111-1111-111111111111",
  "flow": "xtls-rprx-vision"` + transportJSON + `
}]`),
			})

			require.NoError(t, err)
			require.Len(t, result.Nodes, 1)
			require.Equal(t, "xtls-rprx-vision", result.Nodes[0].Flow)
			require.Empty(t, result.Report.Warnings)
		})
	}
}

func TestServiceParseWithProcessorChain(t *testing.T) {
	svc := service.New()
	result, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format: "uri-list",
		Content: []byte(strings.Join([]string{
			"ss://aes-128-gcm:secret@example.com:8388#hk-a",
			"ss://aes-128-gcm:secret@example.com:8388#us-a",
		}, "\n")),
		Processors: []domain.ProcessorSpec{
			{Type: "filter", Params: params(t, map[string]any{"action": "keep", "field": "name", "match": "regex", "pattern": "^hk-"})},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Nodes, 1)
	require.Equal(t, "hk-a", result.Nodes[0].Name)
}
func TestServiceRenderWithProcessorChain(t *testing.T) {
	svc := service.New()
	parsed, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format:  "uri",
		Content: []byte("ss://aes-128-gcm:secret@example.com:8388#a"),
	})
	require.NoError(t, err)
	rendered, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "mihomo-proxies",
		Nodes:  parsed.Nodes,
		Processors: []domain.ProcessorSpec{
			{Type: "rename", Params: params(t, map[string]any{"mode": "prefix", "value": "[X]"})},
		},
	})
	require.NoError(t, err)
	require.Contains(t, string(rendered.Body), "[X]a")
}
