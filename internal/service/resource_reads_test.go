package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

type countedResourceStore struct {
	store.Store
	mu    sync.Mutex
	reads map[string]int
	fail  error
}

func (s *countedResourceStore) Read(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads[key]++
	if s.fail != nil {
		err := s.fail
		s.fail = nil
		return nil, err
	}
	return s.Store.Read(ctx, key)
}

func TestResourceReadsSnapshotHitAndNextRequest(t *testing.T) {
	ctx := t.Context()
	st := &countedResourceStore{Store: store.NewFSStore(afero.NewMemMapFs()), reads: map[string]int{}}
	svc := New(WithStore(st))
	sub := domain.Subscription{
		Name: "example", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:example@example.com:8388#one", SnapshotTTLSeconds: new(60),
	}
	require.NoError(t, svc.PutSubscription(ctx, sub))
	_, err := svc.PreviewSubscription(ctx, sub.Name)
	require.NoError(t, err)
	require.Equal(t, 1, st.reads["subscriptions/example.json"])
	clear(st.reads)
	preview, err := svc.PreviewSubscription(ctx, sub.Name)
	require.NoError(t, err)
	require.Equal(t, snapshotCacheStatusHit, preview.SnapshotCacheStatus)
	require.Equal(t, map[string]int{
		"subscriptions/example.json":                             1,
		"cache/subscription_snapshot/subscriptions/example.json": 1,
	}, st.reads)

	// An independent writer must be visible to the next execution.
	sub.Content = "ss://aes-128-gcm:example@example.com:8388#two"
	require.NoError(t, store.NewMetaStore(st).PutSubscription(ctx, sub))
	preview, err = svc.PreviewSubscription(ctx, sub.Name)
	require.NoError(t, err)
	require.Equal(t, snapshotCacheStatusMiss, preview.SnapshotCacheStatus)
	require.Equal(t, "two", preview.Nodes[0].After.Name)
}

func TestResourceReadScopeIsolationAndInvalidation(t *testing.T) {
	st := &countedResourceStore{Store: store.NewFSStore(afero.NewMemMapFs()), reads: map[string]int{}}
	svc := New(WithStore(st))
	ctx := svc.withResourceReads(t.Context())
	sub := domain.Subscription{
		Name: "example", Type: domain.SubscriptionTypeRemote,
		Remote: &domain.RemoteInput{URL: "https://example.com/sub"},
		Meta:   map[string]string{"value": "original"},
	}
	require.NoError(t, svc.PutSubscription(ctx, sub))
	first, err := svc.loadSubscription(ctx, sub.Name)
	require.NoError(t, err)
	first.Meta["value"] = "changed"
	first.Remote.URL = "https://example.com/changed"
	second, err := svc.loadSubscription(ctx, sub.Name)
	require.NoError(t, err)
	require.Equal(t, "original", second.Meta["value"])
	require.Equal(t, sub.Remote.URL, second.Remote.URL)
	require.Equal(t, 1, st.reads["subscriptions/example.json"])
	wantRevision, err := cacheIdentity(second)
	require.NoError(t, err)
	revision, err := svc.cacheResourceRevision(ctx, domain.ResourceRef{Kind: "subscription", Name: sub.Name})
	require.NoError(t, err)
	require.Equal(t, wantRevision, revision)
	require.Equal(t, 1, st.reads["subscriptions/example.json"])

	sub.Meta["value"] = "saved"
	require.NoError(t, svc.PutSubscription(ctx, sub))
	updated, err := svc.loadSubscription(ctx, sub.Name)
	require.NoError(t, err)
	require.Equal(t, "saved", updated.Meta["value"])
	require.Equal(t, 2, st.reads["subscriptions/example.json"])
	require.NoError(t, svc.DeleteSubscription(ctx, sub.Name))
	_, err = svc.loadSubscription(ctx, sub.Name)
	require.Error(t, err)

	other := New(WithFS(afero.NewMemMapFs()))
	sub.Meta["value"] = "other store"
	require.NoError(t, other.PutSubscription(t.Context(), sub))
	value, err := other.loadSubscription(other.withResourceReads(ctx), sub.Name)
	require.NoError(t, err)
	require.Equal(t, "other store", value.Meta["value"])
}

func TestResourceReadsDoNotCacheErrorsAndCloneFileConfig(t *testing.T) {
	st := &countedResourceStore{Store: store.NewFSStore(afero.NewMemMapFs()), reads: map[string]int{}}
	svc := New(WithStore(st))
	ctx := svc.withResourceReads(t.Context())
	require.NoError(t, st.Write(ctx, "files/example.json", []byte(`{"name":"example","kind":"static","source":{"type":"inline","content":"example"},"meta":{"value":"original"}}`)))
	injected := errors.New("transient read failure")
	st.fail = injected
	_, err := svc.loadFileDefinition(ctx, "example")
	require.ErrorIs(t, err, injected)
	file, err := svc.loadFileDefinition(ctx, "example")
	require.NoError(t, err)
	file.Meta["value"] = "changed"
	file, err = svc.loadFileDefinition(ctx, "example")
	require.NoError(t, err)
	require.Equal(t, "original", file.Meta["value"])
	require.Equal(t, 2, st.reads["files/example.json"])
}

func TestRestoreInvalidatesRequestResourceDefinitions(t *testing.T) {
	svc := New(WithFS(afero.NewMemMapFs()))
	ctx := svc.withResourceReads(t.Context())
	file := domain.FileSpec{Name: "example", Kind: domain.FileKindStatic, Source: domain.FileSource{Type: "inline", Content: "restored"}}
	require.NoError(t, svc.PutFile(ctx, file))
	backup, err := svc.ExportBackup(ctx)
	require.NoError(t, err)
	file.Source.Content = "before restore"
	require.NoError(t, svc.PutFile(ctx, file))
	before, err := svc.loadFileDefinition(ctx, file.Name)
	require.NoError(t, err)
	require.Equal(t, "before restore", before.Source.Content)
	require.NoError(t, svc.RestoreBackup(ctx, backup.Body))
	after, err := svc.loadFileDefinition(ctx, file.Name)
	require.NoError(t, err)
	require.Equal(t, "restored", after.Source.Content)
}
