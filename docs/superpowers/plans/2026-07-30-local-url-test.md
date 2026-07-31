# Local URL-Test Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make sing-box and Mihomo URL probes use one Sandrone-owned HTTP executor and one Sandrone-owned `expected_status` range matcher.

**Architecture:** Add an immutable HTTP status matcher and a shared build-tagged URL-test executor under `internal/probe`. Each core backend keeps node construction, lookup, retry orchestration, and its existing timing/TLS policy, but adapts its selected outbound to the executor's existing string-address `Dialer` boundary.

**Tech Stack:** Go 1.25.11, `net/http`, `crypto/tls`, sing-box v1.13.14, Mihomo v1.19.28, `testify/require`, local `httptest` servers.

## Global Constraints

- Do not add a public or internal `latency_mode`.
- Do not intentionally change the existing `duration_ms` meaning.
- Keep retries, timeout, concurrency, caching, rendering, and backend selection unchanged.
- Do not fork or replace sing-box or Mihomo.
- Keep unexpected HTTP status under the existing node-level `probe_core_api_failed` contract.
- Preserve the current expression grammar: empty or `*`, exact values, inclusive ranges, `/` or `,` alternatives, ignored empty alternatives, unsigned 16-bit values, and at most 28 alternatives.
- Parse and validate `expected_status` once before core startup or per-node probing.
- Do not access the external network in tests.
- Preserve build-tag isolation: shared core URL-test execution compiles only with `probe_singbox` or `probe_mihomo`.

## File Map

- Create `internal/probe/urltest_status.go`: immutable local expected-status parser and matcher; no core build tag so its contract receives ordinary unit coverage.
- Create `internal/probe/urltest_status_test.go`: table-driven parser and matcher tests.
- Create `internal/probe/core_urltest.go`: shared HTTP/HTTPS `HEAD` executor, private timing policy, and unexpected-status error.
- Create `internal/probe/core_urltest_test.go`: local-server executor tests; same core build expression as the implementation.
- Modify `internal/probe/core_urltest_helpers.go`: replace validation-only URL parsing with a reusable parsed target while retaining error classification.
- Modify `internal/probe/singbox.go`: replace upstream `urltest.URLTest` with the local executor and remove the unsupported-status warning.
- Modify `internal/probe/mihomo.go`: replace `Proxy.URLTest`, `AliveForTestUrl`, and Mihomo's generic range utility with the local executor and matcher.
- Modify `internal/service/probe_singbox_test.go`: prove sing-box matching, mismatch, invalid-expression, and warning behavior through the service boundary.
- Modify `internal/service/probe_mihomo_test.go`: prove the identical Mihomo contract through the service boundary.
- Modify `docs/reference/http-api/probing.md`: document the shared status grammar and both-core support.
- Modify `docs/reference/processors.md`: link processor behavior to the canonical probe contract.
- Modify `docs/reference/errors.md`: remove the obsolete sing-box unsupported-status statement while retaining the non-URL-method warning.

---

### Task 1: Local expected-status matcher

**Files:**
- Create: `internal/probe/urltest_status.go`
- Create: `internal/probe/urltest_status_test.go`

**Interfaces:**
- Consumes: raw `domain.ProbeRequest.ExpectedStatus` strings.
- Produces:

```go
const maxExpectedStatusAlternatives = 28

type expectedStatusRange struct {
	start uint16
	end   uint16
}

type expectedStatusMatcher struct {
	expression string
	ranges     []expectedStatusRange
}

func parseExpectedStatus(raw string) (expectedStatusMatcher, error)
func (m expectedStatusMatcher) Match(statusCode int) bool
func (m expectedStatusMatcher) String() string
```

- `String` returns `*` for an unrestricted matcher and otherwise returns the trimmed, comma-normalized expression used for diagnostics.
- The matcher is immutable after parsing and safe to reuse across probe goroutines.

- [ ] **Step 1: Write parser and matcher tests**

Create `internal/probe/urltest_status_test.go` with a matching table:

```go
package probe

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpectedStatusMatcher(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		expression string
		status     int
		want       bool
		wantString string
	}{
		{name: "empty accepts any", status: 599, want: true, wantString: "*"},
		{name: "wildcard accepts any", expression: " * ", status: 204, want: true, wantString: "*"},
		{name: "exact match", expression: "204", status: 204, want: true, wantString: "204"},
		{name: "exact mismatch", expression: "204", status: 200, wantString: "204"},
		{name: "inclusive lower bound", expression: "200-299", status: 200, want: true, wantString: "200-299"},
		{name: "inclusive upper bound", expression: "200-299", status: 299, want: true, wantString: "200-299"},
		{name: "outside range", expression: "200-299", status: 300, wantString: "200-299"},
		{name: "slash alternatives", expression: "200/204/301-303", status: 302, want: true, wantString: "200/204/301-303"},
		{name: "comma alternatives", expression: "200, 204, 301-303", status: 204, want: true, wantString: "200/204/301-303"},
		{name: "empty alternatives", expression: "/204//", status: 204, want: true, wantString: "/204//"},
		{name: "zero", expression: "0", status: 0, want: true, wantString: "0"},
		{name: "uint16 maximum", expression: "65535", status: 65535, want: true, wantString: "65535"},
		{name: "restricted negative runtime status", expression: "0-65535", status: -1, wantString: "0-65535"},
		{name: "restricted overflow runtime status", expression: "0-65535", status: 65536, wantString: "0-65535"},
		{name: "negative runtime status", expression: "*", status: -1, want: true, wantString: "*"},
		{name: "overflow runtime status", expression: "*", status: 65536, want: true, wantString: "*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := parseExpectedStatus(tt.expression)
			require.NoError(t, err)
			require.Equal(t, tt.want, matcher.Match(tt.status))
			require.Equal(t, tt.wantString, matcher.String())
		})
	}
}

func TestParseExpectedStatusRejectsInvalidExpressions(t *testing.T) {
	t.Parallel()
	tests := []string{
		"abc",
		"-1",
		"65536",
		"200-",
		"-299",
		"299-200",
		"200-299-399",
		strings.Repeat("200/", maxExpectedStatusAlternatives) + "200",
	}
	for _, expression := range tests {
		t.Run(expression, func(t *testing.T) {
			_, err := parseExpectedStatus(expression)
			require.Error(t, err)
		})
	}
}
```

The runtime-status cases deliberately confirm that an unrestricted matcher does
not impose a new HTTP code policy while a restricted matcher cannot overflow a
`uint16`.

- [ ] **Step 2: Run the matcher tests and verify they fail**

Run:

```bash
go test -mod=readonly ./internal/probe -run 'Test(ExpectedStatusMatcher|ParseExpectedStatusRejectsInvalidExpressions)$'
```

Expected: build failure because `parseExpectedStatus` and
`maxExpectedStatusAlternatives` do not exist.

- [ ] **Step 3: Implement the minimal HTTP-specific parser**

Create `internal/probe/urltest_status.go`. Normalize commas to slashes after
trimming the full expression. Split before discarding empty segments so the
28-alternative limit matches Mihomo. Parse every endpoint with
`strconv.ParseUint(value, 10, 16)`, treat a missing or extra hyphen as invalid,
and reject a start greater than its end.

The implementation shape is:

