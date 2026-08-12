# Community Configuration Presets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add managed, editable community configuration presets for Mihomo, sing-box, and Shadowrocket with explicit risks, safe dependency/conflict planning, new-file defaults, fixed target revision display, and Tailscale coexistence/native modes.

**Architecture:** Extend each existing Web `FileDriverDefinition` with driver-owned preset descriptors and keep all planning as pure TypeScript. Presets still materialize only existing `merge` or `script` file-stage processors; array element mutations and ordered rule insertion use editable inline scripts, while safe object/INI changes use override merges. No backend business model or processor type is added.

**Tech Stack:** TypeScript, React 19, React Router, MUI, Vitest, Testing Library, Vite raw assets, Go service integration tests, existing Sandrone YAML/JSON/INI/script APIs.

## Global Constraints

- Mihomo output is fixed to v1.19.25, sing-box output to v1.13.14, and Shadowrocket output to source revision `5f1916b5897fc59fb7172aca59ae52050a3532fe`.
- Read target revisions from the existing authenticated `GET /v1/inspect` capability payload; do not persist them in `FileSpec` and do not copy hard-coded revisions into the Web preset catalog.
- Add no `NodeIR` field, backend business model, processor type, final semantic auditor, migration, or version selector.
- Existing files and groups are never rewritten automatically. Defaults apply only when creating a new file or new group.
- A preset is a copied, editable processor. Exact managed recognition may remove only unchanged recognized processors; edited or unknown processors are user-owned and must remain untouched.
- Preserve all non-conflicting processor relative order. Dependency addition and conflict removal happen in the same explicit add action and the UI lists dependencies, risks, and removed conflicts.
- Rule order is user/service-specific rules, managed scenario rules in processor order, generic private/region rules, then `MATCH`/`FINAL`. Never rewrite the final policy.
- Use `merge` only when it can update a map/INI field without replacing a user array. Use managed inline scripts for sing-box TUN/DNS/endpoint arrays and ordered rules.
- sing-box v1.13.14 Tailscale route rules use `preferred_by`; MagicDNS uses the official pre-1.14 `ip_accept_any` DNS-rule form and `accept_default_resolvers: false`.
- Never emit, accept, store, or request Tailscale `auth-key`/`auth_key`, `control-url`/`control_url`, login URLs, QR codes, Headscale fields, or an automatic Exit Node.
- Prefer Vitest node tests for planning and codec logic. Keep one focused React interaction test for atomic conflict replacement and its visible notice; do not add broad E2E coverage.
- Preserve opaque processors byte-for-byte and keep existing processor declaration order.
- Run narrow tests after every task. Before final handoff run Web `pnpm typecheck`, `pnpm lint`, relevant Vitest tests, and repo-root `make check`.

---

### Task 1: Add the pure managed-preset contract and planner

**Files:**
- Create: `web/app/features/files/drivers/core/processor-presets.ts`
- Create: `web/app/features/files/drivers/core/processor-presets.test.ts`
- Modify: `web/app/features/files/drivers/core/file-driver.ts`
- Modify: `web/app/features/files/drivers/core/registry.ts`
- Modify: `web/app/features/files/drivers/core/registry.test.ts`
- Modify: `web/app/features/files/drivers/mihomo/driver.ts`
- Modify: `web/app/features/files/drivers/sing-box/driver.ts`
- Modify: `web/app/features/files/drivers/shadowrocket/driver.ts`
- Modify: `web/app/features/files/drivers/static/driver.ts`

**Interfaces:**
- Produces `FileProcessorPreset`, `FileProcessorPresetCategory`, `FileProcessorPresetPlan`, `planFileProcessorPresetAddition`, `recognizedFileProcessorPresetID`, and `filterForeignManagedProcessors`.
- Later tasks attach `readonly FileProcessorPreset[]` at `driver.processors.presets` and consume plans without rebuilding existing drafts.
- A preset builds exactly one `ProcessorDetail`; compound behavior belongs in one editable script or in explicit dependency presets.

- [ ] **Step 1: Write planner tests that fail before the contract exists**

Cover this exact matrix in node Vitest:

```ts
it("adds dependencies once in topological order", () => {
  const plan = planFileProcessorPresetAddition(catalog, "mptcp", []);
  expect(plan.addedPresetIDs).toEqual(["tun", "linux-acceleration", "mptcp"]);
  expect(plan.additions.map((item) => item.name)).toEqual(["TUN", "Linux", "MPTCP"]);
});

it("atomically removes recognized conflicts and preserves every other relative position", () => {
  const current = [custom("before"), built("tailscale-external"), custom("middle"), built("stun"), custom("after")];
  const plan = planFileProcessorPresetAddition(catalog, "tailscale-native", current);
  expect(plan.removeIndices).toEqual([1, 3]);
  expect(plan.removedPresetIDs).toEqual(["tailscale-external", "stun"]);
  expect(applyPlan(current, plan).filter(isCustom).map(nameOf)).toEqual(["before", "middle", "after"]);
});

it("never removes an edited processor that no longer matches exactly", () => {
  const edited = { ...built("stun"), params: { mode: "yaml_override", content: "edited" } };
  const plan = planFileProcessorPresetAddition(catalog, "tailscale-native", [edited]);
  expect(plan.removeIndices).toEqual([]);
});
```

Also test duplicate requests are no-ops, missing IDs throw, missing dependency/conflict IDs throw at registry construction, dependency cycles throw, self-dependencies throw, and foreign managed processors are filtered while unknown processors remain byte-for-byte.

- [ ] **Step 2: Run the focused test and verify the failure**

Run:

```bash
cd web
pnpm exec vitest run app/features/files/drivers/core/processor-presets.test.ts app/features/files/drivers/core/registry.test.ts
```

Expected: FAIL because the preset module and `processors.presets` do not exist.

- [ ] **Step 3: Implement the frozen descriptor and deterministic planner**

Use these exact public shapes:

