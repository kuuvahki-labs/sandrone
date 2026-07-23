package mcpapi

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kuuvahki-labs/sandrone/internal/agentcatalog"
	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func registerResources(server *mcp.Server, rt *app.Runtime) {
	addJSONResource(server, "sandrone://capabilities", "capabilities", "Sandrone capability summary", func(context.Context) (any, error) {
		return rt.Service.CapabilitySummary(), nil
	})
	addJSONResource(server, "sandrone://schemas/processors", "processor-schemas", "Public processor schema summary", func(context.Context) (any, error) {
		return agentcatalog.ProcessorSummary(rt.Service.Registry().PublicDescriptors()), nil
	})
	addJSONResource(server, "sandrone://schemas/script-api/v1", "script-api-v1", "Sandboxed script API version 1", func(context.Context) (any, error) {
		return agentcatalog.ScriptAPI()
	})

	addResourceTemplate(server, "sandrone://subscriptions/{name}", "subscriptions", "Stored subscription definitions", func(ctx context.Context, uri string) (any, error) {
		name, err := singleResourceSegment(uri, "subscriptions", "resource name")
		if err != nil {
			return nil, err
		}
		return rt.Service.GetSubscription(ctx, name)
	})
	addResourceTemplate(server, "sandrone://files/{name}", "files", "Stored FileSpec definitions", func(ctx context.Context, uri string) (any, error) {
		name, err := singleResourceSegment(uri, "files", "resource name")
		if err != nil {
			return nil, err
		}
		return rt.Service.GetFileSpec(ctx, name)
	})
	addResourceTemplate(server, "sandrone://schemas/processors/{stage}/{type}", "processor-schema", "Public processor schema by canonical stage and type", func(_ context.Context, uri string) (any, error) {
		stage, processorType, err := processorResourceKey(uri)
		if err != nil {
			return nil, err
		}
		for _, descriptor := range rt.Service.Registry().PublicDescriptors() {
			if descriptor.Stage == stage && descriptor.Type == processorType {
				return agentcatalog.ProcessorDetail(descriptor)
			}
		}
		return nil, domain.NewError(domain.CodeInvalidArgument, "public processor schema not found")
	})
	addResourceTemplate(server, "sandrone://schemas/file-kinds/{kind}", "file-kind-schema", "File-kind settings schema by canonical kind", func(_ context.Context, uri string) (any, error) {
		kind, err := fileKindResourceKey(uri)
		if err != nil {
			return nil, err
		}
		for _, capability := range rt.Service.FileKindCapabilities() {
			if capability.Kind == kind {
				return agentcatalog.FileKindDetail(capability)
			}
		}
		return nil, domain.NewError(domain.CodeInvalidArgument, "file kind schema not found")
	})
}

func addJSONResource(server *mcp.Server, uri, name, desc string, load func(context.Context) (any, error)) {
	server.AddResource(&mcp.Resource{URI: uri, Name: name, Description: desc, MIMEType: "application/json"}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		value, err := load(ctx)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, value)
	})
}

func addResourceTemplate(server *mcp.Server, tmpl, name, desc string, load func(context.Context, string) (any, error)) {
	server.AddResourceTemplate(&mcp.ResourceTemplate{URITemplate: tmpl, Name: name, Description: desc, MIMEType: "application/json"}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		value, err := load(ctx, req.Params.URI)
		if err != nil {
			return nil, err
		}
		return jsonResource(req.Params.URI, value)
	})
}

func jsonResource(uri string, value any) (*mcp.ReadResourceResult, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(body),
		}},
	}, nil
}

func singleResourceSegment(uri, host, label string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if parsed.Host != host {
		return "", domain.NewError(domain.CodeInvalidArgument, "resource URI is invalid")
	}
	name := strings.TrimPrefix(parsed.EscapedPath(), "/")
	if name == "" {
		return "", domain.NewError(domain.CodeInvalidArgument, label+" is required")
	}
	name, err = url.PathUnescape(name)
	if err != nil {
		return "", err
	}
	if err := validateRequiredPublicResourceName(label, name); err != nil {
		return "", err
	}
	return name, nil
}

func processorResourceKey(uri string) (domain.Stage, string, error) {
	parts, err := schemaResourceSegments(uri, "processors", 2)
	if err != nil {
		return "", "", err
	}
	stage := domain.Stage(parts[0])
	if stage != domain.StageNodes && stage != domain.StageFile {
		return "", "", domain.NewError(domain.CodeInvalidArgument, "processor stage must be nodes or file")
	}
	if err := validateRequiredPublicResourceName("processor type", parts[1]); err != nil {
		return "", "", err
	}
	return stage, parts[1], nil
}

func fileKindResourceKey(uri string) (domain.FileKind, error) {
	parts, err := schemaResourceSegments(uri, "file-kinds", 1)
	if err != nil {
		return "", err
	}
	if err := validateRequiredPublicResourceName("file kind", parts[0]); err != nil {
		return "", err
	}
	return domain.FileKind(parts[0]), nil
}

func schemaResourceSegments(uri, collection string, count int) ([]string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if parsed.Host != "schemas" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "schema resource URI is invalid")
	}
	escaped := strings.Split(strings.TrimPrefix(parsed.EscapedPath(), "/"), "/")
	if len(escaped) != count+1 || escaped[0] != collection {
		return nil, domain.NewError(domain.CodeInvalidArgument, "schema resource URI is invalid")
	}
	parts := make([]string, count)
	for index := range parts {
		parts[index], err = url.PathUnescape(escaped[index+1])
		if err != nil {
			return nil, err
		}
	}
	return parts, nil
}
