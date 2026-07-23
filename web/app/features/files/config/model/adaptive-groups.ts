import type { FileConfigDraft } from "~/features/files/model/types";

import {
  ADAPTIVE_REGION_GROUPS,
  adaptiveRegionName,
  type CanonicalGroupDefinition,
} from "./adaptive-regions";
import type { ConfigMap } from "./editor-model";
import {
  configAnchorName,
  type ConfigNamingLocale,
} from "./naming";

export type AdaptiveGroupType = string;

export interface AdaptiveGroupOptions {
  enabledRegionIds?: readonly string[];
  type: AdaptiveGroupType;
}

export interface AdaptiveGroupCandidate {
  active: boolean;
  excludeFilter?: string;
  filter: string;
  id: string;
  matchedNodeCount: number;
  name: string;
}

export type AdaptiveGroupWarning =
  | { code: "node_name_conflict"; groupName: string }
  | { code: "anchor_missing" }
  | { code: "anchor_duplicate"; count: number }
  | { code: "anchor_type_invalid" }
  | { code: "anchor_members_invalid" }
  | { code: "group_name_conflict"; groupName: string }
  | { code: "referenced_stale_group"; groupName: string }
  | { code: "empty_regions_skipped"; groupNames: string[] };

export interface AdaptiveGroupGeneration {
  candidates: AdaptiveGroupCandidate[];
  groups: ConfigMap[];
  namingLocale: ConfigNamingLocale;
  uniqueNodeCount: number;
  warnings: AdaptiveGroupWarning[];
}

export type AdaptiveGroupAnchorProblem = Extract<
  AdaptiveGroupWarning,
  { code: "anchor_missing" | "anchor_duplicate" | "anchor_type_invalid" | "anchor_members_invalid" }
>;

export interface AdaptiveGroupMergeResult {
  changed: boolean;
  config: FileConfigDraft;
  generatedGroupNames: string[];
  preservedGroupNames: string[];
  removedGroupNames: string[];
  warnings: AdaptiveGroupWarning[];
}

export interface AdaptiveGroupStripResult {
  changed: boolean;
  config: FileConfigDraft;
  strippedGroupNames: string[];
}

export interface AdaptiveGroupStaleInput {
  config: Readonly<FileConfigDraft>;
  editorMode: "wizard" | "advanced";
  enabled: boolean;
  namingLocale: ConfigNamingLocale;
  nodeNames?: readonly string[];
  options: Readonly<AdaptiveGroupOptions>;
}

export interface ConfigAdaptiveDialect {
  anchorProblem: (config: Readonly<FileConfigDraft>) => AdaptiveGroupAnchorProblem | null;
  canonicalName: (group: ConfigMap) => string | undefined;
  defaultType: string;
  groupMembers: (group: ConfigMap) => string[] | undefined;
  groupName: (group: ConfigMap) => string;
  inboundReferences: (config: Readonly<FileConfigDraft>) => Readonly<Record<string, number>>;
  materialize: (
    definition: CanonicalGroupDefinition,
    type: string,
    nodeNames: readonly string[],
  ) => ConfigMap;
  replaceGroupMembers: (group: ConfigMap, members: readonly string[]) => ConfigMap;
  requiresNodePreview: boolean;
  typeOptions: readonly { label: string; value: string }[];
}

export interface ConfigAdaptiveHelpers {
  anchorProblem: (config: Readonly<FileConfigDraft>) => AdaptiveGroupAnchorProblem | null;
  canonicalNames: (groups: readonly ConfigMap[]) => string[];
  defaultOptions: () => AdaptiveGroupOptions;
  generate: (
    nodeNames: readonly string[],
    options: Readonly<AdaptiveGroupOptions>,
    namingLocale?: ConfigNamingLocale,
  ) => AdaptiveGroupGeneration;
  merge: (
    config: Readonly<FileConfigDraft>,
    generation: Readonly<AdaptiveGroupGeneration>,
  ) => AdaptiveGroupMergeResult;
  requiresNodePreview: boolean;
  strip: (config: Readonly<FileConfigDraft>) => AdaptiveGroupStripResult;
  typeOptions: readonly { label: string; value: string }[];
}

