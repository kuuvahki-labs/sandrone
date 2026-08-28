//go:build probe_mihomo || probe_singbox

package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
)

func parseURLTestTarget(rawURL string) (urlTestTarget, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return urlTestTarget{}, err
	}
	if parsed.Hostname() == "" {
		return urlTestTarget{}, fmt.Errorf("missing host")
	}
	var defaultPort string
	switch parsed.Scheme {
	case "http":
		defaultPort = "80"
	case "https":
		defaultPort = "443"
	default:
		return urlTestTarget{}, fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	port := parsed.Port()
	if port == "" {
		port = defaultPort
	} else if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return urlTestTarget{}, fmt.Errorf("invalid port %q: %w", port, err)
	}
	return urlTestTarget{
		raw:     rawURL,
		address: net.JoinHostPort(parsed.Hostname(), port),
	}, nil
}

func errorCodeForURLTest(err error) string {
	if err == nil {
		return "probe_core_api_failed"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "probe_timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "probe_context_canceled"
	}
	return "probe_core_api_failed"
}
