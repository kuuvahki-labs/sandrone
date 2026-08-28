import { arrayField, asRecord, numberField, optionalNumberField, stringField } from "~/shared/resources/model-fields";
import type { ProcessorDetail, RemoteInputDetail } from "~/shared/resources/types";

import { fileSourceSummary } from "./summary";
import type {
  FileConfigDetail,
  FileDetail,
  FileItem,
  FilePreview,
  FilePreviewWarning,
  FileSourceDetail,
  RuleSetCatalogResult,
} from "./types";

export function ruleSetCatalogFromAPI(value: unknown): RuleSetCatalogResult {
  return {
    items: arrayField(asRecord(value).items).flatMap((value) => {
      const item = asRecord(value);
      const ruleKind = stringField(item.rule_kind);
      const referenceType = stringField(item.reference_type);
      const name = stringField(item.name);
      const url = stringField(item.url);
      return name && url && (ruleKind === "domain" || ruleKind === "ip" || ruleKind === "mixed")
        ? [{
          name,
          url,
          ruleKind,
          ...(referenceType === "RULE-SET" || referenceType === "DOMAIN-SET" ? { referenceType } : {}),
        }]
        : [];
    }),
  };
}

export function filesFromResourceList(list: unknown): FileItem[] {
  return arrayField(asRecord(list).items).map((value) => {
    const item = asRecord(value);
    const meta = asRecord(item.meta);
    const name = stringField(item.name);
    const displayName = stringField(item.display_name) || undefined;
    const processors = arrayField(item.processors).map(processorFromAPI).filter((processor) => processor.type);
    const kind = stringField(item.target);
    const sourceType = stringField(item.type) || stringField(asRecord(item.source).type) || "inline";
    return {
      name,
      kind,
      displayName,
      title: displayName || name,
      description: stringField(meta.description),
      createdAt: stringField(item.created_at) || undefined,
      updatedAt: stringField(item.updated_at) || undefined,
      sourceType,
      sourceSummary: fileSourceSummary({ type: sourceType }),
      processorCount: numberField(item.processor_count) || processors.length,
    };
  }).filter((item) => item.name);
}

export function fileDetailFromAPI(value: unknown): FileDetail {
  const item = asRecord(value);
  const meta = stringMapField(item.meta);
  const processors = arrayField(item.processors).map(processorFromAPI).filter((processor) => processor.type);
  const kind = stringField(item.kind);
  return {
    name: stringField(item.name),
    displayName: stringField(item.display_name) || undefined,
    kind,
    createdAt: stringField(item.created_at) || undefined,
    updatedAt: stringField(item.updated_at) || undefined,
    source: fileSourceFromAPI(item.source),
    config: fileConfigFromAPI(item.config),
  processors,
    meta: Object.keys(meta).length ? meta : undefined,
    rawSpec: { ...item },
  };
}

export function filePreviewFromAPI(value: unknown): FilePreview {
  const item = asRecord(value);
  const response = asRecord(item.response);
  return {
    contentType: stringField(item.content_type) || stringField(response.content_type) || "application/octet-stream",
    body: stringField(item.body),
    response: Object.keys(response).length ? response : undefined,
    warnings: previewWarningsFromAPI(item),
  };
}

function fileConfigFromAPI(value: unknown): FileConfigDetail | undefined {
  const item = asRecord(value);
  if (Object.keys(item).length === 0 && (typeof value !== "object" || value === null || Array.isArray(value))) {
    return undefined;
  }
  const subscriptions = arrayField(item.subscriptions).map(stringField).filter(Boolean);
  const hasSettings = Object.prototype.hasOwnProperty.call(item, "settings");
  return {
    subscriptions: subscriptions.length ? subscriptions : undefined,
    settingsPresent: hasSettings,
    ...(hasSettings ? { settings: item.settings } : {}),
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

function fileSourceFromAPI(value: unknown): FileSourceDetail {
  const item = asRecord(value);
  const remote = remoteInputFromAPI(item.remote);
  const type = stringField(item.type);
  const hasContent = Object.prototype.hasOwnProperty.call(item, "content");
  if (type || hasContent || remote) {
    return {
      type: type || (remote ? "remote" : "inline"),
      ...(hasContent ? { content: stringField(item.content) } : {}),
      remote,
    };
  }
  return {};
}

function previewWarningsFromAPI(item: Record<string, unknown>): FilePreviewWarning[] {
  return arrayField(item.warnings).map(previewWarningFromAPI).filter((warning) => warning.code || warning.message);
}

function previewWarningFromAPI(value: unknown): FilePreviewWarning {
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
