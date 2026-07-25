# Sandrone Agent Skill HTTP Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the Sandrone Agent Skill complete its full conversion, probing, subscription, FileSpec, processor, and resource-management workflow through a bundled authenticated HTTP script while retaining MCP as an optional fallback.

**Architecture:** Extract the existing MCP-owned processor, FileSpec-kind, and script schema builders into an entrypoint-neutral `internal/agentcatalog` package. Add authenticated HTTP adapters for the three MCP-only execution operations and for `/v1/schemas/*`; all handlers call the same service methods and catalog builders as MCP. The Skill ships a transport-only shell wrapper that reads `SANDRONE_URL` and `SANDRONE_TOKEN`; Skill instructions select that wrapper first and MCP second.

**Tech Stack:** Go 1.24, `net/http`, `httptest`, `github.com/google/jsonschema-go/jsonschema`, Testify, POSIX `sh`, `curl`, Agent Skills Markdown/YAML.

## Global Constraints

- Follow `docs/superpowers/specs/2026-07-25-agent-skill-http-client-design.md`.
- Keep business orchestration in `internal/service`; HTTP and MCP remain protocol adapters.
- Do not copy processor, FileSpec-kind, or script schemas into the Skill.
- Preserve all existing MCP resource JSON shapes and URIs.
- Protect every `/v1/schemas/*` route with the existing `/v1/*` bearer middleware.
- Prefer the HTTP script only when shell execution, `curl`, and `SANDRONE_URL` are available; otherwise use a connected Sandrone MCP server.
- Never accept `SANDRONE_TOKEN` as a script argument, print it, or place its value in the `curl` process arguments.
- Do not retry mutations after an ambiguous transport failure.
- Use explicit canonical `FileSpec.kind` values and keep `adaptive_groups` out of public capability schemas.
- Preserve unrelated worktree changes and stage only files belonging to each task.

---

## File Structure

### New files

- `internal/agentcatalog/catalog.go`: protocol-neutral public schema document types and builders moved from MCP.
- `internal/agentcatalog/catalog_test.go`: direct catalog contract tests.
- `internal/entry/httpapi/handlers_agent_execution.go`: full authenticated convert, probe, and subscription-render adapters.
- `internal/entry/httpapi/agent_execution_test.go`: execution route, authentication, wire-shape, and MCP-semantic parity tests.
- `internal/entry/httpapi/handlers_schemas.go`: authenticated read-only schema handlers and canonical lookup.
- `internal/entry/httpapi/schemas_test.go`: schema route, parity, authentication, and malformed-path tests.
- `skills/sandrone/scripts/sandrone-api.sh`: transport-only HTTP wrapper.
- `internal/entry/httpapi/skill_script_test.go`: black-box tests for the bundled script.
- `internal/entry/httpapi/skill_contract_test.go`: installed Skill metadata and execution-plane contract tests.
- `docs/reference/http-api/schemas.md`: canonical HTTP schema route reference.
- `docs/reference/http-api/probing.md`: canonical authenticated HTTP probe reference.

### Modified files

- `internal/entry/mcpapi/resources.go`: read schema documents from `internal/agentcatalog`.
- `internal/entry/mcpapi/tools_discovery.go`: use shared detail document types/builders.
- `internal/entry/mcpapi/prompts.go`: use the shared processor schema URI helper.
- `internal/entry/mcpapi/schema.go`: delegate shared resource-schema builders to `internal/agentcatalog`.
- `internal/entry/httpapi/server.go`: register the execution and schema route families.
- `internal/entry/httpapi/types.go`: add HTTP wire types for full conversion and subscription rendering.
- `skills/sandrone/SKILL.md`: select HTTP first, MCP second, and update compatibility.
- `skills/sandrone/agents/openai.yaml`: remove mandatory MCP dependency and MCP-only UI copy.
- `skills/sandrone/references/workflows.md`: map workflows to HTTP and MCP.
- `skills/sandrone/references/safety.md`: make safety rules transport-neutral.
- `docs/reference/http-api/README.md`: link the schema route reference.
- `docs/reference/http-api/conversion.md`: document authenticated full conversion.
- `docs/reference/http-api/subscriptions.md`: document direct subscription rendering.
- `docs/reference/mcp.md`: describe the Skill’s optional MCP role.
- `docs/README.md`: expose the HTTP schema reference through existing HTTP navigation.

### Removed file

- `internal/entry/mcpapi/catalog.go`: its entrypoint-neutral contents move to `internal/agentcatalog/catalog.go`.

---

### Task 1: Extract the Shared Agent Schema Catalog

**Files:**
- Create: `internal/agentcatalog/catalog.go`
- Create: `internal/agentcatalog/catalog_test.go`
- Modify: `internal/entry/mcpapi/resources.go`
- Modify: `internal/entry/mcpapi/tools_discovery.go`
- Modify: `internal/entry/mcpapi/prompts.go`
- Modify: `internal/entry/mcpapi/schema.go`
- Remove: `internal/entry/mcpapi/catalog.go`
- Test: `internal/agentcatalog/catalog_test.go`
- Test: `internal/entry/mcpapi/catalog_test.go`
- Test: `internal/entry/mcpapi/resources_test.go`
- Test: `internal/entry/mcpapi/prompts_test.go`

