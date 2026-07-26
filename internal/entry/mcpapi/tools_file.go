package mcpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kuuvahki-labs/sandrone/internal/app"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type validateFileOutput struct {
	OK     bool          `json:"ok"`
	Report domain.Report `json:"report"`
}

type fileSpecOutput struct {
	Name        string                `json:"name"`
	DisplayName string                `json:"display_name,omitempty"`
	Kind        domain.FileKind       `json:"kind"`
	Source      domain.FileSource     `json:"source"`
	Config      *fileConfigOutput     `json:"config,omitempty"`
	Processors  []processorSpecOutput `json:"processors,omitempty"`
	CreatedAt   time.Time             `json:"created_at,omitempty"`
	UpdatedAt   time.Time             `json:"updated_at,omitempty"`
	Meta        map[string]string     `json:"meta,omitempty"`
}

type fileConfigOutput struct {
	Subscriptions []string `json:"subscriptions,omitempty"`
	Settings      any      `json:"settings,omitempty"`
}

type processorSpecOutput struct {
	Type   string         `json:"type"`
	Stage  domain.Stage   `json:"stage,omitempty"`
	Name   string         `json:"name,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

type fileSourceOutput struct {
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	MediaType string            `json:"media_type,omitempty"`
	Encoding  string            `json:"encoding,omitempty"`
	Parts     []filePartOutput  `json:"parts,omitempty"`
	Content   string            `json:"content,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
	Warnings  []domain.Warning  `json:"warnings,omitempty"`
}

type filePartOutput struct {
	Name        string           `json:"name"`
	Role        string           `json:"role"`
	Kind        string           `json:"kind"`
	SourceRef   domain.SourceRef `json:"source_ref,omitempty"`
	Content     string           `json:"content,omitempty"`
	ContentHash string           `json:"content_hash,omitempty"`
	Nodes       []map[string]any `json:"nodes,omitempty"`
}

func registerFileTools(server *mcp.Server, rt *app.Runtime) {
	openWorld := true
	addTool(server, &mcp.Tool{
		Name:        "sandrone_get_file",
		Description: "Get a stored file as rendered content, source content, or a FileSpec.",
		InputSchema: getFileInputSchema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getFileInput) (*mcp.CallToolResult, renderOutput, error) {
		if err := validateRequiredPublicResourceName("file name", in.File); err != nil {
			return nil, renderOutput{}, err
		}
		switch strings.ToLower(strings.TrimSpace(in.Mode)) {
		case "spec":
			spec, err := rt.Service.GetFileSpec(ctx, in.File)
			if err != nil {
				return nil, renderOutput{}, err
			}
			outputSpec, err := newFileSpecOutput(*spec)
			if err != nil {
				return nil, renderOutput{}, err
			}
			return nil, renderOutput{
				Spec: &outputSpec, ResourceURI: definitionResourceURI("files", in.File),
			}, nil
		case "source":
			source, err := rt.Service.GetFileSource(ctx, in.File)
			if err != nil {
				return nil, renderOutput{}, err
			}
			outputSource, err := newFileSourceOutput(*source)
			if err != nil {
				return nil, renderOutput{}, err
			}
			return nil, renderOutput{Source: &outputSource}, nil
		case "", "render":
		default:
			return nil, renderOutput{}, domain.NewError(domain.CodeInvalidArgument, "unsupported file mode")
		}
		result, err := rt.Service.GetFile(ctx, domain.FileRequest{
			Name:    in.File,
			Target:  in.Target,
			Request: domain.RequestInfo{Args: in.Args},
		})
		if err != nil {
			return nil, renderOutput{}, err
		}
		return nil, limitedRenderOutput(rt, renderOutput{
			ContentType: result.ContentType,
			Body:        string(result.Content),
			Report:      result.Report,
		}), nil
	})

	addTool(server, &mcp.Tool{
		Name:        "sandrone_validate_file",
		Description: "Generate and validate a FileSpec without bypassing service file flow.",
		InputSchema: validateFileInputSchema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: &openWorld,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in validateFileInput) (*mcp.CallToolResult, validateFileOutput, error) {
		if err := validateOptionalPublicResourceName("file name", in.File); err != nil {
			return nil, validateFileOutput{}, err
		}
		var spec *domain.FileSpec
		if in.Spec != nil {
			converted, err := in.Spec.domain()
			if err != nil {
				return nil, validateFileOutput{}, err
			}
			spec = &converted
		}
		result, err := rt.Service.ValidateFile(ctx, domain.FileRequest{
			Name:    in.File,
			Spec:    spec,
			Target:  in.Target,
			Request: domain.RequestInfo{Args: in.Args},
		})
		if err != nil {
			return nil, validateFileOutput{}, err
		}
		return nil, validateFileOutput{OK: result.OK, Report: result.Report}, nil
	})
}

