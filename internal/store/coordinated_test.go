package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

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
