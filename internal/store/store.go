package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"
)

var ErrInvalidKey = errors.New("invalid store key")

// Store is the storage boundary used by service. Keys are safe relative
// slash-separated paths; implementations must reject absolute paths and
// traversal.
type Store interface {
	Read(ctx context.Context, key string) ([]byte, error)
	Write(ctx context.Context, key string, value []byte) error
	// CompareAndSwap atomically writes newValue only when the current bytes equal
	// oldValue. A nil oldValue matches a missing key; a nil newValue deletes it.
	CompareAndSwap(ctx context.Context, key string, oldValue, newValue []byte) (bool, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]Entry, error)
	Stat(ctx context.Context, key string) (Entry, error)
}

type AtomicWriter interface {
	WriteAtomic(ctx context.Context, key string, value []byte, mode fs.FileMode) error
}

type Entry struct {
	Key     string    `json:"key" yaml:"key"`
	Size    int64     `json:"size" yaml:"size"`
	IsDir   bool      `json:"is_dir" yaml:"is_dir"`
	ModTime time.Time `json:"mod_time" yaml:"mod_time"`
}

// CleanKey validates and canonicalises a store key. Empty keys, NUL bytes,
// Windows drive prefixes, absolute paths, backslashes and dot traversal are
// rejected.
func CleanKey(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("%w: empty key", ErrInvalidKey)
	}
	if strings.IndexByte(key, 0) >= 0 {
		return "", fmt.Errorf("%w: NUL bytes are not allowed", ErrInvalidKey)
	}
	if len(key) >= 2 && ((key[0] >= 'A' && key[0] <= 'Z') || (key[0] >= 'a' && key[0] <= 'z')) && key[1] == ':' {
		return "", fmt.Errorf("%w: Windows drive prefixes are not allowed", ErrInvalidKey)
	}
	if strings.Contains(key, `\`) {
		return "", fmt.Errorf("%w: backslashes are not allowed", ErrInvalidKey)
	}
	if path.IsAbs(key) {
		return "", fmt.Errorf("%w: absolute paths are not allowed", ErrInvalidKey)
	}
	parts := strings.Split(key, "/")
	for _, part := range parts {
		switch part {
		case "", ".", "..":
			return "", fmt.Errorf("%w: unsafe segment %q", ErrInvalidKey, part)
		}
	}
	clean := path.Clean(key)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("%w: traversal is not allowed", ErrInvalidKey)
	}
	return clean, nil
}

func cleanPrefix(prefix string) (string, error) {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return "", nil
	}
	return CleanKey(prefix)
}
