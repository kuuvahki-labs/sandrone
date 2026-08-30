import type { GroupDraft, RuleSetDraft } from "./editor-model";
import type { ConfigNodeSummary } from "./node-source";

export type ConfigReferenceKind = "builtin" | "group" | "macro" | "node";

export interface ConfigReferenceOption {
  detail?: string;
  kind: ConfigReferenceKind;
  value: string;
}

export interface ConfigReferenceStrategy {
  groupBuiltins: readonly string[];
  includeAllNodesMacro?: boolean;
  includeNode: (node: ConfigNodeSummary) => boolean;
  rulePolicyBuiltins: readonly string[];
}

export function memberReferenceOptions(
  strategy: Readonly<ConfigReferenceStrategy>,
  nodes: readonly ConfigNodeSummary[],
  groups: readonly Pick<GroupDraft, "name">[],
  currentGroup: string,
): ConfigReferenceOption[] {
  return uniqueReferenceOptions([
    ...(strategy.includeAllNodesMacro === false ? [] : [{ kind: "macro" as const, value: "$nodes" }]),
    ...nodeOptions(strategy, nodes),
    ...groupOptions(groups).filter((option) => option.value !== currentGroup),
    ...builtinOptions(strategy.groupBuiltins),
  ]);
}

export function policyReferenceOptions(
  strategy: Readonly<ConfigReferenceStrategy>,
  nodes: readonly ConfigNodeSummary[],
  groups: readonly Pick<GroupDraft, "name">[],
): ConfigReferenceOption[] {
  return uniqueReferenceOptions([
    ...nodeOptions(strategy, nodes),
    ...groupOptions(groups),
    ...builtinOptions(strategy.rulePolicyBuiltins),
  ]);
}

export function ruleSetReferenceOptions(
  ruleSets: Array<Pick<RuleSetDraft, "name">>,
): string[] {
  return [...new Set(ruleSets.map((ruleSet) => ruleSet.name.trim()).filter(Boolean))];
}

function nodeOptions(
  strategy: Readonly<ConfigReferenceStrategy>,
  nodes: readonly ConfigNodeSummary[],
): ConfigReferenceOption[] {
  return nodes.filter(strategy.includeNode).map((node) => ({
    kind: "node",
    value: node.name,
    detail: [node.type, node.endpoint].filter(Boolean).join(" · "),
  }));
}

function groupOptions(
  groups: readonly Pick<GroupDraft, "name">[],
): ConfigReferenceOption[] {
  return groups
    .map((group) => group.name.trim())
    .filter(Boolean)
    .map((value) => ({ kind: "group", value }));
}

function builtinOptions(values: readonly string[]): ConfigReferenceOption[] {
  return values.map((value) => ({ kind: "builtin", value }));
}

function uniqueReferenceOptions(
  options: ConfigReferenceOption[],
): ConfigReferenceOption[] {
  const seen = new Set<string>();
  return options.filter((option) => {
    if (seen.has(option.value)) return false;
    seen.add(option.value);
    return true;
  });
}
