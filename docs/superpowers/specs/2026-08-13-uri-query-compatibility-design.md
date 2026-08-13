# URI WebSocket Early-Data and TLS Alias Compatibility Design

## Goal

Normalize established URI query aliases that already have exact `NodeIR`
representations, so compatible inputs do not produce `parse_unknown_field`
warnings.

## Scope

- For VMess AEAD, VLESS, and Trojan URI inputs using WebSocket transport,
  accept a positive decimal `ed` value as `transport.max_early_data`.
- When `ed` is accepted, use non-empty `eh` as
  `transport.early_data_header_name`; when `eh` is absent, default to
  `Sec-WebSocket-Protocol` to match the existing path-based early-data
  normalization.
- If path-based early-data has already populated the typed fields, accept a
  query value only when it is consistent; conflicting or invalid query fields
  remain in `NodeIR.Raw` and retain their warning.
- Accept URI query key `allowinsecure` as an alias of `allowInsecure` anywhere
  the shared TLS query parser is used.
- Do not change `spx` handling.

## Boundaries

URI syntax is decoded only in `internal/adapter/uri`. The adapter writes typed
transport and TLS fields into `NodeIR`; service and renderers require no new
special cases. Recognized aliases are removed from warning-producing raw
fields only when their values are valid and semantically applied.

## Verification

Parser tests cover accepted query aliases, default header behavior, conflict
and invalid-value preservation, and the lowercase TLS alias. Cross-format
service tests verify the typed early-data fields reach Mihomo and sing-box
renderers. A real subscription validation confirms the warning reduction,
followed by the repository `make check` and lint gates.
