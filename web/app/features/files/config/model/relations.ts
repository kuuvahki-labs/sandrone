import type { TranslationKey, TranslationParams } from "~/shared/i18n/context";

export type ConfigIssueSeverity = "error" | "warning";
export type ConfigSectionID = "groups" | "rule_sets" | "rules";

export interface ConfigValidationIssue {
  severity: ConfigIssueSeverity;
  code: string;
  section: ConfigSectionID;
  itemId: string;
  message: string;
  messageKey?: TranslationKey;
  messageParams?: TranslationParams;
  reference?: string;
}

export interface ConfigRelationIdentity {
  itemId: string;
  name: string;
}

export type ConfigRelationReferenceRole =
  | "group-member"
  | "group-policy"
  | "rule-policy"
  | "rule-set";

export interface ConfigRelationReference {
  allowed: boolean;
  danglingIssue?: ConfigValidationIssue;
  itemId: string;
  missingIssue?: ConfigValidationIssue;
  role: ConfigRelationReferenceRole;
  section: ConfigSectionID;
  sourceGroup?: string;
  target: string;
}

export type ConfigRelationEvent =
  | { type: "issue"; issue: ConfigValidationIssue }
  | { type: "reference"; reference: ConfigRelationReference };

/** Target-neutral identities and edges projected by one structured driver. */
export interface ConfigRelationProjection {
  deferredIssues?: ConfigValidationIssue[];
  events: ConfigRelationEvent[];
  groups: ConfigRelationIdentity[];
  issues: ConfigValidationIssue[];
  ruleSets: ConfigRelationIdentity[];
}

export interface ConfigRelationStrategy {
  project: (
    groups: Record<string, unknown>[],
    ruleSets: Record<string, unknown>[],
    rules: unknown[],
    nodeNames?: string[],
  ) => ConfigRelationProjection;
}

export interface ConfigRelationModel {
  groupInboundReferences: Record<string, number>;
  ruleSetInboundReferences: Record<string, number>;
  issues: ConfigValidationIssue[];
}

/**
 * Computes only shared graph facts. Native parsing, built-ins, target identity
 * rules, and target-specific validation belong to the driver projection.
 */
export function buildConfigRelationModel(
  projection: Readonly<ConfigRelationProjection>,
): ConfigRelationModel {
  const model: ConfigRelationModel = {
    groupInboundReferences: {},
    ruleSetInboundReferences: {},
    issues: [],
  };
  const groupOccurrences = countNames(projection.groups);
  const ruleSetOccurrences = countNames(projection.ruleSets);
  addNameIssues(model.issues, projection.groups, groupOccurrences, "group");
  addNameIssues(model.issues, projection.ruleSets, ruleSetOccurrences, "ruleset");
  model.issues.push(...projection.issues);

  model.groupInboundReferences = initialInboundCounts(projection.groups);
  model.ruleSetInboundReferences = initialInboundCounts(projection.ruleSets);
  const knownGroups = new Set(Object.keys(model.groupInboundReferences));
  const knownRuleSets = new Set(Object.keys(model.ruleSetInboundReferences));
  const uniqueGroups = new Set(
    [...groupOccurrences]
      .filter(([, count]) => count === 1)
      .map(([name]) => name),
  );
  const graph = new Map([...uniqueGroups].map((name) => [name, new Set<string>()]));

  for (const event of projection.events) {
    if (event.type === "issue") {
      model.issues.push(event.issue);
      continue;
    }
    const { reference } = event;
    if (reference.role === "rule-set") {
      if (!reference.target) {
        if (reference.missingIssue) model.issues.push(reference.missingIssue);
      } else if (knownRuleSets.has(reference.target)) {
        model.ruleSetInboundReferences[reference.target] += 1;
      } else if (!reference.allowed && reference.danglingIssue) {
        model.issues.push(reference.danglingIssue);
      }
      continue;
    }

    if (!reference.target) {
      if (reference.missingIssue) model.issues.push(reference.missingIssue);
      continue;
    }

    if (knownGroups.has(reference.target)) {
      model.groupInboundReferences[reference.target] += 1;
      if (reference.sourceGroup
        && uniqueGroups.has(reference.sourceGroup)
        && uniqueGroups.has(reference.target)) {
        graph.get(reference.sourceGroup)?.add(reference.target);
      }
    } else if (!reference.allowed && reference.danglingIssue) {
      model.issues.push(reference.danglingIssue);
    }
  }

  const cyclicGroups = findCyclicGroups(graph);
  for (const group of projection.groups) {
    if (!cyclicGroups.has(group.name)) continue;
    model.issues.push(relationIssue(
      "error",
      "group_reference_cycle",
      "groups",
      group.itemId,
      `Group "${group.name}" participates in a reference cycle.`,
      group.name,
    ));
  }
  model.issues.push(...(projection.deferredIssues ?? []));
  return model;
}

export function relationIssue(
  severity: ConfigIssueSeverity,
  code: string,
  section: ConfigSectionID,
  itemId: string,
  message: string,
  reference?: string,
): ConfigValidationIssue {
  return reference
    ? { severity, code, section, itemId, message, reference }
    : { severity, code, section, itemId, message };
}

function countNames(items: readonly ConfigRelationIdentity[]): Map<string, number> {
  const counts = new Map<string, number>();
  for (const item of items) {
    if (item.name) counts.set(item.name, (counts.get(item.name) ?? 0) + 1);
  }
  return counts;
}

function initialInboundCounts(items: readonly ConfigRelationIdentity[]): Record<string, number> {
  return Object.fromEntries(
    [...new Set(items.map((item) => item.name).filter(Boolean))].map((name) => [name, 0]),
  );
}

function addNameIssues(
  issues: ConfigValidationIssue[],
  items: readonly ConfigRelationIdentity[],
  counts: ReadonlyMap<string, number>,
  itemKind: "group" | "ruleset",
): void {
  const isGroup = itemKind === "group";
  const label = isGroup ? "Group" : "Rule-set";
  const section: ConfigSectionID = isGroup ? "groups" : "rule_sets";
  for (const item of items) {
    if (!item.name) {
      issues.push(relationIssue(
        "error",
        isGroup ? "group_name_empty" : "rule_set_name_empty",
        section,
        item.itemId,
        `${label} name is required.`,
      ));
    } else if ((counts.get(item.name) ?? 0) > 1) {
      issues.push(relationIssue(
        "error",
        isGroup ? "group_name_duplicate" : "rule_set_name_duplicate",
        section,
        item.itemId,
        `${label} name "${item.name}" is duplicated.`,
        item.name,
      ));
    }
  }
}

function findCyclicGroups(
  graph: ReadonlyMap<string, ReadonlySet<string>>,
): Set<string> {
  const cyclic = new Set<string>();
  for (const name of graph.keys()) {
    if (canReach(name, name, graph, new Set(), true)) cyclic.add(name);
  }
  return cyclic;
}

function canReach(
  current: string,
  target: string,
  graph: ReadonlyMap<string, ReadonlySet<string>>,
  visited: Set<string>,
  requireEdge: boolean,
): boolean {
  if (!requireEdge && current === target) return true;
  if (visited.has(current)) return false;
  visited.add(current);
  for (const next of graph.get(current) ?? []) {
    if (canReach(next, target, graph, visited, false)) return true;
  }
  return false;
}
