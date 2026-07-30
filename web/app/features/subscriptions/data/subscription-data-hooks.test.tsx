import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { subscriptions, subscriptionTraffic } from "~/features/subscriptions/test-data";
import { type ApiClient, ApiError } from "~/shared/api/client";
import { createTranslator, type Translator } from "~/shared/i18n/context";

import {
  useSubscriptionDetailsResource,
  useSubscriptionResources,
} from "./use-subscription-resources";
import { useSubscriptionTrafficByKey } from "./use-subscription-traffic";

const t: Translator = (key) => key === "errors.subscriptionPreviewFailed"
  ? "subscription preview fallback"
  : key;
const ignoreNotice = () => undefined;

describe("subscription resource data", () => {
  it("decodes and sorts subscription resources by the default resource order", async () => {
    const client = apiClient({
      listSubscriptions: vi.fn().mockResolvedValue({ items: [
        { name: "older", type: "remote", created_at: "2026-01-01T00:00:00Z" },
        { name: "newer", type: "local", created_at: "2026-02-01T00:00:00Z" },
      ] }),
    });
    const { result } = renderHook(() => useSubscriptionResources({ client, showNotice: ignoreNotice, t }));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.items.map((item) => item.name)).toEqual(["newer", "older"]);
  });

  it("loads and decodes definition, preview, and traffic through their matching client methods", async () => {
    const getSubscription = vi.fn().mockResolvedValue({
      name: "bundle",
      type: "collection",
      inputs: [{ type: "subscription", name: "provider" }],
    });
    const previewSubscription = vi.fn().mockResolvedValue({
      subscription_name: "bundle",
      before_count: 0,
      after_count: 1,
      status_counts: { added: 1 },
      nodes: [{ identity: "node", status: "added", after: { name: "Node", server: "example.test", port: 443 } }],
    });
    const subscriptionTraffic = vi.fn().mockResolvedValue({
      subscription_name: "provider",
      type: "remote",
      cached: false,
      traffic: { upload_bytes: 1, download_bytes: 2, used_bytes: 3 },
    });
    const client = apiClient({ getSubscription, previewSubscription, subscriptionTraffic });
    const { result } = renderHook(() => useSubscriptionDetailsResource({ client, showNotice: ignoreNotice, t }));

    let definition = null;
    let preview = null;
    let traffic = null;
    await act(async () => {
      definition = await result.current.loadSubscriptionDefinition("bundle");
      preview = await result.current.loadSubscriptionPreview("bundle");
      traffic = await result.current.loadSubscriptionTraffic("provider", { refresh: true });
    });

    expect(getSubscription).toHaveBeenCalledWith("bundle");
    expect(definition).toMatchObject({ name: "bundle", kind: "collection", sourceRefs: ["provider"] });
    expect(previewSubscription).toHaveBeenCalledWith("bundle");
    expect(preview).toMatchObject({
      subscriptionName: "bundle",
      statusCounts: { added: 1, modified: 0, removed: 0, unchanged: 0 },
      nodes: [expect.objectContaining({ identity: "node", status: "added" })],
    });
    expect(subscriptionTraffic).toHaveBeenCalledWith("provider", { refresh: true });
    expect(traffic).toMatchObject({
      subscriptionName: "provider",
      kind: "remote",
      cached: false,
      traffic: { uploadBytes: 1, downloadBytes: 2, usedBytes: 3 },
    });
  });

  it("keeps 401 definition failures silent and reports non-Error preview failures with fallback copy", async () => {
    const showNotice = vi.fn();
    const client = apiClient({
      getSubscription: vi.fn().mockRejectedValue(new ApiError(401, "unauthorized", "unauthorized")),
      previewSubscription: vi.fn().mockRejectedValue("offline"),
    });
    const { result } = renderHook(() => useSubscriptionDetailsResource({ client, showNotice, t }));

    let definition = undefined;
    await act(async () => {
      definition = await result.current.loadSubscriptionDefinition("private");
    });
    expect(definition).toBeNull();
    expect(showNotice).not.toHaveBeenCalled();

    let preview = undefined;
    await act(async () => {
      preview = await result.current.loadSubscriptionPreview("private");
    });
    expect(preview).toBeNull();
    expect(showNotice).toHaveBeenCalledOnce();
    expect(showNotice).toHaveBeenCalledWith("subscription preview fallback", "error");
  });

  it("identifies each subscription when concurrent traffic loads fail", async () => {
    const showNotice = vi.fn();
    const subscriptionTraffic = vi.fn().mockRejectedValue(
      new ApiError(400, "file_input_not_found", "remote returned status 503"),
    );
    const client = apiClient({ subscriptionTraffic });
    const { result } = renderHook(() => useSubscriptionDetailsResource({
      client,
      showNotice,
      t: createTranslator("zh-CN"),
    }));

    let traffic = undefined;
    await act(async () => {
      traffic = await Promise.all([
        result.current.loadSubscriptionTraffic("provider"),
        result.current.loadSubscriptionTraffic("warn"),
      ]);
    });

    expect(traffic).toEqual([null, null]);
    expect(showNotice).toHaveBeenNthCalledWith(
      1,
      "订阅「provider」的用量加载失败：remote returned status 503",
      "error",
    );
    expect(showNotice).toHaveBeenNthCalledWith(
      2,
      "订阅「warn」的用量加载失败：remote returned status 503",
      "error",
    );
  });

  it("keeps 401 traffic failures silent", async () => {
    const showNotice = vi.fn();
    const client = apiClient({
      subscriptionTraffic: vi.fn().mockRejectedValue(new ApiError(401, "unauthorized", "unauthorized")),
    });
    const { result } = renderHook(() => useSubscriptionDetailsResource({ client, showNotice, t }));

    let traffic = undefined;
    await act(async () => {
      traffic = await result.current.loadSubscriptionTraffic("private");
    });

    expect(traffic).toBeNull();
    expect(showNotice).not.toHaveBeenCalled();
  });
});

function apiClient(methods: Partial<ApiClient>): ApiClient {
  return methods as ApiClient;
}

describe("useSubscriptionTrafficByKey", () => {
  it("clears pending remote traffic and ignores its result when automatic loading is disabled", async () => {
    const pending = deferred<typeof subscriptionTraffic>();
    const loadSubscriptionTraffic = vi.fn<(name: string) => Promise<typeof subscriptionTraffic>>(() => pending.promise);
    const { rerender, result } = renderHook(
      ({ enabled }: { enabled: boolean }) => useSubscriptionTrafficByKey(subscriptions, enabled, loadSubscriptionTraffic),
      { initialProps: { enabled: true } },
    );

    await waitFor(() => expect(loadSubscriptionTraffic).toHaveBeenCalledTimes(2));
    expect(loadSubscriptionTraffic.mock.calls.map(([name]) => name)).toEqual(["provider", "warn"]);

    rerender({ enabled: false });
    expect(result.current).toEqual({});

    await act(async () => {
      pending.resolve({
        ...subscriptionTraffic,
        traffic: { uploadBytes: 1024, downloadBytes: 2048, usedBytes: 3072 },
      });
      await pending.promise;
    });

    expect(result.current).toEqual({});
  });
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}
