package mcpapi_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

type listedResources struct {
	Items []struct {
		Kind        string `json:"kind"`
		Name        string `json:"name"`
		ResourceURI string `json:"resource_uri"`
	} `json:"items"`
	NextCursor string `json:"next_cursor"`
}

func TestListResourcesReturnsEmptyListsAndEachStoredKind(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	emptyResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "sandrone_list_resources"})
	require.NoError(t, err)
	emptyJSON, err := json.Marshal(emptyResult.StructuredContent)
	require.NoError(t, err)
	require.Contains(t, string(emptyJSON), `"items":[]`)

	for _, arguments := range []map[string]any{
		{},
		{"kind": "subscription"},
		{"kind": "file"},
	} {
		got := callListResources(t, ctx, session, arguments)
		require.Empty(t, got.Items)
		require.Empty(t, got.NextCursor)
	}

	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "beta", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
	}))
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name: "alpha.yaml", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "body"},
	}))

	subscriptions := callListResources(t, ctx, session, map[string]any{"kind": "subscription"})
	require.Equal(t, []string{"subscription:beta:sandrone://subscriptions/beta"}, listedResourceKeys(subscriptions))

	files := callListResources(t, ctx, session, map[string]any{"kind": "file"})
	require.Equal(t, []string{"file:alpha.yaml:sandrone://files/alpha.yaml"}, listedResourceKeys(files))

	both := callListResources(t, ctx, session, nil)
	require.Equal(t, []string{
		"file:alpha.yaml:sandrone://files/alpha.yaml",
		"subscription:beta:sandrone://subscriptions/beta",
	}, listedResourceKeys(both))
}

func TestListResourcesPaginatesStableSortedItemsWithoutDuplicatesOrSkips(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
			Name: name, Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		}))
	}
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	var names []string
	cursor := ""
	for {
		arguments := map[string]any{"kind": "subscription", "limit": 1}
		if cursor != "" {
			arguments["cursor"] = cursor
		}
		page := callListResources(t, ctx, session, arguments)
		require.Len(t, page.Items, 1)
		require.NotEmpty(t, page.Items[0].ResourceURI)
		names = append(names, page.Items[0].Name)
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	require.Equal(t, []string{"alpha", "bravo", "charlie"}, names)
}

func TestListResourcesCursorSurvivesServerRebuild(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
			Name: name, Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		}))
	}

	sessionA := connect(t, ctx, mcpapi.SDKServer(rt))
	page1 := callListResources(t, ctx, sessionA, map[string]any{
		"kind": "subscription", "limit": 1,
	})
	require.NoError(t, sessionA.Close())
	require.Equal(t, []string{"alpha"}, listedResourceNames(page1))
	require.NotEmpty(t, page1.NextCursor)

	sessionB := connect(t, ctx, mcpapi.SDKServer(rt))
	defer sessionB.Close()
	page2 := callListResources(t, ctx, sessionB, map[string]any{
		"kind": "subscription", "limit": 1, "cursor": page1.NextCursor,
	})
	require.Equal(t, []string{"bravo"}, listedResourceNames(page2))
	require.NotEmpty(t, page2.NextCursor)
	page3 := callListResources(t, ctx, sessionB, map[string]any{
		"kind": "subscription", "limit": 1, "cursor": page2.NextCursor,
	})
	require.Equal(t, []string{"charlie"}, listedResourceNames(page3))
	require.Empty(t, page3.NextCursor)
}

func TestListResourcesReadsCurrentStoreStateOnEveryCall(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	require.Empty(t, callListResources(t, ctx, session, nil).Items)
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name: "live.txt", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "live"},
	}))
	require.Equal(t, []string{"file:live.txt:sandrone://files/live.txt"},
		listedResourceKeys(callListResources(t, ctx, session, nil)))
	require.NoError(t, rt.Service.DeleteFile(ctx, "live.txt"))
	require.Empty(t, callListResources(t, ctx, session, nil).Items)
}

func TestListResourcesDoesNotPublishUnreadableMultiSegmentNames(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "remote/provider", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
	}))
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name: "stored/base.txt", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "body"},
	}))
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	require.Empty(t, callListResources(t, ctx, session, nil).Items)
}

