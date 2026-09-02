import type { NavigateFunction } from "react-router";

import type { SubscriptionDefinition, SubscriptionItem } from "~/features/subscriptions/model/types";
import type { ApiClient, SubscriptionInput } from "~/shared/api/client";
import { secondsInputToMilliseconds } from "~/shared/api/duration";
import { defaultTranslator, type Translator } from "~/shared/i18n/context";
import { snapshotTTLFromForm } from "~/shared/resources/snapshot-cache-policy";
import { sourceNameFromUrl, subscriptionEditPath } from "~/shared/routing/paths";

type SubscriptionNotice = (
  message: string,
  severity?: "success" | "error" | "warning",
) => void;

export function createSubscriptionActions({
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
  showNotice: SubscriptionNotice;
  t?: Translator;
}) {
  async function createSubscription(form: FormData, existing?: SubscriptionItem) {
    assertCreatableSubscription(form, t);
    const subscription = subscriptionInputFromForm(form, existing, undefined, showNotice, t);
    if (!subscription) {
      return;
    }
    await client.createSubscription(subscription);
    await refreshResources();
    closeSheet();
    showNotice(t(existing ? "messages.subscriptionOverwritten" : "messages.subscriptionAdded"));
    navigate(subscriptionEditPath(subscription.type, subscription.name));
  }

  async function saveSubscriptionEdit(item: SubscriptionItem, form: FormData, definition?: SubscriptionDefinition | null) {
    const subscription = subscriptionInputFromForm(form, item, definition, showNotice, t);
    if (!subscription) {
      return false;
    }
    await client.createSubscription(subscription);
    await refreshResources();
    showNotice(t("messages.subscriptionSaved"));
    navigate(subscriptionEditPath(subscription.type, subscription.name), { replace: true });
    return true;
  }

  async function copySubscriptionSource(value: string, target: "content" | "url") {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(value);
    }
    showNotice(t(target === "content" ? "messages.subscriptionContentCopied" : "messages.subscriptionUrlCopied"));
  }

  return {
    copySubscriptionSource,
    createSubscription,
    saveSubscriptionEdit,
  };
}

function assertCreatableSubscription(form: FormData, t: Translator) {
  const type = subscriptionTypeFromForm(form, undefined);
  if (type === "remote" && !sourceRemoteFromInput(form, sourceInputFromForm(form))) {
    throw new Error(t("errors.validation.remoteSubscriptionUrlRequired"));
  }
  if (type === "local" && !sourceInputFromForm(form)) {
    throw new Error(t("errors.validation.localSubscriptionRequired"));
  }
  if (type === "collection" && subscriptionRefsFromForm(form).length === 0) {
    throw new Error(t("errors.validation.collectionRequired"));
  }
}

function subscriptionInputFromForm(
  form: FormData,
  item: SubscriptionItem | undefined,
  definition: SubscriptionDefinition | null | undefined,
  showNotice: SubscriptionNotice,
  t: Translator,
): SubscriptionInput | null {
  const type = subscriptionTypeFromForm(form, item);
  const name = item?.name || subscriptionCreateName(form);
  const displayName = String(form.get("display_name") ?? "").trim();
  const format = sourceFormatForPayload(String(form.get("format") ?? ""));
  const timestamps = subscriptionTimestamps(item, definition);
  const processors = parseProcessors(String(form.get("processors") ?? ""), showNotice, t);
  if (processors === null) {
    return null;
  }
  const meta = parseStringMeta(String(form.get("meta") ?? ""), showNotice, t);
  if (meta === null) {
    return null;
  }
  const description = String(form.get("description") ?? "").trim();
  const snapshotTTLSeconds = snapshotTTLFromForm(form);
  const snapshotPolicy = snapshotTTLSeconds === undefined
  ? {}
  : { snapshot_ttl_seconds: snapshotTTLSeconds };
  if (description) {
    meta.description = description;
  }
  meta.ui = meta.ui || "web";

  if (type === "remote") {
    const sourceInput = sourceInputFromForm(form);
    const remote = sourceRemoteFromInput(form, sourceInput);
    if (!name || !remote) {
      showNotice(t("errors.validation.remoteSubscriptionUrlRequired"), "error");
      return null;
    }
    return {
      name,
      display_name: displayName || undefined,
      type,
      format,
      ...timestamps,
    ...snapshotPolicy,
      remote,
      processors: processors.length ? processors : undefined,
      meta,
    };
  }

  if (type === "local") {
    const content = sourceInputFromForm(form);
    if (!name || !content) {
      showNotice(t("errors.validation.localSubscriptionRequired"), "error");
      return null;
    }
    return {
      name,
      display_name: displayName || undefined,
      type,
      format,
      content,
      ...timestamps,
    ...snapshotPolicy,
      processors: processors.length ? processors : undefined,
      meta,
    };
  }

  const refs = subscriptionRefsFromForm(form);
  if (!name || refs.length === 0) {
    showNotice(t("errors.validation.collectionRequired"), "error");
    return null;
  }
  return {
    name,
    display_name: displayName || undefined,
    type,
    ...timestamps,
  ...snapshotPolicy,
    inputs: refs.map((ref) => ({ name: ref, type: "subscription", ref: { kind: "subscription", name: ref } })),
    processors: processors.length ? processors : undefined,
    meta,
  };
}

