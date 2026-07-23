// Package file contains built-in file-stage processors for Sandrone file specs.
package file

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

// InjectNodesParams describes how to inject a `nodes` part into the file.
//
// From selects which FilePart (by name) supplies the node payload.
// Defaults to the first part whose role is "nodes".
//
// Path is the JSON Pointer to the target list. A single-segment shorthand
// (e.g. "proxies") is accepted for convenience and treated as "/proxies".
//
// Mode controls how the existing value at Path is combined with the
// injected list. The default "replace" overwrites it; "append" merges the
// two lists in order.
type InjectNodesParams struct {
	From string `json:"from,omitempty" jsonschema:"File part name that contains rendered nodes"`
	Path string `json:"path,omitempty" jsonschema:"JSON Pointer target for the injected list"`
	Mode string `json:"mode,omitempty" jsonschema:"How injected nodes combine with the target list" enum:"replace,append" default:"replace"`
}

// InjectNodes is the file-stage processor that splices a rendered nodes
// part into the FileDocument at a given path. The processor is agnostic to
// the target platform: it autodetects YAML vs JSON via FileDocument.Kind.
type InjectNodes struct {
	params InjectNodesParams
}

// NewInjectNodes returns a processor bound to the supplied parameters.
// path is normalised the same way as ApplyFile (single segment becomes
// "/segment") so callers can pass either spelling.
func NewInjectNodes(target, path string) *InjectNodes {
	return &InjectNodes{params: InjectNodesParams{Path: path}}
}

func buildInjectNodes(spec domain.ProcessorSpec) (domain.FileProcessor, error) {
	var params InjectNodesParams
	if err := processor.UnmarshalParams(spec, &params); err != nil {
		return nil, err
	}
	if params.Mode == "" {
		params.Mode = "replace"
	}
	switch params.Mode {
	case "replace", "append":
	default:
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   fmt.Sprintf("unknown inject_nodes mode %q", params.Mode),
			Processor: spec.Type,
		}
	}
	return &InjectNodes{params: params}, nil
}

func (p *InjectNodes) Name() string { return "inject_nodes" }

func (p *InjectNodes) ApplyFile(_ context.Context, in domain.FileProcessInput) (domain.FileProcessOutput, error) {
	part, ok := selectNodePart(in.Parts, p.params.From)
	if !ok {
		return domain.FileProcessOutput{File: in.File}, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "nodes part not found",
			Processor: p.Name(),
		}
	}
	if len(part.Content) == 0 {
		return domain.FileProcessOutput{File: in.File}, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "nodes part content is empty",
			Processor: p.Name(),
			Part:      part.Name,
		}
	}
	path := normalisePath(p.params.Path, in.File.Kind)
	if path == "" {
		return domain.FileProcessOutput{File: in.File}, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "inject_nodes path is required",
			Processor: p.Name(),
		}
	}

	mode := p.params.Mode
	if mode == "" {
		mode = "replace"
	}

	switch fileFormat(in.File.Kind, part.Kind) {
	case "yaml":
		body, err := injectYAML(in.File.Content, part.Content, path, mode)
		if err != nil {
			return domain.FileProcessOutput{File: in.File}, err
		}
		doc := in.File
		doc.Content = body
		return domain.FileProcessOutput{File: doc}, nil
	case "json":
		body, err := injectJSON(in.File.Content, part.Content, path, mode)
		if err != nil {
			return domain.FileProcessOutput{File: in.File}, err
		}
		doc := in.File
		doc.Content = body
		return domain.FileProcessOutput{File: doc}, nil
	default:
		return domain.FileProcessOutput{File: in.File}, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   fmt.Sprintf("inject_nodes does not support file kind %q", in.File.Kind),
			Processor: p.Name(),
		}
	}
}

// fileFormat decides whether to use YAML or JSON tree manipulation based on
// the file kind. Unknown kinds fall back to the part kind so a JSON nodes
// part can be inserted into an inline base file with a plain text kind.
func fileFormat(fileKind, partKind string) string {
	switch strings.ToLower(fileKind) {
	case "yaml", "mihomo", "uri-list":
		return "yaml"
	case "json", "sing-box":
		return "json"
	}
	switch strings.ToLower(partKind) {
	case "yaml":
		return "yaml"
	case "json":
		return "json"
	}
	return ""
}

