package file

import "github.com/kuuvahki-labs/sandrone/internal/processor"

// Register installs all built-in file-stage processors into r.
func Register(r *processor.Registry) {
	r.RegisterFile("inject_nodes", buildInjectNodes)
	r.RegisterFile("merge", buildMerge)
	r.RegisterFile("yaml_patch", buildYAMLPatch)
	r.RegisterFile("json_patch", buildJSONPatch)
	r.RegisterFile("template", buildTemplate)
}
