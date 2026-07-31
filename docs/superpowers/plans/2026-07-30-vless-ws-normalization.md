# VLESS WebSocket Compatibility Normalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Normalize URI WebSocket early-data syntax and remove incompatible VLESS Vision flow from parsed non-TCP nodes with an explicit warning.

**Architecture:** URI-specific syntax is decoded in `internal/adapter/uri`. Cross-format VLESS transport compatibility is normalized once in `internal/service` immediately after parsing, before validation and node processors, so URI, Mihomo, sing-box, and JSON inputs behave consistently.

**Tech Stack:** Go, testify, existing adapter/service/report contracts.

## Global Constraints

- Preserve input order and existing parser fallback behavior.
- Do not make renderer-specific semantic mutations.
- Do not expose credentials or raw subscription lines in compatibility warnings.
- Use test-first red/green cycles and run the narrow package tests before `make check`.

---

### Task 1: Decode URI WebSocket early-data syntax

**Files:**
- Modify: `internal/adapter/uri/parser_query.go`
- Test: `internal/adapter/uri/parser_test.go`

**Interfaces:**
- Consumes: `domain.TransportOptions` populated by `applyTransportQuery`.
- Produces: WebSocket `Path`, `MaxEarlyData`, and `EarlyDataHeaderName` canonical fields.

- [ ] **Step 1: Write a failing parser test**

Add a VLESS URI case with `path=%2Fdo%3Fed%3D2048` and assert:

```go
require.Equal(t, "/do", got.Transport.Path)
require.Equal(t, 2048, got.Transport.MaxEarlyData)
require.Equal(t, "Sec-WebSocket-Protocol", got.Transport.EarlyDataHeaderName)
```

- [ ] **Step 2: Run the test and verify the missing normalization failure**

Run:

```sh
go test -mod=readonly ./internal/adapter/uri -run TestParseVLESSWebSocketEarlyDataPath -count=1
```

Expected: FAIL because the current path remains `/do?ed=2048`.

- [ ] **Step 3: Implement the minimal URI normalization**

After WebSocket transport aliases are normalized, recognize an exact positive-integer `ed` query suffix, remove it from the canonical path, and set:

```go
transport.MaxEarlyData = parsedEarlyData
transport.EarlyDataHeaderName = "Sec-WebSocket-Protocol"
```

Malformed or mixed path queries remain unchanged.

- [ ] **Step 4: Run the URI adapter tests**

Run:

```sh
go test -mod=readonly ./internal/adapter/uri -count=1
```

Expected: PASS.

### Task 2: Normalize incompatible Vision flow across parsed formats

**Files:**
- Create: `internal/service/node_normalization.go`
- Modify: `internal/service/remote_input.go`
- Test: `internal/service/transform_test.go`

**Interfaces:**
- Consumes: `*parseInputResult` from explicit and auto-detected parsers.
- Produces: parsed nodes where `xtls-rprx-vision` is retained only for nil, empty, `tcp`, or `raw` transport; incompatible flow is removed with `node_normalized_incompatible_flow`.

- [ ] **Step 1: Write failing service tests**

Parse JSON nodes containing VLESS Vision over WebSocket and assert:

```go
require.Empty(t, result.Nodes[0].Flow)
require.Equal(t, "node_normalized_incompatible_flow", result.Report.Warnings[0].Code)
require.Equal(t, "flow", result.Report.Warnings[0].Field)
```

Also parse VLESS Vision over TCP and assert the flow is retained without that warning.

- [ ] **Step 2: Run the tests and verify the incompatible flow remains**

Run:

```sh
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/service -run TestServiceParseNormalizesIncompatibleVLESSVisionFlow -count=1
```

Expected: FAIL because the current service returns `xtls-rprx-vision`.

- [ ] **Step 3: Implement shared post-parse normalization**

Add a pure helper that walks parsed nodes once, clears incompatible VLESS Vision flow, and appends a warning containing node identity, index, field, and normalized source without embedding UUID/password/raw input.

Call it exactly once from `parseNodeContent` after explicit or auto-detected parsing succeeds.

- [ ] **Step 4: Run service tests**

Run:

```sh
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/service -count=1
```

Expected: PASS.

### Task 3: Document and verify the canonical contract

**Files:**
- Modify: `docs/architecture/node-pipeline.md`

**Interfaces:**
- Consumes: implemented URI and service normalization behavior.
- Produces: one canonical description of parsing/normalization responsibility.

- [ ] **Step 1: Document the behavior**

State that URI WebSocket `?ed=<positive integer>` is decoded into typed early-data fields and that service normalization removes VLESS Vision flow from non-TCP/raw transports with a structured warning.

- [ ] **Step 2: Run focused and repository gates**

Run:

```sh
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/adapter/uri ./internal/service -count=1
make check
git diff --check
git status --short
```

Expected: all commands pass; status shows only the planned source, tests, documentation, and this plan.
