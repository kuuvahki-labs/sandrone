package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
)

func TestNormalizeNodesMatchesUpstreamVMessAndVLESSMapping(t *testing.T) {
	nodes := []domain.NodeIR{
		{Name: "vmess", Type: domain.NodeTypeVMess, UUID: "123456"},
		{Name: "vless", Type: domain.NodeTypeVLESS, UUID: "a9dk23bz0"},
		{Name: "canonical", Type: domain.NodeTypeVLESS, UUID: "11111111-1111-1111-1111-111111111111"},
		{Name: "compact", Type: domain.NodeTypeVMess, UUID: "22222222222222222222222222222222"},
		{Name: "tuic-invalid", Type: domain.NodeTypeTUIC, UUID: "not-a-uuid"},
		{Name: "tuic-compact", Type: domain.NodeTypeTUIC, UUID: "33333333333333333333333333333333"},
	}

	normalized := normalizeNodes(nodes)

	require.Equal(t, "f8598425-92f2-5508-a071-4fc67f9040ac", normalized[0].UUID)
	require.Equal(t, "c91481b6-fc0f-5d9e-b166-5ddf07b9c3c5", normalized[1].UUID)
	require.Equal(t, "11111111-1111-1111-1111-111111111111", normalized[2].UUID)
	require.Equal(t, "22222222-2222-2222-2222-222222222222", normalized[3].UUID)
	require.Equal(t, "not-a-uuid", normalized[4].UUID)
	require.Equal(t, "33333333-3333-3333-3333-333333333333", normalized[5].UUID)
	require.Equal(t, "123456", nodes[0].UUID)
	require.Equal(t, "33333333333333333333333333333333", nodes[5].UUID)
}

func TestNormalizeNodesDefaultsRealityClientFingerprintsWithoutMutatingInput(t *testing.T) {
	nodes := []domain.NodeIR{
		{
			Name: "top-level-default",
			TLS:  &domain.TLSOptions{Enabled: true, Reality: &domain.RealityOptions{Enabled: true, PublicKey: "public"}},
		},
		{
			Name: "explicit",
			TLS: &domain.TLSOptions{
				Enabled: true, ClientFingerprint: "firefox",
				Reality: &domain.RealityOptions{Enabled: true, PublicKey: "public"},
			},
		},
		{
			Name: "download-default",
			Transport: &domain.TransportOptions{Type: "xhttp", XHTTP: &domain.XHTTPTransportOptions{
				DownloadSettings: &domain.XHTTPDownloadSettings{TLS: &domain.TLSOptions{
					Enabled: true, Reality: &domain.RealityOptions{Enabled: true, PublicKey: "download-public"},
				}},
			}},
		},
	}

	normalized := normalizeNodes(nodes)

	require.Equal(t, defaultRealityClientFingerprint, normalized[0].TLS.ClientFingerprint)
	require.Equal(t, "firefox", normalized[1].TLS.ClientFingerprint)
	require.Equal(t, defaultRealityClientFingerprint, normalized[2].Transport.XHTTP.DownloadSettings.TLS.ClientFingerprint)
	require.Empty(t, nodes[0].TLS.ClientFingerprint)
	require.Empty(t, nodes[2].Transport.XHTTP.DownloadSettings.TLS.ClientFingerprint)
}

func TestValidateNodeBatchSilentlyCanonicalizesVMessAndVLESSUserIDs(t *testing.T) {
	nodes := []domain.NodeIR{
		{Name: "vmess", Type: domain.NodeTypeVMess, Server: "vmess.example", Port: 443, UUID: "123456", Cipher: "auto"},
		{Name: "vless", Type: domain.NodeTypeVLESS, Server: "vless.example", Port: 443, UUID: "a9dk23bz0", Encryption: "none"},
	}

	result, warnings, err := validateNodeBatch(nodes, nodevalidation.StageNormalized, "json-nodes")

	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Equal(t, 2, result.Counts.Valid)
	require.Equal(t, "f8598425-92f2-5508-a071-4fc67f9040ac", result.Nodes[0].UUID)
	require.Equal(t, "c91481b6-fc0f-5d9e-b166-5ddf07b9c3c5", result.Nodes[1].UUID)
}
