# VMessAEAD URL Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Import Discussion #716-style `vmess://UUID@host:port?...` links while preserving legacy Base64-JSON VMess input and output behavior.

**Architecture:** Keep `parseVMess` as the registered adapter entrypoint and dispatch by the unambiguous `@` marker to a new URL-profile parser or the unchanged legacy parser. Reuse the existing URI TLS, transport, XHTTP, raw-field, warning, and source-metadata infrastructure; do not add domain fields or change renderers.

**Tech Stack:** Go 1.25.11, `net/url`, existing Sandrone `NodeIR`, `testify/require`.

## Global Constraints

- VMessAEAD URL support is additive import compatibility only.
- Legacy standard and URL-safe Base64-JSON VMess imports must remain unchanged.
- VMess URI rendering must remain legacy Base64 JSON.
- The URL profile requires UUID userinfo, host, and an explicit port in `1..65535`.
- Exact duplicate query keys must fail with `parse_failed`.
- Unsupported query fields must remain in `NodeIR.Raw` and surface through existing structured warnings.
- No new `NodeIR` fields, renderer format, processor, entrypoint, subscription type, or `FileSpec.kind`.
- VMessAEAD URL input uses a Discussion #716 source reference; legacy input keeps its existing source reference.
- File-stage and adapter/service dependency boundaries from `AGENTS.md` remain unchanged.

---

### Task 1: Add the dual VMess parser and basic URL-profile normalization

**Files:**
- Modify: `internal/adapter/uri/parser_vmess.go`
- Modify: `internal/adapter/uri/parser_test.go`
- Modify: `internal/adapter/uri/renderer_test.go`
- Modify: `internal/adapter/shared/source.go`
- Modify: `internal/adapter/shared/source_test.go`

**Interfaces:**
- Consumes: existing `shared.ParseURLHostPort`, `shared.DecodeName`, `applyTLSQuery`, `applyTransportQuery`, `preserveURIQuery`.
- Produces: `parseVMessAEAD(raw string) (domain.NodeIR, *domain.SourceInfo, error)` and the existing `parseVMess` entrypoint with dual-format dispatch.

- [ ] **Step 1: Write failing behavior tests**

Add these tests near the existing VMess parser tests:

