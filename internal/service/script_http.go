package service

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http/httpguts"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
)

// RequestNodeHTTP performs one uncached request per node. It deliberately does not
// use Probe's connection deduplication: result indices address the supplied slice.
func (s *Service) RequestNodeHTTP(ctx context.Context, nodes []domain.NodeIR, opts domain.NodeHTTPOptions) ([]domain.NodeHTTPResult, error) {
	opts, err := normalizeNodeHTTPOptions(opts)
	if err != nil {
		return nil, err
	}
	if !s.probeEnabled() || !probe.NodeHTTPAvailable(opts.Core) {
		return nil, domain.NewError(domain.CodeProbeBackendUnavailable, fmt.Sprintf("node HTTP requests are not available for core %q", opts.Core))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return []domain.NodeHTTPResult{}, nil
	}
	// Copy before assigning lookup tags; user node names and identities stay intact.
	runtimeNodes := append([]domain.NodeIR(nil), nodes...)
	for i := range runtimeNodes {
		runtimeNodes[i].Name = fmt.Sprintf("sandrone-http-%d", i)
	}
	req := domain.ProbeRequest{Method: domain.ProbeURLTest, Core: opts.Core}
	payloads, warnings, err := s.renderProbePayloads(ctx, &req, runtimeNodes)
	if err != nil {
		if allProbeNodesSkipped(warnings, len(runtimeNodes)) {
			results := make([]domain.NodeHTTPResult, len(runtimeNodes))
			for i := range results {
				results[i] = domain.NodeHTTPResult{Index: i, Error: "node is unsupported by " + opts.Core}
			}
			return results, nil
		}
		return nil, err
	}
	if len(payloads) != 1 {
		return nil, domain.NewError(domain.CodeProbeInvalidTarget, "node HTTP request payload is missing")
	}
	return probe.RequestNodeHTTP(ctx, runtimeNodes, opts, payloads[0])
}

func normalizeNodeHTTPOptions(opts domain.NodeHTTPOptions) (domain.NodeHTTPOptions, error) {
	invalid := func(message string) (domain.NodeHTTPOptions, error) {
		return opts, domain.NewError(domain.CodeInvalidArgument, message)
	}
	if opts.Core == "" {
		opts.Core = "sing-box"
	}
	if opts.Core != "sing-box" && opts.Core != "mihomo" {
		return invalid("api.http.request core must be sing-box or mihomo")
	}
	if opts.Method == "" {
		opts.Method = http.MethodGet
	}
	if opts.Method != http.MethodGet && opts.Method != http.MethodHead {
		return invalid("api.http.request supports only GET and HEAD")
	}
	target, err := url.Parse(opts.URL)
	if err != nil || target.Hostname() == "" || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil || target.Fragment != "" {
		return invalid("api.http.request requires an HTTP(S) URL without userinfo or fragment")
	}
	if port := target.Port(); port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil || number == 0 {
			return invalid("invalid api.http.request URL port")
		}
	}
	if _, err := http.NewRequestWithContext(context.Background(), opts.Method, opts.URL, nil); err != nil {
		return invalid("invalid api.http.request URL")
	}
	if opts.TimeoutMS < 0 || int64(opts.TimeoutMS) > math.MaxInt64/int64(time.Millisecond) || opts.Concurrency < 0 || opts.MaxResponseBytes < 0 || opts.MaxResponseBytes == math.MaxInt64 {
		return invalid("api.http.request limits must be non-negative and within range")
	}
	for key, value := range opts.Headers {
		if !httpguts.ValidHeaderFieldName(key) || !httpguts.ValidHeaderFieldValue(value) {
			return invalid("invalid api.http.request header")
		}
		if strings.EqualFold(key, "Host") {
			return invalid("api.http.request Host must be specified by URL")
		}
	}
	if opts.TimeoutMS == 0 {
		opts.TimeoutMS = 5000
	}
	if opts.Concurrency == 0 {
		opts.Concurrency = 4
	}
	if opts.MaxResponseBytes == 0 {
		opts.MaxResponseBytes = 65536
	}
	return opts, nil
}