interface CanonicalGroupEntry {
  index: number;
  name: string;
}

export const ADAPTIVE_REGION_IDS = ADAPTIVE_REGION_GROUPS.map((region) => region.id);
export const DEFAULT_ADAPTIVE_REGION_IDS = ["hk", "tw", "jp", "us", "sg"] as const;

export function adaptiveGroupHelpers(
  dialect: Readonly<ConfigAdaptiveDialect>,
): ConfigAdaptiveHelpers {
  const helpers: ConfigAdaptiveHelpers = {
    anchorProblem: dialect.anchorProblem,
    canonicalNames: (groups) => canonicalAdaptiveGroupNames(dialect, groups),
    defaultOptions: () => defaultAdaptiveGroupOptions(dialect),
    generate: (nodeNames, options, namingLocale = "en-US") => generateAdaptiveGroups(
      dialect,
      nodeNames,
      options,
      namingLocale,
    ),
    merge: (config, generation) => mergeAdaptiveGroups(dialect, config, generation),
    requiresNodePreview: dialect.requiresNodePreview,
    strip: (config) => stripCanonicalAdaptiveGroups(dialect, config),
    typeOptions: Object.freeze([...dialect.typeOptions]),
  };
  return Object.freeze(helpers);
}

export function defaultAdaptiveGroupOptions(
  dialect: Pick<ConfigAdaptiveDialect, "defaultType">,
): AdaptiveGroupOptions {
  return {
    type: dialect.defaultType,
    enabledRegionIds: [...DEFAULT_ADAPTIVE_REGION_IDS],
  };
}

export function adaptiveGroupOptionsFromValues(
  dialect: Pick<ConfigAdaptiveDialect, "defaultType" | "typeOptions">,
  values: Readonly<{
    enabledRegionIds?: readonly string[];
    type?: unknown;
  }> | undefined,
): AdaptiveGroupOptions {
  const defaults = defaultAdaptiveGroupOptions(dialect);
  if (!values) return defaults;
  const enabled = new Set(values.enabledRegionIds ?? []);
  return {
    type: isAdaptiveGroupType(dialect, values.type) ? values.type : defaults.type,
    enabledRegionIds: ADAPTIVE_REGION_IDS.filter((id) => enabled.has(id)),
  };
}

export function generateAdaptiveGroups(
  dialect: Readonly<ConfigAdaptiveDialect>,
  nodeNames: readonly string[],
  options: Readonly<AdaptiveGroupOptions>,
  namingLocale: ConfigNamingLocale = "en-US",
): AdaptiveGroupGeneration {
  if (!isAdaptiveGroupType(dialect, options.type)) {
    throw new RangeError("adaptive group type is invalid");
  }

  const uniqueNodeNames = [...new Set(nodeNames.filter((name) => name.trim()))];
  const candidates = ADAPTIVE_REGION_GROUPS.map((region) => candidate(
    region.id,
    adaptiveRegionName(region.id, namingLocale),
    region.filter,
    region.excludeFilter,
    uniqueNodeNames,
  ));
  const warnings: AdaptiveGroupWarning[] = [];
  const groups: ConfigMap[] = [];
  const emptyRegionNames: string[] = [];
  const enabledRegionIds = new Set(
    options.enabledRegionIds ?? candidates.filter((item) => item.active).map((item) => item.id),
  );

  for (const item of candidates) {
    if (!enabledRegionIds.has(item.id)) continue;
    if (uniqueNodeNames.includes(item.name)) {
      warnings.push({ code: "node_name_conflict", groupName: item.name });
      continue;
    }
    if (dialect.requiresNodePreview && !item.active) {
      emptyRegionNames.push(item.name);
      continue;
    }
    groups.push(dialect.materialize(
      item,
      options.type,
      adaptiveMatchingNodeNames(item, uniqueNodeNames),
    ));
  }
  if (emptyRegionNames.length > 0) {
    warnings.push({ code: "empty_regions_skipped", groupNames: emptyRegionNames });
  }
  return { candidates, groups, namingLocale, uniqueNodeCount: uniqueNodeNames.length, warnings };
}

