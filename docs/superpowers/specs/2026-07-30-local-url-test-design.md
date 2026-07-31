# Local URL-Test Design

## Goal

Move URL-test HTTP execution and expected-status matching into Sandrone so the
sing-box and Mihomo probe backends expose one product contract:

- both backends accept the same `expected_status` expressions;
- HTTP request behavior and status matching are maintained locally;
- backend-specific code only adapts a core-native outbound or proxy into a
  connection used by the local executor.

This change does not add `latency_mode` and does not intentionally change the
existing `duration_ms` meaning.

## Current State

The sing-box backend calls `common/urltest.URLTest`. That upstream helper sends
a `HEAD` request and returns a duration, but it neither exposes nor validates
the response status. Sandrone therefore ignores `expected_status` for sing-box
and emits `probe_expected_status_unsupported`.

The Mihomo backend calls `adapter.Proxy.URLTest`. Mihomo owns the HTTP request,
duration measurement, status-range parser, and match result. Sandrone must then
consult `AliveForTestUrl` to distinguish a status mismatch from a successful
request.

Although the upstream APIs differ, both cores already expose the primitive
Sandrone needs: a way to establish a `net.Conn` through a selected node.

## Scope

### Included

- A local URL-test HTTP executor shared by the sing-box and Mihomo backends.
- A local HTTP status-range parser and matcher.
- sing-box support for `expected_status`.
- Removal of the sing-box unsupported-status warning.
- Backend adapters that preserve core-specific dialing.
- Tests that enforce identical status behavior across both backends.
- Canonical probe, processor, and error documentation updates.

### Excluded

- A public or internal `latency_mode` option.
- A new RTT, TTFB, ICMP, TCP-handshake, or warm-connection measurement mode.
- Changes to retry, timeout, concurrency, caching, payload rendering, or backend
  selection.
- Forking or replacing either upstream module.
- A new public error code for unexpected HTTP status.

## Architecture

### Shared URL-Test Executor

Add a focused local executor under `internal/probe`. It owns:

1. validating and resolving the HTTP or HTTPS target;
2. asking a backend-provided dial function for the target connection;
3. constructing an HTTP transport around that connection;
4. sending one `HEAD` request without following redirects;
5. closing the response body and idle transport state;
6. matching the response status against the local status matcher;
7. returning the measured duration or an error.

The executor accepts a small dial callback instead of importing either core's
proxy abstraction. This keeps shared HTTP behavior independent from sing-box
and Mihomo types and preserves the existing dependency direction.

The executor must continue using the attempt context supplied by the backend.
The HTTP client and request must honor that context; an internal client timeout
must not outlive or override the shorter attempt deadline.

### Dial Adapters

The sing-box adapter looks up the node outbound in the running sing-box
instance, then adapts its metadata-based `DialContext` call to the executor's
callback.

The Mihomo adapter parses the rendered proxy mapping as it does today, converts
the target URL into Mihomo metadata, then adapts the proxy's `DialContext` call
to the same callback.

Backend files continue to own node lookup, proxy parsing, retries, concurrency,
result construction, backend names, and backend versions. They no longer own
HTTP request execution or status matching.

### Duration Compatibility

The shared executor provides a private timing-policy hook because the two
upstream implementations do not always choose the same start point:

- Mihomo currently starts before its proxy dial.
- sing-box starts before dial, but resets the start after dialing for
  connections marked as requiring handshake on first write.

Each adapter selects the policy matching its current upstream behavior. The
policy is deliberately private and is not a `latency_mode`; it only prevents
this refactor from silently redefining existing results. A later latency
contract change can replace these policies inside the shared boundary.

## Expected-Status Contract

Sandrone owns a non-generic HTTP status matcher compatible with the currently
accepted Mihomo expression syntax:

- an empty string or `*` matches every response status;
- a single value matches that exact value, such as `204`;
- an inclusive range matches every value between its bounds, such as
  `200-299`;
- `/` and `,` separate alternatives, such as `200/204/301-303`;
- surrounding whitespace is ignored;
- empty alternatives are ignored for compatibility;
- malformed integers, reversed ranges, overflow, or excessive alternatives
  make the probe request invalid.

The existing maximum of 28 alternatives is retained to avoid an unbounded
expression and to preserve current Mihomo-facing behavior. Values are parsed as
unsigned 16-bit integers, matching the existing contract rather than adding a
new HTTP-status policy in this refactor.

The expression is parsed once per probe batch before any core instance or
per-node work begins. An invalid expression returns
`probe_invalid_target`. A valid expression is reused concurrently and is
immutable.

When a response status does not match, that node attempt fails. Retries proceed
normally. If all attempts fail due to status mismatch, the node remains a
`probe_core_api_failed` result under the existing public error contract.

## Data Flow

1. Service normalization supplies the canonical URL, timeout, attempts,
   concurrency, core, and `expected_status`.
2. The chosen backend validates the URL and parses the status expression once.
3. The backend starts or parses its core-specific node representation.
4. For each attempt, the backend creates the attempt context and calls the
   shared executor with its dial adapter and timing policy.
5. The executor dials through the selected node, sends `HEAD`, records the
   duration, and validates the returned status.
6. The backend converts success or the final error into the existing
   `NodeProbeResult`.
7. Reporting and cache behavior remain unchanged.

## Error Handling

- Invalid URL and invalid status expressions fail the batch with
  `probe_invalid_target`.
- Context deadline and cancellation continue to map through
  `errorCodeForURLTest`.
- Dial, HTTP transport, TLS, and unexpected-status failures remain node-level
  URL-test failures.
- The error for a status mismatch includes the observed status and expected
  expression for diagnosis, but must not include response headers or bodies.
- Redirects are not followed; the redirect response status is matched directly.
- The sing-box backend no longer emits
  `probe_expected_status_unsupported`.
- `tcp_connect` behavior is unchanged: it may still warn when
  `expected_status` is supplied to a method for which the field is meaningless.

## Testing

### Shared Unit Tests

Test the local matcher independently for:

- empty and `*` expressions;
- exact statuses, inclusive ranges, slash alternatives, comma alternatives,
  and surrounding whitespace;
- boundary values and multiple ranges;
- malformed values, reversed ranges, overflow, and more than 28 alternatives.

Test the local executor with controlled dialers and HTTP servers for:

- a matching response;
- a non-matching response;
- redirect handling without following the redirect;
- HTTP and HTTPS targets;
- dial, request, timeout, and cancellation failures;
- response-body and connection cleanup;
- the two private timing policies.

### Backend and Service Tests

Run equivalent integration cases through both build-tagged backends:

- `204` matches a `204` response;
- `200-299` matches a `204` response;
- a mismatching range makes the node dead;
- an invalid expression fails with `probe_invalid_target`;
- sing-box produces no unsupported-status warning;
- method, core, backend, target, and result ordering remain unchanged.

Use local proxy and target servers only. No test may depend on external network
access.

## Documentation

Update the canonical HTTP probing and processor references to state that both
URL-test cores support the same status expression. Remove the backend-
unsupported wording and the sing-box-specific
`probe_expected_status_unsupported` explanation from the error reference while
retaining the warning for non-URL methods where applicable.

No additional configuration field, capability value, API schema property, or
Web control is introduced.

## Acceptance Criteria

- sing-box and Mihomo use the same local HTTP executor and status matcher.
- Both accept and reject identical `expected_status` expressions.
- A status mismatch fails the corresponding node on both backends.
- Invalid expressions fail before per-node probing begins.
- sing-box emits no unsupported-status warning.
- Existing duration behavior is preserved intentionally through private timing
  policies.
- Relevant build-tagged tests and repository gates pass.
