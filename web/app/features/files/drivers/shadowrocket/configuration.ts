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
  isHTTPURL,
  issue,
  nativeValuesEqual,
  scalarString,
  stringList,
  validateDraft,
} from "~/features/files/drivers/core/adapter-helpers";
import type { StructuredFileConfigurationAdapter } from "~/features/files/drivers/core/file-driver";
import {
  adaptiveGroups,
  createStructuredConfigurationAdapter,
  recordArray,
  strictSettingsObject,
} from "~/features/files/drivers/core/structured-adapter";
import type { FileConfigDraft } from "~/features/files/model/types";
import type { Translator } from "~/shared/i18n/context";

import { conflictsWithShadowrocketBuiltinRulePolicy } from "./policies";
import { shadowrocketConfigurationStrategies } from "./strategies";

const GROUP_TYPES = new Set(["select", "url-test", "fallback", "load-balance", "random"]);
const ADAPTIVE_GROUP_TYPES = new Set(["select", "url-test", "load-balance"]);
const REGION_IDS = new Set([
  "hk", "tw", "sg", "jp", "kr", "us", "ca", "uk", "de", "fr", "mo",
  "au", "ru", "th", "in", "my", "ph", "tr", "ua", "fi", "ar", "eg",
]);
const GROUP_KEYS = [
  "name", "type", "proxies", "policy-regex-filter", "interval", "timeout", "tolerance",
  "hidden",
] as const;
const GROUP_TYPE_OPTIONS = [
  { value: "select", label: "select" },
  { value: "url-test", label: "url-test" },
  { value: "fallback", label: "fallback" },
  { value: "load-balance", label: "load-balance" },
  { value: "random", label: "random" },
] as const;
const RULE_NAMES: Readonly<Record<string, string>> = {
  "rule-set": "RULE-SET",
  "domain-set": "DOMAIN-SET",
  domain: "DOMAIN",
  "domain-suffix": "DOMAIN-SUFFIX",
  "domain-keyword": "DOMAIN-KEYWORD",
  "dst-port": "DST-PORT",
  geoip: "GEOIP",
  "ip-cidr": "IP-CIDR",
  "user-agent": "USER-AGENT",
  "url-regex": "URL-REGEX",
  final: "FINAL",
};
const NO_RESOLVE_TYPES = new Set(["rule-set", "geoip", "ip-cidr"]);

const groups = shadowrocketGroups();
const ruleSets = shadowrocketRuleSets();
const rules = shadowrocketRules();

export const shadowrocketConfigurationAdapter = createStructuredConfigurationAdapter({
	...shadowrocketConfigurationStrategies,
	kind: "shadowrocket",
  catalogTarget: "shadowrocket",
  decodeSettings: decodeShadowrocketSettings,
  groups,
  ruleSets,
  rules,
  defaults: {
    ruleSets: () => [],
    rules: defaultRules,
    runtimeGroups: () => [{ name: "Proxy", type: "select", proxies: ["$nodes", "DIRECT"] }],
    runtimeRules: () => runtimeRules(),
  },
  validate: (draft) => validateDraft(draft, {
    groups: {
      isHealthCheck: groups.isHealthCheck,
      requireHealthCheckInterval: false,
      requireHealthCheckURL: false,
      supportsExcludeFilter: false,
      validateFilter: groups.validateFilter,
      validate: validateGroup,
    },
    ruleSets: { validate: validateRuleSet },
    rules: {
      requiresPolicy: rules.requiresPolicy,
      requiresValue: rules.requiresValue,
      validate: validateRule,
    },
  }),
});

function decodeShadowrocketSettings(value: unknown): Partial<FileConfigDraft> | null {
  const settings = strictSettingsObject(value, ["adaptive_groups", "groups", "rule_sets", "rules"]);
  if (!settings) return null;
  if ("adaptive_groups" in settings && !validAdaptiveGroups(settings.adaptive_groups)) return null;
  if ("groups" in settings && !validGroups(settings.groups)) return null;
  if ("rule_sets" in settings && !validRuleSets(settings.rule_sets)) return null;
  if ("rules" in settings && !validRules(settings.rules)) return null;
  return {
    ...(Object.hasOwn(settings, "adaptive_groups") ? { adaptive_groups: adaptiveGroups(settings.adaptive_groups) } : {}),
    ...(Object.hasOwn(settings, "groups") ? { groups: settings.groups as ConfigMap[] } : {}),
    ...(Object.hasOwn(settings, "rule_sets") ? { rule_sets: settings.rule_sets as ConfigMap[] } : {}),
    ...(Object.hasOwn(settings, "rules") ? { rules: settings.rules as string[] } : {}),
  };
}

