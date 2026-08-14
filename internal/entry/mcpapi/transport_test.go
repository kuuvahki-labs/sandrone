package mcpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/internal/entry/mcpapi"
)

func TestStreamableHTTPSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := httptest.NewServer(mcpapi.New(testRuntime(t, app.Config{})).Handler())
	defer server.Close()

	session := connectClient(t, ctx, &mcp.StreamableClientTransport{Endpoint: server.URL})
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	require.NotEmpty(t, tools.Tools)
	resources, err := session.ListResources(ctx, nil)
	require.NoError(t, err)
	require.NotEmpty(t, resources.Resources)
	prompts, err := session.ListPrompts(ctx, nil)
	require.NoError(t, err)
	require.NotEmpty(t, prompts.Prompts)

	inspect, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sandrone_inspect",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, inspect.IsError)
	inspectJSON, err := json.Marshal(inspect.StructuredContent)
	require.NoError(t, err)
	require.Contains(t, string(inspectJSON), `"formats"`)
	require.Contains(t, string(inspectJSON), `"catalogs"`)

	schema, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "sandrone://schemas/script-api/v1"})
	require.NoError(t, err)
	require.Len(t, schema.Contents, 1)
	require.Equal(t, "application/json", schema.Contents[0].MIMEType)
	require.Contains(t, schema.Contents[0].Text, `"version": 1`)
}

func TestStreamableHTTPDiscover(t *testing.T) {
	server := httptest.NewServer(mcpapi.New(testRuntime(t, app.Config{})).Handler())
	defer server.Close()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "server/discover",
		"params": map[string]any{
			"_meta": currentRequestMeta(),
		},
	})
	require.NoError(t, err)

	response := doMCPRequest(t, http.MethodPost, server.URL, bytes.NewReader(body), map[string]string{
		"Mcp-Protocol-Version": mcpapi.ProtocolVersion,
		"Mcp-Method":           "server/discover",
	})
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Empty(t, response.Header.Get("Mcp-Session-Id"))

	responseBody, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	var envelope struct {
		Result struct {
			ResultType        string                     `json:"resultType"`
			Meta              map[string]json.RawMessage `json:"_meta"`
			SupportedVersions []string                   `json:"supportedVersions"`
			Capabilities      map[string]json.RawMessage `json:"capabilities"`
			TTLMs             int                        `json:"ttlMs"`
			CacheScope        string                     `json:"cacheScope"`
		} `json:"result"`
		Error *rpcError `json:"error"`
	}
	require.NoError(t, json.Unmarshal(responseBody, &envelope), string(responseBody))
	require.Nil(t, envelope.Error)
	require.Equal(t, "complete", envelope.Result.ResultType)
	require.Equal(t, []string{mcpapi.ProtocolVersion}, envelope.Result.SupportedVersions)
	require.Equal(t, 300_000, envelope.Result.TTLMs)
	require.Equal(t, "private", envelope.Result.CacheScope)
	require.ElementsMatch(t, []string{"prompts", "resources", "tools"}, rawMessageKeys(envelope.Result.Capabilities))

	var serverInfo mcp.Implementation
	require.NoError(t, json.Unmarshal(envelope.Result.Meta[mcp.MetaKeyServerInfo], &serverInfo))
	require.Equal(t, "sandrone", serverInfo.Name)
	require.Equal(t, buildinfo.Version(), serverInfo.Version)
}

func TestStreamableHTTPListMetadata(t *testing.T) {
	server := httptest.NewServer(mcpapi.New(testRuntime(t, app.Config{})).Handler())
	defer server.Close()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params": map[string]any{
			"_meta": currentRequestMeta(),
		},
	})
	require.NoError(t, err)
	response := doMCPRequest(t, http.MethodPost, server.URL, bytes.NewReader(body), map[string]string{
		"Mcp-Protocol-Version": mcpapi.ProtocolVersion,
		"Mcp-Method":           "tools/list",
	})
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Empty(t, response.Header.Get("Mcp-Session-Id"))

	responseBody, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	var envelope struct {
		Result struct {
			ResultType string                     `json:"resultType"`
			Meta       map[string]json.RawMessage `json:"_meta"`
			TTLMs      int                        `json:"ttlMs"`
			CacheScope string                     `json:"cacheScope"`
		} `json:"result"`
		Error *rpcError `json:"error"`
	}
	require.NoError(t, json.Unmarshal(responseBody, &envelope), string(responseBody))
	require.Nil(t, envelope.Error)
	require.Equal(t, "complete", envelope.Result.ResultType)
	require.Equal(t, 300_000, envelope.Result.TTLMs)
	require.Equal(t, "private", envelope.Result.CacheScope)
	require.NotEmpty(t, envelope.Result.Meta[mcp.MetaKeyServerInfo])
}

func TestStreamableHTTPRejectsLegacyInitialize(t *testing.T) {
	server := httptest.NewServer(mcpapi.New(testRuntime(t, app.Config{})).Handler())
	defer server.Close()

	body := []byte(`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"legacy-client","version":"1.0.0"}}}`)
	response := doMCPRequest(t, http.MethodPost, server.URL, bytes.NewReader(body), map[string]string{
		"Mcp-Protocol-Version": "2025-11-25",
	})
	defer response.Body.Close()
	require.Equal(t, http.StatusBadRequest, response.StatusCode)

	envelope := decodeRPCError(t, response.Body)
	require.Equal(t, "2.0", envelope.JSONRPC)
	require.Equal(t, float64(2), envelope.ID)
	require.Equal(t, int64(mcp.CodeUnsupportedProtocolVersion), envelope.Error.Code)
	var data mcp.UnsupportedProtocolVersionData
	require.NoError(t, json.Unmarshal(envelope.Error.Data, &data))
	require.Equal(t, []string{mcpapi.ProtocolVersion}, data.Supported)
	require.Equal(t, "2025-11-25", data.Requested)
}

