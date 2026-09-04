import {
  ADAPTIVE_REGION_IDS,
  adaptiveGroupHelpers,
  adaptiveGroupOptionsFromValues,
  type ConfigAdaptiveDialect,
  DEFAULT_ADAPTIVE_REGION_IDS,
} from "~/features/files/config/model/adaptive-groups";
import { CANONICAL_ADAPTIVE_GROUP_DEFINITIONS } from "~/features/files/config/model/adaptive-regions";
import type { ConfigMap } from "~/features/files/config/model/editor-model";
import { configGroupName } from "~/features/files/config/model/naming";
import type { ConfigPreviewStrategy } from "~/features/files/config/model/preview";
import type { ConfigReferenceStrategy } from "~/features/files/config/model/references";
import {
  buildConfigRelationModel,
  type ConfigRelationProjection,
  type ConfigRelationStrategy,
  type ConfigValidationIssue,
  relationIssue,
} from "~/features/files/config/model/relations";
import {
  type ConfigTemplateBlueprint,
  type ConfigTemplateStrategy,
  createConfigTemplateStrategy,
  templateGroupTargets,
} from "~/features/files/config/model/templates";
import { nativeValuesEqual } from "~/features/files/drivers/core/adapter-helpers";
import type { ConfigAdaptiveStrategy } from "~/features/files/drivers/core/file-driver";
import {
  isHTTPURL,
  relationIdentity,
  relationIssueEvent,
  relationReferenceEvent,
  strictStringList,
  trimmedString,
} from "~/features/files/drivers/core/strategy-helpers";
import type { FileConfigDraft } from "~/features/files/model/types";

import {
  conflictsWithShadowrocketBuiltinRulePolicy,
  shadowrocketGroupBuiltinPolicies,
  shadowrocketRuleBuiltinPolicies,
} from "./policies";

const ADAPTIVE_TYPE_OPTIONS = [
  { value: "select", label: "select" },
  { value: "url-test", label: "url-test" },
  { value: "load-balance", label: "load-balance" },
] as const;
const RULE_BASE = "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket";
const AI_RULE_URL = "https://raw.githubusercontent.com/iab0x00/ProxyRules/main/Rule/AI.txt";

const relations = shadowrocketRelations();
const adaptive = shadowrocketAdaptive(shadowrocketAdaptiveDialect(relations));

export const shadowrocketConfigurationStrategies = Object.freeze({
  adaptive,
  preview: Object.freeze<ConfigPreviewStrategy>({
    projectNodes: () => [],
    relationNodeNames: () => [],
    validate: () => ({ valid: true }),
  }),
  references: Object.freeze<ConfigReferenceStrategy>({
    groupBuiltins: shadowrocketGroupBuiltinPolicies,
    includeAllNodesMacro: false,
    includeNode: () => false,
    rulePolicyBuiltins: shadowrocketRuleBuiltinPolicies,
  }),
  relations,
  templates: shadowrocketTemplates(adaptive),
});