```ts
export type FileProcessorPresetCategory = "privacy" | "network" | "platform" | "tailscale";

export interface FileProcessorPreset {
  readonly id: string;
  readonly category: FileProcessorPresetCategory;
  readonly labelKey: Parameters<Translator>[0];
  readonly descriptionKey: Parameters<Translator>[0];
  readonly riskKey?: Parameters<Translator>[0];
  readonly defaultOn: boolean;
  readonly dependencies: readonly string[];
  readonly conflicts: readonly string[];
  build(): ProcessorDetail;
  recognize(processor: Pick<ProcessorDetail, "type" | "params">): boolean;
}

export interface FileProcessorPresetPlan {
  readonly additions: readonly ProcessorDetail[];
  readonly addedPresetIDs: readonly string[];
  readonly dependencyPresetIDs: readonly string[];
  readonly removeIndices: readonly number[];
  readonly removedPresetIDs: readonly string[];
  readonly requestedPresetID: string;
}

export function recognizedFileProcessorPresetID(
  catalog: readonly FileProcessorPreset[],
  processor: Pick<ProcessorDetail, "type" | "params">,
): string | null;

export function filterForeignManagedProcessors(
  targetCatalog: readonly FileProcessorPreset[],
  allCatalogs: readonly (readonly FileProcessorPreset[])[],
  current: readonly ProcessorDetail[],
): ProcessorDetail[];
```

Planner rules:

1. Validate non-empty unique IDs, existing dependency/conflict IDs, no self-edge, and acyclic dependencies.
2. Resolve dependencies depth-first in declaration order, then requested preset.
3. Treat a preset as present only when its `recognize` function matches a current processor exactly.
4. Remove current processors recognized by any conflict of the dependency closure or requested preset; de-duplicate removed preset IDs in first-seen processor order.
5. Build only missing dependency/request processors after conflict removal.
6. Return indices/additions; do not mutate or clone current processors.
7. `filterForeignManagedProcessors(target, all, current)` drops processors exactly recognized by another driver but preserves target-managed and unrecognized values by identity.

Extend `FileDriverDefinition.processors` with mandatory `presets: readonly FileProcessorPreset[]`. Freeze descriptor objects plus `dependencies` and `conflicts` arrays in `createFileDriverRegistry`, and validate each driver's catalog during registry construction. In this task add `presets: []` to all four current definitions so the branch typechecks; Task 2 replaces those temporary empty arrays for Mihomo and sing-box while keeping existing adapter behavior until that migration is complete.

- [ ] **Step 4: Run focused tests and typecheck**

```bash
cd web
pnpm exec vitest run app/features/files/drivers/core/processor-presets.test.ts app/features/files/drivers/core/registry.test.ts
pnpm typecheck
```

Expected: all commands pass.

- [ ] **Step 5: Commit Task 1**

```bash
git add web/app/features/files/drivers/core
git commit -m "feat(web): add managed file preset planner"
```

---

### Task 2: Integrate grouped presets into the existing processor editor

**Files:**
- Modify: `web/app/features/files/processors/processor-builder.tsx`
- Modify: `web/app/features/files/processors/processor-builder.test.tsx`
- Modify: `web/app/shared/processors/components/processor-editor-list.tsx`
- Modify: `web/app/shared/ui/form-fields.tsx`
- Modify: `web/app/shared/ui/shared-ui.test.tsx`
- Modify: `web/app/features/files/drivers/mihomo/processor-presets.ts`
- Modify: `web/app/features/files/drivers/mihomo/processor-adapter.ts` (delete after migration)
- Modify: `web/app/features/files/drivers/mihomo/driver.ts`
- Modify: `web/app/features/files/drivers/sing-box/processor-presets.ts`
- Modify: `web/app/features/files/drivers/sing-box/driver.ts`
- Modify: `web/app/features/files/drivers/shadowrocket/driver.ts`
- Modify: `web/app/features/files/drivers/static/driver.ts`
- Modify: `web/app/shared/i18n/translations/processors.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/processors.en-US.ts`

**Interfaces:**
- Consumes Task 1's descriptor and plan APIs.
- Produces grouped select options and the only React-level preset notice path used by all later presets.
- Existing `script`, `merge`, and GitHub rule-source rewrite shortcuts remain available.

- [ ] **Step 1: Add failing pure/editor tests for integration behavior**

Update the existing processor-builder test to assert:

```ts
expect(screen.getByRole("option", { name: "Tailscale coexistence" })).toBeInTheDocument();
expect(screen.getByText("Tailscale")).toBeInTheDocument();
```

Keep exactly one focused preset-notice interaction test. In this task select the existing Tailnet Share preset and assert its TUN/Tailscale dependencies are added in one update and listed in the visible alert. Task 10 replaces/extends this same test case with the final STUN/Tailscale conflict assertion; do not add a second React preset test or duplicate it per driver.

Add a shared `SelectField` test proving group headers are rendered in option order and ungrouped call sites remain unchanged.

- [ ] **Step 2: Run the focused UI tests and verify failure**

```bash
cd web
pnpm exec vitest run app/features/files/processors/processor-builder.test.tsx app/shared/ui/shared-ui.test.tsx
```

Expected: FAIL because select grouping and planner notices are absent.

- [ ] **Step 3: Add optional grouped select rendering**

Extend the existing type without breaking other callers:

```ts
export type SelectOption = {
  value: string;
  label: string;
  group?: string;
};
```

In `SelectField`, emit one MUI `ListSubheader` before the first option of each changed non-empty group. Keep `renderValue` based only on the selected option label. Do not synthesize disabled fake options.

- [ ] **Step 4: Replace the Mihomo adapter callbacks with driver-owned descriptors**

Create descriptors for the existing unchanged shortcuts `sniffer`, `tun`, `fake-ip-compat`, `tailscale-external` (recognizing the old `tailscale` marker), and `tailnet-share`. Preserve current dependency behavior: Tailscale external depends on TUN; Tailnet share depends on TUN and external coexistence. Recognition must compare type, mode, and exact preset content; changing content makes it custom even if its comment marker remains.

