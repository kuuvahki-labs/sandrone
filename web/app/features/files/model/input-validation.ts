import type { ProcessorDetail } from "~/shared/resources/types";

import type { FileSourceDetail } from "./types";

export type FileInputValidationCode =
  | "source_json_invalid"
  | "source_json_object_required"
  | "source_remote_url_invalid";

export interface FileProcessorValidationIssue {
  code: "processor_json_invalid" | "processor_json_object_required" | "processor_json_override_array_required";
  index: number;
  path?: string;
}

export function validateRemoteConfigSource(source: FileSourceDetail): FileInputValidationCode | null {
  return source.type === "remote" && !isHTTPURL(source.remote?.url ?? "")
    ? "source_remote_url_invalid"
    : null;
}

export function validateJSONConfigSource(source: FileSourceDetail): FileInputValidationCode | null {
  if (!source.type && source.content === undefined && !source.remote?.url) return null;
  if (source.type === "remote") {
    return validateRemoteConfigSource(source);
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(source.content ?? "") as unknown;
  } catch {
    return "source_json_invalid";
  }
  return isRecord(parsed) ? null : "source_json_object_required";
}

export function validateJSONMergeProcessors(processors: ProcessorDetail[]): FileProcessorValidationIssue[] {
  const issues: FileProcessorValidationIssue[] = [];
  processors.forEach((processor, index) => {
    if (processor.enabled === false) return;
    if (processor.type !== "merge") return;
    const mode = processor.params?.mode;
    if (mode !== "json_overlay" && mode !== "json_override") return;
    let parsed: unknown;
    try {
      parsed = JSON.parse(typeof processor.params?.content === "string" ? processor.params.content : "") as unknown;
    } catch {
      issues.push({ code: "processor_json_invalid", index });
      return;
    }
    if (!isRecord(parsed)) {
      issues.push({ code: "processor_json_object_required", index });
      return;
    }
    if (mode === "json_override") validateOverrideObject(parsed, index, "", issues);
  });
  return issues;
}

function validateOverrideObject(
  value: Record<string, unknown>,
  index: number,
  parent: string,
  issues: FileProcessorValidationIssue[],
): void {
  for (const [rawKey, entry] of Object.entries(value)) {
    const path = parent ? `${parent}.${rawKey}` : rawKey;
    if (entry === null) continue;
    if (isArrayOperation(rawKey) && !Array.isArray(entry)) {
      issues.push({ code: "processor_json_override_array_required", index, path });
      continue;
    }
    if (isLiteralKey(rawKey) || isReplaceOperation(rawKey) || !isRecord(entry)) continue;
    validateOverrideObject(entry, index, path, issues);
  }
}

function isArrayOperation(key: string): boolean {
  return (key.startsWith("+") && key.length > 1) || (key.endsWith("+") && key.length > 1);
}

function isReplaceOperation(key: string): boolean {
  return key.endsWith("!") && key.length > 1;
}

function isLiteralKey(key: string): boolean {
  return key.length >= 2 && key.startsWith("<") && key.endsWith(">");
}

function isHTTPURL(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