function subscriptionTimestamps(
  item: SubscriptionItem | undefined,
  definition: SubscriptionDefinition | null | undefined,
): Pick<SubscriptionInput, "created_at" | "updated_at"> {
  const now = new Date().toISOString();
  if (!item) {
    return { created_at: now, updated_at: now };
  }
  return {
    created_at: definition?.createdAt || item.createdAt || definition?.updatedAt || item.updatedAt || now,
    updated_at: now,
  };
}

export function subscriptionCreateName(form: FormData): string {
  const type = subscriptionTypeFromForm(form);
  const submittedName = String(form.get("name") ?? "").trim();
  return submittedName || defaultSubscriptionName(type, sourceInputFromForm(form));
}

function subscriptionTypeFromForm(form: FormData, item?: SubscriptionItem): SubscriptionInput["type"] {
  const value = String(form.get("subscription_type") ?? "").trim();
  if (value === "remote" || value === "local" || value === "collection") {
    return value;
  }
  return item?.kind ?? "remote";
}

function defaultSubscriptionName(type: SubscriptionInput["type"], sourceInput: string): string {
  if (type === "remote") {
    return sourceNameFromUrl(sourceInput);
  }
  if (type === "local") {
    return "manual";
  }
  return "default";
}

function sourceInputFromForm(form: FormData): string {
  return String(form.get("source_input") ?? form.get("url") ?? "").trim();
}

function sourceFormatForPayload(value: string): string | undefined {
  const format = value.trim();
  return format && format.toLowerCase() !== "auto" ? format : undefined;
}

function sourceRemoteFromInput(form: FormData, sourceInput: string) {
  if (!isRemoteSubscriptionInput(sourceInput)) {
    return undefined;
  }
  const timeout = secondsInputToMilliseconds(String(form.get("timeout_ms") ?? ""));
  const cacheTTLSeconds = optionalNumber(String(form.get("cache_ttl_seconds") ?? "").trim());
  return {
    url: sourceInput,
    user_agent: optionalString(form, "user_agent"),
    proxy: optionalString(form, "proxy"),
    ...(timeout !== undefined && timeout > 0 ? { timeout_ms: timeout } : {}),
    ...(cacheTTLSeconds !== undefined && cacheTTLSeconds > 0 ? { cache_ttl_seconds: cacheTTLSeconds } : {}),
  };
}

function isRemoteSubscriptionInput(value: string): boolean {
  const lines = value.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  if (lines.length !== 1) {
    return false;
  }
  try {
    const parsed = new URL(lines[0]);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

function optionalString(form: FormData, name: string): string | undefined {
  const value = String(form.get(name) ?? "").trim();
  return value || undefined;
}

function optionalNumber(value: string): number | undefined {
  if (!value) {
    return undefined;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function subscriptionRefsFromForm(form: FormData): string[] {
  const refs = form.getAll("subscriptions")
    .flatMap((value) => String(value).split(","))
    .map((value) => value.trim())
    .filter(Boolean);
  return Array.from(new Set(refs));
}

function parseProcessors(raw: string, showNotice: SubscriptionNotice, t: Translator): Array<Record<string, unknown>> | null {
  const value = raw.trim();
  if (!value) {
    return [];
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch {
    showNotice(t("errors.invalidProcessorsJson"), "error");
    return null;
  }
  if (!Array.isArray(parsed) || !parsed.every((entry) => typeof entry === "object" && entry !== null && typeof (entry as { type?: unknown }).type === "string")) {
    showNotice(t("errors.invalidProcessorsShape"), "error");
    return null;
  }
  return parsed as Array<Record<string, unknown>>;
}

function parseStringMeta(raw: string, showNotice: SubscriptionNotice, t: Translator): Record<string, string> | null {
  const value = raw.trim();
  if (!value) {
    return {};
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch {
    showNotice(t("errors.invalidMetadataJson"), "error");
    return null;
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed) || !Object.values(parsed).every((entry) => typeof entry === "string")) {
    showNotice(t("errors.invalidMetadataShape"), "error");
    return null;
  }
  return parsed as Record<string, string>;
}
