package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/fetcher"
)

var autoSubscriptionFormats = []string{"base64", "uri-list", "mihomo", "sing-box"}

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
	candidates := autoSubscriptionCandidateFormats(content)
	candidateErrors := make([]string, 0, len(candidates))
	for _, format := range candidates {
		parsed, err := s.parseNodeContentAutoCandidate(ctx, format, content, sourceRef)
		if err != nil {
			candidateErrors = append(candidateErrors, fmt.Sprintf("%s: %v", format, err))
			continue
		}
		if parsed == nil {
			candidateErrors = append(candidateErrors, fmt.Sprintf("%s: parser returned no result", format))
			continue
		}
		if len(parsed.Nodes) == 0 {
			candidateErrors = append(candidateErrors, fmt.Sprintf("%s: parsed zero nodes", format))
			continue
		}
		return parsed, nil
	}
	return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("could not detect subscription format: %s", strings.Join(candidateErrors, "; ")))
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