**Interfaces:**
- Produces: `agentcatalog.ProcessorSummary([]processor.Descriptor) ProcessorSummaryDocument`
- Produces: `agentcatalog.ProcessorDetail(processor.Descriptor) (ProcessorCatalogDocument, error)`
- Produces: `agentcatalog.FileKindDetail(service.FileKindCapability) (FileKindCatalogDocument, error)`
- Produces: `agentcatalog.ScriptAPI() (ScriptAPIDocument, error)`
- Produces: `agentcatalog.ProcessorSchemaURI(domain.Stage, string) string`
- Produces: `agentcatalog.SubscriptionSchema() *jsonschema.Schema`
- Produces: `agentcatalog.FileSpecSchema(requireName bool) *jsonschema.Schema`
- Consumes: existing `processor.Descriptor`, `service.FileKindCapability`, typed settings prototypes, and script envelope types.

- [ ] **Step 1: Write a failing direct catalog test**

Create `internal/agentcatalog/catalog_test.go`:

```go
package agentcatalog_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/agentcatalog"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestCatalogBuildsServerOwnedDocuments(t *testing.T) {
	svc := service.New()

	summary := agentcatalog.ProcessorSummary(svc.Registry().PublicDescriptors())
	require.NotEmpty(t, summary.Processors)
	require.Equal(t,
		"sandrone://schemas/processors/nodes/rename",
		agentcatalog.ProcessorSchemaURI(domain.StageNodes, "rename"),
	)

	script, err := agentcatalog.ScriptAPI()
	require.NoError(t, err)
	require.Equal(t, 1, script.Version)

	var mihomo service.FileKindCapability
	for _, capability := range svc.FileKindCapabilities() {
		if capability.Kind == domain.FileKindMihomo {
			mihomo = capability
			break
		}
	}
	fileKind, err := agentcatalog.FileKindDetail(mihomo)
	require.NoError(t, err)
	require.Equal(t, domain.FileKindMihomo, fileKind.Kind)
	require.True(t, fileKind.SettingsSupported)
	require.NotNil(t, fileKind.SettingsSchema)

	require.Equal(t, "object", agentcatalog.SubscriptionSchema().Type)
	require.Contains(t, agentcatalog.SubscriptionSchema().Required, "name")
	require.Equal(t, "object", agentcatalog.FileSpecSchema(true).Type)
	require.Contains(t, agentcatalog.FileSpecSchema(true).Required, "kind")
}
```

- [ ] **Step 2: Run the direct test and verify RED**

Run:

```bash
go test ./internal/agentcatalog
```

Expected: FAIL because `internal/agentcatalog` and its exported builders do not exist.

- [ ] **Step 3: Move catalog construction into the neutral package**

Move the document types and schema-generation helpers from
`internal/entry/mcpapi/catalog.go` into
`internal/agentcatalog/catalog.go`, change the package declaration, and export:

```go
func ProcessorSummary(descriptors []processor.Descriptor) ProcessorSummaryDocument
func ProcessorDetail(descriptor processor.Descriptor) (ProcessorCatalogDocument, error)
func FileKindDetail(capability service.FileKindCapability) (FileKindCatalogDocument, error)
func ScriptAPI() (ScriptAPIDocument, error)
func ProcessorSchemaURI(stage domain.Stage, processorType string) string
func SubscriptionSchema() *jsonschema.Schema
func FileSpecSchema(requireName bool) *jsonschema.Schema
```

Export the document types and fields already serialized by JSON:

```go
type ProcessorSummaryDocument struct {
	Processors []ProcessorSummaryEntry `json:"processors"`
}

type ProcessorCatalogDocument struct {
	Type         string             `json:"type"`
	Stage        domain.Stage       `json:"stage"`
	Description  string             `json:"description"`
	ParamsSchema *jsonschema.Schema `json:"params_schema"`
	Effects      processor.Effects  `json:"effects"`
	Examples     []map[string]any   `json:"examples"`
	ErrorCodes   []domain.ErrorCode `json:"error_codes"`
}

type FileKindCatalogDocument struct {
	Kind              domain.FileKind             `json:"kind"`
	Description       string                      `json:"description"`
	SettingsSupported bool                        `json:"settings_supported"`
	SettingsSchema    *jsonschema.Schema          `json:"settings_schema,omitempty"`
	MediaType         string                      `json:"media_type"`
	Syntax            string                      `json:"syntax"`
	DefaultExtension  string                      `json:"default_extension"`
	SourceRules       service.FileKindSourceRules `json:"source_rules"`
	Defaults          map[string]any              `json:"defaults"`
	Examples          []map[string]any            `json:"examples"`
}
```

Keep all existing JSON tags, constraint-tag projection, nullable handling,
strict-settings null stripping, examples, method descriptions, error codes,
and `adaptive_groups` exclusion behavior unchanged.

Move the existing closed Subscription/FileSpec wire-schema builders and their
shared `ProcessorSpec`, `NodeInput`, `RemoteInput`, source, config, and basic
schema helpers into `internal/agentcatalog`. Keep MCP-only tool wrappers in
`mcpapi/schema.go`, but make `putSubscriptionInputSchema`,
`putFileInputSchema`, and `validateFileInputSchema` call the shared exported
builders so MCP and HTTP cannot drift.

- [ ] **Step 4: Rewire MCP to the shared package**

In MCP resources, discovery tools, and prompts, replace unqualified catalog
calls and types with `agentcatalog`:

```go
return agentcatalog.ProcessorSummary(
	rt.Service.Registry().PublicDescriptors(),
), nil

document, err := agentcatalog.ProcessorDetail(descriptor)
document, err := agentcatalog.FileKindDetail(capability)
return agentcatalog.ScriptAPI()
```

Use `agentcatalog.ProcessorSchemaURI` in prompt guidance. Remove
`internal/entry/mcpapi/catalog.go` only after all references have moved.

- [ ] **Step 5: Run focused catalog and MCP tests**

Run:

