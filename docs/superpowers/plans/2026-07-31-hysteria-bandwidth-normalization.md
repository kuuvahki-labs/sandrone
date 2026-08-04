# Hysteria Bandwidth Normalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve Hysteria v1 bandwidth semantics across every supported input and output format while isolating target-incompatible nodes from otherwise valid probe batches.

**Architecture:** Keep the public `domain.HysteriaOptions` fields and add shared adapter normalization that produces either an explicit-unit string or an integer Mbps value per direction. Source parsers resolve their own defaults and precedence before `NodeIR`; JSON/inline compatibility normalization repairs legacy ambiguous values; renderers validate and emit only target-native fields.

**Tech Stack:** Go, `encoding/json`, checked integer arithmetic, Testify, existing Mihomo/sing-box adapters, build-tagged sing-box probe integration.

## Global Constraints

- Do not add or rename public `NodeIR` fields.
- After normalization, each direction uses exactly one of `Up`/`UpMbps` or `Down`/`DownMbps`.
- Mihomo bare `up/down` means Mbps; sing-box JSON-number `up/down` means Bps.
- Common and legacy inputs remain parseable; ambiguous assumptions use `parse_implicit_bandwidth_unit`.
- Invalid source values remain diagnosable through `NodeIR.Raw` and structured warnings.
- A bad Hysteria node must not make an otherwise renderable sing-box probe batch fail to decode.
- Tests use synthetic endpoints and credentials only; never commit the real subscription URL, nodes, or local runtime output.
- Preserve Hysteria2 behavior except for regression assertions proving it is unchanged.
- Run relevant narrow tests first and `make check` before completion.

---

### Task 1: Shared Hysteria Rate Normalization

**Files:**
- Create: `internal/adapter/shared/hysteria_bandwidth.go`
- Create: `internal/adapter/shared/hysteria_bandwidth_test.go`

**Interfaces:**
- Produces: `type HysteriaImplicitUnit string` with `HysteriaImplicitNone`, `HysteriaImplicitMbps`, and `HysteriaImplicitBps`.
- Produces: `type HysteriaRate struct { Text string; Mbps int }`.
- Produces: `NormalizeHysteriaRate(raw string, implicit HysteriaImplicitUnit) (HysteriaRate, error)`.
- Produces: `ValidateCanonicalHysteriaBandwidth(options *domain.HysteriaOptions) error`.
- Consumed by: all parser and renderer tasks below.

- [ ] **Step 1: Write table-driven failing normalization tests**

Create `internal/adapter/shared/hysteria_bandwidth_test.go` with:

```go
func TestNormalizeHysteriaRate(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		implicit shared.HysteriaImplicitUnit
		want     shared.HysteriaRate
	}{
		{name: "mihomo bare Mbps", raw: "55", implicit: shared.HysteriaImplicitMbps, want: shared.HysteriaRate{Mbps: 55}},
		{name: "sing-box numeric Bps", raw: "55", implicit: shared.HysteriaImplicitBps, want: shared.HysteriaRate{Text: "55 Bps"}},
		{name: "whole Mbps string", raw: " 55Mbps ", implicit: shared.HysteriaImplicitNone, want: shared.HysteriaRate{Mbps: 55}},
		{name: "non-integral Mbps", raw: "640 KBps", implicit: shared.HysteriaImplicitNone, want: shared.HysteriaRate{Text: "640 KBps"}},
		{name: "exact byte conversion", raw: "125 KBps", implicit: shared.HysteriaImplicitNone, want: shared.HysteriaRate{Mbps: 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := shared.NormalizeHysteriaRate(test.raw, test.implicit)
			require.NoError(t, err)
			require.Equal(t, test.want, got)
			require.True(t, got.Text == "" || got.Mbps == 0)
		})
	}
}
```

Add an error table for `""`, `"0"`, `"-1"`, a bare number with `HysteriaImplicitNone`, `"10 mbps"`, `"1.5 Mbps"`, `"10 XBps"`, and multiplication overflow. Where the current architecture permits a whole-Mbps bit rate that fits `uint64` but exceeds `int`, add a success case proving it remains an explicit `Text` value rather than overflowing the integer field.

- [ ] **Step 2: Write failing canonical preflight tests**

```go
func TestValidateCanonicalHysteriaBandwidth(t *testing.T) {
	require.NoError(t, shared.ValidateCanonicalHysteriaBandwidth(&domain.HysteriaOptions{
		UpMbps: 55, Down: "640 KBps",
	}))
	for _, options := range []*domain.HysteriaOptions{
		nil,
		{Up: "55", DownMbps: 100},
		{Up: "125 KBps", DownMbps: 100},
		{Up: "55 Mbps", UpMbps: 55, DownMbps: 100},
		{UpMbps: 55},
		{UpMbps: -1, DownMbps: 100},
	} {
		require.Error(t, shared.ValidateCanonicalHysteriaBandwidth(options))
	}
}
```