function shadowrocketGroups(): StructuredFileConfigurationAdapter["groups"] {
  const projectKnown = (values: ConfigMap[]) => values.map((value, index): GroupDraft => ({
    id: draftID("group", index),
    name: stringField(value.name),
    type: stringField(value.type) || "select",
    memberMode: typeof value["policy-regex-filter"] === "string" ? "runtime-filter" : "fixed",
    members: stringList(value.proxies) ?? [],
    filter: stringField(value["policy-regex-filter"]),
    excludeFilter: "",
    healthCheckURL: "",
    healthCheckInterval: scalarString(value.interval),
    healthCheckTimeout: optionalNumber(value.timeout),
    healthCheckTolerance: optionalNumber(value.tolerance),
    hidden: typeof value.hidden === "boolean" ? value.hidden : undefined,
  }));
  const serialize = (values: GroupDraft[]): ConfigMap[] => values.map((draft) => {
    const value: ConfigMap = { name: draft.name, type: draft.type };
    if (draft.memberMode === "runtime-filter") value["policy-regex-filter"] = draft.filter;
    else value.proxies = [...draft.members];
    if (isHealthCheck(draft.type)) {
      if (draft.healthCheckInterval) value.interval = Number(draft.healthCheckInterval);
      if (draft.healthCheckTimeout !== undefined) value.timeout = draft.healthCheckTimeout;
      if (draft.healthCheckTolerance !== undefined) value.tolerance = draft.healthCheckTolerance;
    }
    if (draft.hidden !== undefined) value.hidden = draft.hidden;
    return value;
  });
  const project = (values: ConfigMap[]) => {
    const drafts = projectKnown(values);
    return nativeValuesEqual(values, serialize(drafts)) ? drafts : null;
  };
  return {
    create: (locale = "en-US") => projectKnown([{
      name: configCustomGroupName(locale),
      type: "select",
      proxies: ["$nodes", "DIRECT"],
    }])[0],
    defaults: (preset, locale = "en-US") => projectKnown(defaultGroups(preset, locale)),
    isHealthCheck,
    project,
    serialize,
    supportsExcludeFilter: false,
    supportsHidden: true,
    supportsRuntimeFilter: true,
    transitionMemberMode: (group, mode, restoredMembers) => ({
      ...group,
      memberMode: mode,
      members: mode === "fixed"
        ? restoredMembers?.length ? restoredMembers : group.members.length ? group.members : ["$nodes"]
        : group.members,
      filter: mode === "runtime-filter" ? group.filter || "(?i)" : "",
    }),
    transitionType: (group, type) => !isHealthCheck(type)
      ? { ...group, type, healthCheckInterval: "", healthCheckTimeout: undefined, healthCheckTolerance: undefined }
      : {
        ...group,
        type,
        healthCheckInterval: group.healthCheckInterval || "300",
        healthCheckTimeout: group.healthCheckTimeout ?? 5,
        healthCheckTolerance: type === "url-test" || type === "load-balance"
          ? group.healthCheckTolerance ?? 50
          : undefined,
      },
    typeOptions: GROUP_TYPE_OPTIONS,
    validateFilter,
  };
}

