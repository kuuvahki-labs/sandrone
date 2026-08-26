package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

type resourceCacheProbeEngine struct {
	calls [][]string
}

type countingCache struct {
	cachepkg.Cache
	gets int
}

func (c *countingCache) Get(ctx context.Context, key string) (cachepkg.Item, bool, error) {
	c.gets++
	return c.Cache.Get(ctx, key)
}

func (e *resourceCacheProbeEngine) Probe(_ context.Context, req domain.ProbeRequest, nodes []domain.NodeIR, _ ...probe.Payload) (*domain.ProbeResult, error) {
	servers := make([]string, len(nodes))
	results := make([]domain.NodeProbeResult, len(nodes))
	for index, node := range nodes {
		servers[index] = node.Server
		results[index] = domain.NodeProbeResult{
			RuntimeID: domain.NodeRuntimeID(node), Method: string(req.Method), Core: req.Core,
			Alive: true, DurationMS: index + 1, CheckedAt: time.Date(2026, 8, 26, 1, 2, 3, 0, time.UTC),
		}
	}
	e.calls = append(e.calls, servers)
	return &domain.ProbeResult{Results: results, Report: domain.Report{Probe: &domain.ProbeReport{Backend: "fake"}}}, nil
}

func TestSavedResourceProbeCacheIsPartialAndResourceLocal(t *testing.T) {
	ctx := context.Background()
	fs := afero.NewMemMapFs()
	resourceStore := store.NewFSStore(fs)
	engine := &resourceCacheProbeEngine{}
	persistentCache := &countingCache{Cache: cachepkg.New(resourceStore, time.Now)}
	svc := New(WithStore(resourceStore), WithCache(persistentCache), WithProbeEngine(engine))
	base := []domain.NodeIR{
		{Name: "a", Type: domain.NodeTypeShadowsocks, Server: "a.example", Port: 1, Cipher: "aes-128-gcm", Password: "p"},
		{Name: "b", Type: domain.NodeTypeShadowsocks, Server: "b.example", Port: 2, Cipher: "aes-128-gcm", Password: "p"},
		{Name: "c", Type: domain.NodeTypeShadowsocks, Server: "c.example", Port: 3, Cipher: "aes-128-gcm", Password: "p"},
	}
	request := func(scope string, nodes []domain.NodeIR) *domain.ProbeResult {
		result, err := svc.Probe(withSubscriptionCacheScope(ctx, scope), domain.ProbeRequest{
			Input:  domain.NodeInput{Type: "inline_nodes", Nodes: nodes},
			Method: domain.ProbeTCPConnect, CacheTTLSeconds: 60,
		})
		require.NoError(t, err)
		return result
	}

	first := request("A", base)
	require.Zero(t, first.Report.Probe.CacheHitCount)
	getsAfterFirst := persistentCache.gets
	reordered := []domain.NodeIR{base[2], base[0], base[1]}
	reordered[0].Name = "renamed-c"
	second := request("A", reordered)
	require.Equal(t, 3, second.Report.Probe.CacheHitCount)
	require.Len(t, engine.calls, 1)
	require.Equal(t, getsAfterFirst+1, persistentCache.gets, "a full probe hit reads its single cache key once")

	reordered[2].Server = "changed.example"
	third := request("A", reordered)
	require.Equal(t, 2, third.Report.Probe.CacheHitCount)
	require.Equal(t, [][]string{{"a.example", "b.example", "c.example"}, {"changed.example"}}, engine.calls)

	separate := request("B", base)
	require.Zero(t, separate.Report.Probe.CacheHitCount)
	require.Len(t, engine.calls, 3)

	for _, name := range []string{"A", "B"} {
		item, found, err := svc.cache.Get(ctx, "probe/subscriptions/"+name)
		require.NoError(t, err)
		require.True(t, found)
		var document struct {
			Entries map[string]json.RawMessage `json:"entries"`
		}
		require.NoError(t, json.Unmarshal(item.Value, &document))
		require.NotEmpty(t, document.Entries)
	}
}

func TestSavedResourceProbeProfileSeparatesTargets(t *testing.T) {
	ctx := withSubscriptionCacheScope(context.Background(), "A")
	engine := &resourceCacheProbeEngine{}
	svc := New(WithFS(afero.NewMemMapFs()), WithProbeEngine(engine))
	node := domain.NodeIR{Name: "n", Type: domain.NodeTypeShadowsocks, Server: "same.example", Port: 443, Cipher: "aes-128-gcm", Password: "p"}
	request := func(url string) *domain.ProbeResult {
		result, err := svc.Probe(ctx, domain.ProbeRequest{
			Input:  domain.NodeInput{Type: "inline_nodes", Nodes: []domain.NodeIR{node}},
			Method: domain.ProbeURLTest, Core: "sing-box", URL: url, CacheTTLSeconds: 60,
		})
		require.NoError(t, err)
		return result
	}
	require.False(t, request("https://one.example/generate_204").Results[0].CacheHit)
	require.True(t, request("https://one.example/generate_204").Results[0].CacheHit)
	require.False(t, request("https://two.example/generate_204").Results[0].CacheHit)
	require.Len(t, engine.calls, 2)
}
