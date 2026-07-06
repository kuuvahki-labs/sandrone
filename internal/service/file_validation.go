package service

import (
	"fmt"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) validateFileSpecStructure(spec domain.FileSpec) error {
	if spec.Kind == "" {
		return domain.NewError(domain.CodeInvalidArgument, "file kind is required")
	}
	if spec.Kind == domain.FileKindStatic {
		if spec.Config != nil {
			return domain.NewError(domain.CodeInvalidArgument, `file kind "static" does not allow config`)
		}
		return nil
	}
	driver, err := s.typedFiles.Lookup(spec.Kind)
	if err != nil {
		return err
	}
	descriptor := driver.Descriptor()
	if _, ok := s.renderers[descriptor.NodeRenderFormat]; !ok {
		return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("file kind %q requires node renderer %q", spec.Kind, descriptor.NodeRenderFormat))
	}
	if spec.Config == nil {
		return driver.ValidateSettings(nil)
	}
	return driver.ValidateSettings(spec.Config.Settings)
}
