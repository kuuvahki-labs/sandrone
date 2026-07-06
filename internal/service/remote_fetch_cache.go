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
	cacheLayerRemoteFetch         = "remote_fetch"
	cacheLayerProbe               = "probe"
	cacheLayerSubscriptionTraffic = "subscription_traffic"
)

type remoteFetchBypassContextKey struct{}

type remoteFetchCacheRecord struct {
	Body        []byte           `json:"body"`
	Headers     http.Header      `json:"headers,omitempty"`
	StatusCode  int              `json:"status_code,omitempty"`
	ContentHash string           `json:"content_hash,omitempty"`
	SourceRef   domain.SourceRef `json:"source_ref,omitempty"`
}

func withRemoteFetchCacheBypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, remoteFetchBypassContextKey{}, true)
}

func remoteFetchCacheBypass(ctx context.Context) bool {
	bypass, _ := ctx.Value(remoteFetchBypassContextKey{}).(bool)
	return bypass
}

func (s *Service) cache() *cachepkg.Cache {
	if s.store == nil {
		return nil
	}
	return cachepkg.New(s.store, s.now)
}

func (s *Service) fetchRemoteCached(ctx context.Context, input domain.RemoteInput) (*remoteInputResult, error) {
	if strings.TrimSpace(input.URL) == "" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "remote input url is required")
	}
	if s.fetcher == nil {
		return nil, domain.NewError(domain.CodeNotImplemented, "remote fetcher is not configured")
	}
	input, err := s.remoteInputWithDefaults(ctx, input)
	if err != nil {
		return nil, err
	}
	ttl := time.Duration(input.CacheTTLSeconds) * time.Second
	cacheKey := ""
	if ttl > 0 {
		cacheKey, err = remoteFetchCacheKey(input)
		if err != nil {
			return nil, err
		}
		if !remoteFetchCacheBypass(ctx) {
			if cached := s.readRemoteFetchCache(ctx, cacheKey, ttl); cached != nil {
				s.log(ctx, slog.LevelInfo, "service remote fetch cache hit",
					"operation", "remote_fetch",
					"cache_layer", cacheLayerRemoteFetch,
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
	if ttl > 0 && cacheKey != "" {
		if err := s.writeRemoteFetchCache(ctx, cacheKey, out); err != nil {
			s.log(ctx, slog.LevelWarn, "service remote fetch cache write failed",
				"operation", "remote_fetch",
				"cache_layer", cacheLayerRemoteFetch,
				"cache_hit", false,
				"cache_ttl_seconds", input.CacheTTLSeconds,
				"url", input.URL,
				"error", err.Error(),
			)
		}
	}
	return out, nil
}

func (s *Service) readRemoteFetchCache(ctx context.Context, key string, ttl time.Duration) *remoteInputResult {
	c := s.cache()
	if c == nil {
		return nil
	}
	var record remoteFetchCacheRecord
	if !c.GetJSON(ctx, key, ttl, &record) {
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

func (s *Service) writeRemoteFetchCache(ctx context.Context, key string, result *remoteInputResult) error {
	c := s.cache()
	if c == nil || result == nil {
		return nil
	}
	return c.PutJSON(ctx, key, remoteFetchCacheRecord{
		Body:        append([]byte{}, result.Body...),
		Headers:     result.Headers.Clone(),
		StatusCode:  result.StatusCode,
		ContentHash: result.ContentHash,
		SourceRef:   result.SourceRef,
	})
}

func remoteFetchCacheKey(input domain.RemoteInput) (string, error) {
	return cachepkg.HashKey(cacheLayerRemoteFetch, struct {
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
