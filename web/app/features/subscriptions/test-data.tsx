import { screen, within } from "@testing-library/react";
import type userEvent from "@testing-library/user-event";

import type { ProbeDefaultsInput } from "~/shared/api/client";
import type { ResourceOption } from "~/shared/resources/types";
import type { CreateSpeedDialAction } from "~/shared/ui/resource-list";

import type {
  SubscriptionDefinition,
  SubscriptionItem,
  SubscriptionPreview,
  SubscriptionTraffic,
} from "./model/types";

export const subscriptions: SubscriptionItem[] = [
  { kind: "remote", name: "provider", title: "provider", label: "远程订阅", status: "ready", format: "uri-list" },
  { kind: "remote", name: "warn", title: "warn", label: "远程订阅", status: "warning", format: "uri-list", warning: "远程订阅暂时不可用" },
  { kind: "collection", name: "default", title: "default", label: "组合订阅", status: "ready" },
];

export const manySourceSubscriptions: SubscriptionItem[] = Array.from({ length: 12 }, (_, index) => {
  const suffix = String(index + 1).padStart(2, "0");
  return { kind: "remote", name: `source-${suffix}`, title: `source-${suffix}`, label: "远程订阅", status: "ready", format: "uri-list" };
});

export const scriptFiles: ResourceOption[] = [
  { name: "default.yaml", title: "default.yaml" },
  { name: "rename.js", title: "rename.js" },
  { name: "other.js", title: "other.js" },
];

export const probeDefaults: ProbeDefaultsInput = {
  method: "url_test",
  core: "sing-box",
  url: "https://cp.cloudflare.com",
  ntp_server: "time.apple.com",
  timeout_ms: 5000,
  attempts: 1,
  concurrency: 10,
};

export const probeCacheTTLSeconds = 0;

export const remoteSubscriptionDefinition: SubscriptionDefinition = {
  name: "provider",
  kind: "remote",
  format: "base64",
  content: "c3M6Ly9leGFtcGxl",
  remote: {
    url: "https://example.com/sub",
    user_agent: "Sandrone Test",
    proxy: "http://127.0.0.1:7890",
    timeout_ms: 10000,
    cache_ttl_seconds: 45,
  },
  renderCacheTTLSeconds: 0,
  processors: [
    {
      name: "入口重命名",
      type: "rename",
      stage: "nodes",
      params: { mode: "prefix", value: "source-" },
    },
  ],
  sourceRefs: [],
  meta: { description: "daily", owner: "ops" },
};

export const subscriptionPreview: SubscriptionPreview = {
  subscriptionName: "provider",
  format: "uri-list",
  beforeCount: 2,
  afterCount: 1,
  statusCounts: { added: 0, modified: 1, removed: 1, unchanged: 0 },
  warnings: [{ code: "quick_settings_warning", message: "left unchanged", node: "keep" }],
  nodes: [
    {
      identity: "sha256:one",
      status: "modified",
      before: { name: "keep", type: "ss", server: "example.com", port: 8388, endpoint: "example.com:8388" },
      after: {
        name: "source-keep",
        type: "ss",
        server: "example.com",
        port: 8388,
        endpoint: "example.com:8388",
        raw: {
          password: "a".repeat(160),
        },
      },
    },
    {
      identity: "sha256:two",
      status: "removed",
      before: { name: "drop", type: "ss", server: "example.org", port: 8389, endpoint: "example.org:8389" },
    },
  ],
};

export const subscriptionTraffic: SubscriptionTraffic = {
  subscriptionName: "provider",
  kind: "remote",
  format: "uri-list",
  cached: false,
};

export const allStatusSubscriptionPreview: SubscriptionPreview = {
  subscriptionName: "provider",
  format: "uri-list",
  beforeCount: 3,
  afterCount: 3,
  statusCounts: { added: 1, modified: 1, removed: 1, unchanged: 1 },
  warnings: [],
  nodes: [
    {
      identity: "sha256:modified",
      status: "modified",
      before: { name: "before-node", type: "ss", endpoint: "before.example.com:8388" },
      after: { name: "after-node", type: "ss", endpoint: "after.example.com:8388" },
    },
    {
      identity: "sha256:removed",
      status: "removed",
      before: { name: "removed-node", type: "vmess", endpoint: "removed.example.com:443" },
    },
    {
      identity: "sha256:added",
      status: "added",
      after: { name: "added-node", type: "trojan", endpoint: "added.example.com:443" },
    },
    {
      identity: "sha256:unchanged",
      status: "unchanged",
      before: { name: "stable-node", type: "hysteria2", endpoint: "stable.example.com:8443" },
      after: { name: "stable-node", type: "hysteria2", endpoint: "stable.example.com:8443" },
    },
  ],
};

export const noop = () => undefined;

export function createAction(label: string, onSelect: () => void, ariaLabel?: string): CreateSpeedDialAction {
  return { ariaLabel, icon: <span aria-hidden>{label.slice(0, 1)}</span>, label, onSelect };
}

type TestUser = ReturnType<typeof userEvent.setup>;

export async function selectMuiOption(user: TestUser, combobox: HTMLElement, optionName: string) {
  await user.click(combobox);
  const listbox = await screen.findByRole("listbox");
  await user.click(within(listbox).getByRole("option", { name: optionName }));
}
