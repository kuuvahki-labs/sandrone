// Package shadowrocket renders Sandrone node IR as Shadowrocket local proxy lines.
package shadowrocket

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const (
	rendererName       = "shadowrocket-proxies"
	ProxySectionHeader = "[Proxy]"
)

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

// PreviewNodeNames returns each input node's final rendered name. An empty
// entry marks a node that the renderer would skip.
func PreviewNodeNames(nodes []domain.NodeIR) []string {
	names := make([]string, len(nodes))
	usedNames := map[string]bool{}
	for index, node := range nodes {
		if _, _, _, err := renderNode(node); err != nil {
			continue
		}
		names[index] = uniqueNodeName(node.Name, index, usedNames)
	}
	return names
}

func (r *Renderer) Name() string {
	return rendererName
}

func (r *Renderer) Render(ctx context.Context, nodes []domain.NodeIR, options domain.RenderOptions) ([]byte, error) {
	out, _, err := r.RenderWithReport(ctx, nodes, options)
	return out, err
}

func (r *Renderer) RenderWithReport(_ context.Context, nodes []domain.NodeIR, _ domain.RenderOptions) ([]byte, domain.RenderReport, error) {
	var body strings.Builder
	body.WriteString(ProxySectionHeader)
	body.WriteByte('\n')
	report := domain.RenderReport{}
	usedNames := map[string]bool{}

	for index, node := range nodes {
		parts, emitted, warnings, err := renderNode(node)
		if err != nil {
			shared.MergeWarnings(&report, []domain.Warning{shared.RenderNodeSkippedWarning(node, r.Name(), err)})
			continue
		}

		name := uniqueNodeName(node.Name, index, usedNames)
		warnings = append(warnings, structuredLossWarnings(node, emitted)...)
		if name != node.Name {
			warnings = append(warnings, shared.RenderLossyWarning(
				node,
				r.Name(),
				"name",
				"node name normalized for Shadowrocket local proxy syntax",
			))
		}
		warnings = append(warnings, shared.RawWarnings(node, nil, r.Name())...)
		shared.MergeWarnings(&report, warnings)

		body.WriteString(name)
		body.WriteString(" = ")
		body.WriteString(strings.Join(parts, ", "))
		body.WriteByte('\n')
		report.SuccessCount++
	}

	if len(nodes) > 0 && report.SuccessCount == 0 {
		return nil, report, shared.NoRenderableNodesError(report)
	}
	return []byte(body.String()), report, nil
}

type emittedFields map[string]bool

func renderNode(node domain.NodeIR) ([]string, emittedFields, []domain.Warning, error) {
	switch node.Type {
	case domain.NodeTypeShadowsocks:
		return renderShadowsocks(node)
	case domain.NodeTypeVMess:
		return renderVMess(node)
	case domain.NodeTypeVLESS:
		return renderVLESS(node)
	case domain.NodeTypeTrojan:
		return renderTrojan(node)
	case domain.NodeTypeHysteria:
		return renderHysteria(node)
	case domain.NodeTypeHysteria2:
		return renderHysteria2(node)
	case domain.NodeTypeTUIC:
		return renderTUIC(node)
	case domain.NodeTypeHTTP:
		return renderHTTP(node)
	case domain.NodeTypeSOCKS:
		return renderSOCKS(node)
	case domain.NodeTypeWireGuard:
		return renderWireGuard(node)
	case domain.NodeTypeSnell:
		return renderSnell(node)
	default:
		return nil, nil, nil, domain.NewError(domain.CodeRenderFailed, "node type is not documented for Shadowrocket local proxy output")
	}
}

func baseParts(protocol, server string, port uint16) ([]string, error) {
	if err := requiredScalar("server", server); err != nil {
		return nil, err
	}
	if port == 0 {
		return nil, domain.NewError(domain.CodeRenderFailed, "port is required")
	}
	return []string{protocol, server, strconv.Itoa(int(port))}, nil
}

func appendRequiredField(parts []string, key, field, value string) ([]string, error) {
	if err := requiredScalar(field, value); err != nil {
		return nil, err
	}
	return append(parts, key+"="+value), nil
}

func appendOptionalField(parts []string, key, field, value string) ([]string, error) {
	if value == "" {
		return parts, nil
	}
	if err := safeScalar(field, value); err != nil {
		return nil, err
	}
	return append(parts, key+"="+value), nil
}

func requiredScalar(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return domain.NewError(domain.CodeRenderFailed, field+" is required")
	}
	return safeScalar(field, value)
}

func safeScalar(field, value string) error {
	if !utf8.ValidString(value) {
		return domain.NewError(domain.CodeRenderFailed, field+" is not valid UTF-8")
	}
	if strings.ContainsAny(value, "\r\n,") {
		return domain.NewError(domain.CodeRenderFailed, field+" contains an unsupported delimiter")
	}
	return nil
}

func uniqueNodeName(original string, inputIndex int, used map[string]bool) string {
	base := normalizeNodeName(original)
	if base == "" {
		base = "node-" + strconv.Itoa(inputIndex+1)
	}
	name := base
	for suffix := 2; used[name]; suffix++ {
		name = fmt.Sprintf("%s (%d)", base, suffix)
	}
	used[name] = true
	return name
}

func normalizeNodeName(name string) string {
	var normalized strings.Builder
	inLineBreak := false
	for _, char := range name {
		if char == '\r' || char == '\n' {
			if !inLineBreak {
				normalized.WriteByte(' ')
			}
			inLineBreak = true
			continue
		}
		inLineBreak = false
		switch char {
		case ',':
			normalized.WriteRune('，')
		case '=':
			normalized.WriteRune('＝')
		default:
			normalized.WriteRune(char)
		}
	}
	result := strings.TrimSpace(normalized.String())
	switch {
	case strings.HasPrefix(result, "#"):
		result = "＃" + strings.TrimPrefix(result, "#")
	case strings.HasPrefix(result, ";"):
		result = "；" + strings.TrimPrefix(result, ";")
	case strings.HasPrefix(result, "["):
		result = "［" + strings.TrimPrefix(result, "[")
	}
	if ConflictsWithBuiltinRulePolicy(result) {
		result += " (Node)"
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
