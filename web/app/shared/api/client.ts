import type { IgnoredWarning } from "~/shared/resources/types";
import { getAdminToken } from "~/shared/storage/preferences";

export type Fetcher = typeof fetch;

type RuleSetCatalogTransportTarget = "mihomo" | "sing-box" | "shadowrocket";

const inFlightRequests = new Map<string, Promise<unknown>>();

export interface ApiClientOptions {
  baseUrl?: string;
  fetcher?: Fetcher;
  onUnauthorized?: () => void;
}

export interface ShareCreateRequest {
  name?: string;
  target_kind: "file" | "subscription";
  target_name: string;
  target_format?: string;
  content_type?: string;
  valid_from?: string;
  valid_until?: string;
  age_recipient?: string;
  meta?: Record<string, string>;
}

export interface SubscriptionInput {
  name: string;
  display_name?: string;
  type: "remote" | "local" | "collection";
  format?: string;
  content?: string;
  created_at?: string;
  updated_at?: string;
  remote?: {
    url: string;
    user_agent?: string;
    proxy?: string;
    timeout_ms?: number;
    cache_ttl_seconds?: number;
  };
  inputs?: Array<{
    name: string;
    type: "subscription";
    ref: { kind: "subscription"; name: string };
  }>;
  processors?: Array<Record<string, unknown>>;
  snapshot_ttl_seconds?: number;
  meta?: Record<string, string>;
}

export interface SubscriptionTrafficRequest {
  refresh?: boolean;
}

export interface SubscriptionPreviewRequest {
  refresh?: boolean;
}

export interface ConvertRequest {
  from_format: string;
  to_format: string;
  content: string;
}

export interface NodeInspectRequest {
  node: Record<string, unknown>;
  include: Array<"uri" | "ip">;
}

export interface FileSpecInput {
  name: string;
  display_name?: string;
  kind: string;
  source: Record<string, unknown>;
  config?: Record<string, unknown>;
  processors?: Array<Record<string, unknown>>;
  created_at?: string;
  updated_at?: string;
  meta?: Record<string, string>;
}

export interface RemoteDefaultsInput {
  user_agent?: string;
  proxy?: string;
  timeout_ms: number;
}

export interface ProbeDefaultsInput {
  method: "tcp_connect" | "udp_ntp" | "url_test";
  core: "mihomo" | "sing-box";
  url: string;
  ntp_server: string;
  expected_status: string;
  timeout_ms: number;
  attempts: number;
  concurrency: number;
}

export interface ScriptDefaultsInput {
  timeout_ms: number;
}

export interface CacheDefaultsInput {
  remote_fetch_ttl_seconds: number;
  probe_ttl_seconds: number;
  subscription_snapshot_ttl_seconds: number;
}

export type ScheduledRefreshTargetKind = "subscription" | "file";

export interface ScheduledRefreshTarget {
  kind: ScheduledRefreshTargetKind;
  name: string;
}

export interface SettingsView {
  schema_version: number;
  http: {
    listen: string;
  };
  mcp: {
    path: string;
    max_output_bytes: number;
  };
  log: {
    level: "debug" | "info" | "warn" | "error";
  };
  remote_defaults: RemoteDefaultsInput;
  probe_defaults: ProbeDefaultsInput;
  script_defaults: ScriptDefaultsInput;
  cache_defaults: CacheDefaultsInput;
  appearance: {
    theme_mode: "system" | "light" | "dark";
    locale: "auto" | "zh-CN" | "en-US";
  };
  subscriptions: {
    auto_load_traffic: boolean;
    ignored_warnings: IgnoredWarning[];
  };
  scheduled_refresh: {
    enabled: boolean;
    schedule: string;
    targets: ScheduledRefreshTarget[];
  };
}

export interface SettingsUpdate {
  schema_version: number;
  http: {
    listen: string;
  };
  mcp: SettingsView["mcp"];
  log: SettingsView["log"];
  remote_defaults: RemoteDefaultsInput;
  probe_defaults: ProbeDefaultsInput;
  script_defaults: ScriptDefaultsInput;
  cache_defaults: CacheDefaultsInput;
  appearance: SettingsView["appearance"];
  subscriptions: SettingsView["subscriptions"];
  scheduled_refresh: SettingsView["scheduled_refresh"];
}

export interface SettingsEnvelope {
  settings: SettingsView;
  effective: SettingsView;
  overrides: Record<string, string>;
  restart_required: string[];
}

export interface UICapability {
  key: string;
  enabled: boolean;
  reason?: string;
  dependencies?: string[];
}

export interface UICapabilityList {
  features: UICapability[];
}

export type FormatCapabilityDirection = "parse" | "render";

export interface FormatCapabilitySummary {
  direction: FormatCapabilityDirection;
  field_counts: {
    supported: number;
    lossy: number;
    raw_only: number;
  };
  format: string;
  href: string;
  node_types: string[];
  reversible: boolean;
  revisions: string[];
}

