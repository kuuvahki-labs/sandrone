package singbox_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/singbox"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestParseSingBoxMieruUnsupported(t *testing.T) {
	parser := singbox.NewParser()
	_, _, err := parser.Parse(context.Background(), []byte(`{"type":"mieru","tag":"m"}`))

	require.ErrorContains(t, err, "unsupported sing-box outbound type")
}

func TestRenderSingBoxMieruUnsupported(t *testing.T) {
	r := singbox.NewRenderer()
	_, _, err := r.RenderWithReport(context.Background(), []domain.NodeIR{{
		Name: "mieru",
		Type: domain.NodeTypeMieru,
	}}, domain.RenderOptions{Format: "sing-box-outbounds"})

	require.ErrorContains(t, err, "unsupported node type")
}
