import { type ComponentType, StrictMode } from "react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { render } from "@testing-library/react";
import { vi } from "vitest";

import { defaultProjectSettings, defaultSettingsEnvelope } from "~/features/settings/model/project-settings";
import AppRoot from "~/root";
import IndexRoute from "~/routes/_index";
import FilesRoute from "~/routes/files";
import FileEditRoute from "~/routes/files.$name.edit";
import FilePreviewRoute from "~/routes/files.$name.preview";
import FilesNewRoute from "~/routes/files.new";
import SettingsRoute from "~/routes/settings";
import SettingsDataRoute from "~/routes/settings.data";
import SettingsServiceRoute from "~/routes/settings.service";
import SharesRoute from "~/routes/shares";
import SubscriptionsRoute from "~/routes/subscriptions";
import SubscriptionEditRoute from "~/routes/subscriptions.$kind.$name.edit";
import SubscriptionPreviewRoute from "~/routes/subscriptions.$kind.$name.preview";
import NewSubscriptionRoute from "~/routes/subscriptions.new";

interface IntegrationRouteEntryBase {
  readonly Component: ComponentType;
  readonly file: string;
  readonly id: string;
}

export type IntegrationRouteEntry = IntegrationRouteEntryBase & (
  | { readonly index: true; readonly path?: never }
  | { readonly index?: never; readonly path: string }
);

export const integrationRouteEntries = [
  { id: "home", index: true, file: "routes/_index.tsx", Component: IndexRoute },
  { id: "subscriptions", path: "subscriptions", file: "routes/subscriptions.tsx", Component: SubscriptionsRoute },
  { id: "subscriptions-new", path: "subscriptions/new", file: "routes/subscriptions.new.tsx", Component: NewSubscriptionRoute },
  { id: "subscriptions-edit", path: "subscriptions/:kind/:name/edit", file: "routes/subscriptions.$kind.$name.edit.tsx", Component: SubscriptionEditRoute },
  { id: "subscriptions-preview", path: "subscriptions/:kind/:name/preview", file: "routes/subscriptions.$kind.$name.preview.tsx", Component: SubscriptionPreviewRoute },
  { id: "files", path: "files", file: "routes/files.tsx", Component: FilesRoute },
  { id: "files-new", path: "files/new", file: "routes/files.new.tsx", Component: FilesNewRoute },
  { id: "files-edit", path: "files/:name/edit", file: "routes/files.$name.edit.tsx", Component: FileEditRoute },
  { id: "files-preview", path: "files/:name/preview", file: "routes/files.$name.preview.tsx", Component: FilePreviewRoute },
  { id: "shares", path: "shares", file: "routes/shares.tsx", Component: SharesRoute },
  { id: "settings", path: "settings", file: "routes/settings.tsx", Component: SettingsRoute },
  { id: "settings-service", path: "settings/service", file: "routes/settings.service.tsx", Component: SettingsServiceRoute },
  { id: "settings-data", path: "settings/data", file: "routes/settings.data.tsx", Component: SettingsDataRoute },
] satisfies readonly IntegrationRouteEntry[];

export interface ResourceFixture {
  files: Array<Record<string, unknown>>;
  shares: Array<Record<string, unknown>>;
  subscriptions: Array<Record<string, unknown>>;
}

export const resources: ResourceFixture = {
  subscriptions: [
    { name: "provider", type: "remote", format: "uri-list", meta: { description: "daily" } },
    { name: "warn", type: "remote", format: "uri-list", meta: { description: "backup" } },
    { name: "default", type: "collection", meta: { node_count: "12", source_count: "2" } },
  ],
  files: [{ name: "default.yaml", type: "remote", target: "static", meta: { description: "main config" } }],
  shares: [{
    id: "sh_123",
    name: "mobile",
    target_kind: "file",
    target_name: "default.yaml",
    public_filename: "mobile",
  }],
};

export const remoteSubscriptionDefinition = {
  name: "provider",
  type: "remote",
  format: "base64",
  remote: {
    url: "https://example.com/sub",
    timeout_ms: 10000,
  },
  processors: [
    {
      name: "入口重命名",
      type: "rename",
      stage: "nodes",
      params: { mode: "prefix", value: "source-" },
    },
  ],
  meta: { description: "daily", owner: "ops" },
};

export const subscriptionPreview = {
  subscription_name: "provider",
  format: "uri-list",
  before_count: 2,
  after_count: 1,
  status_counts: { added: 0, modified: 1, removed: 1, unchanged: 0 },
  nodes: [
    {
      runtimeId: "runtime-one",
      status: "modified",
      before: { name: "node-a", type: "ss", server: "example.com", port: 8388 },
      after: { name: "source-node-a", type: "ss", server: "example.com", port: 8388 },
    },
    {
      runtimeId: "runtime-two",
      status: "removed",
      before: { name: "node-b", type: "ss", server: "example.org", port: 8389 },
    },
  ],
  warnings: [],
};

