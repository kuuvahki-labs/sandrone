# Sandrone Agent Skill HTTP Client Design

## Context

Sandrone currently exposes complete Agent-oriented discovery, JSON Schema,
resource management, rendering, probing, and processor guidance through MCP.
The published `sandrone` Skill therefore requires a connected MCP server and
uses MCP as its only execution plane.

That requirement is heavier than necessary for a trusted local Agent that can
run bundled Skill scripts. Sandrone already has a bearer-protected HTTP API for
most execution and resource management operations, but its processor,
FileSpec-kind, and script API schemas are currently exposed only as MCP
resources. A generic HTTP wrapper alone would therefore let an Agent send
requests without giving it the live contracts needed to construct reliable
requests.

## Goals

- Let an installed Sandrone Skill operate through the HTTP API when the Agent
  has shell access, without requiring an MCP connection.
- Keep MCP as an optional, fully supported execution plane for clients that
  support it.
- Keep runtime capabilities and schemas server-owned rather than copying them
  into Skill documentation.
- Use `SANDRONE_URL` and optional `SANDRONE_TOKEN` as the HTTP connection
  contract.
- Preserve the existing authorization, resource safety, processor ordering,
  strict decoding, and structured error semantics.

## Non-goals

- Reimplement Sandrone business logic inside a shell script.
- Replace or remove MCP.
- Add a general-purpose arbitrary HTTP client to the Sandrone Go CLI.
- Embed a Sandrone server or MCP server inside the portable Skill package.
- Guarantee script execution on Agent clients that do not grant shell access.
- Add OpenAPI generation or duplicate all HTTP reference documentation in the
  Skill.

## Chosen Architecture

The Skill supports two execution planes:

1. **HTTP script, preferred when configured.** When the bundled script is
   executable and `SANDRONE_URL` is set, the Agent calls the Sandrone HTTP API
   through that script.
2. **MCP, optional fallback.** When HTTP script execution is unavailable and a
   Sandrone MCP server is connected, the Agent follows the existing MCP
   workflow.

If both are available, the HTTP script is preferred. This makes the lightweight
mode deterministic instead of loading MCP merely because it happens to be
configured. If neither is available, the Skill reports the missing execution
dependency and provides configuration instructions without installing or
starting services implicitly.

The HTTP and MCP entrypoints continue to delegate to the same service and
domain layers. Schema document construction moves to, or is exposed through, a
shared internal package so the two protocols serialize the same owner-maintained
catalogs.

## Complete HTTP Execution Surface

The HTTP execution plane must cover every operation the Skill advertises. In
addition to existing resource CRUD, preview, traffic, file validation, and file
rendering routes, add these bearer-protected endpoints:

| Method and path | Result |
| --- | --- |
| `POST /v1/convert` | Full parse-then-render conversion, including parse/render processors, options, remote input, metadata, report, and warnings |
| `POST /v1/probe` | Node probing through the existing controlled probe service |
| `POST /v1/subscriptions/{name}/render` | Render a stored subscription to an explicit target format with optional request arguments |

These handlers decode HTTP wire objects and call the same service methods as
the corresponding MCP tools:

- `sandrone_convert`;
- `sandrone_probe_nodes`;
- `sandrone_render_subscription`.

They do not introduce new business operations. Request JSON uses strings for
text bodies and JSON objects for processor parameters; it must not expose Go
`[]byte` base64 or `json.RawMessage` implementation details. Conversion and
subscription rendering return JSON containing `content_type`, `body`, and the
complete structured `report`. Probe returns `results` and its complete
structured `report`.

`POST /v1/convert` is distinct from the existing public, processor-free
`GET /convert`. The authenticated endpoint supports the complete controlled
conversion contract; the public endpoint keeps its current restrictions.
`POST /v1/probe` may access the network or a configured client core through the
existing probe boundary. Subscription rendering materializes only an existing
stored subscription and never persists the rendered body.

## Read-only Schema HTTP API

Add these bearer-protected routes:

| Method and path | Result |
| --- | --- |
| `GET /v1/schemas/processors` | Public processor summary catalog |
| `GET /v1/schemas/processors/{stage}/{type}` | Exact processor parameter schema, effects, examples, and error codes |
| `GET /v1/schemas/file-kinds/{kind}` | Canonical FileSpec-kind settings schema, source rules, defaults, and examples |
| `GET /v1/schemas/script-api/v1` | Versioned processor script API contract |
| `GET /v1/schemas/subscription` | Complete Subscription write schema, including inputs and processor objects |
| `GET /v1/schemas/file-spec` | Complete named FileSpec write schema, including source, typed config, and processors |

These endpoints expose the same JSON document shapes currently returned by:

- `sandrone://schemas/processors`
- `sandrone://schemas/processors/{stage}/{type}`
- `sandrone://schemas/file-kinds/{kind}`
- `sandrone://schemas/script-api/v1`

The Subscription and FileSpec documents reuse the same closed JSON Schema
builders as MCP management tool inputs. They are necessary because an HTTP-only
Agent does not receive MCP `tools/list`; the Skill must not carry a stale copy
of those top-level resource contracts.

The endpoints are read-only and use the existing `/v1/*` bearer boundary.
Unknown or non-canonical stages, processor types, kinds, versions, extra path
segments, encoded separators, and invalid resource names return the existing
structured Sandrone errors. HTTP and MCP tests assert semantic JSON equality
for matching documents so the two transports cannot drift.

The existing `GET /v1/inspect` remains the capability summary. It is not
expanded to contain every schema because that would produce an unnecessarily
large response and defeat progressive discovery.