function shadowrocketRelations(): ConfigRelationStrategy {
	const strategy: ConfigRelationStrategy = {
    project(groups, ruleSets, rules): ConfigRelationProjection {
      const groupIdentities = groups.map((group, index) => relationIdentity("group", index, group.name));
      const ruleSetIdentities = ruleSets.map((ruleSet, index) => relationIdentity("ruleset", index, ruleSet.name));
      const knownGroups = new Set(groupIdentities.map((group) => group.name).filter(Boolean));
      const knownRuleSets = new Set(ruleSetIdentities.map((ruleSet) => ruleSet.name).filter(Boolean));
      const allowedGroupTargets = new Set<string>(shadowrocketGroupBuiltinPolicies);
      const allowedRulePolicies = new Set<string>(shadowrocketRuleBuiltinPolicies);
      const issues = shadowrocketFieldIssues(groups, ruleSets);
      const events = groups.flatMap((group, index) => {
        const sourceGroup = groupIdentities[index].name;
        const projected = strictStringList(group.proxies).map((target) => relationReferenceEvent({
          allowed: allowedGroupTargets.has(target) || knownGroups.has(target),
          danglingIssue: relationIssue(
            "error",
            "unknown_group_target",
            "groups",
            `group-${index}`,
            `Group references unknown target "${target}".`,
            target,
          ),
          itemId: `group-${index}`,
          role: "group-member",
          section: "groups",
          sourceGroup,
          target,
        }));
        return projected;
      });
      const ruleSetTypes = new Map(ruleSets.map((ruleSet, index) => [
        ruleSetIdentities[index].name,
        trimmedString(ruleSet.type),
      ]));
      const deferredIssues = [];
      for (const [index, rule] of rules.entries()) {
        if (typeof rule !== "string") continue;
        const parts = rule.split(",").map((part) => part.trim());
        const type = (parts[0] ?? "").toUpperCase();
        if (type === "RULE-SET" || type === "DOMAIN-SET") {
          const target = parts[1] ?? "";
          if (!isHTTPURL(target)) {
            events.push(relationReferenceEvent({
              allowed: knownRuleSets.has(target),
              danglingIssue: relationIssue(
                "error",
                "unknown_rule_set",
                "rules",
                `rule-${index}`,
                `Rule references unknown rule set "${target}".`,
                target,
              ),
              itemId: `rule-${index}`,
              missingIssue: relationIssue(
                "error",
                "rule_set_reference_empty",
                "rules",
                `rule-${index}`,
                "Rule-set reference is required.",
              ),
              role: "rule-set",
              section: "rules",
              target,
            }));
            const expectedType = type === "DOMAIN-SET" ? "domain-set" : "rule-set";
            if (knownRuleSets.has(target) && ruleSetTypes.get(target) !== expectedType) {
              events.push(relationIssueEvent(relationIssue(
                "error",
                "rule_set_type_mismatch",
                "rules",
                `rule-${index}`,
                `Rule references "${target}" with a mismatched rule-set type.`,
                target,
              )));
            }
          }
        }
        const policyIndex = type === "FINAL" || type === "MATCH"
          ? 1
          : type === "RULE-SET" || type === "DOMAIN-SET" || parts.length < 3
            ? 2
            : parts.at(-1)?.toLowerCase() === "no-resolve" ? parts.length - 2 : parts.length - 1;
        if (type) {
          const target = parts[policyIndex] ?? "";
          events.push(relationReferenceEvent({
            allowed: allowedRulePolicies.has(target) || knownGroups.has(target),
            danglingIssue: relationIssue(
              "error",
              "unknown_rule_policy",
              "rules",
              `rule-${index}`,
              `Rule references unknown policy "${target}".`,
              target,
            ),
            itemId: `rule-${index}`,
            missingIssue: relationIssue(
              "error",
              "rule_policy_empty",
              "rules",
              `rule-${index}`,
              "Rule policy is required.",
            ),
            role: "rule-policy",
            section: "rules",
            target,
          }));
        }
        if ((type === "FINAL" || type === "MATCH") && index < rules.length - 1) {
          deferredIssues.push(relationIssue(
            "warning",
            "final_rule_not_last",
            "rules",
            `rule-${index}`,
            "Final routing rule must be last.",
          ));
        }
      }
      return { deferredIssues, events, groups: groupIdentities, issues, ruleSets: ruleSetIdentities };
    },
	};
	return Object.freeze(strategy);
}

function shadowrocketFieldIssues(
  groups: Record<string, unknown>[],
  ruleSets: Record<string, unknown>[],
) {
  const issues: ConfigValidationIssue[] = [];
  for (const [index, group] of groups.entries()) {
    const itemId = `group-${index}`;
    const name = trimmedString(group.name);
    if (name && (["#", ";", "["].some((prefix) => name.startsWith(prefix)) || /[\r\n=,]/.test(name))) {
      issues.push(relationIssue("error", "shadowrocket_group_name_invalid", "groups", itemId, "Group name must not start with #, ;, or [ and must not contain a line break, equals sign, or comma."));
    }
    if (name === "$nodes" || conflictsWithShadowrocketBuiltinRulePolicy(name)) {
      issues.push(relationIssue("error", "shadowrocket_group_name_reserved", "groups", itemId, `Group name "${name}" is reserved.`, name));
    }
    if (Array.isArray(group.proxies)) {
      const seen = new Set<string>();
      for (const member of group.proxies) {
        const value = trimmedString(member);
        if (value && seen.has(value)) {
          issues.push(relationIssue("error", "shadowrocket_group_member_duplicate", "groups", itemId, `Group member "${value}" is duplicated.`, value));
          break;
        }
        if (value) seen.add(value);
      }
    }
    addIntegerIssue(issues, group.interval, 1, 86400, "shadowrocket_group_interval_invalid", itemId, "interval");
    addIntegerIssue(issues, group.timeout, 1, 300, "shadowrocket_group_timeout_invalid", itemId, "timeout");
    addIntegerIssue(issues, group.tolerance, 0, 65535, "shadowrocket_group_tolerance_invalid", itemId, "tolerance");
  }
  for (const [index, ruleSet] of ruleSets.entries()) {
    const itemId = `ruleset-${index}`;
    const name = trimmedString(ruleSet.name);
    if (name && /[\r\n,]/.test(name)) {
      issues.push(relationIssue("error", "shadowrocket_rule_set_name_invalid", "rule_sets", itemId, "Rule-set name must not contain a line break or comma."));
    }
    const url = trimmedString(ruleSet.url);
    if (!isHTTPURL(url) || url.includes(",")) {
      issues.push(relationIssue("error", "rule_set_url_invalid", "rule_sets", itemId, "Rule-set URL must be a comma-free HTTP(S) URL."));
    }
  }
  return issues;
}

