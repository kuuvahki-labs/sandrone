# File Processor Preset Internationalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Parameterize the GitHub rule-source mirror preset as a reusable string-replacement script and localize every newly generated Web file-processor preset name.

**Architecture:** `FileProcessorPreset.build` receives the current Web translator, so defaults, direct additions, and dependency additions derive `ProcessorDetail.name` from the same `labelKey`. The GitHub shortcut becomes a normal shared preset whose generic inline script reads an ordered replacement table from `params.args`, while recognition accepts both the new parameterized form and the legacy marker form.

**Tech Stack:** TypeScript, React, Vitest, Testing Library, JavaScript processor scripts, Sandrone file-driver registry, Markdown reference docs.

## Global Constraints

- New files and newly added presets use the current Web UI language; existing saved processor names remain unchanged.
- Preserve product names, protocol names, and technical abbreviations such as `GitHub`, `TUN`, `DNS`, `QUIC`, `UDP/P2P`, `MPTCP`, `Fake-IP`, `IPv4`, `Tailscale`, and `Tailnet`.
- Do not add a backend processor type or change the file-stage `script` execution contract.
- Preset recognition must ignore `ProcessorDetail.name` and continue to recognize edited managed parameters where specified.
- Every Web built-in preset script that reads `input.args` starts with a concise parameter-name, format, and purpose header.
- Keep only the legacy GitHub marker recognizer as a compatibility point; do not retain the old builder branch, option constant, or script-generation path.
- Preserve unrelated working-tree changes and run focused tests before broad gates.

## File Map

- Create `web/app/features/files/processors/scripts/replace-strings.js`: generic ordered literal string replacement using managed args.
- Create `web/app/features/files/processors/github-rule-source-mirror-preset.ts`: shared GitHub mirror preset factory, descriptor, defaults, and legacy recognition.
- Create `web/app/features/files/processors/github-rule-source-mirror-preset.test.ts`: parameter serialization, execution, validation, and compatibility coverage.
- Delete `web/app/features/files/processors/scripts/github-rule-source-rewrite.js`: mappings move to preset args.
- Delete `web/app/features/files/processors/rule-source-rewrite-preset.ts` and its test: old naming and special preset surface are replaced.
- Modify `web/app/features/files/drivers/core/file-driver.ts`: translator-aware default processor factory contract.
- Modify `web/app/features/files/drivers/core/processor-presets.ts` and tests: translator-aware preset build and planner contract.
- Modify the Mihomo, sing-box, and Shadowrocket `processor-presets.ts` files and tests: derive generated names from `labelKey` and include the shared GitHub preset.
- Modify `web/app/features/files/processors/ordered-rule-preset.ts` and tests: receive a localized name separately from semantic options.
- Modify `web/app/features/files/processors/sing-box-structure-preset.ts` and tests: receive a localized name separately from semantic options.
- Modify `web/app/features/files/editor/file-form.tsx` and driver/page tests: pass the current translator to default factories.
- Modify `web/app/features/files/processors/processor-builder.tsx` and tests: remove the GitHub special path and pass the translator into normal preset planning.
- Modify processor and file translation bundles: add the renamed GitHub label and finish Chinese labels whose action wording is still English.
- Modify the four parameterized scripts under `web/app/features/files/processors/scripts/`: add accurate header comments.
- Modify `docs/reference/processors.md` and `docs/reference/community-config-presets.md`: document the generic args and renamed shortcut.
- Delete the completed design and plan documents in the final cleanup commit, per `web/AGENTS.md` documentation hygiene.

---

### Task 1: Generic string-replacement preset

**Files:**
- Create: `web/app/features/files/processors/scripts/replace-strings.js`
- Create: `web/app/features/files/processors/github-rule-source-mirror-preset.ts`
- Create: `web/app/features/files/processors/github-rule-source-mirror-preset.test.ts`
- Delete: `web/app/features/files/processors/scripts/github-rule-source-rewrite.js`
- Delete: `web/app/features/files/processors/rule-source-rewrite-preset.ts`
- Delete: `web/app/features/files/processors/rule-source-rewrite-preset.test.ts`

**Interfaces:**
- Produces: `githubRuleSourceMirrorProcessorPreset(name: string): ProcessorDetail`.
- Produces: `recognizeGitHubRuleSourceMirrorProcessorPreset(processor): boolean` accepting new args and the legacy marker.
- Produces: `GITHUB_RULE_SOURCE_MIRROR_PRESET_ID = "github-rule-source-mirror"`.
- Produces: generic script args `preset_id: string` and `replacements: [string, string][]`.

- [ ] **Step 1: Replace the old test with failing parameterization and compatibility tests**

