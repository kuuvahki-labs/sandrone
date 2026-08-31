export type ResourceStatus = "ready" | "warning" | "empty" | "unknown";

export interface ResourceOption {
  name: string;
  title: string;
}

export interface RemoteInputDefaults {
  cacheTTLSeconds: number;
  proxy?: string;
  timeoutMS?: number;
  userAgent?: string;
}

export interface RemoteInputDetail {
  url?: string;
  user_agent?: string;
  proxy?: string;
  timeout_ms?: number;
  cache_ttl_seconds?: number;
}

export interface ProcessorDetail {
  [key: string]: unknown;
  enabled?: boolean;
  name?: string;
  type: string;
  stage?: string;
  params?: Record<string, unknown>;
}

export interface WarningNodeContext {
  [key: string]: unknown;
  format?: string;
  line?: number;
  name?: string;
  port?: number;
  raw?: Record<string, unknown>;
  raw_line?: string;
  server?: string;
  type?: string;
}

export interface PreviewWarning {
  [key: string]: unknown;
  code: string;
  field?: string;
  line?: number;
  message: string;
  node?: string;
  node_context?: WarningNodeContext;
  node_index?: number;
  raw_line?: string;
  source?: string;
  target?: string;
}

export interface IgnoredWarning {
  code: string;
  field?: string;
  source?: string;
  target?: string;
}
