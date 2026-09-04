// Package fetcher retrieves bounded local and remote inputs for Sandrone services.
package fetcher

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const (
	DefaultTimeout  = 15 * time.Second
	DefaultMaxBytes = int64(16 << 20)
)

type Request struct {
	URL       string
	UserAgent string
	Proxy     string
	TimeoutMS int
}

type Result struct {
	Body        []byte
	StatusCode  int
	ContentHash string
	Headers     http.Header
	SourceRef   domain.SourceRef
	Warnings    []domain.Warning
}

type Fetcher struct {
	client      *http.Client
	resolver    ipResolver
	dialContext func(context.Context, string, string) (net.Conn, error)
}

type ipResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type Option func(*Fetcher)

func WithResolver(resolver ipResolver) Option {
	return func(f *Fetcher) {
		if resolver != nil {
			f.resolver = resolver
		}
	}
}

func WithDialContext(dial func(context.Context, string, string) (net.Conn, error)) Option {
	return func(f *Fetcher) {
		if dial != nil {
			f.dialContext = dial
		}
	}
}

func New(opts ...Option) *Fetcher {
	dialer := &net.Dialer{}
	f := &Fetcher{
		client:      &http.Client{},
		resolver:    net.DefaultResolver,
		dialContext: dialer.DialContext,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

func (f *Fetcher) Fetch(ctx context.Context, req Request) (*Result, error) {
	return f.fetch(ctx, req, false)
}

func (f *Fetcher) FetchPublic(ctx context.Context, req Request) (*Result, error) {
	return f.fetch(ctx, req, true)
}

func (f *Fetcher) fetch(ctx context.Context, req Request, publicOnly bool) (*Result, error) {
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "remote url is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, "parse remote url", err)
	}
	switch parsed.Scheme {
	case "http", "https":
	default:
		return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported remote scheme %q", parsed.Scheme))
	}
	if parsed.Hostname() == "" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "remote url host is required")
	}
	timeout := DefaultTimeout
	if req.TimeoutMS > 0 {
		timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	}
	client, err := f.clientFor(req.Proxy, timeout, publicOnly)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, "build remote request", err)
	}
	ua := strings.TrimSpace(req.UserAgent)
	if ua == "" {
		ua = buildinfo.UserAgent()
	}
	httpReq.Header.Set("User-Agent", ua)
	resp, err := client.Do(httpReq)
	if err != nil {
		if domain.IsCode(err, domain.CodeInvalidArgument) {
			return nil, err
		}
		return nil, domain.WrapError(domain.CodeFileInputNotFound, "fetch remote input", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &domain.AppError{
			Code:    domain.CodeFileInputNotFound,
			Message: fmt.Sprintf("remote returned status %d", resp.StatusCode),
			Source:  rawURL,
		}
	}
	limited := io.LimitReader(resp.Body, DefaultMaxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, domain.WrapError(domain.CodeFileInputNotFound, "read remote input", err)
	}
	if int64(len(body)) > DefaultMaxBytes {
		return nil, &domain.AppError{
			Code:    domain.CodeFileInputNotFound,
			Message: fmt.Sprintf("remote response exceeds %d bytes", DefaultMaxBytes),
			Source:  rawURL,
		}
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	return &Result{
		Body:        body,
		StatusCode:  resp.StatusCode,
		ContentHash: hash,
		Headers:     resp.Header.Clone(),
		SourceRef: domain.SourceRef{
			Kind: "remote",
			Name: rawURL,
			URL:  rawURL,
			Note: fmt.Sprintf("status=%d sha256=%s", resp.StatusCode, hash),
		},
	}, nil
}

func (f *Fetcher) clientFor(proxyURL string, timeout time.Duration, publicOnly bool) (*http.Client, error) {
	if publicOnly {
		if proxyURL != "" {
			return nil, domain.NewError(domain.CodeInvalidArgument, "public remote fetch does not support a proxy")
		}
		baseTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, domain.NewError(domain.CodeInvalidArgument, "default HTTP transport is not configurable")
		}
		transport := baseTransport.Clone()
		transport.Proxy = nil
		transport.DialContext = f.publicDialContext
		return &http.Client{
			Transport: transport,
			Timeout:   timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return domain.NewError(domain.CodeFileInputNotFound, "too many remote redirects")
				}
				if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
					return domain.NewError(domain.CodeInvalidArgument, "remote redirect must use http or https")
				}
				if req.URL.Hostname() == "" {
					return domain.NewError(domain.CodeInvalidArgument, "remote redirect host is required")
				}
				return nil
			},
		}, nil
	}
	if proxyURL == "" {
		client := *f.client
		client.Timeout = timeout
		return &client, nil
	}
	parsedProxy, err := url.Parse(proxyURL)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, "parse remote proxy url", err)
	}
	switch parsedProxy.Scheme {
	case "http", "https", "socks5":
	default:
		return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported remote proxy scheme %q", parsedProxy.Scheme))
	}
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, domain.NewError(domain.CodeInvalidArgument, "default HTTP transport is not configurable")
	}
	transport := baseTransport.Clone()
	transport.Proxy = http.ProxyURL(parsedProxy)
	if parsedProxy.Scheme == "https" {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func (f *Fetcher) publicDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, "parse remote address", err)
	}
	addresses, err := f.resolvePublicAddresses(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, ip := range addresses {
		conn, dialErr := f.dialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, domain.WrapError(domain.CodeFileInputNotFound, "connect remote input", lastErr)
}

func (f *Fetcher) resolvePublicAddresses(ctx context.Context, host string) ([]netip.Addr, error) {
	if parsed, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		parsed = parsed.Unmap()
		if !IsPublicAddress(parsed) {
			return nil, domain.NewError(domain.CodeInvalidArgument, "remote address must be public")
		}
		return []netip.Addr{parsed}, nil
	}
	resolved, err := f.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, domain.WrapError(domain.CodeFileInputNotFound, "resolve remote host", err)
	}
	if len(resolved) == 0 {
		return nil, domain.NewError(domain.CodeFileInputNotFound, "remote host resolved to no addresses")
	}
	addresses := make([]netip.Addr, 0, len(resolved))
	for _, candidate := range resolved {
		ip, ok := netip.AddrFromSlice(candidate.IP)
		if !ok {
			return nil, domain.NewError(domain.CodeInvalidArgument, "remote host resolved to an invalid address")
		}
		ip = ip.Unmap()
		if !IsPublicAddress(ip) {
			return nil, domain.NewError(domain.CodeInvalidArgument, "remote address must be public")
		}
		addresses = append(addresses, ip)
	}
	return addresses, nil
}

var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

// IsPublicAddress reports whether ip may be sent to an external network
// service. It rejects private, reserved, documentation, transition, and
// Mihomo fake-IP ranges in addition to non-global-unicast addresses.
func IsPublicAddress(ip netip.Addr) bool {
	if !ip.IsValid() || !ip.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
