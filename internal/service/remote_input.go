package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/fetcher"
)

var autoSubscriptionFormats = []string{"uri-list", "mihomo", "sing-box"}

type publicRemoteFetchContextKey struct{}

type parseInputResult struct {
	Nodes  []domain.NodeIR
	Source *domain.SourceInfo
}

type remoteInputResult struct {
	SourceRef   domain.SourceRef
	Body        []byte
	Headers     http.Header
	StatusCode  int
	ContentHash string
}

func (s *Service) parseRequestInput(ctx context.Context, req domain.ParseRequest) (*parseInputResult, error) {
	if req.Remote != nil {
		if len(req.Content) > 0 {
			return nil, domain.NewError(domain.CodeInvalidArgument, "content and remote input are mutually exclusive")
		}
		remoteInput, err := s.fetchRemoteInput(ctx, *req.Remote)
		if err != nil {
			return nil, err
		}
		return s.parseNodeContent(ctx, req.Format, remoteInput.Body, true, &remoteInput.SourceRef)
	}
	return s.parseNodeContent(ctx, req.Format, req.Content, false, nil)
}

func (s *Service) fetchRemoteInput(ctx context.Context, input domain.RemoteInput) (*remoteInputResult, error) {
	if publicRemoteFetch(ctx) {
		return s.fetchPublicRemoteInput(ctx, input)
	}
	result, err := s.fetchRemoteCached(ctx, input)
	if err != nil {
		return nil, err
	}
	ref := result.SourceRef
	ref.Name = sanitizedTrafficSourceURL(ref.Name)
	ref.URL = sanitizedTrafficSourceURL(ref.URL)
	return &remoteInputResult{
		SourceRef:   ref,
		Body:        append([]byte{}, result.Body...),
		Headers:     result.Headers.Clone(),
		StatusCode:  result.StatusCode,
		ContentHash: result.ContentHash,
	}, nil
}

func withPublicRemoteFetch(ctx context.Context) context.Context {
	return context.WithValue(ctx, publicRemoteFetchContextKey{}, true)
}

func publicRemoteFetch(ctx context.Context) bool {
	enabled, _ := ctx.Value(publicRemoteFetchContextKey{}).(bool)
	return enabled
}

func (s *Service) fetchPublicRemoteInput(ctx context.Context, input domain.RemoteInput) (*remoteInputResult, error) {
	if s.fetcher == nil {
		return nil, domain.NewError(domain.CodeNotImplemented, "remote fetcher is not configured")
	}
	result, err := s.fetcher.FetchPublic(ctx, fetcher.Request{URL: input.URL})
	if err != nil {
		return nil, err
	}
	ref := result.SourceRef
	ref.Name = sanitizedTrafficSourceURL(ref.Name)
	ref.URL = sanitizedTrafficSourceURL(ref.URL)
	return &remoteInputResult{
		SourceRef:   ref,
		Body:        append([]byte{}, result.Body...),
		Headers:     result.Headers.Clone(),
		StatusCode:  result.StatusCode,
		ContentHash: result.ContentHash,
	}, nil
}

func (s *Service) parseNodeContent(ctx context.Context, format string, content []byte, allowAuto bool, sourceRef *domain.SourceRef) (*parseInputResult, error) {
	var (
		parsed *parseInputResult
		err    error
	)
	if !isAutoNodeFormat(format) {
		parsed, err = s.parseNodeContentExplicit(ctx, format, content, sourceRef)
	} else {
		if !allowAuto {
			return nil, domain.NewError(domain.CodeInvalidArgument, "node input format is required")
		}
		parsed, err = s.parseNodeContentAuto(ctx, content, sourceRef)
	}
	if err != nil {
		return nil, err
	}
	normalizeParsedNodes(parsed)
	return parsed, nil
}

func (s *Service) parseNodeContentExplicit(ctx context.Context, format string, content []byte, sourceRef *domain.SourceRef) (*parseInputResult, error) {
	parser, ok := s.parsers[normalizeFormat(format)]
	if !ok {
		return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported parse format %q", format))
	}
	nodes, source, err := s.invokeParser(ctx, parser, format, content)
	if err != nil {
		return nil, err
	}
	return &parseInputResult{Nodes: nodes, Source: withInputSourceRef(source, format, sourceRef)}, nil
}

func (s *Service) parseNodeContentAuto(ctx context.Context, content []byte, sourceRef *domain.SourceRef) (*parseInputResult, error) {
	contentCandidates := autoSubscriptionContentCandidates(content)
	candidateErrors := make([]string, 0, len(contentCandidates)*len(autoSubscriptionFormats))
	for _, contentCandidate := range contentCandidates {
		formats := autoSubscriptionCandidateFormats(contentCandidate.content)
		for _, format := range formats {
			parsed, err := s.parseNodeContentAutoCandidate(ctx, format, contentCandidate.content, sourceRef)
			if err != nil || parsed == nil || len(parsed.Nodes) == 0 {
				candidateErrors = append(candidateErrors, fmt.Sprintf("%s/%s: rejected", contentCandidate.name, format))
				continue
			}
			return parsed, nil
		}
	}
	return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("could not detect subscription format: %s", strings.Join(candidateErrors, "; ")))
}

type autoSubscriptionContentCandidate struct {
	name    string
	content []byte
}

