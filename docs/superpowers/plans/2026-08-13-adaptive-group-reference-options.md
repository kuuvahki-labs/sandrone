# Adaptive Group Reference Options Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make generated adaptive region groups immediately selectable as proxy-group references in proxy-group members and routing-rule policies, while keeping them out of node options.

**Architecture:** Keep `ConfigNodeSummary[]` as the real-node source and reuse `editorState.structure.groups` as the only proxy-group reference source. Add an explicit localized type label to reference options, then verify through the real `FileConfigEditor` flow that adaptive generation refreshes both dependent selectors without a page reload.

**Tech Stack:** React 19, TypeScript, Material UI Autocomplete, Vitest, Testing Library.

## Global Constraints

- Generated regions remain proxy groups and must never be copied into `ConfigNodeSummary[]`.
- Proxy-group member selectors exclude the group currently being edited.
- Routing-rule policy selectors include generated region groups.
- Regeneration must replace stale choices through the existing `structureRevision` flow.
- Do not change adaptive matching, naming, placement, serialization, or target-driver behavior.

---

### Task 1: Prove Generated Groups Reach Both Selectors

**Files:**
- Modify: `web/app/features/files/config/components/editor.integration.test.tsx`

**Interfaces:**
- Consumes: `FileConfigEditor`, subscription preview loading, `applyConfigEditorAdaptiveGeneration`, and the existing proxy-group/rule editors.
- Produces: An integration regression proving generated groups are selectable immediately in both reference surfaces and are not presented as nodes.

- [ ] **Step 1: Add a failing editor integration test**

Add a test that renders the Mihomo editor with a subscription preview containing `HK-01`, expands the adaptive-region scope, enables only Hong Kong if necessary, and clicks `Generate`.

The test must then:

```ts
expect(screen.getByRole("button", { name: "Expand proxy group Hong Kong" }))
  .toBeInTheDocument();
```

Open a non-Hong-Kong proxy group, add or open a fixed member field, and assert its autocomplete contains a group reference whose accessible option text includes both `Hong Kong` and `Proxy group`.

Open a routing rule with a policy field and assert the policy autocomplete contains the same group reference.

Also assert the `Hong Kong` choice does not expose node metadata such as `ss` or `node-1.example:8388`.

- [ ] **Step 2: Run the focused integration test and verify failure**

Run:

```bash
pnpm --dir web test:run app/features/files/config/components/editor.integration.test.tsx
```

Expected: FAIL because group reference options currently render only their value and do not identify themselves as proxy groups.

- [ ] **Step 3: Commit the regression test only if independently useful**

Do not commit a red test separately. Keep it staged with Task 2 unless the test unexpectedly passes and exposes a different root cause requiring plan revision.

### Task 2: Label Group References Without Polluting Nodes

**Files:**
- Modify: `web/app/features/files/config/model/references.ts`
- Modify: `web/app/features/files/config/model/references.test.ts`
- Modify: `web/app/features/files/config/components/reference-fields.tsx`
- Modify: `web/app/shared/i18n/translations/files.en-US.ts`
- Modify: `web/app/shared/i18n/translations/files.zh-CN.ts`
- Test: `web/app/features/files/config/components/editor.integration.test.tsx`

**Interfaces:**
- Consumes: `ConfigReferenceOption.kind`, `memberReferenceOptions`, `policyReferenceOptions`, and `CreatableReferenceField`.
- Produces: Localized display detail for proxy-group references while preserving existing option values and serialization.

- [ ] **Step 1: Add reference-model expectations**

Extend `references.test.ts` to assert that group options remain `kind: "group"`, node options remain `kind: "node"`, and generated-style group names such as `Hong Kong` are included in member and policy options without appearing in the node input array.

- [ ] **Step 2: Add localized reference-kind copy**

Add these translation keys:

```ts
"files.config.referenceKind.group": "Proxy group"
"files.config.referenceKind.node": "Node"
"files.config.referenceKind.macro": "Node collection"
"files.config.referenceKind.builtin": "Built-in policy"
```

Use the corresponding Chinese values:

```ts
"files.config.referenceKind.group": "代理组"
"files.config.referenceKind.node": "节点"
"files.config.referenceKind.macro": "节点集合"
"files.config.referenceKind.builtin": "内置策略"
```

- [ ] **Step 3: Render the option kind as secondary detail**

In `CreatableReferenceField`, derive the localized kind label from `option.kind`. Render it as the option's secondary caption, combining it with existing node detail when present:

```ts
const detail = [
  t(`files.config.referenceKind.${option.kind}`),
  option.detail,
].filter(Boolean).join(" · ");
```

Do not change `option.value`, `isOptionEqualToValue`, `onChange`, or serialized configuration values.

- [ ] **Step 4: Run focused tests**

Run:

```bash
pnpm --dir web test:run \
  app/features/files/config/model/references.test.ts \
  app/features/files/config/components/editor.integration.test.tsx \
  app/features/files/config/components/group-editor.test.tsx \
  app/features/files/config/components/rule-editor.test.tsx
```

Expected: PASS.

- [ ] **Step 5: Run Web validation**

Run:

```bash
pnpm --dir web typecheck
pnpm --dir web lint
```

Expected: both commands exit successfully.

- [ ] **Step 6: Confirm scope and stale references**

Run:

```bash
git diff --check
rg -n "referenceKind\.(group|node|macro|builtin)" web/app
```

Expected: no whitespace errors; all four keys are defined in both locales and consumed by the reference field.

- [ ] **Step 7: Commit the implementation**

```bash
git add \
  web/app/features/files/config/model/references.ts \
  web/app/features/files/config/model/references.test.ts \
  web/app/features/files/config/components/reference-fields.tsx \
  web/app/features/files/config/components/editor.integration.test.tsx \
  web/app/shared/i18n/translations/files.en-US.ts \
  web/app/shared/i18n/translations/files.zh-CN.ts
git commit -m "fix(web): expose adaptive groups as references"
```
