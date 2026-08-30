import {
  type ConfigEditorDraft,
  configJSON,
  type ConfigMap,
  isRecord,
  parseJSONList,
} from "~/features/files/config/model/editor-model";
import type { ConfigNamingLocale } from "~/features/files/config/model/naming";
import type { FileConfigDetail, FileConfigDraft, RuleSetCatalogTarget } from "~/features/files/model/types";

import type { StructuredFileConfigurationAdapter } from "./file-driver";

export interface StructuredConfigurationAdapterSpec {
	adaptive: StructuredFileConfigurationAdapter["adaptive"];
	catalogTarget?: RuleSetCatalogTarget;
  decodeSettings: (value: unknown) => Partial<FileConfigDraft> | null;
  groups: StructuredFileConfigurationAdapter["groups"];
	kind: string;
	preview: StructuredFileConfigurationAdapter["preview"];
	references: StructuredFileConfigurationAdapter["references"];
	relations: StructuredFileConfigurationAdapter["relations"];
  ruleSets: StructuredFileConfigurationAdapter["ruleSets"];
  rules: StructuredFileConfigurationAdapter["rules"];
  defaults: {
    ruleSets: () => ConfigMap[];
    rules: (namingLocale: ConfigNamingLocale) => unknown[];
  };
	templates: StructuredFileConfigurationAdapter["templates"];
	validate: StructuredFileConfigurationAdapter["validate"];
}

export function createStructuredConfigurationAdapter(
  spec: StructuredConfigurationAdapterSpec,
): StructuredFileConfigurationAdapter {
  const initialize = (draft?: FileConfigDraft, namingLocale: ConfigNamingLocale = "en-US"): ConfigEditorDraft => {
    const value = draft ?? {};
      const groupPreset = value.group_preset || "basic";
    const projectedGroups = value.groups !== undefined
      ? spec.groups.project(value.groups)
      : spec.groups.defaults(groupPreset, namingLocale);
    const nativeGroups = value.groups ?? spec.groups.serialize(projectedGroups ?? []);
    const nativeRuleSets = value.rule_sets ?? spec.defaults.ruleSets();
    const nativeRules = value.rules ?? spec.defaults.rules(namingLocale);
    const ruleSets = spec.ruleSets.project(nativeRuleSets);
    const rules = spec.rules.project(nativeRules);
    return {
      subscriptions: value.subscriptions,
      settingsMode: value.settingsMode === "raw" ? "raw" : "structured",
      rawSettings: value.rawSettings,
      adaptiveGroups: value.adaptive_groups,
      advancedGroupsText: configJSON(nativeGroups),
      advancedRuleSetsText: configJSON(nativeRuleSets),
      advancedRulesText: configJSON(nativeRules),
      groupPreset,
      groups: projectedGroups ?? [],
      mode: projectedGroups !== null && ruleSets !== null && rules !== null ? "wizard" : "advanced",
      ruleSetPreset: value.ruleset_preset || "default",
      ruleSets: ruleSets ?? [],
      rules: rules ?? [],
    };
  };

  const toNativeDraft = (draft: ConfigEditorDraft): FileConfigDraft => {
    if (draft.settingsMode === "raw") {
      return {
        subscriptions: draft.subscriptions,
        settingsMode: "raw",
        rawSettings: draft.rawSettings,
      };
    }
    const advancedGroups = parseJSONList(draft.advancedGroupsText).value?.filter(isRecord);
    const advancedRuleSets = parseJSONList(draft.advancedRuleSetsText).value?.filter(isRecord);
    const advancedRules = parseJSONList(draft.advancedRulesText).value;
    return {
      subscriptions: draft.subscriptions,
      group_preset: draft.groupPreset,
      ruleset_preset: draft.ruleSetPreset,
      adaptive_groups: draft.adaptiveGroups,
      groups: draft.mode === "wizard" ? spec.groups.serialize(draft.groups) : advancedGroups,
      rule_sets: draft.mode === "wizard" ? spec.ruleSets.serialize(draft.ruleSets) : advancedRuleSets,
      rules: draft.mode === "wizard" ? spec.rules.serialize(draft.rules) : advancedRules,
    };
  };

	const adapter: StructuredFileConfigurationAdapter = {
		adaptive: spec.adaptive,
		kind: spec.kind,
    catalogTarget: spec.catalogTarget,
    decode(config?: FileConfigDetail, namingLocale: ConfigNamingLocale = "en-US") {
      if (!config) return undefined;
      if (!config.settingsPresent) {
        return initialize({ subscriptions: config.subscriptions, settingsMode: "structured" }, namingLocale);
      }
      const decoded = spec.decodeSettings(config.settings);
      return decoded
        ? initialize({ subscriptions: config.subscriptions, settingsMode: "structured", ...decoded }, namingLocale)
        : initialize({
          subscriptions: config.subscriptions,
          settingsMode: "raw",
          rawSettings: config.settings,
        }, namingLocale);
    },
    encode(draft) {
      const native = toNativeDraft(draft);
      const settings = native.settingsMode === "raw"
        ? native.rawSettings
        : Object.fromEntries(Object.entries({
          adaptive_groups: native.adaptive_groups,
          groups: native.groups,
          rule_sets: native.rule_sets,
          rules: native.rules,
        }).filter(([, value]) => value !== undefined));
      return {
        ...(draft.subscriptions?.length ? { subscriptions: draft.subscriptions } : {}),
        settings,
      };
    },
    initialize,
		toNativeDraft,
		groups: spec.groups,
		preview: spec.preview,
		references: spec.references,
		relations: spec.relations,
		ruleSets: spec.ruleSets,
		rules: spec.rules,
		templates: spec.templates,
		validateSettings(settings) {
			return completeSettingsObject(settings) && spec.decodeSettings(settings) !== null;
		},
		validate: spec.validate,
  };
  return Object.freeze(adapter);
}

function completeSettingsObject(value: unknown): value is ConfigMap {
  if (!isRecord(value)) return false;
  return ["groups", "rule_sets", "rules"].every((name) => (
    Object.hasOwn(value, name) && Array.isArray(value[name])
  ));
}

export function strictSettingsObject(
  value: unknown,
  allowedKeys: readonly string[],
): ConfigMap | null {
  if (!isRecord(value)) return null;
  const allowed = new Set(allowedKeys);
  return Object.keys(value).some((key) => !allowed.has(key)) ? null : value;
}

export function recordArray(value: unknown): value is ConfigMap[] {
  return Array.isArray(value) && value.every(isRecord);
}

export function adaptiveGroups(value: unknown): FileConfigDraft["adaptive_groups"] {
  return isRecord(value)
    ? { ...value } as FileConfigDraft["adaptive_groups"]
    : undefined;
}

export function omitKeys(value: ConfigMap, keys: readonly string[]): ConfigMap {
  const omitted = new Set(keys);
  return Object.fromEntries(Object.entries(value).filter(([key]) => !omitted.has(key)));
}

export function stateRecord(value: unknown): ConfigMap {
  return isRecord(value) ? value : {};
}
