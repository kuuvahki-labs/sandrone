package file

import (
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

// Register installs all built-in file-stage processors into r.
func Register(r *processor.Registry) {
	r.RegisterFileWithDescriptor("inject_nodes", buildInjectNodes, processor.Descriptor{
		Description:     "Inject a rendered node part into a structured file.",
		ParamsPrototype: InjectNodesParams{},
		Examples:        []map[string]any{{"path": "/proxies", "mode": "replace"}},
		ErrorCodes:      []domain.ErrorCode{domain.CodeProcessorConfigInvalid, domain.CodeFileProcessorFailed},
	})
	r.RegisterFileWithDescriptor("merge", buildMerge, processor.Descriptor{
		Description:     "Merge inline content into the current file.",
		ParamsPrototype: MergeParams{}, Public: true,
		Examples:   []map[string]any{{"mode": "yaml_overlay", "content": "rules:\n  - MATCH,DIRECT\n"}},
		ErrorCodes: []domain.ErrorCode{domain.CodeProcessorConfigInvalid, domain.CodeFileMergeFailed},
	})
	patchDescriptor := func(description string) processor.Descriptor {
		return processor.Descriptor{
			Description: description, ParamsPrototype: PatchParams{}, Public: true,
			Examples:   []map[string]any{{"ops": []any{map[string]any{"op": "add", "path": "/enabled", "value": true}}}},
			ErrorCodes: []domain.ErrorCode{domain.CodeProcessorConfigInvalid, domain.CodeFileProcessorFailed},
		}
	}
	r.RegisterFileWithDescriptor("yaml_patch", buildYAMLPatch, patchDescriptor("Apply RFC 6902-style operations to a YAML file."))
	r.RegisterFileWithDescriptor("json_patch", buildJSONPatch, patchDescriptor("Apply RFC 6902 operations to a JSON file."))
	r.RegisterFileWithDescriptor("template", buildTemplate, processor.Descriptor{
		Description:     "Substitute request metadata and explicit string variables in a file.",
		ParamsPrototype: TemplateParams{}, Public: true,
		Examples:   []map[string]any{{"vars": map[string]any{"region": "HK"}}},
		ErrorCodes: []domain.ErrorCode{domain.CodeProcessorConfigInvalid, domain.CodeFileProcessorFailed},
	})
}