- [ ] **Step 3: Run the shared tests and verify RED**

```bash
go test -mod=readonly ./internal/adapter/shared -run 'TestNormalizeHysteriaRate|TestValidateCanonicalHysteriaBandwidth' -count=1
```

Expected: compilation fails because the new types and functions do not exist.

- [ ] **Step 4: Implement the minimal shared normalizer**

Create `internal/adapter/shared/hysteria_bandwidth.go` beginning with:

```go
type HysteriaImplicitUnit string

const (
	HysteriaImplicitNone HysteriaImplicitUnit = ""
	HysteriaImplicitMbps HysteriaImplicitUnit = "Mbps"
	HysteriaImplicitBps  HysteriaImplicitUnit = "Bps"
)

type HysteriaRate struct {
	Text string
	Mbps int
}

var hysteriaRateBitMultipliers = map[string]uint64{
	"bps": 1, "Bps": 8,
	"Kbps": 1_000, "KBps": 8_000,
	"Mbps": 1_000_000, "MBps": 8_000_000,
	"Gbps": 1_000_000_000, "GBps": 8_000_000_000,
	"Tbps": 1_000_000_000_000, "TBps": 8_000_000_000_000,
}
```

Implement `NormalizeHysteriaRate` by trimming input, splitting leading ASCII digits from the unit, applying `implicit` only when no unit exists, rejecting zero/malformed input, checking multiplication with `math.MaxUint64/value`, and promoting exact whole Mbps only when it fits platform `int`. Otherwise return `Text: strconv.FormatUint(value, 10) + " " + unit`.

Implement `ValidateCanonicalHysteriaBandwidth` with one private per-direction helper: exactly one field is populated, Mbps is positive, and text re-normalizes with `HysteriaImplicitNone` to the identical `Text` value. A text value such as `125 KBps` that should have been promoted to integer Mbps is therefore non-canonical.

- [ ] **Step 5: Run shared tests and verify GREEN**

Run the Step 3 command. Expected: PASS.

- [ ] **Step 6: Format and commit**

```bash
gofmt -w internal/adapter/shared/hysteria_bandwidth.go internal/adapter/shared/hysteria_bandwidth_test.go
git add internal/adapter/shared/hysteria_bandwidth.go internal/adapter/shared/hysteria_bandwidth_test.go
git commit -m "feat(nodes): normalize hysteria bandwidth rates"
```

---

### Task 2: Normalize Mihomo and sing-box Source Semantics

**Files:**
- Modify: `internal/adapter/mihomo/parser.go:449`
- Modify: `internal/adapter/mihomo/parser_test.go`
- Modify: `internal/adapter/singbox/parser.go:304`
- Modify: `internal/adapter/singbox/parser_test.go`

**Interfaces:**
- Consumes: `shared.NormalizeHysteriaRate` and Task 1 constants.
- Produces: mutually exclusive Hysteria v1 bandwidth fields and raw preservation for invalid source values.

- [ ] **Step 1: Add failing Mihomo parser tests**

Add `TestParseMihomoNormalizesHysteriaBandwidth` using:

```yaml
proxies:
  - {name: bare, type: hysteria, server: bare.example, port: 443, up: "11", down: "55", auth-str: secret}
  - {name: units, type: hysteria, server: units.example, port: 443, up: "640 KBps", down: "1 Gbps", auth-str: secret}
  - {name: compat, type: hysteria, server: compat.example, port: 443, up: "11 Mbps", down: "55 Mbps", up-speed: 20, down-speed: 100, auth-str: secret}
```

Assert `bare` becomes `UpMbps: 11`/`DownMbps: 55`, `units` keeps `Up: "640 KBps"` and promotes `DownMbps: 1000`, and `compat` keeps only `UpMbps: 20`/`DownMbps: 100`.

Add `TestParseMihomoPreservesInvalidHysteriaBandwidthAsRaw` with `up: fast`; assert typed upload is empty, `mihomo.up` exists in `Raw`, and the source report contains `parse_unknown_field`.

In the same test, cover an invalid negative `up-speed` alongside a valid `up`; assert the valid native `up` is used while `mihomo.up-speed` is preserved in Raw. Zero compatibility values remain absent and do not override native fields.

- [ ] **Step 2: Run Mihomo tests and verify RED**

```bash
go test -mod=readonly ./internal/adapter/mihomo -run 'TestParseMihomo(Normalizes|PreservesInvalid)HysteriaBandwidth' -count=1
```

Expected: bare values remain strings, both compatibility forms remain populated, and invalid values are not raw.

- [ ] **Step 3: Implement Mihomo normalization**

