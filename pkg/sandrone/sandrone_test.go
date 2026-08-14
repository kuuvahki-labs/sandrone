package sandrone_test

import (
	"context"
	"encoding/json"
	"mime"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/pkg/sandrone"
)

func TestEngineParseAndRender(t *testing.T) {
	engine := sandrone.New()
	parsed, err := engine.Parse(context.Background(), sandrone.ParseRequest{
		Format:  "uri-list",
		Content: []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
	})
	require.NoError(t, err)
	require.Len(t, parsed.Nodes, 1)

	rendered, err := engine.Render(context.Background(), sandrone.RenderRequest{
		Format: "mihomo-proxies",
		Nodes:  parsed.Nodes,
	})
	require.NoError(t, err)
	require.Contains(t, string(rendered.Body), "node-a")
}

func TestPublicFileKindAndConfigAliases(t *testing.T) {
	var kind sandrone.FileKind = sandrone.FileKindMihomo
	config := sandrone.FileConfig{
		Subscriptions: []string{"default"},
		Settings:      json.RawMessage(`{"groups":[]}`),
	}
	require.Equal(t, sandrone.FileKind("mihomo"), kind)
	require.Equal(t, sandrone.FileKind("static"), sandrone.FileKindStatic)
	require.Equal(t, sandrone.FileKind("sing-box"), sandrone.FileKindSingBox)
	require.Equal(t, sandrone.FileKind("shadowrocket"), sandrone.FileKindShadowrocket)
	require.JSONEq(t, `{"groups":[]}`, string(config.Settings))
}

func TestEngineConvert(t *testing.T) {
	engine := sandrone.New()
	rendered, err := engine.Convert(context.Background(), sandrone.ConvertRequest{
		FromFormat: "uri-list",
		ToFormat:   "mihomo-proxies",
		Content:    []byte("ss://aes-128-gcm:secret@example.com:8388#node-a"),
	})
	require.NoError(t, err)
	require.Contains(t, string(rendered.Body), "node-a")
	require.Equal(t, "convert", rendered.Report.Kind)
}

func TestEngineGetFile(t *testing.T) {
	engine := sandrone.New()
	spec := sandrone.FileSpec{
		Name:   "base.yaml",
		Kind:   sandrone.FileKindStatic,
		Source: sandrone.FileSource{Type: "inline", Content: "key: value\n"},
	}
	result, err := engine.GetFile(context.Background(), sandrone.FileRequest{Spec: &spec})
	require.NoError(t, err)
	require.Contains(t, string(result.File.Content), "key: value")
}

func TestEngineRenderSubscriptionExposesResultCacheControls(t *testing.T) {
	ctx := context.Background()
	engine := sandrone.NewWithFS(afero.NewMemMapFs())
	ttl := 60
	require.NoError(t, engine.PutSubscription(ctx, sandrone.Subscription{
		Name: "cached", Type: sandrone.SubscriptionTypeLocal, Format: "uri-list",
		Content:               "ss://aes-128-gcm:secret@example.com:8388#node-a",
		RenderCacheTTLSeconds: &ttl,
	}))

	first, err := engine.RenderSubscription(ctx, sandrone.SubscriptionRenderRequest{
		Name: "cached", Format: "uri-list",
	})
	require.NoError(t, err)
	require.False(t, first.Cached)
	second, err := engine.RenderSubscription(ctx, sandrone.SubscriptionRenderRequest{
		Name: "cached", Format: "uri-list",
	})
	require.NoError(t, err)
	require.True(t, second.Cached)
	refreshed, err := engine.RenderSubscription(ctx, sandrone.SubscriptionRenderRequest{
		Name: "cached", Format: "uri-list", Refresh: true,
	})
	require.NoError(t, err)
	require.False(t, refreshed.Cached)
}

