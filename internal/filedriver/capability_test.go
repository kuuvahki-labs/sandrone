package filedriver

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestCapabilities(t *testing.T) {
	registry := New()
	capabilities := registry.Capabilities()
	kinds := make([]domain.FileKind, len(capabilities))
	for i, capability := range capabilities {
		kinds[i] = capability.Kind
		require.Equal(t, []string{"inline", "remote"}, capability.SourceRules.AllowedTypes, "%s", capability.Kind)
		require.NotEmpty(t, capability.Examples, "%s", capability.Kind)
	}
	require.Equal(t, []domain.FileKind{
		domain.FileKindStatic,
		domain.FileKindMihomo,
		domain.FileKindSingBox,
		domain.FileKindShadowrocket,
	}, kinds)

	for _, capability := range capabilities[1:] {
		require.NotNil(t, capability.SettingsPrototype, "%s", capability.Kind)
		require.NotEmpty(t, capability.MediaType, "%s", capability.Kind)
		require.NotEmpty(t, capability.Syntax, "%s", capability.Kind)
		require.NotEmpty(t, capability.DefaultExtension, "%s", capability.Kind)
		require.NotEmpty(t, capability.Defaults, "%s", capability.Kind)
		schema, err := jsonschema.ForType(reflect.TypeOf(capability.SettingsPrototype), nil)
		require.NoError(t, err, "%s settings prototype must support schema reflection", capability.Kind)
		require.ElementsMatch(t, []string{"groups", "rule_sets", "rules"}, schema.Required,
			"%s settings schema must require complete materialization", capability.Kind)

		raw, err := json.Marshal(capability.SettingsPrototype)
		require.NoError(t, err)
		driver, err := registry.Lookup(capability.Kind)
		require.NoError(t, err)
		require.NoError(t, driver.ValidateSettings(raw), "%s settings prototype must be accepted by its driver", capability.Kind)
		for index, example := range capability.Examples {
			body, err := json.Marshal(example)
			require.NoError(t, err)
			var spec domain.FileSpec
			require.NoError(t, json.Unmarshal(body, &spec), "%s example %d", capability.Kind, index)
			require.Equal(t, capability.Kind, spec.Kind)
			require.NotNil(t, spec.Config, "%s example %d", capability.Kind, index)
			settings := bytes.TrimSpace(spec.Config.Settings)
			require.NotEmpty(t, settings, "%s example %d settings", capability.Kind, index)
			require.NotEqual(t, "{}", string(settings), "%s example %d settings", capability.Kind, index)
			if capability.Kind == domain.FileKindMihomo {
				require.NotContains(t, string(settings), "adaptive_groups", "published Mihomo example %d", index)
				var fields map[string]jsontext.Value
				require.NoError(t, json.Unmarshal(settings, &fields))
				for _, name := range []string{"groups", "rule_sets", "rules"} {
					var items []any
					require.NoError(t, json.Unmarshal(fields[name], &items), "published Mihomo example %d %s", index, name)
					require.NotEmpty(t, items, "published Mihomo example %d %s", index, name)
				}
			}
			require.NoError(t, driver.ValidateSettings(spec.Config.Settings), "%s example %d settings", capability.Kind, index)
		}
	}
}

func TestMihomoCapabilityPrototypeMatchesExecutedSettings(t *testing.T) {
	capabilities := New().Capabilities()
	prototypeType := reflect.TypeOf(capabilities[1].SettingsPrototype)
	_, exists := prototypeType.FieldByName("AdaptiveGroups")
	require.False(t, exists)
	schema, err := jsonschema.ForType(prototypeType, nil)
	require.NoError(t, err)
	require.NotContains(t, schema.Properties, "adaptive_groups")
	require.Contains(t, schema.Properties, "groups")
	require.Contains(t, schema.Properties, "rule_sets")
	require.Contains(t, schema.Properties, "rules")
}

func TestFileDriversRejectRemovedAdaptiveGroups(t *testing.T) {
	for _, kind := range []domain.FileKind{domain.FileKindMihomo, domain.FileKindShadowrocket} {
		t.Run(string(kind), func(t *testing.T) {
			driver, err := New().Lookup(kind)
			require.NoError(t, err)
			settings := jsontext.Value(`{"adaptive_groups":{"type":"url-test","regions":["hk"]},"groups":[],"rule_sets":[],"rules":[]}`)
			err = driver.ValidateSettings(settings)
			require.True(t, domain.IsCode(err, domain.CodeInvalidArgument))
			require.ErrorContains(t, err, "config.settings.adaptive_groups")
		})
	}
}

func TestCapabilitiesReturnImmutableCopies(t *testing.T) {
	registry := New()
	first := registry.Capabilities()
	first[0].SourceRules.AllowedTypes[0] = "mutated"
	first[1].Defaults["source"] = "mutated"
	first[1].Examples[0]["kind"] = "mutated"

	second := registry.Capabilities()
	require.Equal(t, "inline", second[0].SourceRules.AllowedTypes[0])
	require.NotEqual(t, "mutated", second[1].Defaults["source"])
	require.Equal(t, string(domain.FileKindMihomo), second[1].Examples[0]["kind"])
}