Allocate `HysteriaOptions` without rate fields, then call:

```go
func applyMihomoHysteriaRate(node *domain.NodeIR, value, speedValue any, rawKey, speedKey string, text *string, mbps *int) {
	if speedValue != nil {
		speed, err := shared.IntValue(speedValue)
		if err != nil || speed < 0 {
			shared.AddRaw(node.Raw, "mihomo."+speedKey, speedValue)
		} else if speed > 0 {
			*mbps = speed
			return
		}
	}
	raw := strings.TrimSpace(shared.StringValue(value))
	if raw == "" {
		return
	}
	rate, err := shared.NormalizeHysteriaRate(raw, shared.HysteriaImplicitMbps)
	if err != nil {
		shared.AddRaw(node.Raw, "mihomo."+rawKey, value)
		return
	}
	*text, *mbps = rate.Text, rate.Mbps
}
```

Call it for both directions with the original map values, not `intValueZero`, so invalid compatibility values remain diagnosable. Keep Hysteria2 unchanged.

- [ ] **Step 4: Run Mihomo tests and verify GREEN**

Run Step 2, then:

```bash
go test -mod=readonly ./internal/adapter/mihomo -count=1
```

Expected: PASS.

- [ ] **Step 5: Add failing sing-box parser tests**

Add `TestParseSingBoxNormalizesHysteriaBandwidth` using:

```json
{"outbounds":[
  {"type":"hysteria","tag":"bytes","server":"bytes.example","server_port":443,"up":55,"down":100,"auth_str":"secret"},
  {"type":"hysteria","tag":"units","server":"units.example","server_port":443,"up":"640 KBps","down":"1 Gbps","auth_str":"secret"},
  {"type":"hysteria","tag":"precedence","server":"precedence.example","server_port":443,"up":"55 Mbps","down":"100 Mbps","up_mbps":20,"down_mbps":30,"auth_str":"secret"}
]}
```

Assert JSON numbers become `"55 Bps"`/`"100 Bps"`, explicit whole Mbps promotes, and non-empty `up/down` wins over `up_mbps/down_mbps`. Add an invalid explicit string case and assert `sing-box.up` is raw with `parse_unknown_field`; add an invalid negative `up_mbps` case with no `up` and assert it is cleared and preserved as `sing-box.up_mbps`.

- [ ] **Step 6: Run sing-box tests and verify RED**

```bash
go test -mod=readonly ./internal/adapter/singbox -run 'TestParseSingBox(Normalizes|PreservesInvalid)HysteriaBandwidth' -count=1
```

Expected: JSON numbers lose Bps type and both field forms remain populated.

- [ ] **Step 7: Implement sing-box normalization**

Use:

```go
func singBoxHysteriaRate(node *domain.NodeIR, value, fallbackValue any, rawKey, fallbackKey string) shared.HysteriaRate {
	if value == nil || strings.TrimSpace(shared.StringValue(value)) == "" {
		if fallbackValue == nil {
			return shared.HysteriaRate{}
		}
		fallbackMbps, err := shared.IntValue(fallbackValue)
		if err != nil || fallbackMbps < 0 {
			shared.AddRaw(node.Raw, "sing-box."+fallbackKey, fallbackValue)
			return shared.HysteriaRate{}
		}
		return shared.HysteriaRate{Mbps: fallbackMbps}
	}
	implicit := shared.HysteriaImplicitNone
	if _, ok := value.(json.Number); ok {
		implicit = shared.HysteriaImplicitBps
	}
	rate, err := shared.NormalizeHysteriaRate(shared.StringValue(value), implicit)
	if err != nil {
		shared.AddRaw(node.Raw, "sing-box."+rawKey, value)
		return shared.HysteriaRate{}
	}
	return rate
}
```

Treat zero fallback values as absent. A present non-empty `up/down` remains authoritative even when invalid: preserve it in Raw and do not fall back to the compatibility field. Assign only the returned canonical field. Keep Hysteria2 unchanged.

- [ ] **Step 8: Run sing-box tests and verify GREEN**

Run Step 6, then the complete sing-box package. Expected: PASS.

- [ ] **Step 9: Format and commit**

```bash
gofmt -w internal/adapter/mihomo/parser.go internal/adapter/mihomo/parser_test.go internal/adapter/singbox/parser.go internal/adapter/singbox/parser_test.go
git add internal/adapter/mihomo/parser.go internal/adapter/mihomo/parser_test.go internal/adapter/singbox/parser.go internal/adapter/singbox/parser_test.go
git commit -m "fix(adapter): preserve hysteria source rate semantics"
```

---

### Task 3: Normalize URI, JSON Nodes, and Inline Nodes

