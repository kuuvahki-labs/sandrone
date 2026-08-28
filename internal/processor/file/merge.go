package file

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/inidoc"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

// MergeParams describes how a set of FileParts should be merged together.
//
// Mode selects the merge strategy:
//
//	append         - concatenate part content; uses Separator (defaults to "\n")
//	replace        - last successful include wins
//	yaml_overlay   - recursive map merge over YAML trees; arrays replace whole
//	yaml_override  - ordered YAML merge with array and replacement operators
//	json_overlay   - recursive map merge over JSON trees; arrays replace whole
//	json_override  - ordered JSON merge with array and replacement operators
//	ini_override   - ordered, lossless INI section overrides
//
// Include is the ordered list of part names to merge. When empty, the merger
// uses every part whose Role is "base" (the implicit MergePolicy semantics).
type MergeParams struct {
	Mode      string   `json:"mode" jsonschema:"File merge strategy" enum:"append,replace,yaml_overlay,yaml_override,json_overlay,json_override,ini_override"`
	Include   []string `json:"include,omitempty" jsonschema:"Internal file part names to include in order"`
	Separator string   `json:"separator,omitempty" jsonschema:"Separator used by append mode" default:"\\n"`
	Content   string   `json:"content,omitempty" jsonschema:"Inline content merged after the current public file content"`
}

// MergePolicy converts a domain.FileMergePolicy into MergeParams. This is
// used by service when applying the implicit MergePolicy step before the
// file chain runs.
func MergePolicy(policy domain.FileMergePolicy) MergeParams {
	return MergeParams{
		Mode:      policy.Mode,
		Include:   policy.Include,
		Separator: policy.Separator,
	}
}

// MergeParts merges parts according to params and returns the combined bytes.
// The kind argument selects which structured merger is used for *_overlay modes.
func MergeParts(parts []domain.FilePart, kind string, params MergeParams) ([]byte, error) {
	selected := selectParts(parts, params.Include)
	if len(selected) == 0 {
		return nil, &domain.AppError{
			Code:    domain.CodeFileMergeFailed,
			Message: "no parts available to merge",
		}
	}
	mode := strings.ToLower(strings.TrimSpace(params.Mode))
	if mode == "" {
		mode = "append"
	}
	_ = kind
	switch mode {
	case "append":
		return mergeAppend(selected, params.Separator), nil
	case "replace":
		return selected[len(selected)-1].Content, nil
	case "yaml_overlay":
		return mergeYAMLOverlay(selected)
	case "yaml_override":
		return mergeYAMLOverride(selected)
	case "json_overlay":
		return mergeJSONOverlay(selected)
	case "json_override":
		return mergeJSONOverride(selected)
	case "ini_override":
		return mergeINIOverride(selected)
	default:
		return nil, &domain.AppError{
			Code:    domain.CodeFileMergeFailed,
			Message: fmt.Sprintf("unsupported merge mode %q", mode),
		}
	}
}

func mergeINIOverride(parts []domain.FilePart) ([]byte, error) {
	doc, err := inidoc.Parse(parts[0].Content)
	if err != nil {
		return nil, iniOverridePartError(parts[0].Name, "parse ini base", err)
	}
	combined := doc.Bytes()
	for _, part := range parts[1:] {
		combined, err = inidoc.Override(combined, part.Content)
		if err != nil {
			return nil, iniOverridePartError(part.Name, "apply ini override", err)
		}
	}
	return combined, nil
}

func iniOverridePartError(part, message string, cause error) error {
	path := ""
	if iniErr, ok := errors.AsType[*inidoc.Error](cause); ok && iniErr.Section != "" {
		path = "/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(iniErr.Section)
	}
	return &domain.AppError{
		Code:    domain.CodeFileMergeFailed,
		Message: fmt.Sprintf("%s part %q", message, part),
		Part:    part,
		Path:    path,
		Cause:   cause,
	}
}

func selectParts(parts []domain.FilePart, include []string) []domain.FilePart {
	if len(include) == 0 {
		out := make([]domain.FilePart, 0, len(parts))
		for _, p := range parts {
			if p.Role == "base" {
				out = append(out, p)
			}
		}
		return out
	}
	byName := map[string]domain.FilePart{}
	for _, p := range parts {
		byName[p.Name] = p
	}
	out := make([]domain.FilePart, 0, len(include))
	for _, name := range include {
		if p, ok := byName[name]; ok {
			out = append(out, p)
		}
	}
	return out
}

func mergeAppend(parts []domain.FilePart, separator string) []byte {
	if separator == "" {
		separator = "\n"
	}
	buf := bytes.Buffer{}
	for i, p := range parts {
		if i > 0 {
			buf.WriteString(separator)
		}
		buf.Write(p.Content)
	}
	return buf.Bytes()
}

