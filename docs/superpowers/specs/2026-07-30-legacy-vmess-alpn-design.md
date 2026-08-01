# Legacy VMess ALPN Parsing Design

## Goal

Parse the top-level `alpn` field in legacy Base64-encoded VMess JSON sharing
links into canonical `NodeIR.TLS.ALPN`. This removes the paired
`parse_unknown_field` and `render_lossy_field` warnings without changing probe
behavior or weakening invalid-source diagnostics.

## Scope

The URI adapter accepts these legacy `alpn` representations:

- a comma-separated string such as `"h2,http/1.1"`;
- an array of strings such as `["h2", "http/1.1"]`.

Values are trimmed, empty entries are discarded, and order is preserved. The
field is consumed whenever the complete source value has a supported shape.
An empty string, empty array, or value containing only empty entries produces
an empty ALPN list without a warning. Unsupported scalars, objects, or arrays
containing non-string entries remain in `NodeIR.Raw` and continue to produce
the existing structured unknown-field warning.

This change does not alter VMess TLS enablement, probe execution, health
classification, or the semantics of other URI fields.

## Data Flow

`parseLegacyVMess` decodes the JSON document, normalizes a supported `alpn`
value, and creates `TLSOptions` when ALPN is present even if no other TLS field
exists. The normalized protocols are assigned to `TLSOptions.ALPN`.

The parser then marks `alpn` as known only when normalization succeeded.
`shared.AddUnknownRaw` therefore preserves and warns about invalid values while
omitting valid values from `NodeIR.Raw`.

Existing Mihomo and sing-box renderers already emit canonical
`TLSOptions.ALPN`, so no renderer change is required.

## Testing

Focused URI parser tests cover:

- comma-separated string input;
- string-array input;
- trimming and empty-entry removal;
- empty supported input, which is consumed without a warning;
- an array containing a non-string value, which remains raw and warns.

The existing URI adapter test package must remain green. A CLI regression check
using `data/subscriptions/local2.json` must show that both the Mihomo and
sing-box probe paths no longer emit `vmess.alpn` parse or render warnings.
Probe success and failure counts are observed but are not acceptance criteria
for this parser-only change.

## Acceptance Criteria

- Valid legacy VMess `alpn` values populate `NodeIR.TLS.ALPN` in source order.
- Valid empty legacy VMess `alpn` values are consumed without creating
  `TLSOptions` solely for an empty list.
- Valid values do not remain under `raw["vmess.alpn"]` and do not produce an
  unknown-field warning.
- Invalid values remain raw and retain the existing warning.
- Mihomo and sing-box rendering preserve the normalized ALPN value.
- No probe behavior or unrelated adapter behavior changes.
