package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestCapabilitySummaryIncludesAdapterCapabilities(t *testing.T) {
	svc := service.New()
	summary := svc.CapabilitySummary()
	capabilities, ok := summary["capabilities"].([]shared.Capability)
	require.True(t, ok)
	require.NotEmpty(t, capabilities)

	seen := map[string]shared.Capability{}
	for _, capability := range capabilities {
		require.NotEmpty(t, capability.Types, capability.Format)
		require.NotEmpty(t, capability.Fields, capability.Format)
		seen[capability.Format+"\x00"+string(capability.Direction)] = capability
	}
	parseFormats := summary["parse_formats"].([]string)
	require.ElementsMatch(t, []string{"uri", "uri-list", "base64", "mihomo", "sing-box", "json-nodes"}, parseFormats)
	for _, format := range parseFormats {
		require.Contains(t, seen, format+"\x00"+string(shared.DirectionParse))
	}
	renderFormats := summary["render_formats"].([]string)
	require.ElementsMatch(t, []string{"mihomo-proxies", "shadowrocket-proxies", "sing-box-outbounds", "json-nodes", "uri-list"}, renderFormats)
	for _, format := range renderFormats {
		require.Contains(t, seen, format+"\x00"+string(shared.DirectionRender))
	}

	uriList := seen["uri-list\x00"+string(shared.DirectionParse)]
	require.NotContains(t, uriList.Types, domain.NodeTypeWireGuard)
	require.NotEmpty(t, seen["uri-list\x00"+string(shared.DirectionRender)].Lossy)
}
func TestServiceWithProcessor(t *testing.T) {
	svc := service.New(service.WithProcessor(func(r *processor.Registry) {
		r.RegisterNode("tag_x", func(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
			return &stubTagProcessor{}, nil
		})
	}))
	result, err := svc.Parse(context.Background(), domain.ParseRequest{
		Format:  "uri",
		Content: []byte("ss://aes-128-gcm:secret@example.com:8388#a"),
		Processors: []domain.ProcessorSpec{
			{Type: "tag_x", Stage: domain.StageNodes},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"x"}, result.Nodes[0].Tags)
}
func TestServiceRegistryExposed(t *testing.T) {
	svc := service.New()
	require.True(t, svc.Registry().HasNode("filter"))
	require.True(t, svc.Registry().HasFile("script"))
	require.True(t, svc.Registry().HasFile("template"))
	require.True(t, svc.Registry().HasFile("merge"))
	require.True(t, svc.Registry().HasFile("inject_nodes"))
	require.True(t, svc.Registry().HasFile("yaml_patch"))
	require.True(t, svc.Registry().HasFile("json_patch"))
}
