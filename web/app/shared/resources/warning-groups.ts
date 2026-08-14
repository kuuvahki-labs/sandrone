import type { PreviewWarning } from "~/shared/resources/types";

export interface PreviewWarningGroup {
  fingerprint: string;
  kind: "diagnostic" | "probe-failure";
  warning: PreviewWarning;
  warnings: readonly PreviewWarning[];
}

export function groupPreviewWarnings(warnings: readonly PreviewWarning[]): PreviewWarningGroup[] {
  const groups: Array<{
    fingerprint: string;
    kind: PreviewWarningGroup["kind"];
    warning: PreviewWarning;
    warnings: PreviewWarning[];
  }> = [];
  const groupsByFingerprint = new Map<string, (typeof groups)[number]>();

  for (const warning of warnings) {
    const kind = warningGroupKind(warning);
    const fingerprint = warningFingerprint(warning, kind);
    const existing = groupsByFingerprint.get(fingerprint);
    if (existing) {
      existing.warnings.push(warning);
      continue;
    }
    const group = { fingerprint, kind, warning, warnings: [warning] };
    groupsByFingerprint.set(fingerprint, group);
    groups.push(group);
  }

  return groups;
}

function warningGroupKind(warning: PreviewWarning): PreviewWarningGroup["kind"] {
  const hasNodeLocation = typeof warning.node === "string"
    || typeof warning.node_index === "number"
    || (typeof warning.node_context === "object" && warning.node_context !== null);
  return warning.code.startsWith("probe_") && hasNodeLocation ? "probe-failure" : "diagnostic";
}

function warningFingerprint(warning: PreviewWarning, kind: PreviewWarningGroup["kind"]): string {
  if (kind === "probe-failure") {
    return JSON.stringify([kind]);
  }
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
