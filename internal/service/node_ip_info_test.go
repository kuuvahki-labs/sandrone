package service

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cachepkg "github.com/kuuvahki-labs/sandrone/internal/cache"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/iplookup"
)

type nodeIPProvider struct {
	calls []netip.Addr
	err   error
}

func (p *nodeIPProvider) Lookup(_ context.Context, ip netip.Addr) (iplookup.Attribution, error) {
	p.calls = append(p.calls, ip)
	if p.err != nil {
		return iplookup.Attribution{}, p.err
	}
	return iplookup.Attribution{
		CountryCode: "US", Country: "United States",
		ContinentCode: "NA", Continent: "North America",
		ASN: "AS64500", ASName: "Example Network", ASDomain: "example.net",
	}, nil
}

type nodeIPResolver struct {
	addresses []net.IPAddr
	calls     int
	err       error
	host      string
}

type blockingNodeIPResolver struct{}

type forbiddenNodeIPCache struct {
	t *testing.T
}

func (c forbiddenNodeIPCache) Get(context.Context, string) (cachepkg.Item, bool, error) {
	c.t.Fatal("IP attribution must not read cache")
	return cachepkg.Item{}, false, nil
}

func (c forbiddenNodeIPCache) Set(context.Context, string, []byte, time.Duration) error {
	c.t.Fatal("IP attribution must not write cache")
	return nil
}

func (c forbiddenNodeIPCache) Delete(context.Context, string) error {
	c.t.Fatal("IP attribution must not delete cache entries")
	return nil
}

func (c forbiddenNodeIPCache) Clear(context.Context) error {
	c.t.Fatal("IP attribution must not clear cache")
	return nil
}

func (blockingNodeIPResolver) LookupIPAddr(ctx context.Context, _ string) ([]net.IPAddr, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (r *nodeIPResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	r.calls++
	r.host = host
	return r.addresses, r.err
}

func TestLookupNodeIPInfoNormalizesLiteralsAndProtectsNonPublicAddresses(t *testing.T) {
	provider := &nodeIPProvider{}
	svc := New(WithIPLookupProvider(provider))
	tests := []struct {
		server    string
		wantIP    string
		version   int
		public    bool
		callDelta int
	}{
		{server: "8.8.8.8", wantIP: "8.8.8.8", version: 4, public: true, callDelta: 1},
		{server: "::ffff:8.8.8.8", wantIP: "8.8.8.8", version: 4, public: true, callDelta: 1},
		{server: "2606:4700:4700::1111", wantIP: "2606:4700:4700::1111", version: 6, public: true, callDelta: 1},
		{server: "192.168.1.2", wantIP: "192.168.1.2", version: 4, public: false},
		{server: "198.18.0.1", wantIP: "198.18.0.1", version: 4, public: false},
		{server: "203.0.113.10", wantIP: "203.0.113.10", version: 4, public: false},
	}
	wantCalls := 0
	for _, test := range tests {
		t.Run(test.server, func(t *testing.T) {
			inspection, err := svc.InspectNode(t.Context(), domain.NodeInspectRequest{
				Node: domain.NodeIR{Server: test.server}, Include: []domain.NodeInspectField{domain.NodeInspectIP},
			})

			require.NoError(t, err)
			result := inspection.IP
			require.Equal(t, test.wantIP, result.IP)
			require.Equal(t, test.version, result.IPVersion)
			require.Equal(t, test.public, result.Public)
			wantCalls += test.callDelta
			require.Len(t, provider.calls, wantCalls)
			if test.public {
				require.Equal(t, "ipwho.is", result.Source.Name)
				require.Equal(t, "AS64500", result.ASN)
			} else {
				require.Nil(t, result.Source)
				require.Empty(t, result.CountryCode)
			}
		})
	}
}

func TestLookupNodeIPInfoUsesTheFirstUniqueResolverAddress(t *testing.T) {
	provider := &nodeIPProvider{}
	resolver := &nodeIPResolver{addresses: []net.IPAddr{
		{IP: net.ParseIP("::ffff:8.8.8.8")},
		{IP: net.ParseIP("8.8.8.8")},
		{IP: net.ParseIP("1.1.1.1")},
	}}
	svc := New(WithIPLookupProvider(provider), WithIPResolver(resolver))

	request := domain.NodeInspectRequest{
		Node: domain.NodeIR{Server: "proxy.example.com"}, Include: []domain.NodeInspectField{domain.NodeInspectIP},
	}
	inspection, err := svc.InspectNode(t.Context(), request)

	require.NoError(t, err)
	result := inspection.IP
	require.Equal(t, "proxy.example.com", resolver.host)
	require.Equal(t, "8.8.8.8", result.IP)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("8.8.8.8")}, provider.calls)
	_, err = svc.InspectNode(t.Context(), request)
	require.NoError(t, err)
	require.Equal(t, 2, resolver.calls, "DNS results must not be cached")
	require.Len(t, provider.calls, 2, "attribution results must not be cached")
}