func TestStreamableHTTPRejectsFutureProtocolVersion(t *testing.T) {
	server := httptest.NewServer(mcpapi.New(testRuntime(t, app.Config{})).Handler())
	defer server.Close()

	const futureVersion = "2027-01-01"
	meta := currentRequestMeta()
	meta[mcp.MetaKeyProtocolVersion] = futureVersion
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "server/discover",
		"params": map[string]any{
			"_meta": meta,
		},
	})
	require.NoError(t, err)
	response := doMCPRequest(t, http.MethodPost, server.URL, bytes.NewReader(body), map[string]string{
		"Mcp-Protocol-Version": futureVersion,
		"Mcp-Method":           "server/discover",
	})
	defer response.Body.Close()
	require.Equal(t, http.StatusBadRequest, response.StatusCode)

	envelope := decodeRPCError(t, response.Body)
	require.Equal(t, "2.0", envelope.JSONRPC)
	require.Equal(t, float64(3), envelope.ID)
	require.Equal(t, int64(mcp.CodeUnsupportedProtocolVersion), envelope.Error.Code)
	var data mcp.UnsupportedProtocolVersionData
	require.NoError(t, json.Unmarshal(envelope.Error.Data, &data))
	require.Equal(t, []string{mcpapi.ProtocolVersion}, data.Supported)
	require.Equal(t, futureVersion, data.Requested)
}

func TestStreamableHTTPPreservesProtocolHeaderMismatch(t *testing.T) {
	server := httptest.NewServer(mcpapi.New(testRuntime(t, app.Config{})).Handler())
	defer server.Close()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/list",
		"params": map[string]any{
			"_meta": currentRequestMeta(),
		},
	})
	require.NoError(t, err)
	response := doMCPRequest(t, http.MethodPost, server.URL, bytes.NewReader(body), map[string]string{
		"Mcp-Protocol-Version": "2025-11-25",
		"Mcp-Method":           "tools/list",
	})
	defer response.Body.Close()
	require.Equal(t, http.StatusBadRequest, response.StatusCode)

	envelope := decodeRPCError(t, response.Body)
	require.Equal(t, int64(mcp.CodeHeaderMismatch), envelope.Error.Code)
}

func TestStreamableHTTPRejectsHeaderMismatch(t *testing.T) {
	server := httptest.NewServer(mcpapi.New(testRuntime(t, app.Config{})).Handler())
	defer server.Close()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/list",
		"params": map[string]any{
			"_meta": currentRequestMeta(),
		},
	})
	require.NoError(t, err)
	response := doMCPRequest(t, http.MethodPost, server.URL, bytes.NewReader(body), map[string]string{
		"Mcp-Protocol-Version": mcpapi.ProtocolVersion,
		"Mcp-Method":           "prompts/list",
	})
	defer response.Body.Close()
	require.Equal(t, http.StatusBadRequest, response.StatusCode)

	envelope := decodeRPCError(t, response.Body)
	require.Equal(t, int64(mcp.CodeHeaderMismatch), envelope.Error.Code)
}

func TestStreamableHTTPRejectsStatelessGETAndDELETE(t *testing.T) {
	server := httptest.NewServer(mcpapi.New(testRuntime(t, app.Config{})).Handler())
	defer server.Close()

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			response := doMCPRequest(t, method, server.URL, nil, nil)
			defer response.Body.Close()
			require.Equal(t, http.StatusMethodNotAllowed, response.StatusCode)
		})
	}
}

func TestStreamableHTTPRejectsOversizedBody(t *testing.T) {
	server := httptest.NewServer(mcpapi.New(testRuntime(t, app.Config{})).Handler())
	defer server.Close()

	response := doMCPRequest(t, http.MethodPost, server.URL,
		bytes.NewReader(make([]byte, mcp.DefaultMaxRequestBodyBytes+1)), map[string]string{
			"Mcp-Protocol-Version": "2027-01-01",
		})
	defer response.Body.Close()
	require.Equal(t, http.StatusRequestEntityTooLarge, response.StatusCode)
}

type rpcError struct {
	Code    int64           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type rpcErrorEnvelope struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Error   *rpcError `json:"error"`
}

func currentRequestMeta() map[string]any {
	return map[string]any{
		mcp.MetaKeyProtocolVersion:    mcpapi.ProtocolVersion,
		mcp.MetaKeyClientInfo:         map[string]any{"name": "test-client", "version": "1.0.0"},
		mcp.MetaKeyClientCapabilities: map[string]any{},
	}
}

func doMCPRequest(t *testing.T, method string, url string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, body)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	return response
}

func decodeRPCError(t *testing.T, body io.Reader) rpcErrorEnvelope {
	t.Helper()
	responseBody, err := io.ReadAll(body)
	require.NoError(t, err)
	var envelope rpcErrorEnvelope
	require.NoError(t, json.Unmarshal(responseBody, &envelope), string(responseBody))
	require.NotNil(t, envelope.Error, string(responseBody))
	return envelope
}

func rawMessageKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
