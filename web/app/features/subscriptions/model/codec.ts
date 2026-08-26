import { arrayField, asRecord, numberField, optionalNumberField, stringField } from "~/shared/resources/model-fields";
import type { ProcessorDetail, RemoteInputDetail } from "~/shared/resources/types";

import type {
  SubscriptionDefinition,
  SubscriptionItem,
  SubscriptionKind,
  SubscriptionPreview,
  SubscriptionPreviewNode,
  SubscriptionPreviewNodeDiff,
  SubscriptionPreviewProbe,
  SubscriptionPreviewStatus,
  SubscriptionPreviewWarning,
  SubscriptionTraffic,
  SubscriptionTrafficItem,
} from "./types";

const positiveIntegerPattern = /^[1-9]\d*$/;

export function subscriptionsFromResourceList(list: unknown): SubscriptionItem[] {
  return arrayField(asRecord(list).items).map(subscriptionFromAPI).filter(hasName);
}

export function subscriptionDefinitionFromAPI(value: unknown): SubscriptionDefinition {
  const item = asRecord(value);
  const remote = remoteInputFromAPI(item.remote);
  const processors = arrayField(item.processors).map(processorFromAPI).filter((processor) => processor.type);
  const meta = stringMapField(item.meta);
  return {
    name: stringField(item.name),
    displayName: stringField(item.display_name) || undefined,
    kind: subscriptionKindFromAPI(item.type),
    format: stringField(item.format) || undefined,
    content: stringField(item.content) || undefined,
    createdAt: stringField(item.created_at) || undefined,
    updatedAt: stringField(item.updated_at) || undefined,
    remote,
    sourceRefs: uniqueStrings(arrayField(item.inputs).map(collectionInputSourceName).filter(Boolean)),
    processors: processors.length ? processors : undefined,
    renderCacheTTLSeconds: optionalNumberField(item.render_cache_ttl_seconds),
    meta: Object.keys(meta).length ? meta : undefined,
  };
}

export function subscriptionPreviewFromAPI(value: unknown): SubscriptionPreview {
  const item = asRecord(value);
  return {
    subscriptionName: stringField(item.subscription_name),
    format: stringField(item.format) || undefined,
    beforeCount: numberField(item.before_count),
    afterCount: numberField(item.after_count),
    statusCounts: previewStatusCounts(item.status_counts),
    nodes: arrayField(item.nodes).map(previewDiffFromAPI).filter((node) => node.runtimeId),
    warnings: previewWarningsFromAPI(item),
  };
}

export function subscriptionTrafficFromAPI(value: unknown): SubscriptionTraffic {
  const item = asRecord(value);
  return {
    subscriptionName: stringField(item.subscription_name),
    kind: subscriptionKindFromAPI(item.type),
    format: stringField(item.format) || undefined,
    cached: Boolean(item.cached),
    traffic: subscriptionTrafficItemOptionalFromAPI(item.traffic),
  };
}

function remoteInputFromAPI(value: unknown): RemoteInputDetail | undefined {
  const item = asRecord(value);
  const remote: RemoteInputDetail = {
    url: stringField(item.url) || undefined,
    user_agent: stringField(item.user_agent) || undefined,
    proxy: stringField(item.proxy) || undefined,
    timeout_ms: optionalNumberField(item.timeout_ms),
    cache_ttl_seconds: optionalNumberField(item.cache_ttl_seconds),
  };
  return hasDefinedValue(remote) ? remote : undefined;
}

function processorFromAPI(value: unknown): ProcessorDetail {
  const item = asRecord(value);
  const params = asRecord(item.params);
  return {
    ...item,
    name: stringField(item.name) || undefined,
    type: stringField(item.type),
    stage: stringField(item.stage) || undefined,
    params: Object.keys(params).length ? params : undefined,
  };
}

function collectionInputSourceName(value: unknown): string {
  const item = asRecord(value);
  const ref = asRecord(item.ref);
  const refKind = stringField(ref.kind);
  const refName = stringField(ref.name);
  if (refKind === "subscription" && refName) {
    return refName;
  }
  if (refKind) {
    return "";
  }
  const type = stringField(item.type) || stringField(item.kind);
  if (type === "subscription") {
    return refName || stringField(item.name);
  }
  return "";
}

function uniqueStrings(values: string[]): string[] {
  return Array.from(new Set(values));
}

function previewStatusCounts(value: unknown): Record<SubscriptionPreviewStatus, number> {
  const item = asRecord(value);
  return {
    added: numberField(item.added),
    modified: numberField(item.modified),
    removed: numberField(item.removed),
    unchanged: numberField(item.unchanged),
  };
}

function previewDiffFromAPI(value: unknown): SubscriptionPreviewNodeDiff {
  const item = asRecord(value);
  const targetNames = stringMapField(item.target_names);
  return {
    runtimeId: stringField(item.runtime_id),
    status: previewStatus(item.status),
    before: previewNodeFromAPI(item.before),
    after: previewNodeFromAPI(item.after),
    ...(Object.keys(targetNames).length ? { targetNames } : {}),
  };
}

function previewStatus(value: unknown): SubscriptionPreviewStatus {
  const status = stringField(value);
  if (status === "added" || status === "modified" || status === "removed" || status === "unchanged") {
    return status;
  }
  return "unchanged";
}