```bash
go test ./internal/agentcatalog ./internal/entry/mcpapi \
  -run 'Test(CatalogBuildsServerOwnedDocuments|Published|SchemaResources|.*Prompt)'
```

Expected: PASS with unchanged MCP schema JSON and prompt URIs.

- [ ] **Step 6: Confirm no old private catalog symbols remain**

Run:

```bash
rg -n 'processorSummaryCatalog|processorDetailCatalog|fileKindDetailCatalog|scriptAPICatalog|processorSchemaURI' internal
```

Expected: no matches outside history or intentionally renamed exported calls.

- [ ] **Step 7: Commit the extraction**

```bash
git add internal/agentcatalog internal/entry/mcpapi
git commit -m "refactor: share Agent schema catalog"
```

---

### Task 2: Add the Complete Authenticated HTTP Execution Surface

**Files:**
- Create: `internal/entry/httpapi/handlers_agent_execution.go`
- Create: `internal/entry/httpapi/agent_execution_test.go`
- Modify: `internal/entry/httpapi/server.go`
- Modify: `internal/entry/httpapi/types.go`
- Modify: `internal/entry/httpapi/resources.go`
- Test: `internal/entry/httpapi/agent_execution_test.go`

**Interfaces:**
- Produces: `POST /v1/convert` consuming `convertRequest` and returning `agentRenderResponse`.
- Produces: `POST /v1/probe` consuming `domain.ProbeRequest` and returning `domain.ProbeResult`.
- Produces: `POST /v1/subscriptions/{name}/render` consuming `subscriptionRenderRequest` and returning `agentRenderResponse`.
- Consumes: `Service.Convert`, `Service.Probe`, `Service.RenderSubscription`, existing path-name validation, JSON decoding, bearer middleware, and structured service errors.

- [ ] **Step 1: Write failing conversion and bearer-boundary tests**

Create `internal/entry/httpapi/agent_execution_test.go` with an authenticated
full-conversion request:

```go
func TestAgentConvertSupportsProcessorsAndStructuredReport(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	handler := httpapi.New(rt).Handler()
	body := `{
		"from_format":"uri-list",
		"to_format":"json-nodes",
		"content":"ss://aes-128-gcm:secret@example.com:8388#node-a",
		"parse_processors":[
			{"type":"rename","stage":"nodes","params":{"mode":"prefix","value":"http-"}}
		],
		"meta":{"caller":"skill"}
	}`
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost, "/v1/convert", strings.NewReader(body),
	))

	require.Equal(t, http.StatusOK, rec.Code)
	var result struct {
		ContentType string        `json:"content_type"`
		Body        string        `json:"body"`
		Report      domain.Report `json:"report"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Equal(t, "application/json", result.ContentType)
	require.Contains(t, result.Body, `"name": "http-node-a"`)
	require.Equal(t, "convert", result.Report.Kind)
}

func TestAgentExecutionRoutesRequireBearer(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	handler := httpapi.New(rt).Handler()
	for _, path := range []string{
		"/v1/convert",
		"/v1/probe",
		"/v1/subscriptions/demo/render",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(
			http.MethodPost, path, strings.NewReader(`{}`),
		))
		require.Equal(t, http.StatusUnauthorized, rec.Code, path)
	}
}
```

- [ ] **Step 2: Write failing subscription-render and probe tests**

In the same test file, save a local subscription and assert that
`POST /v1/subscriptions/local/render` accepts an explicit format and request
arguments:

```go
func TestAgentSubscriptionRenderReturnsBodyAndReport(t *testing.T) {
	ctx := context.Background()
	rt := testRuntime(t, app.Config{})
	require.NoError(t, rt.Service.PutSubscription(ctx, domain.Subscription{
		Name: "local", Type: domain.SubscriptionTypeLocal, Format: "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
	}))

	rec := httptest.NewRecorder()
	httpapi.New(rt).Handler().ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/v1/subscriptions/local/render",
		strings.NewReader(`{"format":"json-nodes","args":{"source":"skill"}}`),
	))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"content_type": "application/json"`)
	require.Contains(t, rec.Body.String(), `"kind": "subscription_render"`)
	require.Contains(t, rec.Body.String(), `"node-a"`)
}
```

Open a local `net.Listener` on `127.0.0.1:0`, build one synthetic Shadowsocks
URI targeting that listener, and post a `domain.ProbeRequest` using
`method: "tcp_connect"`. Assert the response returns
one live result plus the complete probe report. This exercises the same real
service/probe boundary as production without external network access.

Before running RED, also add table-driven cases for:

- conversion with both missing input sources and both input sources;
- conversion processor `params` as a JSON object, never base64;
- invalid probe method and unavailable backend errors;
- missing, multi-segment, and percent-encoded subscription names;
- missing subscription render format;
- GET on the three POST-only routes;
- report warnings preserved in full;
- no resource mutation after conversion, probing, or subscription rendering.

For conversion and subscription rendering, compute an expected result by
calling the corresponding service method with the same logical input and
compare `content_type`, body, report kind, warnings, and
dependency/source-ref fields. For probe, compare the HTTP result with a direct
`Service.Probe` call against an equivalent fresh local listener.

- [ ] **Step 3: Run the new tests and verify RED**

Run:

```bash
go test ./internal/entry/httpapi -run 'TestAgent(Convert|Execution|Subscription|Probe)'
```

Expected: FAIL with `404` because the three authenticated execution routes do
not exist.

- [ ] **Step 4: Add explicit HTTP wire types**

Add to `internal/entry/httpapi/types.go`:

```go
type convertRequest struct {
	FromFormat       string                 `json:"from_format"`
	ToFormat         string                 `json:"to_format"`
	Content          string                 `json:"content,omitempty"`
	Remote           *domain.RemoteInput    `json:"remote,omitempty"`
	ParseProcessors  []domain.ProcessorSpec `json:"parse_processors,omitempty"`
	RenderProcessors []domain.ProcessorSpec `json:"render_processors,omitempty"`
	Options          domain.RenderOptions   `json:"options,omitempty"`
	Meta             map[string]string      `json:"meta,omitempty"`
}

type subscriptionRenderRequest struct {
	Format string            `json:"format"`
	Args   map[string]string `json:"args,omitempty"`
}

type agentRenderResponse struct {
	ContentType string        `json:"content_type,omitempty"`
	Body        string        `json:"body"`
	Report      domain.Report `json:"report"`
}
```

`content` is a JSON string rather than Go `[]byte`; processor `params` remain
normal JSON objects decoded by `json.RawMessage`.

- [ ] **Step 5: Register and implement the three adapters**

In `Server.routes()`, replace the current explicit
`/v1/convert` method-not-allowed registration and add:

```go
s.mux.HandleFunc("POST /v1/convert", s.agentConvert)
s.mux.HandleFunc("POST /v1/probe", s.agentProbe)
```

Keep the existing `POST /v1/subscriptions/` action route and extend its action
parser/handler with `render`; do not register that pattern twice. Keep
`GET /convert` public and unchanged. Implement:

```go
func (s *Server) agentConvert(w http.ResponseWriter, r *http.Request) {
	var in convertRequest
	if !decodeJSON(w, r, &in) {
		return
	}
	result, err := s.rt.Service.Convert(r.Context(), domain.ConvertRequest{
		FromFormat:       in.FromFormat,
		ToFormat:         in.ToFormat,
		Content:          []byte(in.Content),
		Remote:           in.Remote,
		ParseProcessors:  in.ParseProcessors,
		RenderProcessors: in.RenderProcessors,
		Options:          in.Options,
		Meta:             in.Meta,
	})
	writeAgentRenderResult(w, result, err)
}

func (s *Server) agentProbe(w http.ResponseWriter, r *http.Request) {
	var in domain.ProbeRequest
	if !decodeJSON(w, r, &in) {
		return
	}
	result, err := s.rt.Service.Probe(r.Context(), in)
	writeResult(w, result, err)
}
```

Extend `subscriptionActionName` to recognize `/render`. In the render branch,
decode `subscriptionRenderRequest`, require a non-empty explicit `format`, use
`requestArgs(r, in.Args)`, call:

```go
s.rt.Service.RenderSubscription(
	r.Context(),
	name,
	in.Format,
	domain.RequestInfo{Args: requestArgs(r, in.Args)},
)
```

Update the GET subscription action guard so `/render` is treated like
`/preview` and `/traffic`: it requires POST and cannot be mistaken for a
subscription resource name. Update action-path error text to list all three
actions.

Return `agentRenderResponse` through a shared `writeAgentRenderResult`. Do not
apply the MCP-only `MaxOutputBytes` body omission policy to HTTP responses.

- [ ] **Step 6: Complete validation until all prewritten edge tests pass**

Use the existing single-segment resource-name validator and structured service
errors. Add only the minimum handler validation needed by the tests: require a
non-empty subscription render format and leave conversion/probe domain
validation in the service. Do not duplicate processor, format, or probe
capability rules in the HTTP entrypoint.

- [ ] **Step 7: Run focused HTTP execution tests**

Run:

```bash
go test ./internal/entry/httpapi -run 'TestAgent'
```

Expected: PASS.

- [ ] **Step 8: Run service, HTTP, and MCP regression tests**

Run:

```bash
go test ./internal/service ./internal/entry/httpapi ./internal/entry/mcpapi
```

Expected: PASS; public `GET /convert` remains restricted and existing MCP tools
remain unchanged.

- [ ] **Step 9: Commit the execution API**

```bash
git add internal/entry/httpapi
git commit -m "feat: expose full Agent execution over HTTP"
```

---

### Task 3: Add Authenticated HTTP Schema Routes

**Files:**
- Create: `internal/entry/httpapi/handlers_schemas.go`
- Create: `internal/entry/httpapi/schemas_test.go`
- Modify: `internal/entry/httpapi/server.go`
- Test: `internal/entry/httpapi/schemas_test.go`

**Interfaces:**
- Consumes: all exported `internal/agentcatalog` document and schema builders from Task 1.
- Produces: `GET /v1/schemas/processors`
- Produces: `GET /v1/schemas/processors/{stage}/{type}`
- Produces: `GET /v1/schemas/file-kinds/{kind}`
- Produces: `GET /v1/schemas/script-api/v1`
- Produces: `GET /v1/schemas/subscription`
- Produces: `GET /v1/schemas/file-spec`

- [ ] **Step 1: Write failing happy-path and bearer tests**

Create `internal/entry/httpapi/schemas_test.go` with table-driven requests:

```go
func TestSchemaRoutesPublishServerCatalogs(t *testing.T) {
	rt := testRuntime(t, app.Config{})
	handler := httpapi.New(rt).Handler()

	tests := []struct {
		path   string
		assert func(*testing.T, string)
	}{
		{"/v1/schemas/processors", func(t *testing.T, body string) {
			require.Contains(t, body, `"processors"`)
			require.Contains(t, body, `"rename"`)
		}},
		{"/v1/schemas/processors/nodes/rename", func(t *testing.T, body string) {
			require.Contains(t, body, `"params_schema"`)
			require.Contains(t, body, `"examples"`)
		}},
		{"/v1/schemas/file-kinds/mihomo", func(t *testing.T, body string) {
			require.Contains(t, body, `"kind": "mihomo"`)
			require.Contains(t, body, `"settings_schema"`)
		}},
		{"/v1/schemas/script-api/v1", func(t *testing.T, body string) {
			require.Contains(t, body, `"version": 1`)
			require.Contains(t, body, `"sandbox"`)
		}},
		{"/v1/schemas/subscription", func(t *testing.T, body string) {
			require.Contains(t, body, `"name"`)
			require.Contains(t, body, `"processors"`)
		}},
		{"/v1/schemas/file-spec", func(t *testing.T, body string) {
			require.Contains(t, body, `"kind"`)
			require.Contains(t, body, `"config"`)
		}},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, test.path, nil))
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			test.assert(t, rec.Body.String())
		})
	}
}

func TestSchemaRoutesUseV1BearerBoundary(t *testing.T) {
	rt := testRuntime(t, app.Config{HTTP: app.HTTPConfig{Token: "secret"}})
	handler := httpapi.New(rt).Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet, "/v1/schemas/processors", nil,
	))
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	req := httptest.NewRequest(http.MethodGet, "/v1/schemas/processors", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}
```

In the same RED phase, add `TestSchemaRoutesRejectUnknownCanonicalKeys` for
unknown stages/types/kinds/versions and extra segments. Add a parity test that
marshals shared catalog documents and compares them with HTTP bodies using
`require.JSONEq` for the processor summary, every public processor, every
public file kind, script API v1, the complete Subscription schema, and the
complete named FileSpec schema.

- [ ] **Step 2: Run route tests and verify RED**

Run:

```bash
go test ./internal/entry/httpapi -run 'TestSchemaRoutes'
```

Expected: FAIL with `404` because the schema routes are not registered.

- [ ] **Step 3: Register exact routes**

Add these patterns in `Server.routes()`:

```go
s.mux.HandleFunc("GET /v1/schemas/processors", s.listProcessorSchemas)
s.mux.HandleFunc("GET /v1/schemas/processors/{stage}/{type}", s.getProcessorSchema)
s.mux.HandleFunc("GET /v1/schemas/file-kinds/{kind}", s.getFileKindSchema)
s.mux.HandleFunc("GET /v1/schemas/script-api/v1", s.getScriptAPISchema)
s.mux.HandleFunc("GET /v1/schemas/subscription", s.getSubscriptionSchema)
s.mux.HandleFunc("GET /v1/schemas/file-spec", s.getFileSpecSchema)
```

Implement `handlers_schemas.go` using the shared catalog. Validate `stage` as
exactly `nodes` or `file`; validate `type` and `kind` with
`validateRequiredPublicResourceName`; match only current public descriptors and
file-kind capabilities:

```go
func (s *Server) listProcessorSchemas(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, agentcatalog.ProcessorSummary(
		s.rt.Service.Registry().PublicDescriptors(),
	))
}

func (s *Server) getScriptAPISchema(w http.ResponseWriter, _ *http.Request) {
	document, err := agentcatalog.ScriptAPI()
	writeResult(w, document, err)
}

func (s *Server) getSubscriptionSchema(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, agentcatalog.SubscriptionSchema())
}

func (s *Server) getFileSpecSchema(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, agentcatalog.FileSpecSchema(true))
}
```

For a missing canonical processor or file kind, return
`domain.CodeInvalidArgument` through `writeServiceError`; do not silently
return an empty document.

- [ ] **Step 4: Complete canonical lookup and malformed-path behavior**

Use exact `ServeMux` patterns plus descriptor/capability lookup. Unknown
canonical values return structured `invalid_argument`; unmatched versions,
extra segments, and wrong methods remain normal `404`/`405` router responses.
Do not add prefix catch-all handlers that would shadow Web UI or other `/v1`
routes.

- [ ] **Step 5: Run malformed-path and parity tests**

Run:

```bash
go test ./internal/entry/httpapi -run 'TestSchemaRoutes(Reject|Match)'
```

Expected: PASS with exact route behavior and JSON parity.

- [ ] **Step 6: Run HTTP and MCP suites**

Run:

```bash
go test ./internal/entry/httpapi ./internal/entry/mcpapi
```

Expected: PASS.

- [ ] **Step 7: Commit the HTTP API**

```bash
git add internal/entry/httpapi
git commit -m "feat: expose Agent schemas over HTTP"
```

---

### Task 4: Add the Bundled HTTP Transport Script

**Files:**
- Create: `skills/sandrone/scripts/sandrone-api.sh`
- Create: `internal/entry/httpapi/skill_script_test.go`
- Test: `internal/entry/httpapi/skill_script_test.go`

**Interfaces:**
- Consumes: `SANDRONE_URL`, optional `SANDRONE_TOKEN`, `curl`, and
  `METHOD PATH [BODY_FILE|-]`.
- Produces: response body on stdout and process exit `0` for HTTP `2xx`.
- Produces: response body plus a concise stderr diagnostic and non-zero exit
  for transport failure or non-`2xx`.

- [ ] **Step 1: Write failing black-box script tests**

Create `internal/entry/httpapi/skill_script_test.go`. Resolve the script from
the package directory with:

```go
func sandroneAPIScript(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../../../skills/sandrone/scripts/sandrone-api.sh")
	require.NoError(t, err)
	return path
}
```

Use `httptest.NewServer` and `exec.Command("sh", script, ...)` to test:

```go
func TestSandroneAPIScriptSendsBearerAndStdinBody(t *testing.T) {
	var gotMethod, gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cmd := exec.Command("sh", sandroneAPIScript(t), "POST", "/v1/files", "-")
	cmd.Env = append(os.Environ(),
		"SANDRONE_URL="+server.URL,
		"SANDRONE_TOKEN=secret-value",
	)
	cmd.Stdin = strings.NewReader(`{"name":"demo"}`)
	output, err := cmd.Output()
	require.NoError(t, err)
	require.JSONEq(t, `{"ok":true}`, string(output))
	require.Equal(t, "POST", gotMethod)
	require.Equal(t, "Bearer secret-value", gotAuth)
	require.JSONEq(t, `{"name":"demo"}`, gotBody)
}
```

Add cases for:

- GET without token or request body;
- a JSON body read from a named file;
- missing/relative/non-HTTP `SANDRONE_URL`;
- a path not beginning with `/`;
- unsupported method;
- HTTP `401` with a structured JSON body;
- HTTP `500` with plain text;
- connection failure;
- stderr/stdout never containing the token;
- a fake `curl` executable proving the token value is absent from curl argv;
- one HTTP request only when a POST response terminates ambiguously.

- [ ] **Step 2: Run the script tests and verify RED**

Run:

```bash
go test ./internal/entry/httpapi -run 'TestSandroneAPIScript'
```

Expected: FAIL because `skills/sandrone/scripts/sandrone-api.sh` does not exist.

- [ ] **Step 3: Implement the minimal POSIX shell wrapper**

Create the executable script with this control structure:

```sh
#!/bin/sh
set -eu

usage() {
  printf '%s\n' 'usage: sandrone-api.sh METHOD PATH [BODY_FILE|-]' >&2
  exit 64
}

[ "$#" -ge 2 ] && [ "$#" -le 3 ] || usage
method=$1
path=$2
body=${3-}

case "$method" in
  GET|POST|PUT|DELETE) ;;
  *) printf '%s\n' 'unsupported HTTP method' >&2; exit 64 ;;
esac

case "${SANDRONE_URL-}" in
  http://*|https://*) ;;
  *) printf '%s\n' 'SANDRONE_URL must be an absolute HTTP(S) URL' >&2; exit 64 ;;
esac

case "$path" in
  /*) ;;
  *) printf '%s\n' 'PATH must begin with /' >&2; exit 64 ;;
esac
```

Reject carriage returns and line feeds in `SANDRONE_URL`, `SANDRONE_TOKEN`, and
`PATH`. Strip trailing `/` from the base without changing the rest of the URL.
Use `umask 077`, `mktemp`, and a trap to remove response/header files.

When a token exists, write only this header to the mode-`0600` temporary header
file:

```text
Authorization: Bearer <token>
```

Pass it to curl as `--header @<file>` so the token value is not present in
`curl` argv. Do not use `eval`, `-L`, `--location`, `--trace`, or shell tracing.
Use `--data-binary @-` for stdin and `--data-binary @<file>` for a named body.
Capture status with `--write-out '%{http_code}'` while storing the body in a
temporary file. Print the body once. Exit non-zero for curl failure or any
status outside `200..299`; do not loop or retry.

- [ ] **Step 4: Mark the script executable**

Run:

```bash
chmod 0755 skills/sandrone/scripts/sandrone-api.sh
```

- [ ] **Step 5: Run script tests**

Run:

```bash
go test ./internal/entry/httpapi -run 'TestSandroneAPIScript'
```

Expected: PASS, including token-argv and single-request assertions.

- [ ] **Step 6: Run shell syntax validation**

Run:

```bash
sh -n skills/sandrone/scripts/sandrone-api.sh
```

Expected: exit `0` with no output.

- [ ] **Step 7: Commit the script**

```bash
git add skills/sandrone/scripts/sandrone-api.sh \
  internal/entry/httpapi/skill_script_test.go
git commit -m "feat: add Skill HTTP transport script"
```

---

### Task 5: Convert the Skill to HTTP-first, MCP-optional

**Files:**
- Create: `internal/entry/httpapi/skill_contract_test.go`
- Modify: `skills/sandrone/SKILL.md`
- Modify: `skills/sandrone/agents/openai.yaml`
- Modify: `skills/sandrone/references/workflows.md`
- Modify: `skills/sandrone/references/safety.md`
- Test: `internal/entry/httpapi/skill_contract_test.go`

**Interfaces:**
- Consumes: bundled `scripts/sandrone-api.sh`, `SANDRONE_URL`,
  `SANDRONE_TOKEN`, and optional MCP tools.
- Produces: deterministic HTTP-first execution-plane selection.
- Produces: task recipes with equivalent HTTP paths and MCP tool/resource names.

- [ ] **Step 1: Write a failing Skill contract test**

Create `internal/entry/httpapi/skill_contract_test.go`:

```go
func TestSandroneSkillSupportsHTTPWithoutMandatoryMCP(t *testing.T) {
	skill := readRepoFile(t, "../../../skills/sandrone/SKILL.md")
	metadata := readRepoFile(t, "../../../skills/sandrone/agents/openai.yaml")
	workflows := readRepoFile(t, "../../../skills/sandrone/references/workflows.md")

	require.Contains(t, skill, "SANDRONE_URL")
	require.Contains(t, skill, "scripts/sandrone-api.sh")
	require.Contains(t, skill, "HTTP")
	require.Contains(t, skill, "MCP")
	require.NotContains(t, skill, "Use Sandrone MCP as the only execution plane")

	require.NotContains(t, metadata, `type: "mcp"`)
	httpIndex := strings.Index(skill, "HTTP script")
	mcpIndex := strings.Index(skill, "MCP")
	require.NotEqual(t, -1, httpIndex)
	require.NotEqual(t, -1, mcpIndex)
	require.Less(t, httpIndex, mcpIndex)

	for _, endpoint := range []string{
		"/v1/convert",
		"/v1/probe",
		"/v1/subscriptions/{name}/render",
		"/v1/schemas/processors",
		"/v1/schemas/file-kinds/{kind}",
		"/v1/schemas/script-api/v1",
		"/v1/schemas/subscription",
		"/v1/schemas/file-spec",
	} {
		require.Contains(t, workflows, endpoint)
	}
}
```

