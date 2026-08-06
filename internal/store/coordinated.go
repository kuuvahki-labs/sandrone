package store

import (
	"context"
	"sync"
)

// Coordinator serializes Store updates against consistent read views.
// Callbacks receive the raw Store so compound operations can use it without
// attempting to acquire the same coordination lock again.
type Coordinator interface {
	Store
	View(context.Context, func(Store) error) error
	Update(context.Context, func(Store) error) error
}

type coordinatedStore struct {
	store Store
	mu    sync.RWMutex
}

// Coordinate wraps a Store in a shared coordination boundary. Coordinators
// are returned unchanged so callers do not create independent nested locks.
func Coordinate(resourceStore Store) Coordinator {
	if coordinator, ok := resourceStore.(Coordinator); ok {
		return coordinator
	}
	return &coordinatedStore{store: resourceStore}
}

func (s *coordinatedStore) View(_ context.Context, view func(Store) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return view(s.store)
}

func (s *coordinatedStore) Update(_ context.Context, update func(Store) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return update(s.store)
}

func (s *coordinatedStore) Read(ctx context.Context, key string) ([]byte, error) {
	var body []byte
	err := s.View(ctx, func(resourceStore Store) error {
		var err error
		body, err = resourceStore.Read(ctx, key)
		return err
	})
	return body, err
}

func (s *coordinatedStore) Write(ctx context.Context, key string, value []byte) error {
	return s.Update(ctx, func(resourceStore Store) error {
		return resourceStore.Write(ctx, key, value)
	})
}

func (s *coordinatedStore) Delete(ctx context.Context, key string) error {
	return s.Update(ctx, func(resourceStore Store) error {
		return resourceStore.Delete(ctx, key)
	})
}

func (s *coordinatedStore) List(ctx context.Context, prefix string) ([]Entry, error) {
	var entries []Entry
	err := s.View(ctx, func(resourceStore Store) error {
		var err error
		entries, err = resourceStore.List(ctx, prefix)
		return err
	})
	return entries, err
}

func (s *coordinatedStore) Stat(ctx context.Context, key string) (Entry, error) {
	var entry Entry
	err := s.View(ctx, func(resourceStore Store) error {
		var err error
		entry, err = resourceStore.Stat(ctx, key)
		return err
	})
	return entry, err
}
