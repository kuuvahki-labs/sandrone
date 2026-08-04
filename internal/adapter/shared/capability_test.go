package shared_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestNodeTypeLists(t *testing.T) {
	require.Equal(t, []domain.NodeType{
		domain.NodeTypeShadowsocks,
		domain.NodeTypeShadowsocksR,
		domain.NodeTypeVMess,
		domain.NodeTypeVLESS,
		domain.NodeTypeTrojan,
		domain.NodeTypeHysteria,
		domain.NodeTypeHysteria2,
		domain.NodeTypeTUIC,
		domain.NodeTypeMieru,
		domain.NodeTypeSOCKS,
		domain.NodeTypeHTTP,
		domain.NodeTypeWireGuard,
		domain.NodeTypeSnell,
		domain.NodeTypeAnyTLS,
	}, shared.AllNodeTypes())

	uriTypes := shared.URIProfileNodeTypes()
	require.Contains(t, uriTypes, domain.NodeTypeShadowsocksR)
	require.Contains(t, uriTypes, domain.NodeTypeMieru)
	require.NotContains(t, uriTypes, domain.NodeTypeWireGuard)
	require.NotContains(t, uriTypes, domain.NodeTypeSnell)
	require.Contains(t, uriTypes, domain.NodeTypeHTTP)
	require.Contains(t, uriTypes, domain.NodeTypeAnyTLS)
	require.NotContains(t, shared.SingBoxNodeTypes(), domain.NodeTypeMieru)
	require.Contains(t, shared.SingBoxNodeTypes(), domain.NodeTypeAnyTLS)
}

func TestCapabilityForCopiesTypesAndBuildsFieldRefs(t *testing.T) {
	types := []domain.NodeType{domain.NodeTypeVMess}
	capability := shared.CapabilityFor("uri-list", shared.DirectionParse, types, true)
	types[0] = domain.NodeTypeHTTP

	require.Equal(t, "uri-list", capability.Format)
	require.Equal(t, shared.DirectionParse, capability.Direction)
	require.True(t, capability.Reversible)
	require.Equal(t, []domain.NodeType{domain.NodeTypeVMess}, capability.Types)
	require.Contains(t, irFields(capability.Fields), "uuid")
	require.NotContains(t, irFields(capability.Fields), "multiplex")
	require.Equal(t, "vmess", capability.Fields[0].Protocol)
	require.Equal(t, "vmess", capability.Fields[0].SourceRef.Name)
	require.Equal(t, shared.FieldStatusSupported, capability.Fields[0].Status)
}

func TestMihomoCapabilitiesExposeAdvancedSnellAndAnyTLSFields(t *testing.T) {
	t.Parallel()

	fields := irFields(shared.CapabilityFor("mihomo-proxies", shared.DirectionRender, []domain.NodeType{
		domain.NodeTypeSnell,
		domain.NodeTypeAnyTLS,
	}, false).Fields)
	for _, field := range []string{
		"snell.reuse", "snell.client_fingerprint", "snell.shadow_tls",
		"anytls.idle_session_check_interval", "anytls.idle_session_timeout", "anytls.min_idle_session",
	} {
		require.Contains(t, fields, field)
	}
}

func TestCapabilityUsesTargetSchemaSourcesAndLossyCatalog(t *testing.T) {
	capability := shared.CapabilityFor("sing-box-outbounds", shared.DirectionRender, []domain.NodeType{
		domain.NodeTypeVLESS,
		domain.NodeTypeHysteria2,
		domain.NodeTypeTUIC,
	}, false)
	gotFields := irFields(capability.Fields)

	require.Contains(t, gotFields, "uuid")
	require.Contains(t, gotFields, "hysteria.up_mbps")
	require.Contains(t, gotFields, "tuic.udp_over_stream")
	require.NotContains(t, gotFields, "encryption")
	require.Contains(t, irFields(capability.Lossy), "encryption")
	require.Contains(t, irFields(capability.Lossy), "hysteria.realm")
	require.Contains(t, irFields(capability.Lossy), "tuic.reduce_rtt")

	for _, field := range capability.Fields {
		require.Equal(t, shared.FieldStatusSupported, field.Status)
		require.NotEmpty(t, field.SourceRef.Name)
		if field.Protocol == string(domain.NodeTypeVLESS) {
			require.Equal(t, "sing-box vless outbound schema", field.SourceRef.Name)
		}
	}
	for _, field := range capability.Lossy {
		require.Equal(t, shared.FieldStatusLossy, field.Status)
		require.NotEmpty(t, field.SourceRef.Path)
	}
}

func TestFieldRefsCoverTargetSpecificFieldsAndRawOnlyCatalog(t *testing.T) {
	fields := shared.FieldRefs("mihomo-proxies", []domain.NodeType{
		domain.NodeTypeShadowsocks,
		domain.NodeTypeVLESS,
		domain.NodeTypeHysteria2,
		domain.NodeTypeTUIC,
		domain.NodeTypeWireGuard,
	})
	gotFields := irFields(fields)

	for _, field := range []string{
		"cipher", "udp_over_tcp", "flow", "reality", "password", "tls",
		"hysteria.hop_interval", "hysteria.realm", "token",
		"tuic.udp_over_stream", "wireguard.private_key", "wireguard.allowed_ips",
	} {
		require.Contains(t, gotFields, field)
	}

	capability := shared.CapabilityFor("mihomo", shared.DirectionParse, []domain.NodeType{domain.NodeTypeVLESS}, false)
	require.Contains(t, irFields(capability.RawOnly), "mihomo.xhttp-opts.x-padding-bytes")
	require.NotContains(t, irFields(capability.RawOnly), "mihomo.xhttp-opts.mode")
	require.Equal(t, shared.FieldStatusRawOnly, capability.RawOnly[0].Status)
}

func TestURICapabilityDeclaresVLESSPacketEncodingRoundtrip(t *testing.T) {
	capability := shared.CapabilityFor("uri-list", shared.DirectionRender, []domain.NodeType{domain.NodeTypeVLESS}, true)

	require.Contains(t, irFields(capability.Fields), "packet_encoding")
	require.NotContains(t, irFields(capability.Lossy), "packet_encoding")
}

func TestMihomoAndURIRenderCapabilitiesDeclareCanonicalNetworkLoss(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"mihomo-proxies", "uri-list"} {
		capability := shared.CapabilityFor(format, shared.DirectionRender, []domain.NodeType{
			domain.NodeTypeVMess,
			domain.NodeTypeVLESS,
			domain.NodeTypeTrojan,
		}, false)
		require.Contains(t, irFields(capability.Lossy), "network", format)
	}
}

func TestMieruCapabilities(t *testing.T) {
	mihomo := shared.CapabilityFor("mihomo-proxies", shared.DirectionRender, []domain.NodeType{domain.NodeTypeMieru}, false)
	for _, field := range []string{
		"name", "type", "server", "port", "username", "password", "dialer.udp_relay",
		"mieru.port_range", "mieru.transport", "mieru.multiplexing", "mieru.handshake_mode", "mieru.traffic_pattern",
	} {
		require.Contains(t, irFields(mihomo.Fields), field)
	}

	uri := shared.CapabilityFor("uri-list", shared.DirectionRender, []domain.NodeType{domain.NodeTypeMieru}, false)
	require.Contains(t, irFields(uri.Fields), "mieru.port_range")
	require.Contains(t, irFields(uri.Fields), "mieru.transport")
}

func irFields(fields []shared.FieldRef) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		out = append(out, field.IRField)
	}
	return out
}
