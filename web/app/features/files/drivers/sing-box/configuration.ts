import type { ConfigMap, GroupDraft, RuleDraft, RuleSetDraft } from "~/features/files/config/model/editor-model";
import { isRecord, stringField } from "~/features/files/config/model/editor-model";
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
  nativeValuesEqual,
  ruleLines,
  scalarString,
  stringList,
  validateDraft,
} from "~/features/files/drivers/core/adapter-helpers";
import type { StructuredFileConfigurationAdapter } from "~/features/files/drivers/core/file-driver";
import {
  createStructuredConfigurationAdapter,
  omitKeys,
  recordArray,
  stateRecord,
  strictSettingsObject,
} from "~/features/files/drivers/core/structured-adapter";
import type { FileConfigDraft } from "~/features/files/model/types";
import type { Translator } from "~/shared/i18n/context";
import { DEFAULT_PROBE_URL } from "~/shared/probe/defaults";

import { singBoxConfigurationStrategies } from "./strategies";

const RULE_SET_EXTENSIONS: Readonly<Record<string, string>> = { binary: "srs", source: "json" };
const OFFICIAL_RULE_SET_URL = /^(https:\/\/raw\.githubusercontent\.com\/MetaCubeX\/meta-rules-dat\/sing\/geo\/(?:geosite|geoip)\/[^/?#]+)\.(?:srs|json)$/;
const GROUP_TYPE_OPTIONS = [
  { value: "select", label: "selector" },
  { value: "url-test", label: "urltest" },
] as const;

const groups = singBoxGroups();
const ruleSets = singBoxRuleSets();
const rules = singBoxRules();

export const singBoxConfigurationAdapter = createStructuredConfigurationAdapter({
	...singBoxConfigurationStrategies,
	kind: "sing-box",
  catalogTarget: "sing-box",
  decodeSettings: decodeSingBoxSettings,
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
      supportsExcludeFilter: false,
      validateFilter: groups.validateFilter,
      validInterval: validDuration,
    },
    ruleSets: { validInterval: validDuration },
    rules: {},
  }),
});

function decodeSingBoxSettings(value: unknown): Partial<FileConfigDraft> | null {
  const settings = strictSettingsObject(value, ["groups", "rule_sets", "rules"]);
  if (!settings) return null;
  if ("groups" in settings && !recordArray(settings.groups)) return null;
  if ("rule_sets" in settings && !recordArray(settings.rule_sets)) return null;
  if ("rules" in settings && (!Array.isArray(settings.rules) || !settings.rules.every(isRecord))) return null;
  return {
    ...(Object.hasOwn(settings, "groups") ? { groups: settings.groups as ConfigMap[] } : {}),
    ...(Object.hasOwn(settings, "rule_sets") ? { rule_sets: settings.rule_sets as ConfigMap[] } : {}),
    ...(Object.hasOwn(settings, "rules") ? { rules: settings.rules as unknown[] } : {}),
  };
}

function singBoxGroups(): StructuredFileConfigurationAdapter["groups"] {
  const projectKnown = (values: ConfigMap[]) => values.map((value, index): GroupDraft => ({
    id: draftID("group", index),
    name: stringField(value.tag),
    type: stringField(value.type) === "urltest" ? "url-test" : "select",
    memberMode: "fixed",
    members: stringList(value.outbounds) ?? [],
    filter: "",
    excludeFilter: "",
    healthCheckURL: stringField(value.url),
    healthCheckInterval: scalarString(value.interval),
    adapterState: omitKeys(value, ["type", "tag", "outbounds", "url", "interval"]),
  }));
  const serialize = (values: GroupDraft[]): ConfigMap[] => values.map((draft) => {
    const value: ConfigMap = {
      ...stateRecord(draft.adapterState),
      type: draft.type === "url-test" ? "urltest" : "selector",
      tag: draft.name,
      outbounds: [...draft.members],
    };
    if (draft.type === "url-test") {
      value.url = draft.healthCheckURL;
      value.interval = draft.healthCheckInterval;
    }
    return value;
  });
  const project = (values: ConfigMap[]) => {
    const drafts = projectKnown(values);
    return nativeValuesEqual(values, serialize(drafts)) ? drafts : null;
  };
  return {
    create: (locale = "en-US") => projectKnown([{
      type: "selector",
      tag: configCustomGroupName(locale),
      outbounds: ["$nodes", "direct"],
    }])[0],
    defaults: (preset, locale = "en-US") => projectKnown(defaultGroups(preset, locale)),
    isHealthCheck: (type) => type === "url-test",
    project,
    serialize,
    supportsExcludeFilter: false,
    supportsHidden: false,
    supportsRuntimeFilter: false,
    transitionMemberMode: (group) => group,
    transitionType: (group, type) => {
      const adapterState = { ...stateRecord(group.adapterState) };
      if (type === "select") {
        return { ...group, adapterState, type, healthCheckURL: "", healthCheckInterval: "" };
      }
      delete adapterState.default;
      return {
        ...group,
        adapterState,
        type,
        healthCheckURL: group.healthCheckURL || DEFAULT_PROBE_URL,
        healthCheckInterval: group.healthCheckInterval || "5m",
      };
    },
    typeOptions: GROUP_TYPE_OPTIONS,
    validateFilter: () => false,
  };
}