**Files:**
- Modify: `internal/adapter/uri/parser_protocols.go:153`
- Modify: `internal/adapter/uri/parser_test.go`
- Modify: `internal/adapter/jsonnodes/jsonnodes.go:19`
- Modify: `internal/adapter/jsonnodes/jsonnodes_test.go`
- Modify: `internal/adapter/shared/hysteria_bandwidth.go`
- Modify: `internal/adapter/shared/hysteria_bandwidth_test.go`
- Modify: `internal/service/file_inputs.go:11`
- Modify: `internal/service/probe_test.go`

**Interfaces:**
- Produces: `shared.NormalizeLegacyHysteriaBandwidth(node *domain.NodeIR) []domain.Warning`.
- Produces: `parse_implicit_bandwidth_unit` warnings attached to parser source, node warnings, or `NodeSet.Warnings`.

- [ ] **Step 1: Add failing legacy normalization tests**

```go
func TestNormalizeLegacyHysteriaBandwidthUsesSourceProvenance(t *testing.T) {
	tests := []struct {
		source string
		want   domain.HysteriaOptions
		warn   bool
	}{
		{source: "mihomo", want: domain.HysteriaOptions{UpMbps: 55, DownMbps: 100}},
		{source: "uri", want: domain.HysteriaOptions{UpMbps: 55, DownMbps: 100}},
		{source: "sing-box", want: domain.HysteriaOptions{Up: "55 Bps", Down: "100 Bps"}},
		{source: "json-nodes", want: domain.HysteriaOptions{UpMbps: 55, DownMbps: 100}, warn: true},
		{source: "", want: domain.HysteriaOptions{UpMbps: 55, DownMbps: 100}, warn: true},
	}
	for _, test := range tests {
		node := domain.NodeIR{Name: "hy", Type: domain.NodeTypeHysteria, SourceFormat: test.source, Hysteria: &domain.HysteriaOptions{Up: "55", Down: "100"}}
		warnings := shared.NormalizeLegacyHysteriaBandwidth(&node)
		require.Equal(t, test.want, *node.Hysteria)
		require.Equal(t, test.warn, len(warnings) == 2)
	}
}
```

Add cases for source-specific precedence when both canonical forms exist and invalid input moving to `Raw["json-nodes.hysteria.up"]`.

- [ ] **Step 2: Run the test and verify RED**

```bash
go test -mod=readonly ./internal/adapter/shared -run TestNormalizeLegacyHysteriaBandwidth -count=1
```

Expected: compilation fails because the function is absent.

- [ ] **Step 3: Implement legacy normalization**

For each direction:

- use implicit Bps only for `source_format == "sing-box"`;
- use implicit Mbps for Mihomo/URI/Base64, empty, JSON Nodes, and unknown provenance;
- let sing-box non-empty text win over Mbps; let Mihomo-family/unknown positive Mbps win over text;
- emit `parse_implicit_bandwidth_unit` only for empty, `json-nodes`, or unknown provenance;
- move invalid values to `json-nodes.hysteria.up` or `.down`, clear typed fields, and return one node-scoped `parse_unknown_field` warning for each newly preserved value.

Message: `bare Hysteria bandwidth assumed to be Mbps`.

- [ ] **Step 4: Run shared tests and verify GREEN**

Run Step 2. Expected: PASS.

- [ ] **Step 5: Add failing URI tests**

Cover:

```text
hysteria://example.com:8443?auth=secret&up=55&down=100#bare
hysteria://example.com:8443?auth=secret&up=640KBps&down=1Gbps#units
hysteria://example.com:8443?auth=secret&up=bad&downmbps=100#invalid
hysteria://example.com:8443?auth=secret&upmbps=-1&up=20Mbps&downmbps=100#invalid-compat
```

Assert bare values become Mbps with two implicit warnings, units normalize without those warnings, invalid upload becomes `uri.query.up` raw, and invalid `upmbps` is preserved as `uri.query.upmbps` while the valid explicit `up` is used. Assert Base64 wrapping produces identical canonical fields.

- [ ] **Step 6: Run URI tests and verify RED**

```bash
go test -mod=readonly ./internal/adapter/uri -run 'TestParseHysteria.*Bandwidth' -count=1
```

Expected: values remain unnormalized and warnings are absent.

- [ ] **Step 7: Implement URI normalization**

Give positive `upmbps/downmbps` precedence. Otherwise normalize explicit `up/down`; use implicit Mbps only for bare digits and append `parse_implicit_bandwidth_unit` to `SourceInfo`. Exclude invalid rate keys from the known query map so `preserveURIQuery` stores them under `uri.query.*`.

- [ ] **Step 8: Run URI tests and verify GREEN**

Run Step 6 and the complete URI package. Expected: PASS.

- [ ] **Step 9: Add failing JSON and inline tests**

In JSON Nodes, parse `source_format: "sing-box"` with bare upload/download and assert Bps strings. Parse without provenance and assert Mbps plus two warnings.