```go
package probe

import (
	"fmt"
	"strconv"
	"strings"
)

const maxExpectedStatusAlternatives = 28

type expectedStatusRange struct {
	start uint16
	end   uint16
}

type expectedStatusMatcher struct {
	expression string
	ranges     []expectedStatusRange
}

func parseExpectedStatus(raw string) (expectedStatusMatcher, error) {
	expression := strings.TrimSpace(raw)
	if expression == "" || expression == "*" {
		return expectedStatusMatcher{expression: "*"}, nil
	}
	expression = strings.ReplaceAll(expression, ",", "/")
	alternatives := strings.Split(expression, "/")
	if len(alternatives) > maxExpectedStatusAlternatives {
		return expectedStatusMatcher{}, fmt.Errorf(
			"expected_status has %d alternatives; maximum is %d",
			len(alternatives),
			maxExpectedStatusAlternatives,
		)
	}
	matcher := expectedStatusMatcher{}
	normalizedAlternatives := make([]string, len(alternatives))
	for i, alternative := range alternatives {
		alternative = strings.TrimSpace(alternative)
		normalizedAlternatives[i] = alternative
		if alternative == "" {
			continue
		}
		start, end, err := parseExpectedStatusRange(alternative)
		if err != nil {
			return expectedStatusMatcher{}, fmt.Errorf(
				"invalid expected_status alternative %q: %w",
				alternative,
				err,
			)
		}
		matcher.ranges = append(matcher.ranges, expectedStatusRange{start: start, end: end})
	}
	matcher.expression = strings.Join(normalizedAlternatives, "/")
	return matcher, nil
}

func parseExpectedStatusRange(raw string) (uint16, uint16, error) {
	parts := strings.Split(raw, "-")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
		return 0, 0, fmt.Errorf("expected an unsigned integer or inclusive range")
	}
	startValue, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 16)
	if err != nil {
		return 0, 0, err
	}
	endValue := startValue
	if len(parts) == 2 {
		endValue, err = strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 16)
		if err != nil {
			return 0, 0, err
		}
	}
	if startValue > endValue {
		return 0, 0, fmt.Errorf("range start %d exceeds end %d", startValue, endValue)
	}
	return uint16(startValue), uint16(endValue), nil
}

func (m expectedStatusMatcher) Match(statusCode int) bool {
	if len(m.ranges) == 0 {
		return true
	}
	if statusCode < 0 || statusCode > int(^uint16(0)) {
		return false
	}
	status := uint16(statusCode)
	for _, candidate := range m.ranges {
		if status >= candidate.start && status <= candidate.end {
			return true
		}
	}
	return false
}

func (m expectedStatusMatcher) String() string {
	if len(m.ranges) == 0 {
		return "*"
	}
	return m.expression
}
```

- [ ] **Step 4: Run matcher tests**

Run:

```bash
go test -mod=readonly ./internal/probe -run 'Test(ExpectedStatusMatcher|ParseExpectedStatusRejectsInvalidExpressions)$'
```

Expected: PASS.

- [ ] **Step 5: Commit the matcher**

```bash
git add internal/probe/urltest_status.go internal/probe/urltest_status_test.go
git commit -m "feat(probe): own expected status matching"
```

---

### Task 2: Shared local HTTP URL-test executor

**Files:**
- Create: `internal/probe/core_urltest.go`
- Create: `internal/probe/core_urltest_test.go`
- Modify: `internal/probe/core_urltest_helpers.go`

**Interfaces:**
- Consumes: `expectedStatusMatcher` from Task 1 and `Dialer` from
  `internal/probe/probe.go`.
- Produces:

```go
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

func parseURLTestTarget(rawURL string) (urlTestTarget, error)
func runURLTest(ctx context.Context, target urlTestTarget, options urlTestOptions) (time.Duration, error)
```

- `parseURLTestTarget` accepts only HTTP and HTTPS, requires a host, applies
  port 80 or 443 when omitted, and stores an IPv6-safe `net.JoinHostPort`
  address.
- `runURLTest` sends exactly one `HEAD`, never follows redirects, clones the
  provided TLS configuration, and closes both response and idle transport
  state.
- A nil `resetStartAfterDial` keeps the pre-dial start. A non-nil function may
  reset the start immediately after dialing when it returns true.
- A nil `now` defaults to `time.Now`.

- [ ] **Step 1: Replace validation tests with parsed-target tests**

Add table-driven tests to `internal/probe/core_urltest_test.go` under:

```go
//go:build probe_mihomo || probe_singbox

package probe
```

Cover these exact inputs:

```go
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
```

Assert the successful target retains the exact raw URL for the request.

- [ ] **Step 2: Write failing executor behavior tests**

