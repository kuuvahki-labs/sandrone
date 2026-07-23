package processor_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	fileproc "github.com/kuuvahki-labs/sandrone/internal/processor/file"
	nodeproc "github.com/kuuvahki-labs/sandrone/internal/processor/node"
	scriptproc "github.com/kuuvahki-labs/sandrone/internal/processor/script"
)

type descriptorProbeRunner struct{}

func (descriptorProbeRunner) Probe(context.Context, domain.ProbeRequest) (*domain.ProbeResult, error) {
	return &domain.ProbeResult{}, nil
}

func TestRegistryBuiltInDescriptors(t *testing.T) {
	registry := processor.NewRegistry()
	nodeproc.Register(registry)
	fileproc.Register(registry)
	scriptproc.Register(registry)

	descriptors := registry.Descriptors()
	require.Equal(t, []string{
		"file:inject_nodes",
		"file:json_patch",
		"file:merge",
		"file:script",
		"file:template",
		"file:yaml_patch",
		"nodes:dedup",
		"nodes:filter",
		"nodes:probe",
		"nodes:quick_settings",
		"nodes:rename",
		"nodes:script",
		"nodes:sort",
	}, descriptorKeys(descriptors))

	require.True(t, registry.HasFile("inject_nodes"), "inject_nodes must remain executable")
	require.NotContains(t, descriptorKeys(registry.PublicDescriptors()), "file:inject_nodes")
	for _, descriptor := range registry.PublicDescriptors() {
		require.NotNil(t, descriptor.ParamsPrototype, "%s:%s", descriptor.Stage, descriptor.Type)
		require.NotEmpty(t, descriptor.Examples, "%s:%s", descriptor.Stage, descriptor.Type)
		_, err := jsonschema.ForType(reflect.TypeOf(descriptor.ParamsPrototype), nil)
		require.NoError(t, err, "%s:%s prototype must support schema reflection", descriptor.Stage, descriptor.Type)
	}
}

func TestRegistryPublicDescriptorExamplesBuild(t *testing.T) {
	registry := processor.NewRegistry()
	nodeproc.Register(registry, descriptorProbeRunner{})
	fileproc.Register(registry)
	scriptproc.Register(registry)

	for _, descriptor := range registry.PublicDescriptors() {
		for index, example := range descriptor.Examples {
			assertTaggedExample(t, descriptor.ParamsPrototype, example)
			spec := domain.ProcessorSpec{
				Type: descriptor.Type, Stage: descriptor.Stage,
				Params: descriptorRawParams(t, example),
			}
			var err error
			switch descriptor.Stage {
			case domain.StageNodes:
				_, err = registry.BuildNode(spec)
			case domain.StageFile:
				_, err = registry.BuildFile(spec)
			default:
				t.Fatalf("unexpected stage %q", descriptor.Stage)
			}
			require.NoError(t, err, "%s:%s example %d", descriptor.Stage, descriptor.Type, index)
		}
	}
}

func TestProbeDescriptorConstraintTagsArePresent(t *testing.T) {
	probeType := reflect.TypeOf(nodeproc.ProbeParams{})
	layer, ok := probeType.FieldByName("Layer")
	require.True(t, ok)
	require.Equal(t, "protocol,proxy", layer.Tag.Get("enum"))
	timeout, ok := probeType.FieldByName("TimeoutMS")
	require.True(t, ok)
	require.Equal(t, "0", timeout.Tag.Get("minimum"))
}

func TestRegistryDescriptorOrderIsStableByStageThenType(t *testing.T) {
	registry := processor.NewRegistry()
	registry.RegisterNodeWithDescriptor("zeta", nil, processor.Descriptor{Description: "z", ParamsPrototype: struct{}{}, Examples: []map[string]any{{}}})
	registry.RegisterFileWithDescriptor("alpha", nil, processor.Descriptor{Description: "a", ParamsPrototype: struct{}{}, Examples: []map[string]any{{}}})
	registry.RegisterNodeWithDescriptor("alpha", nil, processor.Descriptor{Description: "a", ParamsPrototype: struct{}{}, Examples: []map[string]any{{}}})

	require.Equal(t, []string{"file:alpha", "nodes:alpha", "nodes:zeta"}, descriptorKeys(registry.Descriptors()))
}

func TestLegacyRegistrationDoesNotAdvertiseUndocumentedSchema(t *testing.T) {
	registry := processor.NewRegistry()
	registry.RegisterNodeWithDescriptor("owned", nil, processor.Descriptor{
		Description: "owned", ParamsPrototype: struct{}{}, Examples: []map[string]any{{}}, Public: true,
	})

	registry.RegisterNode("owned", nil)
	registry.RegisterFile("custom", nil)

	require.Empty(t, registry.Descriptors())
}

func descriptorKeys(descriptors []processor.Descriptor) []string {
	keys := make([]string, len(descriptors))
	for i, descriptor := range descriptors {
		keys[i] = string(descriptor.Stage) + ":" + descriptor.Type
	}
	return keys
}

func descriptorRawParams(t *testing.T, example map[string]any) map[string]json.RawMessage {
	t.Helper()
	params := make(map[string]json.RawMessage, len(example))
	for name, value := range example {
		body, err := json.Marshal(value)
		require.NoError(t, err)
		params[name] = body
	}
	return params
}

func assertTaggedExample(t *testing.T, prototype any, example map[string]any) {
	t.Helper()
	assertTaggedObject(t, reflect.TypeOf(prototype), example)
}

func assertTaggedObject(t *testing.T, structType reflect.Type, value map[string]any) {
	t.Helper()
	for index := 0; index < structType.NumField(); index++ {
		field := structType.Field(index)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		actual, ok := value[jsonName]
		if !ok {
			continue
		}
		if enum := field.Tag.Get("enum"); enum != "" {
			require.Contains(t, strings.Split(enum, ","), actual, "%s enum", jsonName)
		}
		if minimum := field.Tag.Get("minimum"); minimum != "" {
			want, err := strconv.ParseFloat(minimum, 64)
			require.NoError(t, err)
			require.GreaterOrEqual(t, numericValue(t, actual), want, "%s minimum", jsonName)
		}
		if maximum := field.Tag.Get("maximum"); maximum != "" {
			want, err := strconv.ParseFloat(maximum, 64)
			require.NoError(t, err)
			require.LessOrEqual(t, numericValue(t, actual), want, "%s maximum", jsonName)
		}
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct {
			if nested, ok := actual.(map[string]any); ok {
				assertTaggedObject(t, fieldType, nested)
			}
		}
		if fieldType.Kind() == reflect.Slice && fieldType.Elem().Kind() == reflect.Struct {
			if items, ok := actual.([]any); ok {
				for _, item := range items {
					if nested, ok := item.(map[string]any); ok {
						assertTaggedObject(t, fieldType.Elem(), nested)
					}
				}
			}
		}
	}
}

func numericValue(t *testing.T, value any) float64 {
	t.Helper()
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case float64:
		return typed
	default:
		t.Fatalf("expected numeric value, got %T", value)
		return 0
	}
}