In `internal/service/probe_test.go`, construct the service with `WithProbeEngine(fakeProbeEngine{...})`, call `Probe` with method `tcp_connect` and `inline_nodes` containing a valid Hysteria node with bare rates, and capture the nodes received by the fake engine. Assert the captured node contains `UpMbps`/`DownMbps`, the result report contains two `parse_implicit_bandwidth_unit` warnings, and the caller-owned input slice is unchanged. This public path exercises `resolveNodeInputWithSubscriptionState` without network access or a core payload.

Also add end-to-end JSON Nodes render assertions for four inputs: legacy JSON Nodes with bare fields, Mihomo with bare rates, sing-box with numeric rates, and URI with `upmbps/downmbps`. Render each parser's returned nodes with `jsonnodes.NewRenderer`; verify the JSON contains only the normalized mutually exclusive fields and no typed `up`/`down` value matching `^[0-9]+$`. The legacy JSON case drives RED; the other three lock the cross-format boundary after their parser tasks are GREEN.

- [ ] **Step 10: Run JSON/service tests and verify RED**

```bash
go test -mod=readonly ./internal/adapter/jsonnodes ./internal/service -run 'TestParserNormalizesLegacyHysteriaBandwidth|TestService.*InlineHysteriaBandwidth' -count=1
```

Expected: bare strings and missing warnings.

- [ ] **Step 11: Apply normalization at both entry paths**

In JSON Nodes, normalize every decoded node and append returned warnings to `node.Warnings`.

In `resolveNodeInputWithSubscriptionState`, route both `inline_nodes` and `inline` with `input.Nodes` through:

```go
func normalizeInlineNodes(nodes []domain.NodeIR) ([]domain.NodeIR, []domain.Warning) {
	out := make([]domain.NodeIR, len(nodes))
	copy(out, nodes)
	warnings := []domain.Warning{}
	for i := range out {
		if out[i].Hysteria != nil {
			hysteria := *out[i].Hysteria
			out[i].Hysteria = &hysteria
		}
		if out[i].Raw != nil {
			raw := make(map[string]json.RawMessage, len(out[i].Raw))
			for key, value := range out[i].Raw {
				raw[key] = append(json.RawMessage(nil), value...)
			}
			out[i].Raw = raw
		}
		warnings = append(warnings, shared.NormalizeLegacyHysteriaBandwidth(&out[i])...)
	}
	return out, warnings
}
```

Import `encoding/json` for the Raw clone. Return warnings in `NodeSet.Warnings`; cloning the nested Hysteria pointer and Raw map ensures normalization does not mutate caller-owned nodes, not merely the outer slice.

- [ ] **Step 12: Run JSON/service tests and verify GREEN**

Run Step 10, then both full packages. Expected: PASS.

- [ ] **Step 13: Format and commit**

```bash
gofmt -w internal/adapter/shared/hysteria_bandwidth.go internal/adapter/shared/hysteria_bandwidth_test.go internal/adapter/uri/parser_protocols.go internal/adapter/uri/parser_test.go internal/adapter/jsonnodes/jsonnodes.go internal/adapter/jsonnodes/jsonnodes_test.go internal/service/file_inputs.go internal/service/probe_test.go
git add internal/adapter/shared/hysteria_bandwidth.go internal/adapter/shared/hysteria_bandwidth_test.go internal/adapter/uri/parser_protocols.go internal/adapter/uri/parser_test.go internal/adapter/jsonnodes/jsonnodes.go internal/adapter/jsonnodes/jsonnodes_test.go internal/service/file_inputs.go internal/service/probe_test.go
git commit -m "fix(nodes): normalize legacy hysteria bandwidth input"
```

---

### Task 4: Emit Target-native Bandwidth

**Files:**
- Modify: `internal/adapter/shared/hysteria_bandwidth.go`
- Modify: `internal/adapter/shared/hysteria_bandwidth_test.go`
- Modify: `internal/adapter/mihomo/hysteria.go:10`
- Modify: `internal/adapter/mihomo/renderer_test.go`
- Modify: `internal/adapter/singbox/renderer.go:186`
- Modify: `internal/adapter/singbox/renderer_test.go`
- Create: `internal/adapter/singbox/renderer_probe_singbox_test.go`
- Modify: `internal/adapter/uri/renderer.go:245`
- Modify: `internal/adapter/uri/renderer_warnings.go`
- Modify: `internal/adapter/uri/renderer_test.go`
- Modify: `internal/adapter/shadowrocket/protocols.go:164`
- Modify: `internal/adapter/shadowrocket/warnings.go`
- Modify: `internal/adapter/shadowrocket/renderer_test.go`