function singBoxRuleSets(): StructuredFileConfigurationAdapter["ruleSets"] {
  const project = (values: ConfigMap[]): RuleSetDraft[] | null => {
    const drafts: RuleSetDraft[] = [];
    for (const [index, value] of values.entries()) {
      const name = stringField(value.tag);
      const type = stringField(value.type) || "inline";
      if (!name) return null;
      if (type === "remote") {
        if (!hasOnlyKeys(value, ["type", "tag", "format", "url", "update_interval"])) return null;
        const url = stringField(value.url);
        if (!url) return null;
        drafts.push({
          id: draftID("ruleset", index),
          name,
          source: "remote",
          behavior: "classical",
          format: stringField(value.format) || remoteFormat(url),
          interval: scalarString(value.update_interval) || "1d",
          url,
          payloadText: "",
        });
        continue;
      }
      const nativeRules = Array.isArray(value.rules) ? value.rules : undefined;
      if (type !== "inline" || !nativeRules || !hasOnlyKeys(value, ["type", "tag", "rules"])) return null;
      drafts.push({
        id: draftID("ruleset", index),
        name,
        source: "inline",
        behavior: "classical",
        format: "source",
        interval: "1d",
        url: "",
        payloadText: ruleSetLines(nativeRules).join("\n"),
      });
    }
    return nativeValuesEqual(values, serializeRuleSets(drafts)) ? drafts : null;
  };
  return {
    behaviorOptions: classicalBehaviorOptions,
    create: (index) => ({
      id: draftID("ruleset", Date.now() + index),
      name: "custom",
      source: "inline",
      behavior: "classical",
      format: "source",
      interval: "1d",
      url: "",
      payloadText: "domain_suffix=example.com",
    }),
    formatOptions: [{ value: "source", label: "source" }, { value: "binary", label: "binary" }],
    formatPatch: (url, format) => {
      const extension = RULE_SET_EXTENSIONS[format];
      const match = extension ? OFFICIAL_RULE_SET_URL.exec(url) : null;
      return { format, url: match ? `${match[1]}.${extension}` : url };
    },
    fromCatalog: (entry, current) => catalogResult(entry, current, (item, index) => ({
      id: `ruleset-catalog-${Date.now()}-${index}`,
      name: item.name,
      source: "remote",
      behavior: "classical",
      format: "binary",
      interval: "1d",
      payloadText: "",
      url: item.url,
    })),
    project,
    serialize: serializeRuleSets,
  };
}

function singBoxRules(): StructuredFileConfigurationAdapter["rules"] {
  const project = (values: unknown[]): RuleDraft[] | null => {
    const drafts: RuleDraft[] = [];
    for (const [index, value] of values.entries()) {
      if (!isRecord(value)) return null;
      const outbound = stringField(value.outbound);
      if (!outbound) return null;
      const ruleSet = stringList(value.rule_set);
      if (ruleSet?.length) {
        drafts.push({ id: draftID("rule", index), type: "rule-set", value: ruleSet[0], policy: outbound });
        continue;
      }
      if (value.ip_is_private === true) {
        drafts.push({ id: draftID("rule", index), type: "private", value: "", policy: outbound });
        continue;
      }
      if (Object.keys(value).length !== 1 && !(Object.keys(value).length === 2 && "outbound" in value)) return null;
      drafts.push({ id: draftID("rule", index), type: "match", value: "", policy: outbound });
    }
    return nativeValuesEqual(values, serializeRoutingRules(drafts)) ? drafts : null;
  };
  return {
    create: (index, locale = "en-US") => ({
      id: draftID("rule", Date.now() + index),
      type: "rule-set",
      value: "custom",
      policy: configAnchorName(locale),
    }),
    project,
    referencesRuleSet: (type) => type === "rule-set",
    requiresValue: (type) => type !== "private" && type !== "match",
    serialize: serializeRoutingRules,
    supportsNoResolve: () => false,
    transitionType: (rule, type) => ({
      ...rule,
      type,
      value: type === "private" || type === "match" ? "" : rule.value,
      noResolve: false,
    }),
    typeOptions: singBoxRuleTypeOptions,
    validateComponent: (value) => typeof value === "string" && Boolean(value.trim()),
  };
}

