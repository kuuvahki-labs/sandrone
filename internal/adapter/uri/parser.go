package uri

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type schemeParser func(raw string) (domain.NodeIR, *domain.SourceInfo, error)
type multiSchemeParser func(raw string) ([]domain.NodeIR, *domain.SourceInfo, error)

type Parser struct {
	schemes      map[string]schemeParser
	multiSchemes map[string]multiSchemeParser
}

func NewParser() *Parser {
	p := &Parser{
		schemes:      map[string]schemeParser{},
		multiSchemes: map[string]multiSchemeParser{},
	}
	p.register("ss", parseSS)
	p.register("ssr", parseSSR)
	p.register("vmess", parseVMess)
	p.register("vmess1", parseVMess1)
	p.register("vless", parseVLESSCompat)
	p.register("trojan", parseTrojan)
	p.register("hysteria", parseHysteria)
	p.register("hy", parseHysteria)
	p.register("hysteria2", parseHysteria2)
	p.register("hy2", parseHysteria2)
	p.register("tuic", parseTUIC)
	p.register("anytls", parseAnyTLS)
	p.registerMulti("mierus", parseMieru)
	p.register("socks", parseSOCKS)
	p.register("socks5", parseSOCKS)
	p.register("tg", parseTelegramProxy)
	p.register("http", parseHTTP)
	p.register("https", parseHTTP)
	return p
}

func (p *Parser) register(scheme string, parser schemeParser) {
	p.schemes[scheme] = parser
}

func (p *Parser) registerMulti(scheme string, parser multiSchemeParser) {
	p.multiSchemes[scheme] = parser
}

func (p *Parser) Name() string {
	return "uri"
}

func (p *Parser) Parse(_ context.Context, in []byte) ([]domain.NodeIR, *domain.SourceInfo, error) {
	raw := strings.TrimSpace(string(in))
	if raw == "" {
		return nil, &domain.SourceInfo{Format: "uri"}, domain.NewError(domain.CodeParseFailed, "empty input")
	}
	return p.parseOne(raw)
}

func (p *Parser) ParseList(_ context.Context, in []byte) ([]domain.NodeIR, *domain.SourceInfo, error) {
	return p.parseList(in, false)
}

func (p *Parser) ParseStrictList(_ context.Context, in []byte) ([]domain.NodeIR, *domain.SourceInfo, error) {
	return p.parseList(in, true)
}

func (p *Parser) parseList(in []byte, strict bool) ([]domain.NodeIR, *domain.SourceInfo, error) {
	raw := strings.TrimSpace(string(in))
	if raw == "" {
		return nil, &domain.SourceInfo{Format: "uri-list"}, domain.NewError(domain.CodeParseFailed, "empty input")
	}
	decoded, ok := decodeSubscription(raw)
	if strict {
		decoded, ok = decodeStrictSubscription(raw)
	}
	if ok {
		raw = decoded
	}
	nodes := []domain.NodeIR{}
	info := &domain.SourceInfo{Format: "uri-list"}
	lines := strings.Split(raw, "\n")
	for i, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parsed, source := p.parseListLine(line, i+1, strict)
		nodeIndex := len(nodes)
		if source != nil {
			info.SourceRefs = append(info.SourceRefs, source.SourceRefs...)
			source.Warnings = append(source.Warnings, parseListUnknownWarnings(parsed, line, i+1, nodeIndex)...)
			info.Warnings = append(info.Warnings, source.Warnings...)
		}
		nodes = append(nodes, parsed...)
	}
	if len(nodes) == 0 {
		return nil, info, domain.NewError(domain.CodeParseFailed, noListNodesMessage(info.Warnings))
	}
	return nodes, info, nil
}