**Interfaces:**
- Consumes: `shared.ValidateCanonicalHysteriaBandwidth`.
- Produces: Mihomo native explicit `up/down`, locked-core-safe sing-box outbounds, and exact Mbps URI/Shadowrocket fields.

- [ ] **Step 1: Add failing Mihomo renderer tests**

For `UpMbps: 55, DownMbps: 100`, assert YAML contains native `up: 55 Mbps` and `down: 100 Mbps`, with no `up-speed/down-speed`. Decode those strings with the locked Mihomo `common/utils.StringToBps` helper and assert the effective byte rates are `6_875_000` and `12_500_000`. Parse a sing-box fixture with numeric `up: 55`/`down: 100`, render it to Mihomo, and assert the same helper returns exactly 55/100 Bps rather than Mbps. For direct `Up: "55 Bps", Down: "640 KBps"`, assert exact explicit strings. In a mixed valid/invalid batch where invalid has `Up: "55"`, assert one success and one `render_node_skipped`.

Update the existing official-URI-to-Mihomo assertion from `up-speed/down-speed` to native explicit `up/down`, and verify its effective rate with the same locked helper.

- [ ] **Step 2: Run Mihomo tests and verify RED**

```bash
go test -mod=readonly ./internal/adapter/mihomo -run 'TestRenderMihomoHysteria.*Bandwidth' -count=1
```

Expected: Mbps still uses compatibility keys and invalid text is emitted.

- [ ] **Step 3: Implement Mihomo rendering**

Call `ValidateCanonicalHysteriaBandwidth` first. Render integers with:

```go
if hy.UpMbps > 0 {
	out["up"] = strconv.Itoa(hy.UpMbps) + " Mbps"
} else {
	out["up"] = hy.Up
}
```

Apply the same rule to download and stop emitting `up-speed/down-speed` for canonical values.

- [ ] **Step 4: Run Mihomo tests and verify GREEN**

Run Step 2 and the complete package. Expected: PASS.

- [ ] **Step 5: Add failing sing-box locked-core tests**

Create `renderer_probe_singbox_test.go` with `//go:build probe_singbox`. Render one Mbps Hysteria and one explicit-unit Hysteria, then decode the document with:

```go
boxContext := include.Context(context.Background())
var options option.Options
require.NoError(t, options.UnmarshalJSONContext(boxContext, out))
```

Add a mixed invalid Hysteria + valid HTTP batch; assert one outbound and one `render_node_skipped`. Parse the Task 2 Mihomo bare-rate fixture and the Task 3 URI Mbps fixture, render each to sing-box, assert `up_mbps/down_mbps`, and run both through the locked decoder.

- [ ] **Step 6: Run sing-box tests and verify RED**

```bash
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/adapter/singbox -run 'TestRenderSingBoxHysteria.*Bandwidth' -count=1
```

Expected: invalid text reaches output or locked decode fails.

- [ ] **Step 7: Implement sing-box preflight**

Call `ValidateCanonicalHysteriaBandwidth` before building Hysteria v1 output. Return its wrapped `render_failed` error; the outer renderer converts this to `render_node_skipped`. Emit exactly one field per direction.

- [ ] **Step 8: Run sing-box tests and verify GREEN**

Run Step 6 and the full build-tagged package. Expected: PASS.

- [ ] **Step 9: Add failing URI and Shadowrocket exact-conversion tests**

Pass direct legacy-compatible explicit rates `Up: "125 KBps"` and `Down: "250 KBps"`; assert both targets emit `upmbps=1` and `downmbps=2` without bandwidth loss warnings. These assertions fail before the render-time exact-conversion helper is added. After GREEN, add cross-format cases: Mihomo bare rates emit exact Mbps fields, while sing-box numeric Bps rates omit the unrepresentable direction with `render_lossy_field`; URI Mbps input remains exact. For `Up: "55 Bps"` and `DownMbps: 2`, assert unrelated fields and `downmbps` still render while one warning names `hysteria.up`.

- [ ] **Step 10: Run output tests and verify RED**

```bash
go test -mod=readonly ./internal/adapter/shared -run TestExactHysteriaMbps -count=1
go test -mod=readonly ./internal/adapter/uri ./internal/adapter/shadowrocket -run 'TestRender.*Hysteria.*Bandwidth' -count=1
```

Expected: the helper does not compile, and both targets currently omit the explicit fields instead of converting them to exact Mbps.

- [ ] **Step 11: Implement exact-target behavior**

Keep integer Mbps query emission. Add `shared.ExactHysteriaMbps(text string) (int, bool)`, backed by `NormalizeHysteriaRate(text, HysteriaImplicitNone)`, with focused unit cases for `125 KBps` (1, true), `55 Bps` (false), and malformed input (false). Use it in both target emitters. Update `uri/renderer_warnings.go` and `shadowrocket/warnings.go` so an exactly converted field is not also reported as lossy; otherwise omit the value and retain one loss warning per direction. Do not change Hysteria2 behavior.

