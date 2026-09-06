---
name: sandrone
description: Use when an Agent must operate Sandrone to convert proxy or subscription data, inspect or manage subscriptions and FileSpecs, render client configurations, author processors or sandbox scripts, or explain Sandrone reports.
---

# Sandrone

## Overview

Use the bundled HTTP script as the preferred execution plane and a connected
Sandrone MCP server as the fallback. Use this Skill for workflow and safety
decisions; obtain capabilities, schemas, defaults, examples, and current
resource definitions from the selected live plane when the task needs them.
Explaining an already supplied report does not require a server connection.

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

For first connection or uncertain capabilities, use the relevant discovery section
in [references/workflows.md](references/workflows.md#discover). Read only the
recipe and endpoint mappings needed for the task. Reuse still-valid discovery
and schema results within the task; refresh after a server change or schema mismatch.

Before a mutation, read the authorization and retry sections of
[references/safety.md](references/safety.md). Read its external-effects or script
sections when those operations are involved; already-read unchanged guidance
need not be reread.

## Treat the Server as Canonical

- Fetch the applicable processor, file-kind, Subscription, FileSpec, or script
  schema through the selected plane when constructing or validating that payload.
  Reuse a still-valid live schema from this task. Never reconstruct a dynamic schema
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
ambiguous names or scopes, first use available read-only context to resolve them;
ask only if the mutation target remains uncertain. Do not add a redundant confirmation
when the user's instruction is already explicit and the target is exact.

## Return Useful Results

Complete the requested draft, read, conversion, or authorized mutation and its
applicable verification before reporting the outcome. Do not stop at a draft
when persistence was authorized. If verification fails after a successful write,
report the saved state and the verification gap separately; do not repeat the
write merely to retry verification.

Lead with the completed outcome. Include the relevant resource name, whether state
was persisted, any preview/render result, and material warnings. Never
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