function shadowrocketRuleSets(): StructuredFileConfigurationAdapter["ruleSets"] {
  const project = (values: ConfigMap[]): RuleSetDraft[] | null => {
    const drafts: RuleSetDraft[] = [];
    for (const [index, value] of values.entries()) {
      if (!hasOnlyKeys(value, ["name", "type", "url"])) return null;
      const name = stringField(value.name);
      const behavior = stringField(value.type);
      const url = stringField(value.url);
      if (!name) return null;
      if ((behavior !== "rule-set" && behavior !== "domain-set") || !isHTTPURL(url)) return null;
      drafts.push({
        id: draftID("ruleset", index),
        name,
        source: "remote",
        behavior,
        format: "",
        interval: "",
        url,
        payloadText: "",
      });
    }
    return nativeValuesEqual(values, serializeShadowrocketRuleSets(drafts)) ? drafts : null;
  };
  return {
    behaviorOptions: shadowrocketRuleSetBehaviorOptions,
    create: (index) => ({
      id: draftID("ruleset", Date.now() + index),
      name: "custom",
      source: "remote",
      behavior: "rule-set",
      format: "",
      interval: "",
      url: "https://example.com/rules.list",
      payloadText: "",
    }),
    formatOptions: [],
    formatPatch: (url, format) => ({ format, url }),
    fromCatalog: (entry, current) => catalogResult(entry, current, (item, index) => ({
      id: `ruleset-catalog-${Date.now()}-${index}`,
      name: item.name,
      source: "remote",
      behavior: item.referenceType?.toLocaleLowerCase() ?? "rule-set",
      format: "",
      interval: "",
      payloadText: "",
      url: item.url,
    })),
    project,
    serialize: serializeShadowrocketRuleSets,
  };
}

function serializeShadowrocketRuleSets(drafts: RuleSetDraft[]): ConfigMap[] {
  return drafts.map((draft) => ({
    name: draft.name.trim(),
    type: draft.behavior === "domain-set" ? "domain-set" : "rule-set",
    url: draft.url.trim(),
  })).filter((item) => Boolean(item.name));
}

function shadowrocketRules(): StructuredFileConfigurationAdapter["rules"] {
  const project = (values: unknown[]): RuleDraft[] | null => {
    const drafts: RuleDraft[] = [];
    for (const [index, value] of values.entries()) {
      if (typeof value !== "string") return null;
      const parts = value.split(",").map((part) => part.trim());
      const type = Object.entries(RULE_NAMES).find(([, native]) => native === parts[0].toUpperCase())?.[0];
      if (!type) return null;
      if (type === "final") {
        if (parts.length !== 2 || !parts[1]) return null;
        drafts.push({ id: draftID("rule", index), type, value: "", policy: parts[1] });
        continue;
      }
      const noResolve = parts.at(-1)?.toLowerCase() === "no-resolve";
      if (parts.length !== (noResolve ? 4 : 3) || !parts[1] || !parts[2]) return null;
      if (noResolve && !NO_RESOLVE_TYPES.has(type)) return null;
      drafts.push({
        id: draftID("rule", index),
        type,
        value: parts[1],
        policy: parts[2],
        ...(noResolve ? { noResolve: true } : {}),
      });
    }
    return nativeValuesEqual(values, serializeShadowrocketRules(drafts)) ? drafts : null;
  };
  return {
    create: (index, locale = "en-US") => ({
      id: draftID("rule", Date.now() + index),
      type: "rule-set",
      value: "custom",
      policy: configAnchorName(locale),
    }),
    project,
    referencesRuleSet: (type) => type === "rule-set" || type === "domain-set",
    requiresPolicy: () => true,
    requiresValue: (type) => type !== "final",
    serialize: serializeShadowrocketRules,
    supportsNoResolve: (type) => NO_RESOLVE_TYPES.has(type),
    transitionType: (rule, type) => ({
      ...rule,
      type,
      value: type === "final" ? "" : rule.value,
      noResolve: type === "geoip" || type === "ip-cidr"
        ? rule.noResolve ?? true
        : NO_RESOLVE_TYPES.has(type) ? rule.noResolve : false,
    }),
    typeOptions: shadowrocketRuleTypeOptions,
    validateComponent: validRuleComponent,
  };
}

