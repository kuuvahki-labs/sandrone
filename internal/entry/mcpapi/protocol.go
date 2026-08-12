package mcpapi

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ProtocolVersion   = "2026-07-28"
	privateCacheTTLMS = 300_000
)

type protocolVersionRequest interface {
	ProtocolVersion() string
}

func protocolPolicyMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		requested := requestProtocolVersion(method, req)
		if method == "initialize" || requested != ProtocolVersion {
			return nil, unsupportedProtocolVersion(requested)
		}

		result, err := next(ctx, method, req)
		if err != nil {
			return nil, err
		}
		applyProtocolPolicy(result)
		return result, nil
	}
}

func requestProtocolVersion(method string, req mcp.Request) string {
	if method == "initialize" {
		if params, ok := req.GetParams().(*mcp.InitializeParams); ok && params != nil {
			return params.ProtocolVersion
		}
	}
	if versioned, ok := req.(protocolVersionRequest); ok {
		return versioned.ProtocolVersion()
	}
	return ""
}

func unsupportedProtocolVersion(requested string) error {
	data, err := json.Marshal(mcp.UnsupportedProtocolVersionData{
		Supported: []string{ProtocolVersion},
		Requested: requested,
	})
	if err != nil {
		panic("marshal static MCP protocol version error: " + err.Error())
	}
	return &jsonrpc.Error{
		Code:    mcp.CodeUnsupportedProtocolVersion,
		Message: "unsupported protocol version",
		Data:    data,
	}
}

func applyProtocolPolicy(result mcp.Result) {
	switch result := result.(type) {
	case *mcp.DiscoverResult:
		result.SupportedVersions = []string{ProtocolVersion}
		setPrivateCache(&result.Cacheable, privateCacheTTLMS)
	case *mcp.ListToolsResult:
		setPrivateCache(&result.Cacheable, privateCacheTTLMS)
	case *mcp.ListPromptsResult:
		setPrivateCache(&result.Cacheable, privateCacheTTLMS)
	case *mcp.ListResourcesResult:
		setPrivateCache(&result.Cacheable, privateCacheTTLMS)
	case *mcp.ListResourceTemplatesResult:
		setPrivateCache(&result.Cacheable, privateCacheTTLMS)
	case *mcp.ReadResourceResult:
		setPrivateCache(&result.Cacheable, 0)
	}
}

func setPrivateCache(cache *mcp.Cacheable, ttlMS int) {
	cache.TTLMs = ttlMS
	cache.CacheScope = "private"
}
