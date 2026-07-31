# VMessAEAD URL Import Design

## Context

Sandrone currently treats every `vmess://` URI as the legacy Base64-encoded
JSON profile. XTLS/Xray-core Discussion #716 also defines an AEAD sharing
profile with the URL shape:

```text
vmess://UUID@host:port?query#name
```

The URL profile is used by current XTLS ecosystem implementations, while
legacy Base64 JSON remains a common VMess interchange and output format.
Sandrone therefore needs additive import compatibility without changing its
existing output contract.

## Goals

- Import VMessAEAD URL-profile links through the existing `uri`, `uri-list`,
  and outer `base64` input paths.
- Preserve legacy `vmess://Base64(JSON)` parsing without behavior changes.
- Normalize fields that the current `NodeIR` can represent, using the same
  TLS, Reality, ECH, transport, and raw-query conventions as VLESS URI input.
- Reject structurally ambiguous input instead of selecting one of several
  conflicting values.
- Preserve unsupported query fields in `NodeIR.Raw` so existing reporting
  emits structured compatibility warnings.
- Record the Discussion #716 profile as a source reference for URL-form input.

## Non-goals

- Change VMess rendering from legacy Base64 JSON to the URL profile.
- Claim complete, lossless, or version-stable Discussion #716 support.
- Add fields to `NodeIR` for gRPC mode, mKCP tuning, FinalMask, TLS
  verification-name controls, or post-quantum Reality options.
- Add a renderer format, processor, entrypoint, subscription type, or
  `FileSpec.kind`.
- Tighten the existing VLESS parser or unrelated URI schemes in this change.

## Chosen Parser Design

`parseVMess` remains the only registered VMess entrypoint and dispatches
between two private paths:

1. If the payload after `vmess://` contains userinfo syntax (`@`), parse it as
   the VMessAEAD URL profile.
2. Otherwise, parse it with the existing Base64-JSON implementation.

The marker is unambiguous for supported legacy payloads because neither the
standard nor URL-safe Base64 alphabet contains `@`. Malformed URL-profile
input that contains `@` must return a URL-profile parse error and must not fall
back to Base64 JSON.

The URL-profile path:

- parses the URI with `net/url`;
- requires exactly one username-style userinfo value, a host, and an explicit
  port in the range `1..65535`;
- maps the fragment to `NodeIR.Name`;
- maps `encryption` to `NodeIR.Cipher`, defaulting to `auto`;
- keeps `AlterID` at zero because the profile covers VMess AEAD;
- maps the existing packet-encoding compatibility field;
- reuses the URI adapter's TLS and transport query helpers;
- enables the existing typed XHTTP `mode` and `extra` promotion for VMess as
  well as VLESS;
- preserves every unpromoted query key as `uri.query.<key>` in `NodeIR.Raw`.

Exact duplicate query keys are rejected before any value is consumed. This
prevents first-value/last-value disagreements across clients. Existing query
aliases handled by the shared helpers remain accepted for interoperability;
unrecognized or currently unrepresentable fields remain raw and produce the
normal parse warning.

## Supported Field Boundary

The new path promotes the stable intersection already expressible by
Sandrone:

- identity and endpoint: UUID, host, explicit port, display name;
- VMess: `encryption`;
- TLS/Reality/ECH: existing URI query fields understood by `applyTLSQuery`;
- transport: `type`, path, host or authority, and gRPC service name;
- XHTTP: mode and the currently typed subset of `extra`;
- packet encoding compatibility aliases.

Fields such as gRPC `mode`, mKCP `mtu` and `tti`, `fm`, `vcn`, `pqv`, and
`spx` remain in `Raw`. The parser must not silently claim that Sandrone can
round-trip them.

## Output Compatibility

The URI renderer remains unchanged. A VMess `NodeIR` still renders as legacy
Base64 JSON, and outer `base64` output continues to wrap that URI list. This
keeps current subscriptions, public shares, and downstream client behavior
stable.

## Error and Warning Behavior

- Invalid URL syntax, missing UUID, password-style userinfo, missing host,
  missing or invalid port, and duplicate query keys return
  `parse_failed`.
- A legacy payload continues to return the existing Base64/JSON/field errors.
- Unsupported URL-profile fields do not fail parsing. They are preserved in
  `Raw` and reported by the existing unknown-field warning pipeline.
- URI-list parsing retains its current per-line error behavior.

## Source Metadata

Legacy VMess input keeps the existing legacy sharing-profile reference.
VMessAEAD URL input receives a separate source reference to XTLS/Xray-core
Discussion #716. Both paths continue to report the stable source format name
`vmess`.

## Testing

Implementation follows red-green-refactor:

1. Add failing parser tests for a no-query VMessAEAD URL and a TLS WebSocket
   URL with an escaped name.
2. Add failing coverage for Reality/XHTTP promotion and unsupported raw
   fields.
3. Add failing error tests for duplicate query keys, password-style userinfo,
   and missing explicit port.
4. Add IPv6 endpoint coverage.
5. Retain and rerun all legacy Base64-JSON VMess tests.
6. Add source-reference coverage distinguishing the two input profiles.
7. Run URI/shared narrow tests, repository formatting and static checks, then
   the repository `make check` gate.

## Acceptance Criteria

- Valid VMessAEAD URL-profile input produces one normalized VMess `NodeIR`.
- Both queryless and single-query links parse successfully.
- Legacy standard and URL-safe Base64 JSON inputs remain unchanged.
- Duplicate query keys and malformed authorities fail deterministically.
- Unsupported fields are visible in structured warnings.
- VMess rendering output is byte-for-byte governed by the existing legacy
  renderer tests.