export interface FormatCapabilityList {
  items: FormatCapabilitySummary[];
}

export interface ScheduledRefreshStatus {
  enabled: boolean;
  running: boolean;
  next_run_at?: string;
  last_started_at?: string;
  last_completed_at?: string;
  last_success_count: number;
  last_failure_count: number;
  skipped_count: number;
  last_skipped_at?: string;
}

export interface VersionInfo {
  build_time: string;
  name: string;
  version: string;
  revision: string;
}

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export class ApiClient {
  private readonly baseUrl: string;
  private readonly fetcher: Fetcher;
  private readonly onUnauthorized?: () => void;

  constructor(options: ApiClientOptions = {}) {
    this.baseUrl = options.baseUrl ?? "";
    this.fetcher = options.fetcher ?? globalThis.fetch.bind(globalThis);
    this.onUnauthorized = options.onUnauthorized;
  }

  getHealth(): Promise<unknown> {
    return this.request("/healthz", { auth: false });
  }

  getVersion(): Promise<VersionInfo> {
    return this.request("/version", { auth: false });
  }

  inspect(): Promise<unknown> {
    return this.dedupedRequest("GET", "/v1/inspect");
  }

  convert(request: ConvertRequest): Promise<unknown> {
    return this.request("/v1/convert", { method: "POST", body: request });
  }

  inspectNode(request: NodeInspectRequest): Promise<unknown> {
    return this.request("/v1/nodes/inspect", { method: "POST", body: request });
  }

  listFormatCapabilities(): Promise<FormatCapabilityList> {
    return this.dedupedRequest("GET", "/v1/capabilities/formats");
  }

  getUICapabilities(): Promise<UICapabilityList> {
    return this.dedupedRequest("GET", "/v1/capabilities/ui");
  }

  listSubscriptions(): Promise<unknown> {
    return this.dedupedRequest("GET", "/v1/subscriptions");
  }

  listFiles(): Promise<unknown> {
    return this.dedupedRequest("GET", "/v1/files");
  }

  listShares(): Promise<unknown> {
    return this.dedupedRequest("GET", "/v1/shares");
  }

  createSubscription(subscription: SubscriptionInput): Promise<unknown> {
    return this.request("/v1/subscriptions", { method: "POST", body: subscription });
  }

  getSubscription(name: string): Promise<unknown> {
    return this.dedupedRequest("GET", `/v1/subscriptions/${encodeURIComponent(name)}`);
  }

  previewSubscription(name: string, options: SubscriptionPreviewRequest = {}): Promise<unknown> {
    const path = `/v1/subscriptions/${encodeURIComponent(name)}/preview`;
    const request = options.refresh
      ? { method: "POST", body: { refresh: true } }
      : { method: "POST" };
    return this.dedupedRequest("POST", path, request);
  }

  subscriptionTraffic(name: string, body: SubscriptionTrafficRequest = {}): Promise<unknown> {
    const path = `/v1/subscriptions/${encodeURIComponent(name)}/traffic`;
    return this.dedupedRequest("POST", path, { method: "POST", body });
  }

  createFile(file: FileSpecInput): Promise<unknown> {
    return this.request("/v1/files", { method: "POST", body: file });
  }

  getSettings(options: { fresh?: boolean } = {}): Promise<SettingsEnvelope> {
    if (options.fresh) {
      return this.replaceDedupedRequest("GET", "/v1/settings");
    }
    return this.dedupedRequest("GET", "/v1/settings");
  }

  updateSettings(settings: SettingsUpdate): Promise<SettingsEnvelope> {
    return this.request("/v1/settings", { method: "PUT", body: settings });
  }

  getScheduledRefreshStatus(options: { fresh?: boolean } = {}): Promise<ScheduledRefreshStatus> {
    if (options.fresh) {
      return this.replaceDedupedRequest("GET", "/v1/settings/scheduled-refresh-status");
    }
    return this.dedupedRequest("GET", "/v1/settings/scheduled-refresh-status");
  }

  runScheduledRefresh(): Promise<{ accepted: boolean }> {
    return this.request("/v1/settings/scheduled-refresh/run", { method: "POST" });
  }

  async downloadBackup(): Promise<{ blob: Blob; filename: string }> {
    const response = await this.rawRequest("/v1/backup");
    return {
      blob: await response.blob(),
      filename: backupFilename(response.headers.get("content-disposition")),
    };
  }

  async restoreBackup(file: Blob): Promise<void> {
    await this.rawRequest("/v1/backup/restore", { method: "POST", body: file });
  }

  async clearCache(): Promise<void> {
    await this.rawRequest("/v1/cache", { method: "DELETE" });
  }

  getFileSpec(name: string): Promise<unknown> {
    return this.request(`/v1/files/${encodeURIComponent(name)}?mode=spec`);
  }

  getFileSource(name: string): Promise<unknown> {
    return this.request(`/v1/files/${encodeURIComponent(name)}?mode=source&response=json`);
  }

  previewFile(name: string, options: { refresh?: boolean } = {}): Promise<unknown> {
    const refresh = options.refresh ? "&refresh=true" : "";
    return this.dedupedRequest("GET", `/v1/files/${encodeURIComponent(name)}?response=json${refresh}`);
  }

  listRuleSetCatalog(target: RuleSetCatalogTransportTarget): Promise<unknown> {
    return this.dedupedRequest("GET", `/v1/rule-set-catalog?target=${encodeURIComponent(target)}`);
  }

  createShare(share: ShareCreateRequest): Promise<unknown> {
    return this.request("/v1/shares", { method: "POST", body: share });
  }

  deleteShare(id: string): Promise<unknown> {
    return this.request(`/v1/shares/${encodeURIComponent(id)}`, { method: "DELETE" });
  }

  deleteResource(kind: "subscriptions" | "files" | "shares", name: string): Promise<unknown> {
    return this.request(`/v1/${kind}/${encodeURIComponent(name)}`, { method: "DELETE" });
  }

  private async request<T = unknown>(
    path: string,
    options: { method?: string; body?: unknown; auth?: boolean } = {},
  ): Promise<T> {
    const headers: Record<string, string> = {};
    const token = getAdminToken();
    if (options.auth !== false && token) {
      headers.Authorization = `Bearer ${token}`;
    }
    if (options.body !== undefined) {
      headers["Content-Type"] = "application/json";
    }

    const response = await this.fetcher(this.baseUrl + path, {
      method: options.method ?? "GET",
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    });
    const contentType = response.headers.get("content-type") ?? "";
    const data = contentType.includes("application/json") ? await response.json() : await response.text();
    if (!response.ok) {
      const error = asRecord(data).error;
      const errorRecord = asRecord(error);
      const code = typeof errorRecord.code === "string" ? errorRecord.code : "http_error";
      const message = typeof errorRecord.message === "string" ? errorRecord.message : `HTTP ${response.status}`;
      if (response.status === 401 && options.auth !== false) {
        this.onUnauthorized?.();
      }
      throw new ApiError(response.status, code, message);
    }
    return data as T;
  }

  private async rawRequest(
    path: string,
    options: { method?: string; body?: BodyInit; auth?: boolean } = {},
  ): Promise<Response> {
    const headers: Record<string, string> = {};
    const token = getAdminToken();
    if (options.auth !== false && token) {
      headers.Authorization = `Bearer ${token}`;
    }

    const response = await this.fetcher(this.baseUrl + path, {
      method: options.method ?? "GET",
      headers,
      body: options.body,
    });
    if (!response.ok) {
      const contentType = response.headers.get("content-type") ?? "";
      const data = contentType.includes("application/json") ? await response.json() : await response.text();
      const error = asRecord(data).error;
      const errorRecord = asRecord(error);
      const code = typeof errorRecord.code === "string" ? errorRecord.code : "http_error";
      const message = typeof errorRecord.message === "string" ? errorRecord.message : `HTTP ${response.status}`;
      if (response.status === 401 && options.auth !== false) {
        this.onUnauthorized?.();
      }
      throw new ApiError(response.status, code, message);
    }
    return response;
  }

  private dedupedRequest<T = unknown>(
    method: string,
    path: string,
    options: { method?: string; body?: unknown; auth?: boolean } = {},
  ): Promise<T> {
    const key = this.requestKey(method, path, options);
    const existing = inFlightRequests.get(key);
    if (existing) {
      return existing as Promise<T>;
    }
    const request = this.request<T>(path, options).finally(() => {
      if (inFlightRequests.get(key) === request) {
        inFlightRequests.delete(key);
      }
    });
    inFlightRequests.set(key, request);
    return request;
  }

  private replaceDedupedRequest<T = unknown>(
    method: string,
    path: string,
    options: { method?: string; body?: unknown; auth?: boolean } = {},
  ): Promise<T> {
    const key = this.requestKey(method, path, options);
    const request = this.request<T>(path, options).finally(() => {
      if (inFlightRequests.get(key) === request) {
        inFlightRequests.delete(key);
      }
    });
    inFlightRequests.set(key, request);
    return request;
  }

  private requestKey(method: string, path: string, options: { body?: unknown; auth?: boolean }): string {
    const token = options.auth === false ? "" : getAdminToken();
    const body = options.body === undefined ? "" : JSON.stringify(options.body);
    return [this.baseUrl, method, path, token, body].join("\n");
  }
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : {};
}

function backupFilename(contentDisposition: string | null): string {
  const fallback = "sandrone-backup.zip";
  if (!contentDisposition) {
    return fallback;
  }
  const match = /(?:^|;)\s*filename\s*=\s*(?:"([^"]*)"|([^;]*))/i.exec(contentDisposition);
  const filename = (match?.[1] ?? match?.[2] ?? "").trim();
  if (!filename.toLowerCase().endsWith(".zip") || filename.includes("/") || filename.includes("\\")) {
    return fallback;
  }
  return filename || fallback;
}
