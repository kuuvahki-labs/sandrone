import type {
  PreviewWarning,
  ProcessorDetail,
  RemoteInputDetail,
  ResourceStatus,
} from "~/shared/resources/types";

export type SubscriptionKind = "remote" | "local" | "collection";

export interface SubscriptionItem {
  kind: SubscriptionKind;
  name: string;
  displayName?: string;
  title: string;
  label: string;
  status: ResourceStatus;
  format?: string;
  description?: string;
  createdAt?: string;
  updatedAt?: string;
  warning?: string;
}

export interface SubscriptionDefinition {
  name: string;
  displayName?: string;
  kind: SubscriptionKind;
  format?: string;
  content?: string;
  createdAt?: string;
  updatedAt?: string;
  remote?: RemoteInputDetail;
  sourceRefs: string[];
  processors?: ProcessorDetail[];
  meta?: Record<string, string>;
}

export type SubscriptionPreviewStatus = "added" | "modified" | "removed" | "unchanged";

export interface SubscriptionPreviewNode {
  name: string;
  type?: string;
  server?: string;
  port?: number;
  endpoint: string;
  raw?: Record<string, unknown>;
}

export type SubscriptionPreviewWarning = PreviewWarning;

export interface SubscriptionPreviewNodeDiff {
  identity: string;
  status: SubscriptionPreviewStatus;
  before?: SubscriptionPreviewNode;
  after?: SubscriptionPreviewNode;
  targetNames?: Record<string, string>;
}

export interface SubscriptionPreview {
  subscriptionName: string;
  format?: string;
  beforeCount: number;
  afterCount: number;
  statusCounts: Record<SubscriptionPreviewStatus, number>;
  nodes: SubscriptionPreviewNodeDiff[];
  warnings: SubscriptionPreviewWarning[];
}

export interface SubscriptionTrafficItem {
  sourceName?: string;
  sourceUrl?: string;
  observedAt?: string;
  uploadBytes: number;
  downloadBytes: number;
  usedBytes: number;
  totalBytes?: number;
  remainingBytes?: number;
  expiresAt?: string;
  remainingDays?: number;
  resetDay?: number;
  appUrl?: string;
  planName?: string;
}

export interface SubscriptionTraffic {
  subscriptionName: string;
  kind: SubscriptionKind;
  format?: string;
  cached: boolean;
  traffic?: SubscriptionTrafficItem;
}
