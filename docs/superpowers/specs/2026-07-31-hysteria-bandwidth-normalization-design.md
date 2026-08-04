# Hysteria Bandwidth Normalization Design

## Context

Sandrone currently stores Hysteria v1 bandwidth in two pairs of `NodeIR`
fields:

- `hysteria.up` and `hysteria.down` as strings;
- `hysteria.up_mbps` and `hysteria.down_mbps` as integers.

The source ecosystems do not assign the same meaning to a bare number.
Mihomo accepts an `up` or `down` string without a unit and interprets it as
Mbps. sing-box accepts a JSON number in `up` or `down` as bytes per second,
requires an explicit unit in the string form, and separately accepts
`up_mbps` and `down_mbps`. Hysteria v1 sharing URLs and Stash-style input
normally use explicitly named Mbps fields such as `upmbps`, `downmbps`,
`up-speed`, and `down-speed`.

The current adapters erase those distinctions. For example:

```text
Mihomo up: "55"       -> NodeIR up: "55" -> sing-box up: "55" (invalid)
sing-box up: 55       -> NodeIR up: "55" -> Mihomo up: "55" (55 Mbps)
```

The first path can prevent the entire sing-box probe payload from starting.
The second path is more dangerous because it succeeds while changing the
effective rate from 55 bytes per second to 55 megabits per second.

Relevant upstream contracts:

- Mihomo Hysteria documents that omitted `up` and `down` units default to
  Mbps: <https://wiki.metacubex.one/en/config/proxies/hysteria/>.
- sing-box Hysteria requires an explicit unit for string `up` and `down`,
  accepts numeric byte-rate values, and provides integer Mbps compatibility
  fields: <https://sing-box.sagernet.org/configuration/outbound/hysteria/>.
- Hysteria v1 and Stash examples use explicit Mbps fields:
  <https://v1.hysteria.network/docs/custom-ca/> and
  <https://stashws.org/proxy-protocols/proxy-types>.

## Goals

- Preserve the effective bandwidth semantics of supported Mihomo, sing-box,
  URI, Base64-wrapped URI, JSON Nodes, and inline-node inputs.
- Keep the existing public `NodeIR` shape and avoid a data migration.
- Resolve source-specific field precedence before data becomes canonical.
- Ensure canonical typed Hysteria bandwidth never contains an ambiguous bare
  numeric string.
- Remain permissive toward common and legacy inputs instead of rejecting a
  whole subscription.
- Prevent one target-incompatible Hysteria node from invalidating an entire
  sing-box probe payload.
- Report assumptions, unsupported values, and target loss through structured
  warnings.
- Verify every affected parser and renderer through the canonical `NodeIR`
  boundary.

## Non-goals

- Add new public bandwidth fields or replace `HysteriaOptions` with a new
  public rate type.
- Preserve the exact spelling, whitespace, or choice between equivalent Mbps
  fields when the effective rate is unchanged.
- Change Hysteria congestion-control behavior or invent a product-wide rate
  default when the source format already defines one.
- Add fractional Mbps fields to `NodeIR`.
- Change Hysteria2's public model beyond regression coverage needed to prove
  that the Hysteria v1 normalization does not affect it.
- Make all possible invalid core configuration fields node-isolated in this
  change; the scope is Hysteria bandwidth.

## Chosen Architecture

### Existing NodeIR Fields Remain the Contract

Keep the existing fields in `domain.HysteriaOptions`:

```go
Up       string
Down     string
UpMbps   int
DownMbps int
```

After a supported source adapter normalizes one direction, exactly one of its
two fields is populated:

- an exact, positive, whole number of megabits per second uses `UpMbps` or
  `DownMbps`;
- every other supported positive rate uses `Up` or `Down` with an explicit,
  case-sensitive unit.

Examples:

| Source value | Canonical value |
| --- | --- |
| Mihomo `up: "55"` | `UpMbps: 55` |
| Mihomo `up: "55 Mbps"` | `UpMbps: 55` |
| Mihomo `up: "640 KBps"` | `Up: "640 KBps"` |
| sing-box `up: 55` | `Up: "55 Bps"` |
| sing-box `up: "55 Mbps"` | `UpMbps: 55` |
| URI `upmbps=55` | `UpMbps: 55` |

Canonical explicit-unit strings use one space between the positive integer
and unit. Supported units are the case-sensitive intersection required for
the affected targets: `bps`, `Bps`, `Kbps`, `KBps`, `Mbps`, `MBps`, `Gbps`,
`GBps`, `Tbps`, and `TBps`. Normalization uses checked integer arithmetic and
does not silently overflow.

When an explicit rate is exactly divisible into a positive whole Mbps value
that fits `int`, normalization uses the Mbps integer field. Otherwise it keeps
the explicit-unit string. Empty and zero compatibility fields remain absent.

### Source Adapters Resolve Meaning and Precedence