Represent sing-box's existing `Sniff & DNS Hijack` as a descriptor with exact JSON-content recognition. Static and Shadowrocket start with empty catalogs. Remove `FileProcessorAdapter` and the Mihomo-specific cross-kind filter after `processor-builder.tsx` uses Task 1's generic foreign-managed filter.

- [ ] **Step 5: Apply plans inside `FileProcessorBuilder` without rebuilding current drafts**

Inside the `addProcessorDrafts` callback:

1. Resolve a preset option value as `file-preset:<preset-id>`.
2. Serialize current drafts only for recognition/planning.
3. Remove drafts by returned indices, retaining every surviving draft object and ID.
4. Convert only `plan.additions` to new drafts with fresh IDs and append them.
5. Save a local `PresetNotice` containing translated description/risk plus dependency and removed labels.
6. Render one `Alert` below the editor list; warning severity when `riskKey` exists, info otherwise.

Group preset options in this order: Privacy, Network Compatibility, Platform, Tailscale. Put the GitHub rule-source rewrite shortcut in Network Compatibility. Native Tailscale will be declared before external coexistence when Task 9/10 add it.

Add translation keys for the four category names and these notice prefixes: description, added dependencies, removed conflicts. Use natural Chinese and English; do not expose internal IDs.

- [ ] **Step 6: Run tests, typecheck, and lint**

```bash
cd web
pnpm exec vitest run app/features/files/processors/processor-builder.test.tsx app/features/files/drivers/mihomo/processor-presets.test.ts app/features/files/drivers/sing-box/processor-presets.test.ts app/shared/ui/shared-ui.test.tsx
pnpm typecheck
pnpm lint
```

Expected: all commands pass and existing opaque-processor/order tests remain green.

- [ ] **Step 7: Commit Task 2**

```bash
git add web/app/features/files web/app/shared/processors web/app/shared/ui web/app/shared/i18n
git commit -m "feat(web): integrate managed file presets"
```

---

### Task 3: Apply approved new-file bases and group semantics

**Files:**
- Modify: `web/app/features/files/drivers/mihomo/base.ts`
- Modify: `web/app/features/files/drivers/mihomo/configuration.ts`
- Modify: `web/app/features/files/drivers/sing-box/configuration.ts`
- Modify: `web/app/features/files/drivers/sing-box/fields.tsx`
- Modify: `web/app/features/files/drivers/shadowrocket/base.ts`
- Modify: `web/app/features/files/config/model/editor-model.ts`
- Modify: `web/app/features/files/drivers/driver-bases.contract.test.ts`
- Modify: `web/app/features/files/drivers/driver-registry.integration.test.ts`
- Modify: `web/app/features/files/config/components/group-editor.test.tsx`
- Modify: `web/app/shared/i18n/translations/files.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/files.en-US.ts`

**Interfaces:**
- Adds optional `interruptExistingConnections?: boolean` to `GroupDraft`.
- Mihomo reuses existing `healthCheckTolerance?: number`; no new Mihomo tolerance UI field is added.
- All changes affect only newly created bases/groups or preserve explicitly present values from existing groups.

- [ ] **Step 1: Write failing base and group codec tests**

Assert exact new Mihomo base fields:

```ts
expect(parsed).toMatchObject({
  "geo-auto-update": true,
  "geo-update-interval": 24,
  "allow-lan": true,
  "lan-allowed-ips": ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"],
});
```

Assert Shadowrocket contains:

```text
dns-direct-fallback-proxy = false
close-if-proxy-chain-missing = true
udp-policy-not-supported-behaviour = REJECT
block-quic = all-proxy
ipv6 = true
prefer-ipv6 = false
```

For Mihomo groups, assert a newly generated or newly transitioned `url-test` serializes `tolerance: 50`, an imported url-test without tolerance remains without it, and an imported explicit tolerance remains unchanged.

For sing-box selector/urltest groups, assert imported `interrupt_exist_connections: true` and explicit `false` both round-trip, while newly created false/unchecked groups omit the key. Assert both selector and urltest show the switch; only checked urltest displays the interruption warning.

- [ ] **Step 2: Run focused tests and verify failure**

```bash
cd web
pnpm exec vitest run app/features/files/drivers/driver-bases.contract.test.ts app/features/files/drivers/driver-registry.integration.test.ts app/features/files/config/components/group-editor.test.tsx
```

Expected: FAIL on the new fields and group semantics.

- [ ] **Step 3: Implement bases and Mihomo tolerance without backfill**

Add `geo-auto-update: true` and `geo-update-interval: 24` to the default Mihomo YAML. Keep Mihomo's built-in Geo sources and assert the base does not add `geox-url`. In the Mihomo group adapter:

- project native `tolerance` into `healthCheckTolerance` only when present;
- remove it from opaque adapter state;
- serialize it only when `healthCheckTolerance !== undefined`;
- set 50 when a group is newly created/transitioned to `url-test` and in default/template url-test groups;
- never set it while projecting an existing group that omitted it;
- do not render a Mihomo tolerance input.

Change only the two approved Shadowrocket assignments and uncomment/add the exact `close-if-proxy-chain-missing = true` assignment.

- [ ] **Step 4: Implement the sing-box tri-state preservation detail**

Add `interruptExistingConnections?: boolean` to `GroupDraft`. Store whether the native key was originally present in sing-box `adapterState` under a private adapter-only sentinel that is removed before serialization. Serialize:

```ts
if (draft.interruptExistingConnections === true || interruptWasExplicit(draft.adapterState)) {
  value.interrupt_exist_connections = Boolean(draft.interruptExistingConnections);
}
```

New groups have no sentinel and omit unchecked false. Existing explicit false keeps the field. Add one checkbox to `SingBoxGroupFields` for selector and urltest; when checked on urltest show the fixed warning that automatic switching interrupts existing connections.

- [ ] **Step 5: Run focused tests, typecheck, and lint**

```bash
cd web
pnpm exec vitest run app/features/files/drivers/driver-bases.contract.test.ts app/features/files/drivers/driver-registry.integration.test.ts app/features/files/config/components/group-editor.test.tsx
pnpm typecheck
pnpm lint
```

