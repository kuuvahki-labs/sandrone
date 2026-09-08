package store

import (
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"sync"
	"time"
)

const summaryCacheEntries = 1024
const summaryCacheBytes = 8 << 20
const summaryRecheckInterval = 10 * time.Second

type cachedSummary struct {
	value       []byte
	version     string
	fingerprint [32]byte
	recheckAt   time.Time
	used        uint64
	size        int
}

type summaryCache struct {
	mu         sync.Mutex
	items      map[string]cachedSummary
	bytes      int
	generation uint64
	tick       uint64
	now        func() time.Time
}

func (c *summaryCache) currentGeneration() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generation
}

// InvalidateLists also prevents in-flight reads from repopulating pre-write data.
// Restore uses it even when replacement fails and rolls back.
func (s *MetaStore) InvalidateLists() { s.summaries.invalidate("") }

func (c *summaryCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	if key == "" {
		clear(c.items)
		c.bytes = 0
		return
	}
	c.bytes -= c.items[key].size
	delete(c.items, key)
}

func (c *summaryCache) get(key string, generation uint64) (cachedSummary, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if !ok || generation != c.generation {
		return cachedSummary{}, false
	}
	c.tick++
	item.used = c.tick
	c.items[key] = item
	return item, true // value is immutable; callers only decode it.
}

func (c *summaryCache) put(key string, item cachedSummary, generation uint64) {
	item.size = len(key) + len(item.version) + len(item.value) + 128
	if item.size > summaryCacheBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if generation != c.generation {
		return
	}
	if c.items == nil {
		c.items = make(map[string]cachedSummary)
	}
	c.bytes -= c.items[key].size
	delete(c.items, key)
	for len(c.items) >= summaryCacheEntries || c.bytes+item.size > summaryCacheBytes {
		var oldest string
		used := ^uint64(0)
		for key, entry := range c.items {
			if entry.used < used {
				oldest, used = key, entry.used
			}
		}
		c.bytes -= c.items[oldest].size
		delete(c.items, oldest)
	}
	c.tick++
	item.used = c.tick
	c.items[key] = item
	c.bytes += item.size
}

func (s *MetaStore) listEntries(ctx context.Context, prefix string) ([]ListedEntry, uint64, error) {
	generation := s.summaries.currentGeneration()
	entries, err := metadataEntries(ctx, s.store, prefix)
	if err != nil {
		s.InvalidateLists()
		return nil, generation, err
	}
	present := make(map[string]bool, len(entries))
	for _, entry := range entries {
		present[entry.Key] = true
	}
	c := &s.summaries
	c.mu.Lock()
	defer c.mu.Unlock()
	if generation == c.generation {
		for key, item := range c.items {
			if len(key) > len(prefix) && key[:len(prefix)+1] == prefix+"/" && !present[key] {
				c.bytes -= item.size
				delete(c.items, key)
			}
		}
	}
	return entries, generation, nil
}

func (s *MetaStore) readSummary[T any](ctx context.Context, entry ListedEntry, generation uint64, decode func([]byte) (T, error)) (T, error) {
	var out T
	c := &s.summaries
	cached, hit := c.get(entry.Key, generation)
	if hit && entry.Version != "" && cached.version == entry.Version && c.now().Before(cached.recheckAt) {
		err := json.Unmarshal(cached.value, &out)
		return out, err
	}
	body, version, err := readVersion(ctx, s.store, entry.Key)
	if err != nil {
		c.invalidate(entry.Key)
		return out, err
	}
	fingerprint := sha256.Sum256(body)
	if hit && fingerprint == cached.fingerprint {
		err = json.Unmarshal(cached.value, &out)
	} else {
		out, err = decode(body)
	}
	if err != nil {
		c.invalidate(entry.Key)
		return out, err
	}
	// An object changed between LIST and GET: return the read value, but never
	// associate it with a different listed version. Missing tokens use body hashes.
	if entry.Version != "" && version != entry.Version {
		return out, nil
	}
	value, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	c.put(entry.Key, cachedSummary{value: value, version: version, fingerprint: fingerprint, recheckAt: c.now().Add(summaryRecheckInterval)}, generation)
	return out, nil
}
