# Sandrone Safety

## Writes and retries

Apply the [write rule](../SKILL.md#writes-and-verification). `put` immediately
saves and replaces the same name; `delete` has no recycle bin. Read the existing
definition before either operation. Deleting a resource does not cascade to its
references, which may then fail to resolve.

A transport error may hide whether a write succeeded. Verify the server identity
and reread the exact target before retrying. Switching interfaces is not evidence
that the first write failed. Report success using the actual response; HTTP
resource deletion returns `{"ok":true}`.

## External effects and sensitive data

Remote inputs, traffic refresh, render flows and probes may access the network.
Choose verification that exercises the changed behavior. Scripts have no arbitrary
filesystem, subprocess, environment-variable or general network access.

Definitions, outputs, reports and warnings can contain credentials and private
subscription URLs. Avoid copying them into chat or logs unless needed for the
user's request; keep request-body files private and out of tracked files.
The bundled HTTP script reads bearer credentials from `SANDRONE_TOKEN` and avoids
putting them in curl arguments. Use a trusted network or TLS for cross-host HTTP.