export function sortAdaptiveCandidates(
  candidates: readonly AdaptiveGroupCandidate[],
  enabledRegionIds: readonly string[],
): AdaptiveGroupCandidate[] {
  const enabled = new Set(enabledRegionIds);
  const order = new Map(candidates.map((item, index) => [item.id, index]));
  return [...candidates].sort((left, right) => {
    const selectionDifference = Number(enabled.has(right.id)) - Number(enabled.has(left.id));
    if (selectionDifference !== 0) return selectionDifference;
    const countDifference = right.matchedNodeCount - left.matchedNodeCount;
    return countDifference !== 0
      ? countDifference
      : (order.get(left.id) ?? 0) - (order.get(right.id) ?? 0);
  });
}

export function canonicalAdaptiveGroupNames(
  dialect: Pick<ConfigAdaptiveDialect, "canonicalName">,
  groups: readonly ConfigMap[],
): string[] {
  return groups.flatMap((group) => {
    const name = dialect.canonicalName(group);
    return name ? [name] : [];
  });
}

export function mergeAdaptiveGroups(
  dialect: Readonly<ConfigAdaptiveDialect>,
  config: Readonly<FileConfigDraft>,
  generation: Readonly<AdaptiveGroupGeneration>,
): AdaptiveGroupMergeResult {
  const warnings = uniqueWarnings(generation.warnings);
  const anchorProblem = dialect.anchorProblem(config);
  if (anchorProblem) {
    pushUniqueWarning(warnings, anchorProblem);
    return unchangedMergeResult(config, warnings);
  }

  const currentGroups = config.groups ?? [];
  const anchorIndex = currentGroups.findIndex((group) => isAnchorGroup(dialect, group));
  const proxyTargets = dialect.groupMembers(currentGroups[anchorIndex]) ?? [];
  const groupsByName = indexGroupsByName(dialect, currentGroups);
  const existingCanonical = canonicalGroupEntries(dialect, currentGroups);
  const canonicalByName = indexCanonicalEntries(existingCanonical);
  const nodeNameConflicts = new Set(generation.warnings.flatMap((warning) => (
    warning.code === "node_name_conflict" ? [warning.groupName] : []
  )));
  const inboundReferences = dialect.inboundReferences(config);
  const desiredGroups: ConfigMap[] = [];
  const desiredNames = new Set<string>();

  for (const group of generation.groups) {
    const name = dialect.groupName(group);
    if (!name || desiredNames.has(name)) continue;
    const occurrences = groupsByName.get(name) ?? [];
    const hasConflict = occurrences.length > 1
      || (occurrences.length === 1 && dialect.canonicalName(occurrences[0]) === undefined);
    if (hasConflict) {
      pushUniqueWarning(warnings, { code: "group_name_conflict", groupName: name });
      continue;
    }
    desiredNames.add(name);
    desiredGroups.push(group);
  }

  const managedNames = new Set<string>();
  const removableIndexes = new Set<number>();
  const removedGroupNames: string[] = [];
  const preservedGroupNames: string[] = [];

  for (const [name, entries] of canonicalByName) {
    if (entries.length !== 1 || (groupsByName.get(name)?.length ?? 0) !== 1) {
      pushUniqueWarning(warnings, { code: "group_name_conflict", groupName: name });
      continue;
    }
    managedNames.add(name);
    const entry = entries[0];
    if (desiredNames.has(name)) {
      removableIndexes.add(entry.index);
      continue;
    }
    if (nodeNameConflicts.has(name)) {
      managedNames.delete(name);
      removableIndexes.add(entry.index);
      removedGroupNames.push(name);
      continue;
    }
    if (hasExternalReferences(name, inboundReferences, proxyTargets)) {
      preservedGroupNames.push(name);
      pushUniqueWarning(warnings, { code: "referenced_stale_group", groupName: name });
      continue;
    }
    removableIndexes.add(entry.index);
    removedGroupNames.push(name);
  }

  for (const name of desiredNames) managedNames.add(name);
  const rewrittenProxyTargets = proxyTargets.filter((target) => !managedNames.has(target));
  const generatedGroupNames = desiredGroups.map((group) => dialect.groupName(group));
  const nodesIndex = rewrittenProxyTargets.findIndex((target) => target === "$nodes");
  rewrittenProxyTargets.splice(
    nodesIndex < 0 ? rewrittenProxyTargets.length : nodesIndex,
    0,
    ...generatedGroupNames,
  );

  const retainedGroups = currentGroups.flatMap((group, index) => {
    if (removableIndexes.has(index)) return [];
    if (index === anchorIndex) return [dialect.replaceGroupMembers(group, rewrittenProxyTargets)];
    return [group];
  });
  const nextGroups = [
    ...retainedGroups,
    ...desiredGroups,
  ];
  const changed = !exactValueEqual(currentGroups, nextGroups);
  const nextConfig = changed ? { ...config, groups: nextGroups } : config;
  return {
    changed,
    config: nextConfig as FileConfigDraft,
    generatedGroupNames,
    preservedGroupNames,
    removedGroupNames,
    warnings,
  };
}