func (p *Parser) parseListLine(line string, lineNumber int, strict bool) ([]domain.NodeIR, *domain.SourceInfo) {
	parsed, source, uriErr := p.parseOneRaw(line)
	if uriErr == nil {
		return parsed, source
	}
	if skipListLineError(uriErr) {
		if source == nil {
			source = &domain.SourceInfo{Format: "uri-list"}
		}
		source.Warnings = append(source.Warnings, skippedListLineWarning(line, lineNumber))
		return nil, source
	}
	if strict {
		if source == nil {
			source = &domain.SourceInfo{Format: "uri-list"}
		}
		err := domain.NewError(domain.CodeParseFailed, fmt.Sprintf("line %d (%s): %s; JSON/YAML node lines are not allowed in strict uri-list", lineNumber, safeListLineScheme(line), safeListLineCause(uriErr)))
		source.Warnings = append(source.Warnings, failedListLineWarning(line, lineNumber, err))
		return nil, source
	}
	parsed, jsonErr := parseJSONNodeLine(line)
	if jsonErr == nil {
		return parsed, nil
	}
	parsed, yamlErr := parseYAMLNodeLine(line)
	if yamlErr == nil {
		return parsed, nil
	}
	if source == nil {
		source = &domain.SourceInfo{Format: "uri-list"}
	}
	err := domain.NewError(domain.CodeParseFailed, fmt.Sprintf("line %d (%s): %s; JSON/YAML node fallback also failed", lineNumber, safeListLineScheme(line), safeListLineCause(uriErr)))
	source.Warnings = append(source.Warnings, failedListLineWarning(line, lineNumber, err))
	return nil, source
}

func safeListLineScheme(line string) string {
	scheme, _, ok := strings.Cut(line, "://")
	if !ok || scheme == "" || len(scheme) > 32 {
		return "unknown scheme"
	}
	for _, value := range scheme {
		if (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
			(value >= '0' && value <= '9') || value == '+' || value == '-' || value == '.' {
			continue
		}
		return "unknown scheme"
	}
	return "scheme " + strings.ToLower(scheme)
}

func safeListLineCause(err error) string {
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || strings.TrimSpace(appErr.Message) == "" {
		return "URI parse failed"
	}
	message := strings.Map(func(value rune) rune {
		if value < ' ' || value == '\u007f' {
			return ' '
		}
		return value
	}, strings.TrimSpace(appErr.Message))
	const maxCauseLength = 160
	if len(message) > maxCauseLength {
		message = message[:maxCauseLength] + "..."
	}
	return message
}

func skipListLineError(err error) bool {
	return errors.Is(err, errVMessZeroPort)
}

func skippedListLineWarning(rawLine string, lineNumber int) domain.Warning {
	return domain.Warning{
		Code:    "parse_line_skipped",
		Message: "skipped vmess URI with zero port",
		Field:   "port",
		Source:  "uri-list",
		NodeContext: &domain.WarningNodeContext{
			Format:  "uri-list",
			Type:    domain.NodeTypeVMess,
			RawLine: rawLine,
			Line:    lineNumber,
		},
	}
}

func failedListLineWarning(rawLine string, lineNumber int, err error) domain.Warning {
	return domain.Warning{
		Code:    "parse_line_failed",
		Message: err.Error(),
		Source:  "uri-list",
		NodeContext: &domain.WarningNodeContext{
			Format:  "uri-list",
			RawLine: rawLine,
			Line:    lineNumber,
		},
	}
}

func noListNodesMessage(warnings []domain.Warning) string {
	for _, warning := range warnings {
		if warning.Code == "parse_line_failed" && warning.Message != "" {
			return "no nodes found: " + warning.Message
		}
	}
	return "no nodes found"
}

func (p *Parser) parseOne(raw string) ([]domain.NodeIR, *domain.SourceInfo, error) {
	parsed, source, err := p.parseOneRaw(raw)
	if err != nil {
		return nil, source, err
	}
	if source != nil {
		source.Warnings = append(source.Warnings, uriUnknownWarnings(parsed, raw, 0, 0, "uri")...)
	}
	return parsed, source, nil
}

func (p *Parser) parseOneRaw(raw string) ([]domain.NodeIR, *domain.SourceInfo, error) {
	scheme, _, ok := strings.Cut(raw, "://")
	if !ok || scheme == "" {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, &domain.SourceInfo{Format: "uri"}, domain.WrapError(domain.CodeParseFailed, "parse URI", err)
		}
		scheme = u.Scheme
	}
	normalizedScheme := strings.ToLower(scheme)
	if parser, ok := p.multiSchemes[normalizedScheme]; ok {
		return parser(raw)
	}
	parser, ok := p.schemes[normalizedScheme]
	if !ok {
		return nil, &domain.SourceInfo{Format: "uri"}, domain.NewError(domain.CodeParseFailed, "unsupported URI scheme")
	}
	node, source, err := parser(raw)
	if err != nil {
		return nil, source, err
	}
	return []domain.NodeIR{node}, source, nil
}

func parseListUnknownWarnings(nodes []domain.NodeIR, rawLine string, line, nodeIndex int) []domain.Warning {
	return uriUnknownWarnings(nodes, rawLine, line, nodeIndex, "uri-list")
}