In the same file, define:

```go
type dialerFunc func(context.Context, string, string) (net.Conn, error)

func (f dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}
```

Then add focused tests using only local servers:

- `TestRunURLTestMatchesExpectedStatus`: an `httptest.NewServer` verifies
  `HEAD`, returns 204, and the matcher is `200-299`.
- `TestRunURLTestRejectsUnexpectedStatus`: a local server returns 503 while the
  matcher is `200-299`; require an error containing both `503` and `200-299`.
- `TestRunURLTestDoesNotFollowRedirect`: the first handler returns 302 with a
  local `Location`; the destination handler increments an atomic counter;
  matcher `302` succeeds and the counter remains zero.
- `TestRunURLTestSupportsHTTPS`: use `httptest.NewTLSServer` and pass
  `server.Client().Transport.(*http.Transport).TLSClientConfig`.
- `TestRunURLTestUsesAttemptContext`: a handler blocks on
  `request.Context().Done()` and the call uses a 25 ms context deadline;
  require `context.DeadlineExceeded`.
- `TestRunURLTestReturnsDialError`: the dialer returns the sentinel
  `errors.New("dial failed")`; require `errors.Is`.
- `TestRunURLTestClosesConnection`: wrap the delegated `net.Conn`, record its
  `Close` call with a channel, and require that channel is closed before
  `runURLTest` returns.
- `TestRunURLTestAppliesTimingPolicy`: provide an ordered fake `now` returning
  `0 ms`, `40 ms`, and `45 ms`; with a reset policy returning true, require
  `5*time.Millisecond`; without reset, provide `0 ms` and `45 ms` and require
  `45*time.Millisecond`.

For the successful local-server tests, use a dialer that records the requested
network/address and delegates to `net.Dialer.DialContext`. Assert network
`tcp` and the target listener address.

- [ ] **Step 3: Run executor tests and verify they fail**

Run:

```bash
go test -mod=readonly -tags probe_singbox ./internal/probe -run 'Test(ParseURLTestTarget|RunURLTest)'
```

Expected: build failure because the parsed target and executor do not exist.

- [ ] **Step 4: Implement parsed targets**

Replace `validateURLTestURL` in `internal/probe/core_urltest_helpers.go` with
`parseURLTestTarget`. Keep `urlFromRequest` and `errorCodeForURLTest`.

Use `url.Parse`, require `parsed.Hostname()` to be non-empty, accept only
`http` and `https`, select the default port, and validate an explicit port with
`strconv.ParseUint(port, 10, 16)`. Return:

```go
return urlTestTarget{
	raw:     rawURL,
	address: net.JoinHostPort(parsed.Hostname(), port),
}, nil
```

This makes URL validation and dial destination derivation a single batch-level
operation.

- [ ] **Step 5: Implement the local executor**

Create `internal/probe/core_urltest.go` with the shared build expression. The
core execution must follow this sequence:

```go
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
	transport := &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
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
```

Prevent a transport retry from reusing the same one-shot connection: guard the
`DialContext` closure with `sync.Once`-equivalent state and return a clear
`url test connection already consumed` error on a second call. Do not add an
HTTP-client timeout because the per-attempt context is already canonical and
shorter than both upstream hard-coded client timeouts.

- [ ] **Step 6: Run executor and ordinary probe tests**

Run:

```bash
go test -mod=readonly -tags probe_singbox ./internal/probe -run 'Test(ParseURLTestTarget|RunURLTest|ExpectedStatus)'
go test -mod=readonly ./internal/probe
```

Expected: PASS.

- [ ] **Step 7: Commit the executor**

```bash
git add internal/probe/core_urltest.go internal/probe/core_urltest_test.go internal/probe/core_urltest_helpers.go
git commit -m "feat(probe): add local url test executor"
```

---

### Task 3: Migrate the sing-box backend

**Files:**
- Modify: `internal/probe/singbox.go`
- Modify: `internal/service/probe_singbox_test.go`