Implement `readRepoFile` with `os.ReadFile` and `require.NoError`.

- [ ] **Step 2: Run the contract test and verify RED**

Run:

```bash
go test ./internal/entry/httpapi -run 'TestSandroneSkill'
```

Expected: FAIL because the current Skill requires MCP and has no HTTP script
workflow.

- [ ] **Step 3: Update frontmatter and UI metadata**

Use frontmatter equivalent to:

```yaml
---
name: sandrone
description: Use when an Agent must operate Sandrone to convert proxy or subscription data, inspect or manage subscriptions and FileSpecs, render client configurations, probe nodes, author processors or sandbox scripts, or explain Sandrone reports.
compatibility: Requires either shell access with curl, SANDRONE_URL, and the bundled HTTP script, or a connected Sandrone MCP server.
---
```

Update `agents/openai.yaml`:

```yaml
interface:
  display_name: "Sandrone"
  short_description: "Operate Sandrone resources and conversion workflows safely"
  default_prompt: "Use $sandrone to inspect my Sandrone resources and complete the requested subscription or FileSpec task safely."
```

Remove the entire mandatory `dependencies.tools` MCP block. Do not add URL or
token values to tracked files.

- [ ] **Step 4: Replace the MCP-only startup workflow**

Make `SKILL.md` select an execution plane:

1. If shell access, `curl`, the bundled script, and `SANDRONE_URL` are
   available, use the HTTP script.
2. Otherwise, if Sandrone MCP tools are connected, use MCP.
3. Otherwise, report both supported setup choices.

For HTTP, call `/healthz`, `/version`, `/v1/inspect`, then the narrowest schema
endpoint. For MCP, retain capability and resource discovery. State that the
chosen plane remains fixed for a mutation until the exact resource is reread
after any ambiguous failure.

- [ ] **Step 5: Rewrite workflows as transport mappings**

For each existing recipe, give the HTTP operation first and the MCP operation
second. Include at least:

| Operation | HTTP | MCP |
| --- | --- | --- |
| Inspect | `GET /v1/inspect` | `sandrone_inspect_capabilities` |
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

Use the authenticated POST endpoints for complete conversion, probing, and
subscription rendering. Keep public `GET /convert` documented as a separate,
processor-free convenience route rather than substituting it for
`POST /v1/convert`.

- [ ] **Step 6: Make safety guidance transport-neutral**

Replace MCP-only bypass language with:

- shell execution grants transport access, not mutation authorization;
- an explicit user request for an exact resource authorizes that mutation;
- both HTTP and MCP definitions/results may contain secrets;
- HTTP bearer configuration comes from environment only;
- ambiguous mutation failures require rereading before retry;
- processor scripts remain embedded ECMAScript without arbitrary host access.

- [ ] **Step 7: Run Skill contract and script tests**

Run:

```bash
go test ./internal/entry/httpapi \
  -run 'TestSandrone(Skill|APIScript)'
```

Expected: PASS.

- [ ] **Step 8: Validate the Skill package**

Run:

```bash
uvx --from 'git+https://github.com/agentskills/agentskills#subdirectory=skills-ref' \
  skills-ref validate skills/sandrone
npx -y skills add . --list
```

Expected: `Valid skill: skills/sandrone` and one discovered Skill named
`sandrone`.

- [ ] **Step 9: Commit the Skill update**

```bash
git add skills/sandrone internal/entry/httpapi/skill_contract_test.go
git commit -m "feat: make Sandrone Skill HTTP-first"
```

---

### Task 6: Publish the HTTP Schema and Dual-plane Documentation

**Files:**
- Create: `docs/reference/http-api/schemas.md`
- Create: `docs/reference/http-api/probing.md`
- Modify: `docs/reference/http-api/README.md`
- Modify: `docs/reference/http-api/conversion.md`
- Modify: `docs/reference/http-api/subscriptions.md`
- Modify: `docs/reference/mcp.md`
- Modify: `docs/README.md`

**Interfaces:**
- Consumes: execution routes from Task 2, schema routes from Task 3, and the
  environment/script contract from Task 4.
- Produces: canonical public documentation without schema duplication.

- [ ] **Step 1: Write a failing documentation contract check**

Extend `TestSandroneSkillSupportsHTTPWithoutMandatoryMCP` or add
`TestSandroneHTTPShapeDocumentationIsLinked` to assert:

```go
schemas := readRepoFile(t, "../../../docs/reference/http-api/schemas.md")
conversion := readRepoFile(t, "../../../docs/reference/http-api/conversion.md")
probing := readRepoFile(t, "../../../docs/reference/http-api/probing.md")
subscriptions := readRepoFile(t, "../../../docs/reference/http-api/subscriptions.md")
httpIndex := readRepoFile(t, "../../../docs/reference/http-api/README.md")
mcpReference := readRepoFile(t, "../../../docs/reference/mcp.md")

for _, route := range []string{
	"GET /v1/schemas/processors",
	"GET /v1/schemas/processors/{stage}/{type}",
	"GET /v1/schemas/file-kinds/{kind}",
	"GET /v1/schemas/script-api/v1",
	"GET /v1/schemas/subscription",
	"GET /v1/schemas/file-spec",
} {
	require.Contains(t, schemas, route)
}
require.Contains(t, conversion, "POST /v1/convert")
require.Contains(t, probing, "POST /v1/probe")
require.Contains(t, subscriptions, "POST /v1/subscriptions/{name}/render")
require.Contains(t, httpIndex, "schemas.md")
require.Contains(t, httpIndex, "probing.md")
require.Contains(t, mcpReference, "SANDRONE_URL")
require.Contains(t, mcpReference, "可选")
```