function serializeShadowrocketRules(drafts: RuleDraft[]): string[] {
  return drafts.flatMap((draft) => {
    if (!validRuleComponent(draft.policy)) return [];
    const policy = draft.policy.trim();
    if (draft.type === "final") return [`FINAL,${policy}`];
    if (!validRuleComponent(draft.value)) return [];
    const nativeType = RULE_NAMES[draft.type];
    if (!nativeType) return [];
    const suffix = draft.noResolve && NO_RESOLVE_TYPES.has(draft.type) ? ",no-resolve" : "";
    return [`${nativeType},${draft.value.trim()},${policy}${suffix}`];
  });
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
      { name: auto, type: "url-test", proxies: ["$nodes"], interval: 300, timeout: 5, tolerance: 50 },
    ];
  }
  return [
    { name: anchor, type: "select", proxies: [auto, fallback, "$nodes", "DIRECT"] },
    { name: auto, type: "url-test", proxies: ["$nodes"], interval: 300, timeout: 5, tolerance: 50 },
    { name: fallback, type: "fallback", proxies: ["$nodes"], interval: 300, timeout: 5 },
  ];
}

function defaultRules(locale: ConfigNamingLocale): string[] {
  return [
    "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
    "IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
    "IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
    "GEOIP,CN,DIRECT,no-resolve",
    `FINAL,${configAnchorName(locale)}`,
  ];
}

function runtimeRules(): string[] {
  return [
    "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
    "IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
    "IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
    "GEOIP,CN,DIRECT",
    "FINAL,Proxy",
  ];
}

function validAdaptiveGroups(value: unknown): boolean {
  const item = strictSettingsObject(value, ["type", "regions"]);
  if (!item) return false;
  if ("type" in item && (typeof item.type !== "string" || !ADAPTIVE_GROUP_TYPES.has(item.type.trim()))) return false;
  if ("regions" in item) {
    if (!Array.isArray(item.regions) || item.regions.some((region) => typeof region !== "string" || !REGION_IDS.has(region))) return false;
    if (new Set(item.regions).size !== item.regions.length) return false;
  }
  return true;
}

function validGroups(value: unknown): value is ConfigMap[] {
  if (!recordArray(value)) return false;
  return value.every((group) => {
    if (!strictSettingsObject(group, GROUP_KEYS)) return false;
    if (!assignmentSafeName(group.name) || typeof group.type !== "string" || !GROUP_TYPES.has(group.type.trim())) return false;
    const fixed = Object.hasOwn(group, "proxies");
    const filtered = Object.hasOwn(group, "policy-regex-filter");
    if (fixed === filtered) return false;
    if (fixed) {
      if (!Array.isArray(group.proxies) || group.proxies.some((member) => typeof member !== "string" || !member.trim())) return false;
      const members = group.proxies.map((member) => String(member).trim());
      if (new Set(members).size !== members.length) return false;
    }
    if (filtered && !validateFilter(group["policy-regex-filter"])) return false;
    if (!optionalIntegerInRange(group.interval, 1, 86400)) return false;
    if (!optionalIntegerInRange(group.timeout, 1, 300)) return false;
    if (!optionalIntegerInRange(group.tolerance, 0, 65535)) return false;
    return group.hidden === undefined || typeof group.hidden === "boolean";
  });
}

function validRuleSets(value: unknown): value is ConfigMap[] {
  if (!recordArray(value)) return false;
  return value.every((ruleSet) => {
    if (!strictSettingsObject(ruleSet, ["name", "type", "url"])) return false;
    const type = typeof ruleSet.type === "string" ? ruleSet.type.trim() : "";
    const url = typeof ruleSet.url === "string" ? ruleSet.url.trim() : "";
    return ruleReferenceName(ruleSet.name)
      && (type === "rule-set" || type === "domain-set")
      && typeof ruleSet.url === "string"
      && isHTTPURL(url)
      && !ruleSet.url.includes(",");
  });
}

function validRules(value: unknown): value is string[] {
  return Array.isArray(value)
    && value.every((rule) => typeof rule === "string" && Boolean(rule.trim()) && !/[\r\n]/.test(rule));
}

function validateGroup(group: GroupDraft, index: number) {
  const issues = [];
  if (!assignmentSafeName(group.name)) {
    issues.push(issue("group_name_invalid", "groups", `group-${index}`, "Proxy group name is invalid."));
  }
  if (isHealthCheck(group.type)) {
    if (!optionalIntegerInRange(numberOrUndefined(group.healthCheckInterval), 1, 86400)) {
      issues.push(issue("group_interval_invalid", "groups", `group-${index}`, "Proxy group check interval is invalid."));
    }
    if (!optionalIntegerInRange(group.healthCheckTimeout, 1, 300)) {
      issues.push(issue("group_timeout_invalid", "groups", `group-${index}`, "Proxy group check timeout is invalid."));
    }
    if (!optionalIntegerInRange(group.healthCheckTolerance, 0, 65535)) {
      issues.push(issue("group_tolerance_invalid", "groups", `group-${index}`, "Proxy group tolerance is invalid."));
    }
  }
  return issues;
}

