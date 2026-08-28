import type {
  PreviewWarning,
  ProcessorDetail,
  RemoteInputDetail,
} from "~/shared/resources/types";

export interface FileItem {
  name: string;
  /** Canonical file kind returned in the resource summary target field. */
  kind: FileKind;
  displayName?: string;
  title: string;
  description?: string;
  createdAt?: string;
  updatedAt?: string;
  sourceType?: string;
  sourceSummary?: string;
  processorCount?: number;
}

export type FileKind = string;

export interface FileSourceDetail {
  type?: string;
  content?: string;
  remote?: RemoteInputDetail;
}

export interface FileAdaptiveGroupConfigDetail {
  type?: string;
  regions?: string[];
}

export interface FileConfigDetail {
  subscriptions?: string[];
  settingsPresent: boolean;
  settings?: unknown;
}

/** Internal editor draft produced and consumed by a structured file-driver adapter. */
export interface FileConfigDraft {
  subscriptions?: string[];
  settingsMode?: "structured" | "raw";
  rawSettings?: unknown;
  ruleset_preset?: string;
  group_preset?: string;
  adaptive_groups?: FileAdaptiveGroupConfigDetail;
  groups?: Record<string, unknown>[];
  rule_sets?: Record<string, unknown>[];
  rules?: unknown[];
}

export interface FileDetail {
  name: string;
  displayName?: string;
  kind: FileKind;
  source: FileSourceDetail;
  config?: FileConfigDetail;
  processors: ProcessorDetail[];
  createdAt?: string;
  updatedAt?: string;
  meta?: Record<string, string>;
  rawSpec: Record<string, unknown>;
}

export type FilePreviewWarning = PreviewWarning;

export interface FilePreview {
  contentType: string;
  body: string;
  response?: Record<string, unknown>;
  warnings: FilePreviewWarning[];
}

export type RuleSetCatalogTarget = "mihomo" | "sing-box" | "shadowrocket";

export interface RuleSetCatalogItem {
  name: string;
  url: string;
  ruleKind: "domain" | "ip" | "mixed";
  referenceType?: "RULE-SET" | "DOMAIN-SET";
}

export interface RuleSetCatalogResult {
  items: RuleSetCatalogItem[];
}
