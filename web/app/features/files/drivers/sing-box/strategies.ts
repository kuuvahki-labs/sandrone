import {
  type AdaptiveGroupAnchorProblem,
  adaptiveGroupHelpers,
  adaptiveGroupOptionsFromValues,
  adaptiveGroupsAreStale,
  adaptiveMatchingNodeNames,
  type ConfigAdaptiveDialect,
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
  countTrimmedStrings,
  isRecord,
  relationIdentity,
  relationIssueEvent,
  relationReferenceEvent,
  strictStringList,
  trimmedString,
} from "~/features/files/drivers/core/strategy-helpers";
import type { FileConfigDraft } from "~/features/files/model/types";
import { DEFAULT_PROBE_URL } from "~/shared/probe/defaults";

const BUILTIN_POLICIES = ["direct", "block"] as const;
const ADAPTIVE_TYPE_OPTIONS = [
  { value: "selector", label: "selector" },
  { value: "urltest", label: "urltest" },
] as const;
const RULE_BASE = "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo";

const relations = singBoxRelations();
const adaptive = singBoxAdaptive(singBoxAdaptiveDialect(relations));

export const singBoxConfigurationStrategies = Object.freeze({
  adaptive,
  preview: Object.freeze<ConfigPreviewStrategy>({
    projectNodes: (preview) => preview.options,
    relationNodeNames: (nodes) => nodes?.map((node) => node.name),
    validate: ({ formMode, preview, selected }) => ({
      valid: formMode !== "create" || Boolean(
        selected
        && preview
        && preview.options.length > 0
        && preview.duplicateNames.length === 0
        && preview.unnamedCount === 0,
      ),
    }),
  }),
  references: Object.freeze<ConfigReferenceStrategy>({
    groupBuiltins: BUILTIN_POLICIES,
    includeNode: () => true,
    rulePolicyBuiltins: BUILTIN_POLICIES,
  }),
  relations,
  templates: singBoxTemplates(adaptive),
});

