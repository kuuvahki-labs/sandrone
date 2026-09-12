# Sandrone Workflows

HTTP calls use `scripts/sandrone-api.sh METHOD PATH [BODY_FILE|-]`. Pass JSON on
stdin with `-` or in a private body file. MCP tool names may have a client namespace
prefix. Read-only calls may use either interface; keep track of the server identity.

## Discover

Use `GET /v1/inspect` or `sandrone_inspect` when capabilities are uncertain.
`/version` identifies an HTTP build. Read exact named resources directly; list
when the name is unknown. Follow pagination cursors returned by the selected
interface without changing its filter.

The runtime schema catalog is `GET /v1/schemas` or `sandrone://schemas`. It links
processor, file-kind, Subscription, FileSpec and script API schemas. Read the
relevant schemas when constructing unfamiliar payloads. Format capabilities are
at `/v1/capabilities/formats` or `sandrone://capabilities/formats`.

## Resource operations

| Operation | HTTP | MCP |
| --- | --- | --- |
| Convert temporary input | `POST /v1/convert` | `sandrone_convert` |
| List subscriptions/files | `GET /v1/subscriptions` / `GET /v1/files` | `sandrone_list_resources` |
| Read subscription | `GET /v1/subscriptions/{name}` | `sandrone://subscriptions/{name}` |
| Save subscription | `POST /v1/subscriptions` | `sandrone_put_subscription` |
| Preview subscription | `POST /v1/subscriptions/{name}/preview` | `sandrone_preview_subscription` |
| Render subscription | `POST /v1/subscriptions/{name}/render` | `sandrone_render_subscription` |
| Read FileSpec | `GET /v1/files/{name}?mode=spec` | `sandrone://files/{name}` |
| Save FileSpec | `POST /v1/files` | `sandrone_put_file` |
| Render file | `GET /v1/files/{name}?response=json` | `sandrone_get_file` |
| Delete subscription/file | `DELETE /v1/subscriptions/{name}` / `DELETE /v1/files/{name}` | `sandrone_delete_subscription` / `sandrone_delete_file` |

Saving overwrites the complete definition at the same name. Preserve unrelated
fields and processor order when editing. Preview/render by subscription name
requires a saved resource; do not claim to have previewed a new unsaved definition.
A file's `spec`, `source`, and `render` modes return its definition, base content,
and final content respectively.

Full conversion accepts temporary input and processors without creating a
Subscription or FileSpec. Public `GET /convert` is a separate processor-free
convenience route. Use full conversion when reports or processors are needed.
HTTP request shapes are in the [HTTP reference](https://github.com/kuuvahki-labs/sandrone/blob/main/docs/reference/http-api/README.md).

## Processor scripts and reports

Use the processor schema and script API schema for the chosen stage. Scripts run
as embedded ECMAScript, with the injected Sandrone API, not Node.js. File-stage
processors run in declaration order. Reusable script examples and registration
commands are in [the script guide](https://github.com/kuuvahki-labs/sandrone/blob/main/docs/how-to/write-processor-script.md).

Verify affected saved subscriptions through preview and files through render.
Local CLI `diagnose` supports draft diagnosis; HTTP and MCP do not expose diagnose
directly. MCP prompts can assist drafting or explanation but do not execute the
operation. Report warnings and loss using the structured report fields.