func selectNodePart(parts []domain.FilePart, name string) (domain.FilePart, bool) {
	if name != "" {
		for _, part := range parts {
			if part.Name == name {
				return part, true
			}
		}
		return domain.FilePart{}, false
	}
	for _, part := range parts {
		if part.Role == "nodes" {
			return part, true
		}
	}
	return domain.FilePart{}, false
}

// normalisePath returns the JSON Pointer form of path. A single segment is
// promoted to "/segment" so callers can write either "/proxies" or "proxies".
func normalisePath(path string, kind string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		switch strings.ToLower(kind) {
		case "mihomo":
			return "/proxies"
		case "sing-box":
			return "/outbounds"
		}
		return ""
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return "/" + trimmed
}

// injectYAML loads the document root, walks the JSON Pointer path, and
// replaces or appends the target sequence with the rendered nodes payload.
func injectYAML(baseContent, partContent []byte, path, mode string) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(baseContent, &root); err != nil {
		return nil, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "parse base yaml",
			Processor: "inject_nodes",
			Cause:     err,
		}
	}
	listNode, err := loadYAMLList(partContent, path)
	if err != nil {
		return nil, err
	}
	if err := injectYAMLAtPath(&root, path, listNode, mode); err != nil {
		return nil, err
	}
	out, err := yaml.Marshal(&root)
	if err != nil {
		return nil, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "encode yaml",
			Processor: "inject_nodes",
			Cause:     err,
		}
	}
	return bytes.TrimRight(out, "\n"), nil
}

func loadYAMLList(partContent []byte, path string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(partContent, &doc); err != nil {
		return nil, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "parse rendered nodes yaml",
			Processor: "inject_nodes",
			Cause:     err,
		}
	}
	if len(doc.Content) == 0 {
		return nil, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "rendered nodes part is empty",
			Processor: "inject_nodes",
		}
	}
	root := doc.Content[0]
	if root.Kind == yaml.SequenceNode {
		return root, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "rendered nodes part must be a list or mapping",
			Processor: "inject_nodes",
		}
	}
	key := lastSegment(path)
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			value := root.Content[i+1]
			if value.Kind != yaml.SequenceNode {
				return nil, &domain.AppError{
					Code:      domain.CodeFileProcessorFailed,
					Message:   fmt.Sprintf("rendered nodes %s is not a list", key),
					Processor: "inject_nodes",
				}
			}
			return value, nil
		}
	}
	return nil, &domain.AppError{
		Code:      domain.CodeFileProcessorFailed,
		Message:   fmt.Sprintf("rendered nodes part missing %q", key),
		Processor: "inject_nodes",
		Path:      path,
	}
}

func injectYAMLAtPath(root *yaml.Node, path string, value *yaml.Node, mode string) error {
	if root == nil || len(root.Content) == 0 {
		return &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "empty yaml document",
			Processor: "inject_nodes",
		}
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "yaml root must be a mapping",
			Processor: "inject_nodes",
		}
	}
	segments := pointerSegments(path)
	if len(segments) == 0 {
		return &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "inject path must have at least one segment",
			Processor: "inject_nodes",
		}
	}
	parent := doc
	for i, seg := range segments {
		if parent.Kind != yaml.MappingNode {
			return &domain.AppError{
				Code:      domain.CodeFileProcessorFailed,
				Message:   fmt.Sprintf("path %q traverses non-mapping node", path),
				Processor: "inject_nodes",
				Path:      path,
			}
		}
		idx := mappingIndex(parent, seg)
		if idx < 0 {
			return &domain.AppError{
				Code:      domain.CodeFileProcessorFailed,
				Message:   fmt.Sprintf("path %q not found at segment %q", path, seg),
				Processor: "inject_nodes",
				Path:      path,
			}
		}
		if i == len(segments)-1 {
			existing := parent.Content[idx+1]
			if mode == "append" {
				if existing.Kind != yaml.SequenceNode {
					return &domain.AppError{
						Code:      domain.CodeFileProcessorFailed,
						Message:   fmt.Sprintf("path %q is not a sequence", path),
						Processor: "inject_nodes",
						Path:      path,
					}
				}
				existing.Content = append(existing.Content, cloneYAMLNode(value).Content...)
				return nil
			}
			parent.Content[idx+1] = cloneYAMLNode(value)
			return nil
		}
		parent = parent.Content[idx+1]
	}
	return nil
}