function singBoxRelations(): ConfigRelationStrategy {
	const strategy: ConfigRelationStrategy = {
    project(groups, ruleSets, rules, nodeNames): ConfigRelationProjection {
      const groupIdentities = groups.map((group, index) => relationIdentity("group", index, group.tag));
      const ruleSetIdentities = ruleSets.map((ruleSet, index) => relationIdentity("ruleset", index, ruleSet.tag));
      const knownGroups = new Set(groupIdentities.map((group) => group.name).filter(Boolean));
      const knownRuleSets = new Set(ruleSetIdentities.map((ruleSet) => ruleSet.name).filter(Boolean));
      const knownNodes = new Set(nodeNames?.map((name) => name.trim()).filter(Boolean) ?? []);
      const allowedTargets = new Set<string>(["$nodes", ...BUILTIN_POLICIES, ...knownNodes]);
      const severity = nodeNames === undefined ? "warning" : "error";
      const events = groups.flatMap((group, index) => {
        const sourceGroup = groupIdentities[index].name;
        return strictStringList(group.outbounds).map((target) => relationReferenceEvent({
          allowed: nodeNames === undefined || allowedTargets.has(target) || knownGroups.has(target),
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
      });
      const issues = nodeNames === undefined
        ? []
        : singBoxIdentityIssues(groups, groupIdentities, nodeNames);
      if (nodeNames !== undefined) {
        events.push(...singBoxEmptyURLTestIssues(groups, nodeNames).map(relationIssueEvent));
      }
      const deferredIssues = [];
      for (const [index, value] of rules.entries()) {
        if (!isRecord(value)) continue;
        const ruleSetReferences = singBoxRuleSetReferences(value);
        for (const target of ruleSetReferences) {
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
        }
        if (Object.hasOwn(value, "outbound")) {
          const target = trimmedString(value.outbound);
          events.push(relationReferenceEvent({
            allowed: allowedTargets.has(target) || knownGroups.has(target),
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
          if (isFinalRule(value) && index < rules.length - 1) {
            deferredIssues.push(relationIssue(
              "warning",
              "final_rule_not_last",
              "rules",
              `rule-${index}`,
              "Final routing rule must be last.",
            ));
          }
        }
      }
      return { deferredIssues, events, groups: groupIdentities, issues, ruleSets: ruleSetIdentities };
    },
	};
	return Object.freeze(strategy);
}

function singBoxIdentityIssues(
  groups: Record<string, unknown>[],
  identities: Array<{ itemId: string; name: string }>,
  nodeNames: string[],
) {
  const issues = [];
  const anchorNames = new Set([configAnchorName("en-US"), configAnchorName("zh-CN")]);
  const anchors = identities.filter((group) => anchorNames.has(group.name));
  if (anchors.length === 0) {
    issues.push(relationIssue("error", "singbox_proxy_missing", "groups", "group-0", "A unique Proxy selector is required."));
  } else if (anchors.length > 1) {
    issues.push(relationIssue("error", "singbox_proxy_duplicate", "groups", "group-0", "Only one Proxy selector is allowed.", "Proxy"));
  } else {
    const anchorIndex = identities.findIndex((group) => anchorNames.has(group.name));
    if (anchorIndex >= 0 && trimmedString(groups[anchorIndex].type) !== "selector") {
      issues.push(relationIssue("error", "singbox_proxy_type_invalid", "groups", identities[anchorIndex].itemId, "The Proxy group must be a selector.", identities[anchorIndex].name));
    }
  }

  const nodeOccurrences = countTrimmedStrings(nodeNames);
  for (const [name, count] of nodeOccurrences) {
    if (count > 1) {
      issues.push(relationIssue("error", "singbox_node_tag_duplicate", "groups", "group-0", `Node tag "${name}" is duplicated.`, name));
    }
  }
  const knownNodes = new Set(nodeOccurrences.keys());
  const reserved = new Set<string>(BUILTIN_POLICIES);
  for (const name of knownNodes) {
    if (reserved.has(name)) {
      issues.push(relationIssue("error", "singbox_node_reserved_collision", "groups", "group-0", `Node tag "${name}" is reserved.`, name));
    }
  }
  for (const group of identities) {
    if (knownNodes.has(group.name)) {
      issues.push(relationIssue("error", "singbox_group_node_collision", "groups", group.itemId, `Group tag "${group.name}" conflicts with a node tag.`, group.name));
    }
    if (reserved.has(group.name)) {
      issues.push(relationIssue("error", "singbox_group_reserved_collision", "groups", group.itemId, `Group tag "${group.name}" is reserved.`, group.name));
    }
  }
  return issues;
}

function singBoxEmptyURLTestIssues(
  groups: Record<string, unknown>[],
  nodeNames: string[],
) {
  const issues = [];
  for (const [index, group] of groups.entries()) {
    if (trimmedString(group.type) !== "urltest") continue;
    const expanded = strictStringList(group.outbounds)
      .flatMap((target) => target === "$nodes" ? nodeNames : [target]);
    if (expanded.some((target) => target.trim())) continue;
    issues.push(relationIssue("error", "singbox_urltest_empty", "groups", `group-${index}`, "URLTest must contain at least one outbound after expanding subscription nodes."));
  }
  return issues;
}

function singBoxAdaptiveDialect(
  relationStrategy: ConfigRelationStrategy,
): ConfigAdaptiveDialect {
  const materialize: ConfigAdaptiveDialect["materialize"] = (item, type, nodeNames) => {
    const group: ConfigMap = { tag: item.name, type, outbounds: [...nodeNames] };
    if (type === "urltest") {
      group.url = DEFAULT_PROBE_URL;
      group.interval = "5m";
      group.tolerance = 50;
    }
    return group;
  };
  return {
    anchorProblem: (config) => anchorProblem(config.groups ?? []),
    canonicalName: (group) => {
      for (const definition of CANONICAL_ADAPTIVE_GROUP_DEFINITIONS) {
        const members = canonicalMembers(group, definition);
        if (!members) continue;
        for (const name of [definition.name, ...(definition.legacyNames ?? [])]) {
          for (const { value: type } of ADAPTIVE_TYPE_OPTIONS) {
            if (nativeValuesEqual(group, materialize({ ...definition, name }, type, members))) return name;
          }
        }
      }
      return undefined;
    },
    defaultType: "urltest",
    groupMembers: (group) => Array.isArray(group.outbounds)
      && group.outbounds.every((member) => typeof member === "string")
      ? group.outbounds as string[]
      : undefined,
    groupName: (group) => trimmedString(group.tag),
    inboundReferences: (config) => buildConfigRelationModel(relationStrategy.project(
      config.groups ?? [],
      [],
      config.rules ?? [],
    )).groupInboundReferences,
    materialize,
    replaceGroupMembers: (group, members) => ({ ...group, outbounds: [...members] }),
    requiresNodePreview: true,
    typeOptions: ADAPTIVE_TYPE_OPTIONS,
  };
}

function singBoxAdaptive(dialect: ConfigAdaptiveDialect): ConfigAdaptiveStrategy {
  const helpers = adaptiveGroupHelpers(dialect);
  const strategy: ConfigAdaptiveStrategy = {
    ...helpers,
    configFromOptions: () => undefined,
    initiallyEnabled: () => false,
    isStale: (input) => adaptiveGroupsAreStale(dialect, input),
    optionsFromConfig: (config) => adaptiveGroupOptionsFromValues(dialect, config ? {
      enabledRegionIds: config.regions,
      type: config.type,
    } : undefined),
    recognizesCanonicalLayer: () => false,
  };
  return Object.freeze(strategy);
}

function singBoxTemplates(
  adaptiveStrategy: typeof adaptive,
): ConfigTemplateStrategy {
  return createConfigTemplateStrategy({
    groupNames: (groups) => groups.map((group) => trimmedString(group.tag)).filter(Boolean),
    materialize: materializeSingBoxTemplate,
    moduleIDs: (_id, moduleIDs) => moduleIDs.filter((moduleID) => moduleID !== "fallback"),
    normalizeRecognition: (config) => {
      const adaptiveLayer = adaptiveStrategy.recognizesCanonicalLayer(config);
      const stripped = adaptiveLayer ? adaptiveStrategy.strip(config) : { changed: false, config, strippedGroupNames: [] };
      const { adaptive_groups: _adaptiveGroups, ...comparable } = stripped.config;
      return { adaptive: adaptiveLayer, config: comparable };
    },
  });
}

function materializeSingBoxTemplate(
  blueprint: Readonly<ConfigTemplateBlueprint>,
): FileConfigDraft {
  const groups = blueprint.groups.map((item) => {
    const tag = configGroupName(item.id, blueprint.namingLocale);
    const selectName = blueprint.enabled.has("select") ? configGroupName("select", blueprint.namingLocale) : undefined;
    const autoName = blueprint.enabled.has("auto") ? configGroupName("auto", blueprint.namingLocale) : undefined;
    const outbounds = templateGroupTargets(item, selectName, autoName, "direct", "block")
      .filter((target) => target !== tag);
    return item.groupMode === "url-test"
      ? { type: "urltest", tag, outbounds, url: DEFAULT_PROBE_URL, interval: "5m" }
      : { type: "selector", tag, outbounds };
  });
  const ruleSets = blueprint.ruleEntries.map(({ ruleID }) => {
    const source = ruleSource(ruleID);
    return {
      type: "remote",
      tag: ruleID,
      format: source.format,
      update_interval: "1d",
      url: source.url,
    };
  });
  const domainEntries = blueprint.ruleEntries.filter(({ ruleID }) => !ruleID.endsWith("-ip"));
  const ipEntries = blueprint.ruleEntries.filter(({ ruleID }) => ruleID.endsWith("-ip"));
  const materializeRule = ({ module, ruleID }: ConfigTemplateBlueprint["ruleEntries"][number]) => ({
    rule_set: [ruleID],
    outbound: configGroupName(module.id, blueprint.namingLocale),
  });
  const rules: unknown[] = [
    { port: 853, outbound: configGroupName("select", blueprint.namingLocale) },
    ...domainEntries.map(materializeRule),
    { action: "resolve" },
    ...ipEntries.map(materializeRule),
  ];
  rules.push({ outbound: configGroupName("final", blueprint.namingLocale) });
  return {
    group_preset: "basic",
    ruleset_preset: "default",
    groups,
    rule_sets: ruleSets,
    rules,
  };
}

function canonicalMembers(
  group: ConfigMap,
  definition: (typeof CANONICAL_ADAPTIVE_GROUP_DEFINITIONS)[number],
): string[] | undefined {
  if (!Array.isArray(group.outbounds) || group.outbounds.length === 0) return undefined;
  if (group.outbounds.some((member) => typeof member !== "string" || !member.trim())) return undefined;
  const members = group.outbounds as string[];
  if (new Set(members).size !== members.length) return undefined;
  return adaptiveMatchingNodeNames(definition, members).length === members.length ? members : undefined;
}

function anchorProblem(groups: readonly ConfigMap[]): AdaptiveGroupAnchorProblem | null {
  const anchors = groups.filter((group) => {
    const name = trimmedString(group.tag);
    return name === configAnchorName("en-US") || name === configAnchorName("zh-CN");
  });
  if (anchors.length === 0) return { code: "anchor_missing" };
  if (anchors.length > 1) return { code: "anchor_duplicate", count: anchors.length };
  if (anchors[0].type !== "selector") return { code: "anchor_type_invalid" };
  if (!Array.isArray(anchors[0].outbounds)
    || anchors[0].outbounds.some((member) => typeof member !== "string")) {
    return { code: "anchor_members_invalid" };
  }
  return null;
}

function singBoxRuleSetReferences(rule: Record<string, unknown>): string[] {
  if (!Object.hasOwn(rule, "rule_set")) return [];
  if (typeof rule.rule_set === "string") return [trimmedString(rule.rule_set)];
  if (!Array.isArray(rule.rule_set) || rule.rule_set.length === 0) return [""];
  return rule.rule_set.map(trimmedString);
}

function isFinalRule(rule: Record<string, unknown>): boolean {
  const routingKeys = new Set(["outbound", "action", "override_address", "override_port", "network_strategy", "fallback_delay"]);
  return Object.keys(rule).every((key) => routingKeys.has(key));
}

interface SingBoxTemplateRuleSource {
  format: "binary";
  url: string;
}

function ruleSource(ruleID: string): SingBoxTemplateRuleSource {
  const directory = ruleID.endsWith("-ip") ? "geoip" : "geosite";
  const file = ruleID.endsWith("-ip") ? ruleID.slice(0, -3) : ruleID;
  return { format: "binary", url: `${RULE_BASE}/${directory}/${file}.srs` };
}