Assert the new processor contains no GitHub/jsDelivr URLs in its source, stores all three ordered mappings in `replacements`, rewrites literal strings in order, rejects malformed mappings, survives user edits to `replacements`, and recognizes this legacy processor shape:

```ts
expect(recognizeGitHubRuleSourceMirrorProcessorPreset({
  type: "script",
  params: {
    source: {
      type: "inline",
      content: "// sandrone:file-preset=github-rule-source-rewrite\nfunction main(input) { return input; }",
    },
  },
})).toBe(true);
```

- [ ] **Step 2: Run the focused test and verify the new module is missing**

Run: `cd web && pnpm vitest run app/features/files/processors/github-rule-source-mirror-preset.test.ts`

Expected: FAIL because `github-rule-source-mirror-preset.ts` does not exist.

- [ ] **Step 3: Implement the generic script and new preset factory**

The script header and core must follow this shape:

```js
/* eslint-disable unused-imports/no-unused-vars */

// Parameters:
// - preset_id: stable preset identifier; request args must not override it.
// - replacements: ordered array of [source, destination] string pairs.
function main(input) {
  rejectManagedRequestArgOverrides(input);
  const presetID = stringArgument(input, "preset_id");
  const replacements = input.args.replacements;
  if (!Array.isArray(replacements) || replacements.some((pair) => (
    !Array.isArray(pair)
    || pair.length !== 2
    || typeof pair[0] !== "string"
    || typeof pair[1] !== "string"
  ))) {
    throw new Error("Sandrone preset " + presetID + " requires ordered [source, destination] string pairs");
  }
  if (!input.file || typeof input.file.content !== "string") return input;
  let content = input.file.content;
  for (const pair of replacements) content = content.split(pair[0]).join(pair[1]);
  input.file.content = content;
  return input;
}
```

The factory writes:

```ts
params: {
  source: { type: "inline", content: replaceStringsScript },
  args: {
    preset_id: GITHUB_RULE_SOURCE_MIRROR_PRESET_ID,
    replacements: GITHUB_RULE_SOURCE_MIRROR_REPLACEMENTS.map((pair) => [...pair]),
  },
}
```

Recognition requires exact new `source` plus `args.preset_id`, but deliberately does not compare `replacements`; the legacy branch only accepts inline script content containing the old stable marker.

- [ ] **Step 4: Run the focused test and verify it passes**

Run: `cd web && pnpm vitest run app/features/files/processors/github-rule-source-mirror-preset.test.ts`

Expected: PASS.

- [ ] **Step 5: Commit the generic preset**

```bash
git add web/app/features/files/processors
git commit -m "refactor(web): parameterize rule source replacement"
```

### Task 2: Translator-aware preset construction

**Files:**
- Modify: `web/app/features/files/drivers/core/file-driver.ts`
- Modify: `web/app/features/files/drivers/core/processor-presets.ts`
- Modify: `web/app/features/files/drivers/core/processor-presets.test.ts`
- Modify: `web/app/features/files/processors/ordered-rule-preset.ts`
- Modify: `web/app/features/files/processors/ordered-rule-preset.test.ts`
- Modify: `web/app/features/files/processors/sing-box-structure-preset.ts`
- Modify: `web/app/features/files/processors/sing-box-structure-preset.test.ts`
- Modify: `web/app/features/files/editor/file-form.tsx`

**Interfaces:**
- Changes: `FileProcessorPreset.build(t: Translator): ProcessorDetail`.
- Changes: `FileDriverDefinition.processors.defaults(t: Translator): ProcessorDetail[]`.
- Changes: `planFileProcessorPresetAddition(catalog, requestedPresetID, current, t): FileProcessorPresetPlan`.
- Changes: `orderedRuleProcessorPreset(options, name): ProcessorDetail` with `name` removed from semantic options.
- Changes: `singBoxStructureProcessorPreset(options, name): ProcessorDetail` with `name` removed from semantic options.

- [ ] **Step 1: Add failing core tests for translated direct and dependency additions**

Use `createTranslator("zh-CN")` and a catalog whose `labelKey` values resolve to distinct names. Assert both the requested preset and an automatically inserted dependency receive translated `ProcessorDetail.name` values while recognition still ignores names.

- [ ] **Step 2: Run the core tests and verify the old build signature fails**

Run: `cd web && pnpm vitest run app/features/files/drivers/core/processor-presets.test.ts`

Expected: FAIL because the planner does not pass a translator to `build`.

- [ ] **Step 3: Change the core interfaces and planner**

Use these exact signatures:

```ts
export interface FileProcessorPreset {
  // existing metadata
  build(t: Translator): ProcessorDetail;
  recognize(processor: Pick<ProcessorDetail, "type" | "params">): boolean;
}

export function planFileProcessorPresetAddition(
  catalog: readonly FileProcessorPreset[],
  requestedPresetID: string,
  current: readonly ProcessorDetail[],
  t: Translator,
): FileProcessorPresetPlan;
```

Build every planned addition with `preset.build(t)`. Change driver defaults to accept `Translator`, and call `driver.processors.defaults(t)` from `FileFormFields`.

- [ ] **Step 4: Separate semantic options from localized names in script factories**

Remove `name` from `OrderedRuleProcessorPresetOptions` and `SingBoxStructureProcessorPresetOptions`. Pass the localized name as the second factory argument:

```ts
orderedRuleProcessorPreset(options, t(labelKey));
singBoxStructureProcessorPreset(options, t(labelKey));
```

Recognition continues to compare only sources and semantic args.

- [ ] **Step 5: Run core and script-factory tests**

Run: `cd web && pnpm vitest run app/features/files/drivers/core/processor-presets.test.ts app/features/files/processors/ordered-rule-preset.test.ts app/features/files/processors/sing-box-structure-preset.test.ts`

Expected: PASS.

- [ ] **Step 6: Commit the construction contract**

```bash
git add web/app/features/files/drivers/core web/app/features/files/editor/file-form.tsx web/app/features/files/processors/ordered-rule-preset.ts web/app/features/files/processors/ordered-rule-preset.test.ts web/app/features/files/processors/sing-box-structure-preset.ts web/app/features/files/processors/sing-box-structure-preset.test.ts
git commit -m "refactor(web): localize preset construction"
```

### Task 3: Localize all driver preset names and integrate the shared GitHub preset

**Files:**
- Modify: `web/app/features/files/drivers/mihomo/processor-presets.ts`
- Modify: `web/app/features/files/drivers/mihomo/processor-presets.test.ts`
- Modify: `web/app/features/files/drivers/sing-box/processor-presets.ts`
- Modify: `web/app/features/files/drivers/sing-box/processor-presets.test.ts`
- Modify: `web/app/features/files/drivers/shadowrocket/processor-presets.ts`
- Modify: `web/app/features/files/drivers/shadowrocket/processor-presets.test.ts`
- Modify: `web/app/features/files/processors/github-rule-source-mirror-preset.ts`
- Modify: `web/app/shared/i18n/translations/files.en-US.ts`
- Modify: `web/app/shared/i18n/translations/files.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/processors.en-US.ts`
- Modify: `web/app/shared/i18n/translations/processors.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/en-US.ts`
- Modify: `web/app/shared/i18n/translations/zh-CN.ts`

**Interfaces:**
- Produces: `githubRuleSourceMirrorPreset: FileProcessorPreset` with category `network`.
- Consumes: translator-aware `FileProcessorPreset.build(t)` from Task 2.
- Preserves: each driver preset's existing ID, params, dependencies, conflicts, default-on state, and recognition behavior.

- [ ] **Step 1: Add failing locale matrix tests**

For every preset in all three driver catalogs, call `preset.build(createTranslator(locale))` for `zh-CN` and `en-US`, then assert `processor.name === createTranslator(locale)(preset.labelKey)`. Add explicit assertions for:

```ts
expect(zh("processors.filePreset.ntpDirect.label")).toBe("传统 NTP 直连");
expect(zh("processors.filePreset.singBox.sniff.label")).toBe("流量嗅探与 DNS 劫持");
expect(zh("files.processor.githubRuleSourceMirrorPreset")).toBe("GitHub 规则源镜像替换");
expect(en("files.processor.githubRuleSourceMirrorPreset")).toBe("GitHub rule source mirror replacement");
```

- [ ] **Step 2: Run driver tests and verify hard-coded names fail the matrix**

Run: `cd web && pnpm vitest run app/features/files/drivers/mihomo/processor-presets.test.ts app/features/files/drivers/sing-box/processor-presets.test.ts app/features/files/drivers/shadowrocket/processor-presets.test.ts`

Expected: FAIL on existing hard-coded names and missing GitHub descriptor.

- [ ] **Step 3: Replace hard-coded preset names with translated label keys**

Delete `PRESET_NAMES` records. Each descriptor's `build(t)` must use `t(labelKey)` for merge, ordered-rule, structure, and Tailscale script processors. Default factories become:

```ts
export function defaultMihomoProcessors(t: Translator): ProcessorDetail[] {
  return mihomoProcessorPresets
    .filter((preset) => preset.defaultOn)
    .map((preset) => preset.build(t));
}
```