function shadowrocketAdaptiveDialect(
  relationStrategy: ConfigRelationStrategy,
): ConfigAdaptiveDialect {
  const materialize: ConfigAdaptiveDialect["materialize"] = (item, type) => {
    const group: ConfigMap = {
      name: item.name,
      type,
      "policy-regex-filter": policyFilter(item),
    };
    if (type === "url-test" || type === "load-balance") {
      group.interval = 300;
      group.timeout = 5;
      group.tolerance = 50;
    }
    return group;
  };
  return {
    anchorProblem: () => null,
    canonicalName: (group) => {
      for (const definition of CANONICAL_ADAPTIVE_GROUP_DEFINITIONS) {
        for (const name of [definition.name, ...(definition.legacyNames ?? [])]) {
          for (const { value: type } of ADAPTIVE_TYPE_OPTIONS) {
            if (nativeValuesEqual(group, materialize({ ...definition, name }, type, []))) return name;
          }
        }
      }
      return undefined;
    },
    defaultType: "url-test",
    groupMembers: (group) => Array.isArray(group.proxies)
      && group.proxies.every((member) => typeof member === "string")
      ? group.proxies as string[]
      : undefined,
    groupName: (group) => trimmedString(group.name),
    inboundReferences: (config) => buildConfigRelationModel(relationStrategy.project(
      config.groups ?? [],
      [],
      config.rules ?? [],
    )).groupInboundReferences,
    referenceTargets: ["PROXY"],
    referenceInsertionIndex: (members) => {
      const index = members.findIndex((member) => member === "DIRECT" || member === "REJECT");
      return index < 0 ? members.length : index;
    },
    materialize,
    replaceGroupMembers: (group, members) => ({ ...group, proxies: [...members] }),
    requiresNodePreview: false,
    typeOptions: ADAPTIVE_TYPE_OPTIONS,
  };
}

function shadowrocketAdaptive(dialect: ConfigAdaptiveDialect): ConfigAdaptiveStrategy {
  const helpers = adaptiveGroupHelpers(dialect);
  const strategy: ConfigAdaptiveStrategy = {
    ...helpers,
    configFromOptions: (options) => {
      const enabled = new Set(options.enabledRegionIds ?? DEFAULT_ADAPTIVE_REGION_IDS);
      return {
        type: options.type,
        regions: ADAPTIVE_REGION_IDS.filter((id) => enabled.has(id)),
      };
    },
    initiallyEnabled: (formMode, config) => formMode === "create" || config !== undefined,
    isStale: () => false,
    optionsFromConfig: (config) => adaptiveGroupOptionsFromValues(dialect, config ? {
      enabledRegionIds: config.regions,
      type: config.type,
    } : undefined),
    recognizesCanonicalLayer: (config) => helpers.canonicalNames(config.groups ?? []).length > 0,
  };
  return Object.freeze(strategy);
}

function shadowrocketTemplates(
  adaptiveStrategy: typeof adaptive,
): ConfigTemplateStrategy {
  return createConfigTemplateStrategy({
    groupNames: (groups) => groups.map((group) => trimmedString(group.name)).filter(Boolean),
    materialize: materializeShadowrocketTemplate,
    moduleIDs: (_id, moduleIDs) => moduleIDs.filter((moduleID) => (
      moduleID !== "select" && moduleID !== "auto" && moduleID !== "fallback"
    )),
    normalizeRecognition: (config) => {
      const adaptiveLayer = adaptiveStrategy.recognizesCanonicalLayer(config);
      const stripped = adaptiveLayer ? adaptiveStrategy.strip(config) : { changed: false, config, strippedGroupNames: [] };
      const { adaptive_groups: _adaptiveGroups, ...comparable } = stripped.config;
      return { adaptive: adaptiveLayer, config: comparable };
    },
  });
}

