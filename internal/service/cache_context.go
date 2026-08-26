package service

import (
	"context"
	"path"
	"strings"
	"sync"
)

const (
	cacheResourceSubscriptions = "subscriptions"
	cacheResourceFiles         = "files"
)

type cacheOwnerContextKey struct{}

type cacheOwner struct {
	ResourceKind string
	ResourceName string
}

type cacheReadBypassContextKey struct{}

type cacheReadBypassState struct {
	mu          sync.Mutex
	initialized map[string]bool
}

func withSubscriptionCacheOwner(ctx context.Context, name string) context.Context {
	return withCacheOwner(ctx, cacheResourceSubscriptions, name)
}

func withFileCacheOwner(ctx context.Context, name string) context.Context {
	return withCacheOwner(ctx, cacheResourceFiles, name)
}

func withCacheOwner(ctx context.Context, kind, name string) context.Context {
	owner := cacheOwner{ResourceKind: strings.TrimSpace(kind), ResourceName: name}
	if strings.TrimSpace(owner.ResourceName) == "" {
		return ctx
	}
	return context.WithValue(ctx, cacheOwnerContextKey{}, owner)
}

func ownedCacheKey(ctx context.Context, prefix string) (string, bool) {
	owner, ok := ctx.Value(cacheOwnerContextKey{}).(cacheOwner)
	if !ok || owner.ResourceKind == "" || owner.ResourceName == "" || strings.TrimSpace(prefix) == "" {
		return "", false
	}
	return path.Join(prefix, owner.ResourceKind, owner.ResourceName), true
}

func withProbeInputCacheOwner(ctx context.Context, inputKind, refKind, refName string) context.Context {
	if _, ok := ctx.Value(cacheOwnerContextKey{}).(cacheOwner); ok {
		return ctx
	}
	inputKind = strings.ToLower(strings.TrimSpace(inputKind))
	refKind = strings.ToLower(strings.TrimSpace(refKind))
	if (inputKind == "subscription" || (inputKind == "ref" && refKind == "subscription")) && strings.TrimSpace(refName) != "" {
		return withSubscriptionCacheOwner(ctx, refName)
	}
	return ctx
}

func withCacheReadBypass(ctx context.Context) context.Context {
	if cacheReadBypass(ctx) {
		return ctx
	}
	return context.WithValue(ctx, cacheReadBypassContextKey{}, &cacheReadBypassState{initialized: map[string]bool{}})
}

func cacheReadBypass(ctx context.Context) bool {
	_, bypass := ctx.Value(cacheReadBypassContextKey{}).(*cacheReadBypassState)
	return bypass
}

func cacheRefreshStartsKey(ctx context.Context, key string) bool {
	state, bypass := ctx.Value(cacheReadBypassContextKey{}).(*cacheReadBypassState)
	if !bypass {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.initialized[key] {
		return false
	}
	state.initialized[key] = true
	return true
}