Apply the same contract to sing-box and Shadowrocket.

- [ ] **Step 4: Define and include the shared GitHub descriptor**

Export a normal descriptor using ID `github-rule-source-mirror`, category `network`, no dependencies or conflicts, and the renamed label/description keys. Include the same descriptor object in the Mihomo, sing-box, and Shadowrocket catalogs; do not include it in the static driver.

- [ ] **Step 5: Update translations without translating professional terms**

Rename the old `files.processor.ruleSourceRewritePreset` keys. Translate action wording around retained technical terms and ensure each driver preset has a localized label suitable for both the menu and saved name.

- [ ] **Step 6: Run driver and i18n tests**

Run: `cd web && pnpm vitest run app/features/files/drivers/mihomo/processor-presets.test.ts app/features/files/drivers/sing-box/processor-presets.test.ts app/features/files/drivers/shadowrocket/processor-presets.test.ts app/shared/i18n/context.test.tsx app/shared/i18n/locales.test.ts`

Expected: PASS.

- [ ] **Step 7: Commit localized catalogs**

```bash
git add web/app/features/files/drivers web/app/features/files/processors/github-rule-source-mirror-preset.ts web/app/shared/i18n/translations
git commit -m "feat(web): localize file processor presets"
```

### Task 4: Use normal preset planning in the builder and preserve historical names

**Files:**
- Modify: `web/app/features/files/processors/processor-builder.tsx`
- Modify: `web/app/features/files/processors/processor-builder.test.tsx`
- Modify: `web/app/features/files/pages/file-new-page.test.tsx`
- Modify: `web/app/features/files/editor/file-form-driver.test.tsx`

**Interfaces:**
- Consumes: `planFileProcessorPresetAddition(catalog, presetID, current, t)`.
- Removes: `RULE_SOURCE_REWRITE_KINDS`, `RULE_SOURCE_REWRITE_PRESET_OPTION`, and the special GitHub addition branch.
- Preserves: loaded `ProcessorDetail.name` values exactly unless the user edits them.

- [ ] **Step 1: Rewrite builder tests around the normal catalog path**

Assert the GitHub option appears in the network group for all three typed kinds and not static; adding it produces the localized name and parameterized args; adding it again is suppressed; a legacy marker processor suppresses another addition; and an existing processor named `GitHub Rule Source Rewrite` remains unchanged on serialization in its typed-file editor.

- [ ] **Step 2: Add new-file locale assertions**

Change the existing sing-box create-page expectation in the default Chinese test to:

```ts
expect(processors.map((processor: ProcessorDetail) => processor.name)).toEqual([
  "流量嗅探与 DNS 劫持",
  "传统 NTP 直连",
]);
```

Add or update an English-locale form test to assert the corresponding English defaults.

- [ ] **Step 3: Run builder and form tests and verify failures**

Run: `cd web && pnpm vitest run app/features/files/processors/processor-builder.test.tsx app/features/files/pages/file-new-page.test.tsx app/features/files/editor/file-form-driver.test.tsx`

Expected: FAIL until the special path is removed and translators flow through both creation paths.

- [ ] **Step 4: Remove the special branch and use the normal planner**

Build menu entries only from `driver.processors.presets`; pass `t` into `planFileProcessorPresetAddition`. Do not normalize names in `draftFromProcessor` or `serializeDraft`, which preserves old saved names.

- [ ] **Step 5: Run builder and form tests**

Run: `cd web && pnpm vitest run app/features/files/processors/processor-builder.test.tsx app/features/files/pages/file-new-page.test.tsx app/features/files/editor/file-form-driver.test.tsx`

Expected: PASS.

- [ ] **Step 6: Commit builder integration**

```bash
git add web/app/features/files/processors/processor-builder.tsx web/app/features/files/processors/processor-builder.test.tsx web/app/features/files/pages/file-new-page.test.tsx web/app/features/files/editor/file-form-driver.test.tsx
git commit -m "refactor(web): use shared rule mirror preset"
```

### Task 5: Parameter headers and canonical documentation

**Files:**
- Modify: `web/app/features/files/processors/scripts/insert-mihomo-rules.js`
- Modify: `web/app/features/files/processors/scripts/insert-shadowrocket-rules.js`
- Modify: `web/app/features/files/processors/scripts/insert-sing-box-rules.js`
- Modify: `web/app/features/files/processors/scripts/update-sing-box-tun.js`
- Modify: `web/app/features/files/processors/ordered-rule-preset.test.ts`
- Modify: `web/app/features/files/processors/sing-box-structure-preset.test.ts`
- Modify: `docs/reference/processors.md`
- Modify: `docs/reference/community-config-presets.md`

