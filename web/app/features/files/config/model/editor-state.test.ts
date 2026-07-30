import { describe, expect, it } from "vitest";

import { configNodePreviewFromSubscription } from "~/features/files/config/model/node-source";
import { requireFileDriver } from "~/features/files/drivers/registry";

import {
  applyConfigEditorAdaptiveGeneration,
  applyConfigEditorCatalogRuleSet,
  applyConfigEditorTemplate,
  deriveConfigEditorOutput,
  deriveConfigEditorValidity,
  initializeConfigEditorState,
  reduceConfigEditorState,
  undoConfigEditorTemplate,
} from "./editor-state";

describe("config editor state initialization and output", () => {
  it("preserves omitted sections, partial adaptive metadata, and legacy multi-subscriptions", () => {
    const adapter = structuredAdapter("mihomo");
    const state = initializeConfigEditorState(adapter, {
      defaultValue: {
        subscriptions: ["one", "two"],
        settingsMode: "structured",
        adaptive_groups: { regions: ["hk"] },
      },
      formMode: "edit",
    });

    const output = deriveConfigEditorOutput(adapter, state);

    expect(state.structure.sectionPresence).toEqual({
      groups: false,
      ruleSets: false,
      rules: false,
    });
    expect(state.structure.groups.length).toBeGreaterThan(0);
    expect(output.multipleSubscriptions).toBe(true);
    expect(output.encoded).toEqual({
      subscriptions: ["one", "two"],
      settings: {
        adaptive_groups: { regions: ["hk"] },
      },
    });
    expect(JSON.parse(output.serialized)).toEqual(output.encoded);
  });

  it("keeps explicit empty structured sections in the serialized envelope", () => {
    const adapter = structuredAdapter("mihomo");
    const state = initializeConfigEditorState(adapter, {
      defaultValue: {
        settingsMode: "structured",
        groups: [],
        rule_sets: [],
        rules: [],
      },
      formMode: "edit",
    });

    const output = deriveConfigEditorOutput(adapter, state);

    expect(state.structure.sectionPresence).toEqual({
      groups: true,
      ruleSets: true,
      rules: true,
    });
    expect(output.encoded).toEqual({
      settings: {
        groups: [],
        rule_sets: [],
        rules: [],
      },
    });
  });

  it("uses the original raw settings as the hidden-output fallback while edits are invalid", () => {
    const adapter = structuredAdapter("mihomo");
    const initial = initializeConfigEditorState(adapter, {
      defaultValue: {
        subscriptions: ["provider"],
        settingsMode: "raw",
        rawSettings: { future: { keep: true } },
      },
      formMode: "edit",
    });

    const invalidJSON = deriveConfigEditorOutput(
      adapter,
      reduceConfigEditorState(initial, {
        type: "edit-raw-settings",
        text: "{",
      }),
    );
    const nonObject = deriveConfigEditorOutput(
      adapter,
      reduceConfigEditorState(initial, {
        type: "edit-raw-settings",
        text: "[]",
      }),
    );
    const validEdit = deriveConfigEditorOutput(
      adapter,
      reduceConfigEditorState(initial, {
        type: "edit-raw-settings",
        text: '{"future":{"keep":false}}',
      }),
    );

    expect(invalidJSON.rawSettingsError).toBe("invalid-json");
    expect(invalidJSON.encoded).toEqual({
      subscriptions: ["provider"],
      settings: { future: { keep: true } },
    });
    expect(nonObject.rawSettingsError).toBe("not-object");
    expect(nonObject.encoded).toEqual(invalidJSON.encoded);
    expect(validEdit.rawSettingsError).toBeUndefined();
    expect(validEdit.encoded).toEqual({
      subscriptions: ["provider"],
      settings: { future: { keep: false } },
    });
  });

  it("leaves advanced native lists to the structured adapter", () => {
    const adapter = structuredAdapter("sing-box");
    const nativeSettings = {
      groups: [],
      rule_sets: [{
        type: "remote",
        tag: "private",
        format: "source",
        url: "https://example.com/private.json",
        http_client: "rules-client",
      }],
      rules: [{ rule_set: ["private"], outbound: "direct" }],
    };
    const state = initializeConfigEditorState(adapter, {
      defaultValue: {
        settingsMode: "structured",
        ...nativeSettings,
      },
      formMode: "edit",
    });

    expect(state.structure.editorMode).toBe("advanced");
    expect(deriveConfigEditorOutput(adapter, state).encoded).toEqual({
      settings: nativeSettings,
    });
  });

  it("lets a single subscription selection change while preserving a legacy multi-selection", () => {
    const adapter = structuredAdapter("mihomo");
    const single = initializeConfigEditorState(adapter, {
      defaultValue: {
        subscriptions: ["provider"],
        settingsMode: "structured",
        groups: [],
        rule_sets: [],
        rules: [],
      },
      formMode: "edit",
    });
    const multiple = initializeConfigEditorState(adapter, {
      defaultValue: {
        subscriptions: ["one", "two"],
        settingsMode: "structured",
        groups: [],
        rule_sets: [],
        rules: [],
      },
      formMode: "edit",
    });

    expect(deriveConfigEditorOutput(adapter, single).envelopeSubscriptions).toEqual(["provider"]);
    expect(deriveConfigEditorOutput(
      adapter,
      reduceConfigEditorState(single, {
        type: "select-subscription",
        name: "",
      }),
    ).envelopeSubscriptions).toEqual([]);
    expect(deriveConfigEditorOutput(
      adapter,
      reduceConfigEditorState(multiple, {
        type: "select-subscription",
        name: "ignored",
      }),
    ).envelopeSubscriptions).toEqual(["one", "two"]);
  });
});

