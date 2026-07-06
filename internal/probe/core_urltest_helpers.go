//go:build probe_mihomo || probe_singbox

package probe

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func urlFromRequest(req domain.ProbeRequest) string {
	return urlTestTarget(req)
}

func validateURLTestURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Host == "" {
		return fmt.Errorf("missing host")
	}
	switch parsed.Scheme {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
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
