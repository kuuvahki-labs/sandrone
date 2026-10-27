//go:build probe_singbox || probe_mihomo

package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type nodeHTTPRoute struct {
	dialer    Dialer
	tlsConfig *tls.Config
	close     func() error
}

func executeNodeHTTP(ctx context.Context, nodes []domain.NodeIR, opts domain.NodeHTTPOptions, skipped map[int]string, route func(context.Context, int, domain.NodeIR) (nodeHTTPRoute, error)) ([]domain.NodeHTTPResult, error) {
	results := make([]domain.NodeHTTPResult, len(nodes))
	concurrency := min(opts.Concurrency, len(nodes))
	if concurrency <= 0 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, node := range nodes {
		wg.Go(func() {
			results[i].Index = i
			defer func() {
				if recover() != nil {
					results[i].Error = "HTTP worker panicked"
				}
			}()
			if message, ok := skipped[i]; ok {
				results[i].Error = "node unsupported: " + message
				return
			}
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[i].Error = ctx.Err().Error()
				return
			}
			release := sync.OnceFunc(func() { <-sem })
			ownsLease := true
			defer func() {
				if ownsLease {
					release()
				}
			}()
			start := time.Now()
			defer func() { results[i].DurationMS = time.Since(start).Milliseconds() }()
			requestCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.TimeoutMS)*time.Millisecond)
			defer cancel()
			// Ownership passes to initialization, then to the returned route. A
			// timed-out initializer keeps its slot until late cleanup completes.
			ownsLease = false
			selected, err := prepareNodeHTTPRoute(requestCtx, release, func() (nodeHTTPRoute, error) { return route(requestCtx, i, node) })
			if selected.close != nil {
				defer func() { _ = selected.close() }()
			}
			if err != nil {
				results[i].Error = err.Error()
				return
			}
			if selected.dialer == nil {
				results[i].Error = "node outbound was not found"
				return
			}
			transport := &http.Transport{
				DialContext:            httpSafeDialer{Dialer: selected.dialer}.DialContext,
				DisableKeepAlives:      true,
				MaxResponseHeaderBytes: 64 << 10,
			}
			if selected.tlsConfig != nil {
				transport.TLSClientConfig = selected.tlsConfig.Clone()
			}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			results[i] = requestNodeHTTP(requestCtx, client, i, opts)
		})
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// A core may perform blocking initialization before returning its dialer. Keep
// that work within the node deadline and close a late result rather than leaking
// it or allowing it to issue a request after the caller has timed out. The lease
// covers initialization, HTTP and cleanup; timing out does not release capacity
// while the core is still alive. Queued workers remain cancelable by the caller.
func prepareNodeHTTPRoute(ctx context.Context, release func(), create func() (nodeHTTPRoute, error)) (nodeHTTPRoute, error) {
	type outcome struct {
		route nodeHTTPRoute
		err   error
	}
	done := make(chan outcome)
	go func() {
		result := outcome{}
		func() {
			defer func() {
				if recover() != nil {
					result.err = fmt.Errorf("HTTP core initialization panicked")
				}
			}()
			result.route, result.err = create()
		}()
		closeRoute := result.route.close
		result.route.close = func() error {
			defer release()
			if closeRoute != nil {
				return closeRoute()
			}
			return nil
		}
		select {
		case done <- result:
		case <-ctx.Done():
			if result.route.close != nil {
				func() {
					defer func() { _ = recover() }() // Match worker isolation for late cleanup.
					_ = result.route.close()
				}()
			}
		}
	}()
	select {
	case result := <-done:
		return result.route, result.err
	case <-ctx.Done():
		return nodeHTTPRoute{}, ctx.Err()
	}
}

func requestNodeHTTP(ctx context.Context, client *http.Client, index int, opts domain.NodeHTTPOptions) (result domain.NodeHTTPResult) {
	result.Index = index
	request, err := http.NewRequestWithContext(ctx, opts.Method, opts.URL, nil)
	if err != nil {
		result.Error = err.Error()
		return
	}
	for name, value := range opts.Headers {
		request.Header.Set(name, value)
	}
	response, err := client.Do(request)
	if err != nil {
		result.Error = err.Error()
		return
	}
	defer func() { _ = response.Body.Close() }()
	result.StatusCode = response.StatusCode
	result.Headers = response.Header.Clone()
	for name, values := range result.Headers {
		for i, value := range values {
			result.Headers[name][i] = strings.ToValidUTF8(value, "\uFFFD")
		}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, opts.MaxResponseBytes+1))
	if err != nil {
		result.Error = err.Error()
		return
	}
	if int64(len(body)) > opts.MaxResponseBytes {
		result.Error = fmt.Sprintf("response exceeds max_response_bytes (%d)", opts.MaxResponseBytes)
		return
	}
	result.Body = strings.ToValidUTF8(string(body), "\uFFFD")
	return
}
