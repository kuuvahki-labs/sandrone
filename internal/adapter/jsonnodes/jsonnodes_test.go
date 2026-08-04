package jsonnodes_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/jsonnodes"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/mihomo"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type nodeParser interface {
	Parse(context.Context, []byte) ([]domain.NodeIR, *domain.SourceInfo, error)
}

func TestParserParseArrayAndSetsSourceFormat(t *testing.T) {
	parser := jsonnodes.NewParser()

	nodes, source, err := parser.Parse(context.Background(), []byte(`[
  {
    "name": "node-a",
    "type": "ss",
    "server": "example.com",
    "port": 8388,
    "cipher": "aes-128-gcm",
    "password": "secret"
  }
]`))

	require.NoError(t, err)
	require.Equal(t, "json-nodes", parser.Name())
	require.Equal(t, "json-nodes", source.Format)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeShadowsocks, nodes[0].Type)
	require.Equal(t, "json-nodes", nodes[0].SourceFormat)
}

func TestParserParseWrappedNodesPreservesExistingSourceFormat(t *testing.T) {
	parser := jsonnodes.NewParser()

	nodes, source, err := parser.Parse(context.Background(), []byte(`{
  "nodes": [
    {
      "name": "node-a",
      "type": "socks",
      "server": "example.com",
      "port": 1080,
      "source_format": "custom"
    }
  ]
}`))

	require.NoError(t, err)
	require.Equal(t, "json-nodes", source.Format)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeSOCKS, nodes[0].Type)
	require.Equal(t, "custom", nodes[0].SourceFormat)
}

func TestParserRejectsInvalidJSON(t *testing.T) {
	parser := jsonnodes.NewParser()

	nodes, source, err := parser.Parse(context.Background(), []byte(`{"nodes":`))

	require.Error(t, err)
	require.Nil(t, nodes)
	require.Equal(t, "json-nodes", source.Format)
	require.True(t, domain.IsCode(err, domain.CodeParseFailed), "unexpected error: %v", err)
}

func TestParserNormalizesLegacyHysteriaBandwidth(t *testing.T) {
	parser := jsonnodes.NewParser()
	tests := []struct {
		name     string
		input    string
		want     domain.HysteriaOptions
		warnings int
	}{
		{
			name: "sing-box provenance uses bytes per second",
			input: `[{
  "name":"sing-box","type":"hysteria","server":"sing-box.example","port":8443,
  "source_format":"sing-box","hysteria":{"up":"55","down":"100"}
}]`,
			want: domain.HysteriaOptions{Up: "55 Bps", Down: "100 Bps"},
		},
		{
			name: "missing provenance uses Mbps and warns",
			input: `[{
  "name":"legacy","type":"hysteria","server":"legacy.example","port":8443,
  "hysteria":{"up":"55","down":"100"}
}]`,
			want:     domain.HysteriaOptions{UpMbps: 55, DownMbps: 100},
			warnings: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodes, _, err := parser.Parse(context.Background(), []byte(test.input))
			require.NoError(t, err)
			require.Len(t, nodes, 1)
			require.Equal(t, test.want, *nodes[0].Hysteria)
			require.Len(t, nodes[0].Warnings, test.warnings)
			for _, warning := range nodes[0].Warnings {
				require.Equal(t, "parse_implicit_bandwidth_unit", warning.Code)
				require.Equal(t, "bare Hysteria bandwidth assumed to be Mbps", warning.Message)
				require.Equal(t, nodes[0].Name, warning.Node)
			}
		})
	}
}

func TestParserChecksLegacyHysteriaMbpsBound(t *testing.T) {
	max := shared.MaxHysteriaMbps()
	if max == int(^uint(0)>>1) {
		t.Skip("max+1 is not representable as int on this platform")
	}
	input := fmt.Sprintf(`[
  {"name":"max","type":"hysteria","server":"max.example","port":8443,"hysteria":{"up_mbps":%d,"down_mbps":%d}},
  {"name":"over","type":"hysteria","server":"over.example","port":8443,"hysteria":{"up_mbps":%d,"down_mbps":%d}}
]`, max, max, max+1, max)

	nodes, _, err := jsonnodes.NewParser().Parse(context.Background(), []byte(input))
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, &domain.HysteriaOptions{UpMbps: max, DownMbps: max}, nodes[0].Hysteria)
	require.Zero(t, nodes[1].Hysteria.UpMbps)
	require.Equal(t, max, nodes[1].Hysteria.DownMbps)
	require.JSONEq(t, fmt.Sprint(max+1), string(nodes[1].Raw["json-nodes.hysteria.up"]))
	require.Equal(t, "parse_unknown_field", nodes[1].Warnings[0].Code)
}

