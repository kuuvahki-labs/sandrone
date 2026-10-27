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

func executeNodeHTTP(ctx context.Context, nodes []domain.NodeIR, opts domain.NodeHTTPOptions, skipped map[int]string, route func(int, domain.NodeIR) (nodeHTTPRoute, error)) ([]domain.NodeHTTPResult, error) {
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
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i].Error = ctx.Err().Error()
				return
			}
			selected, err := route(i, node)
			if err != nil {
				results[i].Error = err.Error()
				return
			}
			if selected.close != nil {
				defer func() { _ = selected.close() }()
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
			results[i] = requestNodeHTTP(ctx, client, i, opts)
		})
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
func requestNodeHTTP(ctx context.Context, client *http.Client, index int, opts domain.NodeHTTPOptions) (result domain.NodeHTTPResult) {
	result.Index = index
	start := time.Now()
	defer func() { result.DurationMS = time.Since(start).Milliseconds() }()
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.TimeoutMS)*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, opts.Method, opts.URL, nil)
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