Expected: all commands pass.

- [ ] **Step 6: Commit Task 3**

```bash
git add web/app/features/files web/app/shared/i18n
git commit -m "feat(web): update client configuration defaults"
```

---

### Task 4: Display fixed target revisions from `/v1/inspect`

**Files:**
- Modify: `web/app/shared/api/client.ts`
- Create: `web/app/features/files/model/renderer-revision.ts`
- Create: `web/app/features/files/model/renderer-revision.test.ts`
- Modify: `web/app/features/files/drivers/core/file-driver.ts`
- Modify: `web/app/features/files/drivers/core/registry.ts`
- Modify: `web/app/features/files/drivers/{mihomo,sing-box,shadowrocket,static}/driver.ts`
- Modify: `web/app/routes/files.$name.preview.tsx`
- Modify: `web/app/features/files/pages/file-preview-page.tsx`
- Modify: `web/app/features/files/pages/file-pages.test.tsx`
- Modify: `web/app/shared/i18n/translations/files.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/files.en-US.ts`

**Interfaces:**
- Adds optional `targetRendererFormat?: string` to a driver; values are `mihomo-proxies`, `sing-box-outbounds`, and `shadowrocket-proxies`.
- Adds `ApiClient.inspect(): Promise<unknown>`.
- Produces `rendererRevisionFromInspect(value, rendererFormat): string | undefined`.

- [ ] **Step 1: Write the failing pure decoder test**

Use a realistic nested payload where `response.capabilities.capabilities` contains render entries and `fields[*].source_ref.revision`. Assert exact formats return `v1.19.25`, `v1.13.14`, and the Shadowrocket SHA; parse entries, lossy/raw-only entries, blank revisions, and conflicting multiple non-empty revisions are ignored/return undefined rather than guessed.

- [ ] **Step 2: Run decoder/page tests and verify failure**

```bash
cd web
pnpm exec vitest run app/features/files/model/renderer-revision.test.ts app/features/files/pages/file-pages.test.tsx
```

Expected: FAIL because inspect decoding and the page prop do not exist.

- [ ] **Step 3: Implement inspect loading and defensive revision selection**

Add `inspect()` through the existing deduped authenticated request path. The decoder must:

1. read only render capability entries matching the requested format;
2. collect trimmed revisions from `fields`, `lossy`, and `raw_only` `source_ref` values;
3. return the sole unique revision;
4. return undefined on missing or multiple revisions.

Add/freeze each typed driver's target renderer format; static has none. In the preview route, load inspect once for the current item, silently omit the label if inspection fails (the file preview itself remains usable), and pass the decoded revision to the page. Render `Target core: <revision>`/`目标核心：<revision>` above warnings/final content. Do not store it in the form or file response.

- [ ] **Step 4: Run focused tests, API client tests, and typecheck**

```bash
cd web
pnpm exec vitest run app/features/files/model/renderer-revision.test.ts app/features/files/pages/file-pages.test.tsx app/shared/api/client.test.ts
pnpm typecheck
```

Expected: all commands pass.

- [ ] **Step 5: Commit Task 4**

```bash
git add web/app/shared/api web/app/features/files web/app/routes web/app/shared/i18n
git commit -m "feat(web): show file target core revisions"
```

---

### Task 5: Add safe ordered-rule scripts and default NTP presets

**Files:**
- Create: `web/app/features/files/processors/scripts/insert-mihomo-rules.js`
- Create: `web/app/features/files/processors/scripts/insert-sing-box-rules.js`
- Create: `web/app/features/files/processors/scripts/insert-shadowrocket-rules.js`
- Create: `web/app/features/files/processors/ordered-rule-preset.ts`
- Create: `web/app/features/files/processors/ordered-rule-preset.test.ts`
- Modify: `web/app/features/files/drivers/mihomo/processor-presets.ts`
- Modify: `web/app/features/files/drivers/sing-box/processor-presets.ts`
- Create: `web/app/features/files/drivers/shadowrocket/processor-presets.ts`
- Modify: `web/app/features/files/drivers/{mihomo,sing-box,shadowrocket}/driver.ts`
- Modify: `web/app/features/files/drivers/{mihomo,sing-box}/processor-presets.test.ts`
- Create: `web/app/features/files/drivers/shadowrocket/processor-presets.test.ts`
- Create: `internal/service/file_community_preset_test.go`
- Modify: `web/app/shared/i18n/translations/processors.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/processors.en-US.ts`

**Interfaces:**
- Produces `orderedRuleProcessorPreset({ id, kind, name, rules })` and exact recognition of its source plus `preset_id`/`rules_json` args.
- Each driver gains managed preset ID `ntp-direct`, `defaultOn: true`.
- Later STUN, QUIC, and Tailscale rules reuse the same scripts/factory.

- [ ] **Step 1: Write failing factory/default tests**

Assert each NTP processor is a file-stage inline script with these exact rules:

```ts
const NTP = {
  mihomo: ["AND,((NETWORK,UDP),(DST-PORT,123)),DIRECT"],
  "sing-box": [{ network: "udp", port: 123, outbound: "direct" }],
  shadowrocket: ["AND,((PROTOCOL,UDP),(DST-PORT,123)),DIRECT"],
};
```

Assert default chains are exactly Mihomo `Sniffer, TUN, Traditional NTP Direct`, sing-box `Sniff & DNS Hijack, Traditional NTP Direct`, and Shadowrocket `Traditional NTP Direct`. Editing source content or `rules_json` must make recognition return false.

- [ ] **Step 2: Run focused tests and verify failure**

```bash
cd web
pnpm exec vitest run app/features/files/processors/ordered-rule-preset.test.ts app/features/files/drivers/mihomo/processor-presets.test.ts app/features/files/drivers/sing-box/processor-presets.test.ts app/features/files/drivers/shadowrocket/processor-presets.test.ts
```

Expected: FAIL because ordered scripts and NTP descriptors do not exist.

- [ ] **Step 3: Implement three strict insertion scripts**