func newFileSourceOutput(source domain.FileDocument) (fileSourceOutput, error) {
	output := fileSourceOutput{
		Name: source.Name, Kind: source.Kind, MediaType: source.MediaType,
		Encoding: source.Encoding, Content: string(source.Content),
		Meta: source.Meta, Warnings: source.Warnings,
	}
	if source.Parts != nil {
		output.Parts = make([]filePartOutput, len(source.Parts))
		for index, part := range source.Parts {
			nodes, err := filePartNodesOutput(part.Nodes)
			if err != nil {
				return fileSourceOutput{}, fmt.Errorf("part %d nodes: %w", index, err)
			}
			output.Parts[index] = filePartOutput{
				Name: part.Name, Role: part.Role, Kind: part.Kind, SourceRef: part.SourceRef,
				Content: string(part.Content), ContentHash: part.ContentHash, Nodes: nodes,
			}
		}
	}
	return output, nil
}

func filePartNodesOutput(nodes []domain.NodeIR) ([]map[string]any, error) {
	if nodes == nil {
		return nil, nil
	}
	output := make([]map[string]any, len(nodes))
	for index, node := range nodes {
		body, err := json.Marshal(node) //nolint:gosec // Node credentials are part of the explicitly requested source document.
		if err != nil {
			return nil, fmt.Errorf("node %d: %w", index, err)
		}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&output[index]); err != nil {
			return nil, fmt.Errorf("node %d: %w", index, err)
		}
		if err := ensureJSONEOF(decoder); err != nil {
			return nil, fmt.Errorf("node %d: trailing JSON: %w", index, err)
		}
	}
	return output, nil
}

func newFileSpecOutput(spec domain.FileSpec) (fileSpecOutput, error) {
	output := fileSpecOutput{
		Name: spec.Name, DisplayName: spec.DisplayName, Kind: spec.Kind, Source: spec.Source,
		CreatedAt: spec.CreatedAt, UpdatedAt: spec.UpdatedAt, Meta: spec.Meta,
	}
	if spec.Config != nil {
		config := &fileConfigOutput{Subscriptions: spec.Config.Subscriptions}
		if len(spec.Config.Settings) > 0 {
			if err := json.Unmarshal(spec.Config.Settings, &config.Settings); err != nil {
				return fileSpecOutput{}, fmt.Errorf("config.settings: %w", err)
			}
		}
		output.Config = config
	}
	if spec.Processors != nil {
		output.Processors = make([]processorSpecOutput, len(spec.Processors))
		for index, processor := range spec.Processors {
			params := make(map[string]any, len(processor.Params))
			for name, raw := range processor.Params {
				var value any
				if err := json.Unmarshal(raw, &value); err != nil {
					return fileSpecOutput{}, fmt.Errorf("processor %d params.%s: %w", index, name, err)
				}
				params[name] = value
			}
			output.Processors[index] = processorSpecOutput{
				Type: processor.Type, Stage: processor.Stage, Name: processor.Name, Params: params,
			}
		}
	}
	return output, nil
}
