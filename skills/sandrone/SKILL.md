---
name: sandrone
description: Use when an Agent must operate Sandrone to convert proxy or subscription data, inspect or manage subscriptions and FileSpecs, render client configurations, probe nodes, author processors or sandbox scripts, or explain Sandrone reports.
---

# Sandrone

## Overview

Use the bundled HTTP script as the preferred execution plane and a connected
Sandrone MCP server as the fallback. Use this Skill for workflow and safety
decisions; obtain capabilities, schemas, defaults, examples, and current
resource definitions from the selected live plane.

## Select One Execution Plane

1. If shell access, `curl`, `SANDRONE_URL`, and
   `scripts/sandrone-api.sh` are available, select the HTTP script.
2. Otherwise, if Sandrone MCP tools are connected, select MCP.
3. Otherwise, report both supported setup choices: configure `SANDRONE_URL`
   (and `SANDRONE_TOKEN` when required) for the bundled script, or connect a
   Sandrone MCP server.

Use the MCP plane only through a client that supports MCP `2026-07-28`.
Sandrone does not support the legacy initialize/initialized session lifecycle
or older protocol negotiation. If the client cannot establish that connection,
report the incompatibility and the HTTP setup alternative.

For HTTP, call `/healthz`, `/version`, `/v1/inspect`, then the narrowest
applicable `/v1/schemas/*` endpoint. For MCP, inspect the live capability
catalog, list relevant resources, and read named definitions and schema
resources.

Keep the selected plane fixed throughout a mutation. After any ambiguous
failure, reread the exact resource through that plane before deciding whether
to retry.

Read [references/workflows.md](references/workflows.md) for HTTP-first operation
mappings and task recipes. Read [references/safety.md](references/safety.md)
before any put, overwrite, delete, remote fetch, probe, or processor-script
task.

## Treat the Server as Canonical

- Fetch the applicable processor, file-kind, Subscription, FileSpec, or script
  schema through the selected plane. Never reconstruct or copy a dynamic schema
  from this Skill.
- Send only fields published by the current schema.
- Use canonical format, processor stage/type, and FileSpec kind values.
- Run file-stage processors in declared order.
- For Mihomo, sing-box, and Shadowrocket, author final `groups`. Never create,
  modify, or derive behavior from `adaptive_groups`; it is editor compatibility
  metadata. Preserve an existing value unchanged only for a compatible
  round-trip, or warn before dropping it.
- Branch on structured error `code` and context fields. Do not parse
  human-readable error text.

## Mutation Rule

Treat an explicit user request to create, update, replace, or delete the exact
named resource as authorization for that action. Do not infer authorization
from a request to inspect, draft, validate, preview, diagnose, or render.

Before overwriting or deleting, read the exact existing definition. For
ambiguous names or scopes, stop and ask. Do not add a redundant confirmation
when the user's instruction is already explicit and the target is exact.

## Return Useful Results

Lead with the completed outcome. Include the resource name, whether state was
persisted, the validation/render/probe result, and material warnings. Never
echo bearer tokens, credentials, subscription URLs, or full node secrets unless
the user explicitly requests the sensitive value.

## Common Mistakes

- Switching from HTTP to MCP during an ambiguous mutation retry.
- Using remembered schemas instead of live HTTP endpoints or MCP resources.
- Treating MCP prompts as actions.
- Substituting public `GET /convert` for full `POST /v1/convert`.
- Claiming a new subscription was previewed before it was persisted.
- Retrying put/delete after an ambiguous failure without rereading the resource.
- Inventing an artifact URI when a large response reports `body_omitted`.