func TestEngineTypedFileConfigRoundTrip(t *testing.T) {
	ctx := context.Background()
	engine := sandrone.NewWithFS(afero.NewMemMapFs())
	spec := sandrone.FileSpec{
		Name: "client.json",
		Kind: sandrone.FileKindSingBox,
		Config: &sandrone.FileConfig{
			Subscriptions: []string{"provider"},
			Settings:      json.RawMessage(`{"groups":[],"rule_sets":[],"rules":[]}`),
		},
	}
	require.NoError(t, engine.PutSubscription(ctx, sandrone.Subscription{
		Name: "provider", Type: sandrone.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))
	require.NoError(t, engine.PutFile(ctx, spec))

	stored, err := engine.GetFileSpec(ctx, spec.Name)
	require.NoError(t, err)
	require.Equal(t, sandrone.FileKindSingBox, stored.Kind)
	require.Equal(t, []string{"provider"}, stored.Config.Subscriptions)
	require.JSONEq(t, string(spec.Config.Settings), string(stored.Config.Settings))

	result, err := engine.GetFile(ctx, sandrone.FileRequest{Name: spec.Name})
	require.NoError(t, err)
	require.Equal(t, "application/json", result.ContentType)
}

func TestEngineGetFileSource(t *testing.T) {
	ctx := context.Background()
	engine := sandrone.NewWithFS(afero.NewMemMapFs())
	require.NoError(t, engine.PutFile(ctx, sandrone.FileSpec{
		Name:   "base.yaml",
		Kind:   sandrone.FileKindStatic,
		Source: sandrone.FileSource{Type: "inline", Content: "key: source\n"},
	}))

	source, err := engine.GetFileSource(ctx, "base.yaml")
	require.NoError(t, err)
	require.Equal(t, "key: source\n", string(source.Content))
}

func TestNewWithStoreBacksResourcesWithCallerStore(t *testing.T) {
	ctx := context.Background()
	store := newPublicMemoryStore()
	engine := sandrone.NewWithStore(store)

	require.NoError(t, engine.PutFile(ctx, sandrone.FileSpec{
		Name:   "custom.yaml",
		Kind:   sandrone.FileKindStatic,
		Source: sandrone.FileSource{Type: "inline", Content: "mixed-port: 7890\n"},
		Meta:   map[string]string{"description": "custom store"},
	}))

	spec, err := engine.GetFileSpec(ctx, "custom.yaml")
	require.NoError(t, err)
	require.Equal(t, "custom.yaml", spec.Name)
	require.Equal(t, "custom store", spec.Meta["description"])
	require.Equal(t, "inline", spec.Source.Type)
	require.Equal(t, "mixed-port: 7890\n", spec.Source.Content)

	result, err := engine.GetFile(ctx, sandrone.FileRequest{Name: "custom.yaml"})
	require.NoError(t, err)
	require.Contains(t, string(result.File.Content), "mixed-port: 7890")

	files, err := engine.ListFiles(ctx)
	require.NoError(t, err)
	require.Len(t, files.Items, 1)
	require.Equal(t, "custom.yaml", files.Items[0].Name)

	require.False(t, store.has("files/custom.yaml"))
	require.True(t, store.has("files/custom.yaml.json"))
}

func TestEngineSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	engine := sandrone.NewWithFS(afero.NewMemMapFs())

	defaults, err := engine.GetSettings(ctx)
	require.NoError(t, err)
	require.Empty(t, defaults.Settings.RemoteDefaults.UserAgent)
	require.Equal(t, "url_test", defaults.Settings.ProbeDefaults.Method)
	require.Equal(t, "sing-box", defaults.Settings.ProbeDefaults.Core)
	require.Equal(t, 60, defaults.Settings.CacheDefaults.SubscriptionTrafficTTLSeconds)

	update := sandrone.SettingsUpdate{
		SchemaVersion: defaults.Settings.SchemaVersion,
		HTTP:          defaults.Settings.HTTP,
		MCP:           defaults.Settings.MCP,
		Log:           defaults.Settings.Log,
		RemoteDefaults: sandrone.RemoteDefaults{
			UserAgent: "Sandrone Test",
			Proxy:     "socks5://127.0.0.1:1080",
			TimeoutMS: 7000,
		},
		ProbeDefaults: sandrone.ProbeDefaults{
			Method:          "url_test",
			Core:            "sing-box",
			URL:             "https://example.com/generate_204",
			NTPServer:       "time.cloudflare.com",
			TimeoutMS:       9000,
			Attempts:        2,
			Concurrency:     12,
			CacheTTLSeconds: 300,
		},
		CacheDefaults: sandrone.CacheDefaults{
			RemoteFetchTTLSeconds:         120,
			SubscriptionTrafficTTLSeconds: 15,
		},
		Appearance:    defaults.Settings.Appearance,
		Subscriptions: defaults.Settings.Subscriptions,
	}
	saved, err := engine.PutSettings(ctx, update)
	require.NoError(t, err)

	got, err := engine.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, saved.Settings, got.Settings)
	require.Equal(t, update.RemoteDefaults, got.Settings.RemoteDefaults)
	require.Equal(t, update.ProbeDefaults, got.Settings.ProbeDefaults)
	require.Equal(t, update.CacheDefaults, got.Settings.CacheDefaults)
}

func TestEngineProbe(t *testing.T) {
	engine := sandrone.New()
	result, err := engine.Probe(context.Background(), sandrone.ProbeRequest{
		Input: sandrone.NodeInput{
			Type: "inline_nodes",
			Nodes: []sandrone.NodeIR{{
				Name:   "invalid",
				Server: "example.com",
			}},
		},
		Method: sandrone.ProbeTCPConnect,
	})
	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, sandrone.IsCode(err, "node_validation_failed"))
}

func TestIsCode(t *testing.T) {
	err := domain.NewError(domain.CodeInvalidArgument, "bad")
	require.True(t, sandrone.IsCode(err, string(domain.CodeInvalidArgument)))
	require.False(t, sandrone.IsCode(err, string(domain.CodeParseFailed)))
}

