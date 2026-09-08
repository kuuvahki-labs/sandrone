import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearAdminToken, saveAdminToken } from "~/shared/storage/preferences";

import { ApiClient } from "./client";

function response(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), { status, headers: { "content-type": "application/json" } });
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}
beforeEach(() => saveAdminToken("test-session"));
afterEach(() => vi.restoreAllMocks());

describe("resource list cache", () => {
  it("reuses lists and traffic, clones results, and does not slide freshness", async () => {
    let now = 10_000;
    vi.spyOn(Date, "now").mockImplementation(() => now);
    const fetcher = vi.fn().mockImplementation(async () => response({ items: [{ name: "a", meta: { label: "safe" } }] }));
    const client = new ApiClient({ fetcher });
    const first = await client.listSubscriptions() as { items: { meta: { label: string } }[] };
    first.items[0].meta.label = "mutated";
    now += 4_000;
    expect(await client.listSubscriptions()).toEqual({ items: [{ name: "a", meta: { label: "safe" } }] });
    expect(fetcher).toHaveBeenCalledTimes(1);
    await client.subscriptionTraffic("a");
    await client.subscriptionTraffic("a");
    expect(fetcher).toHaveBeenCalledTimes(2);
    now += 1_000;
    expect(client.cachedResourceList("subscriptions")?.fresh).toBe(false);
    await client.listSubscriptions();
    expect(fetcher).toHaveBeenCalledTimes(3);
  });
  it("invalidates mutations, partial batches, and restore by resource kind", async () => {
    const fetcher = vi.fn().mockImplementation(async () => response({ items: [] }));
    const client = new ApiClient({ fetcher });
    await Promise.all([client.listSubscriptions(), client.listFiles(), client.listShares()]);
    await client.createSubscription({ name: "a", type: "local" });
    expect(client.cachedResourceList("subscriptions")).toBeUndefined();
    expect(client.cachedResourceList("files")).toBeDefined();
    await client.listSubscriptions();
    await client.deleteResource("files", "a");
    fetcher.mockResolvedValueOnce(response({}, 500));
    await expect(client.deleteResource("files", "b")).rejects.toMatchObject({ status: 500 });
    expect(client.cachedResourceList("files")).toBeUndefined();
    await client.restoreBackup(new Blob(["backup"]));
    expect(client.cachedResourceList("subscriptions")).toBeUndefined();
    expect(client.cachedResourceList("shares")).toBeUndefined();
  });
  it("replaces a pre-write list before returning it to the caller", async () => {
    const old = deferred<Response>();
    const fetcher = vi.fn().mockReturnValueOnce(old.promise).mockResolvedValueOnce(response({})).mockResolvedValueOnce(response({ items: ["new"] }));
    const client = new ApiClient({ fetcher });
    const pending = client.listFiles();
    await client.createFile({ name: "new", kind: "static", source: {} });
    old.resolve(response({ items: ["old"] }));
    expect(await pending).toEqual({ items: ["new"] });
    expect(client.cachedResourceList("files")?.value).toEqual({ items: ["new"] });
    expect(fetcher).toHaveBeenCalledTimes(3);
  });
  it("isolates addresses and resets on same-token login, logout, and 401", async () => {
    const fetcher = vi.fn().mockImplementation(async () => response({ items: [] }));
    const first = new ApiClient({ baseUrl: "https://first.example.com", fetcher });
    const second = new ApiClient({ baseUrl: "https://second.example.com", fetcher });
    await first.listShares();
    await second.listShares();
    expect(fetcher).toHaveBeenCalledTimes(2);
    saveAdminToken("test-session");
    expect(first.cachedResourceList("shares")).toBeUndefined();
    await first.listShares();
    clearAdminToken();
    expect(first.cachedResourceList("shares")).toBeUndefined();
    await first.listShares();
    fetcher.mockResolvedValueOnce(response({}, 401));
    await expect(first.listFiles()).rejects.toMatchObject({ status: 401 });
    expect(first.cachedResourceList("shares")).toBeUndefined();
  });
  it("rejects old-session responses without poisoning a new session", async () => {
    const old = deferred<Response>();
    const fetcher = vi.fn().mockReturnValueOnce(old.promise).mockResolvedValueOnce(response({ items: ["new"] }));
    const client = new ApiClient({ fetcher });
    const pending = client.listFiles();
    const rejected = expect(pending).rejects.toMatchObject({ code: "stale_session" });
    saveAdminToken("next-session");
    await client.listFiles();
    old.resolve(response({ items: ["old"] }));
    await rejected;
    expect(client.cachedResourceList("files")?.value).toEqual({ items: ["new"] });
  });
  it("keeps failed refreshes stale and permits retry", async () => {
    let now = 10_000;
    vi.spyOn(Date, "now").mockImplementation(() => now);
    const fetcher = vi.fn().mockResolvedValueOnce(response({ items: ["old"] })).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce(response({ items: ["new"] }));
    const client = new ApiClient({ fetcher });
    await client.listFiles();
    now += 5_000;
    await expect(client.listFiles()).rejects.toThrow("offline");
    expect(client.cachedResourceList("files")).toEqual({ value: { items: ["old"] }, fresh: false });
    expect(await client.listFiles()).toEqual({ items: ["new"] });
  });
});

it("rejects a detail response started before backup restoration", async () => {
  const old = deferred<Response>();
  const fetcher = vi.fn().mockReturnValueOnce(old.promise).mockResolvedValueOnce(response({}));
  const client = new ApiClient({ fetcher });
  const pending = client.getFileSpec("a");
  const rejected = expect(pending).rejects.toMatchObject({ code: "stale_resource" });
  await client.restoreBackup(new Blob(["backup"]));
  old.resolve(response({ name: "a", source: { content: "old" } }));
  await rejected;
});

it("keeps explicit traffic refresh authoritative for the next automatic read", async () => {
  const fetcher = vi.fn().mockResolvedValueOnce(response({ items: [] }))
    .mockResolvedValueOnce(response({ used: 1 }))
    .mockResolvedValueOnce(response({ used: 2 }));
  const client = new ApiClient({ fetcher });
  await client.listSubscriptions();
  expect(await client.subscriptionTraffic("a")).toEqual({ used: 1 });
  expect(await client.subscriptionTraffic("a", { refresh: true })).toEqual({ used: 2 });
  expect(await client.subscriptionTraffic("a")).toEqual({ used: 2 });
  expect(fetcher).toHaveBeenCalledTimes(3);
});
