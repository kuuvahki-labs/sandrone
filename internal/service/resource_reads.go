package service

import (
	"context"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"sync"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type resourceReadContextKey struct{}

type resourceReadScope struct {
	owner *Service
	mu    sync.Mutex
	items map[string]resourceRead
}

type resourceRead struct {
	body     []byte
	revision string
}

func (s *Service) withResourceReads(ctx context.Context) context.Context {
	if s.resourceReads(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, resourceReadContextKey{}, &resourceReadScope{
		owner: s, items: map[string]resourceRead{},
	})
}

func (s *Service) resourceReads(ctx context.Context) *resourceReadScope {
	scope, _ := ctx.Value(resourceReadContextKey{}).(*resourceReadScope)
	if scope != nil && scope.owner == s {
		return scope
	}
	return nil
}

func (s *Service) readResource[T any](ctx context.Context, kind, name string, load func(context.Context, string) (T, error)) (T, string, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, "", err
	}
	scope := s.resourceReads(ctx)
	if scope == nil {
		value, err := load(ctx, name)
		return value, "", err
	}
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return zero, "", err
	}
	key := kind + "\x00" + name
	if item, ok := scope.items[key]; ok {
		var value T
		err := json.Unmarshal(item.body, &value, jsonv1.DefaultOptionsV1())
		return value, item.revision, err
	}
	value, err := load(ctx, name)
	if err != nil {
		return zero, "", err
	}
	// Keep the persisted model and legacy identity semantics. Encoding the memo
	// also prevents consumers from sharing mutable maps, slices or pointers.
	body, err := json.Marshal(value, jsonv1.DefaultOptionsV1())
	if err != nil {
		return zero, "", err
	}
	revision, err := cacheIdentity(value)
	if err != nil {
		return zero, "", err
	}
	scope.items[key] = resourceRead{body: body, revision: revision}
	return value, revision, nil
}

func (s *Service) loadSubscription(ctx context.Context, name string) (domain.Subscription, error) {
	value, _, err := s.readResource(ctx, "subscription", name, s.metaStore.GetSubscription)
	return value, err
}

func (s *Service) loadFileDefinition(ctx context.Context, name string) (domain.FileSpec, error) {
	value, _, err := s.readResource(ctx, "file", name, s.metaStore.GetFile)
	return value, err
}

func (s *Service) invalidateResourceRead(ctx context.Context, kind, name string) {
	if scope := s.resourceReads(ctx); scope != nil {
		scope.mu.Lock()
		delete(scope.items, kind+"\x00"+name)
		scope.mu.Unlock()
	}
}

func (s *Service) invalidateResourceReads(ctx context.Context) {
	if scope := s.resourceReads(ctx); scope != nil {
		scope.mu.Lock()
		clear(scope.items)
		scope.mu.Unlock()
	}
}
