---
name: sandrone
description: Operate Sandrone subscriptions, FileSpecs, conversions and processor scripts, or explain their reports.
---

# Sandrone

Use the bundled HTTP script when shell access, `curl`, and `SANDRONE_URL` are
available; a connected Sandrone MCP server is another execution option. Follow
the user's chosen interface when specified. MCP client requirements are in the
[MCP reference](https://github.com/kuuvahki-labs/sandrone/blob/main/docs/reference/mcp.md). If neither interface is available,
explain how to configure `SANDRONE_URL` (and `SANDRONE_TOKEN` when required) or
connect MCP. Explaining a supplied report needs no server connection.

Read [workflows](references/workflows.md) for endpoint mappings and resource
operations. Discover only the capabilities, schemas and current definitions
needed for the task; reuse valid results, refreshing after a server change or
schema mismatch. Live schemas describe supported payloads; this Skill does not
maintain a second field catalog.

## Writes and verification

An explicit request to create, update or delete a resource authorizes that
operation. Inspection, drafts, preview and rendering alone do not authorize
persistence. Resolve the target from available context and read its existing
definition before overwriting or deleting it; ask only if the target remains
ambiguous. Read [safety](references/safety.md) before writes or sensitive-data
handling.

Complete authorized persistence and verify the affected behavior. Input or
processor changes usually need preview/render; metadata-only changes can be
verified by reading the saved definition. Report saved state separately from a
failed verification, without repeating the write just to retry verification.

Return the resource name, persistence status, relevant output and material
warnings. Use structured error codes to interpret failures. When MCP reports
`body_omitted`, report the size limit; it has not created a hidden file or share.
