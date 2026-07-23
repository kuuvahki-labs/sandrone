# File Config Content Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify file-editor reading order around a visible configuration-content area and place template selection before node source and adaptive generation.

**Architecture:** Keep the existing file-form and editor boundaries. Change only the structured and raw configuration editor presentation: introduce a shared translation for the visible content heading, rename the base text section, reorder existing structured-editor components, and remove the redundant details heading.

**Tech Stack:** React 19, TypeScript, Material UI, Testing Library, Vitest, project i18n dictionaries.

## Global Constraints

- The top-level reading model is “Basic information → Configuration content → File processing”.
- Structured configuration content is ordered “Configuration template → Node source → Adaptive groups → Base configuration content → Proxy groups → Rule sets → Routing rules”.
- The change must not alter form serialization, template application, node preview, adaptive generation, or backend configuration formats.
- English and Simplified Chinese copy must remain aligned.

---

### Task 1: Lock the configuration-content hierarchy with tests

**Files:**
- Modify: `web/app/features/files/config/components/editor.integration.test.tsx`
- Modify: `web/app/features/files/editor/file-form-driver.test.tsx`

**Interfaces:**
- Consumes: Existing `FileConfigEditor`, `FileKindConfigWorkbench`, and translation-backed accessible labels.
- Produces: Regression contracts for visible headings, labels, and DOM order.

- [x] **Step 1: Write the failing structured-editor order test**

Add a test that renders the English structured editor and asserts:

```tsx
const template = screen.getByRole("group", { name: "Configuration template" });
const nodeSource = screen.getByRole("group", { name: "Node source" });

expect(screen.getByRole("heading", { name: "Configuration content" })).toBeInTheDocument();
expect(screen.getByRole("group", { name: "Base configuration content" })).toBeInTheDocument();
expect(template.compareDocumentPosition(nodeSource) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
expect(screen.queryByRole("heading", { name: "Configuration details" })).not.toBeInTheDocument();
```

- [x] **Step 2: Write the failing raw-editor naming test**

Render a raw configuration driver through `FileKindConfigWorkbench` and assert that
“Configuration content” and “Base configuration content” are visible.

- [x] **Step 3: Run the focused tests and confirm failure**

Run:

```bash
cd web
pnpm test:run app/features/files/config/components/editor.integration.test.tsx app/features/files/editor/file-form-driver.test.tsx
```

Expected: FAIL because the content heading and renamed base label do not exist, and node source still precedes the template.

### Task 2: Implement the hierarchy and copy

**Files:**
- Modify: `web/app/features/files/config/components/editor.tsx`
- Modify: `web/app/features/files/editor/raw-config-editor.tsx`
- Modify: `web/app/shared/i18n/translations/en-US.ts`
- Modify: `web/app/shared/i18n/translations/zh-CN.ts`

**Interfaces:**
- Consumes: `useI18n().t`, `ConfigTemplatePicker`, `ConfigNodeSourceSection`, `ConfigAdaptiveGroupControls`, and `WorkbenchGroupSection`.
- Produces: `files.config.content` and `files.config.baseContent` translation keys and the approved visual order.

- [x] **Step 1: Add the new translation keys and remove obsolete keys**

Use:

```ts
"files.config.content": "Configuration content",
"files.config.baseContent": "Base configuration content",
```

and:

```ts
"files.config.content": "配置内容",
"files.config.baseContent": "基础配置内容",
```

Remove `files.config.base` and `files.config.details` after all call sites migrate.

- [x] **Step 2: Update the structured editor**

At the beginning of the visible editor content, render:

```tsx
<Typography className="font-semibold" component="h2" variant="h6">
  {t("files.config.content")}
</Typography>
```

In structured mode, place the existing template section before
`ConfigNodeSourceSection`, keep adaptive controls immediately after node source,
remove the `files.config.details` heading, and label the base source section with
`files.config.baseContent`.

- [x] **Step 3: Update raw modes**

Render the same visible `files.config.content` heading in `RawFileConfigEditor` and
use `files.config.baseContent` for its base source section. Use the renamed base
label in the structured editor’s preserved-raw branch as well.

- [x] **Step 4: Run focused tests**

Run:

```bash
cd web
pnpm test:run app/features/files/config/components/editor.integration.test.tsx app/features/files/editor/file-form-driver.test.tsx
```

Expected: PASS.

### Task 3: Verify behavior and repository hygiene

**Files:**
- Verify all modified Web files.

**Interfaces:**
- Consumes: Completed Tasks 1 and 2.
- Produces: Evidence that the presentation-only change has no behavioral regressions.

- [x] **Step 1: Confirm obsolete copy keys are gone**

Run:

```bash
rg -n 'files\.config\.(base|details)' web/app
```

Expected: no matches.

- [x] **Step 2: Run Web quality gates**

Run:

```bash
cd web
pnpm typecheck
pnpm lint
pnpm test:run
pnpm build
```

Expected: all commands exit successfully.

- [x] **Step 3: Check the final diff**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors and only intended working-tree changes.
