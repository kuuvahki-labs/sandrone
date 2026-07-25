# MCP Agent Capability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give trusted local Agents a complete, accurately described MCP workflow for discovering, authoring, validating, executing, overwriting, and deleting subscriptions and FileSpecs.

**Architecture:** Keep business execution in `internal/service` and use `internal/entry/mcpapi` only for MCP wire adaptation. Add owner-maintained processor and typed-file descriptors, convert MCP JSON objects into the existing strict domain `json.RawMessage` shapes, and expose the same descriptors through schema resources and parameterized prompts. Use one default-off management switch and preserve the existing script sandbox.

**Tech Stack:** Go 1.24, `github.com/modelcontextprotocol/go-sdk/mcp` v1.6.1, `github.com/google/jsonschema-go/jsonschema` v0.4.3, Cobra, Testify, in-memory MCP transports, Streamable HTTP.

## Global Constraints

- Follow `docs/superpowers/specs/2026-07-25-mcp-agent-capability-design.md`.
- Entrypoint code adapts protocols only; it must call existing service methods instead of reproducing resource, conversion, rendering, validation, probing, or deletion workflows.
- MCP processor `params` and typed-file `config.settings` are JSON objects on the wire and are encoded to the existing domain `json.RawMessage` representations before entering service code.
- `--allow-management-tools` is the only management registration switch and remains disabled by default.
- Script execution remains inside the existing processor registry and Goja sandbox. Do not add generic eval, shell, fetch, filesystem, environment, or process tools.
- `docs/reference/mcp.md` remains the canonical public MCP contract; processor, FileSpec, and scripting details remain canonical in their existing reference pages.
- Run the focused test named in each task before its implementation and confirm the stated failure.
- Make each task a separate commit after its focused tests pass.

---

### Task 1: Replace the double management gate with one switch

**Files:**
- Modify: `internal/app/runtime.go`
- Modify: `internal/entry/cli/serve.go`
- Modify: `internal/entry/cli/cli_test.go`
- Modify: `internal/entry/mcpapi/server.go`
- Modify: `internal/entry/mcpapi/server_test.go`

**Interfaces:**
- Removes: `app.MCPConfig.ReadOnly`, `serveOptions.readonly`, and `--readonly`.
- Keeps: `app.MCPConfig.AllowManagementTools` and `--allow-management-tools`, default `false`.
- Registration rule: management tools are present exactly when `AllowManagementTools` is true.

- [ ] **Step 1: Add failing CLI and MCP registration tests**

Add CLI assertions that `serve mcp --help` and `serve all --help` contain
`--allow-management-tools` and do not contain `--readonly`. Replace management
fixtures with:

```go
rt := testRuntime(t, app.Config{
	MCP: app.MCPConfig{AllowManagementTools: true},
})
```

Add a table test covering `false` and `true` and asserting the presence of all
four eventual management tool names:

```go
want := []string{
	"sandrone_put_subscription",
	"sandrone_delete_subscription",
	"sandrone_put_file",
	"sandrone_delete_file",
}
```

- [ ] **Step 2: Run the focused tests and confirm failure**

Run:

```bash
go test ./internal/entry/cli ./internal/entry/mcpapi
```

Expected: FAIL because `--readonly` and `MCPConfig.ReadOnly` still exist and
delete tools have not been registered.

- [ ] **Step 3: Remove the old flag and config field**

Delete the `readonly` option, flag binding, runtime mapping, and
`MCPConfig.ReadOnly`. Change the MCP registration guard to:

```go
if !rt.Config.MCP.AllowManagementTools {
	return
}
```

Register minimal delete handlers that call the corresponding service methods;
Task 7 will extend their DTOs and output contracts.

- [ ] **Step 4: Run focused tests**

Run:

```bash
go test ./internal/entry/cli ./internal/entry/mcpapi
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/runtime.go internal/entry/cli/serve.go internal/entry/cli/cli_test.go internal/entry/mcpapi/server.go internal/entry/mcpapi/server_test.go
git commit -m "refactor: use one MCP management switch"
```

---

### Task 2: Introduce MCP wire DTOs and fix object schemas

**Files:**
- Add: `internal/entry/mcpapi/wire.go`
- Add: `internal/entry/mcpapi/wire_test.go`
- Add: `internal/entry/mcpapi/schema.go`
- Add: `internal/entry/mcpapi/schema_test.go`
- Modify: `internal/entry/mcpapi/server.go`
- Modify: `internal/entry/mcpapi/server_test.go`

**Interfaces:**
- Adds: `processorSpec`, `fileConfig`, `fileSpec`, `subscription`, and request-specific MCP DTOs.
- Adds: conversion helpers such as `processorSpec.domain()` and `fileSpec.domain()`.
- Publishes: `params` and `settings` as JSON Schema `type: object`.
- Enforces: inline/remote and saved-name/inline-spec exclusivity, enums, and numeric bounds at the MCP boundary.

- [ ] **Step 1: Write failing schema contract tests**

Inspect `session.ListTools` JSON and assert:

```go
require.JSONEq(t, `{"type":"object"}`, extractSchema(
	tools, "sandrone_convert", "properties.parse_processors.items.properties.params",
))
require.JSONEq(t, `{"type":"object"}`, extractSchema(
	tools, "sandrone_put_file", "properties.config.properties.settings",
))
```

Also assert:

- `content` and `remote` are mutually exclusive and one is required;
- `file` and `spec` are mutually exclusive and one is required;
- file mode is `spec|source|render`;
- probe method and core are enums;
- timeout, attempts, concurrency, cache TTL, limit, and cursor constraints match
  the design.

- [ ] **Step 2: Write failing executable wire tests**

Call `sandrone_convert` with ordinary object parameters:

```go
"parse_processors": []any{map[string]any{
	"type":  "rename",
	"stage": "nodes",
	"params": map[string]any{
		"mode":  "prefix",
		"value": "HK-",
	},
}},
```

Add filter and inline script cases. Assert the call reaches service, succeeds,
and changes rendered node output. Add a typed FileSpec case where
`config.settings` is a normal object and assert it reaches driver validation
rather than failing as a byte array.