**Interfaces:**
- Consumes: `parseURLTestTarget`, `parseExpectedStatus`, `runURLTest`, and
  `urlTestOptions` from Tasks 1–2.
- Produces:

```go
type singBoxURLTestDialer struct {
	outbound N.Dialer
}

func (d singBoxURLTestDialer) DialContext(ctx context.Context, networkName, address string) (net.Conn, error)
```

- The adapter calls
  `d.outbound.DialContext(ctx, networkName, M.ParseSocksaddr(address))`.
- The backend supplies `N.NeedHandshakeForWrite` as its private timing
  reset policy and preserves sing-box's context-aware TLS time/root pool.

- [ ] **Step 1: Extend sing-box service tests**

Modify `TestServiceSingBoxURLTestWithLocalProxy` to send
`ExpectedStatus: "200-299"` and assert:

```go
require.True(t, result.Results[0].Alive)
require.Empty(t, result.Report.Warnings)
```

Add `TestServiceSingBoxURLTestRejectsUnexpectedStatus`. Its local target returns
204, its request uses `ExpectedStatus: "200/201"`, and it asserts:

```go
require.NoError(t, err)
require.Len(t, result.Results, 1)
require.False(t, result.Results[0].Alive)
require.Equal(t, "probe_core_api_failed", result.Results[0].ErrorCode)
require.Contains(t, result.Results[0].Error, "204")
require.Contains(t, result.Results[0].Error, "200/201")
```

Add `TestServiceSingBoxURLTestRejectsInvalidExpectedStatus` using
`ExpectedStatus: "not-a-status"` and assert the service returns
`domain.CodeProbeInvalidTarget`. It may use a syntactically valid local URL and
must not start an external request.

- [ ] **Step 2: Run the sing-box service tests and verify the status tests fail**

Run:

```bash
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/service -run 'TestServiceSingBoxURLTest'
```

Expected: the matching case still contains
`probe_expected_status_unsupported`, the mismatch remains alive, and the
invalid expression is not rejected.

- [ ] **Step 3: Replace the upstream sing-box URL-test call**

In `SingBoxBackend.Probe`, before `include.Context`, parse both batch values:

```go
target, err := parseURLTestTarget(testURL)
if err != nil {
	return nil, domain.WrapError(domain.CodeProbeInvalidTarget, "invalid url_test url", err)
}
expectedStatus, err := parseExpectedStatus(req.ExpectedStatus)
if err != nil {
	return nil, domain.WrapError(domain.CodeProbeInvalidTarget, "invalid expected_status", err)
}
```

After creating `boxCtx`, prepare the existing sing-box TLS behavior:

```go
tlsClientConfig := &tls.Config{
	Time:    ntp.TimeFuncFromContext(boxCtx),
	RootCAs: boxadapter.RootPoolFromContext(boxCtx),
}
```

Pass `target`, `expectedStatus`, and `tlsClientConfig` into `probeNode`. Inside
`probeNode`, replace `urltest.URLTest` with:

```go
delay, err := runURLTest(attemptCtx, target, urlTestOptions{
	dialer:              singBoxURLTestDialer{outbound: outbound},
	expectedStatus:      expectedStatus,
	tlsClientConfig:     tlsClientConfig,
	resetStartAfterDial: N.NeedHandshakeForWrite,
})
```

Convert the returned duration with `int(delay / time.Millisecond)` when calling
`successResult`. Remove the import of
`github.com/sagernet/sing-box/common/urltest`.

Delete the block that appends `probe_expected_status_unsupported`. Do not alter
sing-box NTP code.

- [ ] **Step 4: Run sing-box probe and service tests**

Run:

```bash
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/probe ./internal/service -run 'Test(SingBox|ServiceSingBox|RunURLTest|ExpectedStatus)'
```

Expected: PASS, including matching, mismatch, invalid expression, and no
unsupported warning.

- [ ] **Step 5: Commit the sing-box migration**

```bash
git add internal/probe/singbox.go internal/service/probe_singbox_test.go
git commit -m "feat(probe): match sing-box response status"
```