export function stripCanonicalAdaptiveGroups(
  dialect: Readonly<ConfigAdaptiveDialect>,
  config: Readonly<FileConfigDraft>,
): AdaptiveGroupStripResult {
  const currentGroups = config.groups ?? [];
  const canonicalEntries = canonicalGroupEntries(dialect, currentGroups);
  if (canonicalEntries.length === 0 || dialect.anchorProblem(config)) {
    return unchangedStripResult(config);
  }
  const canonicalByName = indexCanonicalEntries(canonicalEntries);
  if ([...canonicalByName.values()].some((entries) => entries.length > 1)) {
    return unchangedStripResult(config);
  }

  const anchorIndex = currentGroups.findIndex((group) => isAnchorGroup(dialect, group));
  const proxyTargets = dialect.groupMembers(currentGroups[anchorIndex]) ?? [];
  const inboundReferences = dialect.inboundReferences(config);
  const namesWithCustomReplacement = new Set<string>();
  for (const name of canonicalByName.keys()) {
    const hasCustomReplacement = currentGroups.some((group) => (
      dialect.groupName(group) === name && dialect.canonicalName(group) === undefined
    ));
    if (hasCustomReplacement) {
      namesWithCustomReplacement.add(name);
      continue;
    }
    if (hasExternalReferences(name, inboundReferences, proxyTargets)) {
      return unchangedStripResult(config);
    }
  }

  const canonicalIndexes = new Set(canonicalEntries.map((entry) => entry.index));
  const strippedGroupNames = canonicalEntries.map((entry) => entry.name);
  const namesToRemoveFromProxy = new Set(
    strippedGroupNames.filter((name) => !namesWithCustomReplacement.has(name)),
  );
  const nextProxyTargets = proxyTargets.filter((target) => !namesToRemoveFromProxy.has(target));
  const nextGroups = currentGroups.flatMap((group, index) => {
    if (canonicalIndexes.has(index)) return [];
    if (index === anchorIndex) return [dialect.replaceGroupMembers(group, nextProxyTargets)];
    return [group];
  });
  return {
    changed: true,
    config: { ...config, groups: nextGroups },
    strippedGroupNames,
  };
}

export function adaptiveMatchingNodeNames(
  item: Pick<CanonicalGroupDefinition, "filter" | "excludeFilter">,
  nodeNames: readonly string[],
): string[] {
  const include = browserRegex(item.filter);
  const exclude = item.excludeFilter ? browserRegex(item.excludeFilter) : null;
  return nodeNames.filter((nodeName) => include.test(nodeName) && !exclude?.test(nodeName));
}

export function adaptiveGroupsAreStale(
  dialect: Readonly<ConfigAdaptiveDialect>,
  input: Readonly<AdaptiveGroupStaleInput>,
): boolean {
  if (input.editorMode === "advanced" || !input.enabled) return false;
  if (canonicalAdaptiveGroupNames(dialect, input.config.groups ?? []).length === 0) return false;
  if (!input.nodeNames) return true;
  const generation = generateAdaptiveGroups(
    dialect,
    input.nodeNames,
    input.options,
    input.namingLocale,
  );
  const result = mergeAdaptiveGroups(dialect, input.config, generation);
  return result.changed
    || result.warnings.some((warning) => warning.code === "referenced_stale_group");
}

