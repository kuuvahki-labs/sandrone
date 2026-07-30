import {
  ADAPTIVE_REGION_IDS,
  type AdaptiveGroupAnchorProblem,
  adaptiveGroupHelpers,
  adaptiveGroupOptionsFromValues,
  type ConfigAdaptiveDialect,
  DEFAULT_ADAPTIVE_REGION_IDS,
} from "~/features/files/config/model/adaptive-groups";
import { CANONICAL_ADAPTIVE_GROUP_DEFINITIONS } from "~/features/files/config/model/adaptive-regions";
import type { ConfigMap } from "~/features/files/config/model/editor-model";
import { configAnchorName, configGroupName } from "~/features/files/config/model/naming";
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

const relations = shadowrocketRelations();
const adaptive = shadowrocketAdaptive(shadowrocketAdaptiveDialect(relations));

export const shadowrocketConfigurationStrategies = Object.freeze({
  adaptive,
  preview: Object.freeze<ConfigPreviewStrategy>({
    projectNodes: (preview) => {
      const realized = preview.targetOptions?.shadowrocket;
      return realized?.options ?? preview.options.filter((node) => (
        !conflictsWithShadowrocketBuiltinRulePolicy(node.name)
      ));
    },
    relationNodeNames: (nodes, selected) => selected
      ? nodes?.map((node) => node.name)
      : [],
    validate: ({ preview }) => {
      const realized = preview?.targetOptions?.shadowrocket;
      const valid = realized === undefined
        || preview?.renderCandidates.length === 0
        || realized.options.length > 0;
      return valid
        ? { valid }
        : { valid, issueKey: "files.config.shadowrocketNoRenderableNodes" };
    },
  }),
  references: Object.freeze<ConfigReferenceStrategy>({
    groupBuiltins: shadowrocketGroupBuiltinPolicies,
    includeNode: (node) => !conflictsWithShadowrocketBuiltinRulePolicy(node.name),
    rulePolicyBuiltins: shadowrocketRuleBuiltinPolicies,
  }),
  relations,
  templates: shadowrocketTemplates(adaptive),
});

