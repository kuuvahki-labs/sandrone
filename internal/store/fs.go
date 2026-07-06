package store

import (
	"bytes"
	"context"
	"os"
	"path"
	"sort"
	"sync"

	"github.com/spf13/afero"
)

type FSStore struct {
	fs afero.Fs
	mu sync.RWMutex
}

func NewFSStore(fs afero.Fs) *FSStore {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	return &FSStore{fs: fs}
}

func (s *FSStore) Read(_ context.Context, key string) ([]byte, error) {
	key, err := CleanKey(key)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return afero.ReadFile(s.fs, key)
}

func (s *FSStore) Write(_ context.Context, key string, value []byte) error {
	key, err := CleanKey(key)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(key, value)
}

func (s *FSStore) write(key string, value []byte) error {
	if err := s.fs.MkdirAll(path.Dir(key), 0o755); err != nil {
		return err
	}
	return afero.WriteFile(s.fs, key, value, 0o644)
}

func (s *FSStore) CompareAndSwap(_ context.Context, key string, oldValue, newValue []byte) (bool, error) {
	key, err := CleanKey(key)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := afero.ReadFile(s.fs, key)
	if err != nil {
		if !os.IsNotExist(err) || oldValue != nil {
			return false, err
		}
	} else if oldValue == nil || !bytes.Equal(current, oldValue) {
		return false, nil
	}
	if newValue == nil {
		if err := s.fs.Remove(key); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		return true, nil
	}
	if err := s.write(key, newValue); err != nil {
		return false, err
	}
	return true, nil
}

func (s *FSStore) Delete(_ context.Context, key string) error {
	key, err := CleanKey(key)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fs.Remove(key)
}

func (s *FSStore) List(_ context.Context, prefix string) ([]Entry, error) {
	prefix, err := cleanPrefix(prefix)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	root := "."
	if prefix != "" {
		root = prefix
	}
	info, err := s.fs.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []Entry{entryFromInfo(root, info)}, nil
	}
	entries := []Entry{}
	err = afero.Walk(s.fs, root, func(key string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if key == "." || key == root {
			return nil
		}
		entries = append(entries, entryFromInfo(path.Clean(key), info))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	return entries, nil
}

func (s *FSStore) Stat(_ context.Context, key string) (Entry, error) {
	key, err := CleanKey(key)
	if err != nil {
		return Entry{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, err := s.fs.Stat(key)
	if err != nil {
		return Entry{}, err
	}
	return entryFromInfo(key, info), nil
}

func entryFromInfo(key string, info os.FileInfo) Entry {
	return Entry{
		Key:     key,
		Size:    info.Size(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime(),
	}
}
