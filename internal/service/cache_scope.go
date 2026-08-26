package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"path"
	"strings"
	"time"
)

const (
	cacheResourceSubscriptions = "subscriptions"
	cacheResourceFiles         = "files"
)

type persistentCacheScopeContextKey struct{}

type persistentCacheScope struct {
	ResourceKind string
	ResourceName string
}

type cacheDocument struct {
	Entries map[string]json.RawMessage `json:"entries"`
}

type cacheDocumentUpdate struct {
	ID    string
	Value any
}

func withSubscriptionCacheScope(ctx context.Context, name string) context.Context {
	return withPersistentCacheScope(ctx, cacheResourceSubscriptions, name)
}

func withFileCacheScope(ctx context.Context, name string) context.Context {
	return withPersistentCacheScope(ctx, cacheResourceFiles, name)
}

func withPersistentCacheScope(ctx context.Context, kind, name string) context.Context {
	scope := persistentCacheScope{ResourceKind: strings.TrimSpace(kind), ResourceName: name}
	if strings.TrimSpace(scope.ResourceName) == "" {
		return ctx
	}
	return context.WithValue(ctx, persistentCacheScopeContextKey{}, scope)
}

func persistentCacheKey(ctx context.Context, prefix string) (string, bool) {
	scope, ok := ctx.Value(persistentCacheScopeContextKey{}).(persistentCacheScope)
	if !ok || scope.ResourceKind == "" || scope.ResourceName == "" || strings.TrimSpace(prefix) == "" {
		return "", false
	}
	return path.Join(prefix, scope.ResourceKind, scope.ResourceName), true
}

func withProbeInputCacheScope(ctx context.Context, inputKind, refKind, refName string) context.Context {
	if _, ok := ctx.Value(persistentCacheScopeContextKey{}).(persistentCacheScope); ok {
		return ctx
	}
	inputKind = strings.ToLower(strings.TrimSpace(inputKind))
	refKind = strings.ToLower(strings.TrimSpace(refKind))
	if (inputKind == "subscription" || (inputKind == "ref" && refKind == "subscription")) && strings.TrimSpace(refName) != "" {
		return withSubscriptionCacheScope(ctx, refName)
	}
	return ctx
}

func (s *Service) readCacheEntries(ctx context.Context, key string, entryIDs []string, ttl time.Duration) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	if s.cache == nil || key == "" || len(entryIDs) == 0 {
		return out
	}
	item, found, err := s.cache.Get(ctx, key)
	if err != nil {
		s.log(ctx, slog.LevelWarn, "service cache read failed", "cache_key", key, "error", err.Error())
		return out
	}
	if !found {
		return out
	}
	now := s.now().UTC()
	if deadline := now.Add(ttl); ttl > 0 && deadline.Before(item.ExpiresAt) {
		if err := s.cache.Set(ctx, key, item.Value, deadline.Sub(now)); err != nil {
			s.log(ctx, slog.LevelWarn, "service cache deadline update failed", "cache_key", key, "error", err.Error())
		}
	}
	var document cacheDocument
	if json.Unmarshal(item.Value, &document) != nil || len(document.Entries) == 0 {
		return out
	}
	for _, entryID := range entryIDs {
		if value, exists := document.Entries[entryID]; exists && len(value) > 0 {
			out[entryID] = append(json.RawMessage(nil), value...)
		}
	}
	return out
}

func (s *Service) readCacheJSON(ctx context.Context, key, entryID string, ttl time.Duration, out any) bool {
	if entryID == "" {
		return false
	}
	values := s.readCacheEntries(ctx, key, []string{entryID}, ttl)
	body, ok := values[entryID]
	return ok && json.Unmarshal(body, out) == nil
}

// writeCacheEntries merges opaque business entries into one cache value. A
// normal update may preserve or shorten the existing absolute deadline but
// never extend it. A refresh bypass replaces the value and starts a new TTL.
func (s *Service) writeCacheEntries(ctx context.Context, key string, ttl time.Duration, updates []cacheDocumentUpdate) error {
	if s.cache == nil || key == "" || ttl <= 0 || len(updates) == 0 {
		return nil
	}
	now := s.now().UTC()
	deadline := now.Add(ttl)
	document := cacheDocument{Entries: map[string]json.RawMessage{}}
	if !cacheRefreshStartsKey(ctx, key) {
		item, found, err := s.cache.Get(ctx, key)
		if err == nil && found && item.ExpiresAt.After(now) {
			if json.Unmarshal(item.Value, &document) != nil || document.Entries == nil {
				document.Entries = map[string]json.RawMessage{}
			}
			if item.ExpiresAt.Before(deadline) {
				deadline = item.ExpiresAt
			}
		}
	}
	for _, update := range updates {
		entryID := strings.TrimSpace(update.ID)
		if entryID == "" {
			continue
		}
		body, err := json.Marshal(update.Value)
		if err != nil {
			return err
		}
		document.Entries[entryID] = body
	}
	if len(document.Entries) == 0 {
		return nil
	}
	body, err := json.Marshal(document)
	if err != nil {
		return err
	}
	remaining := deadline.Sub(now)
	if remaining <= 0 {
		return s.cache.Delete(ctx, key)
	}
	return s.cache.Set(ctx, key, body, remaining)
}

func (s *Service) writeCacheJSON(ctx context.Context, key, entryID string, ttl time.Duration, value any) error {
	return s.writeCacheEntries(ctx, key, ttl, []cacheDocumentUpdate{{ID: entryID, Value: value}})
}

func cacheIdentity(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
