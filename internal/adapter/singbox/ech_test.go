package singbox_test

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestECHRenderersPreserveConfigBytes(t *testing.T) {
	t.Parallel()
	want, err := base64.StdEncoding.DecodeString(echConfigFixture)
	require.NoError(t, err)
	pemText := string(pem.EncodeToMemory(&pem.Block{Type: "ECH CONFIGS", Bytes: want}))
	for _, config := range [][]string{{echConfigFixture}, {strings.TrimRight(echConfigFixture, "=")}, strings.Split(pemText, "\n")} {
		node := domain.NodeIR{Name: "ech", Type: domain.NodeTypeTrojan, Server: "proxy.example", Port: 443, Password: "fixture",
			TLS: &domain.TLSOptions{Enabled: true, ECH: &domain.ECHOptions{Enabled: true, Config: config}},
		}
		body, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
		require.NoError(t, err)
		require.Empty(t, report.Warnings)
		var singDoc struct {
			Outbounds []struct {
				TLS struct {
					ECH struct {
						Config []string `json:"config"`
					} `json:"ech"`
				} `json:"tls"`
			} `json:"outbounds"`
		}
		require.NoError(t, json.Unmarshal(body, &singDoc))
		block, rest := pem.Decode([]byte(strings.Join(singDoc.Outbounds[0].TLS.ECH.Config, "\n")))
		require.NotNil(t, block)
		require.Equal(t, "ECH CONFIGS", block.Type)
		require.Equal(t, want, block.Bytes)
		require.Empty(t, rest)
		body, report, err = mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
		require.NoError(t, err)
		require.Empty(t, report.Warnings)
		var mihomoDoc struct {
			Proxies []struct {
				ECH struct {
					Config string `yaml:"config"`
				} `yaml:"ech-opts"`
			} `yaml:"proxies"`
		}
		require.NoError(t, yaml.Unmarshal(body, &mihomoDoc))
		require.Equal(t, echConfigFixture, mihomoDoc.Proxies[0].ECH.Config)
		require.Equal(t, config, node.TLS.ECH.Config)
	}
}

func TestECHRenderersSkipInvalidConfigsWithoutDroppingECH(t *testing.T) {
	t.Parallel()
	for _, renderer := range []interface {
		RenderWithReport(context.Context, []domain.NodeIR, domain.RenderOptions) ([]byte, domain.RenderReport, error)
	}{singbox.NewRenderer(), mihomo.NewRenderer()} {
		bad := domain.NodeIR{Name: "bad", Type: domain.NodeTypeTrojan, Server: "bad.example", Port: 443, Password: "fixture",
			TLS: &domain.TLSOptions{Enabled: true, ECH: &domain.ECHOptions{Enabled: true, Config: []string{"invalid-private-config"}}},
		}
		good := domain.NodeIR{Name: "good", Type: domain.NodeTypeTrojan, Server: "good.example", Port: 443, Password: "fixture",
			TLS: &domain.TLSOptions{Enabled: true, ECH: &domain.ECHOptions{Enabled: true, QueryServerName: "query.example"}},
		}
		body, report, err := renderer.RenderWithReport(context.Background(), []domain.NodeIR{bad, good}, domain.RenderOptions{})
		require.NoError(t, err)
		require.Equal(t, 1, report.SuccessCount)
		require.Len(t, report.Warnings, 1)
		require.Equal(t, "render_node_skipped", report.Warnings[0].Code)
		require.NotContains(t, string(body), "bad.example")
		require.Contains(t, string(body), "query.example")
		require.NotContains(t, report.Warnings[0].Message, "invalid-private-config")
		_, report, err = renderer.RenderWithReport(context.Background(), []domain.NodeIR{bad}, domain.RenderOptions{})
		require.True(t, domain.IsCode(err, domain.CodeRenderFailed))
		require.Zero(t, report.SuccessCount)
	}
}

