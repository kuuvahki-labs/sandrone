package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

// PatchOp is a single RFC 6902 patch operation. The From field is only
// meaningful for "move" and "copy" operations.
type PatchOp struct {
	Op    string          `json:"op" jsonschema:"Patch operation" enum:"add,replace,remove,test,move,copy"`
	Path  string          `json:"path" jsonschema:"JSON Pointer target path"`
	From  string          `json:"from,omitempty" jsonschema:"JSON Pointer source path for move and copy"`
	Value json.RawMessage `json:"value,omitempty" jsonschema:"JSON value used by add replace and test"`
}

// PatchParams holds the patch operations applied by the yaml_patch /
// json_patch processors.
type PatchParams struct {
	Ops []PatchOp `json:"ops" jsonschema:"Ordered non-empty patch operations" minItems:"1"`
}

func buildYAMLPatch(spec domain.ProcessorSpec) (domain.FileProcessor, error) {
	var params PatchParams
	if err := processor.UnmarshalParams(spec, &params); err != nil {
		return nil, err
	}
	if len(params.Ops) == 0 {
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "yaml_patch requires ops",
			Processor: spec.Type,
		}
	}
	return &yamlPatchProc{ops: params.Ops}, nil
}

func buildJSONPatch(spec domain.ProcessorSpec) (domain.FileProcessor, error) {
	var params PatchParams
	if err := processor.UnmarshalParams(spec, &params); err != nil {
		return nil, err
	}
	if len(params.Ops) == 0 {
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "json_patch requires ops",
			Processor: spec.Type,
		}
	}
	return &jsonPatchProc{ops: params.Ops}, nil
}

type yamlPatchProc struct {
	ops []PatchOp
}

func (p *yamlPatchProc) Name() string { return "yaml_patch" }

func (p *yamlPatchProc) ApplyFile(_ context.Context, in domain.FileProcessInput) (domain.FileProcessOutput, error) {
	var root any
	if err := yaml.Unmarshal(in.File.Content, &root); err != nil {
		return domain.FileProcessOutput{File: in.File}, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "parse yaml",
			Processor: p.Name(),
			Cause:     err,
		}
	}
	root = normalizeMap(root)
	updated, err := applyPatchOps(root, p.ops, p.Name())
	if err != nil {
		return domain.FileProcessOutput{File: in.File}, err
	}
	body, err := yaml.Marshal(updated)
	if err != nil {
		return domain.FileProcessOutput{File: in.File}, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "encode yaml",
			Processor: p.Name(),
			Cause:     err,
		}
	}
	doc := in.File
	doc.Content = trimTrailingNewline(body)
	return domain.FileProcessOutput{File: doc}, nil
}

type jsonPatchProc struct {
	ops []PatchOp
}

func (p *jsonPatchProc) Name() string { return "json_patch" }

func (p *jsonPatchProc) ApplyFile(_ context.Context, in domain.FileProcessInput) (domain.FileProcessOutput, error) {
	var root any
	if err := json.Unmarshal(in.File.Content, &root); err != nil {
		return domain.FileProcessOutput{File: in.File}, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "parse json",
			Processor: p.Name(),
			Cause:     err,
		}
	}
	updated, err := applyPatchOps(root, p.ops, p.Name())
	if err != nil {
		return domain.FileProcessOutput{File: in.File}, err
	}
	body, err := marshalStableJSON(updated)
	if err != nil {
		return domain.FileProcessOutput{File: in.File}, &domain.AppError{
			Code:      domain.CodeFileProcessorFailed,
			Message:   "encode json",
			Processor: p.Name(),
			Cause:     err,
		}
	}
	doc := in.File
	doc.Content = body
	return domain.FileProcessOutput{File: doc}, nil
}

func trimTrailingNewline(in []byte) []byte {
	for len(in) > 0 && in[len(in)-1] == '\n' {
		in = in[:len(in)-1]
	}
	return in
}

func applyPatchOps(root any, ops []PatchOp, processorName string) (any, error) {
	current := root
	for i, op := range ops {
		updated, err := applyPatchOp(current, op)
		if err != nil {
			if appErr, ok := errors.AsType[*domain.AppError](err); ok {
				appErr.Processor = processorName
				return current, appErr
			}
			return current, &domain.AppError{
				Code:      domain.CodeFileProcessorFailed,
				Message:   fmt.Sprintf("op %d %s failed", i, op.Op),
				Processor: processorName,
				Cause:     err,
			}
		}
		current = updated
	}
	return current, nil
}