function materializeShadowrocketTemplate(
  blueprint: Readonly<ConfigTemplateBlueprint>,
): FileConfigDraft {
  const groups = blueprint.groups.map((item) => {
    const name = configGroupName(item.id, blueprint.namingLocale);
    const targets = templateGroupTargets(item, "PROXY", undefined, "DIRECT", "REJECT")
      .filter((target) => target !== name && target !== "$nodes");
    return item.groupMode === "url-test"
      ? { name, type: "url-test", proxies: targets, interval: 300, timeout: 5, tolerance: 50 }
      : { name, type: "select", proxies: targets };
  });
  const serviceRuleEntries = blueprint.ruleEntries
    .filter(({ ruleID }) => canonicalRuleID(ruleID) === ruleID)
    .flatMap(({ module, ruleID }) => ruleArtifacts(ruleID).map((artifact) => ({
      artifact,
      moduleID: module.id,
      policy: configGroupName(module.id, blueprint.namingLocale),
    })));
  const privateRuleEnd = serviceRuleEntries.map(({ moduleID }) => moduleID).lastIndexOf("private") + 1;
  const dohRuleEntries = ruleArtifacts("category-doh").map((artifact) => ({
    artifact,
    moduleID: "select" as const,
    policy: "PROXY",
  }));
  const ruleEntries = [
    ...serviceRuleEntries.slice(0, privateRuleEnd),
    ...dohRuleEntries,
    ...serviceRuleEntries.slice(privateRuleEnd),
  ];
  const ruleSets = ruleEntries.map(({ artifact }) => ({
    name: artifact.id,
    type: artifact.type,
    url: artifact.url,
  }));
  const rules = [
    "DST-PORT,853,PROXY",
    ...ruleEntries.map(({ artifact, policy }) => {
      const type = artifact.type === "domain-set" ? "DOMAIN-SET" : "RULE-SET";
      return `${type},${artifact.id},${policy}`;
    }),
    `GEOIP,CN,${configGroupName("cn", blueprint.namingLocale)}`,
  ];
  rules.push(`FINAL,${configGroupName("final", blueprint.namingLocale)}`);
  return {
    group_preset: "basic",
    ruleset_preset: "default",
    groups,
    rule_sets: ruleSets,
    rules,
  };
}

const ARTIFACT_NAMES: Readonly<Record<string, string>> = {
  private: "Lan", cn: "China", "geolocation-cn": "China", "geolocation-!cn": "Global",
  youtube: "YouTube", google: "Google", microsoft: "Microsoft",
  apple: "Apple", github: "GitHub", gitlab: "GitLab", atlassian: "Atlassian",
  telegram: "Telegram", twitter: "Twitter",
  discord: "Discord", linkedin: "LinkedIn", tiktok: "TikTok", line: "Line", reddit: "Reddit", snap: "Snap",
  pinterest: "Pinterest", tumblr: "Tumblr", netflix: "Netflix", disney: "Disney", hbo: "HBO", hulu: "Hulu",
  primevideo: "AmazonPrimeVideo", "apple-tvplus": "AppleTV", spotify: "Spotify", twitch: "Twitch", dazn: "DAZN",
  bahamut: "Bahamut", bilibili: "BiliBili", biliintl: "BiliBiliIntl", niconico: "Niconico", abema: "Abema", viu: "ViuTV", kktv: "KKTV",
  steam: "Steam", epicgames: "Epic", ea: "EA", ubisoft: "Ubisoft", blizzard: "Blizzard", gog: "Gog", riot: "Riot",
  playstation: "PlayStation", xbox: "Xbox", nintendo: "Nintendo", cloudflare: "Cloudflare", digitalocean: "DigitalOcean",
  vercel: "Vercel", docker: "Docker", npmjs: "Npmjs", jetbrains: "Jetbrains", stackexchange: "Stackexchange",
  dropbox: "Dropbox", notion: "Notion", paypal: "PayPal", stripe: "Stripe",
  "category-scholar-!cn": "Scholar", wikimedia: "Wikimedia", bbc: "BBC", cnn: "CNN", nytimes: "NYTimes",
  bloomberg: "Bloomberg", amazon: "Amazon", ebay: "eBay",
};

