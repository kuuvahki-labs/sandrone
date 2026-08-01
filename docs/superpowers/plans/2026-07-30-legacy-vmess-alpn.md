# Legacy VMess ALPN Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Parse supported legacy VMess `alpn` values into canonical `NodeIR.TLS.ALPN` while preserving invalid values as raw warning data.

**Architecture:** Keep the change inside the URI adapter. A focused normalizer accepts only comma-separated strings or arrays containing only strings; `parseLegacyVMess` consumes the field only after successful normalization, and existing renderers continue to handle canonical ALPN.

**Tech Stack:** Go, `encoding/json`, Testify, existing URI adapter and CLI probe integration.

## Global Constraints

- Preserve ALPN entry order while trimming whitespace and dropping empty entries.
- Accept a comma-separated string or an array containing only strings.
- Preserve unsupported or partially invalid values in `NodeIR.Raw` and retain the existing unknown-field warning.
- Do not change TLS enablement, probe execution, health classification, or unrelated URI fields.
- Work directly on the current `main` checkout as requested.

---

### Task 1: Parse legacy VMess ALPN

**Files:**
- Modify: `internal/adapter/uri/parser_vmess.go`
- Test: `internal/adapter/uri/parser_test.go`

**Interfaces:**
- Consumes: the decoded legacy VMess `doc["alpn"]` value of type `any`.
- Produces: `parseLegacyVMessALPN(value any) ([]string, bool)`, where the boolean reports whether the complete value had a supported shape; valid empty values return an empty slice and `true`.
- Produces: canonical `node.TLS.ALPN` values consumed by the existing Mihomo and sing-box renderers.

- [ ] **Step 1: Write the failing parser test**

Add a table-driven `TestParseLegacyVMessALPN` to
`internal/adapter/uri/parser_test.go`. The production mutation it catches is
removing or weakening the legacy `alpn` normalization branch.

```go
func TestParseLegacyVMessALPN(t *testing.T) {
	p := uri.NewParser()
	tests := []struct {
		name         string
		alpnJSON     string
		wantALPN     []string
		wantRawALPN  bool
		wantWarning bool
	}{
		{
			name:     "comma separated string",
			alpnJSON: `" h2, http/1.1, "`,
			wantALPN: []string{"h2", "http/1.1"},
		},
		{
			name:     "string array",
			alpnJSON: `[" h2 ", "", "http/1.1"]`,
			wantALPN: []string{"h2", "http/1.1"},
		},
		{
			name:     "empty string is consumed",
			alpnJSON: `""`,
		},
		{
			name:     "empty array is consumed",
			alpnJSON: `[]`,
		},
		{
			name:     "whitespace string is consumed",
			alpnJSON: `" , "`,
		},
		{
			name:     "empty array entries are consumed",
			alpnJSON: `[" ", ""]`,
		},
		{
			name:         "mixed array remains raw",
			alpnJSON:     `["h2", 7]`,
			wantRawALPN:  true,
			wantWarning: true,
		},
		{
			name:         "null remains raw",
			alpnJSON:     `null`,
			wantRawALPN:  true,
			wantWarning: true,
		},
		{
			name:         "object remains raw",
			alpnJSON:     `{}`,
			wantRawALPN:  true,
			wantWarning: true,
		},
		{
			name:         "number remains raw",
			alpnJSON:     `7`,
			wantRawALPN:  true,
			wantWarning: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := `{"v":"2","ps":"vmess-alpn","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","alpn":` + tc.alpnJSON + `}`
			raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(doc))

			nodes, source, err := p.Parse(context.Background(), []byte(raw))

			require.NoError(t, err)
			require.Len(t, nodes, 1)
			if len(tc.wantALPN) > 0 {
				require.NotNil(t, nodes[0].TLS)
				require.False(t, nodes[0].TLS.Enabled)
				require.Equal(t, tc.wantALPN, nodes[0].TLS.ALPN)
			} else {
				require.Nil(t, nodes[0].TLS)
			}
			if tc.wantRawALPN {
				require.Contains(t, nodes[0].Raw, "vmess.alpn")
			} else {
				require.NotContains(t, nodes[0].Raw, "vmess.alpn")
			}
			if tc.wantWarning {
				require.Contains(t, warningFields(source.Warnings), "vmess.alpn")
			} else {
				require.NotContains(t, warningFields(source.Warnings), "vmess.alpn")
			}
		})
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
go test -mod=readonly ./internal/adapter/uri -run '^TestParseLegacyVMessALPN$' -count=1
```

Expected: FAIL in the valid cases because `TLS.ALPN` is empty and
`raw["vmess.alpn"]` is still present.

- [ ] **Step 3: Add strict legacy ALPN normalization**

Add this focused helper to `internal/adapter/uri/parser_vmess.go`:

```go
func parseLegacyVMessALPN(value any) ([]string, bool) {
	var values []string
	switch typed := value.(type) {
	case string:
		values = []string{typed}
	case []any:
		values = make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			values = append(values, text)
		}
	default:
		return nil, false
	}
	alpn := make([]string, 0, len(values))
	for _, value := range values {
		alpn = append(alpn, splitList(value)...)
	}
	return alpn, true
}
```

In `parseLegacyVMess`, normalize `doc["alpn"]` after the existing TLS
fingerprint fields:

```go
alpn, alpnKnown := parseLegacyVMessALPN(doc["alpn"])
if alpnKnown {
	node.TLS.ALPN = alpn
}
```

Include `len(node.TLS.ALPN) > 0` in the condition that decides whether to keep
`node.TLS`. Move the literal known-field map into a local `knownFields`
variable, add `knownFields["alpn"] = true` only when `alpnKnown`, and pass that
map to `shared.AddUnknownRaw`.

- [ ] **Step 4: Run focused and package tests and verify GREEN**

Run:

```bash
go test -mod=readonly ./internal/adapter/uri -run '^TestParseLegacyVMessALPN$' -count=1
go test -mod=readonly ./internal/adapter/uri -count=1
```

Expected: both commands PASS with no test failures.

- [ ] **Step 5: Verify both render/probe paths against `local2`**

Build the two tagged CLIs without overwriting the repository binary:

```bash
go build -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls -o /tmp/sandrone-alpn-singbox ./cmd/sandrone
go build -mod=readonly -tags probe_mihomo,with_quic,with_wireguard,with_utls -o /tmp/sandrone-alpn-mihomo ./cmd/sandrone
```

Pipe `data/subscriptions/local2.json` content into each CLI using the existing
URL-test parameters and aggregate `report.warnings` by code and field. Expected:
neither result contains `parse_unknown_field` nor `render_lossy_field` for
`vmess.alpn`; probe success counts are diagnostic only.

- [ ] **Step 6: Run repository gate**

Run:

```bash
make check
```

Expected: formatting, vet, tests, and build checks all pass.

- [ ] **Step 7: Commit the implementation**

```bash
git add internal/adapter/uri/parser_vmess.go internal/adapter/uri/parser_test.go
git commit -m "fix(uri): parse legacy vmess alpn"
```
