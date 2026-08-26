// Package filekind defines the descriptive contract shared by file drivers and catalogs.
package filekind

import "github.com/kuuvahki-labs/sandrone/internal/domain"

// SourceRules describes which FileSource forms a kind accepts.
// Typed kinds may omit source to use their built-in base document.
type SourceRules struct {
	Required     bool     `json:"required"`
	AllowedTypes []string `json:"allowed_types"`
}

// Capability is the public, descriptive view of one file kind.
// SettingsPrototype drives schema reflection; the real driver remains the
// final settings validator.
type Capability struct {
	Kind              domain.FileKind  `json:"kind"`
	Description       string           `json:"description"`
	SettingsPrototype any              `json:"-"`
	MediaType         string           `json:"media_type"`
	Syntax            string           `json:"syntax"`
	DefaultExtension  string           `json:"default_extension"`
	SourceRules       SourceRules      `json:"source_rules"`
	Defaults          map[string]any   `json:"defaults"`
	Examples          []map[string]any `json:"examples"`
}
