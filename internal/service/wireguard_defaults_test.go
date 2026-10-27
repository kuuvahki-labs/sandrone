package service_test

import (
	"context"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestMihomoWireGuardDefaultsSurviveValidationAndSingBoxRendering(t *testing.T) {
	t.Parallel()
	result, err := service.New().Convert(context.Background(), domain.ConvertRequest{
		FromFormat: "mihomo", ToFormat: "sing-box-outbounds",
		Content: []byte(`{"proxies":[{"name":"wg","type":"wireguard","server":"192.0.2.1","port":51820,"ip":"10.0.0.2","ipv6":"fd00::2","private-key":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","public-key":"AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="}]}`),
	})
	require.NoError(t, err)
	var document struct {
		Endpoints []struct {
			Address []string `json:"address"`
			Peers   []struct {
				AllowedIPs []string `json:"allowed_ips"`
			} `json:"peers"`
		} `json:"endpoints"`
	}
	require.NoError(t, json.Unmarshal(result.Body, &document))
	require.Len(t, document.Endpoints, 1)
	require.Equal(t, []string{"10.0.0.2/32", "fd00::2/128"}, document.Endpoints[0].Address)
	require.Len(t, document.Endpoints[0].Peers, 1)
	require.Equal(t, []string{"0.0.0.0/0", "::/0"}, document.Endpoints[0].Peers[0].AllowedIPs)
}
