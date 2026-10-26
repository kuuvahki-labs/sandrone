package fetcher

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchPublicRejectsNonPublicIPAddresses(t *testing.T) {
	f := New(WithDialContext(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("unexpected dial")
	}))
	tests := []string{
		"127.0.0.1",
		"10.0.0.1",
		"100.64.0.1",
		"169.254.1.1",
		"172.16.0.1",
		"192.168.0.1",
		"192.88.99.1",
		"[::1]",
		"[64:ff9b::7f00:1]",
		"[2002:7f00:1::]",
		"[fc00::1]",
		"[fe80::1]",
	}
	for _, host := range tests {
		t.Run(host, func(t *testing.T) {
			_, err := f.FetchPublic(context.Background(), Request{URL: "http://" + host + "/subscription"})
			require.Error(t, err)
			require.Contains(t, err.Error(), "public")
		})
	}
}

func TestFetchPublicAllowsFakeIPAddresses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("remote-body"))
	}))
	defer server.Close()

	for _, ip := range []string{"198.18.0.0", "198.18.0.1", "198.19.0.1", "198.19.255.255"} {
		for _, viaDNS := range []bool{false, true} {
			host := ip
			if viaDNS {
				host = "subscription.example"
			}
			t.Run(ip+"/"+host, func(t *testing.T) {
				resolver := &stubResolver{addresses: []net.IPAddr{{IP: net.ParseIP(ip)}}}
				var dialAddress string
				f := New(
					WithResolver(resolver),
					WithDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
						dialAddress = address
						return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
					}),
				)

				result, err := f.FetchPublic(t.Context(), Request{URL: "http://" + host + "/sub"})

				require.NoError(t, err)
				require.Equal(t, []byte("remote-body"), result.Body)
				require.Equal(t, net.JoinHostPort(ip, "80"), dialAddress)
				if viaDNS {
					require.Equal(t, 1, resolver.calls)
				} else {
					require.Zero(t, resolver.calls)
				}
			})
		}
	}
}

func TestFetchPublicRejectsDNSResolvingToPrivateAddress(t *testing.T) {
	resolver := &stubResolver{addresses: []net.IPAddr{{IP: net.ParseIP("10.0.0.8")}}}
	f := New(WithResolver(resolver))

	_, err := f.FetchPublic(context.Background(), Request{URL: "https://subscription.example/sub"})

	require.Error(t, err)
	require.Equal(t, 1, resolver.calls)
}

func TestFetchPublicPinsValidatedDNSResultForConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("remote-body"))
	}))
	defer server.Close()

	resolver := &stubResolver{
		addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}},
		next:      []net.IPAddr{{IP: net.ParseIP("10.0.0.8")}},
	}
	var dialAddress string
	f := New(
		WithResolver(resolver),
		WithDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
			dialAddress = address
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		}),
	)

	result, err := f.FetchPublic(context.Background(), Request{URL: "http://subscription.example/sub"})

	require.NoError(t, err)
	require.Equal(t, []byte("remote-body"), result.Body)
	require.Equal(t, 1, resolver.calls)
	require.True(t, strings.HasPrefix(dialAddress, "93.184.216.34:"))
}

func TestFetchPublicRevalidatesRedirectTargets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1/private", http.StatusFound)
	}))
	defer server.Close()

	f := New(
		WithResolver(&stubResolver{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}),
		WithDialContext(func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		}),
	)

	_, err := f.FetchPublic(context.Background(), Request{URL: "http://subscription.example/redirect"})

	require.Error(t, err)
	require.Contains(t, err.Error(), "public")
}

type stubResolver struct {
	addresses []net.IPAddr
	next      []net.IPAddr
	calls     int
}

func (r *stubResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	r.calls++
	if r.calls > 1 && r.next != nil {
		return r.next, nil
	}
	return r.addresses, nil
}