func TestEngineDeleteResources(t *testing.T) {
	ctx := context.Background()
	engine := sandrone.NewWithFS(afero.NewMemMapFs())

	require.NoError(t, engine.PutSubscription(ctx, sandrone.Subscription{Name: "sub", Type: sandrone.SubscriptionTypeLocal, Format: "uri-list"}))
	require.NoError(t, engine.DeleteSubscription(ctx, "sub"))
	_, err := engine.GetSubscription(ctx, "sub")
	require.Error(t, err)

	require.NoError(t, engine.PutFile(ctx, sandrone.FileSpec{
		Name:   "base.yaml",
		Kind:   sandrone.FileKindStatic,
		Source: sandrone.FileSource{Type: "inline", Content: "stored"},
	}))
	require.NoError(t, engine.DeleteFile(ctx, "base.yaml"))
	_, err = engine.GetFileSpec(ctx, "base.yaml")
	require.Error(t, err)

}

func TestEngineShareResources(t *testing.T) {
	ctx := context.Background()
	engine := sandrone.NewWithFS(afero.NewMemMapFs())

	require.NoError(t, engine.PutFile(ctx, sandrone.FileSpec{
		Name:   "default.yaml",
		Kind:   sandrone.FileKindStatic,
		Source: sandrone.FileSource{Type: "inline", Content: "proxies: []\n"},
	}))
	share, err := engine.CreateShare(ctx, sandrone.ShareCreateRequest{
		ID:          "sh_public",
		TargetKind:  "file",
		TargetName:  "default.yaml",
		ContentType: "application/yaml",
	})
	require.NoError(t, err)
	require.Equal(t, "sh_public", share.ID)

	shares, err := engine.ListShares(ctx)
	require.NoError(t, err)
	require.Len(t, shares.Shares, 1)

	rendered, err := engine.RenderShare(ctx, sandrone.ShareRenderRequest{ID: "sh_public"})
	require.NoError(t, err)
	require.Equal(t, "application/yaml", rendered.ContentType)
	require.Contains(t, string(rendered.Body), "proxies")
	dispositionType, dispositionParams, err := mime.ParseMediaType(rendered.Headers["Content-Disposition"])
	require.NoError(t, err)
	require.Equal(t, "inline", dispositionType)
	require.Equal(t, "default.yaml", dispositionParams["filename"])

	require.NoError(t, engine.DeleteShare(ctx, "sh_public"))
	_, err = engine.GetShare(ctx, "sh_public")
	require.Error(t, err)

}

func TestEngineGetFileScriptProducesSubscriptionContent(t *testing.T) {
	ctx := context.Background()
	engine := sandrone.NewWithFS(afero.NewMemMapFs())
	require.NoError(t, engine.PutSubscription(ctx, sandrone.Subscription{
		Name:    "nodes",
		Type:    sandrone.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))

	spec := sandrone.FileSpec{
		Name:   "mihomo.yaml",
		Kind:   sandrone.FileKindStatic,
		Source: sandrone.FileSource{Type: "inline", Content: "mixed-port: 7890\n"},
		Processors: []sandrone.ProcessorSpec{{
			Type:  "script",
			Stage: sandrone.StageFile,
			Params: map[string]json.RawMessage{
				"content": json.RawMessage(strconv.Quote(`
function main(input) {
  const produced = api.subscription.produce("nodes", { target: "mihomo-proxies" });
  input.file.content = input.file.content + produced.content;
  return input;
}`)),
			},
		}},
	}
	result, err := engine.GetFile(ctx, sandrone.FileRequest{Spec: &spec})
	require.NoError(t, err)
	require.Contains(t, string(result.File.Content), "node-a")
}

type publicMemoryStore struct {
	entries map[string]publicMemoryStoreEntry
	now     time.Time
	mu      sync.RWMutex
}

type publicMemoryStoreEntry struct {
	body    []byte
	modTime time.Time
}

func newPublicMemoryStore() *publicMemoryStore {
	return &publicMemoryStore{
		entries: map[string]publicMemoryStoreEntry{},
		now:     time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC),
	}
}

func (s *publicMemoryStore) Read(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte{}, entry.body...), nil
}

func (s *publicMemoryStore) Write(_ context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = publicMemoryStoreEntry{
		body:    append([]byte{}, value...),
		modTime: s.now,
	}
	return nil
}

func (s *publicMemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
	return nil
}

func (s *publicMemoryStore) List(_ context.Context, prefix string) ([]sandrone.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	out := []sandrone.Entry{}
	for key, entry := range s.entries {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		out = append(out, sandrone.Entry{
			Key:     key,
			Size:    int64(len(entry.body)),
			ModTime: entry.modTime,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func (s *publicMemoryStore) Stat(_ context.Context, key string) (sandrone.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[key]
	if !ok {
		return sandrone.Entry{}, os.ErrNotExist
	}
	return sandrone.Entry{
		Key:     key,
		Size:    int64(len(entry.body)),
		ModTime: entry.modTime,
	}, nil
}

func (s *publicMemoryStore) has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.entries[key]
	return ok
}
