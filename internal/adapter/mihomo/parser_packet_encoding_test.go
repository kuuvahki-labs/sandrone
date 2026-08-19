package mihomo_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestMihomoPacketAliasCanonicalizesBeforeSingBoxRender(t *testing.T) {
	nodes, source, err := mihomo.NewParser().Parse(context.Background(), []byte(`
proxies:
  - name: vmess-packet
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    packet-encoding: packet
`))
	require.NoError(t, err)
	require.Empty(t, source.Warnings)
	require.Equal(t, "packetaddr", nodes[0].PacketEncoding)

	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Empty(t, report.Warnings)
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(out, &doc))
	require.Equal(t, "packetaddr", doc.Outbounds[0]["packet_encoding"])
}