All scripts receive string args `preset_id` and `rules_json`, parse with injected APIs, skip an exact duplicate, and throw `Sandrone preset <id> cannot find a safe <kind> rule anchor` if no safe anchor exists.

- Mihomo parses YAML, requires `rules` array, and inserts before the first string beginning `RULE-SET,private,`, then `GEOIP,CN,`, then `MATCH,` as fallback.
- sing-box parses JSON, requires `route.rules` array, and inserts before the first object with `rule_set` containing `private`, then `ip_is_private: true`, then a match-all final object whose only routing selector is `outbound`.
- Shadowrocket parses ordered INI, finds the physical `Rule` section containing the anchor, and inserts before `IP-CIDR,10.0.0.0/8,`, then `GEOIP,CN,`, then `FINAL,`.

Scripts must preserve user/service-specific rules before the anchor, preserve final lines/objects byte-semantically, and set `input.file.content` only after successful parse/mutation/stringify.

Import scripts with Vite `?raw`. Build params exactly as:

```ts
{
  source: { type: "inline", content: scriptForKind(kind) },
  args: { preset_id: id, rules_json: JSON.stringify(rules) },
}
```

- [ ] **Step 4: Add new-file NTP defaults and warnings**

Add the visible warning that UDP destination port 123 bypasses the proxy and exposes the direct egress, and state that the preset is enabled for new files. Existing edit-mode processor lists remain unchanged because `FileFormFields` already calls defaults only in create mode.

- [ ] **Step 5: Exercise the exact raw script assets through the Go service**

In `file_community_preset_test.go`, locate raw JS assets via `runtime.Caller`, construct `domain.FileSpec` values with a user rule, generic rule, and final rule, and run `service.New().GetFile`. For all three kinds assert:

1. user rule remains before NTP;
2. NTP matches UDP plus port 123 only;
3. NTP is before the generic rule;
4. original `MATCH`/`FINAL` is unchanged;
5. a source with no safe anchor returns `script_runtime` and no consumable partial result.

- [ ] **Step 6: Run narrow Web and Go tests**

```bash
cd web
pnpm exec vitest run app/features/files/processors/ordered-rule-preset.test.ts app/features/files/drivers/mihomo/processor-presets.test.ts app/features/files/drivers/sing-box/processor-presets.test.ts app/features/files/drivers/shadowrocket/processor-presets.test.ts
cd ..
go test ./internal/service -run 'CommunityPreset|OrderedNTP'
```

Expected: all commands pass.

- [ ] **Step 7: Commit Task 5**

```bash
git add web/app/features/files web/app/shared/i18n internal/service/file_community_preset_test.go
git commit -m "feat(web): add ordered NTP file presets"
```

---

### Task 6: Add Mihomo privacy, compatibility, and platform presets

**Files:**
- Modify: `web/app/features/files/drivers/mihomo/processor-presets.ts`
- Modify: `web/app/features/files/drivers/mihomo/processor-presets.test.ts`
- Modify: `web/app/shared/i18n/translations/processors.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/processors.en-US.ts`
- Modify: `internal/service/file_community_preset_test.go`

**Interfaces:**
- Consumes the planner and ordered-rule factory.
- Adds IDs `stun-block`, `quic-fallback`, `udp-p2p-eim`, `linux-tun-acceleration`, and `windows-relaxed-route`.
- Keeps all current Mihomo presets and adds no TCP keepalive preset.

- [ ] **Step 1: Add failing content/matrix tests**

Assert these exact materializations:

```yaml
# sandrone:mihomo-preset=udp-p2p-eim
tun:
  endpoint-independent-nat: true
```

```yaml
# sandrone:mihomo-preset=linux-tun-acceleration
find-process-mode: strict
tun:
  auto-route: true
  auto-redirect: true
```

```yaml
# sandrone:mihomo-preset=windows-relaxed-route
tun:
  strict-route: false
```

Ordered rules are:

```ts
stun: [
  "AND,((NETWORK,UDP),(DST-PORT,3478)),REJECT",
  "AND,((NETWORK,UDP),(DST-PORT,5349)),REJECT",
],
quic: ["AND,((NETWORK,UDP),(DST-PORT,443)),REJECT"],
```

Linux acceleration depends on `tun`. STUN and EIM conflict both ways. All five are default off. Assert the catalog has no keepalive ID/label/content.

- [ ] **Step 2: Run the focused test and verify failure**

```bash
cd web
pnpm exec vitest run app/features/files/drivers/mihomo/processor-presets.test.ts
```

Expected: FAIL on the new IDs.

- [ ] **Step 3: Implement exact managed merge/rule descriptors**

Use exact-content recognition. Add these risk statements:

- STUN is a common-port approximation and may break WebRTC, voice/video, P2P, and Tailscale hole punching.
- QUIC blocking forces TCP fallback, removes HTTP/3 benefits, and may break UDP/443 applications.
- EIM may slightly reduce performance/privacy and conflicts with STUN blocking.
- Linux/OpenWrt acceleration is platform-specific, relies on auto-route, may conflict with routing marks, and `strict` only queries process data when rules need it.
- Relaxing Windows strict route may reduce multi-homed DNS leak protection and fail-closed behavior.

Do not add a form field for any of these settings.

- [ ] **Step 4: Extend service assertions for rule positioning and exact settings**

Apply STUN then QUIC to a Mihomo sample and assert processor order becomes STUN rules, QUIC rule, generic rule, final. Apply each merge independently and YAML-decode the result to exact fields. Assert no preset emits `find-process-mode: off` or a keepalive field.

- [ ] **Step 5: Run focused tests and typecheck**

```bash
cd web
pnpm exec vitest run app/features/files/drivers/mihomo/processor-presets.test.ts app/features/files/drivers/core/processor-presets.test.ts
pnpm typecheck
cd ..
go test ./internal/service -run 'CommunityPreset.*Mihomo'
```

Expected: all commands pass.

- [ ] **Step 6: Commit Task 6**

