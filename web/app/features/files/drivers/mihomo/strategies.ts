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
import {
  configAnchorName,
  configGroupName,
} from "~/features/files/config/model/naming";
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
  relationIdentity,
  relationReferenceEvent,
  strictStringList,
  trimmedString,
} from "~/features/files/drivers/core/strategy-helpers";
import type { FileConfigDraft } from "~/features/files/model/types";
import { DEFAULT_PROBE_URL } from "~/shared/probe/defaults";

const BUILTIN_POLICIES = ["DIRECT", "REJECT", "REJECT-DROP", "PASS", "PASS-RULE", "COMPATIBLE", "GLOBAL"] as const;
const ADAPTIVE_TYPE_OPTIONS = [
  { value: "select", label: "select" },
  { value: "url-test", label: "url-test" },
  { value: "load-balance", label: "load-balance" },
] as const;
const RULE_BASE = "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo";

const relations = mihomoRelations();
const adaptive = mihomoAdaptive(mihomoAdaptiveDialect(relations));

export const mihomoConfigurationStrategies = Object.freeze({
  adaptive,
  preview: Object.freeze<ConfigPreviewStrategy>({
    projectNodes: (preview) => preview.options,
    relationNodeNames: (nodes) => nodes?.map((node) => node.name),
    validate: () => ({ valid: true }),
  }),
  references: Object.freeze<ConfigReferenceStrategy>({
    groupBuiltins: BUILTIN_POLICIES,
    includeNode: () => true,
    rulePolicyBuiltins: BUILTIN_POLICIES,
  }),
  relations,
  templates: mihomoTemplates(adaptive),
});