function shadowrocketRelations(): ConfigRelationStrategy {
	const strategy: ConfigRelationStrategy = {
    project(groups, ruleSets, rules, nodeNames): ConfigRelationProjection {
      const groupIdentities = groups.map((group, index) => relationIdentity("group", index, group.name));
      const ruleSetIdentities = ruleSets.map((ruleSet, index) => relationIdentity("ruleset", index, ruleSet.name));
      const knownGroups = new Set(groupIdentities.map((group) => group.name).filter(Boolean));
      const knownRuleSets = new Set(ruleSetIdentities.map((ruleSet) => ruleSet.name).filter(Boolean));
      const knownNodes = new Set(nodeNames?.map((name) => name.trim()).filter(Boolean) ?? []);
      const allowedGroupTargets = new Set<string>(["$nodes", ...shadowrocketGroupBuiltinPolicies, ...knownNodes]);
      const allowedRulePolicies = new Set<string>([...shadowrocketRuleBuiltinPolicies, ...knownNodes]);
      const severity = nodeNames === undefined ? "warning" : "error";
      const issues = shadowrocketFieldIssues(groups, ruleSets, nodeNames);
      const events = groups.flatMap((group, index) => {
        const sourceGroup = groupIdentities[index].name;
        const projected = strictStringList(group.proxies).map((target) => relationReferenceEvent({
          allowed: nodeNames === undefined || allowedGroupTargets.has(target) || knownGroups.has(target),
          danglingIssue: relationIssue(
            severity,
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
              severity,
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
  nodeNames?: string[],
) {
  const issues: ConfigValidationIssue[] = [];
  const knownNodes = new Set(nodeNames?.map((name) => name.trim()).filter(Boolean) ?? []);
  for (const name of knownNodes) {
    if (conflictsWithShadowrocketBuiltinRulePolicy(name)) {
      issues.push({
        ...relationIssue("error", "shadowrocket_node_reserved_collision", "groups", "group-0", `Rendered node name "${name}" is reserved.`, name),
        messageKey: "files.config.issueShadowrocketNodeReserved",
        messageParams: { reference: name },
      });
    }
  }
  for (const [index, group] of groups.entries()) {
    const itemId = `group-${index}`;
    const name = trimmedString(group.name);
    if (name && (["#", ";", "["].some((prefix) => name.startsWith(prefix)) || /[\r\n=,]/.test(name))) {
      issues.push(relationIssue("error", "shadowrocket_group_name_invalid", "groups", itemId, "Group name must not start with #, ;, or [ and must not contain a line break, equals sign, or comma."));
    }
    if (name === "$nodes" || conflictsWithShadowrocketBuiltinRulePolicy(name)) {
      issues.push(relationIssue("error", "shadowrocket_group_name_reserved", "groups", itemId, `Group name "${name}" is reserved.`, name));
    }
    if (knownNodes.has(name)) {
      issues.push(relationIssue("error", "shadowrocket_group_node_collision", "groups", itemId, `Group name "${name}" conflicts with a rendered node.`, name));
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
    const expandedMembers = expandedMembersForValidation(group, nodeNames);
    if (expandedMembers?.length === 0 && Array.isArray(group.proxies) && group.proxies.length > 0) {
      issues.push(relationIssue("error", "shadowrocket_group_members_empty", "groups", itemId, "Group must contain at least one member after expanding subscription nodes."));
    }
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
    anchorProblem: (config) => anchorProblem(config.groups ?? []),
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
      moduleID !== "auto" && moduleID !== "ad"
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
    const selectName = blueprint.enabled.has("select") ? configGroupName("select", blueprint.namingLocale) : undefined;
    const autoName = blueprint.enabled.has("auto") ? configGroupName("auto", blueprint.namingLocale) : undefined;
    const targets = [...new Set([
      ...(item.id === "select" ? ["PROXY"] : []),
      ...templateGroupTargets(item, selectName, autoName, "DIRECT", "REJECT"),
    ].filter((target) => target !== name))];
    return item.groupMode === "url-test"
      ? { name, type: "url-test", proxies: targets, interval: 300, timeout: 5, tolerance: 50 }
      : { name, type: "select", proxies: targets };
  });
  const ruleEntries = blueprint.ruleEntries.filter(({ ruleID }) => canonicalRuleID(ruleID) === ruleID);
  const ruleSets = ruleEntries.map(({ ruleID }) => {
    const artifact = ruleArtifact(ruleID);
    return {
      name: ruleID,
      type: artifact.type,
      url: `${RULE_BASE}/${artifact.name}/${artifact.name}.list`,
    };
  });
  const rules = ruleEntries.map(({ module, ruleID }) => {
    const type = ruleArtifact(ruleID).type === "domain-set" ? "DOMAIN-SET" : "RULE-SET";
    return `${type},${ruleID},${configGroupName(module.id, blueprint.namingLocale)}${type === "RULE-SET" && ruleID.endsWith("-ip") ? ",no-resolve" : ""}`;
  });
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
  private: "Lan", "geolocation-cn": "China", "geolocation-!cn": "Global",
  openai: "OpenAI", anthropic: "Anthropic", youtube: "YouTube", google: "Google", microsoft: "Microsoft",
  onedrive: "OneDrive", apple: "Apple", icloud: "iCloud", github: "GitHub", gitlab: "GitLab", atlassian: "Atlassian",
  telegram: "Telegram", twitter: "Twitter", facebook: "Facebook", instagram: "Instagram", whatsapp: "Whatsapp",
  discord: "Discord", linkedin: "LinkedIn", tiktok: "TikTok", line: "Line", reddit: "Reddit", snap: "Snap",
  pinterest: "Pinterest", tumblr: "Tumblr", netflix: "Netflix", disney: "Disney", hbo: "HBO", hulu: "Hulu",
  primevideo: "AmazonPrimeVideo", "apple-tvplus": "AppleTV", spotify: "Spotify", twitch: "Twitch", dazn: "DAZN",
  bahamut: "Bahamut", biliintl: "BiliBiliIntl", niconico: "Niconico", abema: "Abema", viu: "ViuTV", kktv: "KKTV",
  steam: "Steam", epicgames: "Epic", ea: "EA", ubisoft: "Ubisoft", blizzard: "Blizzard", gog: "Gog", riot: "Riot",
  playstation: "PlayStation", xbox: "Xbox", nintendo: "Nintendo", cloudflare: "Cloudflare", digitalocean: "DigitalOcean",
  vercel: "Vercel", docker: "Docker", npmjs: "Npmjs", jetbrains: "Jetbrains", stackexchange: "Stackexchange",
  dropbox: "Dropbox", notion: "Notion", paypal: "PayPal", stripe: "Stripe", binance: "Binance",
  "category-scholar-!cn": "Scholar", wikimedia: "Wikimedia", bbc: "BBC", cnn: "CNN", nytimes: "NYTimes",
  bloomberg: "Bloomberg", amazon: "Amazon", ebay: "eBay",
};

const RULE_ALIASES: Readonly<Record<string, string>> = {
  "private-ip": "private", "cn-ip": "geolocation-cn", "category-ai-chat-!cn": "openai", "google-ip": "google",
  "telegram-ip": "telegram", "twitter-ip": "twitter", "facebook-ip": "facebook", "netflix-ip": "netflix",
  "cloudflare-ip": "cloudflare", aws: "amazon", azure: "microsoft", netlify: "vercel", wise: "paypal",
  coursera: "category-scholar-!cn", udemy: "category-scholar-!cn", edx: "category-scholar-!cn",
  khanacademy: "category-scholar-!cn", wsj: "bloomberg",
};

function canonicalRuleID(ruleID: string): string {
  return RULE_ALIASES[ruleID] ?? ruleID;
}

function ruleArtifact(ruleID: string): { name: string; type: "rule-set" | "domain-set" } {
  const name = ARTIFACT_NAMES[canonicalRuleID(ruleID)];
  if (!name) throw new Error(`Unknown Shadowrocket template rule: ${ruleID}`);
  return { name, type: "rule-set" };
}

function policyFilter(item: { filter: string; excludeFilter?: string }): string {
  if (!item.excludeFilter) return item.filter;
  const include = item.filter.replace(/^\(\?i\)/, "");
  const exclude = item.excludeFilter.replace(/^\(\?i\)/, "");
  return `(?i)^(?!.*(?:${exclude}))(?=.*(?:${include})).*$`;
}

function anchorProblem(groups: readonly ConfigMap[]): AdaptiveGroupAnchorProblem | null {
  const anchors = groups.filter((group) => {
    const name = trimmedString(group.name);
    return name === configAnchorName("en-US") || name === configAnchorName("zh-CN");
  });
  if (anchors.length === 0) return { code: "anchor_missing" };
  if (anchors.length > 1) return { code: "anchor_duplicate", count: anchors.length };
  if (anchors[0].type !== "select") return { code: "anchor_type_invalid" };
  if (!Array.isArray(anchors[0].proxies)
    || anchors[0].proxies.some((member) => typeof member !== "string")) {
    return { code: "anchor_members_invalid" };
  }
  return null;
}

function expandedMembersForValidation(
  group: Record<string, unknown>,
  nodeNames?: string[],
): string[] | undefined {
  if (typeof group["policy-regex-filter"] === "string") return undefined;
  const targets = strictStringList(group.proxies);
  if (nodeNames === undefined && targets.includes("$nodes")) return undefined;
  return [...new Set(targets
    .flatMap((target) => target === "$nodes" ? nodeNames ?? [] : [target])
    .map((target) => target.trim())
    .filter(Boolean))];
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
