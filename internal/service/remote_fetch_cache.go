package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/fetcher"
)

const (
	cacheKeyPrefixRemoteFetch = "remote_fetch"
	cacheKeyPrefixProbe       = "probe"
)

type remoteFetchCacheRecord struct {
	Body        []byte           `json:"body"`
	Headers     http.Header      `json:"headers,omitempty"`
	StatusCode  int              `json:"status_code,omitempty"`
	ContentHash string           `json:"content_hash,omitempty"`
	SourceRef   domain.SourceRef `json:"source_ref,omitempty"`
}

type remoteFetchCacheValue struct {
	Records map[string]remoteFetchCacheRecord `json:"records"`
}

func (s *Service) fetchRemoteCached(ctx context.Context, input domain.RemoteInput) (*remoteInputResult, error) {
	if strings.TrimSpace(input.URL) == "" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "remote input url is required")
	}
	if s.fetcher == nil {
		return nil, domain.NewError(domain.CodeNotImplemented, "remote fetcher is not configured")
	}
	input = s.remoteInputWithDefaults(input)
	ttl := time.Duration(input.CacheTTLSeconds) * time.Second
	cacheKey, cacheOwned := ownedCacheKey(ctx, cacheKeyPrefixRemoteFetch)
	cacheEntryID := ""
	if ttl > 0 && cacheOwned {
		var err error
		cacheEntryID, err = remoteFetchCacheEntryID(input)
		if err != nil {
			return nil, err
		}
		if !cacheReadBypass(ctx) {
			if cached := s.readRemoteFetchCache(ctx, cacheKey, cacheEntryID, ttl); cached != nil {
				s.log(ctx, slog.LevelInfo, "service remote fetch cache hit",
					"operation", "remote_fetch",
					"cache_key", cacheKey,
					"cache_hit", true,
					"cache_ttl_seconds", input.CacheTTLSeconds,
					"url", input.URL,
				)
				return cached, nil
			}
		}
	}
	result, err := s.fetcher.Fetch(ctx, fetcher.Request{
		URL:       input.URL,
		UserAgent: input.UserAgent,
		Proxy:     input.Proxy,
		TimeoutMS: input.TimeoutMS,
	})
	if err != nil {
		return nil, err
	}
	out := &remoteInputResult{
		SourceRef:   result.SourceRef,
		Body:        append([]byte{}, result.Body...),
		Headers:     result.Headers.Clone(),
		StatusCode:  result.StatusCode,
		ContentHash: result.ContentHash,
	}
	if ttl > 0 && cacheEntryID != "" {
		if err := s.writeRemoteFetchCache(ctx, cacheKey, cacheEntryID, ttl, out); err != nil {
			s.log(ctx, slog.LevelWarn, "service remote fetch cache write failed",
				"operation", "remote_fetch",
				"cache_key", cacheKey,
				"cache_hit", false,
				"cache_ttl_seconds", input.CacheTTLSeconds,
				"url", input.URL,
				"error", err.Error(),
			)
		}
	}
	return out, nil
}

func (s *Service) readRemoteFetchCache(ctx context.Context, key, entryID string, ttl time.Duration) *remoteInputResult {
	if s.cache == nil || entryID == "" {
		return nil
	}
	item, found := s.readCacheValue[remoteFetchCacheValue](ctx, key, ttl)
	if !found {
		return nil
	}
	record, found := item.Value.Records[entryID]
	if !found {
		return nil
	}
	ref := record.SourceRef
	ref.Note = appendSourceRefNote(ref.Note, "cache_hit=true")
	return &remoteInputResult{
		SourceRef:   ref,
		Body:        append([]byte{}, record.Body...),
		Headers:     record.Headers.Clone(),
		StatusCode:  record.StatusCode,
		ContentHash: record.ContentHash,
	}
}

func (s *Service) writeRemoteFetchCache(ctx context.Context, key, entryID string, ttl time.Duration, result *remoteInputResult) error {
	if s.cache == nil || entryID == "" || result == nil {
		return nil
	}
	value, remaining, ok := s.prepareCacheValueWrite[remoteFetchCacheValue](ctx, key, ttl)
	if !ok {
		return nil
	}
	if value.Records == nil {
		value.Records = map[string]remoteFetchCacheRecord{}
	}
	value.Records[entryID] = remoteFetchCacheRecord{
		Body:        append([]byte{}, result.Body...),
		Headers:     result.Headers.Clone(),
		StatusCode:  result.StatusCode,
		ContentHash: result.ContentHash,
		SourceRef:   result.SourceRef,
	}
	return cachepkg.SetJSON(ctx, s.cache, key, value, remaining)
}

func remoteFetchCacheEntryID(input domain.RemoteInput) (string, error) {
	return cacheIdentity(struct {
		URL       string `json:"url"`
		UserAgent string `json:"user_agent,omitempty"`
		Proxy     string `json:"proxy,omitempty"`
	}{
		URL:       input.URL,
		UserAgent: input.UserAgent,
		Proxy:     input.Proxy,
	})
}

func appendSourceRefNote(note string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return note
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return value
	}
	return fmt.Sprintf("%s %s", note, value)
}
