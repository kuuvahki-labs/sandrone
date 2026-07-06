package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type typedFileDescriptor struct {
	Kind             domain.FileKind
	MediaType        string
	Syntax           string
	DefaultExtension string
	DefaultBase      []byte
	NodeRenderFormat string
}

type typedFileCompileInput struct {
	Base          []byte
	RenderedNodes []byte
	Settings      json.RawMessage
}

type typedFileDriver interface {
	Descriptor() typedFileDescriptor
	ValidateSettings(json.RawMessage) error
	Compile(context.Context, typedFileCompileInput) ([]byte, error)
}

type typedFileRegistry struct {
	drivers map[domain.FileKind]typedFileDriver
}

func newTypedFileRegistry() *typedFileRegistry {
	return &typedFileRegistry{drivers: map[domain.FileKind]typedFileDriver{}}
}

func (r *typedFileRegistry) Register(driver typedFileDriver) error {
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
	if descriptor.NodeRenderFormat == "" {
		return fmt.Errorf("typed file driver %q node render format is required", kind)
	}
	if _, exists := r.drivers[kind]; exists {
		return fmt.Errorf("typed file driver %q is already registered", kind)
	}
	r.drivers[kind] = driver
	return nil
}

func (r *typedFileRegistry) Lookup(kind domain.FileKind) (typedFileDriver, error) {
	driver, ok := r.drivers[kind]
	if !ok {
		return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("file kind %q is not registered", kind))
	}
	return driver, nil
}