const RULE_ALIASES: Readonly<Record<string, string>> = {
  "private-ip": "private", "cn-ip": "cn", "google-ip": "google",
  "telegram-ip": "telegram", "twitter-ip": "twitter", "facebook-ip": "facebook", "netflix-ip": "netflix",
  "cloudflare-ip": "cloudflare",
};

function canonicalRuleID(ruleID: string): string {
  return RULE_ALIASES[ruleID] ?? ruleID;
}

interface ShadowrocketRuleArtifact {
  id: string;
  type: "rule-set" | "domain-set";
  url: string;
}

const CUSTOM_RULE_ARTIFACTS: Readonly<Record<string, readonly ShadowrocketRuleArtifact[]>> = {
  "category-ads-all": [
    nativeRuleArtifact("category-ads-all-domain", "Advertising/Advertising_Domain", "domain-set"),
    nativeRuleArtifact("category-ads-all", "Advertising/Advertising"),
  ],
  "category-ai-!cn": [{ id: "category-ai-!cn", type: "rule-set", url: AI_RULE_URL }],
  "category-doh": [nativeRuleArtifact("category-doh", "DNS/DNS")],
  "category-media": [
    nativeRuleArtifact("category-media-domain", "GlobalMedia/GlobalMedia_Domain", "domain-set"),
    nativeRuleArtifact("category-media", "GlobalMedia/GlobalMedia"),
  ],
  meta: [
    nativeRuleArtifact("meta", "Facebook/Facebook"),
    nativeRuleArtifact("meta-instagram", "Instagram/Instagram"),
    nativeRuleArtifact("meta-whatsapp", "Whatsapp/Whatsapp"),
  ],
  aws: [nativeRuleArtifact("aws", "Cloud/AmazonCloud/AmazonCloud")],
};

const UNSUPPORTED_SHADOWROCKET_RULE_IDS = new Set([
  "azure", "coursera", "edx", "khanacademy", "netlify", "udemy", "wise",
]);
const NATIVE_DOMAIN_SET_COMPANION_IDS = new Set(["apple", "geolocation-!cn"]);

function ruleArtifacts(ruleID: string): ShadowrocketRuleArtifact[] {
  const custom = CUSTOM_RULE_ARTIFACTS[ruleID];
  if (custom) return [...custom];
  if (UNSUPPORTED_SHADOWROCKET_RULE_IDS.has(ruleID)) return [];
  const name = ARTIFACT_NAMES[canonicalRuleID(ruleID)];
  if (!name) throw new Error(`Unknown Shadowrocket template rule: ${ruleID}`);
  if (ruleID === "cn") {
    return [
      nativeRuleArtifact("cn-domain", `${name}/${name}_Domain`, "domain-set"),
      nativeRuleArtifact("cn", `${name}/${name}`),
    ];
  }
  const artifacts: ShadowrocketRuleArtifact[] = [];
  if (NATIVE_DOMAIN_SET_COMPANION_IDS.has(ruleID)) {
    artifacts.push(nativeRuleArtifact(`${ruleID}-domain`, `${name}/${name}_Domain`, "domain-set"));
  }
  artifacts.push(nativeRuleArtifact(ruleID, `${name}/${name}`));
  return artifacts;
}

function nativeRuleArtifact(
  id: string,
  path: string,
  type: ShadowrocketRuleArtifact["type"] = "rule-set",
): ShadowrocketRuleArtifact {
  return { id, type, url: `${RULE_BASE}/${path}.list` };
}

function policyFilter(item: { filter: string; excludeFilter?: string }): string {
  if (!item.excludeFilter) return item.filter;
  const include = item.filter.replace(/^\(\?i\)/, "");
  const exclude = item.excludeFilter.replace(/^\(\?i\)/, "");
  return `(?i)^(?!.*(?:${exclude}))(?=.*(?:${include})).*$`;
}

function addIntegerIssue(
  issues: ReturnType<typeof shadowrocketFieldIssues>,
  value: unknown,
  minimum: number,
  maximum: number,
  code: string,
  itemId: string,
  label: string,
): void {
  if (value === undefined) return;
  if (typeof value === "number" && Number.isInteger(value) && value >= minimum && value <= maximum) return;
  issues.push(relationIssue("error", code, "groups", itemId, `Group ${label} must be an integer between ${minimum} and ${maximum}.`));
}
