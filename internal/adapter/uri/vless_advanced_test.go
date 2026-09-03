package uri

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestVLESSXHTTPExtraRoundTrip(t *testing.T) {
	server := "download.example.com"
	port := uint16(8443)
	path := "/download"
	host := "cdn.example.com"
	node := domain.NodeIR{
		Name: "advanced", Type: domain.NodeTypeVLESS,
		Server: "example.com", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111",
		TLS:  &domain.TLSOptions{Enabled: true},
		Transport: &domain.TransportOptions{Type: "xhttp", Path: "/upload", Host: "upload.example.com", Headers: map[string]string{"Host": "upload.example.com"}, XHTTP: &domain.XHTTPTransportOptions{
			Mode: "packet-up",
			ReuseSettings: &domain.XHTTPReuseSettings{
				MaxConcurrency: "8-16", MaxConnections: "2", CMaxReuseTimes: "64",
				HMaxRequestTimes: "100-200", HMaxReusableSecs: "1800", HKeepAlivePeriod: 15,
			},
			DownloadSettings: &domain.XHTTPDownloadSettings{
				Server: &server, Port: &port, Path: &path, Host: &host,
				TLS: &domain.TLSOptions{
					Enabled: true, ServerName: "download.example.com", ALPN: []string{"h2"},
					ClientFingerprint: "chrome", InsecureSkipVerify: true,
					Reality: &domain.RealityOptions{
						Enabled: true, PublicKey: "public-key", ShortID: "08",
						MLDSA65Verify: "verify-key", SpiderX: "/fallback",
					},
				},
				ReuseSettings: &domain.XHTTPReuseSettings{MaxConnections: "4"},
			},
		}},
	}

	rendered, _, err := renderVLESSURI(node)
	require.NoError(t, err)
	parsedURL, err := url.Parse(rendered)
	require.NoError(t, err)
	require.NotEmpty(t, parsedURL.Query().Get("extra"))

	parsed, _, err := parseVLESS(rendered)
	require.NoError(t, err)
	require.Equal(t, node.Transport, parsed.Transport)
}

func TestVLESSECHDNSAndForceQueryRoundTrip(t *testing.T) {
	for _, dns := range []string{"https://1.1.1.1/dns-query", "udp://8.8.8.8"} {
		t.Run(dns, func(t *testing.T) {
			node := domain.NodeIR{
				Name: "ech", Type: domain.NodeTypeVLESS,
				Server: "example.com", Port: 443,
				UUID: "11111111-1111-1111-1111-111111111111",
				TLS: &domain.TLSOptions{Enabled: true, ECH: &domain.ECHOptions{
					Enabled: true, QueryServerName: "cloudflare-ech.com", DNS: dns, ForceQuery: "full",
				}},
			}

			rendered, _, err := renderVLESSURI(node)
			require.NoError(t, err)
			parsed, _, err := parseVLESS(rendered)
			require.NoError(t, err)
			require.Equal(t, node.TLS.ECH, parsed.TLS.ECH)
			require.Empty(t, parsed.Raw)
		})
	}
}
