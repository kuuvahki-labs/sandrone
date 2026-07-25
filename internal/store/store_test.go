package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

func TestFSStoreReadWriteListStatDelete(t *testing.T) {
	ctx := context.Background()
	st := store.NewFSStore(afero.NewMemMapFs())

	require.NoError(t, st.Write(ctx, "dir/file.txt", []byte("hello")))

	body, err := st.Read(ctx, "dir/file.txt")
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))

	entry, err := st.Stat(ctx, "dir/file.txt")
	require.NoError(t, err)
	require.Equal(t, "dir/file.txt", entry.Key)
	require.Equal(t, int64(5), entry.Size)
	require.False(t, entry.IsDir)

	entries, err := st.List(ctx, "dir")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "dir/file.txt", entries[0].Key)

	require.NoError(t, st.Delete(ctx, "dir/file.txt"))
	_, err = st.Read(ctx, "dir/file.txt")
	require.True(t, os.IsNotExist(err))
}

func TestFSStoreCompareAndSwap(t *testing.T) {
	ctx := context.Background()
	st := store.NewFSStore(afero.NewMemMapFs())

	swapped, err := st.CompareAndSwap(ctx, "state/value", nil, []byte("one"))
	require.NoError(t, err)
	require.True(t, swapped)

	swapped, err = st.CompareAndSwap(ctx, "state/value", nil, []byte("wrong"))
	require.NoError(t, err)
	require.False(t, swapped)
	swapped, err = st.CompareAndSwap(ctx, "state/value", []byte("one"), []byte("two"))
	require.NoError(t, err)
	require.True(t, swapped)

	body, err := st.Read(ctx, "state/value")
	require.NoError(t, err)
	require.Equal(t, "two", string(body))
}

func TestFSStoreRejectsUnsafeKeys(t *testing.T) {
	ctx := context.Background()
	st := store.NewFSStore(afero.NewMemMapFs())
	for _, key := range []string{"", "/abs", "../x", "a/../b", `a\b`, "a//b", "./a"} {
		t.Run(strings.ReplaceAll(key, "/", "_"), func(t *testing.T) {
			err := st.Write(ctx, key, []byte("x"))
			require.Error(t, err)
			require.True(t, errors.Is(err, store.ErrInvalidKey))
		})
	}
}

func TestCleanKeyRejectsNULAndWindowsDrivePrefixes(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
	}{
		{name: "NUL", key: "parent\x00child"},
		{name: "drive absolute", key: "C:/x"},
		{name: "drive relative", key: "C:x"},
		{name: "lowercase drive", key: "z:value"},
		{name: "drive only", key: "A:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleaned, err := store.CleanKey(tc.key)
			require.Empty(t, cleaned)
			require.ErrorIs(t, err, store.ErrInvalidKey)
		})
	}
}

func TestMetaStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := store.NewFSStore(afero.NewMemMapFs())
	meta := store.NewMetaStore(st)

	file := domain.FileSpec{
		Name:      "mihomo/base.yaml",
		Source:    domain.FileSource{Type: "inline", Content: "proxies: []"},
		CreatedAt: time.Date(2026, 6, 27, 1, 2, 3, 0, time.UTC),
		UpdatedAt: time.Date(2026, 6, 27, 4, 5, 6, 0, time.UTC),
	}
	require.NoError(t, meta.PutFile(ctx, file))
	gotFile, err := meta.GetFile(ctx, "mihomo/base.yaml")
	require.NoError(t, err)
	require.Equal(t, "mihomo/base.yaml", gotFile.Name)
	require.Equal(t, file.CreatedAt, gotFile.CreatedAt)
	require.Equal(t, file.UpdatedAt, gotFile.UpdatedAt)
	require.Equal(t, "inline", gotFile.Source.Type)
	require.Equal(t, "proxies: []", gotFile.Source.Content)
	entries, err := st.List(ctx, "files/mihomo")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, []string{"files/mihomo/base.yaml.json"}, []string{entries[0].Key})
}