## Bundled HTTP Script

Add `skills/sandrone/scripts/sandrone-api.sh` as a thin transport adapter. Its
interface is:

```text
sandrone-api.sh METHOD PATH [BODY_FILE|-]
```

Examples:

```sh
skills/sandrone/scripts/sandrone-api.sh GET /v1/inspect
skills/sandrone/scripts/sandrone-api.sh GET /v1/schemas/file-kinds/mihomo
skills/sandrone/scripts/sandrone-api.sh POST /v1/files -
```

The script:

- requires an absolute `http://` or `https://` `SANDRONE_URL`;
- removes only trailing slashes from the configured base URL;
- accepts only a relative absolute-path beginning with `/`;
- uses `SANDRONE_TOKEN` as `Authorization: Bearer ...` when non-empty;
- sends `Content-Type: application/json` only when a request body is supplied;
- reads a body from a named file or stdin, never from a command-line JSON
  argument that could leak through process listings;
- uses `curl` without printing the token or enabling tracing;
- preserves successful response bodies on stdout;
- writes transport and HTTP failure diagnostics to stderr and returns non-zero;
- preserves a structured Sandrone error response for the Agent to inspect;
- does not retry mutation requests.

The script does not construct URLs for individual resources, validate
FileSpecs, interpret schemas, or implement endpoint-specific operations. Those
decisions remain in the Skill workflow and the Sandrone server.

## Skill Workflow

Update the Skill metadata and instructions as follows:

- The description triggers for Sandrone operations without requiring the phrase
  “connected MCP server.”
- `compatibility` states that execution requires either shell access with
  `curl`, `SANDRONE_URL`, and the bundled script, or a connected Sandrone MCP
  server.
- `agents/openai.yaml` no longer declares MCP as a mandatory dependency.
- UI strings refer to Sandrone rather than “Sandrone MCP.”
- The task-start workflow selects HTTP first, then MCP.
- HTTP discovery begins with `/healthz`, `/version`, `/v1/inspect`, and the
  narrowest relevant `/v1/schemas/*` endpoint.
- HTTP execution uses `/v1/convert`, `/v1/probe`, or
  `/v1/subscriptions/{name}/render` for the three operations that previously
  existed only as MCP tools.
- MCP discovery keeps using the existing capability and schema resources.
- Task recipes map each operation to both HTTP and MCP without copying schema
  contents.
- Mutation authorization, pre-read rules, validation-before-persistence,
  ambiguity handling, sensitive-output rules, and `adaptive_groups` boundaries
  apply identically to both execution planes.

The Skill never falls back from an ambiguous failed mutation on one execution
plane to retrying it on the other. It rereads the exact resource first.

## Documentation Ownership

- `docs/reference/http-api/README.md` owns the common HTTP contract and links to
  a new schema catalog subsection or page.
- The schema HTTP reference owns route, authentication, response, and error
  behavior, while processor and FileSpec semantic details remain in their
  existing canonical references.
- `docs/reference/mcp.md` continues to own MCP transport and protocol behavior.
- `skills/sandrone/SKILL.md` owns Agent execution-plane selection and links to
  its concise workflow reference.
- No generated schema snapshots are committed to the Skill.

## Error and Security Behavior

- All schema endpoints remain behind the existing `/v1/*` authentication
  middleware.
- Full conversion, probing, and subscription rendering also remain behind the
  same bearer boundary; the existing public `GET /convert` policy does not
  apply to `POST /v1/convert`.
- `SANDRONE_TOKEN` is never accepted as a command argument and is never echoed.
- The script does not use `eval`, concatenate shell commands, or accept a
  caller-supplied host independently of `SANDRONE_URL`.
- HTTP redirects are not followed automatically, preventing credentials from
  being forwarded to an unexpected host.
- Resource bodies, rendered configurations, preview results, and processor
  envelopes remain sensitive even when the bearer token itself is hidden.
- Shell availability authorizes transport execution only; it does not expand
  the user’s authorization to overwrite or delete resources.

## Testing

Implementation follows test-driven development:

1. Add HTTP route tests that fail because the schema route families do not
   exist.
2. Add HTTP execution tests that fail because full conversion, probe, and
   subscription-render routes do not exist, then compare their service-level
   semantics with the corresponding MCP tools.
3. Add parity tests comparing each HTTP document with the corresponding MCP
   catalog output.
4. Add route validation and authentication tests for canonical and malformed
   paths.
5. Add script tests using a local test HTTP server to cover:
   - URL and method validation;
   - optional bearer authentication;
   - file and stdin request bodies;
   - successful output;
   - JSON and non-JSON HTTP failures;
   - connection failures;
   - no mutation retry;
   - no token disclosure.
6. Add Skill contract checks that initially fail under the current MCP-only
   metadata and workflow, then pass after the dual-plane update.
7. Validate the final Skill with the official Agent Skills validator and
   `npx skills add . --list`.
8. Run relevant Go and script tests first, then `make check`.

## Rollout and Compatibility

Existing MCP clients and HTTP routes remain unchanged. The new execution and
schema routes are additive. Existing Skill users with a configured MCP
dependency can
continue using MCP after updating, while new users may configure only:

```sh
export SANDRONE_URL="http://127.0.0.1:1138"
export SANDRONE_TOKEN="replace-with-your-token"
```

The token may be omitted only when the target Sandrone server is intentionally
configured without authentication. Installing the Skill does not persist these
values; users configure them in the environment made available to their Agent.
