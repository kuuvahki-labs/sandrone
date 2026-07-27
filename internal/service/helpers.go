package service

import (
	"fmt"
	"os"
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

func cloneFileResult(result *domain.FileResult) *domain.FileResult {
	if result == nil {
		return nil
	}
	out := *result
	out.File = cloneFileDocument(result.File)
	out.Content = append([]byte{}, result.Content...)
	out.Response = cloneResponseInfo(result.Response)
	out.Report = cloneReport(result.Report)
	return &out
}

func cloneRenderResult(result *domain.RenderResult) *domain.RenderResult {
	if result == nil {
		return nil
	}
	out := *result
	out.Body = append([]byte{}, result.Body...)
	out.Report = cloneReport(result.Report)
	return &out
}

func cloneFileDocument(doc domain.FileDocument) domain.FileDocument {
	out := doc
	out.Content = append([]byte{}, doc.Content...)
	out.Parts = cloneFileParts(doc.Parts)
	if doc.Meta != nil {
		out.Meta = cloneStringMap(doc.Meta)
	}
	out.Warnings = append([]domain.Warning{}, doc.Warnings...)
	return out
}

func cloneFileParts(parts []domain.FilePart) []domain.FilePart {
	if parts == nil {
		return nil
	}
	out := make([]domain.FilePart, len(parts))
	for i, part := range parts {
		out[i] = part
		out[i].Content = append([]byte{}, part.Content...)
		out[i].Nodes = append([]domain.NodeIR{}, part.Nodes...)
	}
	return out
}

func cloneResponseInfo(resp domain.ResponseInfo) domain.ResponseInfo {
	out := resp
	if resp.Headers != nil {
		out.Headers = cloneStringMap(resp.Headers)
	}
	return out
}

func cloneReport(report domain.Report) domain.Report {
	out := report
	out.Dependencies = append([]domain.ResourceRef{}, report.Dependencies...)
	out.SourceRefs = append([]domain.SourceRef{}, report.SourceRefs...)
	out.Warnings = append([]domain.Warning{}, report.Warnings...)
	out.Render.Warnings = append([]domain.Warning{}, report.Render.Warnings...)
	if report.Probe != nil {
		probeReport := *report.Probe
		if report.Probe.ErrorCounts != nil {
			probeReport.ErrorCounts = make(map[string]int, len(report.Probe.ErrorCounts))
			for code, count := range report.Probe.ErrorCounts {
				probeReport.ErrorCounts[code] = count
			}
		}
		out.Probe = &probeReport
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
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
	"shadowrocket-proxies": {contentType: "text/plain; charset=utf-8", extension: ".conf"},
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