- [ ] **Step 12: Run affected adapter suites**

```bash
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/adapter/shared ./internal/adapter/mihomo ./internal/adapter/singbox ./internal/adapter/uri ./internal/adapter/shadowrocket ./internal/adapter/jsonnodes -count=1
```

Expected: PASS.

- [ ] **Step 13: Format and commit**

```bash
gofmt -w internal/adapter/shared/hysteria_bandwidth.go internal/adapter/shared/hysteria_bandwidth_test.go internal/adapter/mihomo/hysteria.go internal/adapter/mihomo/renderer_test.go internal/adapter/singbox/renderer.go internal/adapter/singbox/renderer_test.go internal/adapter/singbox/renderer_probe_singbox_test.go internal/adapter/uri/renderer.go internal/adapter/uri/renderer_warnings.go internal/adapter/uri/renderer_test.go internal/adapter/shadowrocket/protocols.go internal/adapter/shadowrocket/warnings.go internal/adapter/shadowrocket/renderer_test.go
git add internal/adapter/shared/hysteria_bandwidth.go internal/adapter/shared/hysteria_bandwidth_test.go internal/adapter/mihomo/hysteria.go internal/adapter/mihomo/renderer_test.go internal/adapter/singbox/renderer.go internal/adapter/singbox/renderer_test.go internal/adapter/singbox/renderer_probe_singbox_test.go internal/adapter/uri/renderer.go internal/adapter/uri/renderer_warnings.go internal/adapter/uri/renderer_test.go internal/adapter/shadowrocket/protocols.go internal/adapter/shadowrocket/warnings.go internal/adapter/shadowrocket/renderer_test.go
git commit -m "fix(adapter): emit canonical hysteria bandwidth"
```

---

### Task 5: Prove Probe Batch Isolation

**Files:**
- Modify: `internal/service/probe_singbox_test.go`
- Modify only if the regression exposes a gap: `internal/service/probe.go:136`
- Modify: `internal/processor/node/node_test.go`

**Interfaces:**
- Consumes: per-node sing-box rendering from Task 4.
- Produces: one result per original node and regression coverage for `fail_mode: drop`.

- [ ] **Step 1: Add a mixed-batch integration regression test**

Reuse the local target and CONNECT proxy helpers. Probe:

```go
[]domain.NodeIR{
	{
		Name: "invalid-hysteria", Type: domain.NodeTypeHysteria,
		Server: "invalid.example", Port: 443,
		TLS: &domain.TLSOptions{Enabled: true},
		Hysteria: &domain.HysteriaOptions{Up: "fast", DownMbps: 100},
	},
	{
		Name: "valid-http", Type: domain.NodeTypeHTTP,
		Server: proxyHost, Port: proxyPort,
	},
}
```

Assert no batch error, two ordered results, `probe_invalid_target` for the skipped Hysteria node, and alive for HTTP. Assert the report contains the input-preservation `parse_unknown_field` warning and renderer `render_node_skipped` warning.

- [ ] **Step 2: Run the test after Task 4**

```bash
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/service -run TestServiceSingBoxProbeIsolatesInvalidHysteriaBandwidth -count=1
```

Expected after Tasks 3-4: PASS. `fast` is preserved in Raw and removed from the typed field, Task 4's missing-direction preflight skips the node, and the backend still receives the original two-node slice. This is integration coverage for behavior already driven RED/GREEN at the adapter boundary.

- [ ] **Step 3: Make a service adjustment only if the integration test exposes a gap**

The existing service should keep the original `nodes` slice and pass only the reduced payload body. If the test shows otherwise, first capture the failing output, then change `renderProbePayloads` so it never filters the original nodes before `s.prober.Probe`; missing outbound lookup must become the node-level result. Preserve `renderWarnings`. Do not modify production code when the regression already passes.

- [ ] **Step 4: Run the test and verify GREEN**

Run Step 2. Expected: PASS.

- [ ] **Step 5: Add processor regression coverage**

Use a stub result with one `probe_invalid_target` and one alive node. Assert `fail_mode: drop` returns only the alive node and retains the runner warnings.

- [ ] **Step 6: Run processor/service/probe suites**

```bash
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/processor/node ./internal/service ./internal/probe -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/service/probe.go internal/service/probe_singbox_test.go internal/processor/node/node_test.go
git commit -m "test(probe): cover hysteria bandwidth isolation"
```

If `internal/service/probe.go` did not change, omit it from `git add`.

---

### Task 6: Documentation and Full Verification

**Files:**
- Modify: `docs/reference/capabilities.md`
- Modify: `docs/reference/errors.md`
- Modify: `docs/reference/http-api/probing.md`
- Modify: `docs/reference/processors.md`
- Review: every file changed by Tasks 1-5

