# Sandrone Workflows

Use the selected execution plane for the whole operation. HTTP operations below
use `scripts/sandrone-api.sh METHOD PATH [BODY_FILE|-]`; pass JSON on stdin with
`-` or from a permission-restricted body file. MCP names are unqualified; a
client may add a namespace prefix.

## Transport Mapping

| Operation | HTTP | MCP |
| --- | --- | --- |
| Inspect | `GET /v1/inspect` | `sandrone_inspect` |
| List format capabilities | `GET /v1/capabilities/formats` | `sandrone://capabilities/formats` |
| Exact format capability | `GET /v1/capabilities/formats/{direction}/{format}` | `sandrone://capabilities/formats/{direction}/{format}` |
| Full conversion | `POST /v1/convert` | `sandrone_convert` |
| Probe nodes | `POST /v1/probe` | `sandrone_probe_nodes` |
| List subscriptions | `GET /v1/subscriptions` | `sandrone_list_resources` |
| Read subscription | `GET /v1/subscriptions/{name}` | `sandrone://subscriptions/{name}` |
| Put subscription | `POST /v1/subscriptions` | `sandrone_put_subscription` |
| Delete subscription | `DELETE /v1/subscriptions/{name}` | `sandrone_delete_subscription` |
| Preview subscription | `POST /v1/subscriptions/{name}/preview` | `sandrone_preview_subscription` |
| Render subscription | `POST /v1/subscriptions/{name}/render` | `sandrone_render_subscription` |
| List files | `GET /v1/files` | `sandrone_list_resources` |
| Read FileSpec | `GET /v1/files/{name}?mode=spec` | `sandrone://files/{name}` |
| Validate FileSpec | `POST /v1/validate` | `sandrone_validate_file` |
| Put FileSpec | `POST /v1/files` | `sandrone_put_file` |
| Delete FileSpec | `DELETE /v1/files/{name}` | `sandrone_delete_file` |
| Render file | `GET /v1/files/{name}?mode=render&response=json` | `sandrone_get_file` |

## Schema Mapping

Read schemas at task time; do not copy their fields into this reference.

| Schema | HTTP | MCP resource |
| --- | --- | --- |
| Schema catalog | `GET /v1/schemas` | `sandrone://schemas` |
| Processor catalog | `GET /v1/schemas/processors` | `sandrone://schemas/processors` |
| Exact processor | `GET /v1/schemas/processors/{stage}/{type}` | `sandrone://schemas/processors/{stage}/{type}` |
| File-kind catalog | `GET /v1/schemas/file-kinds` | `sandrone://schemas/file-kinds` |
| File kind | `GET /v1/schemas/file-kinds/{kind}` | `sandrone://schemas/file-kinds/{kind}` |
| Script API | `GET /v1/schemas/script-api/v1` | `sandrone://schemas/script-api/v1` |
| Subscription | `GET /v1/schemas/subscription` | `sandrone://schemas/subscription` |
| FileSpec | `GET /v1/schemas/file-spec` | `sandrone://schemas/file-spec` |

## Discover

For HTTP, call `/healthz`, `/version`, `GET /v1/inspect`, then only the format
capability, schema, and resource endpoints needed by the request. For MCP, call
`sandrone_inspect`, read the narrowest capability/schema resources, list with
the narrowest useful `kind`, then read the relevant definitions.

Follow an opaque `next_cursor` without interpreting or modifying it. Keep the
same list filter across pages.

## Convert Inline or Remote Input

1. Read the exact source parse and target render capabilities plus every
   requested parse/render processor schema.
2. Use authenticated `POST /v1/convert` for HTTP or `sandrone_convert` for MCP
   with exactly one supported input source.
3. Inspect report warnings and loss information.
4. If MCP prompts are available, use `diagnose_conversion_loss` only to explain
   loss; prompts do not execute conversion.

The public `GET /convert` route is a separate processor-free convenience route.
It does not replace full `POST /v1/convert`. Conversion creates neither a
Subscription nor a FileSpec.

## Create or Update a Subscription

1. Read `/v1/schemas/subscription` plus the exact target format capability and
   each requested nodes-stage processor schema, or the equivalent live MCP
   resources.
2. If updating, read the exact existing subscription first.
3. Build a complete Subscription definition from the live schemas.
4. Return a draft without mutation when only a draft was requested.
5. For an authorized persistence request, use `POST /v1/subscriptions` or
   `sandrone_put_subscription`.
6. Preview the saved name with
   `POST /v1/subscriptions/{name}/preview` or
   `sandrone_preview_subscription`.
7. When requested, render with authenticated
   `POST /v1/subscriptions/{name}/render` or
   `sandrone_render_subscription`.

Preview/render accepts a saved subscription name. Do not claim that a new
unsaved subscription was previewed.

## Create or Update a FileSpec

1. Read `/v1/schemas/file-spec`,
   `/v1/schemas/file-kinds/{kind}`, and every requested file-stage processor
   schema, or the equivalent live MCP resources.
2. If updating, read the exact existing FileSpec first.
3. Build a complete FileSpec using only live schema fields. Preserve declared
   file-stage processor order.
4. Write concrete `groups`, `rule_sets`, and `rules` where applicable. Never
   create or modify `adaptive_groups`; preserve an existing value unchanged
   only when the accepted write shape permits a compatibility round-trip.
5. Validate inline with `POST /v1/validate` or `sandrone_validate_file`.
6. Resolve validation errors before persistence.
7. Return a draft without mutation when only a draft was requested.
8. For an authorized persistence request, use `POST /v1/files` or
   `sandrone_put_file`.
9. Render a saved file with
   `GET /v1/files/{name}?mode=render&response=json` or `sandrone_get_file` in
   `render` mode. Use `spec` or `source` mode for those representations.

## Delete a Resource

1. Require an explicit delete request and an exact resource kind/name.
2. Read the exact existing definition through the selected plane.
3. Call the matching HTTP `DELETE` endpoint or MCP delete tool once.
4. Report `deleted` exactly as returned. After an ambiguous transport failure,
   keep the same plane and reread the exact resource before retrying.

## Probe Nodes

1. Read probe capabilities.
2. Resolve input, method, core, target, timeout, attempts, concurrency,
   and cache behavior from the request and current schema.
3. Use authenticated `POST /v1/probe` or `sandrone_probe_nodes`.
4. Separate per-node results from aggregate warnings; probing is not a
   configuration mutation.

## Author a Processor or Script

1. Read the exact `/v1/schemas/processors/{stage}/{type}` endpoint or matching
   MCP resource.
2. For scripts, also read `/v1/schemas/script-api/v1` or
   `sandrone://schemas/script-api/v1`.
3. Use `write_processor_script` only as an optional MCP drafting aid.
4. Treat the script as embedded ECMAScript, not Node.js.
5. Validate its containing Subscription or FileSpec through the applicable
   preview or validation flow.

## Explain Reports and Errors

Branch on structured `code` and context fields. An MCP `explain_report` prompt
may help explain results but does not execute an action. When
`body_omitted: true`, report `body_bytes` and `max_output_bytes`; Sandrone did
not create a hidden file or share.
