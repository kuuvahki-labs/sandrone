// Package cache provides a backend-independent key/value TTL cache boundary.
package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/store"
)

const (
	compressionThresholdBytes = 4 << 10
	encodingGZIP              = "gzip"
)

// Item is one unexpired opaque cache value and its absolute deadline.
type Item struct {
	Value     []byte
	ExpiresAt time.Time
}

// Cache stores opaque values by key. Key structure and value contents belong
// to callers; implementations only enforce TTL and cache-wide lifecycle.
type Cache interface {
	Get(ctx context.Context, key string) (Item, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

type storeCache struct {
	store store.Coordinator
	now   func() time.Time
}

type storedItem struct {
	ExpiresAt time.Time `json:"expires_at"`
	Encoding  string    `json:"encoding,omitempty"`
	Value     []byte    `json:"value"`
}

func New(st store.Store, now func() time.Time) Cache {
	if now == nil {
		now = time.Now
	}
	if st == nil {
		return &storeCache{now: now}
	}
	return &storeCache{store: store.Coordinate(st), now: now}
}

func (c *storeCache) Get(ctx context.Context, key string) (Item, bool, error) {
	if c == nil || c.store == nil {
		return Item{}, false, nil
	}
	storeKey, err := cacheStoreKey(key)
	if err != nil {
		return Item{}, false, err
	}
	body, err := c.store.Read(ctx, storeKey)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Item{}, false, nil
		}
		return Item{}, false, err
	}
	var stored storedItem
	if err := json.Unmarshal(body, &stored); err != nil {
		return Item{}, false, fmt.Errorf("decode cache key %q: %w", key, err)
	}
	if stored.ExpiresAt.IsZero() {
		return Item{}, false, fmt.Errorf("decode cache key %q: missing expiration", key)
	}
	if !c.now().Before(stored.ExpiresAt) {
		return Item{}, false, nil
	}
	value, err := decodeStoredValue(stored.Encoding, stored.Value)
	if err != nil {
		return Item{}, false, fmt.Errorf("decode cache key %q: %w", key, err)
	}
	return Item{Value: value, ExpiresAt: stored.ExpiresAt}, true, nil
}

func (c *storeCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if c == nil || c.store == nil {
		return nil
	}
	if ttl <= 0 {
		return c.Delete(ctx, key)
	}
	storeKey, err := cacheStoreKey(key)
	if err != nil {
		return err
	}
	storedValue, encoding, err := encodeStoredValue(value)
	if err != nil {
		return err
	}
	body, err := json.Marshal(storedItem{
		ExpiresAt: c.now().UTC().Add(ttl),
		Encoding:  encoding,
		Value:     storedValue,
	})
	if err != nil {
		return err
	}
	return c.store.Write(ctx, storeKey, body)
}

func encodeStoredValue(value []byte) ([]byte, string, error) {
	raw := append([]byte(nil), value...)
	if len(raw) < compressionThresholdBytes {
		return raw, "", nil
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
	if err != nil {
		return nil, "", err
	}
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	if compressed.Len() >= len(raw) {
		return raw, "", nil
	}
	return compressed.Bytes(), encodingGZIP, nil
}

func decodeStoredValue(encoding string, value []byte) ([]byte, error) {
	switch encoding {
	case "":
		return append([]byte(nil), value...), nil
	case encodingGZIP:
		reader, err := gzip.NewReader(bytes.NewReader(value))
		if err != nil {
			return nil, err
		}
		decoded, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("unsupported cache encoding %q", encoding)
	}
}

func (c *storeCache) Delete(ctx context.Context, key string) error {
	if c == nil || c.store == nil {
		return nil
	}
	storeKey, err := cacheStoreKey(key)
	if err != nil {
		return err
	}
	if err := c.store.Delete(ctx, storeKey); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (c *storeCache) Clear(ctx context.Context) error {
	if c == nil || c.store == nil {
		return nil
	}
	entries, err := c.store.List(ctx, "cache")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, listed := range entries {
		if listed.IsDir {
			continue
		}
		if err := c.store.Delete(ctx, listed.Key); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func cacheStoreKey(key string) (string, error) {
	key, err := store.CleanKey(key)
	if err != nil {
		return "", err
	}
	return store.CleanKey(path.Join("cache", key+".json"))
}
