package service

import (
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) validateFileSpecStructure(spec domain.FileSpec) error {
	if spec.Kind == "" {
		return domain.NewError(domain.CodeInvalidArgument, "file kind is required")
	}
	sourceType := strings.ToLower(strings.TrimSpace(spec.Source.Type))
	if sourceType != "" && sourceType != "inline" && sourceType != "remote" {
		return domain.NewError(domain.CodeInvalidArgument, "file source type must be inline or remote")
	}
	if sourceType == "" && (spec.Source.Content != "" || spec.Source.Remote != nil) {
		return domain.NewError(domain.CodeInvalidArgument, "empty file source must not include content or remote")
	}
	if spec.Kind == domain.FileKindStatic {
		if spec.Config != nil {
			return domain.NewError(domain.CodeInvalidArgument, `file kind "static" does not allow config`)
		}
		if sourceType == "" {
			return domain.NewError(domain.CodeInvalidArgument, `file kind "static" requires an inline or remote source`)
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