func TestMetaStorePutFileStoresCompleteInlineSourceInJSONRecord(t *testing.T) {
	ctx := context.Background()
	fsStore := store.NewFSStore(afero.NewMemMapFs())
	meta := store.NewMetaStore(fsStore)

	require.NoError(t, meta.PutFile(ctx, domain.FileSpec{
		Name:        "rename.js",
		DisplayName: "  rename  ",
		Source: domain.FileSource{
			Type:    "inline",
			Content: "function main(input) { return input; }\n",
		},
		Meta: map[string]string{"description": "node rename script\nused by file stage"},
	}))

	_, err := fsStore.Read(ctx, "files/rename.js")
	require.True(t, os.IsNotExist(err))

	metadataBody, err := fsStore.Read(ctx, "files/rename.js.json")
	require.NoError(t, err)
	require.Contains(t, string(metadataBody), "function main")

	got, err := meta.GetFile(ctx, "rename.js")
	require.NoError(t, err)
	require.Equal(t, "rename.js", got.Name)
	require.Equal(t, "rename", got.DisplayName)
	require.Equal(t, "inline", got.Source.Type)
	require.Equal(t, "function main(input) { return input; }\n", got.Source.Content)
	require.Equal(t, "node rename script\nused by file stage", got.Meta["description"])

	files, err := meta.ListFiles(ctx)
	require.NoError(t, err)
	require.Equal(t, []domain.ResourceSummary{{
		Kind:        "file",
		Type:        "inline",
		Name:        "rename.js",
		DisplayName: "rename",
		Meta:        map[string]string{"description": "node rename script\nused by file stage"},
		Size:        files[0].Size,
	}}, files)
}

func TestMetaStoreRemoteSourceClearsInlineFields(t *testing.T) {
	ctx := context.Background()
	meta := store.NewMetaStore(store.NewFSStore(afero.NewMemMapFs()))

	require.NoError(t, meta.PutFile(ctx, domain.FileSpec{
		Name: "remote.txt",
		Source: domain.FileSource{
			Type:    " REMOTE ",
			Content: "stale inline content",
			Remote:  &domain.RemoteInput{URL: "https://example.com/file"},
		},
	}))

	got, err := meta.GetFile(ctx, "remote.txt")
	require.NoError(t, err)
	require.Equal(t, "remote", got.Source.Type)
	require.Empty(t, got.Source.Content)
	require.Equal(t, "https://example.com/file", got.Source.Remote.URL)
}

func TestMetaStoreFilesNamedConfigAndConfigJSONCoexistAndDeleteIndependently(t *testing.T) {
	ctx := context.Background()
	fsStore := store.NewFSStore(afero.NewMemMapFs())
	meta := store.NewMetaStore(fsStore)

	require.NoError(t, meta.PutFile(ctx, domain.FileSpec{
		Name: "config", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "plain"},
	}))
	require.NoError(t, meta.PutFile(ctx, domain.FileSpec{
		Name: "config.json", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "json"},
	}))

	files, err := meta.ListFiles(ctx)
	require.NoError(t, err)
	require.Len(t, files, 2)
	require.Equal(t, []string{"config", "config.json"}, []string{files[0].Name, files[1].Name})
	plain, err := meta.GetFile(ctx, "config")
	require.NoError(t, err)
	require.Equal(t, "plain", plain.Source.Content)
	jsonFile, err := meta.GetFile(ctx, "config.json")
	require.NoError(t, err)
	require.Equal(t, "json", jsonFile.Source.Content)

	require.NoError(t, meta.DeleteFile(ctx, "config"))
	_, err = meta.GetFile(ctx, "config")
	require.True(t, os.IsNotExist(err))
	jsonFile, err = meta.GetFile(ctx, "config.json")
	require.NoError(t, err)
	require.Equal(t, "json", jsonFile.Source.Content)
	require.NoError(t, meta.DeleteFile(ctx, "config.json"))
	_, err = meta.GetFile(ctx, "config.json")
	require.True(t, os.IsNotExist(err))
}