**Interfaces:**
- Documents: `preset_id`, `rules_json`, optional `insert_mode`, `operation`, and the new replacement args.
- Canonical docs: `docs/reference/processors.md` owns the full generic replacement parameter contract; community presets link to it.

- [ ] **Step 1: Add failing source-header assertions**

For ordered-rule scripts, assert the first comment block names `preset_id`, `rules_json`, and `insert_mode` only for Shadowrocket. For the sing-box structure script, assert it names `operation`. The GitHub test from Task 1 already covers `preset_id` and `replacements`.

- [ ] **Step 2: Run script tests and verify missing headers fail**

Run: `cd web && pnpm vitest run app/features/files/processors/ordered-rule-preset.test.ts app/features/files/processors/sing-box-structure-preset.test.ts app/features/files/processors/github-rule-source-mirror-preset.test.ts`

Expected: FAIL on the four existing parameterized scripts.

- [ ] **Step 3: Add concise English parameter headers**

Use this format directly below the ESLint directive:

```js
// Parameters:
// - preset_id: stable preset identifier; request args must not override it.
// - rules_json: JSON array of rules inserted by this preset.
```

Add `insert_mode` only to Shadowrocket and `operation` only to the sing-box structure updater. Do not add parameter headers to the three Tailscale scripts because they do not read `input.args`.

- [ ] **Step 4: Update canonical docs**

Rename the shortcut to “GitHub 规则源镜像替换”, document `replacements` as an ordered array of string pairs, state that replacements are global literal operations executed in order, and link the community preset page back to this section. Remove wording that says users edit destination literals in source code.

- [ ] **Step 5: Run script tests and docs hygiene checks**

Run: `cd web && pnpm vitest run app/features/files/processors/ordered-rule-preset.test.ts app/features/files/processors/sing-box-structure-preset.test.ts app/features/files/processors/github-rule-source-mirror-preset.test.ts`

Run: `rg -n 'GitHub Rule Source Rewrite|GitHub 规则源地址替换|ruleSourceRewritePreset|github-rule-source-rewrite' web docs --glob '!docs/superpowers/**'`

Expected: tests PASS; `rg` output contains only the explicit legacy marker compatibility assertion and recognizer.

- [ ] **Step 6: Commit headers and docs**

```bash
git add web/app/features/files/processors/scripts web/app/features/files/processors/ordered-rule-preset.test.ts web/app/features/files/processors/sing-box-structure-preset.test.ts docs/reference/processors.md docs/reference/community-config-presets.md
git commit -m "docs(web): describe preset script parameters"
```

### Task 6: Full verification, review, and temporary-doc cleanup

**Files:**
- Delete: `docs/superpowers/specs/2026-08-13-file-processor-preset-i18n-design.md`
- Delete: `docs/superpowers/plans/2026-08-13-file-processor-preset-i18n.md`

**Interfaces:**
- Verifies: all Web and repository contracts touched by the feature.
- Produces: a clean worktree with no completed temporary design or plan artifacts.

- [ ] **Step 1: Run the complete Web test suite**

Run: `cd web && pnpm test:run`

Expected: PASS with zero failed tests.

- [ ] **Step 2: Run static Web gates**

Run: `cd web && pnpm lint && pnpm typecheck`

Expected: both commands exit 0.

- [ ] **Step 3: Run the repository gate**

Run: `make check`

Expected: exit 0.

- [ ] **Step 4: Request a code review and address concrete findings**

Invoke `superpowers:requesting-code-review`, review the full implementation range after `41d0f8a`, and fix any correctness, compatibility, or test-quality findings. Re-run the narrowest affected test after each fix.

- [ ] **Step 5: Re-run completion verification**

Invoke `superpowers:verification-before-completion`, then run:

```bash
git diff --check 41d0f8a..HEAD
git status --short
```

Expected: no whitespace errors; only the two temporary documents remain scheduled for deletion if cleanup has not yet happened.

- [ ] **Step 6: Remove completed temporary documents**

Delete the design and implementation plan with `apply_patch`, then commit:

```bash
git add docs/superpowers/specs/2026-08-13-file-processor-preset-i18n-design.md docs/superpowers/plans/2026-08-13-file-processor-preset-i18n.md
git commit -m "docs: remove completed processor preset plans"
```

- [ ] **Step 7: Verify final repository state**

Run: `git diff --check HEAD^..HEAD && git status --short && git log -8 --oneline`

Expected: no output from the first two checks and a concise implementation commit series ending in the cleanup commit.