describe("config editor structure transitions", () => {
  it("keeps the create naming locale across later template applications", () => {
    const adapter = structuredAdapter("mihomo");
    const initial = initializeConfigEditorState(adapter, {
      createNamingLocale: "zh-CN",
      formMode: "create",
    });

    const applied = applyConfigEditorTemplate(
      adapter,
      initial,
      "minimal",
    );
    const output = deriveConfigEditorOutput(adapter, applied);

    expect(initial.namingLocale).toBe("zh-CN");
    expect(output.nativeConfig.groups?.map((group) => group.name))
      .toContain("🚀 节点选择");
    expect(output.nativeConfig.rules?.at(-1)).toBe("MATCH,🐟 漏网之鱼");
  });

  it("applies templates with the captured naming locale and undoes only the structure", () => {
    const adapter = structuredAdapter("mihomo");
    const minimal = adapter.templates.create("minimal", "zh-CN");
    const generated = adapter.adaptive.generate(
      ["HK-01", "香港-02"],
      adapter.adaptive.defaultOptions(),
      "zh-CN",
    );
    const startingConfig = {
      ...adapter.adaptive.merge(minimal, generated).config,
      subscriptions: ["provider"],
    };
    const initial = initializeConfigEditorState(adapter, {
      defaultValue: startingConfig,
      formMode: "edit",
    });
    const selectionCleared = reduceConfigEditorState(initial, {
      type: "select-subscription",
      name: "",
    });

    const applied = applyConfigEditorTemplate(
      adapter,
      {
        ...selectionCleared,
        adaptiveWarnings: [{ code: "anchor_missing" }],
      },
      "standard",
    );

    expect(applied.namingLocale).toBe("zh-CN");
    expect(applied.selectedSubscription).toBe("");
    expect(applied.structure.sectionPresence).toEqual({
      groups: true,
      ruleSets: true,
      rules: true,
    });
    expect(applied.structure.groups.map((group) => group.name)).toContain("🤖 AI 服务");
    expect(applied.structure.groups.map((group) => group.name)).not.toContain("Hong Kong");
    expect(applied.templateUndo).toEqual(selectionCleared.structure);
    expect(applied.adaptiveWarnings).toEqual([]);
    expect(applied.structureRevision).toBe(1);

    const restored = undoConfigEditorTemplate(applied);

    expect(restored.selectedSubscription).toBe("");
    expect(restored.structure).toEqual(selectionCleared.structure);
    expect(restored.templateUndo).toBeNull();
    expect(restored.adaptiveWarnings).toEqual([]);
    expect(restored.structureRevision).toBe(2);
    expect(deriveConfigEditorOutput(adapter, restored).nativeConfig.groups)
      .toEqual(startingConfig.groups);
  });

  it("leaves state unchanged when template undo is unavailable", () => {
    const adapter = structuredAdapter("mihomo");
    const state = initializeConfigEditorState(adapter, {
      defaultValue: adapter.templates.create("minimal"),
      formMode: "edit",
    });

    expect(undoConfigEditorTemplate(state)).toBe(state);
  });

  it("marks each directly edited structure section present", () => {
    const adapter = structuredAdapter("mihomo");
    const initial = initializeConfigEditorState(adapter, {
      defaultValue: { settingsMode: "structured" },
      formMode: "edit",
    });

    const groups = reduceConfigEditorState(initial, {
      type: "change-groups",
      groups: [],
    });
    const ruleSets = reduceConfigEditorState(initial, {
      type: "change-rule-sets",
      ruleSets: [],
    });
    const rules = reduceConfigEditorState(initial, {
      type: "change-rules",
      rules: [],
    });
    const advancedGroups = reduceConfigEditorState(initial, {
      type: "change-advanced-groups",
      text: '[{"name":"Custom"}]',
    });
    const advancedRuleSets = reduceConfigEditorState(initial, {
      type: "change-advanced-rule-sets",
      text: '[{"name":"remote"}]',
    });
    const advancedRules = reduceConfigEditorState(initial, {
      type: "change-advanced-rules",
      text: '["MATCH,DIRECT"]',
    });

    expect(groups.structure.sectionPresence).toEqual({
      groups: true,
      ruleSets: false,
      rules: false,
    });
    expect(ruleSets.structure.sectionPresence).toEqual({
      groups: false,
      ruleSets: true,
      rules: false,
    });
    expect(rules.structure.sectionPresence).toEqual({
      groups: false,
      ruleSets: false,
      rules: true,
    });
    expect(advancedGroups.structure.advancedGroupsText).toBe('[{"name":"Custom"}]');
    expect(advancedGroups.structure.sectionPresence.groups).toBe(true);
    expect(advancedRuleSets.structure.advancedRuleSetsText).toBe('[{"name":"remote"}]');
    expect(advancedRuleSets.structure.sectionPresence.ruleSets).toBe(true);
    expect(advancedRules.structure.advancedRulesText).toBe('["MATCH,DIRECT"]');
    expect(advancedRules.structure.sectionPresence.rules).toBe(true);
  });

  it("replaces raw settings with explicitly present structured sections", () => {
    const adapter = structuredAdapter("mihomo");
    const state = initializeConfigEditorState(adapter, {
      defaultValue: {
        settingsMode: "raw",
        rawSettings: { future: true },
      },
      formMode: "edit",
    });

    const replaced = reduceConfigEditorState(state, {
      type: "replace-raw-with-structured",
    });

    expect(replaced.settingsMode).toBe("structured");
    expect(replaced.structure.sectionPresence).toEqual({
      groups: true,
      ruleSets: true,
      rules: true,
    });
  });
});

