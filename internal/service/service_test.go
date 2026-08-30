package service_test

import (
	"context"
	"encoding/base64"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestFormatCapabilitiesExposeStableSummaryAndDetail(t *testing.T) {
	svc := service.New()
	result, err := svc.ListFormatCapabilities(context.Background())
	require.NoError(t, err)
	require.Len(t, result.Items, 12)

	seen := map[string]domain.FormatCapabilitySummary{}
	for _, item := range result.Items {
		require.NotEmpty(t, item.NodeTypes, item.Format)
		require.Positive(t, item.FieldCounts.Supported, item.Format)
		seen[item.Format+"\x00"+string(item.Direction)] = item
	}
	parseFormats := formatsForDirection(result.Items, domain.CapabilityDirectionParse)
	require.ElementsMatch(t, []string{"uri", "uri-list", "base64", "mihomo", "sing-box", "json-nodes"}, parseFormats)
	for _, format := range parseFormats {
		require.Contains(t, seen, format+"\x00"+string(domain.CapabilityDirectionParse))
	}
	renderFormats := formatsForDirection(result.Items, domain.CapabilityDirectionRender)
	require.ElementsMatch(t, []string{"base64", "mihomo-proxies", "shadowrocket-proxies", "sing-box-outbounds", "json-nodes", "uri-list"}, renderFormats)
	for _, format := range renderFormats {
		require.Contains(t, seen, format+"\x00"+string(domain.CapabilityDirectionRender))
	}

	uriList := seen["uri-list\x00"+string(domain.CapabilityDirectionParse)]
	require.NotContains(t, uriList.NodeTypes, domain.NodeTypeWireGuard)
	require.Positive(t, seen["uri-list\x00"+string(domain.CapabilityDirectionRender)].FieldCounts.Lossy)

	detail, err := svc.GetFormatCapability(context.Background(), domain.FormatCapabilityRequest{
		Direction: domain.CapabilityDirectionRender,
		Format:    "mihomo-proxies",
	})
	require.NoError(t, err)
	require.Equal(t, "mihomo-proxies", detail.Format)
	require.NotEmpty(t, detail.Fields)
	require.Equal(t, []string{"v1.19.25"}, seen["mihomo-proxies\x00render"].Revisions)
	shadowrocketDetail, err := svc.GetFormatCapability(context.Background(), domain.FormatCapabilityRequest{
		Direction: domain.CapabilityDirectionRender,
		Format:    "shadowrocket-proxies",
	})
	require.NoError(t, err)
	require.Equal(t, detail.Types, shadowrocketDetail.Types)
	require.Equal(t, detail.Fields, shadowrocketDetail.Fields)
	require.Len(t, shadowrocketDetail.Lossy, len(detail.Lossy))
	for index := range detail.Lossy {
		require.Equal(t, detail.Lossy[index].IRField, shadowrocketDetail.Lossy[index].IRField)
		require.Equal(t, detail.Lossy[index].Protocol, shadowrocketDetail.Lossy[index].Protocol)
		require.Equal(t, detail.Lossy[index].Status, shadowrocketDetail.Lossy[index].Status)
		require.Equal(t, detail.Lossy[index].SourceRef, shadowrocketDetail.Lossy[index].SourceRef)
		require.Contains(t, shadowrocketDetail.Lossy[index].Notes, "shadowrocket-proxies")
	}
	require.Equal(t, []string{"v1.19.25"}, seen["shadowrocket-proxies\x00render"].Revisions)
}

func TestGetFormatCapabilityRejectsUnknownKeys(t *testing.T) {
	svc := service.New()
	for _, req := range []domain.FormatCapabilityRequest{
		{Direction: "future", Format: "uri-list"},
		{Direction: domain.CapabilityDirectionRender},
		{Direction: domain.CapabilityDirectionRender, Format: "future"},
	} {
		_, err := svc.GetFormatCapability(context.Background(), req)
		require.Error(t, err)
		require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	}
}

func TestInspectReturnsLightweightRegisteredCapabilities(t *testing.T) {
	result, err := service.New().Inspect(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"base64", "json-nodes", "mihomo", "sing-box", "uri", "uri-list"}, result.Formats.Parse)
	require.Equal(t, []string{"base64", "json-nodes", "mihomo-proxies", "shadowrocket-proxies", "sing-box-outbounds", "uri-list"}, result.Formats.Render)
	require.NotContains(t, result.Processors.File, "inject_nodes")
	require.Equal(t, []domain.FileKind{domain.FileKindStatic, domain.FileKindMihomo, domain.FileKindSingBox, domain.FileKindShadowrocket}, result.FileKinds)
	require.Contains(t, result.Probe.Methods, domain.ProbeTCPConnect)
	backendMethods := make([]domain.ProbeMethod, 0, len(result.Probe.Backends))
	seenMethods := map[domain.ProbeMethod]bool{}
	for _, backend := range result.Probe.Backends {
		if !seenMethods[backend.Method] {
			seenMethods[backend.Method] = true
			backendMethods = append(backendMethods, backend.Method)
		}
	}
	sort.Slice(backendMethods, func(i, j int) bool { return backendMethods[i] < backendMethods[j] })
	require.Equal(t, backendMethods, result.Probe.Methods)
	require.False(t, result.Store.Configured)
	require.Nil(t, result.Store.Subscriptions)
	require.Nil(t, result.Store.Files)
}

func formatsForDirection(items []domain.FormatCapabilitySummary, direction domain.CapabilityDirection) []string {
	formats := []string{}
	for _, item := range items {
		if item.Direction == direction {
			formats = append(formats, item.Format)
		}
	}
	return formats
}

func TestServiceRendersBase64Subscription(t *testing.T) {
	svc := service.New()
	result, err := svc.Render(context.Background(), domain.RenderRequest{
		Format: "base64",
		Nodes: []domain.NodeIR{{
			Type:     domain.NodeTypeShadowsocks,
			Name:     "node",
			Server:   "example.com",
			Port:     8388,
			Cipher:   "aes-128-gcm",
			Password: "secret",
		}},
	})

	require.NoError(t, err)
	require.Equal(t, "text/plain; charset=utf-8", result.ContentType)
	decoded, err := base64.StdEncoding.DecodeString(string(result.Body))
	require.NoError(t, err)
	require.Equal(t, "ss://YWVzLTEyOC1nY206c2VjcmV0@example.com:8388#node", string(decoded))
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