func TestMetaStoreListFilesIncludesProcessorsForUsageInference(t *testing.T) {
	ctx := context.Background()
	meta := store.NewMetaStore(store.NewFSStore(afero.NewMemMapFs()))

	require.NoError(t, meta.PutFile(ctx, domain.FileSpec{
		Name:   "scripted.yaml",
		Source: domain.FileSource{Type: "inline", Content: ""},
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageFile,
			Params: map[string]json.RawMessage{
				"source": json.RawMessage(`{"type":"inline","content":"function main(input) { return input; }"}`),
			},
		}},
	}))

	files, err := meta.ListFiles(ctx)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Len(t, files[0].Processors, 1)
	processor := files[0].Processors[0]
	require.Equal(t, "script", processor.Type)
	require.Equal(t, domain.StageFile, processor.Stage)
	require.JSONEq(t, `{"type":"inline","content":"function main(input) { return input; }"}`, string(processor.Params["source"]))
}

func TestMetaStoreListResourceSummariesExposeTimestamps(t *testing.T) {
	ctx := context.Background()
	meta := store.NewMetaStore(store.NewFSStore(afero.NewMemMapFs()))
	createdAt := time.Date(2026, 6, 27, 1, 2, 3, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 27, 4, 5, 6, 0, time.UTC)

	require.NoError(t, meta.PutSubscription(ctx, domain.Subscription{
		Name:      "provider",
		Type:      domain.SubscriptionTypeRemote,
		Format:    "uri-list",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}))
	require.NoError(t, meta.PutFile(ctx, domain.FileSpec{
		Name:      "default.yaml",
		Source:    domain.FileSource{Type: "inline", Content: "port: 7890\n"},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}))

	subscriptions, err := meta.ListSubscriptions(ctx)
	require.NoError(t, err)
	files, err := meta.ListFiles(ctx)
	require.NoError(t, err)

	for _, summary := range []domain.ResourceSummary{subscriptions[0], files[0]} {
		body, err := json.Marshal(summary)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))
		require.Equal(t, "2026-06-27T01:02:03Z", payload["created_at"])
		require.Equal(t, "2026-06-27T04:05:06Z", payload["updated_at"])
	}
}

func TestMetaStoreDeleteFileRemovesJSONRecord(t *testing.T) {
	ctx := context.Background()
	fsStore := store.NewFSStore(afero.NewMemMapFs())
	meta := store.NewMetaStore(fsStore)

	require.NoError(t, meta.PutFile(ctx, domain.FileSpec{
		Name:   "rename.js",
		Source: domain.FileSource{Type: "inline", Content: "function main(input) { return input; }\n"},
	}))

	require.NoError(t, meta.DeleteFile(ctx, "rename.js"))

	_, err := fsStore.Read(ctx, "files/rename.js.json")
	require.True(t, os.IsNotExist(err), "JSON record should be deleted: %v", err)
}

func TestMetaStoreRoundTripSubscription(t *testing.T) {
	ctx := context.Background()
	meta := store.NewMetaStore(store.NewFSStore(afero.NewMemMapFs()))

	sub := domain.Subscription{
		Name:        "remote/a",
		DisplayName: "  Provider A  ",
		Type:        domain.SubscriptionTypeRemote,
		Format:      "uri-list",
		Content:     "ss://x",
		CreatedAt:   time.Date(2026, 6, 27, 1, 2, 3, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 6, 27, 4, 5, 6, 0, time.UTC),
		Meta:        map[string]string{"description": "daily nodes\nbackup route"},
		Nodes: []domain.NodeIR{{
			Name:   "node-a",
			Type:   domain.NodeTypeShadowsocks,
			Server: "example.com",
			Port:   8388,
			Cipher: "aes-128-gcm",
		}},
	}
	require.NoError(t, meta.PutSubscription(ctx, sub))
	gotSource, err := meta.GetSubscription(ctx, "remote/a")
	require.NoError(t, err)
	require.Equal(t, "remote/a", gotSource.Name)
	require.Equal(t, "Provider A", gotSource.DisplayName)
	require.Equal(t, sub.Type, gotSource.Type)
	require.Equal(t, sub.Format, gotSource.Format)
	require.Equal(t, sub.Content, gotSource.Content)
	require.Equal(t, sub.CreatedAt, gotSource.CreatedAt)
	require.Equal(t, sub.UpdatedAt, gotSource.UpdatedAt)
	require.Equal(t, sub.Nodes, gotSource.Nodes)
	require.Equal(t, "daily nodes\nbackup route", gotSource.Meta["description"])

	subscriptions, err := meta.ListSubscriptions(ctx)
	require.NoError(t, err)
	require.Len(t, subscriptions, 1)
	require.Equal(t, "subscription", subscriptions[0].Kind)
	require.Equal(t, "remote", subscriptions[0].Type)
	require.Equal(t, "remote/a", subscriptions[0].Name)
	require.Equal(t, "Provider A", subscriptions[0].DisplayName)
	require.Equal(t, "uri-list", subscriptions[0].Format)
	require.Equal(t, "daily nodes\nbackup route", subscriptions[0].Meta["description"])
}

