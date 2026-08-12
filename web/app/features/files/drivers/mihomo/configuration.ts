import type { ConfigMap, GroupDraft, RuleDraft, RuleSetDraft } from "~/features/files/config/model/editor-model";
import { stringField } from "~/features/files/config/model/editor-model";
import {
  configAnchorName,
  configAutoName,
  configCustomGroupName,
  configGroupName,
  type ConfigNamingLocale,
  configRegionName,
} from "~/features/files/config/model/naming";
import {
  catalogResult,
  draftID,
  hasOnlyKeys,
  issue,
  nativeValuesEqual,
  positiveInteger,
  ruleLines,
  scalarString,
  stringList,
  validateDraft,
} from "~/features/files/drivers/core/adapter-helpers";
import type { StructuredFileConfigurationAdapter } from "~/features/files/drivers/core/file-driver";
import {
  adaptiveGroups,
  createStructuredConfigurationAdapter,
  omitKeys,
  recordArray,
  stateRecord,
  strictSettingsObject,
} from "~/features/files/drivers/core/structured-adapter";
import type { FileConfigDraft } from "~/features/files/model/types";
import type { Translator } from "~/shared/i18n/context";
import { DEFAULT_PROBE_URL } from "~/shared/probe/defaults";

import { mihomoConfigurationStrategies } from "./strategies";

