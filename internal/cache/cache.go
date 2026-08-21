// Package cache provides backend-independent JSON TTL cache primitives for
// internal service acceleration.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/store"
)

type Cache interface {
	GetJSON(ctx context.Context, key string, out any) bool
	PutJSON(ctx context.Context, key string, ttl time.Duration, value any) error
	DeleteLayer(ctx context.Context, layer string) error
}

type storeCache struct {
	store store.Store
	now   func() time.Time
}

type envelope struct {
	StoredAt  time.Time       `json:"stored_at"`
	ExpiresAt time.Time       `json:"expires_at"`
	Value     json.RawMessage `json:"value"`
}

func New(st store.Store, now func() time.Time) Cache {
	if now == nil {
		now = time.Now
	}
	return &storeCache{store: st, now: now}
}

func HashKey(layer string, identity any) (string, error) {
	body, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return LayerKey(layer, hex.EncodeToString(sum[:])+".json")
}

func LayerKey(layer string, name string) (string, error) {
	layer, err := cleanLayer(layer)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", store.ErrInvalidKey
	}
	key, err := store.CleanKey(path.Join("cache", layer, name))
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(key, "cache/"+layer+"/") {
		return "", store.ErrInvalidKey
	}
	return key, nil
}

func LayerPrefix(layer string) (string, error) {
	layer, err := cleanLayer(layer)
	if err != nil {
		return "", err
	}
	return "cache/" + layer, nil
}

// GetJSON loads a fresh cache entry into out. Cache access and decoding failures
// intentionally degrade to a miss because cache availability must not affect
// the underlying service operation.
func (c *storeCache) GetJSON(ctx context.Context, key string, out any) bool {
	if c == nil || c.store == nil || strings.TrimSpace(key) == "" {
		return false
	}
	key, err := store.CleanKey(key)
	if err != nil {
		return false
	}
	body, err := c.store.Read(ctx, key)
	if err != nil {
		return false
	}
	var record envelope
	if err := json.Unmarshal(body, &record); err != nil {
		return false
	}
	if record.StoredAt.IsZero() || record.ExpiresAt.IsZero() || !c.now().Before(record.ExpiresAt) {
		_ = c.store.Delete(ctx, key)
		return false
	}
	if err := json.Unmarshal(record.Value, out); err != nil {
		return false
	}
	return true
}

func (c *storeCache) PutJSON(ctx context.Context, key string, ttl time.Duration, value any) error {
	if c == nil || c.store == nil || ttl <= 0 || strings.TrimSpace(key) == "" {
		return nil
	}
	key, err := store.CleanKey(key)
	if err != nil {
		return err
	}
	valueBody, err := json.Marshal(value)
	if err != nil {
		return err
	}
	storedAt := c.now().UTC()
	body, err := json.Marshal(envelope{
		StoredAt:  storedAt,
		ExpiresAt: storedAt.Add(ttl),
		Value:     valueBody,
	})
	if err != nil {
		return err
	}
	return c.store.Write(ctx, key, body)
}

func (c *storeCache) DeleteLayer(ctx context.Context, layer string) error {
	if c == nil || c.store == nil {
		return nil
	}
	prefix, err := LayerPrefix(layer)
	if err != nil {
		return err
	}
	entries, err := c.store.List(ctx, prefix)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		if err := c.store.Delete(ctx, entry.Key); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func cleanLayer(layer string) (string, error) {
	layer = strings.TrimSpace(layer)
	if layer == "" || strings.Contains(layer, "/") || strings.Contains(layer, `\`) {
		return "", store.ErrInvalidKey
	}
	return store.CleanKey(layer)
}
