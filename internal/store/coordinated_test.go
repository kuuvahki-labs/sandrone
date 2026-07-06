package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/store"
)

const coordinationTimeout = time.Second

func TestCoordinatorUpdateWaitsForView(t *testing.T) {
	t.Parallel()

	coordinator := store.Coordinate(store.NewFSStore(afero.NewMemMapFs()))
	viewEntered := make(chan struct{})
	releaseView := make(chan struct{})
	viewDone := make(chan error, 1)
	go func() {
		viewDone <- coordinator.View(context.Background(), func(store.Store) error {
			close(viewEntered)
			<-releaseView
			return nil
		})
	}()
	requireSignal(t, viewEntered, "view callback did not start")

	updateAttempted := make(chan struct{})
	updateEntered := make(chan struct{})
	updateDone := make(chan error, 1)
	go func() {
		close(updateAttempted)
		updateDone <- coordinator.Update(context.Background(), func(store.Store) error {
			close(updateEntered)
			return nil
		})
	}()
	requireSignal(t, updateAttempted, "update was not attempted")
	requireBlocked(t, updateEntered, "update entered while a view was active")

	close(releaseView)
	require.NoError(t, requireResult(t, viewDone, "view did not finish"))
	requireSignal(t, updateEntered, "update did not enter after the view finished")
	require.NoError(t, requireResult(t, updateDone, "update did not finish"))
}

func TestCoordinatorReadWaitsForUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rawStore := store.NewFSStore(afero.NewMemMapFs())
	require.NoError(t, rawStore.Write(ctx, "state/value", []byte("ready")))
	coordinator := store.Coordinate(rawStore)
	updateEntered := make(chan struct{})
	releaseUpdate := make(chan struct{})
	updateDone := make(chan error, 1)
	go func() {
		updateDone <- coordinator.Update(ctx, func(store.Store) error {
			close(updateEntered)
			<-releaseUpdate
			return nil
		})
	}()
	requireSignal(t, updateEntered, "update callback did not start")

	readAttempted := make(chan struct{})
	readDone := make(chan error, 1)
	go func() {
		close(readAttempted)
		body, err := coordinator.Read(ctx, "state/value")
		if err == nil && string(body) != "ready" {
			err = fmt.Errorf("read returned %q, want ready", body)
		}
		readDone <- err
	}()
	requireSignal(t, readAttempted, "read was not attempted")
	requireBlocked(t, readDone, "read finished while an update was active")

	close(releaseUpdate)
	require.NoError(t, requireResult(t, updateDone, "update did not finish"))
	require.NoError(t, requireResult(t, readDone, "read did not finish"))
}

func TestCoordinatorCallbacksUseRawStoreWithoutReentrantDeadlock(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rawStore := store.NewFSStore(afero.NewMemMapFs())
	coordinator := store.Coordinate(rawStore)
	require.Same(t, coordinator, store.Coordinate(coordinator))

	updateDone := make(chan error, 1)
	go func() {
		updateDone <- coordinator.Update(ctx, func(callbackStore store.Store) error {
			if callbackStore != rawStore {
				return fmt.Errorf("update received coordinated store instead of raw store")
			}
			return callbackStore.Write(ctx, "state/value", []byte("updated"))
		})
	}()
	require.NoError(t, requireResult(t, updateDone, "callback store write deadlocked"))

	viewDone := make(chan error, 1)
	go func() {
		viewDone <- coordinator.View(ctx, func(callbackStore store.Store) error {
			if callbackStore != rawStore {
				return fmt.Errorf("view received coordinated store instead of raw store")
			}
			body, err := callbackStore.Read(ctx, "state/value")
			if err != nil {
				return err
			}
			if string(body) != "updated" {
				return fmt.Errorf("read returned %q, want updated", body)
			}
			return nil
		})
	}()
	require.NoError(t, requireResult(t, viewDone, "callback store read deadlocked"))
}

