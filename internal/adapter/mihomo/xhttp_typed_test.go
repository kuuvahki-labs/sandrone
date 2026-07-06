package mihomo_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestMihomoTypedXHTTPSettingsRoundTrip(t *testing.T) {
	t.Parallel()

	nodes, _, err := mihomo.NewParser().Parse(context.Background(), []byte(`
proxies:
  - name: xhttp
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: xhttp
    xhttp-opts:
      path: /upload
      host: upload.example.com
      mode: packet-up
      reuse-settings:
        max-concurrency: 2-4
        max-connections: 1
        c-max-reuse-times: 8
        h-max-request-times: 16
        h-max-reusable-secs: 300
        h-keep-alive-period: 30
      download-settings:
        server: download.example.com
        port: 8443
        path: /download
        host: cdn.example.com
        tls: true
        servername: sni.example.com
        skip-cert-verify: true
        alpn: [h2]
        reuse-settings:
          max-concurrency: 4
`))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	transport := nodes[0].Transport
	require.NotNil(t, transport)
	require.Equal(t, &domain.XHTTPTransportOptions{
		Mode: "packet-up",
		ReuseSettings: &domain.XHTTPReuseSettings{
			MaxConcurrency: "2-4", MaxConnections: "1", CMaxReuseTimes: "8",
			HMaxRequestTimes: "16", HMaxReusableSecs: "300", HKeepAlivePeriod: 30,
		},
		DownloadSettings: &domain.XHTTPDownloadSettings{
			Server: stringPointer("download.example.com"), Port: uint16Pointer(8443),
			Path: stringPointer("/download"), Host: stringPointer("cdn.example.com"),
			TLS:           &domain.TLSOptions{Enabled: true, ServerName: "sni.example.com", InsecureSkipVerify: true, ALPN: []string{"h2"}},
			ReuseSettings: &domain.XHTTPReuseSettings{MaxConcurrency: "4"},
		},
	}, transport.XHTTP)

	rendered, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(rendered, &document))
	proxy := document["proxies"].([]any)[0].(map[string]any)
	xhttp := proxy["xhttp-opts"].(map[string]any)
	require.Equal(t, "packet-up", xhttp["mode"])
	require.Equal(t, "2-4", xhttp["reuse-settings"].(map[string]any)["max-concurrency"])
	require.Equal(t, "download.example.com", xhttp["download-settings"].(map[string]any)["server"])
}

func stringPointer(value string) *string { return &value }

func uint16Pointer(value uint16) *uint16 { return &value }
