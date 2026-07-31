//go:build probe_mihomo || probe_singbox

package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

type urlTestTarget struct {
	raw     string
	address string
}

type urlTestOptions struct {
	dialer              Dialer
	expectedStatus      expectedStatusMatcher
	tlsClientConfig     *tls.Config
	resetStartAfterDial func(net.Conn) bool
	now                 func() time.Time
}

func runURLTest(ctx context.Context, target urlTestTarget, options urlTestOptions) (time.Duration, error) {
	if options.dialer == nil {
		return 0, errors.New("url test dialer is required")
	}
	now := options.now
	if now == nil {
		now = time.Now
	}
	start := now()
	conn, err := options.dialer.DialContext(ctx, "tcp", target.address)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if options.resetStartAfterDial != nil && options.resetStartAfterDial(conn) {
		start = now()
	}

	tlsConfig := options.tlsClientConfig
	if tlsConfig != nil {
		tlsConfig = tlsConfig.Clone()
	}
	var connConsumed atomic.Bool
	transport := &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			if connConsumed.Swap(true) {
				return nil, errors.New("url test connection already consumed")
			}
			return conn, nil
		},
		TLSClientConfig: tlsConfig,
	}
	defer transport.CloseIdleConnections()
	client := http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, target.raw, nil)
	if err != nil {
		return 0, err
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	duration := now().Sub(start)
	if !options.expectedStatus.Match(response.StatusCode) {
		return duration, fmt.Errorf(
			"response status %d did not match expected_status %s",
			response.StatusCode,
			options.expectedStatus.String(),
		)
	}
	return duration, nil
}