function mihomoRelations(): ConfigRelationStrategy {
	const strategy: ConfigRelationStrategy = {
    project(groups, ruleSets, rules, nodeNames): ConfigRelationProjection {
      const groupIdentities = groups.map((group, index) => relationIdentity("group", index, group.name));
      const ruleSetIdentities = ruleSets.map((ruleSet, index) => relationIdentity("ruleset", index, ruleSet.name));
      const knownGroups = new Set(groupIdentities.map((group) => group.name).filter(Boolean));
      const knownRuleSets = new Set(ruleSetIdentities.map((ruleSet) => ruleSet.name).filter(Boolean));
      const allowedTargets = new Set<string>([
        "$nodes",
        ...BUILTIN_POLICIES,
        ...(nodeNames?.map((name) => name.trim()).filter(Boolean) ?? []),
      ]);
      const events = groups.flatMap((group, index) => {
        const sourceGroup = groupIdentities[index].name;
        return strictStringList(group.proxies).map((target) => relationReferenceEvent({
          allowed: nodeNames === undefined || allowedTargets.has(target) || knownGroups.has(target),
          danglingIssue: relationIssue(
            "warning",
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
      const deferredIssues = [];
      for (const [index, rule] of rules.entries()) {
        if (typeof rule !== "string") continue;
        const parts = rule.split(",").map((part) => part.trim());
        const type = (parts[0] ?? "").toUpperCase();
        if (type === "RULE-SET") {
          const target = parts[1] ?? "";
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
        const policyIndex = type === "MATCH"
          ? 1
          : type === "RULE-SET" || parts.length < 3
            ? 2
            : parts.at(-1)?.toLowerCase() === "no-resolve" ? parts.length - 2 : parts.length - 1;
        if (type) {
          const target = parts[policyIndex] ?? "";
          events.push(relationReferenceEvent({
            allowed: allowedTargets.has(target) || knownGroups.has(target),
            danglingIssue: relationIssue(
              "warning",
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
        if (type === "MATCH" && index < rules.length - 1) {
          deferredIssues.push(relationIssue(
            "warning",
            "final_rule_not_last",
            "rules",
            `rule-${index}`,
            "Final routing rule must be last.",
          ));
        }
      }
      return {
        deferredIssues,
        events,
        groups: groupIdentities,
        issues: [],
        ruleSets: ruleSetIdentities,
      };
    },
	};
	return Object.freeze(strategy);
}

function mihomoAdaptiveDialect(
  relationStrategy: ConfigRelationStrategy,
): ConfigAdaptiveDialect {
  const materialize: ConfigAdaptiveDialect["materialize"] = (item, type) => {
    const group: ConfigMap = {
      name: item.name,
      type,
      "include-all-proxies": true,
      filter: item.filter,
      ...(item.excludeFilter ? { "exclude-filter": item.excludeFilter } : {}),
    };
    if (type === "url-test" || type === "load-balance") {
      group.url = DEFAULT_PROBE_URL;
      group.interval = 300;
      group.lazy = true;
    }
    if (type === "url-test") group.tolerance = 50;
    if (type === "load-balance") group.strategy = "sticky-sessions";
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

function mihomoAdaptive(dialect: ConfigAdaptiveDialect): ConfigAdaptiveStrategy {
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

function mihomoTemplates(
  adaptiveStrategy: typeof adaptive,
): ConfigTemplateStrategy {
  return createConfigTemplateStrategy({
    groupNames: (groups) => groups.map((group) => trimmedString(group.name)).filter(Boolean),
    materialize: materializeMihomoTemplate,
    moduleIDs: (_id, moduleIDs) => moduleIDs,
    normalizeRecognition: (config) => {
      const adaptiveLayer = adaptiveStrategy.recognizesCanonicalLayer(config);
      const stripped = adaptiveLayer ? adaptiveStrategy.strip(config) : { changed: false, config, strippedGroupNames: [] };
      const { adaptive_groups: _adaptiveGroups, ...comparable } = stripped.config;
      return { adaptive: adaptiveLayer, config: comparable };
    },
  });
}

function materializeMihomoTemplate(
  blueprint: Readonly<ConfigTemplateBlueprint>,
): FileConfigDraft {
  const groups = blueprint.groups.map((item) => {
    const name = configGroupName(item.id, blueprint.namingLocale);
    const selectName = blueprint.enabled.has("select") ? configGroupName("select", blueprint.namingLocale) : undefined;
    const autoName = blueprint.enabled.has("auto") ? configGroupName("auto", blueprint.namingLocale) : undefined;
    const targets = templateGroupTargets(item, selectName, autoName, "DIRECT", "REJECT")
      .filter((target) => target !== name);
    return item.groupMode === "url-test"
      ? { name, type: "url-test", proxies: targets, url: DEFAULT_PROBE_URL, interval: 300, tolerance: 50 }
      : { name, type: "select", proxies: targets };
  });
  const ruleSets = blueprint.ruleEntries.map(({ ruleID }) => {
    const source = ruleSource(ruleID);
    return {
      name: ruleID,
      type: "http",
      behavior: source.behavior,
      format: "mrs",
      interval: 86400,
      url: `${RULE_BASE}/${source.directory}/${source.file}.mrs`,
    };
  });
  const domainEntries = blueprint.ruleEntries.filter(({ ruleID }) => !ruleID.endsWith("-ip"));
  const ipEntries = blueprint.ruleEntries.filter(({ ruleID }) => ruleID.endsWith("-ip"));
  const materializeRule = ({ module, ruleID }: ConfigTemplateBlueprint["ruleEntries"][number]) => (
    `RULE-SET,${ruleID},${configGroupName(module.id, blueprint.namingLocale)}${ruleID !== "cn-ip" && ruleID.endsWith("-ip") ? ",no-resolve" : ""}`
  );
  const rules = [
    `DST-PORT,853,${configGroupName("select", blueprint.namingLocale)}`,
    ...domainEntries.map(materializeRule),
    ...ipEntries.map(materializeRule),
  ];
  rules.push(`MATCH,${configGroupName("final", blueprint.namingLocale)}`);
  return {
    group_preset: "basic",
    ruleset_preset: "default",
    groups,
    rule_sets: ruleSets,
    rules,
  };
}

function ruleSource(ruleID: string): {
  behavior: "domain" | "ipcidr";
  directory: "geosite" | "geoip";
  file: string;
} {
  if (ruleID.endsWith("-ip")) {
    return { behavior: "ipcidr", directory: "geoip", file: ruleID.slice(0, -3) };
  }
  return { behavior: "domain", directory: "geosite", file: ruleID };
}

function anchorProblem(groups: readonly ConfigMap[]): AdaptiveGroupAnchorProblem | null {
  const anchors = groups.filter((group) => {
    const name = trimmedString(group.name);
    return name === configAnchorName("en-US") || name === configAnchorName("zh-CN");
  });
  if (anchors.length === 0) return { code: "anchor_missing" };
  if (anchors.length > 1) return { code: "anchor_duplicate", count: anchors.length };
  if (!Array.isArray(anchors[0].proxies)
    || anchors[0].proxies.some((member) => typeof member !== "string")) {
    return { code: "anchor_members_invalid" };
  }
  return null;
}
