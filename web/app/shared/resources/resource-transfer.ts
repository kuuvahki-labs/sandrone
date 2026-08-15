export type TransferResourceType = "subscription" | "file";

export interface ResourceTransfer {
  resourceType: TransferResourceType;
  resource: Record<string, unknown>;
}

export type ResourceTransferErrorCode =
  | "invalid_json"
  | "invalid_envelope"
  | "missing_name"
  | "type_mismatch";

export class ResourceTransferError extends Error {
  readonly code: ResourceTransferErrorCode;

  constructor(code: ResourceTransferErrorCode) {
    super(code);
    this.name = "ResourceTransferError";
    this.code = code;
  }
}

export function serializeResourceTransfer(resourceType: TransferResourceType, value: unknown): string {
  const resource = requireResource(value);
  return `${JSON.stringify({ resource_type: resourceType, resource }, null, 2)}\n`;
}

export function parseResourceTransfer(text: string, expectedType: TransferResourceType): ResourceTransfer {
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    throw new ResourceTransferError("invalid_json");
  }
  const envelope = recordValue(value);
  if (!envelope) {
    throw new ResourceTransferError("invalid_envelope");
  }
  const resourceType = envelope.resource_type;
  if (resourceType !== "subscription" && resourceType !== "file") {
    throw new ResourceTransferError("invalid_envelope");
  }
  if (resourceType !== expectedType) {
    throw new ResourceTransferError("type_mismatch");
  }
  return {
    resourceType,
    resource: requireResource(envelope.resource),
  };
}

export function renameTransferredResource(resource: Record<string, unknown>, name: string): Record<string, unknown> {
  return { ...resource, name: name.trim() };
}

export function copyTransferredResource(resource: Record<string, unknown>, name: string, now = new Date()): Record<string, unknown> {
  const timestamp = now.toISOString();
  return {
    ...resource,
    name: name.trim(),
    created_at: timestamp,
    updated_at: timestamp,
  };
}

export function resourceTransferFilename(resourceType: TransferResourceType, name: string): string {
  const sanitizedName = [...name.trim()]
    .map((character) => character.charCodeAt(0) < 32 || '<>:"/\\|?*'.includes(character) ? "_" : character)
    .join("");
  const safeName = sanitizedName
    .replace(/[. ]+$/g, "") || "resource";
  return `${resourceType}-${safeName}.json`;
}

export function resourceNameIssue(name: string): "empty" | "invalid" | null {
  const value = name.trim();
  if (!value) return "empty";
  if (value === "." || value === ".." || value.includes("/") || value.includes("\\")) return "invalid";
  return null;
}

function requireResource(value: unknown): Record<string, unknown> {
  const resource = recordValue(value);
  if (!resource) {
    throw new ResourceTransferError("invalid_envelope");
  }
  if (typeof resource.name !== "string" || !resource.name.trim()) {
    throw new ResourceTransferError("missing_name");
  }
  return resource;
}

function recordValue(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}
