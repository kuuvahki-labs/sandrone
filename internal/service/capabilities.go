package service

import (
	"context"
	"sort"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) ListFormatCapabilities(context.Context) (*domain.FormatCapabilityListResult, error) {
	capabilities := s.adapterCapabilities()
	items := make([]domain.FormatCapabilitySummary, len(capabilities))
	for index, capability := range capabilities {
		items[index] = summarizeFormatCapability(capability)
	}
	return &domain.FormatCapabilityListResult{Items: items}, nil
}

func (s *Service) GetFormatCapability(_ context.Context, req domain.FormatCapabilityRequest) (*domain.FormatCapability, error) {
	if req.Direction != domain.CapabilityDirectionParse && req.Direction != domain.CapabilityDirectionRender {
		return nil, domain.NewError(domain.CodeInvalidArgument, "capability direction must be parse or render")
	}
	req.Format = strings.TrimSpace(req.Format)
	if req.Format == "" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "capability format is required")
	}
	for _, capability := range s.adapterCapabilities() {
		if capability.Direction == req.Direction && capability.Format == req.Format {
			cloned := cloneFormatCapability(capability)
			return &cloned, nil
		}
	}
	return nil, domain.NewError(domain.CodeInvalidArgument, "format capability not found")
}

func summarizeFormatCapability(capability domain.FormatCapability) domain.FormatCapabilitySummary {
	revisions := map[string]bool{}
	for _, fields := range [][]domain.CapabilityFieldRef{capability.Fields, capability.Lossy, capability.RawOnly} {
		for _, field := range fields {
			revision := strings.TrimSpace(field.SourceRef.Revision)
			if revision != "" {
				revisions[revision] = true
			}
		}
	}
	revisionList := make([]string, 0, len(revisions))
	for revision := range revisions {
		revisionList = append(revisionList, revision)
	}
	sort.Strings(revisionList)
	return domain.FormatCapabilitySummary{
		Direction:  capability.Direction,
		Format:     capability.Format,
		NodeTypes:  append([]domain.NodeType(nil), capability.Types...),
		Reversible: capability.Reversible,
		FieldCounts: domain.FormatCapabilityFieldCounts{
			Supported: len(capability.Fields),
			Lossy:     len(capability.Lossy),
			RawOnly:   len(capability.RawOnly),
		},
		Revisions: revisionList,
	}
}

func cloneFormatCapability(capability domain.FormatCapability) domain.FormatCapability {
	capability.Types = append([]domain.NodeType(nil), capability.Types...)
	capability.Fields = append([]domain.CapabilityFieldRef(nil), capability.Fields...)
	capability.Lossy = append([]domain.CapabilityFieldRef(nil), capability.Lossy...)
	capability.RawOnly = append([]domain.CapabilityFieldRef(nil), capability.RawOnly...)
	return capability
}
