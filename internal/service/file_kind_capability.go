package service

import "github.com/kuuvahki-labs/sandrone/internal/domain"

// FileKindSourceRules describes which FileSource forms a kind accepts.
// Typed kinds may omit source to use their built-in base document.
type FileKindSourceRules struct {
	Required     bool     `json:"required"`
	AllowedTypes []string `json:"allowed_types"`
}

// FileKindCapability is the public, descriptive view of one file kind.
// SettingsPrototype drives schema reflection; the real driver remains the
// final settings validator.
type FileKindCapability struct {
	Kind              domain.FileKind     `json:"kind"`
	Description       string              `json:"description"`
	SettingsPrototype any                 `json:"-"`
	MediaType         string              `json:"media_type"`
	Syntax            string              `json:"syntax"`
	DefaultExtension  string              `json:"default_extension"`
	SourceRules       FileKindSourceRules `json:"source_rules"`
	Defaults          map[string]any      `json:"defaults"`
	Examples          []map[string]any    `json:"examples"`
}

// FileKindCapabilities returns immutable copies in canonical kind order.
func (s *Service) FileKindCapabilities() []FileKindCapability {
	capabilities := []FileKindCapability{staticFileKindCapability()}
	for _, kind := range []domain.FileKind{
		domain.FileKindMihomo,
		domain.FileKindSingBox,
		domain.FileKindShadowrocket,
	} {
		driver, err := s.typedFiles.Lookup(kind)
		if err != nil {
			continue
		}
		capabilities = append(capabilities, capabilityFromTypedFileDescriptor(driver.Descriptor()))
	}
	return cloneFileKindCapabilities(capabilities)
}

func staticFileKindCapability() FileKindCapability {
	return FileKindCapability{
		Kind:             domain.FileKindStatic,
		Description:      "Serve source content directly before applying file processors.",
		MediaType:        "application/octet-stream",
		Syntax:           "text",
		DefaultExtension: ".txt",
		SourceRules: FileKindSourceRules{
			Required:     true,
			AllowedTypes: []string{"inline", "remote"},
		},
		Defaults: map[string]any{},
		Examples: []map[string]any{{
			"name": "example.txt",
			"kind": string(domain.FileKindStatic),
			"source": map[string]any{
				"type":    "inline",
				"content": "hello\n",
			},
		}},
	}
}

func capabilityFromTypedFileDescriptor(descriptor typedFileDescriptor) FileKindCapability {
	return FileKindCapability{
		Kind:              descriptor.Kind,
		Description:       descriptor.Description,
		SettingsPrototype: descriptor.SettingsPrototype,
		MediaType:         descriptor.MediaType,
		Syntax:            descriptor.Syntax,
		DefaultExtension:  descriptor.DefaultExtension,
		SourceRules:       cloneFileKindSourceRules(descriptor.SourceRules),
		Defaults:          cloneAnyMap(descriptor.Defaults),
		Examples:          cloneExamples(descriptor.Examples),
	}
}

func cloneFileKindCapabilities(input []FileKindCapability) []FileKindCapability {
	cloned := make([]FileKindCapability, len(input))
	for i, capability := range input {
		capability.SourceRules = cloneFileKindSourceRules(capability.SourceRules)
		capability.Defaults = cloneAnyMap(capability.Defaults)
		capability.Examples = cloneExamples(capability.Examples)
		cloned[i] = capability
	}
	return cloned
}

func cloneFileKindSourceRules(rules FileKindSourceRules) FileKindSourceRules {
	rules.AllowedTypes = append([]string(nil), rules.AllowedTypes...)
	return rules
}

func cloneExamples(examples []map[string]any) []map[string]any {
	if examples == nil {
		return nil
	}
	cloned := make([]map[string]any, len(examples))
	for i, example := range examples {
		cloned[i] = cloneAnyMap(example)
	}
	return cloned
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case map[string]any:
			cloned[key] = cloneAnyMap(typed)
		case []any:
			items := make([]any, len(typed))
			for i, item := range typed {
				if child, ok := item.(map[string]any); ok {
					items[i] = cloneAnyMap(child)
				} else {
					items[i] = item
				}
			}
			cloned[key] = items
		case []string:
			cloned[key] = append([]string(nil), typed...)
		default:
			cloned[key] = value
		}
	}
	return cloned
}