- [ ] **Step 3: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/entry/mcpapi -run 'Test(ToolSchemas|ProcessorParams|TypedSettings)'
```

Expected: FAIL with the current byte-array schema or pre-service validation
message.

- [ ] **Step 4: Implement wire conversion**

Use object-shaped fields:

```go
type processorSpec struct {
	Type   string         `json:"type"`
	Stage  domain.Stage   `json:"stage,omitempty"`
	Name   string         `json:"name,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

type fileConfig struct {
	Subscriptions []string       `json:"subscriptions,omitempty"`
	Settings      map[string]any `json:"settings,omitempty"`
}
```

Encode each processor value independently:

```go
func rawObject(values map[string]any) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		body, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		out[key] = body
	}
	return out, nil
}
```

Encode settings as one JSON object. Do not loosen
`processor.UnmarshalParams` or typed driver strict decoders.

- [ ] **Step 5: Add explicit top-level schemas**

Build schemas in `schema.go` using `jsonschema.Schema` and small helpers for
object, enum, bounds, `oneOf`, and `additionalProperties`. Assign schemas to
each `mcp.Tool.InputSchema`; do not let domain structs containing RawMessage
drive public schemas.

- [ ] **Step 6: Run focused and package tests**

Run:

```bash
go test ./internal/entry/mcpapi
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/entry/mcpapi
git commit -m "fix: expose object-shaped MCP resource definitions"
```

---

### Task 3: Add owner-maintained capability descriptors

**Files:**
- Add: `internal/processor/descriptor.go`
- Add: `internal/processor/descriptor_test.go`
- Modify: `internal/processor/registry.go`
- Modify: `internal/processor/node/register.go`
- Modify: `internal/processor/file/register.go`
- Modify: `internal/processor/script/processor.go`
- Modify: processor parameter structs under `internal/processor/node/*.go`
- Modify: processor parameter structs under `internal/processor/file/*.go`
- Modify: `internal/processor/script/engine.go`
- Add: `internal/service/file_kind_capability.go`
- Add: `internal/service/file_kind_capability_test.go`
- Modify: `internal/service/typed_file_registry.go`
- Modify: `internal/service/typed_file_settings.go`
- Modify: `internal/service/typed_file_shadowrocket_settings.go`
- Modify: typed-file driver registrations under `internal/service/typed_file_*.go`

**Interfaces:**
- Adds: `processor.Descriptor` with type, stage, description, parameter prototype,
  effects, examples, error codes, and `Public` visibility.
- Adds: `Registry.Descriptors()` and `Registry.PublicDescriptors()`.
- Keeps: existing `RegisterNode` and `RegisterFile` for internal/custom callers;
  adds descriptor-aware registration methods for built-ins.
- Adds: `service.FileKindCapability` and
  `Service.FileKindCapabilities()` with settings prototype, source rules,
  defaults, and examples.

- [ ] **Step 1: Write failing registry descriptor tests**

Assert that:

- all built-in node processors have a descriptor;
- `script` has both nodes and file descriptors;
- `inject_nodes` is registered for execution but absent from
  `PublicDescriptors`;
- descriptor order is stable by stage then type;
- every public descriptor has a non-nil parameter prototype and examples.

- [ ] **Step 2: Write failing typed-file capability tests**

Assert exact canonical kinds:

```go
require.Equal(t,
	[]domain.FileKind{
		domain.FileKindStatic,
		domain.FileKindMihomo,
		domain.FileKindSingBox,
		domain.FileKindShadowrocket,
	},
	kinds,
)
```

For typed kinds, require a settings prototype, media type, syntax, default
extension, source rules, and minimal example. Assert descriptor settings
marshal into input accepted by the matching driver's `ValidateSettings`.

- [ ] **Step 3: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/processor ./internal/service -run 'Test.*(Descriptor|Capability)'
```

Expected: FAIL because the descriptor APIs do not exist.

- [ ] **Step 4: Implement processor descriptors**

Add:

```go
type Effects struct {
	Probes       bool `json:"probes,omitempty"`
	RemoteReads  bool `json:"remote_reads,omitempty"`
	RunsScript   bool `json:"runs_script,omitempty"`
}

type Descriptor struct {
	Type            string
	Stage           domain.Stage
	Description     string
	ParamsPrototype any
	Effects         Effects
	Examples        []map[string]any
	ErrorCodes      []domain.ErrorCode
	Public          bool
}
```

Descriptor-aware registration stores the builder and descriptor atomically.
Existing registration methods continue to work for service tests and extension
hooks without advertising undocumented schemas.

Add JSON Schema tags to parameter structs for their real enum, minimum,
maximum, and description constraints. The descriptor is metadata only;
builders remain the final validator.

- [ ] **Step 5: Implement typed-file capabilities**

Extend the private driver descriptor with a settings prototype and source
rules. Export only an immutable copied capability view from `Service`. Rename
the settings structs to exported names inside the internal service package so
their concrete types can drive schema reflection without moving driver logic
into MCP.

- [ ] **Step 6: Run focused and owner-package tests**

Run:

```bash
go test ./internal/processor/... ./internal/service
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/processor internal/service
git commit -m "feat: describe public processors and file kinds"
```

---

### Task 4: Publish discoverable definition and schema resources

**Files:**
- Add: `internal/entry/mcpapi/resources.go`
- Add: `internal/entry/mcpapi/resources_test.go`
- Add: `internal/entry/mcpapi/catalog.go`
- Add: `internal/entry/mcpapi/catalog_test.go`
- Modify: `internal/entry/mcpapi/server.go`

**Interfaces:**
- Keeps: `sandrone://capabilities`,
  `sandrone://subscriptions/{name}`, and `sandrone://files/{name}`.
- Adds: processor summary/detail, file-kind detail, and script API v1 resources.
- Lists: fixed schema/capability resources through standard `resources/list`
  and stored-definition templates through `resources/templates/list`.
- Leaves: dynamic stored-name discovery to `sandrone_list_resources` in Task 5.
- Uses: owner descriptors from Task 3 as the only schema/prompt metadata source.

- [ ] **Step 1: Write failing resource discovery tests**

Call `session.ListResources` and assert that the response contains:

```text
sandrone://capabilities
sandrone://schemas/processors
sandrone://schemas/script-api/v1
```

Call `session.ListResourceTemplates` and assert the subscription, file,
processor-detail, and file-kind templates are present. Read every fixed or
instantiated schema URI and assert valid JSON, canonical stage/type or kind,
parameter/settings schema, effects/source rules, examples, and stable error
codes.

- [ ] **Step 2: Write failing schema/decoder consistency tests**

For every public processor example:

1. validate `params` against its published schema;
2. construct through the real registry.

For every file-kind example:

1. validate `settings` against its published schema;
2. validate through the real driver/service path.

Assert that `inject_nodes` is absent and that script schemas describe only
inline, controlled file-resource, and controlled remote sources.

- [ ] **Step 3: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/entry/mcpapi -run 'Test(ResourceDiscovery|SchemaResources|PublishedSchemas)'
```

Expected: FAIL because only capabilities is a concrete resource and schema
resources do not exist.

- [ ] **Step 4: Implement schema catalog projection**

Reflect each descriptor prototype with `jsonschema.For` and add the owner
metadata around the resulting schema. Build the script API v1 document from
the existing exported script envelope/config types plus explicit API method
descriptors; do not copy JavaScript runtime behavior into MCP handlers.

- [ ] **Step 5: Register fixed resources and definition templates**

Register fixed capabilities, schema summaries, and script API entries as
standard MCP resources. Preserve URI templates for direct subscription,
FileSpec, processor-detail, and file-kind reads. Keep public-name and canonical
stage/type/kind validation on template reads. Do not snapshot stored names into
server registration because put/delete must be visible immediately through the
Task 5 listing tool.

- [ ] **Step 6: Run focused and package tests**

Run:

```bash
go test ./internal/entry/mcpapi
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/entry/mcpapi
git commit -m "feat: publish MCP resource and schema catalogs"
```

---

### Task 5: Add paginated listing and filtered capability tools

**Files:**
- Add: `internal/entry/mcpapi/tools_discovery.go`
- Add: `internal/entry/mcpapi/tools_discovery_test.go`
- Modify: `internal/entry/mcpapi/server.go`
- Modify: `internal/entry/mcpapi/schema.go`

**Interfaces:**
- Adds: `sandrone_list_resources` with `kind`, `cursor`, and `limit`.
- Expands: `sandrone_inspect_capabilities` with optional `kind` and `name`
  filters while retaining an unfiltered summary.
- Cursor: opaque URL-safe base64 encoding of kind plus the next stable offset.
- Limit: default 50, minimum 1, maximum 200.

- [ ] **Step 1: Write failing list tests**

Cover empty lists, each kind, both kinds, `limit=1` pagination without duplicate
or skipped items, invalid cursor, invalid kind, and illegal limits. Assert each
item has `resource_uri`.

- [ ] **Step 2: Write failing inspect filter tests**

Cover summary, one format, one public processor, one file kind, unknown name,
and ensure `inject_nodes` is not returned as public.

- [ ] **Step 3: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/entry/mcpapi -run 'Test(ListResources|InspectCapabilities)'
```

Expected: FAIL because list and filters are absent.

- [ ] **Step 4: Implement discovery adapters**

Call `Service.ListSubscriptions`, `Service.ListFiles`,
`Service.CapabilitySummary`, processor descriptors, and file-kind capabilities.
Sort before pagination and keep cursor parsing in MCP code. Do not add
pagination to service solely for this bounded local metadata list.

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/entry/mcpapi -run 'Test(ListResources|InspectCapabilities)'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/entry/mcpapi
git commit -m "feat: add MCP resource discovery tools"
```

---

### Task 6: Expose subscription execution tools

**Files:**
- Add: `internal/entry/mcpapi/tools_subscription.go`
- Add: `internal/entry/mcpapi/tools_subscription_test.go`
- Modify: `internal/entry/mcpapi/server.go`
- Modify: `internal/entry/mcpapi/schema.go`

**Interfaces:**
- Adds: `sandrone_preview_subscription`.
- Adds: `sandrone_render_subscription`.
- Adds: `sandrone_get_subscription_traffic`.
- Calls: `Service.PreviewSubscription`, `Service.RenderSubscription`, and
  `Service.SubscriptionTraffic`.

- [ ] **Step 1: Write failing success-path tests**

Seed a local subscription with a rename processor. Assert preview returns
before/after information and warnings, render returns target content and
report, and request `args` reach processor/script execution.

Seed a remote subscription with an `httptest.Server` traffic header and assert
the traffic tool returns the service result.

- [ ] **Step 2: Write failing boundary tests**

Cover missing resource, invalid one-segment name, unsupported format, remote
fetch failure, and traffic requested for an unsupported local subscription.
Assert annotations:

```go
ReadOnlyHint == true
OpenWorldHint == true
```

for all three because their execution can read remote inputs.

- [ ] **Step 3: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/entry/mcpapi -run 'TestSubscription(Preview|Render|Traffic)'
```

Expected: FAIL because the tools are not registered.

- [ ] **Step 4: Implement thin handlers and schemas**

Map request fields into the existing service request types. Keep preview,
render, traffic interpretation and reporting entirely in service.

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/entry/mcpapi -run 'TestSubscription(Preview|Render|Traffic)'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/entry/mcpapi
git commit -m "feat: expose MCP subscription execution"
```

---

### Task 7: Complete file execution and resource lifecycle tools

**Files:**
- Add: `internal/entry/mcpapi/tools_file.go`
- Add: `internal/entry/mcpapi/tools_file_test.go`
- Add: `internal/entry/mcpapi/tools_management.go`
- Add: `internal/entry/mcpapi/tools_management_test.go`
- Modify: `internal/entry/mcpapi/server.go`
- Modify: `internal/entry/mcpapi/schema.go`

**Interfaces:**
- Expands: `sandrone_get_file` modes to `spec|source|render` and adds `args`.
- Keeps: `sandrone_validate_file`, returning only `ok` and `report`.
- Adds/finalizes: put/delete subscription and file tools.
- Put output: `ok` plus `resource_uri`.
- Delete output: `ok`, `deleted`, plus `resource_uri`.

- [ ] **Step 1: Write failing file flow tests**

Exercise:

```text
put → read definition resource → source → render(args) → validate
→ overwrite → render changed content → delete → missing read
```

Run it for static and at least Mihomo and sing-box typed files. Assert
`source` calls `Service.GetFileSource`, render passes `args` in
`domain.FileRequest`, and validate output has no impossible body/spec fields.

- [ ] **Step 2: Write failing subscription lifecycle test**

Exercise:

```text
put → list → read → preview → render → overwrite → read changed definition
→ delete → missing read
```

Assert delete of a nonexistent resource preserves the existing service/store
error instead of returning fake success.

- [ ] **Step 3: Write failing annotation tests**

Assert put and delete tools have:

```go
ReadOnlyHint:   false
DestructiveHint: ptr(true)
IdempotentHint: true
OpenWorldHint:  ptr(false)
```

Assert get-file render/validate use `OpenWorldHint=true`, while static
definition resources remain read-only.

- [ ] **Step 4: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/entry/mcpapi -run 'Test(FileLifecycle|SubscriptionLifecycle|ToolAnnotations)'
```

Expected: FAIL because source/args/delete/output and annotations are incomplete.

- [ ] **Step 5: Implement lifecycle adapters**

Use explicit request and response DTOs. Generate resource URIs through one
escaped helper. Call only:

```go
rt.Service.PutSubscription
rt.Service.DeleteSubscription
rt.Service.PutFile
rt.Service.DeleteFile
rt.Service.GetFileSpec
rt.Service.GetFileSource
rt.Service.GetFile
rt.Service.ValidateFile
```

Do not pre-read resources or add server-side confirmation/version checks.

- [ ] **Step 6: Run focused and MCP package tests**

Run:

```bash
go test ./internal/entry/mcpapi
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/entry/mcpapi
git commit -m "feat: complete MCP resource lifecycle"
```

---

### Task 8: Make errors and omitted bodies machine-readable

**Files:**
- Add: `internal/entry/mcpapi/tool_handler.go`
- Add: `internal/entry/mcpapi/tool_handler_test.go`
- Add: `internal/entry/mcpapi/errors.go`
- Add: `internal/entry/mcpapi/errors_test.go`
- Modify: `internal/entry/mcpapi/schema.go`
- Modify: all `internal/entry/mcpapi/tools_*.go`
- Modify: `internal/entry/mcpapi/server.go`

**Interfaces:**
- Adds: a low-level generic MCP handler adapter that validates, applies defaults,
  decodes, invokes, validates output, and converts every input/service error to
  a tool result with `IsError=true`.
- Error shape: `error.code`, `message`, optional `field`,
  `resource_kind`, and `resource_name`.
- Body shape adds: `body_omitted`, `body_bytes`, and `max_output_bytes`.

- [ ] **Step 1: Write failing structured error tests**

For every always-registered tool, trigger one input validation error and one
service/domain error. Assert:

```go
require.True(t, result.IsError)
require.Equal(t, "invalid_argument", errorBody.Error.Code)
require.NotEmpty(t, errorBody.Error.Message)
```

For processor config, script timeout, missing resource, remote fetch, and
typed-settings decode errors, assert the existing domain code is preserved.
Where the adapter knows context, assert exact field/resource metadata.

- [ ] **Step 2: Write failing output-limit tests**

Set `MaxOutputBytes` to a small value and call convert, subscription render,
and file render. Assert:

```json
{
  "body_omitted": true,
  "body_bytes": 128,
  "max_output_bytes": 32
}
```

with the actual measured values, an empty omitted body, and the report retained.
Also assert a body exactly at the limit is returned normally.

- [ ] **Step 3: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/entry/mcpapi -run 'Test(StructuredToolErrors|BodyLimit)'
```

Expected: FAIL because SDK-generated validation errors are plain text and body
omission is silent.

- [ ] **Step 4: Implement the shared low-level handler**

Use `Server.AddTool`, `jsonschema.Resolve`, `Resolved.ApplyDefaults`,
`Resolved.Validate`, strict JSON decoding, and output validation. The handler
must return protocol errors only for genuine server/protocol faults; invalid
arguments and application failures become structured MCP tool errors.

Create error results with both text content and structured content, then call:

```go
result.SetError(err)
```

Preserve `domain.AppError` fields and map schema validation paths into MCP wire
field names. Do not expose nested causes containing credentials or remote
subscription bodies.

- [ ] **Step 5: Implement explicit omission metadata**

Measure the original byte length before omitting. Apply the helper only to
body-bearing output DTOs. Add a package test documenting that reports, probe
arrays, and resource JSON are not yet globally bounded.

- [ ] **Step 6: Run focused and package tests**

Run:

```bash
go test ./internal/entry/mcpapi
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/entry/mcpapi
git commit -m "feat: structure MCP errors and output limits"
```

---

### Task 9: Replace static prompts with parameterized guidance

**Files:**
- Add: `internal/entry/mcpapi/prompts.go`
- Add: `internal/entry/mcpapi/prompts_test.go`
- Modify: `internal/entry/mcpapi/server.go`

**Interfaces:**
- Adds: `build_subscription`, `build_file`, `write_processor_script`,
  `diagnose_conversion_loss`, and `explain_report`.
- Removes: `convert_nodes`, `design_mihomo_file`, and
  `design_sing_box_file`.
- Prompt output references schema resource URIs and tool names; it does not
  duplicate full schemas.

- [ ] **Step 1: Write failing prompt catalog tests**

Call `ListPrompts` and assert the exact five prompt names and required/optional
arguments from the design. Call each prompt with representative arguments and
assert its result:

- names the next validation/execution tool;
- links the relevant schema resource URI;
- incorporates supplied target/kind/stage/report inputs;
- never recommends generic eval, shell, unrestricted fetch, or filesystem.

- [ ] **Step 2: Write failing descriptor reuse tests**

For every public processor and file kind, generate the relevant prompt and
assert the canonical name and schema URI come from the same catalog used by
resources. This test must fail if a prompt hard-codes a stale processor list.

- [ ] **Step 3: Run focused tests and confirm failure**

Run:

```bash
go test ./internal/entry/mcpapi -run 'Test.*Prompt'
```

Expected: FAIL because current prompts have no parameters and obsolete names.

- [ ] **Step 4: Implement parameterized prompts**

Declare `mcp.PromptArgument` entries and validate enum-like prompt inputs in the
handler. Compose concise workflow guidance from the Task 4 catalog:

```text
read schema → draft definition/script → preview or validate → put if requested
→ execute and inspect report
```

Treat preview/validate as recommendations, not mandatory write gates.

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/entry/mcpapi -run 'Test.*Prompt'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/entry/mcpapi
git commit -m "feat: add MCP authoring prompts"
```

---

### Task 10: Prove script boundaries and both transports

**Files:**
- Add: `internal/entry/mcpapi/script_contract_test.go`
- Add: `internal/entry/mcpapi/transport_test.go`
- Modify: `internal/entry/mcpapi/server_test.go`

**Interfaces:**
- Verifies: inline, controlled stored-file, and controlled remote script sources.
- Verifies: script timeout and sandbox denial boundaries.
- Verifies: in-memory/stdio protocol behavior and Streamable HTTP behavior.

- [ ] **Step 1: Add script contract tests**

Through public MCP tools, cover:

- inline script success at nodes and file stages;
- stored file-resource script success;
- remote script success with explicit existing permissions;
- timeout returns `script_timeout`;
- `require`, `process`, environment, and arbitrary filesystem access remain
  unavailable;
- arbitrary network access is unavailable except the existing narrow controlled
  service APIs and declared remote source loading.

- [ ] **Step 2: Add Streamable HTTP smoke test**

Serve `Server.Handler()` with `httptest.Server`, connect an MCP client using the
SDK Streamable HTTP transport, initialize, list tools/resources/prompts, call
`sandrone_inspect_capabilities`, and read one schema resource.

- [ ] **Step 3: Add stdio smoke coverage**

Exercise the real stdio transport through a subprocess test of the built
`sandrone serve mcp --transport stdio` binary. Send initialize, initialized,
tools/list, and tools/call frames and assert clean shutdown when stdin closes.
Do not rely only on in-memory transports.

- [ ] **Step 4: Run focused tests and confirm any failures**

Run:

```bash
go test ./internal/entry/mcpapi -run 'Test(ScriptContract|StreamableHTTP|Stdio)'
```

Expected before final wiring: at least one test FAIL if a schema resource,
structured error, or transport path is not connected through `New`.

- [ ] **Step 5: Fix assembly-only gaps**

Limit fixes to server construction, transport test helpers, or MCP adapters.
Do not weaken the script sandbox to make a test pass.

- [ ] **Step 6: Run MCP and related service tests**

Run:

```bash
go test ./internal/entry/mcpapi ./internal/processor/script ./internal/service
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/entry/mcpapi
git commit -m "test: cover MCP scripts and transports"
```

---

### Task 11: Update the canonical contract and run repository gates

**Files:**
- Modify: `docs/reference/mcp.md`
- Modify: `docs/reference/cli.md`
- Modify: `docs/reference/processors.md`
- Modify: `docs/reference/file-spec.md`
- Modify: `docs/reference/scripting-api.md`
- Modify: `docs/README.md`

**Interfaces:**
- Documents: exact tool/resource/prompt catalogs, management switch, wire
  object differences, annotations, structured errors, pagination, omission
  metadata, script boundaries, and recommended Agent flow.
- Links: domain details to existing canonical pages instead of copying them.

- [ ] **Step 1: Update MCP and CLI reference tests or doc checks**

If the repository has no direct doc tests, use search assertions during this
task. The MCP reference must contain every public tool/resource/prompt name and
must not describe the old double gate.

- [ ] **Step 2: Update canonical documentation**

Document this workflow:

```text
inspect → list → read definition/schema → optional prompt
→ preview/validate → put/delete/render/probe
```

Explicitly state:

- management is trusted-local and default-off;
- `params` and `settings` are objects on MCP wire;
- put may overwrite and delete is immediate;
- script is sandboxed and not generic code execution;
- body omission never creates a hidden share/file;
- reports/probe/resource JSON are not globally covered by the first-phase body
  limit.

- [ ] **Step 3: Confirm obsolete identifiers are gone**

Run:

```bash
rg -n -- '--readonly|MCP\\.ReadOnly|ReadOnly:' internal docs
rg -n 'convert_nodes|design_mihomo_file|design_sing_box_file' internal docs
```

Expected: no matches except historical approved design/plan documents where the
removed behavior is explicitly discussed.

- [ ] **Step 4: Run formatting and focused gates**

Run:

```bash
gofmt -w internal/app internal/entry/cli internal/entry/mcpapi internal/processor internal/service
go test ./internal/entry/cli ./internal/entry/mcpapi ./internal/processor/... ./internal/service
```

Expected: PASS.

- [ ] **Step 5: Run the full repository gate**

Run:

```bash
make check
```

Expected: PASS.

- [ ] **Step 6: Review the final diff**

Run:

```bash
git status --short
git diff --check
git diff --stat
```

Verify the implementation has no credentials, private URLs, runtime data,
generated Inspector tokens, unfinished markers, or unrelated edits.

- [ ] **Step 7: Commit**

```bash
git add docs internal
git commit -m "docs: publish complete MCP agent workflow"
```

## Acceptance Checklist

- [ ] `params` and `settings` are object-shaped in listed schemas and executable
  calls.
- [ ] All nine always-on and four gated management tools match the approved
  names and behavior.
- [ ] Stored definitions are discoverable and readable using standard MCP
  resources.
- [ ] Processor, file-kind, and script API resources validate against actual
  builders/drivers.
- [ ] Subscription and file lifecycle tests include create, read, execute,
  overwrite, and delete.
- [ ] No generic execution or sandbox escape surface was introduced.
- [ ] Input and service errors have stable structured codes.
- [ ] Omitted bodies report actual and configured byte sizes.
- [ ] Tool annotations match real remote and destructive behavior.
- [ ] Prompts are parameterized and reuse the schema catalog.
- [ ] Stdio and Streamable HTTP smoke tests pass.
- [ ] The old readonly switch is absent from runtime code, CLI help, tests, and
  current reference documentation.
- [ ] `make check` passes.
