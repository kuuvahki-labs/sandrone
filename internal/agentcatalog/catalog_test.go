package agentcatalog_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/agentcatalog"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/filekind"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestCatalogBuildsServerOwnedDocuments(t *testing.T) {
	svc := service.New()

	summary := agentcatalog.ProcessorSummary(svc.Registry().PublicDescriptors())
	require.NotEmpty(t, summary.Processors)
	require.Equal(t,
		"sandrone://schemas/processors/nodes/rename",
		agentcatalog.ProcessorSchemaURI(domain.StageNodes, "rename"),
	)

	script, err := agentcatalog.ScriptAPI()
	require.NoError(t, err)
	require.Equal(t, 1, script.Version)

	var mihomo filekind.Capability
	for _, capability := range svc.FileKindCapabilities() {
		if capability.Kind == domain.FileKindMihomo {
			mihomo = capability
			break
		}
	}
	fileKind, err := agentcatalog.FileKindDetail(mihomo)
	require.NoError(t, err)
	require.Equal(t, domain.FileKindMihomo, fileKind.Kind)
	require.True(t, fileKind.SettingsSupported)
	require.NotNil(t, fileKind.SettingsSchema)

	require.Equal(t, "object", agentcatalog.SubscriptionSchema().Type)
	require.Contains(t, agentcatalog.SubscriptionSchema().Required, "name")
	require.Equal(t, "object", agentcatalog.FileSpecSchema(true).Type)
	require.Contains(t, agentcatalog.FileSpecSchema(true).Required, "kind")
}

func TestScriptAPICatalogDescribesINIHelpers(t *testing.T) {
	document, err := agentcatalog.ScriptAPI()
	require.NoError(t, err)

	method := func(name string) agentcatalog.ScriptMethodDocument {
		t.Helper()
		for _, candidate := range document.Methods {
			if candidate.Name == name {
				return candidate
			}
		}
		require.FailNow(t, "script API method not found", name)
		return agentcatalog.ScriptMethodDocument{}
	}

	parse := method("api.ini.parse")
	require.Equal(t, []domain.Stage{domain.StageNodes, domain.StageFile}, parse.Stages)
	require.Equal(t, "value_or_void", parse.Returns.Kind)
	require.Equal(t, "object", parse.Returns.Schema.Type)
	require.Contains(t, parse.Returns.Schema.Properties, "sections")

	stringify := method("api.ini.stringify")
	require.Len(t, stringify.Arguments, 1)
	require.False(t, stringify.Arguments[0].Required)
	require.Equal(t, "object", stringify.Arguments[0].Schema.Type)

	override := method("api.ini.override")
	require.Len(t, override.Arguments, 2)
	require.True(t, override.Arguments[0].Required)
	require.True(t, override.Arguments[1].Required)
	require.Equal(t, "string", override.Returns.Schema.Type)
	require.Equal(t, []domain.ErrorCode{domain.CodeScriptRuntime}, override.ErrorCodes)
}
