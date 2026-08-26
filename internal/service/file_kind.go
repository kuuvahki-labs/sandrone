package service

import "github.com/kuuvahki-labs/sandrone/internal/filekind"

// FileKindCapabilities returns immutable file-kind descriptions in canonical order.
func (s *Service) FileKindCapabilities() []filekind.Capability {
	return s.typedFiles.Capabilities()
}