```go
func TestParseVMessAEADURL(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443#vmess-aead"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, "vmess", source.Format)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, domain.NodeTypeVMess, got.Type)
	require.Equal(t, "vmess-aead", got.Name)
	require.Equal(t, "example.com", got.Server)
	require.Equal(t, uint16(443), got.Port)
	require.Equal(t, "11111111-1111-1111-1111-111111111111", got.UUID)
	require.Equal(t, "auto", got.Cipher)
	require.Zero(t, got.AlterID)
	require.Nil(t, got.TLS)
	require.Nil(t, got.Transport)
	require.Empty(t, got.Raw)
}

func TestParseVMessAEADWebSocketTLSURL(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?encryption=zero&security=tls&type=ws&host=cdn.example.com&path=%2Fws&sni=sni.example.com#VMess%20AEAD"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.Equal(t, "VMess AEAD", got.Name)
	require.Equal(t, "zero", got.Cipher)
	require.NotNil(t, got.TLS)
	require.True(t, got.TLS.Enabled)
	require.Equal(t, "sni.example.com", got.TLS.ServerName)
	require.NotNil(t, got.Transport)
	require.Equal(t, "websocket", got.Transport.Type)
	require.Equal(t, "cdn.example.com", got.Transport.Host)
	require.Equal(t, "/ws", got.Transport.Path)
	require.Equal(t, map[string]string{"Host": "cdn.example.com"}, got.Transport.Headers)
	require.Empty(t, got.Raw)
}

func TestParseVMessAEADIPv6URL(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@[2001:db8::1]:8443?encryption=auto#ipv6"

	nodes, _, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "2001:db8::1", nodes[0].Server)
	require.Equal(t, uint16(8443), nodes[0].Port)
}

func TestParseVMessAEADRejectsMalformedAuthority(t *testing.T) {
	p := uri.NewParser()
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "missing explicit port",
			raw:  "vmess://11111111-1111-1111-1111-111111111111@example.com#missing-port",
			want: "parse vmess AEAD server",
		},
		{
			name: "password-style userinfo",
			raw:  "vmess://11111111-1111-1111-1111-111111111111:secret@example.com:443#password",
			want: "userinfo must contain only a uuid",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := p.Parse(context.Background(), []byte(tc.raw))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestParseVMessAEADFromBase64URIList(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?encryption=auto#wrapped"
	wrapped := base64.StdEncoding.EncodeToString([]byte(raw + "\n"))

	nodes, source, err := p.ParseList(context.Background(), []byte(wrapped))

	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, "uri-list", source.Format)
	require.Len(t, nodes, 1)
	require.Equal(t, domain.NodeTypeVMess, nodes[0].Type)
	require.Equal(t, "wrapped", nodes[0].Name)
	require.Equal(t, "example.com", nodes[0].Server)
	require.Equal(t, uint16(443), nodes[0].Port)
}

func TestParseVMessUsesProfileSpecificSourceReference(t *testing.T) {
	p := uri.NewParser()
	aead, aeadSource, err := p.Parse(context.Background(), []byte(
		"vmess://11111111-1111-1111-1111-111111111111@example.com:443",
	))
	require.NoError(t, err)
	require.Len(t, aead, 1)
	require.Len(t, aeadSource.SourceRefs, 1)
	require.Equal(t, "VMessAEAD / VLESS sharing link", aeadSource.SourceRefs[0].Name)
	require.Equal(t, "https://github.com/XTLS/Xray-core/discussions/716", aeadSource.SourceRefs[0].URL)

	doc := `{"add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111"}`
	legacy, legacySource, err := p.Parse(context.Background(), []byte(
		"vmess://"+base64.StdEncoding.EncodeToString([]byte(doc)),
	))
	require.NoError(t, err)
	require.Len(t, legacy, 1)
	require.Len(t, legacySource.SourceRefs, 1)
	require.Equal(t, "vmess", legacySource.SourceRefs[0].Name)
}
```

In `renderer_test.go`, add `encoding/base64` to the imports and add:

```go
func TestRenderParsedVMessAEADStillUsesLegacyBase64JSON(t *testing.T) {
	p := uri.NewParser()
	nodes, _, err := p.Parse(context.Background(), []byte(
		"vmess://11111111-1111-1111-1111-111111111111@example.com:443?encryption=zero#vmess-aead",
	))
	require.NoError(t, err)

	body, _, err := uri.NewRenderer().RenderWithReport(
		context.Background(), nodes, domain.RenderOptions{},
	)
	require.NoError(t, err)
	line := strings.TrimSpace(string(body))
	require.True(t, strings.HasPrefix(line, "vmess://"))
	decoded, ok := decodeTestBase64(strings.TrimPrefix(line, "vmess://"))
	require.True(t, ok)
	require.JSONEq(t, `{
		"v":"2",
		"ps":"vmess-aead",
		"add":"example.com",
		"port":"443",
		"id":"11111111-1111-1111-1111-111111111111",
		"aid":"0",
		"scy":"zero",
		"net":"tcp",
		"type":"none"
	}`, string(decoded))
}

func decodeTestBase64(raw string) ([]byte, bool) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		if decoded, err := encoding.DecodeString(raw); err == nil {
			return decoded, true
		}
	}
	return nil, false
}
```

Extend `internal/adapter/shared/source_test.go` `TestSourceRefs` with:

```go
{format: "vmess-aead", name: "VMessAEAD / VLESS sharing link", kind: "protocol", count: 1},
```

- [ ] **Step 2: Verify the tests fail for the missing URL parser**

Run:

