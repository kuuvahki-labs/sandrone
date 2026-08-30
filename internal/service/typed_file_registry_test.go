package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type warningTestRenderer struct{}

func (warningTestRenderer) Name() string { return "warning-test" }
func (warningTestRenderer) Render(context.Context, []domain.NodeIR, domain.RenderOptions) ([]byte, error) {
	return []byte("proxies: []"), nil
}
func (warningTestRenderer) RenderWithReport(context.Context, []domain.NodeIR, domain.RenderOptions) ([]byte, domain.RenderReport, error) {
	return []byte("proxies: []"), domain.RenderReport{Warnings: []domain.Warning{{Code: "render_lossy_field", Field: "tls.fingerprint"}}}, nil
}

func TestTypedFileCompilationReturnsNodeRendererWarnings(t *testing.T) {
	svc := New()
	svc.renderers["mihomo-proxies"] = warningTestRenderer{}
	spec := domain.FileSpec{
		Name: "warnings.yaml", Kind: domain.FileKindMihomo,
		Config: &domain.FileConfig{Settings: json.RawMessage(`{"groups":[],"rule_sets":[],"rules":[]}`)},
	}

	result, err := svc.GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Equal(t, []domain.Warning{{Code: "render_lossy_field", Field: "tls.fingerprint"}}, result.Report.Warnings)
}

func TestTypedFileValidationNamesMissingRenderer(t *testing.T) {
	svc := New()
	delete(svc.renderers, "sing-box-outbounds")
	spec := domain.FileSpec{Name: "missing.json", Kind: domain.FileKindSingBox}

	_, err := svc.GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	require.ErrorContains(t, err, `file kind "sing-box" requires node renderer "sing-box-outbounds"`)
}
