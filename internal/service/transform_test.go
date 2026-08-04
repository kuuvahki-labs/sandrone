package service_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

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
	require.Contains(t, err.Error(), "base64:")
	require.Contains(t, err.Error(), "uri-list:")
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
      "password": "secret"
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