func mappingIndex(mapping *yaml.Node, key string) int {
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := *node
	clone.Content = make([]*yaml.Node, 0, len(node.Content))
	for _, child := range node.Content {
		clone.Content = append(clone.Content, cloneYAMLNode(child))
	}
	return &clone
}

func pointerSegments(path string) []string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ReplaceAll(p, "~1", "/")
		p = strings.ReplaceAll(p, "~0", "~")
		out = append(out, p)
	}
	return out
}

func lastSegment(path string) string {
	segs := pointerSegments(path)
	if len(segs) == 0 {
		return ""
	}
	return segs[len(segs)-1]
}

// injectJSON loads a JSON document, extracts the rendered nodes list, and
// replaces/appends the target array at path.
func injectJSON(baseContent, partContent []byte, path, mode string) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(baseContent, &doc); err != nil {
		return nil, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "parse base json",
			Processor: "inject_nodes",
			Cause:     err,
		}
	}
	list, err := loadJSONList(partContent, path)
	if err != nil {
		return nil, err
	}
	if err := injectJSONAtPath(doc, path, list, mode); err != nil {
		return nil, err
	}
	return marshalStableJSON(doc)
}

func loadJSONList(partContent []byte, path string) ([]any, error) {
	var generic any
	if err := json.Unmarshal(partContent, &generic); err != nil {
		return nil, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "parse rendered nodes json",
			Processor: "inject_nodes",
			Cause:     err,
		}
	}
	switch typed := generic.(type) {
	case []any:
		return typed, nil
	case map[string]any:
		key := lastSegment(path)
		val, ok := typed[key]
		if !ok {
			return nil, &domain.AppError{
				Code:      domain.CodeFileProcessorFailed,
				Message:   fmt.Sprintf("rendered nodes part missing %q", key),
				Processor: "inject_nodes",
				Path:      path,
			}
		}
		list, ok := val.([]any)
		if !ok {
			return nil, &domain.AppError{
				Code:      domain.CodeFileProcessorFailed,
				Message:   fmt.Sprintf("rendered nodes %s is not a list", key),
				Processor: "inject_nodes",
			}
		}
		return list, nil
	default:
		return nil, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "rendered nodes part must be a list or object",
			Processor: "inject_nodes",
		}
	}
}

func injectJSONAtPath(doc map[string]any, path string, list []any, mode string) error {
	segments := pointerSegments(path)
	if len(segments) == 0 {
		return &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "inject path must have at least one segment",
			Processor: "inject_nodes",
		}
	}
	var current any = doc
	for i, seg := range segments {
		obj, ok := current.(map[string]any)
		if !ok {
			return &domain.AppError{
				Code:      domain.CodeFileProcessorFailed,
				Message:   fmt.Sprintf("path %q traverses non-object node", path),
				Processor: "inject_nodes",
				Path:      path,
			}
		}
		if _, present := obj[seg]; !present {
			return &domain.AppError{
				Code:      domain.CodeFileProcessorFailed,
				Message:   fmt.Sprintf("path %q not found at segment %q", path, seg),
				Processor: "inject_nodes",
				Path:      path,
			}
		}
		if i == len(segments)-1 {
			if mode == "append" {
				existing, ok := obj[seg].([]any)
				if !ok {
					return &domain.AppError{
						Code:      domain.CodeFileProcessorFailed,
						Message:   fmt.Sprintf("path %q is not a list", path),
						Processor: "inject_nodes",
						Path:      path,
					}
				}
				obj[seg] = append(existing, list...)
				return nil
			}
			obj[seg] = list
			return nil
		}
		current = obj[seg]
	}
	return nil
}
