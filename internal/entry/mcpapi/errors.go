package mcpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const internalErrorCode = "internal_error"

type toolErrorEnvelope struct {
	Error toolErrorDetail `json:"error"`
}

type toolErrorDetail struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	Field        string `json:"field,omitempty"`
	ResourceKind string `json:"resource_kind,omitempty"`
	ResourceName string `json:"resource_name,omitempty"`
	Source       string `json:"source,omitempty"`
	Target       string `json:"target,omitempty"`
	File         string `json:"file,omitempty"`
	Part         string `json:"part,omitempty"`
	Processor    string `json:"processor,omitempty"`
	Path         string `json:"path,omitempty"`
}

type toolErrorContext struct {
	Field        string
	ResourceKind string
	ResourceName string
}

func invalidToolArgument(err error, field string) *mcp.CallToolResult {
	return newToolErrorResult(domain.NewError(domain.CodeInvalidArgument, err.Error()), toolErrorContext{Field: field})
}

func newToolErrorResult(err error, context toolErrorContext) *mcp.CallToolResult {
	detail := toolErrorDetail{
		Code:         internalErrorCode,
		Message:      "tool execution failed",
		Field:        context.Field,
		ResourceKind: context.ResourceKind,
		ResourceName: context.ResourceName,
	}
	appErr, hasAppErr := errors.AsType[*domain.AppError](err)
	switch {
	case hasAppErr:
		detail.Code = string(appErr.Code)
		detail.Message = appErr.Message
		detail.Source = publicErrorSource(appErr.Source)
		detail.Target = appErr.Target
		detail.File = appErr.File
		detail.Part = appErr.Part
		detail.Processor = appErr.Processor
		detail.Path = appErr.Path
	case errors.Is(err, os.ErrNotExist):
		detail.Code = string(domain.CodeFileInputNotFound)
		detail.Message = "requested resource was not found"
	}
	envelope := toolErrorEnvelope{Error: detail}
	text := fmt.Sprintf("%s: %s", detail.Code, detail.Message)
	return &mcp.CallToolResult{
		IsError:           true,
		Content:           []mcp.Content{&mcp.TextContent{Text: text}},
		StructuredContent: envelope,
	}
}

func publicErrorSource(source string) string {
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return source
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String()
}

var (
	schemaFullPathPattern     = regexp.MustCompile(`(?:/(?:properties/[^/:]+|items))+`)
	schemaPathPattern         = regexp.MustCompile(`/properties/([^/:]+)|/items`)
	missingPropertyPattern    = regexp.MustCompile(`missing properties:?\s+\["([^"]+)"`)
	additionalPropertyPattern = regexp.MustCompile(`unexpected additional properties \["([^"]+)"`)
	unknownFieldPattern       = regexp.MustCompile(`unknown field "([^"]+)"`)
	settingsPathPattern       = regexp.MustCompile(`config\.settings(?:\.[A-Za-z0-9_-]+)*`)
	validationValuePattern    = regexp.MustCompile(`(?:enum|type):\s+"?([^"\s]+)"?`)
)

func schemaErrorField(err error, input map[string]any) string {
	message := err.Error()
	fullPaths := schemaFullPathPattern.FindAllString(message, -1)
	longestPath := ""
	for _, path := range fullPaths {
		if len(path) > len(longestPath) {
			longestPath = path
		}
	}
	matches := schemaPathPattern.FindAllStringSubmatch(longestPath, -1)
	fields := make([]string, 0, len(matches))
	var current any = input
	if len(matches) > 0 {
		for index, match := range matches {
			if match[1] == "" {
				if len(fields) > 0 {
					itemIndex := schemaErrorItemIndex(message, current, matches[index+1:])
					fields[len(fields)-1] += fmt.Sprintf("[%d]", itemIndex)
					if items, ok := current.([]any); ok && itemIndex < len(items) {
						current = items[itemIndex]
					}
				}
				continue
			}
			field := strings.ReplaceAll(strings.ReplaceAll(match[1], "~1", "/"), "~0", "~")
			if len(fields) == 0 || fields[len(fields)-1] != field {
				fields = append(fields, field)
			}
			if object, ok := current.(map[string]any); ok {
				current = object[field]
			}
		}
	}
	for _, pattern := range []*regexp.Regexp{missingPropertyPattern, additionalPropertyPattern} {
		if match := pattern.FindStringSubmatch(message); len(match) == 2 {
			if len(fields) == 0 || fields[len(fields)-1] != match[1] {
				fields = append(fields, match[1])
			}
			break
		}
	}
	return strings.Join(fields, ".")
}