const RULE_SET_EXTENSIONS: Readonly<Record<string, string>> = { mrs: "mrs", text: "list", yaml: "yaml" };
const OFFICIAL_RULE_SET_URL = /^(https:\/\/raw\.githubusercontent\.com\/MetaCubeX\/meta-rules-dat\/meta\/geo\/(?:geosite|geoip)\/[^/?#]+)\.(?:mrs|yaml|list)$/;
const GROUP_TYPE_OPTIONS = [
  { value: "select", label: "select" },
  { value: "url-test", label: "url-test" },
  { value: "fallback", label: "fallback" },
  { value: "load-balance", label: "load-balance" },
] as const;

const groups = mihomoGroups();
const ruleSets = mihomoRuleSets();
const rules = mihomoRules();

export const mihomoConfigurationAdapter = createStructuredConfigurationAdapter({
	...mihomoConfigurationStrategies,
	kind: "mihomo",
  catalogTarget: "mihomo",
  decodeSettings: decodeMihomoSettings,
  groups,
  ruleSets,
  rules,
  defaults: {
    ruleSets: defaultRuleSets,
    rules: defaultRules,
  },
  validate: (draft) => validateDraft(draft, {
    groups: {
      isHealthCheck: groups.isHealthCheck,
      requireHealthCheckInterval: true,
      requireHealthCheckURL: true,
      supportsExcludeFilter: true,
      validateFilter: groups.validateFilter,
      validate: (group, index) => {
        const state = stateRecord(group.adapterState);
        return group.memberMode === "runtime-filter"
          && !group.excludeFilter
          && Object.hasOwn(state, "exclude-filter")
          && !groups.validateFilter(state["exclude-filter"])
          ? [issue("group_filter_invalid", "groups", `group-${index}`, "Proxy group filter is invalid.")]
          : [];
      },
      validInterval: (value) => positiveInteger(value) !== undefined,
    },
    ruleSets: { validInterval: (value) => positiveInteger(value) !== undefined },
    rules: {
      requiresPolicy: rules.requiresPolicy,
      requiresValue: rules.requiresValue,
      validate: validateMihomoRule,
    },
  }),
});

function decodeMihomoSettings(value: unknown): Partial<FileConfigDraft> | null {
  const settings = strictSettingsObject(value, ["adaptive_groups", "groups", "rule_sets", "rules"]);
  if (!settings) return null;
  if ("groups" in settings && !recordArray(settings.groups)) return null;
  if ("rule_sets" in settings && !recordArray(settings.rule_sets)) return null;
  if ("rules" in settings && (!Array.isArray(settings.rules) || !settings.rules.every((rule) => typeof rule === "string"))) return null;
  if ("adaptive_groups" in settings && !validMihomoAdaptiveGroups(settings.adaptive_groups)) return null;
  return {
    ...(Object.hasOwn(settings, "adaptive_groups") ? { adaptive_groups: adaptiveGroups(settings.adaptive_groups) } : {}),
    ...(Object.hasOwn(settings, "groups") ? { groups: settings.groups as ConfigMap[] } : {}),
    ...(Object.hasOwn(settings, "rule_sets") ? { rule_sets: settings.rule_sets as ConfigMap[] } : {}),
    ...(Object.hasOwn(settings, "rules") ? { rules: settings.rules as unknown[] } : {}),
  };
}

function validMihomoAdaptiveGroups(value: unknown): boolean {
  const item = strictSettingsObject(value, ["type", "regions"]);
  if (!item) return false;
  if ("type" in item && typeof item.type !== "string") return false;
  return !("regions" in item)
    || (Array.isArray(item.regions) && item.regions.every((region) => typeof region === "string"));
}

function mihomoGroups(): StructuredFileConfigurationAdapter["groups"] {
  const projectKnown = (values: ConfigMap[]) => values.map((value, index): GroupDraft => ({
    id: draftID("group", index),
    name: stringField(value.name),
    type: stringField(value.type) || "select",
    memberMode: value["include-all-proxies"] === true ? "runtime-filter" : "fixed",
    members: stringList(value.proxies) ?? [],
    filter: stringField(value.filter),
    excludeFilter: stringField(value["exclude-filter"]),
    healthCheckURL: stringField(value.url),
    healthCheckInterval: scalarString(value.interval),
    hidden: typeof value.hidden === "boolean" ? value.hidden : undefined,
    adapterState: opaqueGroupState(value),
  }));
  const serialize = (values: GroupDraft[]): ConfigMap[] => values.map((draft) => {
    const value: ConfigMap = { ...stateRecord(draft.adapterState), name: draft.name, type: draft.type };
    if (draft.memberMode === "runtime-filter") {
      value["include-all-proxies"] = true;
      value.filter = draft.filter;
      if (draft.excludeFilter) value["exclude-filter"] = draft.excludeFilter;
    } else {
      value.proxies = [...draft.members];
    }
    if (isHealthCheck(draft.type)) {
      value.url = draft.healthCheckURL;
      value.interval = positiveInteger(draft.healthCheckInterval) ?? 300;
    }
    if (draft.hidden !== undefined) value.hidden = draft.hidden;
    return value;
  });
  const project = (values: ConfigMap[]) => {
    const drafts = projectKnown(values);
    return nativeValuesEqual(values, serialize(drafts)) ? drafts : null;
  };
  return {
    create: (locale = "en-US") => projectKnown([{ name: configCustomGroupName(locale), type: "select", proxies: ["$nodes", "DIRECT"] }])[0],
    defaults: (preset, locale = "en-US") => projectKnown(defaultGroups(preset, locale)),
    isHealthCheck,
    project,
    serialize,
    supportsExcludeFilter: true,
    supportsHidden: true,
    supportsRuntimeFilter: true,
    transitionMemberMode: (group, mode, restoredMembers) => ({
      ...group,
      adapterState: omitKeys(stateRecord(group.adapterState), [
        "use", "include-all", "include-all-providers",
        ...(mode === "fixed" ? ["exclude-filter"] : []),
      ]),
      memberMode: mode,
      members: mode === "fixed" ? restoredMembers?.length ? restoredMembers : group.members.length ? group.members : ["$nodes"] : group.members,
      filter: mode === "runtime-filter" ? group.filter || "(?i)" : "",
      excludeFilter: mode === "runtime-filter" ? group.excludeFilter : "",
    }),
    transitionType: (group, type) => {
      const adapterState = { ...stateRecord(group.adapterState) };
      if (type === "select") {
        delete adapterState.lazy;
        delete adapterState.strategy;
        return { ...group, adapterState, type, healthCheckURL: "", healthCheckInterval: "" };
      }
      adapterState.lazy = true;
      if (type === "load-balance") adapterState.strategy = "sticky-sessions";
      else delete adapterState.strategy;
      return {
        ...group,
        adapterState,
        type,
        healthCheckURL: group.healthCheckURL || DEFAULT_PROBE_URL,
        healthCheckInterval: group.healthCheckInterval || "300",
      };
    },
    typeOptions: GROUP_TYPE_OPTIONS,
    validateFilter: validFilter,
  };
}

function opaqueGroupState(value: ConfigMap): ConfigMap {
  const state = omitKeys(value, ["name", "type", "proxies", "include-all-proxies", "filter", "exclude-filter", "url", "interval", "hidden"]);
  if (Object.hasOwn(value, "exclude-filter") && typeof value["exclude-filter"] !== "string") {
    state["exclude-filter"] = value["exclude-filter"];
  }
  return state;
}

function mihomoRuleSets(): StructuredFileConfigurationAdapter["ruleSets"] {
  const project = (values: ConfigMap[]): RuleSetDraft[] | null => {
    const drafts: RuleSetDraft[] = [];
    for (const [index, value] of values.entries()) {
      const name = stringField(value.name);
      const type = stringField(value.type) || "inline";
      if (!name) return null;
      if (type === "http") {
        if (!hasOnlyKeys(value, ["name", "type", "behavior", "format", "interval", "url"])) return null;
        const url = stringField(value.url);
        if (!url) return null;
        drafts.push({
          id: draftID("ruleset", index), name, source: "remote",
          behavior: stringField(value.behavior) || "classical",
          format: stringField(value.format) || "yaml",
          interval: scalarString(value.interval) || "86400", url, payloadText: "",
        });
        continue;
      }
      const payload = stringList(value.payload);
      if (type !== "inline" || !payload || !hasOnlyKeys(value, ["name", "type", "behavior", "payload"])) return null;
      drafts.push({
        id: draftID("ruleset", index), name, source: "inline",
        behavior: stringField(value.behavior) || "classical", format: "yaml",
        interval: "86400", url: "", payloadText: payload.join("\n"),
      });
    }
    return nativeValuesEqual(values, serializeMihomoRuleSets(drafts)) ? drafts : null;
  };
  return {
    behaviorOptions: mihomoRuleSetBehaviorOptions,
    create: (index) => ({
      id: draftID("ruleset", Date.now() + index), name: "custom", source: "inline",
      behavior: "classical", format: "yaml", interval: "86400", url: "", payloadText: "DOMAIN-SUFFIX,example.com",
    }),
    formatOptions: [{ value: "yaml", label: "yaml" }, { value: "text", label: "text" }, { value: "mrs", label: "mrs" }],
    formatPatch: (url, format) => {
      const extension = RULE_SET_EXTENSIONS[format];
      const match = extension ? OFFICIAL_RULE_SET_URL.exec(url) : null;
      return { format, url: match ? `${match[1]}.${extension}` : url };
    },
    fromCatalog: (entry, current) => catalogResult(entry, current, (item, index) => ({
      id: `ruleset-catalog-${Date.now()}-${index}`,
      name: item.name,
      source: "remote",
      behavior: item.ruleKind === "ip" ? "ipcidr" : "domain",
      format: "mrs",
      interval: "86400",
      payloadText: "",
      url: item.url,
    })),
    project,
    serialize: serializeMihomoRuleSets,
  };
}

function serializeMihomoRuleSets(drafts: RuleSetDraft[]): ConfigMap[] {
  return drafts.map((draft) => draft.source === "remote"
    ? {
      name: draft.name.trim(), type: "http", behavior: draft.behavior || "classical",
      format: draft.format || "yaml", interval: positiveInteger(draft.interval) || 86400, url: draft.url.trim(),
    }
    : {
      name: draft.name.trim(), type: "inline", behavior: draft.behavior || "classical", payload: ruleLines(draft.payloadText),
    }).filter((item) => Boolean(item.name));
}

function mihomoRules(): StructuredFileConfigurationAdapter["rules"] {
  const project = (values: unknown[]): RuleDraft[] | null => {
    const drafts: RuleDraft[] = [];
    for (const [index, value] of values.entries()) {
      if (typeof value !== "string") return null;
      const parts = value.split(",").map((part) => part.trim());
      const draft = parts[0] === "RULE-SET"
        ? { id: draftID("rule", index), type: "rule-set", value: parts[1] ?? "", policy: parts[2] ?? "", noResolve: parts[3]?.toLowerCase() === "no-resolve" }
        : parts[0] === "GEOIP"
          ? { id: draftID("rule", index), type: "geoip", value: parts[1] ?? "", policy: parts[2] ?? "", noResolve: parts[3]?.toLowerCase() === "no-resolve" }
          : parts[0] === "DST-PORT"
            ? { id: draftID("rule", index), type: "dst-port", value: parts[1] ?? "", policy: parts[2] ?? "" }
          : parts[0] === "MATCH"
            ? { id: draftID("rule", index), type: "match", value: "", policy: parts[1] ?? "" }
            : null;
      if (!draft) return null;
      drafts.push(draft);
    }
    return nativeValuesEqual(values, serializeMihomoRules(drafts)) ? drafts : null;
  };
  return {
    create: (index, locale = "en-US") => ({ id: draftID("rule", Date.now() + index), type: "rule-set", value: "custom", policy: configAnchorName(locale) }),
    project,
    referencesRuleSet: (type) => type === "rule-set",
    requiresPolicy: () => true,
    requiresValue: (type) => type !== "match",
    serialize: serializeMihomoRules,
    supportsNoResolve: (type) => type === "rule-set" || type === "geoip",
    transitionType: (rule, type) => ({
      ...rule,
      type,
      value: type === "match" ? "" : rule.value,
      noResolve: type === "geoip" ? rule.noResolve ?? true : type === "rule-set" ? rule.noResolve : false,
    }),
    typeOptions: mihomoRuleTypeOptions,
    validateComponent: (value) => typeof value === "string" && Boolean(value.trim()),
  };
}

function serializeMihomoRules(drafts: RuleDraft[]): string[] {
  return drafts.flatMap((draft) => {
    if (!draft.policy.trim()) return [];
    const suffix = draft.noResolve ? ",no-resolve" : "";
    if (draft.type === "rule-set" && draft.value.trim()) return [`RULE-SET,${draft.value.trim()},${draft.policy.trim()}${suffix}`];
    if (draft.type === "geoip" && draft.value.trim()) return [`GEOIP,${draft.value.trim()},${draft.policy.trim()}${suffix}`];
    if (draft.type === "dst-port" && validPort(draft.value)) return [`DST-PORT,${draft.value.trim()},${draft.policy.trim()}`];
    return draft.type === "match" ? [`MATCH,${draft.policy.trim()}`] : [];
  });
}

function validateMihomoRule(rule: RuleDraft, index: number) {
  return rule.type === "dst-port" && !validPort(rule.value)
    ? [issue("rule_port_invalid", "rules", `rule-${index}`, "Destination port must be an integer from 1 to 65535.")]
    : [];
}

function validPort(value: string): boolean {
  const port = Number(value.trim());
  return Number.isInteger(port) && port >= 1 && port <= 65535;
}

function defaultGroups(preset: string, locale: ConfigNamingLocale): ConfigMap[] {
  const anchor = configAnchorName(locale);
  const auto = configAutoName(locale);
  const fallback = configGroupName("fallback", locale);
  const other = configGroupName("other", locale);
  if (preset === "minimal") return [{ name: anchor, type: "select", proxies: ["$nodes", "DIRECT"] }];
  if (preset === "region") {
    return [
      { name: anchor, type: "select", proxies: [auto, ...(["hk", "tw", "jp", "sg", "us"] as const).map((id) => configRegionName(id, locale)), other, "$nodes", "DIRECT"] },
      ...(["hk", "tw", "jp", "sg", "us"] as const).map((id) => ({ name: configRegionName(id, locale), type: "select", proxies: ["$nodes"] })),
      { name: other, type: "select", proxies: ["$nodes"] },
      { name: auto, type: "url-test", proxies: ["$nodes"], url: DEFAULT_PROBE_URL, interval: 300 },
    ];
  }
  return [
    { name: anchor, type: "select", proxies: [auto, fallback, "$nodes", "DIRECT"] },
    { name: auto, type: "url-test", proxies: ["$nodes"], url: DEFAULT_PROBE_URL, interval: 300 },
    { name: fallback, type: "fallback", proxies: ["$nodes"], url: DEFAULT_PROBE_URL, interval: 300 },
  ];
}

function defaultRuleSets(): ConfigMap[] {
  return [
    { name: "private", type: "inline", behavior: "classical", payload: ["DOMAIN-SUFFIX,local", "IP-CIDR,10.0.0.0/8,no-resolve", "IP-CIDR,172.16.0.0/12,no-resolve", "IP-CIDR,192.168.0.0/16,no-resolve"] },
    { name: "reject", type: "inline", behavior: "classical", payload: ["DOMAIN-SUFFIX,invalid"] },
  ];
}

function defaultRules(locale: ConfigNamingLocale): string[] {
  return ["RULE-SET,private,DIRECT", "RULE-SET,reject,REJECT", "GEOIP,CN,DIRECT", `MATCH,${configAnchorName(locale)}`];
}

function validFilter(value: unknown): boolean {
  if (typeof value !== "string") return false;
  const source = value.trim().replace(/^\(\?i\)/, "");
  if (!source || source.includes("`") || /\(\?(?!:)/.test(source) || /\\[1-9]/.test(source)) return false;
  try {
    new RegExp(source, "i");
    return true;
  } catch {
    return false;
  }
}

function isHealthCheck(type: string): boolean {
  return type === "url-test" || type === "fallback" || type === "load-balance";
}

function mihomoRuleSetBehaviorOptions(t: Translator) {
  return [
    { value: "classical", label: t("files.config.behaviorClassical") },
    { value: "domain", label: t("files.config.behaviorDomain") },
    { value: "ipcidr", label: t("files.config.behaviorIPCIDR") },
  ];
}

function mihomoRuleTypeOptions(t: Translator) {
  return [
    { value: "rule-set", label: t("files.config.ruleTypeRuleSet") },
    { value: "dst-port", label: "DST-PORT" },
    { value: "geoip", label: "GEOIP" },
    { value: "match", label: "MATCH" },
  ];
}
