package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// JSONItem is one decoded cache value and its absolute deadline.
type JSONItem[T any] struct {
	Value     T
	ExpiresAt time.Time
}

// GetJSON reads one cache value and decodes it as JSON into T.
func GetJSON[T any](ctx context.Context, backend Cache, key string) (JSONItem[T], bool, error) {
	var out JSONItem[T]
	if backend == nil {
		return out, false, nil
	}
	item, found, err := backend.Get(ctx, key)
	if err != nil || !found {
		return out, found, err
	}
	if err := json.Unmarshal(item.Value, &out.Value); err != nil {
		return JSONItem[T]{}, false, fmt.Errorf("decode cache key %q JSON: %w", key, err)
	}
	out.ExpiresAt = item.ExpiresAt
	return out, true, nil
}

// SetJSON encodes one value as JSON and stores it with the supplied TTL.
func SetJSON[T any](ctx context.Context, backend Cache, key string, value T, ttl time.Duration) error {
	if backend == nil {
		return nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode cache key %q JSON: %w", key, err)
	}
	return backend.Set(ctx, key, body, ttl)
}
