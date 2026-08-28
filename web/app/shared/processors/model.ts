import type { ProcessorDetail } from "~/shared/resources/types";

export type ProcessorDraft = {
  enabled: boolean;
  id: string;
  name: string;
  type: string;
  params: Record<string, unknown>;
	opaque?: ProcessorDetail;
};

export function createProcessorDraftId(prefix: string, index = Date.now()): string {
  return `${prefix}-${index}-${Math.random().toString(36).slice(2)}`;
}

export function processorLabel(type: string): string {
  switch (type) {
    case "script": return "script";
    default: return type || "processor";
  }
}

export function processorIsEnabled(value: unknown): boolean {
  return value !== false;
}

export function processorEnabledProperty(enabled: boolean): Pick<ProcessorDetail, "enabled"> {
  return enabled ? {} : { enabled: false };
}

export function cleanParams(params: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(Object.entries(params).filter(([, value]) => {
    if (value === undefined || value === null || value === "") return false;
    if (Array.isArray(value)) return value.length > 0;
    if (typeof value === "object") return Object.keys(value).length > 0;
    return true;
  }));
}

export function customProcessorName(draft: ProcessorDraft, labelForType: (type: string) => string): string {
  const name = draft.name.trim();
  return name && name !== labelForType(draft.type) ? name : "";
}

export function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

export function arrayValue(value: unknown): string[] {
  return Array.isArray(value) ? value.map((entry) => String(entry)).filter(Boolean) : [];
}

export function listToText(value: unknown): string {
  return arrayValue(value).join(", ");
}

export function textToList(value: string): string[] {
  return value.split(",").map((entry) => entry.trim()).filter(Boolean);
}

export function listToLines(value: unknown): string {
  return arrayValue(value).join("\n");
}

export function linesToList(value: string): string[] {
  return value.split(/\r?\n|,/).map((entry) => entry.trim()).filter(Boolean);
}

export function numberInputValue(value: unknown): string | number {
  return typeof value === "number" && Number.isFinite(value) ? value : "";
}

export function numberOrEmpty(value: string): number | "" {
  if (!value.trim()) return "";
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : "";
}

export function objectToKeyValueText(value: unknown): string {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return "";
  return Object.entries(value).map(([key, entry]) => `${key}=${formatKeyValue(entry)}`).join("\n");
}

export function keyValueTextToObject(value: string): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const line of value.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const index = trimmed.indexOf("=");
    if (index <= 0) continue;
    out[trimmed.slice(0, index).trim()] = parseKeyValue(trimmed.slice(index + 1).trim());
  }
  return out;
}

export function keyValueTextToReplacementPatch(current: Record<string, unknown>, value: string): Record<string, unknown> {
  const next = keyValueTextToObject(value);
  for (const key of Object.keys(current)) {
    if (!(key in next)) {
      next[key] = undefined;
    }
  }
  return next;
}

function parseKeyValue(value: string): unknown {
  if (value === "true") return true;
  if (value === "false") return false;
  if (/^-?\d+(\.\d+)?$/.test(value)) return Number(value);
  if ((value.startsWith("{") && value.endsWith("}")) || (value.startsWith("[") && value.endsWith("]"))) {
    try {
      return JSON.parse(value) as unknown;
    } catch {
      return value;
    }
  }
  return value;
}

function formatKeyValue(value: unknown): string {
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") return String(value);
  return JSON.stringify(value);
}