func mergeYAMLOverlay(parts []domain.FilePart) ([]byte, error) {
	var combined any
	for _, p := range parts {
		var doc any
		if err := yaml.Unmarshal(p.Content, &doc); err != nil {
			return nil, &domain.AppError{
				Code:    domain.CodeFileMergeFailed,
				Message: fmt.Sprintf("parse yaml part %q", p.Name),
				Part:    p.Name,
				Cause:   err,
			}
		}
		combined = overlayValue(combined, doc)
	}
	out, err := yaml.Marshal(combined)
	if err != nil {
		return nil, &domain.AppError{
			Code:    domain.CodeFileMergeFailed,
			Message: "encode yaml overlay",
			Cause:   err,
		}
	}
	return bytes.TrimRight(out, "\n"), nil
}

func mergeYAMLOverride(parts []domain.FilePart) ([]byte, error) {
	var combined *yaml.Node
	for _, p := range parts {
		var doc yaml.Node
		if err := yaml.Unmarshal(p.Content, &doc); err != nil {
			return nil, &domain.AppError{
				Code:    domain.CodeFileMergeFailed,
				Message: fmt.Sprintf("parse yaml part %q", p.Name),
				Part:    p.Name,
				Cause:   err,
			}
		}
		if len(doc.Content) == 0 {
			continue
		}
		if combined == nil {
			combined = cloneYAMLNode(&doc)
			continue
		}
		if err := applyYAMLOverride(combined.Content[0], doc.Content[0], p.Name, ""); err != nil {
			return nil, err
		}
	}
	if combined == nil {
		combined = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	out, err := yaml.Marshal(combined)
	if err != nil {
		return nil, &domain.AppError{Code: domain.CodeFileMergeFailed, Message: "encode yaml override", Cause: err}
	}
	return bytes.TrimRight(out, "\n"), nil
}

type yamlOverrideOperation int

const (
	yamlOverrideMerge yamlOverrideOperation = iota
	yamlOverridePrepend
	yamlOverrideAppend
	yamlOverrideReplace
)

func applyYAMLOverride(current, override *yaml.Node, part, path string) error {
	if current.Kind != yaml.MappingNode || override.Kind != yaml.MappingNode {
		*current = *cloneYAMLNode(override)
		return nil
	}
	for i := 0; i+1 < len(override.Content); i += 2 {
		rawKey := override.Content[i].Value
		key, operation := parseYAMLOverrideKey(rawKey)
		value := override.Content[i+1]
		valuePath := yamlOverridePath(path, key)
		index := yamlMappingValueIndex(current, key)
		if isYAMLNull(value) {
			if index >= 0 {
				current.Content = append(current.Content[:index-1], current.Content[index+1:]...)
			}
			continue
		}
		switch operation {
		case yamlOverrideReplace:
			setYAMLMappingValue(current, key, index, value)
		case yamlOverridePrepend, yamlOverrideAppend:
			if value.Kind != yaml.SequenceNode {
				return yamlOverrideTypeError(part, valuePath, "array override value must be an array")
			}
			if index < 0 {
				setYAMLMappingValue(current, key, index, value)
				continue
			}
			existing := current.Content[index]
			if existing.Kind != yaml.SequenceNode {
				return yamlOverrideTypeError(part, valuePath, "array override target must be an array")
			}
			items := cloneYAMLNodes(value.Content)
			if operation == yamlOverridePrepend {
				existing.Content = append(items, existing.Content...)
			} else {
				existing.Content = append(existing.Content, items...)
			}
		default:
			if value.Kind == yaml.MappingNode {
				if index < 0 || current.Content[index].Kind != yaml.MappingNode {
					setYAMLMappingValue(current, key, index, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"})
					index = yamlMappingValueIndex(current, key)
				}
				if err := applyYAMLOverride(current.Content[index], value, part, valuePath); err != nil {
					return err
				}
				continue
			}
			setYAMLMappingValue(current, key, index, value)
		}
	}
	return nil
}

func parseYAMLOverrideKey(key string) (string, yamlOverrideOperation) {
	if len(key) >= 2 && strings.HasPrefix(key, "<") && strings.HasSuffix(key, ">") {
		return key[1 : len(key)-1], yamlOverrideMerge
	}
	if strings.HasPrefix(key, "+") && len(key) > 1 {
		return key[1:], yamlOverridePrepend
	}
	if strings.HasSuffix(key, "!") && len(key) > 1 {
		return key[:len(key)-1], yamlOverrideReplace
	}
	if strings.HasSuffix(key, "+") && len(key) > 1 {
		return key[:len(key)-1], yamlOverrideAppend
	}
	return key, yamlOverrideMerge
}

func yamlMappingValueIndex(mapping *yaml.Node, key string) int {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i + 1
		}
	}
	return -1
}

func setYAMLMappingValue(mapping *yaml.Node, key string, valueIndex int, value *yaml.Node) {
	cloned := cloneYAMLNode(value)
	if valueIndex >= 0 {
		mapping.Content[valueIndex] = cloned
		return
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		cloned,
	)
}

func cloneYAMLNodes(nodes []*yaml.Node) []*yaml.Node {
	out := make([]*yaml.Node, len(nodes))
	for i, node := range nodes {
		out[i] = cloneYAMLNode(node)
	}
	return out
}