function validateRuleSet(ruleSet: RuleSetDraft, index: number) {
  const issues = [];
  if (ruleSet.source !== "remote") issues.push(issue("rule_set_source_invalid", "rule_sets", `ruleset-${index}`, "Shadowrocket rule sets must be remote."));
  if (ruleSet.behavior !== "rule-set" && ruleSet.behavior !== "domain-set") {
    issues.push(issue("rule_set_type_invalid", "rule_sets", `ruleset-${index}`, "Shadowrocket rule-set type is invalid."));
  }
  if (!ruleReferenceName(ruleSet.name)) issues.push(issue("rule_set_name_invalid", "rule_sets", `ruleset-${index}`, "Rule-set name is invalid."));
  return issues;
}

function validateRule(rule: RuleDraft, index: number) {
  const issues = [];
  if (!validRuleComponent(rule.policy)) issues.push(issue("rule_policy_invalid", "rules", `rule-${index}`, "Rule policy is invalid."));
  if (rule.type !== "final" && !validRuleComponent(rule.value)) issues.push(issue("rule_value_invalid", "rules", `rule-${index}`, "Rule match value is invalid."));
  if (rule.type === "dst-port" && !validPort(rule.value)) {
    issues.push(issue("rule_port_invalid", "rules", `rule-${index}`, "Destination port must be an integer from 1 to 65535."));
  }
  return issues;
}

function assignmentSafeName(value: unknown): boolean {
  if (typeof value !== "string") return false;
  const name = value.trim();
  return Boolean(name)
    && !["#", ";", "["].some((prefix) => name.startsWith(prefix))
    && !/[\r\n=,]/.test(name)
    && name !== "$nodes"
    && !conflictsWithShadowrocketBuiltinRulePolicy(name);
}

function ruleReferenceName(value: unknown): boolean {
  return typeof value === "string" && Boolean(value.trim()) && !/[\r\n,]/.test(value);
}

function validateFilter(value: unknown): boolean {
  return typeof value === "string" && Boolean(value.trim()) && !/[\r\n,]/.test(value);
}

function validRuleComponent(value: unknown): value is string {
  return typeof value === "string" && Boolean(value.trim()) && !/[\r\n,]/.test(value);
}

function validPort(value: string): boolean {
  const port = Number(value.trim());
  return Number.isInteger(port) && port >= 1 && port <= 65535;
}

function isHealthCheck(type: string): boolean {
  return type === "url-test" || type === "fallback" || type === "load-balance";
}

function optionalIntegerInRange(value: unknown, minimum: number, maximum: number): boolean {
  return value === undefined || (Number.isInteger(value) && Number(value) >= minimum && Number(value) <= maximum);
}

function optionalNumber(value: unknown): number | undefined {
  return typeof value === "number" ? value : undefined;
}

function numberOrUndefined(value: string): number | undefined {
  return value ? Number(value) : undefined;
}

function shadowrocketRuleSetBehaviorOptions(_t: Translator) {
  return [{ value: "rule-set", label: "RULE-SET" }, { value: "domain-set", label: "DOMAIN-SET" }];
}

function shadowrocketRuleTypeOptions(_t: Translator) {
  return [
    { value: "domain", label: "DOMAIN" },
    { value: "domain-suffix", label: "DOMAIN-SUFFIX" },
    { value: "domain-keyword", label: "DOMAIN-KEYWORD" },
    { value: "dst-port", label: "DST-PORT" },
    { value: "rule-set", label: "RULE-SET" },
    { value: "domain-set", label: "DOMAIN-SET" },
    { value: "geoip", label: "GEOIP" },
    { value: "ip-cidr", label: "IP-CIDR" },
    { value: "user-agent", label: "USER-AGENT" },
    { value: "url-regex", label: "URL-REGEX" },
    { value: "final", label: "FINAL" },
  ];
}
