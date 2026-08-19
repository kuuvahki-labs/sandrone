package shadowrocket_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shadowrocket"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestRenderShadowrocketTreatsRawAsDefaultTCPTransport(t *testing.T) {
	nodes := []domain.NodeIR{
		{Name: "vmess-raw", Type: domain.NodeTypeVMess, Server: "vmess.example.com", Port: 443, UUID: "11111111-1111-1111-1111-111111111111", Cipher: "auto", Transport: &domain.TransportOptions{Type: "raw"}},
		{Name: "vless-raw", Type: domain.NodeTypeVLESS, Server: "vless.example.com", Port: 443, UUID: "22222222-2222-2222-2222-222222222222", Encryption: "none", Transport: &domain.TransportOptions{Type: "raw"}},
		{Name: "trojan-raw", Type: domain.NodeTypeTrojan, Server: "trojan.example.com", Port: 443, Password: "secret", Transport: &domain.TransportOptions{Type: "raw"}},
	}
	out, report, err := shadowrocket.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, len(nodes), report.SuccessCount)
	require.Empty(t, report.Warnings)
	require.Contains(t, string(out), "vmess-raw = vmess")
	require.Contains(t, string(out), "vless-raw = vless")
	require.Contains(t, string(out), "trojan-raw = trojan")
}
