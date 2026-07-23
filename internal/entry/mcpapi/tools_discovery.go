package mcpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/agentcatalog"
	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const (
	defaultResourceListLimit = 50
	maxResourceListLimit     = 200
	resourceCursorVersion    = 1
)

type listResourcesInput struct {
	Kind   string `json:"kind,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type listedResource struct {
	domain.ResourceSummary
	ResourceURI string `json:"resource_uri"`
}

type listResourcesOutput struct {
	Items      []listedResource `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type inspectCapabilitiesInput struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`
}

type resourceCursor struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Offset  string `json:"offset"`
	Check   string `json:"check"`
}

func registerDiscoveryTools(server *mcp.Server, rt *app.Runtime) {
	openWorld := false
	addTool(server, &mcp.Tool{
		Name:        "sandrone_list_resources",
		Description: "List current stored subscription and file summaries with restart-stable cursor pagination.",
		InputSchema: listResourcesInputSchema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listResourcesInput) (*mcp.CallToolResult, listResourcesOutput, error) {
		output, err := listResources(ctx, rt, in)
		return nil, output, err
	})

	addTool(server, &mcp.Tool{
		Name:        "sandrone_inspect_capabilities",
		Description: "Inspect the capability summary or one format, public processor, or file kind.",
		InputSchema: inspectCapabilitiesInputSchema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in inspectCapabilitiesInput) (*mcp.CallToolResult, inspectOutput, error) {
		result, err := rt.Service.Inspect(ctx, domain.InspectRequest{})
		if err != nil {
			return nil, inspectOutput{}, err
		}
		if in.Kind == "" && in.Name == "" {
			keepOnlyPublicProcessorNames(rt, result.Capabilities)
			return nil, inspectOutput{Capabilities: result.Capabilities, Report: result.Report}, nil
		}
		if in.Kind == "" || in.Name == "" {
			return nil, inspectOutput{}, domain.NewError(domain.CodeInvalidArgument, "capability kind and name must be provided together")
		}
		filtered, err := filteredCapabilities(rt, in.Kind, in.Name)
		if err != nil {
			return nil, inspectOutput{}, err
		}
		return nil, inspectOutput{Capabilities: filtered, Report: result.Report}, nil
	})
}

func listResources(ctx context.Context, rt *app.Runtime, input listResourcesInput) (listResourcesOutput, error) {
	kind := input.Kind
	if kind != "" && kind != "subscription" && kind != "file" {
		return listResourcesOutput{}, domain.NewError(domain.CodeInvalidArgument, "resource kind must be subscription or file")
	}
	limit := input.Limit
	if limit == 0 {
		limit = defaultResourceListLimit
	}
	if limit < 1 || limit > maxResourceListLimit {
		return listResourcesOutput{}, domain.NewError(domain.CodeInvalidArgument, "resource limit must be between 1 and 200")
	}

	offset := ""
	if input.Cursor != "" {
		decoded, err := decodeResourceCursor(input.Cursor, kind)
		if err != nil {
			return listResourcesOutput{}, err
		}
		offset = decoded
	}

	items, err := currentListedResources(ctx, rt, kind)
	if err != nil {
		return listResourcesOutput{}, err
	}
	sort.Slice(items, func(i, j int) bool {
		return listedResourceOffset(items[i]) < listedResourceOffset(items[j])
	})
	start := sort.Search(len(items), func(index int) bool {
		return listedResourceOffset(items[index]) > offset
	})
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	page := make([]listedResource, end-start)
	copy(page, items[start:end])
	output := listResourcesOutput{Items: page}
	if end < len(items) {
		output.NextCursor = encodeResourceCursor(kind, listedResourceOffset(items[end-1]))
	}
	return output, nil
}

func currentListedResources(ctx context.Context, rt *app.Runtime, kind string) ([]listedResource, error) {
	items := []listedResource{}
	if kind == "" || kind == "subscription" {
		result, err := rt.Service.ListSubscriptions(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range result.Items {
			if validateRequiredPublicResourceName("subscription name", item.Name) != nil {
				continue
			}
			item.Kind = "subscription"
			items = append(items, listedResource{
				ResourceSummary: item,
				ResourceURI:     definitionResourceURI("subscriptions", item.Name),
			})
		}
	}
	if kind == "" || kind == "file" {
		result, err := rt.Service.ListFiles(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range result.Items {
			if validateRequiredPublicResourceName("file name", item.Name) != nil {
				continue
			}
			item.Kind = "file"
			items = append(items, listedResource{
				ResourceSummary: item,
				ResourceURI:     definitionResourceURI("files", item.Name),
			})
		}
	}
	return items, nil
}

func listedResourceOffset(item listedResource) string {
	return item.Kind + "\x00" + item.Name
}

func encodeResourceCursor(kind, offset string) string {
	cursor := resourceCursor{Version: resourceCursorVersion, Kind: kind, Offset: offset}
	cursor.Check = resourceCursorCheck(cursor.Version, cursor.Kind, cursor.Offset)
	encoded, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeResourceCursor(encoded, expectedKind string) (string, error) {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", domain.NewError(domain.CodeInvalidArgument, "resource cursor is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cursor resourceCursor
	if err := decoder.Decode(&cursor); err != nil {
		return "", domain.NewError(domain.CodeInvalidArgument, "resource cursor is invalid")
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return "", domain.NewError(domain.CodeInvalidArgument, "resource cursor is invalid")
	}
	if cursor.Version != resourceCursorVersion ||
		cursor.Kind != expectedKind ||
		cursor.Offset == "" ||
		cursor.Check != resourceCursorCheck(cursor.Version, cursor.Kind, cursor.Offset) {
		return "", domain.NewError(domain.CodeInvalidArgument, "resource cursor is invalid or does not match resource kind")
	}
	return cursor.Offset, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return io.ErrUnexpectedEOF
	}
	return err
}

func resourceCursorCheck(version int, kind, offset string) string {
	sum := sha256.Sum256([]byte("sandrone:mcp:list-resources:cursor:v1\x00" +
		strconv.Itoa(version) + "\x00" + kind + "\x00" + offset))
	return hex.EncodeToString(sum[:])
}

func keepOnlyPublicProcessorNames(rt *app.Runtime, capabilities map[string]any) {
	nodeTypes := []string{}
	fileTypes := []string{}
	for _, descriptor := range rt.Service.Registry().PublicDescriptors() {
		switch descriptor.Stage {
		case domain.StageNodes:
			nodeTypes = append(nodeTypes, descriptor.Type)
		case domain.StageFile:
			fileTypes = append(fileTypes, descriptor.Type)
		}
	}
	capabilities["node_processors"] = nodeTypes
	capabilities["file_processors"] = fileTypes
}

func filteredCapabilities(rt *app.Runtime, kind, name string) (map[string]any, error) {
	switch kind {
	case "format":
		summary := rt.Service.CapabilitySummary()
		capabilities, _ := summary["capabilities"].([]shared.Capability)
		matches := make([]shared.Capability, 0, 2)
		for _, capability := range capabilities {
			if capability.Format == name {
				matches = append(matches, capability)
			}
		}
		if len(matches) == 0 {
			return nil, domain.NewError(domain.CodeInvalidArgument, "format capability not found")
		}
		return map[string]any{"formats": matches}, nil
	case "processor":
		matches := make([]agentcatalog.ProcessorCatalogDocument, 0, 2)
		for _, descriptor := range rt.Service.Registry().PublicDescriptors() {
			if descriptor.Type != name {
				continue
			}
			document, err := agentcatalog.ProcessorDetail(descriptor)
			if err != nil {
				return nil, err
			}
			matches = append(matches, document)
		}
		if len(matches) == 0 {
			return nil, domain.NewError(domain.CodeInvalidArgument, "public processor capability not found")
		}
		return map[string]any{"processors": matches}, nil
	case "file_kind":
		matches := make([]agentcatalog.FileKindCatalogDocument, 0, 1)
		for _, capability := range rt.Service.FileKindCapabilities() {
			if string(capability.Kind) != name {
				continue
			}
			document, err := agentcatalog.FileKindDetail(capability)
			if err != nil {
				return nil, err
			}
			matches = append(matches, document)
		}
		if len(matches) == 0 {
			return nil, domain.NewError(domain.CodeInvalidArgument, "file kind capability not found")
		}
		return map[string]any{"file_kinds": matches}, nil
	default:
		return nil, domain.NewError(domain.CodeInvalidArgument, "capability kind must be format, processor, or file_kind")
	}
}
