package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type registryTestDriver struct {
	descriptor typedFileDescriptor
}

type warningTestRenderer struct{}

func (warningTestRenderer) Name() string { return "warning-test" }
func (warningTestRenderer) Render(context.Context, []domain.NodeIR, domain.RenderOptions) ([]byte, error) {
	return []byte("proxies: []"), nil
}
func (warningTestRenderer) RenderWithReport(context.Context, []domain.NodeIR, domain.RenderOptions) ([]byte, domain.RenderReport, error) {
	return []byte("proxies: []"), domain.RenderReport{Warnings: []domain.Warning{{Code: "render_lossy_field", Field: "tls.fingerprint"}}}, nil
}

func (d registryTestDriver) Descriptor() typedFileDescriptor      { return d.descriptor }
func (registryTestDriver) ValidateSettings(json.RawMessage) error { return nil }
func (registryTestDriver) Compile(context.Context, typedFileCompileInput) ([]byte, error) {
	return []byte("compiled"), nil
}

func TestTypedFileRegistryRejectsDuplicateRegistration(t *testing.T) {
	registry := newTypedFileRegistry()
	driver := registryTestDriver{descriptor: typedFileDescriptor{
		Kind: domain.FileKindMihomo, MediaType: "application/yaml", Syntax: "yaml",
		DefaultExtension: ".yaml", NodeRenderFormat: "mihomo-proxies",
		SettingsPrototype: struct{}{}, SourceRules: FileKindSourceRules{AllowedTypes: []string{"inline"}},
	}}
	require.NoError(t, registry.Register(driver))

	err := registry.Register(driver)

	require.ErrorContains(t, err, `typed file driver "mihomo" is already registered`)
}

func TestTypedFileRegistryRequiresCompleteDescriptor(t *testing.T) {
	tests := []struct {
		name       string
		descriptor typedFileDescriptor
		want       string
	}{
		{name: "empty kind", descriptor: typedFileDescriptor{}, want: "kind is required"},
		{name: "static kind", descriptor: typedFileDescriptor{Kind: domain.FileKindStatic}, want: `kind "static" is reserved`},
		{name: "media type", descriptor: typedFileDescriptor{Kind: "future"}, want: `driver "future" media type is required`},
		{name: "syntax", descriptor: typedFileDescriptor{Kind: "future", MediaType: "text/plain"}, want: `driver "future" syntax is required`},
		{name: "extension", descriptor: typedFileDescriptor{Kind: "future", MediaType: "text/plain", Syntax: "text"}, want: `driver "future" default extension is required`},
		{name: "renderer", descriptor: typedFileDescriptor{Kind: "future", MediaType: "text/plain", Syntax: "text", DefaultExtension: ".txt"}, want: `driver "future" node render format is required`},
		{name: "settings prototype", descriptor: typedFileDescriptor{Kind: "future", MediaType: "text/plain", Syntax: "text", DefaultExtension: ".txt", NodeRenderFormat: "text"}, want: `driver "future" settings prototype is required`},
		{name: "source rules", descriptor: typedFileDescriptor{Kind: "future", MediaType: "text/plain", Syntax: "text", DefaultExtension: ".txt", NodeRenderFormat: "text", SettingsPrototype: struct{}{}}, want: `driver "future" source rules are required`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := newTypedFileRegistry().Register(registryTestDriver{descriptor: test.descriptor})
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestTypedFileRegistryRejectsUnknownKind(t *testing.T) {
	registry := newTypedFileRegistry()

	_, err := registry.Lookup(domain.FileKind("future"))

	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	require.ErrorContains(t, err, `file kind "future" is not registered`)
}

func TestTypedFileCompilationReturnsNodeRendererWarnings(t *testing.T) {
	svc := New()
	svc.renderers["mihomo-proxies"] = warningTestRenderer{}
	spec := domain.FileSpec{Name: "warnings.yaml", Kind: domain.FileKindMihomo}

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