describe("config editor adaptive and catalog transitions", () => {
  it.each(["mihomo", "shadowrocket"])(
    "applies five default adaptive groups for %s and persists the selected options",
    (kind) => {
      const adapter = structuredAdapter(kind);
      const initial = initializeConfigEditorState(adapter, {
        defaultValue: adapter.templates.create("minimal"),
        formMode: "edit",
      });
      const options = adapter.adaptive.defaultOptions();

      const first = applyConfigEditorAdaptiveGeneration(adapter, initial, {
        nodeNames: [],
        options,
      });
      const output = deriveConfigEditorOutput(adapter, first.state);

      expect(first.applied).toBe(true);
      expect(first.state.adaptiveEnabled).toBe(true);
      expect(first.state.adaptiveOptionsChanged).toBe(true);
      expect(first.state.structure.sectionPresence.groups).toBe(true);
      expect(first.state.structureRevision).toBe(1);
      expect(adapter.adaptive.canonicalNames(output.nativeConfig.groups ?? []))
        .toEqual([
          "Hong Kong",
          "Taiwan",
          "Singapore",
          "Japan",
          "United States",
        ]);
      expect(output.encoded).toMatchObject({
        settings: {
          adaptive_groups: {
            type: options.type,
            regions: ["hk", "tw", "sg", "jp", "us"],
          },
        },
      });

      const repeated = applyConfigEditorAdaptiveGeneration(
        adapter,
        first.state,
        { nodeNames: [], options },
      );
      expect(repeated.applied).toBe(true);
      expect(repeated.state.structureRevision).toBe(2);
    },
  );

  it("uses omitted Shadowrocket runtime defaults as the adaptive anchor", () => {
    const adapter = structuredAdapter("shadowrocket");
    const initial = initializeConfigEditorState(adapter, {
      defaultValue: {
        subscriptions: ["provider"],
        settingsMode: "structured",
      },
      formMode: "edit",
    });

    expect(initial.structure.sectionPresence.groups).toBe(false);
    expect(deriveConfigEditorOutput(adapter, initial).encoded).toEqual({
      subscriptions: ["provider"],
      settings: {},
    });

    const transition = applyConfigEditorAdaptiveGeneration(adapter, initial, {
      nodeNames: ["HK-01", "香港-02"],
      options: adapter.adaptive.defaultOptions(),
    });
    const output = deriveConfigEditorOutput(adapter, transition.state);

    expect(transition.applied).toBe(true);
    expect(output.nativeConfig.groups).toEqual(expect.arrayContaining([
      expect.objectContaining({ name: "Proxy", type: "select" }),
      expect.objectContaining({
        name: "Hong Kong",
        "policy-regex-filter": expect.any(String),
      }),
    ]));
  });

  it("does not change state when generated groups cannot be projected", () => {
    const base = structuredAdapter("mihomo");
    const adapter = {
      ...base,
      groups: {
        ...base.groups,
        project: () => null,
      },
    };
    const state = initializeConfigEditorState(base, {
      defaultValue: base.templates.create("minimal"),
      formMode: "edit",
    });

    const transition = applyConfigEditorAdaptiveGeneration(adapter, state, {
      nodeNames: [],
      options: base.adaptive.defaultOptions(),
    });

    expect(transition).toEqual({ applied: false, state });
    expect(transition.state).toBe(state);
  });

  it("persists changed adaptive options without clearing template undo", () => {
    const adapter = structuredAdapter("mihomo");
    const initial = initializeConfigEditorState(adapter, {
      defaultValue: adapter.templates.create("minimal"),
      formMode: "edit",
    });
    const withUndo = applyConfigEditorTemplate(adapter, initial, "standard");
    const options = {
      enabledRegionIds: ["hk"],
      type: "load-balance",
    };

    const changed = reduceConfigEditorState(withUndo, {
      type: "change-adaptive-options",
      options,
    });

    expect(changed.adaptiveEnabled).toBe(true);
    expect(changed.adaptiveOptionsChanged).toBe(true);
    expect(changed.adaptiveWarnings).toEqual([]);
    expect(changed.templateUndo).toEqual(withUndo.templateUndo);
    expect(deriveConfigEditorOutput(adapter, changed).encoded).toMatchObject({
      settings: {
        adaptive_groups: {
          type: "load-balance",
          regions: ["hk"],
        },
      },
    });
  });

  it("adds a catalog rule set without changing rules and treats conflicts as no-ops", () => {
    const adapter = structuredAdapter("mihomo");
    const initial = initializeConfigEditorState(adapter, {
      defaultValue: adapter.templates.create("minimal"),
      formMode: "edit",
    });
    const withUndo = applyConfigEditorTemplate(adapter, initial, "standard");
    const request = {
      entry: {
        name: "geosite-cn",
        ruleKind: "domain",
        url: "https://raw.githubusercontent.com/example/catalog/main/geosite-cn.mrs",
      },
    } as const;

    const added = applyConfigEditorCatalogRuleSet(adapter, withUndo, request);

    expect(added.result.status).toBe("added");
    expect(added.state.structure.rules).toEqual(withUndo.structure.rules);
    expect(added.state.structure.ruleSets).toHaveLength(
      withUndo.structure.ruleSets.length + 1,
    );
    expect(added.state.structure.sectionPresence.ruleSets).toBe(true);
    expect(added.state.templateUndo).toBeNull();

    const duplicate = applyConfigEditorCatalogRuleSet(
      adapter,
      added.state,
      request,
    );
    expect(duplicate.result.status).toBe("duplicate-url");
    expect(duplicate.state).toBe(added.state);
  });
});

