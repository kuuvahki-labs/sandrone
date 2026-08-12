import type {
  AddCatalogRuleSetResult,
  ConfigEditorDraft,
  ConfigMap,
  GroupDraft,
  RuleDraft,
  RuleSetDraft,
} from "~/features/files/config/model/editor-model";
import { isRecord } from "~/features/files/config/model/editor-model";
import type { ConfigValidationIssue } from "~/features/files/config/model/relations";
import type { RuleSetCatalogItem } from "~/features/files/model/types";

export function draftID(prefix: string, index: number): string {
  return `${prefix}-${index}`;
}

export function stringList(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) return undefined;
  return value.map(String).filter(Boolean);
}

export function scalarString(value: unknown): string {
  return typeof value === "string" || typeof value === "number" ? String(value) : "";
}

export function ruleLines(value: string): string[] {
  return value.split(/\r?\n/).map((entry) => entry.trim()).filter(Boolean);
}

export function hasOnlyKeys(value: ConfigMap, allowed: readonly string[]): boolean {
  const keys = new Set(allowed);
  return Object.keys(value).every((key) => keys.has(key));
}

export function positiveInteger(value: string): number | undefined {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined;
}

export function isHTTPURL(value: string): boolean {
  try {
    const parsed = new URL(value);
    return (parsed.protocol === "http:" || parsed.protocol === "https:") && Boolean(parsed.host);
  } catch {
    return false;
  }
}

export function nativeValuesEqual(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) || Array.isArray(right)) {
    return Array.isArray(left)
      && Array.isArray(right)
      && left.length === right.length
      && left.every((value, index) => nativeValuesEqual(value, right[index]));
  }
  if (!isRecord(left) || !isRecord(right)) return false;
  const leftKeys = Object.keys(left);
  const rightKeys = Object.keys(right);
  return leftKeys.length === rightKeys.length
    && leftKeys.every((key) => Object.hasOwn(right, key) && nativeValuesEqual(left[key], right[key]));
}

export function catalogResult(
  entry: RuleSetCatalogItem,
  ruleSets: RuleSetDraft[],
  materialize: (entry: RuleSetCatalogItem, index: number) => RuleSetDraft,
): AddCatalogRuleSetResult {
  const name = entry.name.trim();
  const url = entry.url.trim();
  const duplicateURL = ruleSets.find((ruleSet) => ruleSet.source === "remote" && ruleSet.url.trim() === url);
  if (duplicateURL) return { status: "duplicate-url", existingName: duplicateURL.name };
  const nameConflict = ruleSets.find((ruleSet) => ruleSet.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase());
  if (nameConflict) return { status: "name-conflict", existingName: nameConflict.name };
  return { status: "added", ruleSets: [...ruleSets, materialize({ ...entry, name, url }, ruleSets.length)] };
}

export interface DraftValidationOptions {
  groups: {
    isHealthCheck: (type: string) => boolean;
    requireHealthCheckInterval: boolean;
    requireHealthCheckURL: boolean;
    supportsExcludeFilter: boolean;
    validateFilter: (value: unknown) => boolean;
    validate?: (group: GroupDraft, index: number) => ConfigValidationIssue[];
    validInterval?: (value: string) => boolean;
  };
  ruleSets: {
    validate?: (ruleSet: RuleSetDraft, index: number) => ConfigValidationIssue[];
    validInterval?: (value: string) => boolean;
  };
  rules: {
    requiresPolicy: (type: string) => boolean;
    requiresValue: (type: string) => boolean;
    validate?: (rule: RuleDraft, index: number) => ConfigValidationIssue[];
  };
}

export function validateDraft(
  draft: ConfigEditorDraft,
  options: DraftValidationOptions,
): ConfigValidationIssue[] {
  const issues: ConfigValidationIssue[] = [];
  draft.groups.forEach((group, index) => {
    if (group.memberMode === "fixed" ? group.members.length === 0 : !options.groups.validateFilter(group.filter)) {
      issues.push(issue("group_members_empty", "groups", `group-${index}`, "Proxy group members are required."));
    }
    if (group.memberMode === "runtime-filter" && (
      !options.groups.validateFilter(group.filter)
      || (options.groups.supportsExcludeFilter && Boolean(group.excludeFilter) && !options.groups.validateFilter(group.excludeFilter))
    )) {
      issues.push(issue("group_filter_invalid", "groups", `group-${index}`, "Proxy group filter is invalid."));
    }
    if (options.groups.isHealthCheck(group.type)) {
      if (options.groups.requireHealthCheckURL && !isHTTPURL(group.healthCheckURL)) {
        issues.push(issue("group_url_invalid", "groups", `group-${index}`, "Proxy group check URL must use HTTP(S)."));
      }
      if (options.groups.requireHealthCheckInterval && !options.groups.validInterval?.(group.healthCheckInterval)) {
        issues.push(issue("group_interval_invalid", "groups", `group-${index}`, "Proxy group check interval is required."));
      }
    }
    issues.push(...(options.groups.validate?.(group, index) ?? []));
  });
  draft.ruleSets.forEach((ruleSet, index) => {
    if (!ruleSet.name.trim()) issues.push(issue("rule_set_name_empty", "rule_sets", `ruleset-${index}`, "Rule-set name is required."));
    if (ruleSet.source === "remote") {
      if (!isHTTPURL(ruleSet.url)) issues.push(issue("rule_set_url_invalid", "rule_sets", `ruleset-${index}`, "Rule-set URL must use HTTP(S)."));
      if (options.ruleSets.validInterval && !options.ruleSets.validInterval(ruleSet.interval)) {
        issues.push(issue("rule_set_interval_invalid", "rule_sets", `ruleset-${index}`, "Rule-set update interval is required."));
      }
    } else if (!ruleSet.payloadText.trim()) {
      issues.push(issue("rule_set_payload_empty", "rule_sets", `ruleset-${index}`, "Inline rule-set content is required."));
    }
    issues.push(...(options.ruleSets.validate?.(ruleSet, index) ?? []));
  });
  draft.rules.forEach((rule, index) => {
    if (options.rules.requiresPolicy(rule.type) && !rule.policy.trim()) {
      issues.push(issue("rule_policy_empty", "rules", `rule-${index}`, "Rule policy is required."));
    }
    if (options.rules.requiresValue(rule.type) && !rule.value.trim()) {
      issues.push(issue("rule_value_empty", "rules", `rule-${index}`, "Rule match value is required."));
    }
    issues.push(...(options.rules.validate?.(rule, index) ?? []));
  });
  return issues;
}

export function issue(
  code: string,
  section: ConfigValidationIssue["section"],
  itemId: string,
  message: string,
): ConfigValidationIssue {
  return { severity: "error", code, section, itemId, message };
}
