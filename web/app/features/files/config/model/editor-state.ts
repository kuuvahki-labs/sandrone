import type {
  AdaptiveGroupOptions,
  AdaptiveGroupWarning,
} from "~/features/files/config/model/adaptive-groups";
import {
  type AddCatalogRuleSetRequest,
  type ConfigEditorDraft,
  type GroupDraft,
  isRecord,
  parseJSONList,
  type RuleDraft,
  type RuleSetDraft,
} from "~/features/files/config/model/editor-model";
import type { ConfigNamingLocale } from "~/features/files/config/model/naming";
import type {
  ConfigNodePreview,
  ConfigNodeSummary,
} from "~/features/files/config/model/node-source";
import {
  buildConfigRelationModel,
  type ConfigRelationModel,
} from "~/features/files/config/model/relations";
import type { ConfigTemplateID } from "~/features/files/config/model/templates";
import type { StructuredFileConfigurationAdapter } from "~/features/files/drivers/core/file-driver";
import type {
  FileAdaptiveGroupConfigDetail,
  FileConfigDraft,
} from "~/features/files/model/types";

export interface ConfigEditorStructureState {
  advancedGroupsText: string;
  advancedRuleSetsText: string;
  advancedRulesText: string;
  editorMode: "wizard" | "advanced";
  groupPreset: string;
  groups: GroupDraft[];
  ruleSetPreset: string;
  ruleSets: RuleSetDraft[];
  rules: RuleDraft[];
}

export interface ConfigEditorState {
  adaptiveEnabled: boolean;
  adaptiveOptions: AdaptiveGroupOptions;
  adaptiveOptionsChanged: boolean;
  adaptiveWarnings: AdaptiveGroupWarning[];
  formMode: "create" | "edit";
  namingLocale: ConfigNamingLocale;
  originalAdaptiveGroups?: FileAdaptiveGroupConfigDetail;
  originalRawSettings?: unknown;
  originalSubscriptions: string[];
  rawSettingsText: string;
  selectedSubscription: string;
  settingsMode: "structured" | "raw";
  structure: ConfigEditorStructureState;
  structureRevision: number;
  templateUndo: ConfigEditorStructureState | null;
}

export type ConfigEditorAction =
  | { type: "select-subscription"; name: string }
  | { type: "edit-raw-settings"; text: string }
  | { type: "replace-raw-with-structured" }
  | { type: "change-groups"; groups: GroupDraft[] }
  | { type: "change-rule-sets"; ruleSets: RuleSetDraft[] }
  | { type: "change-rules"; rules: RuleDraft[] }
  | { type: "change-advanced-groups"; text: string }
  | { type: "change-advanced-rule-sets"; text: string }
  | { type: "change-advanced-rules"; text: string }
  | { type: "change-adaptive-options"; options: AdaptiveGroupOptions };

export function initializeConfigEditorState(
  adapter: StructuredFileConfigurationAdapter,
  input: Readonly<{
    createNamingLocale?: ConfigNamingLocale;
    defaultValue?: ConfigEditorDraft | FileConfigDraft;
    formMode: "create" | "edit";
  }>,
): ConfigEditorState {
  const createNamingLocale = input.createNamingLocale ?? "en-US";
  const nativeDefault = isEditorDraft(input.defaultValue)
    ? adapter.toNativeDraft(input.defaultValue)
    : input.defaultValue;
  const namingLocale = adapter.templates.resolveNamingLocale(
    nativeDefault,
    createNamingLocale,
  );
  const initial = isEditorDraft(input.defaultValue)
    ? input.defaultValue
    : adapter.initialize(
      input.defaultValue
        ?? adapter.templates.create("standard", namingLocale),
      namingLocale,
    );
  const originalSubscriptions = [...(initial.subscriptions ?? [])];

  return {
    adaptiveEnabled: adapter.adaptive.initiallyEnabled(
      input.formMode,
      initial.adaptiveGroups,
    ),
    adaptiveOptions: adapter.adaptive.optionsFromConfig(initial.adaptiveGroups),
    adaptiveOptionsChanged: input.formMode === "create",
    adaptiveWarnings: [],
    formMode: input.formMode,
    namingLocale,
    originalAdaptiveGroups: initial.adaptiveGroups,
    originalRawSettings: initial.rawSettings,
    originalSubscriptions,
    rawSettingsText: JSON.stringify(initial.rawSettings ?? {}, null, 2),
    selectedSubscription: originalSubscriptions.length === 1
      ? originalSubscriptions[0]
      : "",
    settingsMode: initial.settingsMode,
    structure: structureFromDraft(initial),
    structureRevision: 0,
    templateUndo: null,
  };
}

