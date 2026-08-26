package filedriver

import (
	"slices"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/filekind"
)

// Capabilities returns immutable copies in canonical kind order.
func (r *Registry) Capabilities() []filekind.Capability {
	capabilities := []filekind.Capability{staticCapability()}
	for _, kind := range []domain.FileKind{
		domain.FileKindMihomo,
		domain.FileKindSingBox,
		domain.FileKindShadowrocket,
	} {
		driver, err := r.Lookup(kind)
		if err != nil {
			continue
		}
		capabilities = append(capabilities, capabilityFromDescriptor(driver.Descriptor()))
	}
	return cloneCapabilities(capabilities)
}

func staticCapability() filekind.Capability {
	return filekind.Capability{
		Kind:             domain.FileKindStatic,
		Description:      "Serve source content directly before applying file processors.",
		MediaType:        "application/octet-stream",
		Syntax:           "text",
		DefaultExtension: ".txt",
		SourceRules: filekind.SourceRules{
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

func capabilityFromDescriptor(descriptor Descriptor) filekind.Capability {
	return filekind.Capability{
		Kind:              descriptor.Kind,
		Description:       descriptor.Description,
		SettingsPrototype: descriptor.SettingsPrototype,
		MediaType:         descriptor.MediaType,
		Syntax:            descriptor.Syntax,
		DefaultExtension:  descriptor.DefaultExtension,
		SourceRules:       cloneSourceRules(descriptor.SourceRules),
		Defaults:          cloneAnyMap(descriptor.Defaults),
		Examples:          cloneExamples(descriptor.Examples),
	}
}

func cloneCapabilities(input []filekind.Capability) []filekind.Capability {
	cloned := make([]filekind.Capability, len(input))
	for i, capability := range input {
		capability.SourceRules = cloneSourceRules(capability.SourceRules)
		capability.Defaults = cloneAnyMap(capability.Defaults)
		capability.Examples = cloneExamples(capability.Examples)
		cloned[i] = capability
	}
	return cloned
}

func cloneSourceRules(rules filekind.SourceRules) filekind.SourceRules {
	rules.AllowedTypes = slices.Clone(rules.AllowedTypes)
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
			cloned[key] = slices.Clone(typed)
		default:
			cloned[key] = value
		}
	}
	return cloned
}
