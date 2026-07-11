import type {
  ConfigRelationEvent,
  ConfigRelationIdentity,
  ConfigRelationReference,
  ConfigRelationReferenceRole,
  ConfigValidationIssue,
} from "~/features/files/config/model/relations";

export function relationIdentity(
  prefix: "group" | "ruleset",
  index: number,
  name: unknown,
): ConfigRelationIdentity {
  return { itemId: `${prefix}-${index}`, name: trimmedString(name) };
}

export function relationReferenceEvent(input: {
  allowed?: boolean;
  danglingIssue?: ConfigValidationIssue;
  itemId: string;
  missingIssue?: ConfigValidationIssue;
  role: ConfigRelationReferenceRole;
  section: ConfigRelationReference["section"];
  sourceGroup?: string;
  target: unknown;
}): ConfigRelationEvent {
  return {
    type: "reference",
    reference: {
      allowed: input.allowed ?? false,
      ...(input.danglingIssue ? { danglingIssue: input.danglingIssue } : {}),
      itemId: input.itemId,
      ...(input.missingIssue ? { missingIssue: input.missingIssue } : {}),
      role: input.role,
      section: input.section,
      ...(input.sourceGroup ? { sourceGroup: input.sourceGroup } : {}),
      target: trimmedString(input.target),
    },
  };
}

export function relationIssueEvent(issue: ConfigValidationIssue): ConfigRelationEvent {
  return { type: "issue", issue };
}

export function trimmedString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

export function strictStringList(value: unknown): string[] {
  return Array.isArray(value)
    ? value.map(trimmedString).filter(Boolean)
    : [];
}

export function countTrimmedStrings(values: readonly string[]): Map<string, number> {
  const counts = new Map<string, number>();
  for (const value of values) {
    const name = value.trim();
    if (name) counts.set(name, (counts.get(name) ?? 0) + 1);
  }
  return counts;
}

export function isHTTPURL(value: string): boolean {
  try {
    const url = new URL(value);
    return (url.protocol === "http:" || url.protocol === "https:") && Boolean(url.host);
  } catch {
    return false;
  }
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
