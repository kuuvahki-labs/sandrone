//go:build probe_singbox

package singbox_test

import (
	"strings"
	"testing"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestSingBoxECHBase64InputsConstructLockedCore(t *testing.T) {
	for _, config := range []string{echConfigFixture, strings.TrimRight(echConfigFixture, "=")} {
		body, report, err := singbox.NewRenderer().RenderWithReport(t.Context(), []domain.NodeIR{{
			Name: "ech", Type: domain.NodeTypeTrojan, Server: "127.0.0.1", Port: 9, Password: "fixture",
			TLS: &domain.TLSOptions{Enabled: true, ServerName: "public.example", ECH: &domain.ECHOptions{Enabled: true, Config: []string{config}}},
		}}, domain.RenderOptions{})
		require.NoError(t, err)
		require.Empty(t, report.Warnings)
		ctx := include.Context(t.Context())
		var options option.Options
		require.NoError(t, options.UnmarshalJSONContext(ctx, body))
		instance, err := box.New(box.Options{Context: ctx, Options: options})
		require.NoError(t, err)
		require.NoError(t, instance.Close())
	}
}