```bash
go test ./internal/adapter/uri ./internal/adapter/shared -run 'TestParseVMessAEAD|TestParseVMessUsesProfileSpecificSourceReference|TestRenderParsedVMessAEAD|TestSourceRefs' -count=1
```

Expected: the URL, URI-list, malformed-authority message, and parse-then-render
tests fail because input still takes the existing `decode vmess payload` path.

- [ ] **Step 3: Add the dual-format dispatcher and URL parser**

Rename the current `parseVMess` implementation to `parseLegacyVMess` without
changing its body. Add the `net/url` import and add:

```go
func parseVMess(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	payload := strings.TrimPrefix(raw, "vmess://")
	if strings.Contains(payload, "@") {
		return parseVMessAEAD(raw)
	}
	return parseLegacyVMess(raw)
}

func parseVMessAEAD(raw string) (domain.NodeIR, *domain.SourceInfo, error) {
	node := domain.NodeIR{Type: domain.NodeTypeVMess, SourceFormat: "uri"}
	source := shared.SourceInfo("vmess", shared.SourceRefs("vmess-aead"))
	u, err := url.Parse(raw)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse vmess AEAD URI", err)
	}
	if u.User == nil || u.User.Username() == "" {
		return node, source, domain.NewError(domain.CodeParseFailed, "missing vmess uuid")
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		return node, source, domain.NewError(domain.CodeParseFailed, "vmess AEAD userinfo must contain only a uuid")
	}
	host, port, err := shared.ParseURLHostPort(u, "")
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse vmess AEAD server", err)
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return node, source, domain.WrapError(domain.CodeParseFailed, "parse vmess AEAD query", err)
	}

	node.Name = shared.DecodeName(u.Fragment, host)
	node.Server = host
	node.Port = port
	node.UUID = u.User.Username()
	node.Cipher = firstNonEmpty(values.Get("encryption"), "auto")
	node.PacketEncoding = shared.QueryFirst(values, "packetEncoding", "packet-encoding")
	applyTLSQuery(&node, values)
	applyTransportQuery(&node, values)
	node.Raw = map[string]json.RawMessage{}
	known := vmessAEADKnownQueryFields(node)
	preserveURIQuery(&node, values, known)
	return node, source, nil
}

func vmessAEADKnownQueryFields(node domain.NodeIR) map[string]bool {
	known := map[string]bool{
		"encryption": true, "packetEncoding": true, "packet-encoding": true,
		"security": true, "tls": true, "sni": true, "servername": true, "serverName": true,
		"fp": true, "fingerprint": true, "pinSHA256": true, "pcs": true, "alpn": true,
		"allowInsecure": true, "allow_insecure": true, "allow-insecure": true,
		"skip-cert-verify": true, "insecure": true,
		"pbk": true, "public-key": true, "sid": true, "short-id": true,
		"ech": true, "echForceQuery": true,
		"type": true, "net": true, "transport": true, "host": true, "authority": true,
		"path": true, "wspath": true, "wsPath": true, "ws-path": true, "obfs-uri": true,
		"serviceName": true, "service_name": true,
	}
	return known
}
```

- [ ] **Step 4: Add the branch-specific source reference**

Add this case to `shared.SourceRefs` without changing the existing `vmess`
case:

```go
	case "vmess-aead":
		return []domain.SourceRef{{
			Kind:     "protocol",
			Name:     "VMessAEAD / VLESS sharing link",
			URL:      "https://github.com/XTLS/Xray-core/discussions/716",
			Revision: "accessed-2026-07-30",
			Note:     "XTLS VMessAEAD and VLESS URL sharing profile",
		}}
```

- [ ] **Step 5: Verify basic URL parsing and legacy regression coverage**

Run:

