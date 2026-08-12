package mcpapi_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

func TestServerDiscoversCanonicalProtocolVersion(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	require.Equal(t, mcpapi.ProtocolVersion, session.InitializeResult().ProtocolVersion)
}

func TestServerAppliesPrivateCachePolicy(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	requirePrivateCache(t, tools.Cacheable, 300_000)

	prompts, err := session.ListPrompts(ctx, nil)
	require.NoError(t, err)
	requirePrivateCache(t, prompts.Cacheable, 300_000)

	resources, err := session.ListResources(ctx, nil)
	require.NoError(t, err)
	requirePrivateCache(t, resources.Cacheable, 300_000)

	templates, err := session.ListResourceTemplates(ctx, nil)
	require.NoError(t, err)
	requirePrivateCache(t, templates.Cacheable, 300_000)

	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "sandrone://schemas/script-api/v1"})
	require.NoError(t, err)
	requirePrivateCache(t, resource.Cacheable, 0)
}

func TestServerAdvertisesOnlyCurrentCapabilities(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	capabilities := session.InitializeResult().Capabilities
	require.NotNil(t, capabilities)
	require.NotNil(t, capabilities.Tools)
	require.False(t, capabilities.Tools.ListChanged)
	require.NotNil(t, capabilities.Resources)
	require.False(t, capabilities.Resources.ListChanged)
	require.False(t, capabilities.Resources.Subscribe)
	require.NotNil(t, capabilities.Prompts)
	require.False(t, capabilities.Prompts.ListChanged)
	require.Nil(t, capabilities.Logging)
	require.Nil(t, capabilities.Completions)
	require.Empty(t, capabilities.Experimental)
	require.Empty(t, capabilities.Extensions)
}

func requirePrivateCache(t *testing.T, cache mcp.Cacheable, ttlMS int) {
	t.Helper()
	require.Equal(t, ttlMS, cache.TTLMs)
	require.Equal(t, "private", cache.CacheScope)
}