```bash
git add web/app/features/files/drivers/mihomo web/app/shared/i18n internal/service/file_community_preset_test.go
git commit -m "feat(web): add Mihomo scenario presets"
```

---

### Task 7: Add sing-box privacy, compatibility, and platform presets

**Files:**
- Create: `web/app/features/files/processors/scripts/update-sing-box-tun.js`
- Create: `web/app/features/files/processors/sing-box-structure-preset.ts`
- Create: `web/app/features/files/processors/sing-box-structure-preset.test.ts`
- Modify: `web/app/features/files/drivers/sing-box/processor-presets.ts`
- Modify: `web/app/features/files/drivers/sing-box/processor-presets.test.ts`
- Modify: `web/app/shared/i18n/translations/processors.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/processors.en-US.ts`
- Modify: `internal/service/file_community_preset_test.go`

**Interfaces:**
- Adds managed structural operations `ensure-tun`, `ipv4-only`, `udp-p2p-eim`, `linux-tun-acceleration`, `mptcp-direct`, and `windows-relaxed-route` using one exact inline script plus `operation` arg.
- Adds ordered rule IDs `stun-block` and `quic-fallback`.
- `mptcp-direct` depends on Linux acceleration, which depends on `ensure-tun`; STUN/QUIC depend on existing `sniff`.

- [ ] **Step 1: Add failing structure-script and catalog tests**

Test the exact operation results:

```ts
"ensure-tun" => append { type: "tun", tag: "tun-in", address: ["172.19.0.1/30", "fdfe:dcba:9876::1/126"], auto_route: true, strict_route: true } only when absent
"ipv4-only" => dns.strategy = "ipv4_only" and remove IPv6 entries from selected TUN address
"udp-p2p-eim" => selectedTun.endpoint_independent_nat = true
"linux-tun-acceleration" => selectedTun.auto_route = true and selectedTun.auto_redirect = true
"mptcp-direct" => selectedTun.exclude_mptcp = true
"windows-relaxed-route" => selectedTun.strict_route = false
```

Selection rule: exact `tag: "tun-in"` wins; otherwise accept exactly one `type: "tun"`; throw on zero (except ensure) or ambiguity. Preserve every other inbound and array order. Recognition requires exact raw script plus exact operation arg.

Ordered rules:

```ts
stun: [{ protocol: "stun", action: "reject" }],
quic: [{ protocol: "quic", action: "reject" }],
```

The exact Chinese STUN warning is:

> 阻止应用通过 STUN 获取公网出口地址；可能导致 WebRTC、语音通话、视频会议或 P2P 连接降级或失效。默认关闭。

- [ ] **Step 2: Run focused tests and verify failure**

```bash
cd web
pnpm exec vitest run app/features/files/processors/sing-box-structure-preset.test.ts app/features/files/drivers/sing-box/processor-presets.test.ts
```

Expected: FAIL because the structural preset factory is absent.

- [ ] **Step 3: Implement safe TUN selection/mutation and descriptors**

Use `api.json.parse/stringify`; clone/mutate only after validation and stringify once at the end. The new descriptors are default off. EIM conflicts with STUN and explains that the switch has additional effect only with the gVisor stack because other supported stacks already use endpoint-independent NAT. MPTCP warns that sing-box cannot transparently proxy MPTCP and this setting bypasses it directly, exposing the egress. IPv4-only warns IPv6-only resources become unreachable. Linux/Windows warnings mirror Task 6 and mention sing-box platform limits.

- [ ] **Step 4: Extend service tests with non-destructive array assertions**

Use a sing-box file containing one mixed inbound, one custom inbound, and `tun-in`. For every operation assert both unrelated inbounds remain byte-semantically equivalent and ordered. Assert IPv4-only removes only TUN IPv6 address and does not remove unrelated IPv6 config. Run STUN then QUIC and assert both precede generic/final rules without changing final.

- [ ] **Step 5: Run focused tests, typecheck, and Go service tests**

```bash
cd web
pnpm exec vitest run app/features/files/processors/sing-box-structure-preset.test.ts app/features/files/drivers/sing-box/processor-presets.test.ts app/features/files/drivers/core/processor-presets.test.ts
pnpm typecheck
cd ..
go test ./internal/service -run 'CommunityPreset.*SingBox'
```

Expected: all commands pass.

- [ ] **Step 6: Commit Task 7**

```bash
git add web/app/features/files/drivers/sing-box web/app/features/files/processors web/app/shared/i18n internal/service/file_community_preset_test.go
git commit -m "feat(web): add sing-box scenario presets"
```

---

### Task 8: Add Shadowrocket privacy and compatibility presets

**Files:**
- Modify: `web/app/features/files/drivers/shadowrocket/processor-presets.ts`
- Modify: `web/app/features/files/drivers/shadowrocket/processor-presets.test.ts`
- Modify: `web/app/shared/i18n/translations/processors.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/processors.en-US.ts`
- Modify: `internal/service/file_community_preset_test.go`

**Interfaces:**
- Adds IDs `webrtc-privacy`, `disable-ipv6`, `udp-unsupported-direct`, and `restricted-network-dns-fallback`.
- All are exact `ini_override` processors, default off, and have no dependency.
- Shadowrocket adds no QUIC preset because base `block-quic = all-proxy` remains authoritative.

- [ ] **Step 1: Add failing exact-content tests**

Expected patches:

```ini
# sandrone:shadowrocket-preset=webrtc-privacy
[General]
stun-response-ip = 1.1.1.1
stun-response-ipv6 = ::1
```

```ini
# sandrone:shadowrocket-preset=disable-ipv6
[General]
ipv6 = false
prefer-ipv6 = false
```

```ini
# sandrone:shadowrocket-preset=udp-unsupported-direct
[General]
udp-policy-not-supported-behaviour = DIRECT
```

```ini
# sandrone:shadowrocket-preset=restricted-network-dns-fallback
[General]
dns-direct-fallback-proxy = true
```

Assert exact recognition, all default off, and no Shadowrocket QUIC option.

- [ ] **Step 2: Run the focused test and verify failure**

