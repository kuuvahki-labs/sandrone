package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/fetcher"
)

const (
	cacheKeyPrefixRemoteFetch         = "remote_fetch"
	cacheKeyPrefixProbe               = "probe"
	cacheKeyPrefixSubscriptionTraffic = "subscription_traffic"
)

type cacheReadBypassContextKey struct{}

type cacheReadBypassState struct {
	mu          sync.Mutex
	initialized map[string]bool
}

type remoteFetchCacheRecord struct {
	Body        []byte           `json:"body"`
	Headers     http.Header      `json:"headers,omitempty"`
	StatusCode  int              `json:"status_code,omitempty"`
	ContentHash string           `json:"content_hash,omitempty"`
	SourceRef   domain.SourceRef `json:"source_ref,omitempty"`
}

func withCacheReadBypass(ctx context.Context) context.Context {
	if cacheReadBypass(ctx) {
		return ctx
	}
	return context.WithValue(ctx, cacheReadBypassContextKey{}, &cacheReadBypassState{initialized: map[string]bool{}})
}

func cacheReadBypass(ctx context.Context) bool {
	_, bypass := ctx.Value(cacheReadBypassContextKey{}).(*cacheReadBypassState)
	return bypass
}

func cacheRefreshStartsKey(ctx context.Context, key string) bool {
	state, bypass := ctx.Value(cacheReadBypassContextKey{}).(*cacheReadBypassState)
	if !bypass {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.initialized[key] {
		return false
	}
	state.initialized[key] = true
	return true
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
	cacheKey, cacheScoped := persistentCacheKey(ctx, cacheKeyPrefixRemoteFetch)
	cacheEntryID := ""
	if ttl > 0 && cacheScoped {
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
	c := s.cache
	if c == nil {
		return nil
	}
	var record remoteFetchCacheRecord
	if !s.readCacheJSON(ctx, key, entryID, ttl, &record) {
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
	c := s.cache
	if c == nil || result == nil {
		return nil
	}
	return s.writeCacheJSON(ctx, key, entryID, ttl, remoteFetchCacheRecord{
		Body:        append([]byte{}, result.Body...),
		Headers:     result.Headers.Clone(),
		StatusCode:  result.StatusCode,
		ContentHash: result.ContentHash,
		SourceRef:   result.SourceRef,
	})
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