func schemaErrorItemIndex(message string, current any, remaining [][]string) int {
	items, ok := current.([]any)
	if !ok {
		return 0
	}
	missing := ""
	if match := missingPropertyPattern.FindStringSubmatch(message); len(match) == 2 {
		missing = match[1]
	}
	nextProperty := ""
	for _, match := range remaining {
		if match[1] != "" {
			nextProperty = match[1]
			break
		}
	}
	invalidValue := ""
	if match := validationValuePattern.FindStringSubmatch(message); len(match) == 2 {
		invalidValue = match[1]
	}
	for index, item := range items {
		object, _ := item.(map[string]any)
		if missing != "" {
			if _, exists := object[missing]; !exists {
				return index
			}
		}
		if nextProperty != "" {
			if invalidValue != "" {
				if fmt.Sprint(object[nextProperty]) == invalidValue {
					return index
				}
				continue
			}
			valueJSON, _ := json.Marshal(object[nextProperty])
			valueText := strings.Trim(string(valueJSON), `"`)
			if valueText != "" && strings.Contains(message, valueText) {
				return index
			}
		}
	}
	return 0
}

func contextualizeToolError(err error, context toolErrorContext, toolName string, input map[string]any) toolErrorContext {
	appErr, ok := errors.AsType[*domain.AppError](err)
	if !ok {
		return context
	}
	if context.Field == "" && appErr.Path != "" {
		context.Field = appErr.Path
	}
	if context.Field == "" {
		if path := settingsPathPattern.FindString(appErr.Message); path != "" {
			context.Field = path
		}
	}
	if context.Field == "" && toolName == "sandrone_convert" {
		if _, ok := input["remote"]; ok && appErr.Code == domain.CodeFileInputNotFound {
			context.Field = "remote.url"
		} else {
			context.Field = processorWireField(appErr, input)
		}
	}
	return context
}

func processorWireField(appErr *domain.AppError, input map[string]any) string {
	if appErr.Processor == "" {
		return ""
	}
	matches := processorWireMatches(input, "", appErr.Processor)
	if len(matches) != 1 {
		return ""
	}
	base := matches[0]
	if strings.Contains(appErr.Message, "stage must be set") {
		return base + ".stage"
	}
	if match := unknownFieldPattern.FindStringSubmatch(errorCauseText(appErr)); len(match) == 2 {
		return base + ".params." + match[1]
	}
	for _, field := range []string{"mode", "pattern", "value", "source", "timeout_ms"} {
		if strings.Contains(appErr.Message, field) {
			return base + ".params." + field
		}
	}
	return base
}

func processorWireMatches(value any, path string, processorType string) []string {
	var matches []string
	switch value := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			child := value[key]
			if key == "parse_processors" || key == "render_processors" || key == "processors" {
				processors, _ := child.([]any)
				for index, raw := range processors {
					processor, _ := raw.(map[string]any)
					if processor["type"] == processorType {
						matches = append(matches, fmt.Sprintf("%s[%d]", childPath, index))
					}
				}
			}
			matches = append(matches, processorWireMatches(child, childPath, processorType)...)
		}
	case []any:
		for index, child := range value {
			matches = append(matches, processorWireMatches(child, fmt.Sprintf("%s[%d]", path, index), processorType)...)
		}
	}
	return matches
}

func errorCauseText(appErr *domain.AppError) string {
	if appErr == nil || appErr.Cause == nil {
		return ""
	}
	return appErr.Cause.Error()
}