- [ ] **Step 2: Run the documentation contract test and verify RED**

Run:

```bash
go test ./internal/entry/httpapi -run 'TestSandroneHTTPShapeDocumentationIsLinked'
```

Expected: FAIL because the schema/probe pages and dual-plane HTTP/MCP copy do
not exist yet.

- [ ] **Step 3: Write the canonical schema HTTP reference**

Document in `docs/reference/http-api/schemas.md`:

- bearer authentication inherited from `/v1/*`;
- exact route table and response document purpose;
- complete Subscription and named FileSpec write schemas;
- canonical `nodes`/`file` stages and file kinds;
- structured errors for unknown keys;
- examples using `SANDRONE_URL` and `SANDRONE_TOKEN`;
- links to Processors, FileSpec, and scripting references for semantics;
- statement that MCP and HTTP serialize the same owner-maintained catalog.

Show only synthetic names and tokens.

- [ ] **Step 4: Document full conversion, probing, and subscription rendering**

In `conversion.md`, distinguish public `GET /convert` from authenticated
`POST /v1/convert` and document the string `content`, `remote`,
parse/render-processors, options, metadata, and full report response.

In new `probing.md`, document `POST /v1/probe`, the `NodeInput`, canonical
methods, target options, bounded execution fields, result/report shape,
controlled network effects, and synthetic curl example.

In `subscriptions.md`, document
`POST /v1/subscriptions/{name}/render`, required `format`, optional string
`args`, non-persistence of the body, and full report response.

- [ ] **Step 5: Update navigation and MCP Skill wording**

Link `schemas.md` and `probing.md` from the HTTP reference index. In MCP
documentation, replace the statement that the Skill requires MCP with:

- HTTP script is preferred when configured;
- MCP remains available for clients without script execution;
- HTTP uses `SANDRONE_URL` and optional `SANDRONE_TOKEN`;
- MCP connection settings remain in the MCP client.

Keep MCP transport and tool contracts otherwise unchanged. Update
`docs/README.md` only through its existing HTTP API navigation entry; do not
duplicate the route table there.

- [ ] **Step 6: Run documentation contract and diff checks**

Run:

```bash
go test ./internal/entry/httpapi -run 'TestSandroneHTTPShapeDocumentationIsLinked'
git diff --check
```

Expected: PASS with no whitespace errors.

- [ ] **Step 7: Commit documentation**

```bash
git add docs/README.md docs/reference/http-api/README.md \
  docs/reference/http-api/conversion.md docs/reference/http-api/probing.md \
  docs/reference/http-api/schemas.md docs/reference/http-api/subscriptions.md \
  docs/reference/mcp.md
git commit -m "docs: publish Skill HTTP setup"
```

---

### Task 7: Final Verification and Cleanup

**Files:**
- Modify only files required by failures found during this task.

**Interfaces:**
- Consumes: all prior task outputs.
- Produces: a clean, validated repository ready for review.

- [ ] **Step 1: Format Go and validate shell syntax**

Run:

```bash
gofmt -w internal/agentcatalog internal/entry/httpapi internal/entry/mcpapi
sh -n skills/sandrone/scripts/sandrone-api.sh
```

Expected: no shell syntax error.

- [ ] **Step 2: Run focused tests**

Run:

```bash
go test ./internal/agentcatalog ./internal/entry/httpapi ./internal/entry/mcpapi
```

Expected: PASS.

- [ ] **Step 3: Run Skill validators**

Run:

```bash
uvx --from 'git+https://github.com/agentskills/agentskills#subdirectory=skills-ref' \
  skills-ref validate skills/sandrone
npx -y skills add . --list
```

Expected: valid Skill and `sandrone` discovery.

- [ ] **Step 4: Run the repository gate**

Run:

```bash
make check
```

Expected: formatting, vet, Go tests, and build all PASS.

- [ ] **Step 5: Scan for drift and sensitive values**

Run:

```bash
rg -n 'Use Sandrone MCP as the only execution plane|Skill still requires|connected Sandrone MCP server' \
  skills docs README.md
rg -n 'SANDRONE_TOKEN=.*[^\"<]|Authorization: Bearer (?!<|\\$|replace-|change-|sandrone)' \
  skills docs --pcre2
git diff --check
git status --short
```

Expected:

- no obsolete MCP-only Skill statements;
- no real token values;
- no whitespace errors;
- only intentional task files are modified.

- [ ] **Step 6: Review final history and behavior**

Run:

```bash
git log --oneline -8
git diff HEAD~6 --stat
```

Confirm the history contains separate catalog, HTTP API, script, Skill, and
documentation commits, and that no unrelated files entered the diff.

- [ ] **Step 7: Commit verification-only fixes if necessary**

If and only if Step 1–6 required source changes:

Stage only the exact paths reported by `git status --short`, restricted to
`internal/agentcatalog`, `internal/entry/httpapi`, `internal/entry/mcpapi`,
`skills/sandrone`, `docs/reference`, or `docs/README.md`, then run:

```bash
git commit -m "fix: complete Skill HTTP integration"
```

If no source changes were required, do not create an empty commit.