function previewNodeFromAPI(value: unknown): SubscriptionPreviewNode | undefined {
  const item = asRecord(value);
  if (!Object.keys(item).length) {
    return undefined;
  }
  const server = stringField(item.server);
  const port = numberField(item.port);
  const probe = previewProbeFromAPI(item.meta);
  return {
    name: stringField(item.name),
    type: stringField(item.type) || undefined,
    server: server || undefined,
    port: port || undefined,
    endpoint: server ? (port ? `${server}:${port}` : server) : "-",
    ...(probe ? { probe } : {}),
    raw: item,
  };
}

function previewProbeFromAPI(value: unknown): SubscriptionPreviewProbe | undefined {
  const meta = asRecord(value);
  const aliveValue = stringField(meta["probe.alive"]);
  if (aliveValue !== "true" && aliveValue !== "false") {
    return undefined;
  }
  const durationText = stringField(meta["probe.duration_ms"]).trim();
  const durationValue = positiveIntegerPattern.test(durationText) ? Number(durationText) : Number.NaN;
  const durationMs = Number.isSafeInteger(durationValue) ? durationValue : undefined;
  const method = stringField(meta["probe.method"]).trim();
  const checkedAt = stringField(meta["probe.checked_at"]).trim();
  const errorCode = stringField(meta["probe.error_code"]).trim();
  return {
    alive: aliveValue === "true",
    ...(durationMs !== undefined ? { durationMs } : {}),
    ...(method ? { method } : {}),
    ...(checkedAt ? { checkedAt } : {}),
    ...(errorCode ? { errorCode } : {}),
  };
}

function subscriptionTrafficItemOptionalFromAPI(value: unknown): SubscriptionTrafficItem | undefined {
  const item = asRecord(value);
  if (Object.keys(item).length === 0) {
    return undefined;
  }
  const traffic = subscriptionTrafficItemFromAPI(item);
  return hasDefinedValue(traffic) ? traffic : undefined;
}

function subscriptionTrafficItemFromAPI(value: unknown): SubscriptionTrafficItem {
  const item = asRecord(value);
  const traffic: SubscriptionTrafficItem = {
    sourceName: stringField(item.source_name) || undefined,
    sourceUrl: stringField(item.source_url) || undefined,
    observedAt: stringField(item.observed_at) || undefined,
    uploadBytes: numberField(item.upload_bytes),
    downloadBytes: numberField(item.download_bytes),
    usedBytes: numberField(item.used_bytes),
    totalBytes: optionalNumberField(item.total_bytes),
    remainingBytes: optionalNumberField(item.remaining_bytes),
    expiresAt: stringField(item.expires_at) || undefined,
    remainingDays: optionalNumberField(item.remaining_days),
    resetDay: optionalNumberField(item.reset_day),
    appUrl: stringField(item.app_url) || undefined,
    planName: stringField(item.plan_name) || undefined,
  };
  return Object.fromEntries(
    Object.entries(traffic).filter(([, entry]) => entry !== undefined && entry !== ""),
  ) as unknown as SubscriptionTrafficItem;
}

function previewWarningsFromAPI(item: Record<string, unknown>): SubscriptionPreviewWarning[] {
  return arrayField(item.warnings).map(previewWarningFromAPI).filter((warning) => warning.code || warning.message);
}

function previewWarningFromAPI(value: unknown): SubscriptionPreviewWarning {
  const item = asRecord(value);
  return {
    ...item,
    code: stringField(item.code),
    message: stringField(item.message),
    node: stringField(item.node) || undefined,
    field: stringField(item.field) || undefined,
    source: stringField(item.source) || undefined,
    target: stringField(item.target) || undefined,
  };
}

function stringMapField(value: unknown): Record<string, string> {
  return Object.fromEntries(
    Object.entries(asRecord(value))
      .filter((entry): entry is [string, string] => typeof entry[1] === "string"),
  );
}

function hasDefinedValue(value: object): boolean {
  return Object.values(value).some((entry) => entry !== undefined && entry !== "");
}

function subscriptionFromAPI(value: unknown): SubscriptionItem {
  const item = asRecord(value);
  const meta = asRecord(item.meta);
  const warning = stringField(item.warning);
  const name = stringField(item.name);
  const displayName = stringField(item.display_name) || undefined;
  const kind = subscriptionKindFromAPI(item.type);
  return {
    kind,
    name,
    displayName,
    title: displayName || name,
    label: subscriptionKindLabel(kind),
    status: warning ? "warning" : name ? "ready" : "empty",
    format: kind === "collection" ? undefined : stringField(item.format) || "auto",
    description: stringField(meta.description),
    createdAt: stringField(item.created_at) || undefined,
    updatedAt: stringField(item.updated_at) || undefined,
    warning,
  };
}

function hasName(item: SubscriptionItem): boolean {
  return Boolean(item.name);
}

function subscriptionKindFromAPI(value: unknown): SubscriptionKind {
  const kind = stringField(value);
  if (kind === "remote" || kind === "local" || kind === "collection") {
    return kind;
  }
  return "local";
}

function subscriptionKindLabel(kind: SubscriptionKind): string {
  return kind;
}