func TestParserNormalizesLegacyHysteriaBandwidthAcrossFormats(t *testing.T) {
	tests := []struct {
		name   string
		parser nodeParser
		input  string
		want   domain.HysteriaOptions
	}{
		{
			name:   "legacy JSON Nodes",
			parser: jsonnodes.NewParser(),
			input:  `[{"name":"legacy","type":"hysteria","server":"legacy.example","port":8443,"hysteria":{"up":"55","down":"100"}}]`,
			want:   domain.HysteriaOptions{UpMbps: 55, DownMbps: 100},
		},
		{
			name:   "Mihomo",
			parser: mihomo.NewParser(),
			input:  `proxies: [{name: mihomo, type: hysteria, server: mihomo.example, port: 8443, up: "55", down: "100"}]`,
			want:   domain.HysteriaOptions{UpMbps: 55, DownMbps: 100},
		},
		{
			name:   "sing-box",
			parser: singbox.NewParser(),
			input:  `{"outbounds":[{"type":"hysteria","tag":"sing-box","server":"sing-box.example","server_port":8443,"up":55,"down":100}]}`,
			want:   domain.HysteriaOptions{Up: "55 Bps", Down: "100 Bps"},
		},
		{
			name:   "URI",
			parser: uri.NewParser(),
			input:  `hysteria://uri.example:8443?upmbps=55&downmbps=100#uri`,
			want:   domain.HysteriaOptions{UpMbps: 55, DownMbps: 100},
		},
	}

	renderer := jsonnodes.NewRenderer()
	bareNumber := regexp.MustCompile(`^[0-9]+$`)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodes, _, err := test.parser.Parse(context.Background(), []byte(test.input))
			require.NoError(t, err)
			require.Len(t, nodes, 1)

			out, err := renderer.Render(context.Background(), nodes, domain.RenderOptions{})
			require.NoError(t, err)
			var rendered []domain.NodeIR
			require.NoError(t, json.Unmarshal(out, &rendered))
			require.Equal(t, test.want, *rendered[0].Hysteria)

			var raw []map[string]any
			require.NoError(t, json.Unmarshal(out, &raw))
			hysteria := raw[0]["hysteria"].(map[string]any)
			for _, direction := range []string{"up", "down"} {
				_, hasText := hysteria[direction]
				_, hasMbps := hysteria[direction+"_mbps"]
				require.NotEqual(t, hasText, hasMbps, "exactly one canonical %s field must be rendered", direction)
				if text, ok := hysteria[direction].(string); ok {
					require.False(t, bareNumber.MatchString(text), "bare numeric %s leaked into JSON Nodes", direction)
				}
			}
		})
	}
}

func TestRendererRenderJSONNodes(t *testing.T) {
	renderer := jsonnodes.NewRenderer()
	nodes := []domain.NodeIR{
		{
			Name:     "node-a",
			Type:     domain.NodeTypeShadowsocks,
			Server:   "example.com",
			Port:     8388,
			Cipher:   "aes-128-gcm",
			Password: "secret",
		},
	}

	out, report, err := renderer.RenderWithReport(context.Background(), nodes, domain.RenderOptions{})

	require.NoError(t, err)
	require.Equal(t, "json-nodes", renderer.Name())
	require.Equal(t, 1, report.SuccessCount)
	require.JSONEq(t, `[
  {
    "name": "node-a",
    "type": "ss",
    "server": "example.com",
    "port": 8388,
    "cipher": "aes-128-gcm",
    "password": "secret"
  }
]`, string(out))

	rendered, err := renderer.Render(context.Background(), nodes, domain.RenderOptions{})
	require.NoError(t, err)
	require.Equal(t, out, rendered)

	var decoded []domain.NodeIR
	require.NoError(t, json.Unmarshal(rendered, &decoded))
	require.Equal(t, nodes[0].Name, decoded[0].Name)
}
