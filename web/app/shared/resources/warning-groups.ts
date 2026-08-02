import type { PreviewWarning } from "~/shared/resources/types";

export interface PreviewWarningGroup {
  fingerprint: string;
  warning: PreviewWarning;
  warnings: readonly PreviewWarning[];
}

export function groupPreviewWarnings(warnings: readonly PreviewWarning[]): PreviewWarningGroup[] {
  const groups: Array<{
    fingerprint: string;
    warning: PreviewWarning;
    warnings: PreviewWarning[];
  }> = [];
  const groupsByFingerprint = new Map<string, (typeof groups)[number]>();

  for (const warning of warnings) {
    const fingerprint = warningFingerprint(warning);
    const existing = groupsByFingerprint.get(fingerprint);
    if (existing) {
      existing.warnings.push(warning);
      continue;
    }
    const group = { fingerprint, warning, warnings: [warning] };
    groupsByFingerprint.set(fingerprint, group);
    groups.push(group);
  }

  return groups;
}

function warningFingerprint(warning: PreviewWarning): string {
  return JSON.stringify([
    warning.code,
    warning.message,
    stringValue(warning.field),
    stringValue(warning.source),
    stringValue(warning.target),
  ]);
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}