func applyPatchOp(root any, op PatchOp) (any, error) {
	path := normalisePointer(op.Path)
	switch op.Op {
	case "add":
		value, err := decodeValue(op.Value)
		if err != nil {
			return root, err
		}
		return setAtPointer(root, path, value, true)
	case "replace":
		value, err := decodeValue(op.Value)
		if err != nil {
			return root, err
		}
		return setAtPointer(root, path, value, false)
	case "remove":
		return removeAtPointer(root, path)
	case "test":
		expected, err := decodeValue(op.Value)
		if err != nil {
			return root, err
		}
		actual, err := getAtPointer(root, path)
		if err != nil {
			return root, err
		}
		if !valueEquals(actual, expected) {
			return root, &domain.AppError{
				Code:    domain.CodeFileProcessorFailed,
				Message: fmt.Sprintf("test failed at %q", op.Path),
				Path:    op.Path,
			}
		}
		return root, nil
	case "move":
		from := normalisePointer(op.From)
		value, err := getAtPointer(root, from)
		if err != nil {
			return root, err
		}
		removed, err := removeAtPointer(root, from)
		if err != nil {
			return root, err
		}
		return setAtPointer(removed, path, value, true)
	case "copy":
		from := normalisePointer(op.From)
		value, err := getAtPointer(root, from)
		if err != nil {
			return root, err
		}
		return setAtPointer(root, path, deepCopy(value), true)
	default:
		return root, &domain.AppError{
			Code:    domain.CodeProcessorConfigInvalid,
			Message: fmt.Sprintf("unsupported patch op %q", op.Op),
		}
	}
}

func decodeValue(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil //nolint:nilnil // Empty patch value intentionally decodes to a nil JSON value without an error.
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, &domain.AppError{
			Code:    domain.CodeProcessorConfigInvalid,
			Message: "decode patch value",
			Cause:   err,
		}
	}
	return v, nil
}

func normalisePointer(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
}

func pointerTokens(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	out := make([]string, len(parts))
	for i, p := range parts {
		p = strings.ReplaceAll(p, "~1", "/")
		p = strings.ReplaceAll(p, "~0", "~")
		out[i] = p
	}
	return out
}

func getAtPointer(root any, path string) (any, error) {
	tokens := pointerTokens(path)
	if len(tokens) == 0 {
		return root, nil
	}
	current := root
	for _, token := range tokens {
		switch typed := current.(type) {
		case map[string]any:
			val, ok := typed[token]
			if !ok {
				return nil, &domain.AppError{
					Code:    domain.CodeFileProcessorFailed,
					Message: fmt.Sprintf("path token %q not found", token),
					Path:    path,
				}
			}
			current = val
		case []any:
			idx, err := strconv.Atoi(token)
			if err != nil || idx < 0 || idx >= len(typed) {
				return nil, &domain.AppError{
					Code:    domain.CodeFileProcessorFailed,
					Message: fmt.Sprintf("path index %q invalid", token),
					Path:    path,
				}
			}
			current = typed[idx]
		default:
			return nil, &domain.AppError{
				Code:    domain.CodeFileProcessorFailed,
				Message: fmt.Sprintf("path token %q traverses scalar", token),
				Path:    path,
			}
		}
	}
	return current, nil
}

func setAtPointer(root any, path string, value any, create bool) (any, error) {
	tokens := pointerTokens(path)
	if len(tokens) == 0 {
		return value, nil
	}
	return setAtTokens(root, tokens, value, create, path)
}

