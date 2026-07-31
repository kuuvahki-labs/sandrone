//go:build probe_mihomo || probe_singbox

package probe

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type dialerFunc func(context.Context, string, string) (net.Conn, error)

func (f dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func TestParseURLTestTarget(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw         string
		wantAddress string
		wantErr     bool
	}{
		{raw: "http://example.com/path", wantAddress: "example.com:80"},
		{raw: "https://example.com:8443/path", wantAddress: "example.com:8443"},
		{raw: "http://[2001:db8::1]/", wantAddress: "[2001:db8::1]:80"},
		{raw: "ftp://example.com/file", wantErr: true},
		{raw: "http:///missing-host", wantErr: true},
		{raw: "://bad-url", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			target, err := parseURLTestTarget(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.raw, target.raw)
			require.Equal(t, tt.wantAddress, target.address)
		})
	}
}

func TestRunURLTestMatchesExpectedStatus(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	methods := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		methods <- request.Method
		requests.Add(1)
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	target := mustParseURLTestTarget(t, server.URL)
	dialer, record := recordingDialer()
	matcher := mustParseExpectedStatus(t, "200-299")

	_, err := runURLTest(context.Background(), target, urlTestOptions{
		dialer:         dialer,
		expectedStatus: matcher,
	})

	require.NoError(t, err)
	require.Equal(t, http.MethodHead, <-methods)
	require.Equal(t, int32(1), requests.Load())
	network, address := record.values()
	require.Equal(t, "tcp", network)
	require.Equal(t, server.Listener.Addr().String(), address)
}

func TestRunURLTestRejectsUnexpectedStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	target := mustParseURLTestTarget(t, server.URL)
	matcher := mustParseExpectedStatus(t, "200-299")

	_, err := runURLTest(context.Background(), target, urlTestOptions{
		dialer:         dialerFunc((&net.Dialer{}).DialContext),
		expectedStatus: matcher,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "503")
	require.ErrorContains(t, err, "200-299")
}

func TestRunURLTestDoesNotFollowRedirect(t *testing.T) {
	t.Parallel()
	var destinationRequests atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		destinationRequests.Add(1)
	}))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Location", destination.URL)
		response.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	target := mustParseURLTestTarget(t, server.URL)
	dialer, record := recordingDialer()
	matcher := mustParseExpectedStatus(t, "302")

	_, err := runURLTest(context.Background(), target, urlTestOptions{
		dialer:         dialer,
		expectedStatus: matcher,
	})

	require.NoError(t, err)
	require.Zero(t, destinationRequests.Load())
	network, address := record.values()
	require.Equal(t, "tcp", network)
	require.Equal(t, server.Listener.Addr().String(), address)
}

func TestRunURLTestSupportsHTTPS(t *testing.T) {
	t.Parallel()
	methods := make(chan string, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		methods <- request.Method
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	target := mustParseURLTestTarget(t, server.URL)
	dialer, record := recordingDialer()
	matcher := mustParseExpectedStatus(t, "204")
	tlsConfig := server.Client().Transport.(*http.Transport).TLSClientConfig

	_, err := runURLTest(context.Background(), target, urlTestOptions{
		dialer:          dialer,
		expectedStatus:  matcher,
		tlsClientConfig: tlsConfig,
	})

	require.NoError(t, err)
	require.Equal(t, http.MethodHead, <-methods)
	network, address := record.values()
	require.Equal(t, "tcp", network)
	require.Equal(t, server.Listener.Addr().String(), address)
}

func TestRunURLTestUsesAttemptContext(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()

	target := mustParseURLTestTarget(t, server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	_, err := runURLTest(ctx, target, urlTestOptions{
		dialer:         dialerFunc((&net.Dialer{}).DialContext),
		expectedStatus: mustParseExpectedStatus(t, "204"),
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestRunURLTestReturnsDialError(t *testing.T) {
	t.Parallel()
	dialErr := errors.New("dial failed")
	target := urlTestTarget{
		raw:     "http://example.com/",
		address: "example.com:80",
	}

	_, err := runURLTest(context.Background(), target, urlTestOptions{
		dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
			return nil, dialErr
		}),
	})

	require.ErrorIs(t, err, dialErr)
}

func TestRunURLTestClosesConnection(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	target := mustParseURLTestTarget(t, server.URL)
	closed := make(chan struct{})
	delegate := &net.Dialer{}
	dialer := dialerFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := delegate.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &closeTrackingConn{Conn: conn, closed: closed}, nil
	})

	_, err := runURLTest(context.Background(), target, urlTestOptions{
		dialer:         dialer,
		expectedStatus: mustParseExpectedStatus(t, "204"),
	})

	require.NoError(t, err)
	select {
	case <-closed:
	default:
		t.Fatal("connection was not closed before runURLTest returned")
	}
}

func TestRunURLTestAppliesTimingPolicy(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	target := mustParseURLTestTarget(t, server.URL)
	matcher := mustParseExpectedStatus(t, "204")

	tests := []struct {
		name                string
		times               []time.Time
		resetStartAfterDial func(net.Conn) bool
		want                time.Duration
	}{
		{
			name:                "reset after dial",
			times:               []time.Time{time.UnixMilli(0), time.UnixMilli(40), time.UnixMilli(45)},
			resetStartAfterDial: func(net.Conn) bool { return true },
			want:                5 * time.Millisecond,
		},
		{
			name:  "keep pre-dial start",
			times: []time.Time{time.UnixMilli(0), time.UnixMilli(45)},
			want:  45 * time.Millisecond,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialer, record := recordingDialer()
			nowCalls := 0
			now := func() time.Time {
				require.Less(t, nowCalls, len(tt.times))
				value := tt.times[nowCalls]
				nowCalls++
				return value
			}

			duration, err := runURLTest(context.Background(), target, urlTestOptions{
				dialer:              dialer,
				expectedStatus:      matcher,
				resetStartAfterDial: tt.resetStartAfterDial,
				now:                 now,
			})

			require.NoError(t, err)
			require.Equal(t, tt.want, duration)
			require.Equal(t, len(tt.times), nowCalls)
			network, address := record.values()
			require.Equal(t, "tcp", network)
			require.Equal(t, server.Listener.Addr().String(), address)
		})
	}
}

type closeTrackingConn struct {
	net.Conn
	once   sync.Once
	closed chan struct{}
}

func (c *closeTrackingConn) Close() error {
	c.once.Do(func() {
		close(c.closed)
	})
	return c.Conn.Close()
}

func mustParseURLTestTarget(t *testing.T, rawURL string) urlTestTarget {
	t.Helper()
	target, err := parseURLTestTarget(rawURL)
	require.NoError(t, err)
	return target
}

func mustParseExpectedStatus(t *testing.T, raw string) expectedStatusMatcher {
	t.Helper()
	matcher, err := parseExpectedStatus(raw)
	require.NoError(t, err)
	return matcher
}

type dialRecord struct {
	mu      sync.Mutex
	network string
	address string
}

func (r *dialRecord) values() (string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.network, r.address
}

func recordingDialer() (Dialer, *dialRecord) {
	record := &dialRecord{}
	delegate := &net.Dialer{}
	return dialerFunc(func(ctx context.Context, requestedNetwork, requestedAddress string) (net.Conn, error) {
		record.mu.Lock()
		record.network = requestedNetwork
		record.address = requestedAddress
		record.mu.Unlock()
		return delegate.DialContext(ctx, requestedNetwork, requestedAddress)
	}), record
}
