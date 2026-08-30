package service

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func refNameNode(input domain.NodeInput) string {
	if input.Ref.Name != "" {
		return input.Ref.Name
	}
	if input.Path != "" {
		return input.Path
	}
	return input.Name
}

func nodeInputReadError(input domain.NodeInput, err error) error {
	if os.IsNotExist(err) {
		return missingNodeInputError(input, fmt.Sprintf("node input %q not found", input.Name), err)
	}
	return err
}

func missingNodeInputError(input domain.NodeInput, message string, cause error) error {
	return &domain.AppError{
		Code:    domain.CodeFileInputNotFound,
		Message: message,
		Part:    input.Name,
		Source:  refNameNode(input),
		Cause:   cause,
	}
}

func warningForNodeInputError(input domain.NodeInput, err error) domain.Warning {
	code := "node_input_not_found"
	message := fmt.Sprintf("node input %q not found; skipping", input.Name)
	if domain.IsCode(err, domain.CodeNotImplemented) {
		code = "node_input_unsupported"
		message = fmt.Sprintf("node input type %q not implemented; skipping", input.Type)
	}
	return domain.Warning{
		Code:    code,
		Message: message,
		Field:   input.Name,
		Source:  refNameNode(input),
	}
}

func storeUnavailable() error {
	return domain.NewError(domain.CodeInvalidArgument, "store is not configured")
}

func cloneStringMap(in map[string]string) map[string]string {
	return maps.Clone(in)
}

func cloneNonEmptySlice[T any](values []T) []T {
	if len(values) == 0 {
		return nil
	}
	return slices.Clone(values)
}

func mergeStringMaps(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}
func sourcesSlice(s *domain.SourceInfo) []domain.SourceInfo {
	if s == nil {
		return nil
	}
	return []domain.SourceInfo{*s}
}

func normalizeFormat(format string) string {
	return strings.ToLower(strings.TrimSpace(format))
}

type renderPresentation struct {
	contentType string
	extension   string
}

var renderPresentations = map[string]renderPresentation{
	"base64":               {contentType: "text/plain; charset=utf-8", extension: ".txt"},
	"uri-list":             {contentType: "text/plain", extension: ".txt"},
	"mihomo-proxies":       {contentType: "application/yaml", extension: ".yaml"},
	"shadowrocket-proxies": {contentType: "application/yaml", extension: ".yaml"},
	"sing-box-outbounds":   {contentType: "application/json", extension: ".json"},
	"json-nodes":           {contentType: "application/json", extension: ".json"},
}

func renderPresentationFor(format string) (renderPresentation, bool) {
	presentation, ok := renderPresentations[normalizeFormat(format)]
	return presentation, ok
}

func contentTypeFor(format string) string {
	presentation, ok := renderPresentationFor(format)
	if !ok {
		return ""
	}
	return presentation.contentType
}
