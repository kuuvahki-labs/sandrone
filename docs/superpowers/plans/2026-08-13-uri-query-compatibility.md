# URI Query Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Normalize WebSocket `ed`/`eh` and lowercase `allowinsecure` URI query aliases into existing `NodeIR` fields without hiding invalid or conflicting values.

**Architecture:** URI-specific alias decoding remains in `internal/adapter/uri`. Transport parsing reports exactly which query fields were semantically consumed so each protocol parser can distinguish typed values from warning-producing raw values.

**Tech Stack:** Go, `net/url`, testify, existing URI adapter and service render contracts.

## Global Constraints

- Apply `ed` and `eh` only to WebSocket transport.
- Accept only positive decimal `ed`; invalid values remain in `NodeIR.Raw`.
- Preserve conflicting query fields in `NodeIR.Raw` rather than overriding path-derived semantics.
- Default an accepted `ed` without `eh` to `Sec-WebSocket-Protocol`.
- Do not change `spx` handling or renderer-specific behavior.
- Preserve unrelated working-tree changes and stage only files owned by this task.

---

### Task 1: Decode WebSocket early-data query aliases

**Files:**
- Modify: `internal/adapter/uri/parser_query.go`
- Modify: `internal/adapter/uri/parser_protocols.go`
- Modify: `internal/adapter/uri/parser_vmess.go`
- Test: `internal/adapter/uri/parser_test.go`

**Interfaces:**
- Consumes: WebSocket `domain.TransportOptions` and parsed `url.Values`.
- Produces: typed `MaxEarlyData`, `EarlyDataHeaderName`, and a set of semantically consumed query keys.

- [ ] **Step 1: Write failing parser tests**

Add VLESS cases asserting `ed=2560` produces:

```go
require.Equal(t, 2560, got.Transport.MaxEarlyData)
require.Equal(t, "Sec-WebSocket-Protocol", got.Transport.EarlyDataHeaderName)
require.NotContains(t, got.Raw, "uri.query.ed")
```

Add `ed=2560&eh=X-Early-Data` and assert `X-Early-Data` is typed. Add invalid,
non-WebSocket, and path/query-conflict cases asserting the query keys remain in
`Raw`. Cover VMess AEAD and Trojan once to prove the shared transport path.

- [ ] **Step 2: Run tests and verify the new cases fail**

Run:

```sh
go test -mod=readonly ./internal/adapter/uri -run 'TestParse(VLESS|VMessAEAD|Trojan).*EarlyDataQuery' -count=1
```

Expected: FAIL because `ed` and `eh` are currently preserved in `Raw` and do
not populate the typed early-data fields.

- [ ] **Step 3: Implement minimal typed query consumption**

Add a transport parse result carrying xHTTP completeness plus a map of consumed
fields. Parse positive decimal `ed`, apply a compatible non-empty `eh`, and
merge consumed keys into each protocol parser's known-field set. Leave invalid
or conflicting keys unknown so `preserveURIQuery` retains and warns for them.

- [ ] **Step 4: Run URI adapter tests**

Run:

```sh
go test -mod=readonly ./internal/adapter/uri -count=1
```

Expected: PASS.

### Task 2: Accept lowercase TLS query alias

**Files:**
- Modify: `internal/adapter/uri/parser_query.go`
- Modify: `internal/adapter/uri/parser_protocols.go`
- Modify: `internal/adapter/uri/parser_vmess.go`
- Test: `internal/adapter/uri/parser_test.go`

**Interfaces:**
- Consumes: URI query key `allowinsecure` with existing boolean spellings.
- Produces: `TLS.InsecureSkipVerify` and no raw-field warning for valid values.

- [ ] **Step 1: Write the failing alias test**

Extend the TLS compatibility alias test with `allowinsecure` and assert:

```go
require.True(t, nodes[0].TLS.InsecureSkipVerify)
require.NotContains(t, nodes[0].Raw, "uri.query.allowinsecure")
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```sh
go test -mod=readonly ./internal/adapter/uri -run TestParseTUICTLSCompatibilityAliases -count=1
```

Expected: FAIL because the lowercase key is not currently read or marked known.

- [ ] **Step 3: Add the alias to shared parsing and protocol known sets**

Add `allowinsecure` next to `allowInsecure` in `applyTLSQuery`, VMess AEAD
typed-field detection, and every URI protocol known-field set that uses the
shared TLS parser.

- [ ] **Step 4: Run URI adapter tests**

Run:

```sh
go test -mod=readonly ./internal/adapter/uri -count=1
```

Expected: PASS.

### Task 3: Verify cross-format behavior and update the contract

**Files:**
- Modify: `internal/service/transform_test.go`
- Modify: `docs/architecture/node-pipeline.md`
- Modify: `docs/reference/capabilities.md`

**Interfaces:**
- Consumes: typed URI adapter output.
- Produces: documented canonical behavior and renderer evidence for Mihomo and sing-box.

- [ ] **Step 1: Add a service conversion test**

Parse a VLESS WebSocket URI with `ed=2560&eh=Sec-WebSocket-Protocol`, render to
Mihomo and sing-box, and assert their existing early-data fields contain the
same values without parse warnings.

- [ ] **Step 2: Document the query aliases**

Extend the canonical URI normalization text to cover query-level `ed`/`eh` and
list lowercase `allowinsecure` among accepted TLS aliases.

- [ ] **Step 3: Run focused and repository gates**

Run:

```sh
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/adapter/uri ./internal/service -count=1
make check
make lint
git diff --check
```

Expected: all commands pass. Rebuild the CLI and validate
`data/subscriptions/hthth.cyou.json`; expected remaining parse warning is only
the intentionally unsupported `uri.query.spx`.