func TestLookupNodeIPInfoReturnsStableValidationAndLookupErrors(t *testing.T) {
	tests := []struct {
		name string
		svc  *Service
		req  domain.NodeInspectRequest
		code domain.ErrorCode
	}{
		{name: "empty include", svc: New(), req: domain.NodeInspectRequest{}, code: domain.CodeInvalidArgument},
		{name: "unknown include", svc: New(), req: domain.NodeInspectRequest{Include: []domain.NodeInspectField{"unknown"}}, code: domain.CodeInvalidArgument},
		{name: "duplicate include", svc: New(), req: domain.NodeInspectRequest{Include: []domain.NodeInspectField{domain.NodeInspectIP, domain.NodeInspectIP}}, code: domain.CodeInvalidArgument},
		{name: "empty server", svc: New(), req: inspectIPRequest(""), code: domain.CodeInvalidArgument},
		{name: "URL", svc: New(), req: inspectIPRequest("https://example.com"), code: domain.CodeInvalidArgument},
		{name: "host port", svc: New(), req: inspectIPRequest("example.com:443"), code: domain.CodeInvalidArgument},
		{name: "zoned IP", svc: New(), req: inspectIPRequest("fe80::1%eth0"), code: domain.CodeInvalidArgument},
		{name: "DNS error", svc: New(WithIPLookupProvider(&nodeIPProvider{}), WithIPResolver(&nodeIPResolver{err: errors.New("DNS failed")})), req: inspectIPRequest("example.com"), code: domain.CodeIPLookupFailed},
		{name: "no DNS results", svc: New(WithIPLookupProvider(&nodeIPProvider{}), WithIPResolver(&nodeIPResolver{})), req: inspectIPRequest("example.com"), code: domain.CodeIPLookupFailed},
		{name: "provider error", svc: New(WithIPLookupProvider(&nodeIPProvider{err: errors.New("upstream failed")})), req: inspectIPRequest("8.8.8.8"), code: domain.CodeIPLookupFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.svc.InspectNode(t.Context(), test.req)
			require.Error(t, err)
			require.True(t, domain.IsCode(err, test.code), err)
		})
	}
}

func TestLookupNodeIPInfoDoesNotCacheAttribution(t *testing.T) {
	provider := &nodeIPProvider{}
	svc := New(WithCache(forbiddenNodeIPCache{t: t}), WithIPLookupProvider(provider))
	req := inspectIPRequest("8.8.8.8")

	_, err := svc.InspectNode(t.Context(), req)
	require.NoError(t, err)
	_, err = svc.InspectNode(t.Context(), req)
	require.NoError(t, err)
	require.Len(t, provider.calls, 2)
}

func TestLookupNodeIPInfoMapsDNSTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	svc := New(WithIPLookupProvider(&nodeIPProvider{}), WithIPResolver(blockingNodeIPResolver{}))

	_, err := svc.InspectNode(ctx, inspectIPRequest("proxy.example.com"))

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeIPLookupFailed), err)
}

func TestInspectNodeRendersURIWithoutLookingUpIP(t *testing.T) {
	provider := &nodeIPProvider{}
	svc := New(WithIPLookupProvider(provider))

	result, err := svc.InspectNode(t.Context(), domain.NodeInspectRequest{
		Node: domain.NodeIR{
			Name: "fixture-node", Type: domain.NodeTypeTrojan, Server: "proxy.example.com", Port: 443, Password: "fixture-password",
		},
		Include: []domain.NodeInspectField{domain.NodeInspectURI},
	})

	require.NoError(t, err)
	require.Nil(t, result.IP)
	require.NotNil(t, result.URI)
	require.Contains(t, result.URI.Value, "trojan://fixture-password@proxy.example.com:443")
	require.Empty(t, provider.calls)
}

func inspectIPRequest(server string) domain.NodeInspectRequest {
	return domain.NodeInspectRequest{
		Node: domain.NodeIR{Server: server}, Include: []domain.NodeInspectField{domain.NodeInspectIP},
	}
}