**Interfaces:**
- Consumes: finalized canonical and target behavior.
- Produces: one canonical explanation, warning documentation, and repository verification evidence.

- [ ] **Step 1: Check for existing doc-contract tests**

```bash
rg -n 'capabilities.md|errors.md|probing.md|processors.md' internal docs --glob '*_test.go'
```

If an existing test owns these pages, extend it with exact assertions for `parse_implicit_bandwidth_unit`, Mihomo implicit Mbps, sing-box numeric Bps, and node-level isolation. If none exists, do not create a string-presence-only test.

- [ ] **Step 2: Update canonical docs**

Document the mutually exclusive canonical fields and unit table once in `capabilities.md`. Add `parse_implicit_bandwidth_unit` to `errors.md` as a compatibility assumption warning, not a dropped node. In probing/processor docs, state that target-incompatible nodes become node-level results so `fail_mode` can act after the runner returns. Link to the canonical section instead of duplicating it.

- [ ] **Step 3: Run formatting and focused verification**

```bash
gofmt -w internal/adapter/shared/hysteria_bandwidth.go internal/adapter/shared/hysteria_bandwidth_test.go internal/adapter/mihomo/parser.go internal/adapter/mihomo/parser_test.go internal/adapter/mihomo/hysteria.go internal/adapter/mihomo/renderer_test.go internal/adapter/singbox/parser.go internal/adapter/singbox/parser_test.go internal/adapter/singbox/renderer.go internal/adapter/singbox/renderer_test.go internal/adapter/singbox/renderer_probe_singbox_test.go internal/adapter/uri/parser_protocols.go internal/adapter/uri/parser_test.go internal/adapter/uri/renderer.go internal/adapter/uri/renderer_warnings.go internal/adapter/uri/renderer_test.go internal/adapter/shadowrocket/protocols.go internal/adapter/shadowrocket/warnings.go internal/adapter/shadowrocket/renderer_test.go internal/adapter/jsonnodes/jsonnodes.go internal/adapter/jsonnodes/jsonnodes_test.go internal/service/file_inputs.go internal/service/probe_test.go internal/service/probe_singbox_test.go internal/processor/node/node_test.go
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/adapter/shared ./internal/adapter/mihomo ./internal/adapter/singbox ./internal/adapter/uri ./internal/adapter/shadowrocket ./internal/adapter/jsonnodes ./internal/nodevalidation ./internal/service ./internal/probe ./internal/processor/node -count=1
```

Expected: PASS.

- [ ] **Step 4: Build a fresh CLI and run the private regression**

```bash
go build -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls -o /tmp/sandrone-hysteria-bandwidth ./cmd/sandrone
private_subscription_file="${SANDRONE_PRIVATE_SUBSCRIPTION_FILE:?set this to the ignored local subscription JSON path}"
private_subscription_url="$(jq -er '.remote.url' "$private_subscription_file")"
/tmp/sandrone-hysteria-bandwidth probe \
  --input-url "$private_subscription_url" \
  --method url-test \
  --core sing-box \
  --url https://cp.cloudflare.com \
  --timeout 5s \
  --concurrency 10 \
  --output /tmp/sandrone-hysteria-probe.json \
  2>/tmp/sandrone-hysteria-probe.stderr
jq -e '.results | length > 0' /tmp/sandrone-hysteria-probe.json >/dev/null
! rg -q 'decode sing-box probe config' /tmp/sandrone-hysteria-probe.stderr
```

Set `SANDRONE_PRIVATE_SUBSCRIPTION_FILE` in the shell without committing it. The commands read the URL without printing it and keep all output under `/tmp`. Do not print node bodies, URLs, credentials, or servers.

- [ ] **Step 5: Run the repository gate**

```bash
make check
```

Expected: PASS.

- [ ] **Step 6: Inspect scope and private data**

```bash
git diff --check
git status --short
git diff 5ce4e9c --name-only
git diff 5ce4e9c -- . ':(exclude)go.sum' | rg -n 'https?://|subscription_url|auth-str|auth_str|password' || true
```

Expected: intended files only; every match is a synthetic fixture, documentation link, or field name, with no private runtime value.

- [ ] **Step 7: Commit documentation**

```bash
git add docs/reference/capabilities.md docs/reference/errors.md docs/reference/http-api/probing.md docs/reference/processors.md
git commit -m "docs: define hysteria bandwidth compatibility"
```

- [ ] **Step 8: Apply planning-artifact cleanup policy after acceptance**

After implementation is verified and accepted, remove both this plan and its paired design in the same cleanup commit only when applying the repository rule for completed plans/specs. Never remove only one and leave the other as a stale canonical source.
