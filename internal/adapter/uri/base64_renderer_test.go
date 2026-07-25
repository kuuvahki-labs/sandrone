package uri_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/adapter/uri"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestBase64RendererEncodesExactURIListBytes(t *testing.T) {
	nodes := []domain.NodeIR{{
		Type:     domain.NodeTypeShadowsocks,
		Name:     "node",
		Server:   "example.com",
		Port:     8388,
		Cipher:   "aes-128-gcm",
		Password: "secret",
	}}
	plain, plainReport, err := uri.NewRenderer().RenderWithReport(
		context.Background(), nodes, domain.RenderOptions{},
	)
	require.NoError(t, err)

	encoded, encodedReport, err := uri.NewBase64Renderer(uri.NewRenderer()).RenderWithReport(
		context.Background(), nodes, domain.RenderOptions{},
	)
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(string(encoded))
	require.NoError(t, err)
	require.Equal(t, plain, decoded)
	require.Equal(t, plainReport, encodedReport)
}

func TestBase64RendererPreservesURIListFailureReport(t *testing.T) {
	nodes := []domain.NodeIR{{Type: domain.NodeTypeWireGuard, Name: "unsupported"}}
	_, plainReport, plainErr := uri.NewRenderer().RenderWithReport(
		context.Background(), nodes, domain.RenderOptions{},
	)

	encoded, encodedReport, encodedErr := uri.NewBase64Renderer(uri.NewRenderer()).RenderWithReport(
		context.Background(), nodes, domain.RenderOptions{},
	)

	require.Error(t, plainErr)
	require.Error(t, encodedErr)
	require.Nil(t, encoded)
	require.Equal(t, plainReport, encodedReport)
	require.Equal(t, plainErr.Error(), encodedErr.Error())
}

func TestBase64RendererReportsRenderCapability(t *testing.T) {
	capabilities := uri.NewBase64Renderer(uri.NewRenderer()).RenderCapabilities()

	require.Len(t, capabilities, 1)
	require.Equal(t, "base64", capabilities[0].Format)
	require.Equal(t, shared.DirectionRender, capabilities[0].Direction)
	require.ElementsMatch(t, shared.URIProfileNodeTypes(), capabilities[0].Types)
}