---

### Task 4: Migrate the Mihomo backend

**Files:**
- Modify: `internal/probe/mihomo.go`
- Modify: `internal/service/probe_mihomo_test.go`

**Interfaces:**
- Consumes: the same shared matcher and executor used by Task 3.
- Produces:

```go
type mihomoURLTestDialer struct {
	proxy mihomoconstant.Proxy
}

func (d mihomoURLTestDialer) DialContext(ctx context.Context, networkName, address string) (net.Conn, error)
```

- The method calls `mihomoconstant.Metadata.SetRemoteAddress(address)` and then
  `d.proxy.DialContext(ctx, &metadata)`.
- The backend passes a Mihomo CA TLS configuration and no timing reset function,
  preserving its pre-dial start.

- [ ] **Step 1: Extend Mihomo service tests to mirror sing-box**

Modify `TestServiceMihomoURLTestWithLocalProxy` to use
`ExpectedStatus: "200-299"` and require an empty warning list.

Add `TestServiceMihomoURLTestRejectsUnexpectedStatus` with exactly the same
204 response, `ExpectedStatus: "200/201"`, result assertions, and error text
assertions as the sing-box test.

Keep `TestServiceMihomoURLTestRejectsInvalidExpectedStatus`, but change its URL
to a local syntactically valid URL so the test proves status parsing happens
before any proxy request rather than relying on an unreachable public-shaped
target.

- [ ] **Step 2: Run Mihomo tests before migration**

Run:

```bash
go test -mod=readonly -tags probe_mihomo ./internal/service -run 'TestServiceMihomoURLTest'
```

Expected: existing matching and invalid-expression behavior passes; the new
unexpected-status assertions expose the current indirect
`AliveForTestUrl` path and will be used to lock the direct local error text.

- [ ] **Step 3: Replace Mihomo-owned HTTP execution**

In `MihomoBackend.Probe`, parse the target and matcher before proxy mappings:

```go
target, err := parseURLTestTarget(testURL)
if err != nil {
	return nil, domain.WrapError(domain.CodeProbeInvalidTarget, "invalid url_test url", err)
}
expectedStatus, err := parseExpectedStatus(req.ExpectedStatus)
if err != nil {
	return nil, domain.WrapError(domain.CodeProbeInvalidTarget, "invalid expected_status", err)
}
```

Pass the target and matcher to `probeNode`. After
`mihomoadapter.ParseProxy`, replace `proxy.URLTest` and
`proxy.AliveForTestUrl`. Inside each attempt, create the same Mihomo TLS
configuration used upstream so a TLS configuration failure remains a
retryable node-level URL-test failure:

```go
tlsClientConfig, err := ca.GetTLSConfig(ca.Option{})
if err == nil {
	delay, runErr := runURLTest(attemptCtx, target, urlTestOptions{
		dialer:          mihomoURLTestDialer{proxy: proxy},
		expectedStatus:  expectedStatus,
		tlsClientConfig: tlsClientConfig,
	})
	err = runErr
	if err == nil {
		return successResult(req, node, int(delay/time.Millisecond), b.now())
	}
}
```

Retain `err` as `lastErr` for the existing retry and final-classification path.

Remove `errors` and `github.com/metacubex/mihomo/common/utils` imports. Add the
Mihomo CA import and the adapter implementation. Do not mutate Mihomo global
`UnifiedDelay` and do not call any Mihomo URL-test history methods.

- [ ] **Step 4: Run Mihomo probe and service tests**

Run:

```bash
go test -mod=readonly -tags probe_mihomo ./internal/probe ./internal/service -run 'Test(Mihomo|ServiceMihomo|RunURLTest|ExpectedStatus)'
```

Expected: PASS with the same public success, mismatch, and invalid-expression
behavior as sing-box.

- [ ] **Step 5: Confirm upstream URL-test dependencies are gone**

Run:

```bash
rg -n 'urltest\\.URLTest|\\.URLTest\\(|AliveForTestUrl|common/utils|probe_expected_status_unsupported' internal/probe
```