export function reduceConfigEditorState(
  state: ConfigEditorState,
  event: ConfigEditorAction,
): ConfigEditorState {
  switch (event.type) {
    case "select-subscription":
      return { ...state, selectedSubscription: event.name };
    case "edit-raw-settings":
      return { ...state, rawSettingsText: event.text };
    case "replace-raw-with-structured":
      return {
        ...state,
        settingsMode: "structured",
      };
    case "change-groups":
      return updateStructureSection(
        state,
        { groups: event.groups },
      );
    case "change-rule-sets":
      return updateStructureSection(
        state,
        { ruleSets: event.ruleSets },
      );
    case "change-rules":
      return updateStructureSection(
        state,
        { rules: event.rules },
      );
    case "change-advanced-groups":
      return updateStructureSection(
        state,
        { advancedGroupsText: event.text },
      );
    case "change-advanced-rule-sets":
      return updateStructureSection(
        state,
        { advancedRuleSetsText: event.text },
      );
    case "change-advanced-rules":
      return updateStructureSection(
        state,
        { advancedRulesText: event.text },
      );
    case "change-adaptive-options":
      return {
        ...state,
        adaptiveEnabled: true,
        adaptiveOptions: event.options,
        adaptiveOptionsChanged: true,
        adaptiveWarnings: [],
      };
  }
}

export function applyConfigEditorTemplate(
  adapter: StructuredFileConfigurationAdapter,
  state: ConfigEditorState,
  templateID: ConfigTemplateID,
): ConfigEditorState {
  const next = adapter.initialize(
    adapter.templates.create(templateID, state.namingLocale),
    state.namingLocale,
  );

  return {
    ...state,
    adaptiveWarnings: [],
    structure: {
      ...structureFromDraft(next),
    },
    structureRevision: state.structureRevision + 1,
    templateUndo: copyStructure(state.structure),
  };
}

export function undoConfigEditorTemplate(
  state: ConfigEditorState,
): ConfigEditorState {
  if (!state.templateUndo) return state;

  return {
    ...state,
    adaptiveWarnings: [],
    structure: copyStructure(state.templateUndo),
    structureRevision: state.structureRevision + 1,
    templateUndo: null,
  };
}

export function applyConfigEditorAdaptiveGeneration(
  adapter: StructuredFileConfigurationAdapter,
  state: ConfigEditorState,
  input: Readonly<{
    nodeNames: readonly string[];
    options: AdaptiveGroupOptions;
  }>,
): { applied: boolean; state: ConfigEditorState } {
  const output = deriveConfigEditorOutput(adapter, state);
  const generation = adapter.adaptive.generate(
    input.nodeNames,
    input.options,
    state.namingLocale,
  );
  const result = adapter.adaptive.merge(
    output.effectiveAdaptiveConfig,
    generation,
  );
  const projectedGroups = adapter.groups.project(result.config.groups ?? []);
  if (!projectedGroups) return { applied: false, state };

  return {
    applied: true,
    state: {
      ...state,
      adaptiveEnabled: true,
      adaptiveOptions: input.options,
      adaptiveOptionsChanged: true,
      adaptiveWarnings: result.warnings,
      structure: {
        ...state.structure,
        groups: projectedGroups,
      },
      structureRevision: state.structureRevision + 1,
      templateUndo: null,
    },
  };
}

export function applyConfigEditorCatalogRuleSet(
  adapter: StructuredFileConfigurationAdapter,
  state: ConfigEditorState,
  request: Readonly<AddCatalogRuleSetRequest>,
) {
  const result = adapter.ruleSets.fromCatalog(
    request.entry,
    state.structure.ruleSets,
  );
  if (result.status !== "added") return { result, state };

  return {
    result,
    state: {
      ...state,
      structure: {
        ...state.structure,
        ruleSets: result.ruleSets,
      },
      templateUndo: null,
    },
  };
}

export function deriveConfigEditorOutput(
  adapter: StructuredFileConfigurationAdapter,
  state: ConfigEditorState,
) {
  const rawSettings = parseJSONObject(state.rawSettingsText);
  const structuredDraft = structuredDraftFromState(adapter, state);
  const multipleSubscriptions = state.originalSubscriptions.length > 1;
  const envelopeSubscriptions = multipleSubscriptions
    ? state.originalSubscriptions
    : state.selectedSubscription
      ? [state.selectedSubscription]
      : [];
  const nativeConfig = adapter.toNativeDraft({
    ...structuredDraft,
    subscriptions: envelopeSubscriptions,
  });
  const effectiveAdaptiveConfig = nativeConfig;
  const encoded = adapter.encode({
    ...structuredDraft,
    subscriptions: envelopeSubscriptions,
    settingsMode: state.settingsMode,
    rawSettings: rawSettings.value ?? state.originalRawSettings ?? {},
  });
  const serialized = JSON.stringify(encoded);
  if (serialized === undefined) {
    throw new TypeError("structured adapter returned a non-serializable envelope");
  }

  return {
    structuredDraft,
    envelopeSubscriptions,
    nativeConfig,
    effectiveAdaptiveConfig,
    encoded,
    serialized,
    multipleSubscriptions,
    rawSettingsError: rawSettings.error,
    rawSettingsValid: rawSettings.error === undefined && adapter.validateSettings(rawSettings.value),
  };
}