```bash
gofmt -w internal/adapter/uri/parser_vmess.go internal/adapter/uri/parser_test.go internal/adapter/uri/renderer_test.go internal/adapter/shared/source.go internal/adapter/shared/source_test.go
go test ./internal/adapter/uri ./internal/adapter/shared -run 'TestParseVMess|TestRenderParsedVMessAEAD|TestSourceRefs' -count=1
```

Expected: all new VMessAEAD and all pre-existing legacy VMess tests pass with
pristine output.

- [ ] **Step 6: Commit Task 1**

```bash
git add internal/adapter/uri/parser_vmess.go internal/adapter/uri/parser_test.go internal/adapter/uri/renderer_test.go internal/adapter/shared/source.go internal/adapter/shared/source_test.go
git commit -m "feat(uri): import vmess aead urls"
```

---

### Task 2: Enforce duplicate-key safety and promote VMess XHTTP fields

**Files:**
- Modify: `internal/adapter/uri/parser_vmess.go`
- Modify: `internal/adapter/uri/parser_query.go`
- Modify: `internal/adapter/uri/vless_advanced.go`
- Modify: `internal/adapter/uri/parser_test.go`

**Interfaces:**
- Consumes: Task 1 `parseVMessAEAD` and `vmessAEADKnownQueryFields`.
- Produces: deterministic duplicate-key rejection and VMess XHTTP typed promotion.

- [ ] **Step 1: Write failing safety and advanced-field tests**

Add:

```go
func TestParseVMessAEADRejectsDuplicateQueryKey(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?security=tls&security=reality"

	_, _, err := p.Parse(context.Background(), []byte(raw))

	require.Error(t, err)
	require.Contains(t, err.Error(), `duplicate vmess AEAD query parameter "security"`)
}

func TestParseVMessAEADXHTTPRealityAndUnsupportedRaw(t *testing.T) {
	p := uri.NewParser()
	raw := "vmess://11111111-1111-1111-1111-111111111111@example.com:443?encryption=auto&security=reality&pbk=public-key&sid=08&type=xhttp&path=%2Fxhttp&host=cdn.example.com&mode=packet-up&extra=%7B%22xmux%22%3A%7B%22maxConcurrency%22%3A%228-16%22%7D%7D&pqv=mlkem&fm=1#vmess-xhttp"

	nodes, source, err := p.Parse(context.Background(), []byte(raw))

	require.NoError(t, err)
	require.Len(t, nodes, 1)
	got := nodes[0]
	require.NotNil(t, got.TLS)
	require.NotNil(t, got.TLS.Reality)
	require.Equal(t, "public-key", got.TLS.Reality.PublicKey)
	require.Equal(t, "08", got.TLS.Reality.ShortID)
	require.NotNil(t, got.Transport)
	require.Equal(t, "xhttp", got.Transport.Type)
	require.NotNil(t, got.Transport.XHTTP)
	require.Equal(t, "packet-up", got.Transport.XHTTP.Mode)
	require.NotNil(t, got.Transport.XHTTP.ReuseSettings)
	require.Equal(t, "8-16", got.Transport.XHTTP.ReuseSettings.MaxConcurrency)
	require.NotContains(t, got.Raw, "uri.query.mode")
	require.NotContains(t, got.Raw, "uri.query.extra")
	require.JSONEq(t, `"mlkem"`, string(got.Raw["uri.query.pqv"]))
	require.JSONEq(t, `"1"`, string(got.Raw["uri.query.fm"]))
	require.Equal(t, []string{"uri.query.fm", "uri.query.pqv"}, warningFields(source.Warnings))
}

```

- [ ] **Step 2: Verify each new behavior fails for the expected reason**

Run:

```bash
go test ./internal/adapter/uri -run 'TestParseVMessAEADRejectsDuplicateQueryKey|TestParseVMessAEADXHTTPRealityAndUnsupportedRaw' -count=1
```

Expected:

- duplicate-query test fails because duplicates are currently accepted;
- XHTTP test fails because typed `mode`/`extra` promotion is VLESS-only;

- [ ] **Step 3: Reject duplicate query keys**