Expected:

- no `urltest.URLTest`, `Proxy.URLTest`, `AliveForTestUrl`, or Mihomo
  `common/utils` use in `internal/probe`;
- `probe_expected_status_unsupported` remains only in `tcp.go`, where
  `expected_status` is meaningless for `tcp_connect`.

- [ ] **Step 6: Commit the Mihomo migration**

```bash
git add internal/probe/mihomo.go internal/service/probe_mihomo_test.go
git commit -m "refactor(probe): share mihomo url test execution"
```

---

### Task 5: Canonical documentation and repository verification

**Files:**
- Modify: `docs/reference/http-api/probing.md`
- Modify: `docs/reference/processors.md`
- Modify: `docs/reference/errors.md`

**Interfaces:**
- Consumes: the completed shared product behavior from Tasks 1–4.
- Produces: one canonical public description of `expected_status`; no new API
  fields or capability values.

- [ ] **Step 1: Update the canonical probing contract**

In `docs/reference/http-api/probing.md`, replace “backend 不支持时可产生
warning” with the exact shared grammar:

```markdown
`expected_status` 对 sing-box 和 Mihomo 使用相同语义：空值或 `*` 接受任意
状态；`204` 表示精确值；`200-299` 表示闭区间；`/` 或 `,` 可连接多个候选，
例如 `200/204/301-303`。非法表达式返回 `probe_invalid_target`，响应状态不匹配
则该节点探测失败。
```

Keep this page canonical. In `docs/reference/processors.md`, state only that
processor `expected_status` follows the
`[HTTP probe](http-api/probing.md)` contract; do not duplicate the grammar.

- [ ] **Step 2: Correct the warning/error reference**

In `docs/reference/errors.md`:

- remove the statement that a backend may ignore `expected_status`;
- state that `probe_expected_status_unsupported` applies when
  `expected_status` is supplied to a non-URL method such as `tcp_connect`;
- retain `probe_core_api_failed` as the node-level code for URL request and
  unexpected-status failures.

- [ ] **Step 3: Verify obsolete claims and upstream calls are absent**

Run:

```bash
rg -n 'sing-box.*(不支持|忽略).*expected_status|expected_status.*(不支持|忽略)|urltest\\.URLTest|AliveForTestUrl' docs internal/probe
```

Expected: no obsolete sing-box limitation, ignored-status claim, or upstream
URL-test call.

- [ ] **Step 4: Run focused sing-box and Mihomo gates**

Run:

```bash
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/probe ./internal/service
go test -mod=readonly -tags probe_mihomo ./internal/probe ./internal/service
```

Expected: both commands PASS.

- [ ] **Step 5: Run formatting, static, and full repository gates**

Format only changed Go files:

```bash
gofmt -w internal/probe/urltest_status.go internal/probe/urltest_status_test.go internal/probe/core_urltest.go internal/probe/core_urltest_test.go internal/probe/core_urltest_helpers.go internal/probe/singbox.go internal/probe/mihomo.go internal/service/probe_singbox_test.go internal/service/probe_mihomo_test.go
```

Then run:

```bash
git diff --check
make check
```

Expected: both commands PASS. If `gofmt` changes files committed in earlier
tasks, include those mechanical changes in the final documentation commit
rather than rewriting history.

- [ ] **Step 6: Commit documentation and final formatting**

```bash
git add docs/reference/http-api/probing.md docs/reference/processors.md docs/reference/errors.md internal/probe/urltest_status.go internal/probe/urltest_status_test.go internal/probe/core_urltest.go internal/probe/core_urltest_test.go internal/probe/core_urltest_helpers.go internal/probe/singbox.go internal/probe/mihomo.go internal/service/probe_singbox_test.go internal/service/probe_mihomo_test.go
git commit -m "docs: document shared probe status matching"
```

- [ ] **Step 7: Confirm final repository state**

Run:

```bash
git status --short
git log -5 --oneline
```

Expected: the worktree is clean and the task commits are visible in order.