func isYAMLNull(node *yaml.Node) bool {
	return node.Tag == "!!null"
}

func yamlOverridePath(parent, key string) string {
	escaped := strings.NewReplacer("~", "~0", "/", "~1").Replace(key)
	return parent + "/" + escaped
}

func yamlOverrideTypeError(part, path, message string) error {
	return &domain.AppError{
		Code:    domain.CodeFileMergeFailed,
		Message: message,
		Part:    part,
		Path:    path,
	}
}

func mergeJSONOverride(parts []domain.FilePart) ([]byte, error) {
	var combined *yaml.Node
	for _, p := range parts {
		if !json.Valid(p.Content) {
			var invalid any
			err := json.Unmarshal(p.Content, &invalid)
			return nil, &domain.AppError{
				Code:    domain.CodeFileMergeFailed,
				Message: fmt.Sprintf("parse json part %q", p.Name),
				Part:    p.Name,
				Cause:   err,
			}
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(p.Content, &doc); err != nil {
			return nil, &domain.AppError{
				Code:    domain.CodeFileMergeFailed,
				Message: fmt.Sprintf("parse json part %q", p.Name),
				Part:    p.Name,
				Cause:   err,
			}
		}
		if combined == nil {
			combined = cloneYAMLNode(&doc)
			continue
		}
		if err := applyYAMLOverride(combined.Content[0], doc.Content[0], p.Name, ""); err != nil {
			return nil, err
		}
	}

	var value any
	if err := combined.Decode(&value); err != nil {
		return nil, &domain.AppError{Code: domain.CodeFileMergeFailed, Message: "decode json override", Cause: err}
	}
	body, err := marshalStableJSON(normalizeMap(value))
	if err != nil {
		return nil, &domain.AppError{Code: domain.CodeFileMergeFailed, Message: "encode json override", Cause: err}
	}
	return body, nil
}

func mergeJSONOverlay(parts []domain.FilePart) ([]byte, error) {
	var combined any
	for _, p := range parts {
		var doc any
		if err := yaml.Unmarshal(p.Content, &doc); err != nil {
			return nil, &domain.AppError{
				Code:    domain.CodeFileMergeFailed,
				Message: fmt.Sprintf("parse json part %q", p.Name),
				Part:    p.Name,
				Cause:   err,
			}
		}
		combined = overlayValue(combined, doc)
	}
	body, err := marshalStableJSON(combined)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func overlayValue(prev, next any) any {
	if next == nil {
		return prev
	}
	prevMap, prevOk := prev.(map[string]any)
	nextMap, nextOk := normalizeMap(next).(map[string]any)
	if prevOk && nextOk {
		out := map[string]any{}
		for k, v := range prevMap {
			out[k] = v
		}
		for k, v := range nextMap {
			if v == nil {
				delete(out, k)
				continue
			}
			if existing, present := out[k]; present {
				out[k] = overlayValue(existing, v)
				continue
			}
			out[k] = normalizeMap(v)
		}
		return out
	}
	return normalizeMap(next)
}

// normalizeMap converts yaml.v3's map[interface{}]interface{} into
// map[string]any so JSON encoding and overlay merges work uniformly.
func normalizeMap(v any) any {
	switch m := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = normalizeMap(val)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[k] = normalizeMap(val)
		}
		return out
	case []any:
		out := make([]any, len(m))
		for i, val := range m {
			out[i] = normalizeMap(val)
		}
		return out
	default:
		return v
	}
}

// MergeProcessor implements the file-stage `merge` processor. It re-runs the
// merge algorithm against the parts already on FileProcessInput. The result
// replaces FileDocument.Content.
type MergeProcessor struct {
	params MergeParams
}

func NewMergeProcessor(params MergeParams) *MergeProcessor {
	return &MergeProcessor{params: params}
}

func buildMerge(spec domain.ProcessorSpec) (domain.FileProcessor, error) {
	var params MergeParams
	if err := processor.UnmarshalParams(spec, &params); err != nil {
		return nil, err
	}
	if params.Mode == "" {
		params.Mode = "append"
	}
	return NewMergeProcessor(params), nil
}

func (p *MergeProcessor) Name() string { return "merge" }

func (p *MergeProcessor) ApplyFile(_ context.Context, in domain.FileProcessInput) (domain.FileProcessOutput, error) {
	parts := in.Parts
	params := p.params
	if params.Content != "" {
		parts = []domain.FilePart{
			{Name: "current", Role: "base", Kind: in.File.Kind, Content: in.File.Content},
			{Name: "overlay", Role: "base", Kind: in.File.Kind, Content: []byte(params.Content)},
		}
		params.Include = nil
	}
	body, err := MergeParts(parts, in.File.Kind, params)
	if err != nil {
		return domain.FileProcessOutput{File: in.File}, err
	}
	doc := in.File
	doc.Content = body
	return domain.FileProcessOutput{File: doc}, nil
}