func TestListResourcesRejectsInvalidKindCursorAndLimits(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name: "one.txt", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "one"},
	}))
	require.NoError(t, rt.Service.PutFile(ctx, domain.FileSpec{
		Name: "two.txt", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "two"},
	}))
	session := connect(t, ctx, mcpapi.SDKServer(rt))
	defer session.Close()

	cursor := callListResources(t, ctx, session, map[string]any{"kind": "file", "limit": 1}).NextCursor
	require.NotEmpty(t, cursor)
	decodedCursor, err := base64.RawURLEncoding.DecodeString(cursor)
	require.NoError(t, err)
	cursorWithTrailingJSON := base64.RawURLEncoding.EncodeToString(append(decodedCursor, []byte(`{}`)...))

	tests := []struct {
		name      string
		arguments map[string]any
	}{
		{name: "kind", arguments: map[string]any{"kind": "share"}},
		{name: "malformed cursor", arguments: map[string]any{"kind": "file", "cursor": "not-a-cursor"}},
		{name: "tampered cursor", arguments: map[string]any{"kind": "file", "cursor": "A" + cursor[1:]}},
		{name: "trailing cursor data", arguments: map[string]any{"kind": "file", "cursor": cursorWithTrailingJSON}},
		{name: "mismatched cursor", arguments: map[string]any{"kind": "subscription", "cursor": cursor}},
		{name: "zero limit", arguments: map[string]any{"limit": 0}},
		{name: "negative limit", arguments: map[string]any{"limit": -1}},
		{name: "excessive limit", arguments: map[string]any{"limit": 201}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := callToolError(t, ctx, session, "sandrone_list_resources", test.arguments)
			require.NotEmpty(t, body)
		})
	}
}

func TestInspectCapabilitiesReturnsSummaryAndFilteredOwnerMetadata(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	summary := callInspectCapabilities(t, ctx, session, nil)
	require.Contains(t, summary, "parse_formats")
	require.Contains(t, summary, "render_formats")
	require.Contains(t, summary, "capabilities")
	summaryJSON, err := json.Marshal(summary)
	require.NoError(t, err)
	require.NotContains(t, string(summaryJSON), "inject_nodes")

	format := callInspectCapabilities(t, ctx, session, map[string]any{
		"kind": "format", "name": "uri-list",
	})
	formatItems, ok := format["formats"].([]any)
	require.True(t, ok)
	require.Len(t, formatItems, 2)

	processor := callInspectCapabilities(t, ctx, session, map[string]any{
		"kind": "processor", "name": "rename",
	})
	processorItems, ok := processor["processors"].([]any)
	require.True(t, ok)
	require.Len(t, processorItems, 1)
	require.Equal(t, "rename", processorItems[0].(map[string]any)["type"])
	require.Contains(t, processorItems[0].(map[string]any), "params_schema")

	fileKind := callInspectCapabilities(t, ctx, session, map[string]any{
		"kind": "file_kind", "name": "mihomo",
	})
	fileKindItems, ok := fileKind["file_kinds"].([]any)
	require.True(t, ok)
	require.Len(t, fileKindItems, 1)
	require.Equal(t, "mihomo", fileKindItems[0].(map[string]any)["kind"])
	require.Contains(t, fileKindItems[0].(map[string]any), "settings_schema")
}

func TestInspectCapabilitiesRejectsUnknownOrInternalCapabilities(t *testing.T) {
	ctx := context.Background()
	session := connect(t, ctx, mcpapi.SDKServer(testRuntime(t, app.Config{})))
	defer session.Close()

	for _, arguments := range []map[string]any{
		{"kind": "format", "name": "future"},
		{"kind": "processor", "name": "future"},
		{"kind": "file_kind", "name": "future"},
		{"kind": "processor", "name": "inject_nodes"},
		{"kind": "processor"},
		{"name": "rename"},
	} {
		body := callToolError(t, ctx, session, "sandrone_inspect_capabilities", arguments)
		require.NotEmpty(t, body)
	}
}

func callListResources(t *testing.T, ctx context.Context, session *mcp.ClientSession, arguments map[string]any) listedResources {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "sandrone_list_resources", Arguments: arguments})
	require.NoError(t, err)
	require.False(t, result.IsError)
	var output listedResources
	decodeStructuredContent(t, result, &output)
	return output
}

func callInspectCapabilities(t *testing.T, ctx context.Context, session *mcp.ClientSession, arguments map[string]any) map[string]any {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "sandrone_inspect_capabilities", Arguments: arguments})
	require.NoError(t, err)
	require.False(t, result.IsError)
	var output struct {
		Capabilities map[string]any `json:"capabilities"`
	}
	decodeStructuredContent(t, result, &output)
	return output.Capabilities
}

func decodeStructuredContent(t *testing.T, result *mcp.CallToolResult, output any) {
	t.Helper()
	data, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, output))
}

func listedResourceKeys(resources listedResources) []string {
	keys := make([]string, len(resources.Items))
	for i, item := range resources.Items {
		keys[i] = item.Kind + ":" + item.Name + ":" + item.ResourceURI
	}
	return keys
}

func listedResourceNames(resources listedResources) []string {
	names := make([]string, len(resources.Items))
	for i, item := range resources.Items {
		names[i] = item.Name
	}
	return names
}