func TestMihomoECHDownloadConfigIsConverted(t *testing.T) {
	t.Parallel()
	decoded, err := base64.StdEncoding.DecodeString(echConfigFixture)
	require.NoError(t, err)
	pemText := string(pem.EncodeToMemory(&pem.Block{Type: "ECH CONFIGS", Bytes: decoded}))
	node := domain.NodeIR{Name: "download", Type: domain.NodeTypeVLESS, Server: "proxy.example", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none",
		Transport: &domain.TransportOptions{Type: "xhttp", XHTTP: &domain.XHTTPTransportOptions{
			DownloadSettings: &domain.XHTTPDownloadSettings{TLS: &domain.TLSOptions{Enabled: true, ECH: &domain.ECHOptions{Enabled: true, Config: []string{pemText}}}},
		}},
	}
	body, report, err := mihomo.NewRenderer().RenderWithReport(context.Background(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Empty(t, report.Warnings)
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(body, &doc))
	download := doc.Proxies[0]["xhttp-opts"].(map[string]any)["download-settings"].(map[string]any)
	require.Equal(t, echConfigFixture, download["ech-opts"].(map[string]any)["config"])
	require.Equal(t, []string{pemText}, node.Transport.XHTTP.DownloadSettings.TLS.ECH.Config)
}

func TestECHRenderersAcceptDisabledECHWithResidualOptions(t *testing.T) {
	t.Parallel()
	for _, renderer := range []interface {
		RenderWithReport(context.Context, []domain.NodeIR, domain.RenderOptions) ([]byte, domain.RenderReport, error)
	}{singbox.NewRenderer(), mihomo.NewRenderer()} {
		for _, ech := range []*domain.ECHOptions{
			{Config: []string{"invalid-config"}},
			{DNS: "udp://192.0.2.1"},
			{Config: []string{"invalid-config"}, DNS: "udp://192.0.2.1"},
		} {
			node := domain.NodeIR{Name: "disabled-ech", Type: domain.NodeTypeTrojan, Server: "proxy.example", Port: 443, Password: "fixture",
				TLS: &domain.TLSOptions{Enabled: true, ECH: ech},
			}
			body, report, err := renderer.RenderWithReport(t.Context(), []domain.NodeIR{node}, domain.RenderOptions{})
			require.NoError(t, err)
			require.Equal(t, 1, report.SuccessCount)
			require.Empty(t, report.Warnings)
			require.Contains(t, string(body), "proxy.example")
			require.False(t, node.TLS.ECH.Enabled)
		}
	}
}

func TestMihomoECHDownloadAcceptsDisabledResidualOptions(t *testing.T) {
	t.Parallel()
	node := domain.NodeIR{Name: "download", Type: domain.NodeTypeVLESS, Server: "proxy.example", Port: 443,
		UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none",
		Transport: &domain.TransportOptions{Type: "xhttp", XHTTP: &domain.XHTTPTransportOptions{
			DownloadSettings: &domain.XHTTPDownloadSettings{TLS: &domain.TLSOptions{Enabled: true, ECH: &domain.ECHOptions{
				Config: []string{"invalid-config"}, DNS: "udp://192.0.2.1",
			}}},
		}},
	}
	body, report, err := mihomo.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Empty(t, report.Warnings)
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(body, &doc))
	download := doc.Proxies[0]["xhttp-opts"].(map[string]any)["download-settings"].(map[string]any)
	require.Equal(t, false, download["ech-opts"].(map[string]any)["enable"])
}

func TestMihomoECHConfigIsConvertedForImplicitTLS(t *testing.T) {
	t.Parallel()
	node := domain.NodeIR{Name: "implicit-tls", Type: domain.NodeTypeTrojan, Server: "proxy.example", Port: 443, Password: "fixture",
		TLS: &domain.TLSOptions{ECH: &domain.ECHOptions{Enabled: true, Config: []string{strings.TrimRight(echConfigFixture, "=")}}},
	}
	body, report, err := mihomo.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{node}, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Empty(t, report.Warnings)
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(body, &doc))
	require.Equal(t, echConfigFixture, doc.Proxies[0]["ech-opts"].(map[string]any)["config"])
}