export function deriveConfigEditorValidity(
  adapter: StructuredFileConfigurationAdapter,
  state: ConfigEditorState,
  output: ReturnType<typeof deriveConfigEditorOutput>,
  preview: Readonly<{
    currentPreview: ConfigNodePreview | null;
    projectedNodes: readonly ConfigNodeSummary[] | null;
  }>,
) {
  const previewValidation = adapter.preview.validate({
    formMode: state.formMode,
    preview: preview.currentPreview,
    projectedNodes: preview.projectedNodes,
    selected: Boolean(state.selectedSubscription),
  });
  const relationModel = state.structure.editorMode === "wizard"
    ? wizardRelationModel(adapter, state, output, preview.projectedNodes)
    : emptyRelationModel();
  const structureValid = state.structure.editorMode === "advanced"
    ? !parseJSONList(state.structure.advancedGroupsText).error
      && !parseJSONList(state.structure.advancedRuleSetsText).error
      && !parseJSONList(state.structure.advancedRulesText).error
    : !relationModel.issues.some((issue) => issue.severity === "error");
  const adaptiveStale = adapter.adaptive.isStale({
    config: output.effectiveAdaptiveConfig,
    editorMode: state.structure.editorMode,
    enabled: state.adaptiveEnabled,
    namingLocale: state.namingLocale,
    nodeNames: preview.projectedNodes?.map((node) => node.name),
    options: state.adaptiveOptions,
  });
  const valid = !output.multipleSubscriptions
    && previewValidation.valid
    && (state.settingsMode === "raw"
      ? output.rawSettingsValid
      : structureValid && !adaptiveStale);

  return {
    adaptiveStale,
    previewValidation,
    relationModel,
    structureValid,
    valid,
  };
}

function wizardRelationModel(
  adapter: StructuredFileConfigurationAdapter,
  state: Readonly<ConfigEditorState>,
  output: ReturnType<typeof deriveConfigEditorOutput>,
  projectedNodes: readonly ConfigNodeSummary[] | null,
): ConfigRelationModel {
  const base = buildConfigRelationModel(adapter.relations.project(
    adapter.groups.serialize(state.structure.groups),
    adapter.ruleSets.serialize(state.structure.ruleSets),
    adapter.rules.serialize(state.structure.rules),
    adapter.preview.relationNodeNames(
      projectedNodes,
      Boolean(state.selectedSubscription),
    ),
  ));
  return {
    ...base,
    issues: [
      ...base.issues,
      ...adapter.validate(output.structuredDraft),
    ],
  };
}

function emptyRelationModel(): ConfigRelationModel {
  return {
    groupInboundReferences: {},
    ruleSetInboundReferences: {},
    issues: [],
  };
}

function structureFromDraft(
  draft: Readonly<ConfigEditorDraft>,
): ConfigEditorStructureState {
  return {
    advancedGroupsText: draft.advancedGroupsText,
    advancedRuleSetsText: draft.advancedRuleSetsText,
    advancedRulesText: draft.advancedRulesText,
    editorMode: draft.mode,
    groupPreset: draft.groupPreset,
    groups: draft.groups,
    ruleSetPreset: draft.ruleSetPreset,
    ruleSets: draft.ruleSets,
    rules: draft.rules,
  };
}

function copyStructure(
  structure: Readonly<ConfigEditorStructureState>,
): ConfigEditorStructureState {
  return {
    ...structure,
    groups: [...structure.groups],
    ruleSets: [...structure.ruleSets],
    rules: [...structure.rules],
  };
}

function updateStructureSection(
  state: ConfigEditorState,
  patch: Partial<ConfigEditorStructureState>,
): ConfigEditorState {
  return {
    ...state,
    structure: {
      ...state.structure,
      ...patch,
    },
  };
}

function structuredDraftFromState(
  adapter: StructuredFileConfigurationAdapter,
  state: Readonly<ConfigEditorState>,
): ConfigEditorDraft {
  const { structure } = state;
  return {
    subscriptions: [],
    settingsMode: "structured",
    rawSettings: state.originalRawSettings,
    adaptiveGroups: state.adaptiveEnabled
      ? state.adaptiveOptionsChanged
        ? adapter.adaptive.configFromOptions(state.adaptiveOptions)
        : state.originalAdaptiveGroups
      : undefined,
    advancedGroupsText: structure.advancedGroupsText,
    advancedRuleSetsText: structure.advancedRuleSetsText,
    advancedRulesText: structure.advancedRulesText,
    groupPreset: structure.groupPreset,
    groups: structure.groups,
    mode: structure.editorMode,
    ruleSetPreset: structure.ruleSetPreset,
    ruleSets: structure.ruleSets,
    rules: structure.rules,
  };
}

function parseJSONObject(
  text: string,
): {
  error?: "invalid-json" | "not-object";
  value?: Record<string, unknown>;
} {
  try {
    const value = JSON.parse(text) as unknown;
    return isRecord(value) ? { value } : { error: "not-object" };
  } catch {
    return { error: "invalid-json" };
  }
}

function isEditorDraft(
  value: ConfigEditorDraft | FileConfigDraft | undefined,
): value is ConfigEditorDraft {
  return Boolean(
    value
      && "advancedGroupsText" in value,
  );
}
