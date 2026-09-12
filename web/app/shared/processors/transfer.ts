import type { ProcessorDetail } from "~/shared/resources/types";

export class ProcessorTransferError extends Error {
  constructor(readonly code: "invalid_json" | "invalid_processors") {
    super(code);
    this.name = "ProcessorTransferError";
  }
}

export function parseProcessorTransfer(text: string): ProcessorDetail[] {
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    throw new ProcessorTransferError("invalid_json");
  }
  if (isObject(value)) {
    value = "processors" in value ? value.processors : isObject(value.resource) ? value.resource.processors : undefined;
  }
  if (!Array.isArray(value) || !value.every(isProcessor)) {
    throw new ProcessorTransferError("invalid_processors");
  }
  return value;
}

export function serializeProcessorTransfer(processors: readonly ProcessorDetail[]): string {
  return JSON.stringify(processors, null, 2);
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isProcessor(value: unknown): value is ProcessorDetail {
  return isObject(value)
    && typeof value.type === "string" && value.type.trim().length > 0
    && (!("enabled" in value) || typeof value.enabled === "boolean")
    && (!("name" in value) || typeof value.name === "string")
    && (!("stage" in value) || typeof value.stage === "string")
    && (!("params" in value) || isObject(value.params));
}
