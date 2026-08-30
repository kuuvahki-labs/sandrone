// Package filedriver compiles typed client configurations.
package filedriver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/filekind"
)

// Descriptor declares one typed file format and its public capability data.
type Descriptor struct {
	Kind              domain.FileKind
	Description       string
	MediaType         string
	Syntax            string
	DefaultExtension  string
	DefaultBase       []byte
	NodeRenderFormat  string
	SettingsPrototype any
	SourceRules       filekind.SourceRules
	Defaults          map[string]any
	Examples          []map[string]any
}

// CompileInput contains the resolved base document and, when the driver declares
// a node render format, rendered canonical nodes.
type CompileInput struct {
	Base          []byte
	RenderedNodes []byte
	Settings      json.RawMessage
}

// Driver validates and compiles one typed file format.
type Driver interface {
	Descriptor() Descriptor
	ValidateSettings(json.RawMessage) error
	Compile(context.Context, CompileInput) ([]byte, error)
}

// Registry owns the typed file drivers available to the service.
type Registry struct {
	drivers map[domain.FileKind]Driver
}

// New returns a registry containing all built-in typed file drivers.
func New() *Registry {
	registry := &Registry{drivers: map[domain.FileKind]Driver{}}
	for _, driver := range []Driver{mihomoFileDriver{}, singBoxFileDriver{}, shadowrocketFileDriver{}} {
		if err := registry.register(driver); err != nil {
			panic(err)
		}
	}
	return registry
}

func (r *Registry) register(driver Driver) error {
	if driver == nil {
		return fmt.Errorf("typed file driver is nil")
	}
	kind := driver.Descriptor().Kind
	descriptor := driver.Descriptor()
	if kind == "" {
		return fmt.Errorf("typed file driver kind is required")
	}
	if kind == domain.FileKindStatic {
		return fmt.Errorf("typed file driver kind %q is reserved", kind)
	}
	if descriptor.MediaType == "" {
		return fmt.Errorf("typed file driver %q media type is required", kind)
	}
	if descriptor.Syntax == "" {
		return fmt.Errorf("typed file driver %q syntax is required", kind)
	}
	if descriptor.DefaultExtension == "" {
		return fmt.Errorf("typed file driver %q default extension is required", kind)
	}
	if descriptor.SettingsPrototype == nil {
		return fmt.Errorf("typed file driver %q settings prototype is required", kind)
	}
	if len(descriptor.SourceRules.AllowedTypes) == 0 {
		return fmt.Errorf("typed file driver %q source rules are required", kind)
	}
	if _, exists := r.drivers[kind]; exists {
		return fmt.Errorf("typed file driver %q is already registered", kind)
	}
	r.drivers[kind] = driver
	return nil
}

func (r *Registry) Lookup(kind domain.FileKind) (Driver, error) {
	driver, ok := r.drivers[kind]
	if !ok {
		return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("file kind %q is not registered", kind))
	}
	return driver, nil
}
