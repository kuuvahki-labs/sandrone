//go:build probe_mihomo

package mihomo_test

import (
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"

	upstream "github.com/metacubex/mihomo/adapter"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestMihomoECHPEMAndUnpaddedInputsConstructLockedCore(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(echConfigFixture)
	require.NoError(t, err)
	pemText := string(pem.EncodeToMemory(&pem.Block{Type: "ECH CONFIGS", Bytes: data}))
	for _, config := range [][]string{{strings.TrimRight(echConfigFixture, "=")}, strings.Split(pemText, "\n")} {
		body, report, err := mihomo.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{{
			Name: "ech", Type: domain.NodeTypeTrojan, Server: "127.0.0.1", Port: 9, Password: "fixture",
			TLS: &domain.TLSOptions{Enabled: true, ServerName: "public.example", ECH: &domain.ECHOptions{Enabled: true, Config: config}},
		}}, domain.RenderOptions{})
		require.NoError(t, err)
		require.Empty(t, report.Warnings)
		var doc struct {
			Proxies []map[string]any `yaml:"proxies"`
		}
		require.NoError(t, yaml.Unmarshal(body, &doc))
		proxy, err := upstream.ParseProxy(doc.Proxies[0])
		require.NoError(t, err)
		require.NoError(t, proxy.Close())
	}
}
