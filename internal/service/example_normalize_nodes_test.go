package service_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceExampleNormalizeNodesRendersUniqueMihomoAndSingBoxNames(t *testing.T) {
	scriptBody, err := os.ReadFile(filepath.Join("..", "..", "examples", "scripts", "normalize-nodes.js"))
	require.NoError(t, err)
	processorSpec := domain.ProcessorSpec{
		Type:  "script",
		Stage: domain.StageNodes,
		Params: params(t, map[string]any{
			"source": inlineScriptSource(string(scriptBody)),
		}),
	}
	nodes := []domain.NodeIR{
		{Name: "香港", Type: domain.NodeTypeShadowsocks, Server: "one.example.com", Port: 8388, Cipher: "aes-128-gcm", Password: "one"},
		{Name: "香港", Type: domain.NodeTypeVLESS, Server: "two.example.com", Port: 443, UUID: "22222222-2222-2222-2222-222222222222", Encryption: "none"},
	}
	svc := service.New()

	mihomoResult, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "mihomo-proxies", Target: "mihomo", Nodes: nodes, Processors: []domain.ProcessorSpec{processorSpec},
	})
	require.NoError(t, err)
	require.Equal(t, 2, mihomoResult.Report.Render.SuccessCount)
	var mihomoDocument struct {
		Proxies []struct {
			Name string `yaml:"name"`
		} `yaml:"proxies"`
	}
	require.NoError(t, yaml.Unmarshal(mihomoResult.Body, &mihomoDocument))
	require.Equal(t, []string{"🇭🇰 香港 01", "🇭🇰 香港 02"}, []string{mihomoDocument.Proxies[0].Name, mihomoDocument.Proxies[1].Name})

	singBoxResult, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "sing-box-outbounds", Target: "sing-box", Nodes: nodes, Processors: []domain.ProcessorSpec{processorSpec},
	})
	require.NoError(t, err)
	require.Equal(t, 2, singBoxResult.Report.Render.SuccessCount)
	var singBoxDocument struct {
		Outbounds []struct {
			Tag string `json:"tag"`
		} `json:"outbounds"`
	}
	require.NoError(t, json.Unmarshal(singBoxResult.Body, &singBoxDocument))
	require.Equal(t, []string{"🇭🇰 香港 01", "🇭🇰 香港 02"}, []string{singBoxDocument.Outbounds[0].Tag, singBoxDocument.Outbounds[1].Tag})
}
