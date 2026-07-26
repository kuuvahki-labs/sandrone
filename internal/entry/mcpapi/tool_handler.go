package mcpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func addTool[In, Out any](
	server *mcp.Server,
	tool *mcp.Tool,
	handler mcp.ToolHandlerFor[In, Out],
) {
	inputSchema, ok := tool.InputSchema.(*jsonschema.Schema)
	if !ok {
		panic(fmt.Sprintf("addTool %q: input schema must be *jsonschema.Schema", tool.Name))
	}
	inputResolved, err := inputSchema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		panic(fmt.Sprintf("addTool %q: resolve input schema: %v", tool.Name, err))
	}

	outputSchema, err := jsonschema.For[Out](nil)
	if err != nil {
		panic(fmt.Sprintf("addTool %q: infer output schema: %v", tool.Name, err))
	}
	outputResolved, err := outputSchema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		panic(fmt.Sprintf("addTool %q: resolve output schema: %v", tool.Name, err))
	}
	errorSchema, err := jsonschema.For[toolErrorEnvelope](nil)
	if err != nil {
		panic(fmt.Sprintf("addTool %q: infer error schema: %v", tool.Name, err))
	}
	registeredTool := *tool
	registeredTool.OutputSchema = &jsonschema.Schema{
		Type:  "object",
		OneOf: []*jsonschema.Schema{outputSchema, errorSchema},
	}

	server.AddTool(&registeredTool, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw := json.RawMessage(`{}`)
		if request.Params.Arguments != nil {
			raw = request.Params.Arguments
		}
		validatedInput, genericInput, err := applyToolSchema(raw, inputResolved)
		if err != nil {
			return invalidToolArgument(err, schemaErrorField(err, genericInput)), nil
		}

		var input In
		if err := decodeToolJSON(validatedInput, &input); err != nil {
			return invalidToolArgument(err, schemaErrorField(err, genericInput)), nil
		}
		context := toolContext(registeredTool.Name, genericInput)

		result, output, err := handler(ctx, request, input)
		if err != nil {
			return newToolErrorResult(err, contextualizeToolError(err, context, registeredTool.Name, genericInput)), nil
		}
		if result == nil {
			result = &mcp.CallToolResult{}
		}

		outputJSON, err := json.Marshal(output)
		if err != nil {
			return nil, fmt.Errorf("marshal %q output: %w", registeredTool.Name, err)
		}
		validatedOutput, _, err := applyToolSchema(outputJSON, outputResolved)
		if err != nil {
			return nil, fmt.Errorf("validate %q output: %w", registeredTool.Name, err)
		}
		result.StructuredContent = validatedOutput
		if result.Content == nil {
			result.Content = []mcp.Content{&mcp.TextContent{Text: string(validatedOutput)}}
		}
		return result, nil
	})
}

func applyToolSchema(raw json.RawMessage, resolved *jsonschema.Resolved) (json.RawMessage, map[string]any, error) {
	var value map[string]any
	if err := decodeToolJSON(raw, &value); err != nil {
		return nil, nil, fmt.Errorf("decode JSON object: %w", err)
	}
	if value == nil {
		value = make(map[string]any)
	}
	if err := resolved.ApplyDefaults(&value); err != nil {
		return nil, nil, fmt.Errorf("apply schema defaults: %w", err)
	}
	if err := resolved.Validate(value); err != nil {
		return nil, value, err
	}
	validated, err := json.Marshal(value)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal validated JSON: %w", err)
	}
	return validated, value, nil
}

func decodeToolJSON(raw []byte, output any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func toolContext(toolName string, input map[string]any) toolErrorContext {
	var kind, field string
	switch toolName {
	case "sandrone_preview_subscription", "sandrone_render_subscription",
		"sandrone_get_subscription_traffic", "sandrone_put_subscription",
		"sandrone_delete_subscription":
		kind, field = "subscription", "name"
	case "sandrone_get_file", "sandrone_validate_file":
		kind, field = "file", "file"
	case "sandrone_put_file", "sandrone_delete_file":
		kind, field = "file", "name"
	default:
		return toolErrorContext{}
	}
	name, _ := input[field].(string)
	if name == "" && toolName == "sandrone_validate_file" {
		if spec, ok := input["spec"].(map[string]any); ok {
			name, _ = spec["name"].(string)
		}
	}
	return toolErrorContext{ResourceKind: kind, ResourceName: name}
}