func setAtTokens(root any, tokens []string, value any, create bool, fullPath string) (any, error) {
	if len(tokens) == 0 {
		return value, nil
	}
	token := tokens[0]
	rest := tokens[1:]
	switch typed := root.(type) {
	case map[string]any:
		if len(rest) == 0 {
			if _, exists := typed[token]; !exists && !create {
				return root, &domain.AppError{
					Code:    domain.CodeFileProcessorFailed,
					Message: fmt.Sprintf("replace target missing at %q", fullPath),
					Path:    fullPath,
				}
			}
			out := cloneMap(typed)
			out[token] = value
			return out, nil
		}
		child, ok := typed[token]
		if !ok {
			if !create {
				return root, &domain.AppError{
					Code:    domain.CodeFileProcessorFailed,
					Message: fmt.Sprintf("path token %q not found", token),
					Path:    fullPath,
				}
			}
			child = map[string]any{}
		}
		updated, err := setAtTokens(child, rest, value, create, fullPath)
		if err != nil {
			return root, err
		}
		out := cloneMap(typed)
		out[token] = updated
		return out, nil
	case []any:
		idx, err := indexFor(token, len(typed), create && len(rest) == 0)
		if err != nil {
			err.Path = fullPath
			return root, err
		}
		if len(rest) == 0 {
			out := append([]any{}, typed...)
			if idx == len(typed) || token == "-" {
				out = append(out, value)
				return out, nil
			}
			out[idx] = value
			return out, nil
		}
		if idx < 0 || idx >= len(typed) {
			return root, &domain.AppError{
				Code:    domain.CodeFileProcessorFailed,
				Message: fmt.Sprintf("path index %q out of range", token),
				Path:    fullPath,
			}
		}
		updated, err2 := setAtTokens(typed[idx], rest, value, create, fullPath)
		if err2 != nil {
			return root, err2
		}
		out := append([]any{}, typed...)
		out[idx] = updated
		return out, nil
	default:
		return root, &domain.AppError{
			Code:    domain.CodeFileProcessorFailed,
			Message: fmt.Sprintf("path token %q traverses scalar", token),
			Path:    fullPath,
		}
	}
}

func indexFor(token string, length int, allowAppend bool) (int, *domain.AppError) {
	if token == "-" {
		if !allowAppend {
			return 0, &domain.AppError{
				Code:    domain.CodeFileProcessorFailed,
				Message: "append index '-' only allowed for terminal add",
			}
		}
		return length, nil
	}
	idx, err := strconv.Atoi(token)
	if err != nil {
		return 0, &domain.AppError{
			Code:    domain.CodeFileProcessorFailed,
			Message: fmt.Sprintf("path index %q invalid", token),
		}
	}
	if idx < 0 {
		return 0, &domain.AppError{
			Code:    domain.CodeFileProcessorFailed,
			Message: fmt.Sprintf("path index %q negative", token),
		}
	}
	if idx > length {
		return 0, &domain.AppError{
			Code:    domain.CodeFileProcessorFailed,
			Message: fmt.Sprintf("path index %q out of range", token),
		}
	}
	return idx, nil
}

func removeAtPointer(root any, path string) (any, error) {
	tokens := pointerTokens(path)
	if len(tokens) == 0 {
		return nil, nil //nolint:nilnil // Removing the document root intentionally produces a nil document without an error.
	}
	return removeAtTokens(root, tokens, path)
}

func removeAtTokens(root any, tokens []string, fullPath string) (any, error) {
	if len(tokens) == 0 {
		return nil, nil //nolint:nilnil // Recursive remove on an empty token list means the selected value is removed.
	}
	token := tokens[0]
	rest := tokens[1:]
	switch typed := root.(type) {
	case map[string]any:
		if len(rest) == 0 {
			if _, exists := typed[token]; !exists {
				return root, &domain.AppError{
					Code:    domain.CodeFileProcessorFailed,
					Message: fmt.Sprintf("remove target missing at %q", fullPath),
					Path:    fullPath,
				}
			}
			out := cloneMap(typed)
			delete(out, token)
			return out, nil
		}
		child, ok := typed[token]
		if !ok {
			return root, &domain.AppError{
				Code:    domain.CodeFileProcessorFailed,
				Message: fmt.Sprintf("remove target missing at %q", fullPath),
				Path:    fullPath,
			}
		}
		updated, err := removeAtTokens(child, rest, fullPath)
		if err != nil {
			return root, err
		}
		out := cloneMap(typed)
		out[token] = updated
		return out, nil
	case []any:
		idx, err := strconv.Atoi(token)
		if err != nil || idx < 0 || idx >= len(typed) {
			return root, &domain.AppError{
				Code:    domain.CodeFileProcessorFailed,
				Message: fmt.Sprintf("path index %q invalid", token),
				Path:    fullPath,
			}
		}
		if len(rest) == 0 {
			out := append([]any{}, typed[:idx]...)
			out = append(out, typed[idx+1:]...)
			return out, nil
		}
		updated, err := removeAtTokens(typed[idx], rest, fullPath)
		if err != nil {
			return root, err
		}
		out := append([]any{}, typed...)
		out[idx] = updated
		return out, nil
	default:
		return root, &domain.AppError{
			Code:    domain.CodeFileProcessorFailed,
			Message: fmt.Sprintf("path token %q traverses scalar", token),
			Path:    fullPath,
		}
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func deepCopy(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			out[k] = deepCopy(v)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, v := range typed {
			out[i] = deepCopy(v)
		}
		return out
	default:
		return value
	}
}

func valueEquals(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
