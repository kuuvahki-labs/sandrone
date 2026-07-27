import type { NavigateFunction } from "react-router";

import { fileDriver } from "~/features/files/drivers/registry";
import type { FileDetail, FileItem } from "~/features/files/model/types";
import type { ApiClient, FileSpecInput } from "~/shared/api/client";
import { defaultTranslator, type Translator } from "~/shared/i18n/context";
import { renderCacheTTLFromForm } from "~/shared/resources/render-cache-policy";
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
  async function createFile(kind: string, form: FormData) {
    const name = String(form.get("name") ?? "").trim();
    if (!name) throw new Error("file name is required");
    assertRegisteredFileKind(kind);
    const displayName = String(form.get("display_name") ?? "").trim();
    const description = String(form.get("description") ?? "").trim();
    const timestamps = fileTimestamps(undefined);
    const config = parseOptionalObjectField(form, "config");
    const renderCacheTTLSeconds = renderCacheTTLFromForm(form);
    assertSingleSubscription(config);
    await client.createFile({
      name,
      display_name: displayName || undefined,
      ...timestamps,
      kind,
      source: parseObjectField(form, "source", { type: "inline", content: "" }),
      config,
      processors: parseArrayField(form, "processors"),
      ...(renderCacheTTLSeconds === undefined ? {} : { render_cache_ttl_seconds: renderCacheTTLSeconds }),
      meta: {
        ...(description ? { description } : {}),
        ui: "web",
      },
    });
    await refreshResources();
    closeSheet();
    showNotice(t("messages.fileSaved"));
    navigate(fileEditPath(name));
  }

  async function saveFileEdit(item: FileItem, form: FormData, detail?: FileDetail | null) {
    const existingKind = detail ? detail.kind : item.kind;
    assertRegisteredFileKind(existingKind);
    if ((detail?.config?.subscriptions?.length ?? 0) > 1) throw new Error("multiple subscriptions must be combined in a collection subscription before saving");
    const name = item.name;
    const displayName = String(form.get("display_name") ?? "").trim();
    const description = String(form.get("description") ?? "").trim();
    const timestamps = fileTimestamps(detail);
    const config = parseOptionalObjectField(form, "config");
    const renderCacheTTLSeconds = renderCacheTTLFromForm(form);
    assertSingleSubscription(config);
    await client.createFile({
      name,
      display_name: displayName || undefined,
      ...timestamps,
      kind: existingKind,
      source: parseObjectField(form, "source", { type: "inline", content: "" }),
      config,
      processors: parseArrayField(form, "processors"),
      ...(renderCacheTTLSeconds === undefined ? {} : { render_cache_ttl_seconds: renderCacheTTLSeconds }),
      meta: {
        ...(description ? { description } : {}),
        ui: "web",
      },
    });
    await refreshResources();
    showNotice(t("messages.fileSaved"));
  }

  return {
    createFile,
    saveFileEdit,
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

function fileTimestamps(detail: FileDetail | null | undefined): Pick<FileSpecInput, "created_at" | "updated_at"> {
  const now = new Date().toISOString();
  if (!detail) {
    return { created_at: now, updated_at: now };
  }
  return {
    created_at: detail.createdAt || detail.updatedAt || now,
    updated_at: now,
  };
}

function parseOptionalObjectField(form: FormData, name: string): Record<string, unknown> | undefined {
  const raw = String(form.get(name) ?? "").trim();
  if (!raw) return undefined;
  try {
    const parsed = JSON.parse(raw) as unknown;
    return typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)
      ? parsed as Record<string, unknown>
      : undefined;
  } catch {
    return undefined;
  }
}

function parseObjectField(form: FormData, name: string, fallback: Record<string, unknown>): Record<string, unknown> {
  const raw = String(form.get(name) ?? "").trim();
  if (!raw) return fallback;
  try {
    const parsed = JSON.parse(raw) as unknown;
    return typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)
      ? parsed as Record<string, unknown>
      : fallback;
  } catch {
    return fallback;
  }
}

function parseArrayField(form: FormData, name: string): Array<Record<string, unknown>> {
  const raw = String(form.get(name) ?? "").trim();
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed)
      ? parsed.filter((entry): entry is Record<string, unknown> => typeof entry === "object" && entry !== null && !Array.isArray(entry))
      : [];
  } catch {
    return [];
  }
}
