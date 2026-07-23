package agentcatalog_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/agentcatalog"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
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

	var mihomo service.FileKindCapability
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
