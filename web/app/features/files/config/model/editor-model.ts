import type { FileAdaptiveGroupConfigDetail, RuleSetCatalogItem } from "~/features/files/model/types";

export type ConfigMap = Record<string, unknown>;

export type ProxyGroupMemberMode = "fixed" | "runtime-filter";

export type RuleSetSource = "inline" | "remote";

export interface StructureSectionPresence {
  groups: boolean;
  ruleSets: boolean;
  rules: boolean;
}

/** Target-neutral proxy-group state consumed by the shared workbench. */
export interface GroupDraft {
  adapterState?: unknown;
  excludeFilter: string;
  filter: string;
  healthCheckInterval: string;
  healthCheckTimeout?: number;
  healthCheckTolerance?: number;
  healthCheckURL: string;
  hidden?: boolean;
  id: string;
  memberMode: ProxyGroupMemberMode;
  members: string[];
  name: string;
  policySelectName?: string;
  selectedIndex?: number;
  type: string;
}

/** Semantic proxy-group state exposed to driver-owned UI fields. */
export type GroupFieldsDraft = Omit<GroupDraft, "adapterState">;

export function toGroupFieldsDraft(group: Readonly<GroupDraft>): GroupFieldsDraft {
  const { adapterState: _adapterState, ...draft } = group;
  return draft;
}

export function applyGroupFieldsPatch(
  group: Readonly<GroupDraft>,
  patch: Readonly<Partial<GroupFieldsDraft>>,
): GroupDraft {
  const { adapterState: _adapterState, ...semanticPatch } = patch as Partial<GroupDraft>;
  return { ...group, ...semanticPatch };
}

export interface RuleSetDraft {
  behavior: string;
  format: string;
  id: string;
  interval: string;
  name: string;
  payloadText: string;
  source: RuleSetSource;
  url: string;
}

export type AddCatalogRuleSetResult =
  | { status: "added"; ruleSets: RuleSetDraft[] }
  | { status: "duplicate-url"; existingName: string }
  | { status: "name-conflict"; existingName: string };

export interface AddCatalogRuleSetRequest {
  entry: RuleSetCatalogItem;
}

export interface RuleDraft {
  id: string;
  noResolve?: boolean;
  policy: string;
  type: string;
  value: string;
}

/** Complete target-neutral state for the shared structured editor lifecycle. */
export interface ConfigEditorDraft {
  adaptiveGroups?: FileAdaptiveGroupConfigDetail;
  advancedGroupsText: string;
  advancedRuleSetsText: string;
  advancedRulesText: string;
  groupPreset: string;
  groups: GroupDraft[];
  mode: "wizard" | "advanced";
  rawSettings?: unknown;
  ruleSetPreset: string;
  ruleSets: RuleSetDraft[];
  rules: RuleDraft[];
  sectionPresence: StructureSectionPresence;
  settingsMode: "structured" | "raw";
  subscriptions?: string[];
}

export function parseJSONList(text: string): { error?: string; value?: unknown[] } {
  try {
    const parsed = JSON.parse(text) as unknown;
    return Array.isArray(parsed) ? { value: parsed } : { error: "not-array" };
  } catch {
    return { error: "invalid-json" };
  }
}

export function isRecord(value: unknown): value is ConfigMap {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function stringField(value: unknown): string {
  return typeof value === "string" ? value : "";
}

export function configJSON(value: unknown): string {
  return JSON.stringify(value, null, 2);
}