```bash
cd web
pnpm exec vitest run app/features/files/drivers/shadowrocket/processor-presets.test.ts
```

Expected: FAIL on new descriptors.

- [ ] **Step 3: Implement descriptors and fixed risks**

WebRTC warns that voice/video/WebRTC/P2P can degrade or fail. IPv6 warns it controls only expressible Shadowrocket behavior and does not guarantee node transport never uses IPv6. UDP DIRECT warns the real egress, carrier path, and local DNS may leak. Restricted DNS fallback warns direct domains may resolve through the proxy.

- [ ] **Step 4: Extend the service test**

Apply each patch independently to an INI file with comments and multiple sections. Assert only the named General assignments change, section order/comments remain, and UDP stays REJECT without the optional preset.

- [ ] **Step 5: Run focused tests and commit**

```bash
cd web
pnpm exec vitest run app/features/files/drivers/shadowrocket/processor-presets.test.ts
pnpm typecheck
cd ..
go test ./internal/service -run 'CommunityPreset.*Shadowrocket'
git add web/app/features/files/drivers/shadowrocket web/app/shared/i18n internal/service/file_community_preset_test.go
git commit -m "feat(web): add Shadowrocket scenario presets"
```

---

### Task 9: Add Mihomo Tailscale external and native modes

**Files:**
- Create: `web/app/features/files/processors/scripts/mihomo-tailscale-native.js`
- Modify: `web/app/features/files/drivers/mihomo/processor-presets.ts`
- Modify: `web/app/features/files/drivers/mihomo/processor-presets.test.ts`
- Modify: `web/app/shared/i18n/translations/processors.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/processors.en-US.ts`
- Modify: `internal/service/file_community_preset_test.go`

**Interfaces:**
- Finalizes Mihomo IDs `tailscale-native` then `tailscale-external` in Tailscale category order.
- Both conflict with each other and `stun-block`; STUN gains reciprocal conflicts.
- `tailnet-share` remains and depends on external coexistence to preserve current behavior.

- [ ] **Step 1: Add failing external/native tests**

External coexistence remains an editable YAML override and must add only:

```yaml
dns:
  fake-ip-filter+:
    - "+.ts.net"
  nameserver-policy:
    "<+.ts.net>": 100.100.100.100
tun:
  route-exclude-address+:
    - 100.64.0.0/10
    - fd7a:115c:a1e0::/48
```

Native script output must contain exactly one proxy with semantic fields:

```yaml
name: TAILSCALE
type: tailscale
ephemeral: false
udp: true
accept-routes: false
```

It omits hostname/state-dir to use defaults and omits auth-key, control-url, exit-node, and all advanced fields. It adds MagicDNS for `+.ts.net`, removes the two standard Tailscale ranges from TUN route exclusions, and inserts these rules before generic/final rules without changing `MATCH`:

```text
DOMAIN-SUFFIX,ts.net,TAILSCALE
IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve
IP-CIDR6,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve
```

- [ ] **Step 2: Run focused tests and verify failure**

```bash
cd web
pnpm exec vitest run app/features/files/drivers/mihomo/processor-presets.test.ts app/features/files/drivers/core/processor-presets.test.ts
```

Expected: FAIL because native mode is absent and external content is old.

- [ ] **Step 3: Implement the native script atomically**

Parse YAML, validate `proxies`, `rules`, `dns`, and `tun` shapes before mutation, reject an incompatible existing proxy named `TAILSCALE`, de-duplicate exact values, remove only the two standard exclusions, and stringify once. Use the same safe rule anchors as Task 5. The script source itself is the managed marker; exact source recognition makes edited copies user-owned.

- [ ] **Step 4: Wire conflict planning and risk text**

Both Tailscale modes are default off. Explain external mode expects an independent system Tailscale. Explain native mode may print an interactive login URL in the target core logs because Sandrone deliberately omits Auth Key, and that first access can time out while the endpoint starts. Selecting either mode removes recognized STUN; selecting STUN removes recognized Tailscale modes.

- [ ] **Step 5: Extend full-generation tests**

Generate Mihomo with nodes, structured user rule, native Tailscale, generic rule, and final. Assert user → Tailscale → generic → final, exact proxy fields, no standard route exclusions, no Auth Key/control URL/Exit Node strings, and unchanged final policy. Also apply external mode and assert it has exclusions and no tailscale proxy.

- [ ] **Step 6: Run focused tests and commit**

```bash
cd web
pnpm exec vitest run app/features/files/drivers/mihomo/processor-presets.test.ts app/features/files/drivers/core/processor-presets.test.ts app/features/files/processors/processor-builder.test.tsx
pnpm typecheck
cd ..
go test ./internal/service -run 'CommunityPreset.*Mihomo.*Tailscale'
git add web/app/features/files/drivers/mihomo web/app/features/files/processors/scripts web/app/shared/i18n internal/service/file_community_preset_test.go
git commit -m "feat(web): add Mihomo Tailscale modes"
```

---

### Task 10: Add sing-box and Shadowrocket Tailscale modes, canonical docs, and final gates

**Files:**
- Create: `web/app/features/files/processors/scripts/sing-box-tailscale-external.js`
- Create: `web/app/features/files/processors/scripts/sing-box-tailscale-native.js`
- Modify: `web/app/features/files/drivers/sing-box/processor-presets.ts`
- Modify: `web/app/features/files/drivers/sing-box/processor-presets.test.ts`
- Modify: `web/app/features/files/drivers/shadowrocket/processor-presets.ts`
- Modify: `web/app/features/files/drivers/shadowrocket/processor-presets.test.ts`
- Modify: `web/app/features/files/processors/processor-builder.test.tsx`
- Modify: `web/app/shared/i18n/translations/processors.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/processors.en-US.ts`
- Modify: `internal/service/file_community_preset_test.go`
- Create: `docs/reference/community-config-presets.md`
- Modify: `docs/README.md`
- Modify: `docs/reference/mihomo-fake-ip.md`
- Modify: `docs/how-to/troubleshoot-mihomo-fake-ip.md`
- Modify: `docs/reference/processors.md`