export const fileSpec = {
  name: "default.yaml",
  kind: "static",
  source: { type: "remote", remote: { url: "https://example.com/base.yaml" } },
  processors: [{ name: "文件脚本", type: "script", stage: "file", params: { source: { type: "file", name: "scripts/file.js" } } }],
  meta: { description: "main config" },
};

export const filePreview = {
  content_type: "application/yaml",
  body: "proxies: []\n",
  warnings: [{ code: "file_script_warning", message: "left unchanged", source: "default.yaml" }],
};

export const uiCapabilities = {
  features: [
    { key: "probe.enabled", enabled: true },
    { key: "scheduler.enabled", enabled: true },
    { key: "core.mihomo", enabled: true },
    { key: "core.sing_box", enabled: true },
  ],
};

export const formatCapabilities = {
  items: [
    {
      direction: "parse",
      format: "uri-list",
      node_types: ["ss"],
      reversible: false,
      field_counts: { supported: 1, lossy: 0, raw_only: 0 },
      revisions: [],
      href: "/v1/capabilities/formats/parse/uri-list",
    },
    {
      direction: "render",
      format: "base64",
      node_types: ["ss"],
      reversible: false,
      field_counts: { supported: 1, lossy: 0, raw_only: 0 },
      revisions: [],
      href: "/v1/capabilities/formats/render/base64",
    },
    {
      direction: "render",
      format: "json-nodes",
      node_types: ["ss"],
      reversible: true,
      field_counts: { supported: 1, lossy: 0, raw_only: 0 },
      revisions: [],
      href: "/v1/capabilities/formats/render/json-nodes",
    },
  ],
};

export function installDefaultFetchMock() {
  localStorage.clear();
  vi.restoreAllMocks();
  vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const resourceResponse = resourceListResponse(url, resources, init);
    if (resourceResponse) return resourceResponse;
    if (url === "/v1/settings") {
      return jsonResponse(defaultSettingsEnvelope(defaultProjectSettings));
    }
    if (url === "/v1/capabilities/ui") {
      return jsonResponse(uiCapabilities);
    }
    if (url === "/v1/capabilities/formats") {
      return jsonResponse(formatCapabilities);
    }
    if (url.includes("/v1/subscriptions/provider")) {
      if (url.includes("/preview")) {
        return jsonResponse(subscriptionPreview);
      }
      return jsonResponse(remoteSubscriptionDefinition);
    }
    return jsonResponse({ ok: true });
  }));
}

export function renderApp(initialEntry: string, options: { strictMode?: boolean } = {}) {
  const router = createMemoryRouter(
    [
      {
        path: "/",
        element: <AppRoot />,
        children: integrationRouteEntries.map((entry) => {
          const { Component, id } = entry;
          return "index" in entry
            ? { id, index: true, element: <Component /> }
            : { id, path: entry.path, element: <Component /> };
        }),
      },
    ],
    { initialEntries: [initialEntry] },
  );

  const app = <RouterProvider router={router} />;

  return { router, ...render(options.strictMode ? <StrictMode>{app}</StrictMode> : app) };
}

export function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: init.status,
    headers: { "content-type": "application/json" },
  });
}

export function projectSettingsEnvelope(options: {
  autoLoadTraffic?: boolean;
  locale?: "auto" | "zh-CN" | "en-US";
} = {}) {
  const settings = {
    ...defaultProjectSettings,
    appearance: {
      ...defaultProjectSettings.appearance,
      locale: options.locale ?? defaultProjectSettings.appearance.locale,
    },
    subscriptions: {
      auto_load_traffic: options.autoLoadTraffic ?? false,
    },
  };
  return defaultSettingsEnvelope(settings);
}

export function resourceListResponse(url: string, resources: ResourceFixture, init?: RequestInit): Response | null {
  if ((init?.method ?? "GET") !== "GET") {
    return null;
  }
  if (url.endsWith("/v1/subscriptions")) {
    return jsonResponse({ items: resources.subscriptions });
  }
  if (url.endsWith("/v1/files")) {
    return jsonResponse({ items: resources.files });
  }
  if (url.endsWith("/v1/shares")) {
    return jsonResponse({ shares: resources.shares });
  }
  return null;
}

export function asHeaders(headers: RequestInit["headers"]): Record<string, string> {
  if (!headers) return {};
  if (headers instanceof Headers) return Object.fromEntries(headers.entries());
  if (Array.isArray(headers)) return Object.fromEntries(headers);
  return headers as Record<string, string>;
}