describe("config editor derived validity", () => {
  it("uses raw object parsing instead of structured issues while raw settings are active", () => {
    const adapter = structuredAdapter("mihomo");
    const initial = initializeConfigEditorState(adapter, {
      defaultValue: {
        settingsMode: "raw",
        rawSettings: { future: true },
      },
      formMode: "edit",
    });
    const invalid = reduceConfigEditorState(initial, {
      type: "edit-raw-settings",
      text: "[]",
    });

    expect(validity(adapter, initial).valid).toBe(true);
    expect(validity(adapter, invalid)).toMatchObject({
      structureValid: true,
      valid: false,
    });
  });

  it("validates all three advanced JSON arrays", () => {
    const adapter = structuredAdapter("sing-box");
    const state = initializeConfigEditorState(adapter, {
      defaultValue: {
        settingsMode: "structured",
        groups: [],
        rule_sets: [{
          type: "remote",
          tag: "private",
          format: "source",
          url: "https://example.com/private.json",
          http_client: "rules-client",
        }],
        rules: [{ rule_set: ["private"], outbound: "direct" }],
      },
      formMode: "edit",
    });
    const invalid = reduceConfigEditorState(state, {
      type: "change-advanced-rules",
      text: "{}",
    });

    expect(state.structure.editorMode).toBe("advanced");
    expect(validity(adapter, state).valid).toBe(true);
    expect(validity(adapter, invalid)).toMatchObject({
      structureValid: false,
      valid: false,
    });
  });

  it("keeps the default Shadowrocket template valid without a subscription", () => {
    const adapter = structuredAdapter("shadowrocket");
    const state = initializeConfigEditorState(adapter, {
      formMode: "create",
    });

    expect(validity(adapter, state)).toMatchObject({
      adaptiveStale: false,
      structureValid: true,
      valid: true,
    });
  });

  it("merges adapter validation into wizard relation issues", () => {
    const adapter = structuredAdapter("mihomo");
    const state = initializeConfigEditorState(adapter, {
      defaultValue: {
        groups: [{
          name: "Proxy",
          type: "select",
          proxies: ["DIRECT"],
        }],
        rule_sets: [{
          name: "remote",
          type: "http",
          behavior: "domain",
          format: "mrs",
          interval: 86400,
          url: "not-a-url",
        }],
        rules: [],
      },
      formMode: "edit",
    });

    const result = validity(adapter, state);

    expect(result.relationModel.issues).toEqual(expect.arrayContaining([
      expect.objectContaining({
        section: "rule_sets",
        severity: "error",
      }),
    ]));
    expect(result.structureValid).toBe(false);
    expect(result.valid).toBe(false);
  });

  it("rejects a Shadowrocket nodes-only group without a selected subscription", () => {
    const adapter = structuredAdapter("shadowrocket");
    const state = initializeConfigEditorState(adapter, {
      defaultValue: {
        groups: [{
          name: "Proxy",
          type: "select",
          proxies: ["$nodes"],
        }],
        rule_sets: [],
        rules: [],
      },
      formMode: "edit",
    });

    expect(validity(adapter, state)).toMatchObject({
      structureValid: false,
      valid: false,
    });
  });

  it("rejects legacy multiple subscriptions independently of structured validity", () => {
    const adapter = structuredAdapter("mihomo");
    const state = initializeConfigEditorState(adapter, {
      defaultValue: {
        subscriptions: ["one", "two"],
        groups: [],
        rule_sets: [],
        rules: [],
      },
      formMode: "edit",
    });

    expect(validity(adapter, state)).toMatchObject({
      structureValid: true,
      valid: false,
    });
  });

  it("derives adaptive staleness from current preview projection without mutating state", () => {
    const adapter = structuredAdapter("sing-box");
    const initial = initializeConfigEditorState(adapter, {
      defaultValue: {
        ...adapter.templates.create("minimal"),
        subscriptions: ["provider"],
      },
      formMode: "edit",
    });
    const generated = applyConfigEditorAdaptiveGeneration(adapter, initial, {
      nodeNames: ["HK-01"],
      options: adapter.adaptive.defaultOptions(),
    });
    const preview = configNodePreviewFromSubscription({
      subscriptionName: "provider",
      nodes: [{
        identity: "node-1",
        after: {
          name: "HK-01",
          type: "ss",
          endpoint: "node-1.example:8388",
        },
      }],
      warnings: [],
    });
    const projectedNodes = adapter.preview.projectNodes(preview);
    const snapshot = structuredClone(generated.state);
    const output = deriveConfigEditorOutput(adapter, generated.state);

    const stale = deriveConfigEditorValidity(
      adapter,
      generated.state,
      output,
      { currentPreview: null, projectedNodes: null },
    );
    const current = deriveConfigEditorValidity(
      adapter,
      generated.state,
      output,
      { currentPreview: preview, projectedNodes },
    );

    expect(stale).toMatchObject({ adaptiveStale: true, valid: false });
    expect(current).toMatchObject({ adaptiveStale: false, valid: true });
    expect(generated.state).toEqual(snapshot);
  });
});

function structuredAdapter(kind: string) {
  const configuration = requireFileDriver(kind).configuration;
  if (configuration.mode !== "structured") {
    throw new Error(`expected structured driver: ${kind}`);
  }
  return configuration.adapter;
}

function validity(
  adapter: ReturnType<typeof structuredAdapter>,
  state: ReturnType<typeof initializeConfigEditorState>,
) {
  return deriveConfigEditorValidity(
    adapter,
    state,
    deriveConfigEditorOutput(adapter, state),
    { currentPreview: null, projectedNodes: null },
  );
}