**Interfaces:**
- sing-box adds `tailscale-native` then `tailscale-external`; both conflict with each other and STUN, and STUN gains reciprocal conflicts.
- Shadowrocket adds only `tailscale-native`, implemented as ordered TAILSCALE rules; it has no external mode or module-activation warning.
- Produces the long-term canonical preset reference; implementation spec/plan remain temporary until controller finalization.

- [ ] **Step 1: Add failing sing-box/Shadowrocket Tailscale tests**

sing-box external coexistence must:

- add `100.64.0.0/10` and `fd7a:115c:a1e0::/48` to the selected TUN `route_exclude_address` without replacing other entries;
- add a dedicated UDP DNS server for `100.100.100.100` and route only `ts.net` suffix queries to it;
- create no Tailscale endpoint.

sing-box native must:

```json
{
  "type": "tailscale",
  "tag": "ts-ep",
  "ephemeral": false,
  "accept_routes": false
}
```

It must omit state_directory/hostname to use defaults and omit auth_key, control_url, exit_node, advertise, relay, SSH, MTU, and system-interface fields. It removes external exclusions, adds DNS server `{ "type": "tailscale", "tag": "ts-dns", "endpoint": "ts-ep", "accept_default_resolvers": false }`, adds the v1.13.14-compatible DNS rule `{ "ip_accept_any": true, "server": "ts-dns" }`, and inserts route rule `{ "preferred_by": ["ts-ep"], "action": "route", "outbound": "ts-ep" }` before generic/final.

Shadowrocket native inserts exactly:

```text
DOMAIN-SUFFIX,ts.net,TAILSCALE
IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve
IP-CIDR,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve
```

- [ ] **Step 2: Run focused tests and verify failure**

```bash
cd web
pnpm exec vitest run app/features/files/drivers/sing-box/processor-presets.test.ts app/features/files/drivers/shadowrocket/processor-presets.test.ts app/features/files/processors/processor-builder.test.tsx
```

Expected: FAIL on missing modes and interaction notice.

- [ ] **Step 3: Implement both sing-box scripts with pre-validation**

Reuse Task 7's exact TUN selection. Validate all target arrays/objects first, reject incompatible existing `ts-ep`/`ts-dns` tags, de-duplicate exact owned values, and stringify once. Native removes only standard Tailscale exclusions. External does not create endpoints. Neither changes `route.final` or `dns.final`.

The Shadowrocket preset reuses Task 5's ordered-rule script and shows no module activation warning. All Tailscale presets are default off and shown native-first.

- [ ] **Step 4: Complete cross-kind, security, and full-generation tests**

Extend pure planner tests and the one existing focused React test so:

- native/external replace one another atomically;
- Tailscale removes recognized STUN and lists it;
- STUN removes recognized Tailscale and lists it;
- edited/unrecognized processors remain;
- non-conflict order remains;
- selecting a mode twice is idempotent.

In Go service tests, generate one complete file per target with nodes, user rule, Tailscale, generic rule, and final. Assert exact order, final unchanged, native mode has no external exclusions, external mode has no endpoint, and every result lacks case-insensitive `auth_key`, `auth-key`, `control_url`, `control-url`, and Exit Node fields.

- [ ] **Step 5: Write the canonical documentation and replace duplicate detail with links**

`docs/reference/community-config-presets.md` must include one table/section per preset with motivation, supported clients, default state, exact generated behavior, risk, dependencies/conflicts, and primary official/community sources. State explicitly:

- defaults affect new files/groups only;
- processors are copied/editable and do not auto-update/migrate;
- fixed target revisions and inspect display behavior;
- NTP direct leak warning;
- STUN/QUIC/IPv4/IPv6/EIM/platform warnings;
- Tailscale three-state semantics per client;
- no Auth Key/login form/Headscale/Exit Node/default final rewrite;
- sing-box v1.13.14 uses route `preferred_by` and legacy MagicDNS `ip_accept_any`.
- standard Tailnet CIDRs are only editable starting points; users add advertised subnet CIDRs by editing the copied processor, while `accept_routes` and Exit Node remain off unless they deliberately edit target-native content.

Link it from `docs/README.md`. Replace the detailed Tailscale preset matrix in `mihomo-fake-ip.md` with a short link, retain only fake-IP-specific facts, and point troubleshooting/processors docs to the canonical page rather than copying the matrix.

- [ ] **Step 6: Run narrow checks, full Web gates, and repo gate**

```bash
cd web
pnpm exec vitest run app/features/files/drivers/core/processor-presets.test.ts app/features/files/drivers/mihomo/processor-presets.test.ts app/features/files/drivers/sing-box/processor-presets.test.ts app/features/files/drivers/shadowrocket/processor-presets.test.ts app/features/files/processors/ordered-rule-preset.test.ts app/features/files/processors/sing-box-structure-preset.test.ts app/features/files/processors/processor-builder.test.tsx app/features/files/model/renderer-revision.test.ts
pnpm typecheck
pnpm lint
cd ..
go test ./internal/service -run CommunityPreset
make check
```

Expected: every command exits 0.

- [ ] **Step 7: Scan forbidden fields and stale identifiers**

```bash
rg -n -i 'auth[_-]key|control[_-]url|headscale|exit[_-]node' web/app/features/files docs/reference/community-config-presets.md
rg -n 'find-process-mode:\s*off|tcp-keep-alive|always-real-ip\s*=' web/app/features/files/drivers
git diff --check
```

Expected: forbidden strings appear only in explicit documentation/tests asserting omission, `always-real-ip` remains only the existing commented Shadowrocket base example, and no prohibited preset/config field is emitted.

- [ ] **Step 8: Commit Task 10**

```bash
git add web/app/features/files web/app/shared/i18n internal/service/file_community_preset_test.go docs
git commit -m "feat(web): complete community configuration presets"
```

After the final whole-branch review is clean, remove this completed plan and its superseded design spec as repository hygiene, commit that documentation-only deletion, re-run `git diff --check` and `make check`, and keep the canonical reference.