func uriUnknownWarnings(nodes []domain.NodeIR, rawLine string, line, nodeIndex int, format string) []domain.Warning {
	warnings := []domain.Warning{}
	for i, node := range nodes {
		if len(node.Raw) == 0 {
			continue
		}
		index := nodeIndex + i
		context := domain.WarningNodeContext{
			Format:  format,
			Name:    node.Name,
			Type:    node.Type,
			Server:  node.Server,
			Port:    node.Port,
			RawLine: rawLine,
			Line:    line,
		}
		warnings = append(warnings, shared.ParseUnknownWarningsWithContext(node, node.Raw, format, &index, &context)...)
	}
	return warnings
}

func parseJSONNodeLine(line string) ([]domain.NodeIR, error) {
	var container struct {
		Nodes []domain.NodeIR `json:"nodes"`
	}
	if err := decodeJSONLine(line, &container); err == nil && container.Nodes != nil {
		return normalizeLineNodes(container.Nodes, "json-nodes")
	}
	var node domain.NodeIR
	if err := decodeJSONLine(line, &node); err != nil {
		return nil, err
	}
	return normalizeLineNodes([]domain.NodeIR{node}, "json-nodes")
}

func parseYAMLNodeLine(line string) ([]domain.NodeIR, error) {
	var container struct {
		Nodes []domain.NodeIR `yaml:"nodes"`
	}
	if err := yaml.Unmarshal([]byte(line), &container); err == nil && container.Nodes != nil {
		return normalizeLineNodes(container.Nodes, "yaml-node")
	}
	var node domain.NodeIR
	if err := yaml.Unmarshal([]byte(line), &node); err != nil {
		return nil, err
	}
	return normalizeLineNodes([]domain.NodeIR{node}, "yaml-node")
}

func decodeJSONLine(line string, target any) error {
	decoder := json.NewDecoder(bytes.NewReader([]byte(line)))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return err
		}
		return domain.NewError(domain.CodeParseFailed, "json line contains trailing values")
	}
	return nil
}

func normalizeLineNodes(nodes []domain.NodeIR, sourceFormat string) ([]domain.NodeIR, error) {
	if len(nodes) == 0 {
		return nil, domain.NewError(domain.CodeParseFailed, "nodes container must contain at least one node")
	}
	for i := range nodes {
		if nodes[i].Type == "" {
			return nil, domain.NewError(domain.CodeParseFailed, "missing node type")
		}
		if nodes[i].SourceFormat == "" {
			nodes[i].SourceFormat = sourceFormat
		}
		nodes[i].Warnings = append(nodes[i].Warnings, shared.NormalizeLegacyHysteriaBandwidth(&nodes[i])...)
	}
	return nodes, nil
}

var base64Decoders = []*base64.Encoding{
	base64.StdEncoding,
	base64.RawStdEncoding,
	base64.URLEncoding,
	base64.RawURLEncoding,
}

func decodeSubscription(raw string) (string, bool) {
	return decodeSubscriptionMatching(raw, looksLikeSubscriptionContent)
}

func decodeStrictSubscription(raw string) (string, bool) {
	return decodeSubscriptionMatching(raw, looksLikeStrictSubscriptionContent)
}

func decodeSubscriptionMatching(raw string, looksLike func(string) bool) (string, bool) {
	candidate := strings.TrimSpace(raw)
	if strings.Contains(candidate, "://") || strings.Contains(candidate, "\n") {
		return "", false
	}
	decoded, ok := decodeBase64String(candidate)
	if !ok || !looksLike(decoded) {
		return "", false
	}
	return decoded, true
}

func decodeBase64String(s string) (string, bool) {
	decoded, ok := decodeBase64Bytes(s)
	if !ok || !utf8.Valid(decoded) {
		return "", false
	}
	return string(decoded), true
}

func decodeBase64Bytes(s string) ([]byte, bool) {
	for _, decoder := range base64Decoders {
		decoded, err := decoder.DecodeString(s)
		if err == nil {
			return decoded, true
		}
	}
	return nil, false
}

func looksLikeSubscriptionContent(decoded string) bool {
	for _, line := range strings.Split(decoded, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return strings.Contains(line, "://") ||
			strings.HasPrefix(line, "{") ||
			strings.HasPrefix(line, "nodes:") ||
			strings.Contains(line, "type:")
	}
	return false
}

func looksLikeStrictSubscriptionContent(decoded string) bool {
	for _, line := range strings.Split(decoded, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return strings.Contains(line, "://")
	}
	return false
}
