import type { NavigateFunction } from "react-router";

import { fileDriver } from "~/features/files/drivers/registry";
import type { FileDetail, FileItem } from "~/features/files/model/types";
import type { ApiClient, FileSpecInput } from "~/shared/api/client";
import { defaultTranslator, type Translator } from "~/shared/i18n/context";
import { fileEditPath } from "~/shared/routing/paths";

type FileNotice = (message: string, severity?: "success" | "error" | "warning") => void;

export function createFileActions({
  client,
  closeSheet,
  navigate,
  refreshResources,
  showNotice,
  t = defaultTranslator(),
}: {
  client: ApiClient;
  closeSheet: () => void;
  navigate: NavigateFunction;
  refreshResources: () => Promise<void>;
  showNotice: FileNotice;
  t?: Translator;
}) {
  async function createFile(kind: string, form: FormData, existing?: FileItem) {
    const name = String(form.get("name") ?? "").trim();
    if (!name) throw new Error("file name is required");
    const payload = buildFilePayload(name, kind, form, fileTimestamps(existing, new Date().toISOString()));
    await client.createFile(payload);
    await refreshResources();
    closeSheet();
    showNotice(t(existing ? "messages.fileOverwritten" : "messages.fileSaved"));
    navigate(fileEditPath(name));
  }

  async function saveFileEdit(item: FileItem, form: FormData, detail?: FileDetail | null) {
    const existingKind = detail ? detail.kind : item.kind;
    assertRegisteredFileKind(existingKind);
    if ((detail?.config?.subscriptions?.length ?? 0) > 1) throw new Error("multiple subscriptions must be combined in a collection subscription before saving");
    const payload = buildFilePayload(item.name, existingKind, form, fileTimestamps(detail, new Date().toISOString()));
    await client.createFile(payload);
    await refreshResources();
    showNotice(t("messages.fileSaved"));
  }

  return {
    createFile,
    saveFileEdit,
  };
}

function buildFilePayload(
  name: string,
  kind: string,
  form: FormData,
  timestamps: Pick<FileSpecInput, "created_at" | "updated_at">,
): FileSpecInput {
  assertRegisteredFileKind(kind);
  const displayName = String(form.get("display_name") ?? "").trim();
  const description = String(form.get("description") ?? "").trim();
  const config = parseOptionalObjectField(form, "config");
  assertSingleSubscription(config);
  return {
    name,
    display_name: displayName || undefined,
    ...timestamps,
    kind,
    source: parseOptionalObjectField(form, "source") ?? { type: "inline", content: "" },
    config,
    processors: parseArrayField(form, "processors"),
    meta: {
      ...(description ? { description } : {}),
      ui: "web",
    },
  };
}

function assertRegisteredFileKind(kind: string): void {
  if (!fileDriver(kind)) throw new Error(`unregistered file kind: ${kind || "(missing)"}`);
}

function assertSingleSubscription(config: Record<string, unknown> | undefined): void {
  if (Array.isArray(config?.subscriptions) && config.subscriptions.length > 1) {
    throw new Error("multiple subscriptions must be combined in a collection subscription before saving");
  }
}

function fileTimestamps(detail: Pick<FileDetail, "createdAt" | "updatedAt"> | null | undefined, now: string): Pick<FileSpecInput, "created_at" | "updated_at"> {
  if (!detail) {
    return { created_at: now, updated_at: now };
  }
  return {
    created_at: detail.createdAt || detail.updatedAt || now,
    updated_at: now,
  };
}

function parseJSONField(form: FormData, name: string): unknown {
  const raw = String(form.get(name) ?? "").trim();
  if (!raw) return undefined;
  try {
    return JSON.parse(raw) as unknown;
  } catch {
    throw new Error(`${name} must contain valid JSON`);
  }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function parseOptionalObjectField(form: FormData, name: string): Record<string, unknown> | undefined {
  const parsed = parseJSONField(form, name);
  if (parsed === undefined) return undefined;
  if (!isObject(parsed)) throw new Error(`${name} must be a JSON object`);
  return parsed;
}

function parseArrayField(form: FormData, name: string): Array<Record<string, unknown>> {
  const parsed = parseJSONField(form, name);
  if (parsed === undefined) return [];
  if (!Array.isArray(parsed) || !parsed.every(isObject)) {
    throw new Error(`${name} must be a JSON array of objects`);
  }
  return parsed;
}