Each parser must resolve the source format's effective compatibility value
before writing the canonical pair. This prevents a target with different
field behavior from selecting the wrong value.

#### Mihomo

- A bare numeric `up` or `down` string has an implicit `Mbps` unit.
- `up-speed` and `down-speed` are Stash-compatible integer Mbps input fields.
- A positive `up-speed` or `down-speed` overrides the corresponding `up` or
  `down` during import so providers using those compatibility fields retain
  their intended effective rate.
- The parser writes only the resulting effective canonical field.

#### sing-box

- A JSON number in `up` or `down` has an implicit `Bps` unit.
- A string in `up` or `down` must carry a supported explicit unit.
- `up_mbps` and `down_mbps` are integer Mbps compatibility fields.
- A positive `up` or `down` overrides the corresponding Mbps compatibility
  field, matching the locked sing-box runtime.
- The parser writes only the resulting effective canonical field.

#### URI and Base64

Base64 input continues to decode into the URI parser and therefore has the
same behavior as URI and URI-list input.

- `upmbps` and `downmbps` are integer Mbps fields and take precedence when
  present and positive.
- An explicit-unit `up` or `down` is normalized normally.
- A bare numeric `up` or `down` is accepted as implicit Mbps for compatibility
  with the dominant Mihomo-style convention and emits
  `parse_implicit_bandwidth_unit`.

#### JSON Nodes and Inline Nodes

JSON Nodes remains a permissive interchange format, and inline nodes must
receive the same compatibility normalization even though they do not pass
through a text parser.

- Existing explicit-unit strings and positive Mbps integers normalize
  normally.
- A bare numeric string consults `source_format` when it is present:
  `mihomo` and URI-family provenance imply Mbps; `sing-box` provenance implies
  Bps.
- Missing, `json-nodes`, or unknown provenance defaults to Mbps for practical
  compatibility and emits `parse_implicit_bandwidth_unit`.
- This compatibility rule is a migration behavior for ambiguous legacy
  `NodeIR`; new normalized output never emits a bare numeric string.

### Unsupported Rate Values

Parsing remains tolerant. A non-empty value that cannot be normalized as a
positive supported rate does not abort the subscription:

- the typed canonical direction remains unset;
- the original source value is preserved in `NodeIR.Raw` under the existing
  source-specific namespace;
- the parser emits the existing structured unknown-field warning with node
  context.

For JSON Nodes and inline nodes, where there is no source wire object to
preserve separately, the compatibility normalizer clears the invalid typed
field and adds equivalent raw provenance before validation. This keeps the
invalid value diagnosable without allowing it to poison a target payload.

No deterministic, source-defined normalization warning is needed. Only an
ambiguous bare number whose unit Sandrone must assume emits
`parse_implicit_bandwidth_unit`. Add that warning code to the canonical error
reference.

## Rendering Contract

Renderers consume only the normalized effective pair and never infer the
original source format.

### sing-box

- `UpMbps` and `DownMbps` render as `up_mbps` and `down_mbps`.
- explicit-unit `Up` and `Down` render as `up` and `down`.
- a Hysteria v1 node missing either effective direction is not added to the
  sing-box outbound document; it produces `render_node_skipped`.
- the renderer validates supported explicit-rate syntax before emitting an
  outbound.

This guarantees that every emitted Hysteria rate is accepted by the locked
sing-box option decoder. A mixed batch can therefore start even when one
source node has unusable bandwidth.

### Mihomo

- integer Mbps values render through Mihomo's native `up` and `down` fields as
  explicit `"N Mbps"` strings; the renderer does not emit Stash-only
  `up-speed` or `down-speed` as the sole rate.
- explicit-unit rates render as `up` and `down`.
- the renderer validates the same supported rate syntax before output.
- a Hysteria v1 node missing either effective direction produces
  `render_node_skipped` rather than an unusable proxy definition.

Tests must compare the effective byte rate computed by Mihomo, not only the
rendered YAML spelling.

### URI and Shadowrocket

- integer Mbps values render through `upmbps` and `downmbps`.
- an explicit rate that converts exactly to a positive whole Mbps value may
  use the same fields.
- a supported rate that cannot be represented exactly by the target is
  omitted with `render_lossy_field`, preserving the existing target capability
  boundary.
- unrelated node fields continue rendering normally.

### JSON Nodes

JSON Nodes output writes the normalized mutually exclusive pair. It never
emits a typed `up` or `down` containing only digits, regardless of the input
format.

## Probe Failure Isolation

The probe processor's `fail_mode` applies only after the runner returns
node-level results, so it cannot recover from a sing-box document that fails
to decode. Isolation must happen while rendering the probe payload.

