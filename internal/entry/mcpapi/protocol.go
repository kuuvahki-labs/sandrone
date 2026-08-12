package mcpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

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

func unsupportedProtocolVersion(requested string) *jsonrpc.Error {
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

func strictProtocolHTTPHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requested := req.Header.Get("Mcp-Protocol-Version")
		if req.Method != http.MethodPost || requested == "" || requested == ProtocolVersion {
			next.ServeHTTP(w, req)
			return
		}

		request, ok := decodeProtocolRequest(req)
		if !ok {
			next.ServeHTTP(w, req)
			return
		}
		if bodyVersion := protocolVersionFromParams(request.Params); bodyVersion != "" && bodyVersion != requested {
			// Let the SDK preserve the standard HeaderMismatch error when the
			// version mirror disagrees with the JSON-RPC body.
			next.ServeHTTP(w, req)
			return
		}

		response, err := jsonrpc.EncodeMessage(&jsonrpc.Response{
			ID:    request.ID,
			Error: unsupportedProtocolVersion(requested),
		})
		if err != nil {
			http.Error(w, "failed to encode protocol error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(response)
	})
}

func decodeProtocolRequest(req *http.Request) (*jsonrpc.Request, bool) {
	if req.Body == nil {
		return nil, false
	}
	original := req.Body
	prefix, err := io.ReadAll(io.LimitReader(original, mcp.DefaultMaxRequestBodyBytes+1))
	req.Body = &replayReadCloser{
		Reader: io.MultiReader(bytes.NewReader(prefix), original),
		Closer: original,
	}
	if err != nil || int64(len(prefix)) > mcp.DefaultMaxRequestBodyBytes {
		return nil, false
	}
	message, err := jsonrpc.DecodeMessage(prefix)
	if err != nil {
		return nil, false
	}
	request, ok := message.(*jsonrpc.Request)
	return request, ok
}

func protocolVersionFromParams(params json.RawMessage) string {
	var values struct {
		Meta            map[string]any `json:"_meta"`
		ProtocolVersion string         `json:"protocolVersion"`
	}
	if err := json.Unmarshal(params, &values); err != nil {
		return ""
	}
	if version, ok := values.Meta[mcp.MetaKeyProtocolVersion].(string); ok && version != "" {
		return version
	}
	return values.ProtocolVersion
}

type replayReadCloser struct {
	io.Reader
	io.Closer
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