function isAdaptiveGroupType(
  dialect: Pick<ConfigAdaptiveDialect, "typeOptions">,
  value: unknown,
): value is string {
  return typeof value === "string"
    && dialect.typeOptions.some((option) => option.value === value);
}

function canonicalGroupEntries(
  dialect: Pick<ConfigAdaptiveDialect, "canonicalName">,
  groups: readonly ConfigMap[],
): CanonicalGroupEntry[] {
  return groups.flatMap((group, index) => {
    const name = dialect.canonicalName(group);
    return name ? [{ index, name }] : [];
  });
}

function indexCanonicalEntries(
  entries: readonly CanonicalGroupEntry[],
): Map<string, CanonicalGroupEntry[]> {
  const indexed = new Map<string, CanonicalGroupEntry[]>();
  for (const entry of entries) {
    indexed.set(entry.name, [...(indexed.get(entry.name) ?? []), entry]);
  }
  return indexed;
}

function indexGroupsByName(
  dialect: Pick<ConfigAdaptiveDialect, "groupName">,
  groups: readonly ConfigMap[],
): Map<string, ConfigMap[]> {
  const indexed = new Map<string, ConfigMap[]>();
  for (const group of groups) {
    const name = dialect.groupName(group);
    if (name) indexed.set(name, [...(indexed.get(name) ?? []), group]);
  }
  return indexed;
}

function isAnchorGroup(
  dialect: Pick<ConfigAdaptiveDialect, "groupName">,
  group: ConfigMap,
): boolean {
  const name = dialect.groupName(group);
  return name === configAnchorName("en-US") || name === configAnchorName("zh-CN");
}

function hasExternalReferences(
  name: string,
  inboundReferences: Readonly<Record<string, number>>,
  proxyTargets: readonly string[],
): boolean {
  const proxyReferenceCount = proxyTargets.filter((target) => target === name).length;
  return (inboundReferences[name] ?? 0) - proxyReferenceCount > 0;
}

function unchangedMergeResult(
  config: Readonly<FileConfigDraft>,
  warnings: AdaptiveGroupWarning[],
): AdaptiveGroupMergeResult {
  return {
    changed: false,
    config: config as FileConfigDraft,
    generatedGroupNames: [],
    preservedGroupNames: [],
    removedGroupNames: [],
    warnings,
  };
}

function unchangedStripResult(
  config: Readonly<FileConfigDraft>,
): AdaptiveGroupStripResult {
  return { changed: false, config: config as FileConfigDraft, strippedGroupNames: [] };
}

function uniqueWarnings(
  warnings: readonly AdaptiveGroupWarning[],
): AdaptiveGroupWarning[] {
  const unique: AdaptiveGroupWarning[] = [];
  for (const warning of warnings) pushUniqueWarning(unique, warning);
  return unique;
}

function pushUniqueWarning(
  warnings: AdaptiveGroupWarning[],
  warning: AdaptiveGroupWarning,
): void {
  if (!warnings.some((current) => exactValueEqual(current, warning))) warnings.push(warning);
}

function exactValueEqual(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) || Array.isArray(right)) {
    return Array.isArray(left)
      && Array.isArray(right)
      && left.length === right.length
      && left.every((value, index) => exactValueEqual(value, right[index]));
  }
  if (!isRecord(left) || !isRecord(right)) return false;
  const leftKeys = Object.keys(left);
  const rightKeys = Object.keys(right);
  return leftKeys.length === rightKeys.length
    && leftKeys.every((key) => Object.hasOwn(right, key) && exactValueEqual(left[key], right[key]));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function candidate(
  id: string,
  name: string,
  filter: string,
  excludeFilter: string | undefined,
  nodeNames: readonly string[],
): AdaptiveGroupCandidate {
  const matchedNodeCount = adaptiveMatchingNodeNames({ filter, excludeFilter }, nodeNames).length;
  return {
    active: matchedNodeCount > 0,
    ...(excludeFilter ? { excludeFilter } : {}),
    filter,
    id,
    matchedNodeCount,
    name,
  };
}

function browserRegex(pattern: string): RegExp {
  return new RegExp(pattern.replace(/^\(\?i\)/, ""), "i");
}