func TestMetaStorePutFileCannotBeViewedBetweenBodyAndMetadataWrites(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	bodyWritten := make(chan struct{})
	releaseBodyWrite := make(chan struct{})
	rawStore := &writeGateStore{
		Store:   store.NewFSStore(afero.NewMemMapFs()),
		gateKey: "files/atomic.txt",
		written: bodyWritten,
		release: releaseBodyWrite,
	}
	coordinator := store.Coordinate(rawStore)
	metaStore := store.NewMetaStore(coordinator)

	putDone := make(chan error, 1)
	go func() {
		putDone <- metaStore.PutFile(ctx, domain.FileSpec{
			Name:   "atomic.txt",
			Source: domain.FileSource{Type: "inline", Content: "complete"},
		})
	}()
	requireSignal(t, bodyWritten, "file body was not written")

	viewAttempted := make(chan struct{})
	viewEntered := make(chan struct{})
	viewDone := make(chan error, 1)
	go func() {
		close(viewAttempted)
		viewDone <- coordinator.View(ctx, func(callbackStore store.Store) error {
			close(viewEntered)
			if _, err := callbackStore.Read(ctx, "files/atomic.txt"); err != nil {
				return fmt.Errorf("read body: %w", err)
			}
			if _, err := callbackStore.Read(ctx, "files/atomic.txt.json"); err != nil {
				return fmt.Errorf("read metadata: %w", err)
			}
			return nil
		})
	}()
	requireSignal(t, viewAttempted, "view was not attempted")
	requireBlocked(t, viewEntered, "view observed the file after its body write but before its metadata write")

	close(releaseBodyWrite)
	require.NoError(t, requireResult(t, putDone, "PutFile did not finish"))
	requireSignal(t, viewEntered, "view did not enter after PutFile finished")
	require.NoError(t, requireResult(t, viewDone, "view did not finish"))
}

func TestMetaStoreDeleteFileCannotBeViewedBetweenMetadataAndBodyDeletes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	baseStore := store.NewFSStore(afero.NewMemMapFs())
	require.NoError(t, store.NewMetaStore(baseStore).PutFile(ctx, domain.FileSpec{
		Name:   "atomic.txt",
		Source: domain.FileSource{Type: "inline", Content: "complete"},
	}))
	metadataDeleted := make(chan struct{})
	releaseMetadataDelete := make(chan struct{})
	rawStore := &deleteGateStore{
		Store:   baseStore,
		gateKey: "files/atomic.txt.json",
		deleted: metadataDeleted,
		release: releaseMetadataDelete,
	}
	coordinator := store.Coordinate(rawStore)
	metaStore := store.NewMetaStore(coordinator)

	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- metaStore.DeleteFile(ctx, "atomic.txt")
	}()
	requireSignal(t, metadataDeleted, "file metadata was not deleted")

	viewAttempted := make(chan struct{})
	viewEntered := make(chan struct{})
	viewDone := make(chan error, 1)
	go func() {
		close(viewAttempted)
		viewDone <- coordinator.View(ctx, func(callbackStore store.Store) error {
			close(viewEntered)
			for _, key := range []string{"files/atomic.txt.json", "files/atomic.txt"} {
				if _, err := callbackStore.Read(ctx, key); !os.IsNotExist(err) {
					return fmt.Errorf("Read(%q) error = %v, want not exist", key, err)
				}
			}
			return nil
		})
	}()
	requireSignal(t, viewAttempted, "view was not attempted")
	requireBlocked(t, viewEntered, "view observed the file after its metadata delete but before its body delete")

	close(releaseMetadataDelete)
	require.NoError(t, requireResult(t, deleteDone, "DeleteFile did not finish"))
	requireSignal(t, viewEntered, "view did not enter after DeleteFile finished")
	require.NoError(t, requireResult(t, viewDone, "view did not finish"))
}

type writeGateStore struct {
	store.Store
	gateKey string
	written chan<- struct{}
	release <-chan struct{}
}

func (s *writeGateStore) Write(ctx context.Context, key string, value []byte) error {
	if err := s.Store.Write(ctx, key, value); err != nil {
		return err
	}
	if key == s.gateKey {
		close(s.written)
		<-s.release
	}
	return nil
}

type deleteGateStore struct {
	store.Store
	gateKey string
	deleted chan<- struct{}
	release <-chan struct{}
}

func (s *deleteGateStore) Delete(ctx context.Context, key string) error {
	if err := s.Store.Delete(ctx, key); err != nil {
		return err
	}
	if key == s.gateKey {
		close(s.deleted)
		<-s.release
	}
	return nil
}

func requireSignal(t *testing.T, ch <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(coordinationTimeout):
		t.Fatal(message)
	}
}

func requireBlocked[T any](t *testing.T, ch <-chan T, message string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal(message)
	case <-time.After(50 * time.Millisecond):
	}
}

func requireResult[T any](t *testing.T, ch <-chan T, message string) T {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(coordinationTimeout):
		t.Fatal(message)
		var zero T
		return zero
	}
}