function serializeRuleSets(drafts: RuleSetDraft[]): ConfigMap[] {
  return drafts.map((draft) => draft.source === "remote"
    ? {
      type: "remote",
      tag: draft.name.trim(),
      format: draft.format || "source",
      update_interval: draft.interval || "1d",
      url: draft.url.trim(),
    }
    : {
      type: "inline",
      tag: draft.name.trim(),
      rules: rulesFromLines(ruleLines(draft.payloadText)),
    }).filter((item) => Boolean(item.tag));
}

function serializeRoutingRules(drafts: RuleDraft[]): unknown[] {
  return drafts.flatMap((draft) => {
    const policy = draft.policy.trim();
    if (!policy) return [];
    if (draft.type === "rule-set" && draft.value.trim()) return [{ rule_set: [draft.value.trim()], outbound: policy }];
    if (draft.type === "private") return [{ ip_is_private: true, outbound: policy }];
    return draft.type === "match" ? [{ outbound: policy }] : [];
  });
}

function defaultGroups(preset: string, locale: ConfigNamingLocale): ConfigMap[] {
  const anchor = configAnchorName(locale);
  const auto = configAutoName(locale);
  const fallback = configGroupName("fallback", locale);
  const other = configGroupName("other", locale);
  if (preset === "minimal") return [{ type: "selector", tag: anchor, outbounds: ["$nodes", "direct"] }];
  if (preset === "region") {
    return [
      { type: "selector", tag: anchor, outbounds: [auto, ...(["hk", "tw", "jp", "sg", "us"] as const).map((id) => configRegionName(id, locale)), other, "$nodes", "direct"], default: auto },
      ...(["hk", "tw", "jp", "sg", "us"] as const).map((id) => ({ type: "selector", tag: configRegionName(id, locale), outbounds: ["$nodes"] })),
      { type: "selector", tag: other, outbounds: ["$nodes"] },
      { type: "urltest", tag: auto, outbounds: ["$nodes"], url: DEFAULT_PROBE_URL, interval: "5m" },
    ];
  }
  return [
    { type: "selector", tag: anchor, outbounds: [auto, fallback, "$nodes", "direct"], default: auto },
    { type: "urltest", tag: auto, outbounds: ["$nodes"], url: DEFAULT_PROBE_URL, interval: "5m" },
    { type: "selector", tag: fallback, outbounds: ["$nodes"] },
  ];
}

function defaultRuleSets(): ConfigMap[] {
  return [
    {
      type: "inline",
      tag: "private",
      rules: [
        { domain_suffix: ["local"] },
        { ip_cidr: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"] },
      ],
    },
    { type: "inline", tag: "reject", rules: [{ domain_suffix: ["invalid"] }] },
  ];
}

function defaultRules(locale: ConfigNamingLocale): unknown[] {
  return [
    { rule_set: ["private"], outbound: "direct" },
    { rule_set: ["reject"], outbound: "block" },
    { ip_is_private: true, outbound: "direct" },
    { outbound: configAnchorName(locale) },
  ];
}

function ruleSetLines(values: unknown[]): string[] {
  const lines: string[] = [];
  for (const value of values) {
    if (!isRecord(value)) continue;
    for (const domain of stringList(value.domain_suffix) ?? []) lines.push(`domain_suffix=${domain}`);
    for (const cidr of stringList(value.ip_cidr) ?? []) lines.push(`ip_cidr=${cidr}`);
  }
  return lines;
}

function rulesFromLines(lines: string[]): ConfigMap[] {
  const domains: string[] = [];
  const cidrs: string[] = [];
  for (const line of lines) {
    const [rawKey, ...rest] = line.split("=");
    const key = rawKey.trim();
    const value = rest.join("=").trim();
    if (key === "ip_cidr" && value) cidrs.push(value);
    else if (key === "domain_suffix" && value) domains.push(value);
    else if (line.trim()) domains.push(line.trim());
  }
  return [
    ...(domains.length ? [{ domain_suffix: domains }] : []),
    ...(cidrs.length ? [{ ip_cidr: cidrs }] : []),
  ];
}

function remoteFormat(url: string): string {
  return /\.srs(?:[?#]|$)/i.test(url) ? "binary" : "source";
}

function validDuration(value: string): boolean {
  return /^[+-]?(?:0|(?:(?:\d+(?:\.\d*)?|\.\d+)(?:ns|us|µs|μs|ms|s|m|h|d))+)$/.test(value.trim());
}

function classicalBehaviorOptions(t: Translator) {
  return [{ value: "classical", label: t("files.config.behaviorClassical") }];
}

function singBoxRuleTypeOptions(t: Translator) {
  return [
    { value: "rule-set", label: t("files.config.ruleTypeRuleSet") },
    { value: "private", label: t("files.config.ruleTypePrivate") },
    { value: "match", label: t("files.config.ruleTypeDefault") },
  ];
}
