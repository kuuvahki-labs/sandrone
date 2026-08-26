package agentcatalog

import (
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/filekind"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

type ProcessorSummaryDocument struct {
	Processors []ProcessorSummaryEntry `json:"processors"`
}

type ProcessorSummaryEntry struct {
	Type        string            `json:"type"`
	Stage       domain.Stage      `json:"stage"`
	Description string            `json:"description"`
	Effects     processor.Effects `json:"effects"`
}

type FileKindSummaryDocument struct {
	FileKinds []FileKindSummaryEntry `json:"file_kinds"`
}

type FileKindSummaryEntry struct {
	Kind              domain.FileKind `json:"kind"`
	Description       string          `json:"description"`
	SettingsSupported bool            `json:"settings_supported"`
	MediaType         string          `json:"media_type"`
	Syntax            string          `json:"syntax"`
}

type SchemaSummaryDocument struct {
	Schemas []SchemaSummaryEntry `json:"schemas"`
}

type SchemaSummaryEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func ProcessorSummary(descriptors []processor.Descriptor) ProcessorSummaryDocument {
	items := make([]ProcessorSummaryEntry, len(descriptors))
	for index, descriptor := range descriptors {
		items[index] = ProcessorSummaryEntry{
			Type: descriptor.Type, Stage: descriptor.Stage,
			Description: descriptor.Description, Effects: descriptor.Effects,
		}
	}
	return ProcessorSummaryDocument{Processors: items}
}

func FileKindSummary(capabilities []filekind.Capability) FileKindSummaryDocument {
	items := make([]FileKindSummaryEntry, len(capabilities))
	for index, capability := range capabilities {
		items[index] = FileKindSummaryEntry{
			Kind: capability.Kind, Description: capability.Description,
			SettingsSupported: capability.SettingsPrototype != nil,
			MediaType:         capability.MediaType, Syntax: capability.Syntax,
		}
	}
	return FileKindSummaryDocument{FileKinds: items}
}

func SchemaSummary() SchemaSummaryDocument {
	return SchemaSummaryDocument{Schemas: []SchemaSummaryEntry{
		{Name: "processors", Description: "Public processor catalog and parameter schemas."},
		{Name: "file_kinds", Description: "Canonical file-kind catalog and settings schemas."},
		{Name: "script_api_v1", Description: "Version 1 sandboxed script API schema."},
		{Name: "subscription", Description: "Stored Subscription write schema."},
		{Name: "file_spec", Description: "Stored FileSpec write schema."},
	}}
}
