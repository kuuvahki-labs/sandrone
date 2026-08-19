package singbox_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestRenderSingBoxTreatsRawAsDefaultTCPTransport(t *testing.T) {
	nodes := rawTransportNodes()
	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, len(nodes), report.SuccessCount)
	require.Empty(t, report.Warnings)
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(out, &doc))
	require.Len(t, doc.Outbounds, len(nodes))
	for _, outbound := range doc.Outbounds {
		require.NotContains(t, outbound, "transport")
	}
}

func TestRenderSingBoxSkipsTransportDetailsWithoutType(t *testing.T) {
	nodes := []domain.NodeIR{
		{Name: "invalid", Type: domain.NodeTypeVLESS, Server: "invalid.example.com", Port: 443, UUID: "11111111-1111-1111-1111-111111111111", Encryption: "none", Transport: &domain.TransportOptions{Path: "/must-not-drop"}},
		{Name: "neighbor", Type: domain.NodeTypeVLESS, Server: "neighbor.example.com", Port: 443, UUID: "22222222-2222-2222-2222-222222222222", Encryption: "none"},
	}
	out, report, err := singbox.NewRenderer().RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, 1, report.SuccessCount)
	require.Len(t, report.Warnings, 1)
	require.Equal(t, "render_node_skipped", report.Warnings[0].Code)
	require.Contains(t, report.Warnings[0].Message, "no transport type")
	require.Contains(t, string(out), "neighbor")
	require.NotContains(t, string(out), "invalid.example.com")
}

func rawTransportNodes() []domain.NodeIR {
	return []domain.NodeIR{
		{Name: "vmess-raw", Type: domain.NodeTypeVMess, Server: "vmess.example.com", Port: 443, UUID: "11111111-1111-1111-1111-111111111111", Transport: &domain.TransportOptions{Type: "raw"}},
		{Name: "vless-raw", Type: domain.NodeTypeVLESS, Server: "vless.example.com", Port: 443, UUID: "22222222-2222-2222-2222-222222222222", Encryption: "none", Transport: &domain.TransportOptions{Type: "raw"}},
		{Name: "trojan-raw", Type: domain.NodeTypeTrojan, Server: "trojan.example.com", Port: 443, Password: "secret", Transport: &domain.TransportOptions{Type: "raw"}},
	}
}