Add this check immediately after `url.ParseQuery` in `parseVMessAEAD`:

```go
	for _, key := range sortedQueryKeys(values) {
		if len(values[key]) > 1 {
			return node, source, domain.NewError(
				domain.CodeParseFailed,
				fmt.Sprintf("duplicate vmess AEAD query parameter %q", key),
			)
		}
	}
```

Add `fmt` to `parser_vmess.go` imports for the formatted error.

- [ ] **Step 4: Generalize typed XHTTP promotion**

Rename `applyVLESSXHTTPExtra` to `applyXHTTPExtra` in
`internal/adapter/uri/vless_advanced.go`, then replace the VLESS-only branch
at the end of `applyTransportQuery` with:

```go
	if node.Type == domain.NodeTypeVLESS || node.Type == domain.NodeTypeVMess {
		applyXHTTPExtra(node.Transport, values)
	}
```

Then add this conditional before `return known` in
`vmessAEADKnownQueryFields`:

```go
	if node.Transport != nil && node.Transport.Type == "xhttp" {
		known["mode"] = true
		known["extra"] = true
	}
```

- [ ] **Step 5: Verify advanced behavior and the affected packages**

Run:

```bash
gofmt -w internal/adapter/uri/parser_vmess.go internal/adapter/uri/parser_query.go internal/adapter/uri/vless_advanced.go internal/adapter/uri/parser_test.go
go test ./internal/adapter/uri ./internal/adapter/shared -count=1
```

Expected: both packages pass with pristine output.

- [ ] **Step 6: Commit Task 2**

```bash
git add internal/adapter/uri/parser_vmess.go internal/adapter/uri/parser_query.go internal/adapter/uri/vless_advanced.go internal/adapter/uri/parser_test.go
git commit -m "fix(uri): validate vmess aead profiles"
```

---

### Task 3: Document the asymmetric compatibility boundary

**Files:**
- Modify: `docs/reference/capabilities.md`

**Interfaces:**
- Consumes: completed dual parser and unchanged URI renderer.
- Produces: the canonical documentation for accepted VMess URI dialects and legacy output.

- [ ] **Step 1: Update the canonical capability documentation**

After the URI input paragraphs in `docs/reference/capabilities.md`, add:

```markdown
VMess URI 输入同时接受 legacy `vmess://Base64(JSON)` 与 Discussion #716
风格的 `vmess://UUID@host:port?...` URL；URL 输入只提升当前 `NodeIR` 可表达
的字段，未支持参数保留为 Raw 并产生 warning。VMess URI 输出仍为 legacy
Base64 JSON，不承诺对 #716 的完整或无损往返。
```

- [ ] **Step 2: Verify documentation formatting and affected packages**

Run:

```bash
go test ./internal/adapter/uri ./internal/adapter/shared -count=1
git diff --check
```

Expected: all commands exit zero with pristine test output.

- [ ] **Step 3: Commit Task 3**

```bash
git add docs/reference/capabilities.md
git commit -m "docs: describe vmess aead import compatibility"
```

---

### Task 4: Run repository verification

**Files:**
- No production-file changes expected.
- Modify only files required to correct failures caused by Tasks 1–3.

**Interfaces:**
- Consumes: the complete implementation from Tasks 1–3.
- Produces: fresh evidence that the feature and repository gates pass.

- [ ] **Step 1: Run the exact focused feature tests**

```bash
go test ./internal/adapter/uri ./internal/adapter/shared -count=1
```

- [ ] **Step 2: Run the repository gate**

```bash
make check
```

- [ ] **Step 3: Inspect the final branch state**

```bash
git status --short
git diff --check
git log --oneline --decorate -5
```

Expected: clean worktree, no whitespace errors, and all task commits present.

- [ ] **Step 4: Record verification**

Do not create an empty commit. Record the exact commands, exit codes, and any
environment-related failure in the task report and SDD ledger.