When a mixed batch contains a target-incompatible Hysteria node, the sing-box
renderer skips that outbound and records `render_node_skipped`. The sing-box
backend still returns one result per original `NodeIR`; lookup of the skipped
node produces the existing node-level `probe_invalid_target`, while renderable
nodes continue through URL-test. The processor can then apply `keep`, `drop`,
or `error` normally.

If no node in a non-empty batch can be rendered, the existing no-renderable-
nodes batch error remains appropriate.

## Data Flow

1. The source parser determines the source-specific effective value and
   implicit unit.
2. Shared bandwidth normalization converts the value into an explicit-unit
   string or an integer Mbps value with checked arithmetic.
3. JSON Nodes and inline nodes pass through the same compatibility
   normalization before normalized-node validation.
4. `NodeIR` carries one unambiguous effective field per direction.
5. The selected renderer validates representability and emits only the target
   format's native field form.
6. Render warnings enter the existing report pipeline.
7. Probe payloads contain only core-decodable outbounds; skipped original
   nodes become node-level probe failures instead of batch-start failures.

## Testing

Implementation follows red-green-refactor. Tests use local data only and must
not store the real subscription URL, credentials, server addresses, or node
names.

### Shared Normalization Tests

Cover:

- every supported bit and byte unit;
- optional whitespace around source values and canonical one-space output;
- bare-number defaults for Mbps and Bps sources;
- exact whole-Mbps promotion;
- non-integral-Mbps explicit-unit preservation;
- empty, zero, negative, malformed, unsupported-unit, overflow, and `int`
  range cases;
- mutual exclusivity of the resulting string and Mbps fields.

### Parser Tests

Mihomo tests cover bare numbers, explicit units, `up-speed`/`down-speed`, and
the source precedence when both forms are present.

sing-box tests cover JSON numbers, explicit-unit strings,
`up_mbps`/`down_mbps`, and the opposite source precedence when both forms are
present.

URI tests cover `upmbps`/`downmbps`, explicit-unit `up`/`down`, ambiguous bare
numbers with the new warning, and outer Base64 input.

JSON Nodes and service inline-node tests cover provenance-based legacy
normalization, unknown-provenance Mbps fallback with warning, and invalid
value preservation in raw metadata.

### Renderer and Cross-format Tests

Exercise these paths through real parsers and renderers:

```text
Mihomo -> NodeIR -> sing-box / URI / Shadowrocket / JSON Nodes
sing-box -> NodeIR -> Mihomo / URI / Shadowrocket / JSON Nodes
URI -> NodeIR -> sing-box / Mihomo / JSON Nodes
JSON Nodes -> each affected renderer
```

For sing-box, decode every produced Hysteria outbound with the project's
locked `option.Options`. For Mihomo, compute the target runtime's effective
rate and compare bytes per second. URI and Shadowrocket tests assert exact
Mbps output or the expected structured loss warning.

### Service and Probe Tests

- A minimal Mihomo fixture with `up: "11"` and `down: "55"` renders a
  sing-box payload accepted by the locked core.
- A valid sing-box fixture with numeric `up` and `down` retains its byte-rate
  meaning when rendered to Mihomo.
- A mixed probe batch containing one unusable Hysteria node and one valid
  node starts successfully; the bad node receives `probe_invalid_target` and
  the valid node is probed.
- An entirely unrenderable non-empty batch retains the existing batch error.
- Result ordering, warning merging, and `fail_mode` behavior remain unchanged.

The ignored local subscription may be used for a final manual CLI regression:
fetch and parsing should still produce the full source batch, and the probe
must progress to node-level results instead of failing while decoding the
sing-box payload. This private input is never committed.

### Verification Scope

Run focused tests for:

```text
./internal/adapter/mihomo
./internal/adapter/singbox
./internal/adapter/uri
./internal/adapter/shadowrocket
./internal/adapter/jsonnodes
./internal/nodevalidation
./internal/service
./internal/probe
```

Then run the repository `make check` gate.

## Documentation

- Document the canonical Hysteria bandwidth representation and source
  defaults in the adapter capability reference.
- Add `parse_implicit_bandwidth_unit` to the warning catalog.
- Keep format-specific details in their existing canonical reference pages;
  other documentation links to those pages rather than duplicating the full
  contract.

## Acceptance Criteria

- Mihomo bare Hysteria rates reach sing-box with their Mbps meaning intact.
- sing-box numeric Hysteria rates reach Mihomo without changing from Bps to
  Mbps.
- Source field precedence is resolved before `NodeIR` and cannot be reversed
  by a target renderer.
- Normalized JSON Nodes contain no ambiguous bare numeric `up` or `down`
  strings.
- Common and legacy inputs remain parseable; assumptions and unsupported
  values are visible in structured warnings.
- One invalid Hysteria node cannot prevent other nodes in the batch from being
  probed.
- URI and Shadowrocket report unavoidable precision loss instead of silently
  changing the rate.
- No private subscription data or external-network test dependency enters the
  repository.
- Focused tests and `make check` pass.
