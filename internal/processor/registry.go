// Package processor provides the processor registry and chain runners for the
// nodes and file stages.
package processor

import (
	"fmt"
	"sort"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

// Registry holds processor constructors keyed by (type, stage).
//
// A single processor type can register for multiple stages (e.g. the script
// processor registers for nodes and file). When ProcessorSpec.Stage
// is empty, the registry can infer it if and only if a single stage is
// registered for the type.
type Registry struct {
	nodeBuilders map[string]NodeBuilder
	fileBuilders map[string]FileBuilder
}

type NodeBuilder func(spec domain.ProcessorSpec) (domain.NodeProcessor, error)
type FileBuilder func(spec domain.ProcessorSpec) (domain.FileProcessor, error)

func NewRegistry() *Registry {
	return &Registry{
		nodeBuilders: map[string]NodeBuilder{},
		fileBuilders: map[string]FileBuilder{},
	}
}

func (r *Registry) RegisterNode(typeName string, build NodeBuilder) {
	r.nodeBuilders[typeName] = build
}

func (r *Registry) RegisterFile(typeName string, build FileBuilder) {
	r.fileBuilders[typeName] = build
}

func (r *Registry) HasNode(typeName string) bool { _, ok := r.nodeBuilders[typeName]; return ok }
func (r *Registry) HasFile(typeName string) bool { _, ok := r.fileBuilders[typeName]; return ok }

func (r *Registry) NodeTypes() []string {
	return sortedKeys(r.nodeBuilders)
}

func (r *Registry) FileTypes() []string {
	return sortedKeys(r.fileBuilders)
}

// ResolveStage infers the stage for spec when spec.Stage is empty.
// Returns processor_config_invalid when inference is ambiguous, and
// processor_unknown when the type is not registered for any stage.
func (r *Registry) ResolveStage(spec domain.ProcessorSpec) (domain.Stage, error) {
	if spec.Stage != "" {
		return spec.Stage, nil
	}
	stages := []domain.Stage{}
	if r.HasNode(spec.Type) {
		stages = append(stages, domain.StageNodes)
	}
	if r.HasFile(spec.Type) {
		stages = append(stages, domain.StageFile)
	}
	switch len(stages) {
	case 0:
		return "", &domain.AppError{
			Code:      domain.CodeProcessorUnknown,
			Message:   fmt.Sprintf("processor type %q not registered for any stage", spec.Type),
			Processor: spec.Type,
		}
	case 1:
		return stages[0], nil
	default:
		return "", &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   fmt.Sprintf("processor type %q registered for multiple stages, stage must be set explicitly", spec.Type),
			Processor: spec.Type,
		}
	}
}

// BuildNode constructs a NodeProcessor for spec.
func (r *Registry) BuildNode(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
	b, ok := r.nodeBuilders[spec.Type]
	if !ok {
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorUnknown,
			Message:   fmt.Sprintf("node processor %q not registered", spec.Type),
			Processor: spec.Type,
		}
	}
	return b(spec)
}

// BuildFile constructs a FileProcessor for spec.
func (r *Registry) BuildFile(spec domain.ProcessorSpec) (domain.FileProcessor, error) {
	b, ok := r.fileBuilders[spec.Type]
	if !ok {
		return nil, &domain.AppError{
			Code:      domain.CodeProcessorUnknown,
			Message:   fmt.Sprintf("file processor %q not registered", spec.Type),
			Processor: spec.Type,
		}
	}
	return b(spec)
}

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