func TestMetaStoreWritesJSONWithoutHTMLEscaping(t *testing.T) {
	ctx := context.Background()
	fsStore := store.NewFSStore(afero.NewMemMapFs())
	meta := store.NewMetaStore(fsStore)

	require.NoError(t, meta.PutSubscription(ctx, domain.Subscription{
		Name:    "local/plain",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://x?left=1&right=<tag>#a>b",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: map[string]json.RawMessage{
				"source": json.RawMessage(`{"type":"inline","content":"function main(input) { return input.nodes.filter((node) => node.name > 'a' && node.name < 'z' && node.name !== '&'); }"}`),
			},
		}},
	}))
	subscriptionBody, err := fsStore.Read(ctx, "subscriptions/local/plain.json")
	require.NoError(t, err)
	require.Contains(t, string(subscriptionBody), "ss://x?left=1&right=<tag>#a>b")
	require.Contains(t, string(subscriptionBody), "node.name > 'a' && node.name < 'z' && node.name !== '&'")
	require.NotContains(t, string(subscriptionBody), `\u0026`)
	require.NotContains(t, string(subscriptionBody), `\u003c`)
	require.NotContains(t, string(subscriptionBody), `\u003e`)

}

func TestMetaStoreDeleteResources(t *testing.T) {
	ctx := context.Background()
	meta := store.NewMetaStore(store.NewFSStore(afero.NewMemMapFs()))

	sub := domain.Subscription{Name: "subscription/a", Type: domain.SubscriptionTypeLocal, Format: "uri-list", Content: "ss://x"}
	require.NoError(t, meta.PutSubscription(ctx, sub))
	require.NoError(t, meta.DeleteSubscription(ctx, "subscription/a"))
	_, err := meta.GetSubscription(ctx, "subscription/a")
	require.True(t, os.IsNotExist(err))

	file := domain.FileSpec{Name: "stored/base.yaml", Source: domain.FileSource{Type: "inline", Content: "hello: true\n"}}
	require.NoError(t, meta.PutFile(ctx, file))
	require.NoError(t, meta.DeleteFile(ctx, "stored/base.yaml"))
	_, err = meta.GetFile(ctx, "stored/base.yaml")
	require.True(t, os.IsNotExist(err))

}

func TestMetaStoreListMissingPrefixIsEmpty(t *testing.T) {
	ctx := context.Background()
	meta := store.NewMetaStore(store.NewFSStore(afero.NewMemMapFs()))

	sources, err := meta.ListSubscriptions(ctx)
	require.NoError(t, err)
	require.Empty(t, sources)
}

func TestMetaStoreRejectsMissingNames(t *testing.T) {
	ctx := context.Background()
	meta := store.NewMetaStore(store.NewFSStore(afero.NewMemMapFs()))

	require.ErrorIs(t, meta.PutSubscription(ctx, domain.Subscription{}), store.ErrInvalidKey)
	require.ErrorIs(t, meta.PutFile(ctx, domain.FileSpec{}), store.ErrInvalidKey)
}

func TestMetaStoreRejectsInvalidJSON(t *testing.T) {
	ctx := context.Background()
	fsStore := store.NewFSStore(afero.NewMemMapFs())
	meta := store.NewMetaStore(fsStore)
	require.NoError(t, fsStore.Write(ctx, "subscriptions/bad.json", []byte("{broken}")))

	_, err := meta.GetSubscription(ctx, "bad")
	require.Error(t, err)
}

func TestFSStoreReadOnlyWriteError(t *testing.T) {
	ctx := context.Background()
	st := store.NewFSStore(afero.NewReadOnlyFs(afero.NewMemMapFs()))

	err := st.Write(ctx, "x.txt", []byte("x"))
	require.Error(t, err)
}
