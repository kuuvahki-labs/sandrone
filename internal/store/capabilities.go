package store

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// ListedEntry carries an optional opaque version, comparable with ReadVersion.
// Directory entries have no version. It does not change the public Store API.
type ListedEntry struct {
	Entry
	Version string
}

type prefixLister interface {
	ListPrefix(context.Context, string) ([]ListedEntry, error)
}
type versionReader interface {
	ReadVersion(context.Context, string) ([]byte, string, error)
}
type concurrentReader interface{ ConcurrentReads() bool }

// ListPrefix lists only descendants, with validated unique keys in key order.
// Missing directories and an exact file prefix produce an empty list.
func ListPrefix(ctx context.Context, st Store, prefix string) ([]ListedEntry, error) {
	prefix, err := cleanPrefix(prefix)
	if err != nil {
		return nil, err
	}
	var entries []ListedEntry
	if capable, ok := st.(prefixLister); ok {
		entries, err = capable.ListPrefix(ctx, prefix)
	} else {
		var listed []Entry
		listed, err = st.List(ctx, prefix)
		for _, entry := range listed {
			if entry.Key != prefix {
				entries = append(entries, ListedEntry{Entry: entry})
			}
		}
	}
	if errors.Is(err, fs.ErrNotExist) {
		return []ListedEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		key, err := CleanKey(entry.Key)
		if err != nil || key != entry.Key || (prefix != "" && !strings.HasPrefix(key, prefix+"/")) {
			return nil, fmt.Errorf("%w: entry outside listed prefix", ErrInvalidKey)
		}
		if seen[key] {
			return nil, fmt.Errorf("%w: duplicate listed key %s", ErrInvalidKey, key)
		}
		seen[key] = true
	}
	slices.SortFunc(entries, func(a, b ListedEntry) int { return cmp.Compare(a.Key, b.Key) })
	return entries, nil
}

func readVersion(ctx context.Context, st Store, key string) ([]byte, string, error) {
	if capable, ok := st.(versionReader); ok {
		return capable.ReadVersion(ctx, key)
	}
	body, err := st.Read(ctx, key)
	return body, "", err
}

func allowsConcurrentReads(st Store) bool {
	capable, ok := st.(concurrentReader)
	return ok && capable.ConcurrentReads()
}

func (*FSStore) ConcurrentReads() bool            { return true }
func (*S3Store) ConcurrentReads() bool            { return true }
func (s *coordinatedStore) ConcurrentReads() bool { return allowsConcurrentReads(s.store) }
func (s *coordinatedStore) ListPrefix(ctx context.Context, prefix string) ([]ListedEntry, error) {
	var entries []ListedEntry
	err := s.View(ctx, func(st Store) error {
		var err error
		entries, err = ListPrefix(ctx, st, prefix)
		return err
	})
	return entries, err
}
func (s *coordinatedStore) ReadVersion(ctx context.Context, key string) ([]byte, string, error) {
	var body []byte
	var version string
	err := s.View(ctx, func(st Store) error {
		var err error
		body, version, err = readVersion(ctx, st, key)
		return err
	})
	return body, version, err
}
