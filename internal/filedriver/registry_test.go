package filedriver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/filekind"
)

type registryTestDriver struct {
	descriptor Descriptor
}

func (d registryTestDriver) Descriptor() Descriptor               { return d.descriptor }
func (registryTestDriver) ValidateSettings(json.RawMessage) error { return nil }
func (registryTestDriver) Compile(context.Context, CompileInput) ([]byte, error) {
	return []byte("compiled"), nil
}

func TestRegistryRejectsDuplicateRegistration(t *testing.T) {
	registry := &Registry{drivers: map[domain.FileKind]Driver{}}
	driver := registryTestDriver{descriptor: completeTestDescriptor(domain.FileKindMihomo)}
	require.NoError(t, registry.register(driver))

	err := registry.register(driver)

	require.ErrorContains(t, err, `typed file driver "mihomo" is already registered`)
}

func TestRegistryRequiresCompleteDescriptor(t *testing.T) {
	tests := []struct {
		name       string
		descriptor Descriptor
		want       string
	}{
		{name: "empty kind", descriptor: Descriptor{}, want: "kind is required"},
		{name: "static kind", descriptor: Descriptor{Kind: domain.FileKindStatic}, want: `kind "static" is reserved`},
		{name: "media type", descriptor: Descriptor{Kind: "future"}, want: `driver "future" media type is required`},
		{name: "syntax", descriptor: Descriptor{Kind: "future", MediaType: "text/plain"}, want: `driver "future" syntax is required`},
		{name: "extension", descriptor: Descriptor{Kind: "future", MediaType: "text/plain", Syntax: "text"}, want: `driver "future" default extension is required`},
		{name: "settings prototype", descriptor: Descriptor{Kind: "future", MediaType: "text/plain", Syntax: "text", DefaultExtension: ".txt"}, want: `driver "future" settings prototype is required`},
		{name: "source rules", descriptor: Descriptor{Kind: "future", MediaType: "text/plain", Syntax: "text", DefaultExtension: ".txt", SettingsPrototype: struct{}{}}, want: `driver "future" source rules are required`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := &Registry{drivers: map[domain.FileKind]Driver{}}
			err := registry.register(registryTestDriver{descriptor: test.descriptor})
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestRegistryRejectsUnknownKind(t *testing.T) {
	_, err := New().Lookup(domain.FileKind("future"))

	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	require.ErrorContains(t, err, `file kind "future" is not registered`)
}

func completeTestDescriptor(kind domain.FileKind) Descriptor {
	return Descriptor{
		Kind: kind, MediaType: "application/yaml", Syntax: "yaml",
		DefaultExtension: ".yaml", NodeRenderFormat: "mihomo-proxies",
		SettingsPrototype: struct{}{}, SourceRules: filekind.SourceRules{AllowedTypes: []string{"inline"}},
	}
}
