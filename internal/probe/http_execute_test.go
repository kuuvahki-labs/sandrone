//go:build probe_singbox || probe_mihomo

package probe

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestNodeHTTPInitializationDeadlineAndLateCleanup(t *testing.T) {
	release := make(chan struct{})
	closed := make(chan struct{})
	ctxSeen := make(chan context.Context, 1)
	opts := backendHTTPOptions("sing-box")
	opts.TimeoutMS = 20
	results, err := executeNodeHTTP(t.Context(), []domain.NodeIR{{Name: "node"}}, opts, nil, func(ctx context.Context, _ int, _ domain.NodeIR) (nodeHTTPRoute, error) {
		ctxSeen <- ctx
		<-release
		return nodeHTTPRoute{dialer: &net.Dialer{}, close: func() error { close(closed); return nil }}, nil
	})
	// Release even if assertions fail so the intentionally blocked initializer
	// cannot survive the test. It must still close its late result.
	close(release)
	require.NoError(t, err)
	require.Contains(t, results[0].Error, "deadline exceeded")
	require.GreaterOrEqual(t, results[0].DurationMS, int64(15))
	require.ErrorIs(t, (<-ctxSeen).Err(), context.DeadlineExceeded)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("late core result was not closed")
	}
}

func TestNodeHTTPInitializationCancellationIsFatal(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := executeNodeHTTP(ctx, []domain.NodeIR{{Name: "node"}}, backendHTTPOptions("sing-box"), nil, func(requestCtx context.Context, _ int, _ domain.NodeIR) (nodeHTTPRoute, error) {
			close(started)
			<-requestCtx.Done()
			return nodeHTTPRoute{}, requestCtx.Err()
		})
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("initialization ignored request cancellation")
	}
}

func TestNodeHTTPInitializationPanicIsNodeFailure(t *testing.T) {
	results, err := executeNodeHTTP(t.Context(), []domain.NodeIR{{Name: "node"}}, backendHTTPOptions("sing-box"), nil, func(context.Context, int, domain.NodeIR) (nodeHTTPRoute, error) {
		panic("private-core-detail")
	})
	require.NoError(t, err)
	require.Equal(t, "HTTP core initialization panicked", results[0].Error)
}

func TestNodeHTTPInitializationTimeoutRetainsConcurrencyUntilCleanup(t *testing.T) {
	releaseInit := make(chan struct{})
	releaseClose := make(chan struct{})
	finishInit := sync.OnceFunc(func() { close(releaseInit) })
	finishClose := sync.OnceFunc(func() { close(releaseClose) })
	defer finishInit()
	defer finishClose()
	firstContext := make(chan context.Context, 1)
	firstIndex := make(chan int, 1)
	closeStarted := make(chan struct{})
	secondStarted := make(chan int, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "healthy") }))
	defer server.Close()
	opts := backendHTTPOptions("sing-box")
	opts.URL, opts.Concurrency, opts.TimeoutMS = server.URL, 1, 100
	var calls atomic.Int32
	type outcome struct {
		results []domain.NodeHTTPResult
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		results, err := executeNodeHTTP(t.Context(), []domain.NodeIR{{Name: "one"}, {Name: "two"}}, opts, nil, func(ctx context.Context, index int, _ domain.NodeIR) (nodeHTTPRoute, error) {
			if calls.Add(1) == 1 {
				firstIndex <- index
				firstContext <- ctx
				<-releaseInit
				return nodeHTTPRoute{close: func() error {
					close(closeStarted)
					<-releaseClose
					return nil
				}}, nil
			}
			secondStarted <- index
			return nodeHTTPRoute{dialer: &net.Dialer{}}, nil
		})
		done <- outcome{results, err}
	}()
	<-((<-firstContext).Done())
	select {
	case <-secondStarted:
		t.Fatal("next node initialized while timed-out initialization still owned capacity")
	case <-time.After(20 * time.Millisecond):
	}
	finishInit()
	<-closeStarted
	select {
	case <-secondStarted:
		t.Fatal("next node initialized before late core cleanup completed")
	case <-time.After(20 * time.Millisecond):
	}
	finishClose()
	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.Contains(t, result.results[<-firstIndex].Error, "deadline exceeded")
		healthy := result.results[<-secondStarted]
		require.Empty(t, healthy.Error)
		require.Equal(t, "healthy", healthy.Body)
	case <-time.After(time.Second):
		t.Fatal("next node did not resume after cleanup released capacity")
	}
}

func TestNodeHTTPParentCancellationDoesNotWaitForLateInitialization(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	releaseInit := make(chan struct{})
	finishInit := sync.OnceFunc(func() { close(releaseInit) })
	defer finishInit()
	started := make(chan context.Context, 2)
	closed := make(chan struct{}, 2)
	done := make(chan error, 1)
	opts := backendHTTPOptions("sing-box")
	opts.Concurrency, opts.TimeoutMS = 1, 20
	go func() {
		_, err := executeNodeHTTP(ctx, []domain.NodeIR{{Name: "one"}, {Name: "two"}}, opts, nil, func(requestCtx context.Context, _ int, _ domain.NodeIR) (nodeHTTPRoute, error) {
			started <- requestCtx
			<-releaseInit
			return nodeHTTPRoute{close: func() error { closed <- struct{}{}; return nil }}, nil
		})
		done <- err
	}()
	<-((<-started).Done())
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("parent cancellation waited for a stuck initializer or queued worker")
	}
	select {
	case <-started:
		t.Fatal("queued node initialized while the first lease was retained")
	default:
	}
	finishInit()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("late initialization result was not closed after parent cancellation")
	}
}

func TestPrepareNodeHTTPRouteCancellationReleasesLeaseAfterCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	allowInit := make(chan struct{})
	block := make(chan struct{})
	closeStarted := make(chan struct{})
	finish := sync.OnceFunc(func() { close(block) })
	defer finish()
	defer cancel()
	released := make(chan struct{})
	cancel()
	_, err := prepareNodeHTTPRoute(ctx, sync.OnceFunc(func() { close(released) }), func() (nodeHTTPRoute, error) {
		<-allowInit
		return nodeHTTPRoute{close: func() error { close(closeStarted); <-block; return nil }}, nil
	})
	close(allowInit)
	require.ErrorIs(t, err, context.Canceled)
	<-closeStarted
	select {
	case <-released:
		t.Fatal("capacity was released before cleanup completed")
	default:
	}
	finish()
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("late cleanup did not release capacity")
	}
}
