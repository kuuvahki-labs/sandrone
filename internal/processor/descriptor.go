package processor

import "github.com/kuuvahki-labs/sandrone/internal/domain"

// Effects identifies externally observable work a processor can perform.
type Effects struct {
	Probes      bool `json:"probes,omitempty"`
	RemoteReads bool `json:"remote_reads,omitempty"`
	RunsScript  bool `json:"runs_script,omitempty"`
}

// Descriptor is owner-maintained metadata for a registered processor.
// ParamsPrototype is descriptive only; the registered builder remains the
// final authority for parameter validation.
type Descriptor struct {
	Type            string
	Stage           domain.Stage
	Description     string
	ParamsPrototype any
	Effects         Effects
	Examples        []map[string]any
	ErrorCodes      []domain.ErrorCode
	Public          bool
}

func cloneDescriptor(descriptor Descriptor) Descriptor {
	descriptor.Examples = cloneExamples(descriptor.Examples)
	descriptor.ErrorCodes = append([]domain.ErrorCode(nil), descriptor.ErrorCodes...)
	return descriptor
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