func autoSubscriptionContentCandidates(content []byte) []autoSubscriptionContentCandidate {
	candidates := make([]autoSubscriptionContentCandidate, 0, 4)
	add := func(name string, candidate []byte) {
		candidate = bytes.TrimSpace(candidate)
		if len(candidate) == 0 || !utf8.Valid(candidate) {
			return
		}
		for _, existing := range candidates {
			if bytes.Equal(existing.content, candidate) {
				return
			}
		}
		candidates = append(candidates, autoSubscriptionContentCandidate{
			name:    name,
			content: append([]byte{}, candidate...),
		})
	}

	add("raw", content)
	unescaped, unescapeErr := url.PathUnescape(string(content))
	if unescapeErr == nil {
		add("percent", []byte(unescaped))
	}
	if decoded, ok := decodeAutoSubscriptionBase64(content); ok {
		add("base64", decoded)
	}
	if unescapeErr == nil {
		if decoded, ok := decodeAutoSubscriptionBase64([]byte(unescaped)); ok {
			add("percent+base64", decoded)
		}
	}
	return candidates
}

func decodeAutoSubscriptionBase64(content []byte) ([]byte, bool) {
	compact := make([]byte, 0, len(content))
	for _, value := range content {
		switch value {
		case ' ', '\t', '\r', '\n', '\v', '\f':
			continue
		default:
			compact = append(compact, value)
		}
	}
	if len(compact) == 0 {
		return nil, false
	}
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding.Strict(),
		base64.RawStdEncoding.Strict(),
		base64.URLEncoding.Strict(),
		base64.RawURLEncoding.Strict(),
	} {
		decoded, err := encoding.DecodeString(string(compact))
		if err == nil && len(decoded) > 0 && utf8.Valid(decoded) && looksLikeAutoSubscriptionContent(decoded) {
			return decoded, true
		}
	}
	return nil, false
}

func looksLikeAutoSubscriptionContent(content []byte) bool {
	if len(detectStructuredSubscriptionFormats(content)) > 0 {
		return true
	}
	return looksLikeAutoURIList(content)
}

func looksLikeAutoURIList(content []byte) bool {
	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return strings.Contains(line, "://")
	}
	return false
}

func (s *Service) parseNodeContentAutoCandidate(ctx context.Context, format string, content []byte, sourceRef *domain.SourceRef) (*parseInputResult, error) {
	switch normalizeFormat(format) {
	case "base64", "uri-list":
		nodes, source, err := s.uriParser.ParseStrictList(ctx, content)
		if err != nil {
			return nil, err
		}
		return &parseInputResult{Nodes: nodes, Source: withInputSourceRef(source, format, sourceRef)}, nil
	default:
		return s.parseNodeContentExplicit(ctx, format, content, sourceRef)
	}
}

func autoSubscriptionCandidateFormats(content []byte) []string {
	candidates := make([]string, 0, len(autoSubscriptionFormats)+2)
	add := func(format string) {
		format = normalizeFormat(format)
		if format == "" {
			return
		}
		for _, candidate := range candidates {
			if candidate == format {
				return
			}
		}
		candidates = append(candidates, format)
	}
	for _, format := range detectStructuredSubscriptionFormats(content) {
		add(format)
	}
	for _, format := range autoSubscriptionFormats {
		if format == "uri-list" && !looksLikeAutoURIList(content) {
			continue
		}
		add(format)
	}
	return candidates
}

func detectStructuredSubscriptionFormats(content []byte) []string {
	doc, ok := decodeTopLevelDocument(content)
	if !ok {
		return nil
	}
	formats := []string{}
	if hasAnyTopLevelKey(doc, "outbounds", "endpoints") {
		formats = append(formats, "sing-box")
	}
	if hasAnyTopLevelKey(doc, "proxies") {
		formats = append(formats, "mihomo")
	}
	return formats
}

func decodeTopLevelDocument(content []byte) (map[string]any, bool) {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return nil, false
	}
	if trimmed[0] == '{' {
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		decoder.UseNumber()
		var doc map[string]any
		if err := decoder.Decode(&doc); err == nil {
			return doc, true
		}
	}
	var doc map[string]any
	if err := yaml.Unmarshal(trimmed, &doc); err != nil {
		return nil, false
	}
	if len(doc) == 0 {
		return nil, false
	}
	return doc, true
}

func hasAnyTopLevelKey(doc map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := doc[key]; ok {
			return true
		}
	}
	return false
}

func isAutoNodeFormat(format string) bool {
	switch normalizeFormat(format) {
	case "", "auto":
		return true
	default:
		return false
	}
}

func withInputSourceRef(source *domain.SourceInfo, format string, sourceRef *domain.SourceRef) *domain.SourceInfo {
	if source == nil && sourceRef == nil {
		return nil
	}
	cloned := &domain.SourceInfo{}
	if source != nil {
		cloned.Format = source.Format
		cloned.SourceRefs = append(cloned.SourceRefs, source.SourceRefs...)
		cloned.Warnings = append(cloned.Warnings, source.Warnings...)
	}
	if strings.TrimSpace(cloned.Format) == "" && strings.TrimSpace(format) != "" {
		cloned.Format = normalizeFormat(format)
	}
	if sourceRef != nil {
		cloned.SourceRefs = append([]domain.SourceRef{*sourceRef}, cloned.SourceRefs...)
	}
	return cloned
}
