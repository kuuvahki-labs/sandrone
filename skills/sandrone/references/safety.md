# Sandrone Safety

## Authorization and Transport

Shell execution grants HTTP transport access, not mutation authorization. MCP
tool availability likewise does not authorize mutation. An explicit user
request to create, update, replace, or delete the exact named resource
authorizes that action; inspection, drafting, validation, preview, rendering,
or diagnosis does not.

`put` immediately saves and overwrites the same name. Read an existing
definition before replacement. `delete` is immediate and has no recycle bin:

- Subscription deletion removes its saved definition.
- File deletion removes the single JSON record containing the complete
  FileSpec, including inline content.

Read the complete definition before deletion.

## External Effects

Conversion with remote input, remote subscription/file sources, traffic
refresh, rendering flows, and probes may access the network through Sandrone's
controlled fetch/probe boundaries. Use them only when required by the request.
Read-only operations can still access the external world.

## Processor Scripts

Sandrone processor scripts remain embedded ECMAScript in both HTTP and MCP
flows. They have no arbitrary filesystem, subprocess, environment-variable, or
general network access. Do not write Node.js-specific code or claim that a
reserved permissions object grants host access.

## Sensitive Data

Both HTTP and MCP definitions and results may contain subscription URLs, node
credentials, processor scripts, source references, and warning context.
Minimize copying them into chat, logs, files, commits, or bug reports.

HTTP bearer configuration comes from `SANDRONE_TOKEN` in the environment only.
Never put tokens, private URLs, or private resource definitions in tracked Skill
files or command arguments. Use a trusted network or TLS-terminating reverse
proxy for cross-host HTTP.

## Retry Safety

Keep one execution plane fixed throughout each mutation. A transport error may
hide whether the server applied a put or delete. Reread the exact resource
through the same plane before retrying; idempotence is not confirmation,
rollback, or recoverability.
