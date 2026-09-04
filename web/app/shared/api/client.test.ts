import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { saveAdminToken } from "~/shared/storage/preferences";

import { ApiClient, type Fetcher } from "./client";

type FetchInput = Parameters<Fetcher>[0];
type FetchOptions = NonNullable<Parameters<Fetcher>[1]>;
type FetchBody = FetchOptions["body"];

class MemoryStorage implements Storage {
  readonly #items = new Map<string, string>();

  get length(): number {
    return this.#items.size;
  }

  clear(): void {
    this.#items.clear();
  }

  getItem(key: string): string | null {
    return this.#items.get(key) ?? null;
  }

  key(index: number): string | null {
    return Array.from(this.#items.keys())[index] ?? null;
  }

  removeItem(key: string): void {
    this.#items.delete(key);
  }

  setItem(key: string, value: string): void {
    this.#items.set(key, value);
  }
}

const originalLocalStorageDescriptor = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
const testStorage = new MemoryStorage();

beforeAll(() => {
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: testStorage,
  });
});

afterAll(() => {
  if (originalLocalStorageDescriptor) {
    Object.defineProperty(globalThis, "localStorage", originalLocalStorageDescriptor);
  } else {
    Reflect.deleteProperty(globalThis, "localStorage");
  }
});

describe("ApiClient", () => {
  beforeEach(() => {
    testStorage.clear();
  });

  it("sends bearer auth for protected requests", async () => {
    saveAdminToken("secret");
    const calls: FetchOptions[] = [];
    const client = new ApiClient({
      fetcher: async (_input, init) => {
        calls.push(init ?? {});
        return new Response(JSON.stringify({ subscriptions: [] }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    await client.listSubscriptions();

    expect(calls[0]?.headers).toEqual({ Authorization: "Bearer secret" });
  });

  it("posts a node and requested information to the protected inspection endpoint", async () => {
    saveAdminToken("secret");
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(JSON.stringify({ server: "proxy.example.com", ip: "198.18.0.1", ip_version: 4, public: false }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    const node = { name: "fixture", server: "proxy.example.com", type: "trojan" };
    await client.inspectNode({ node, include: ["ip"] });

    expect(String(calls[0]?.input)).toBe("/v1/nodes/inspect");
    expect(calls[0]?.init?.method).toBe("POST");
    expect(calls[0]?.init?.headers).toEqual({ Authorization: "Bearer secret", "Content-Type": "application/json" });
    expect(calls[0]?.init?.body).toBe(JSON.stringify({ node, include: ["ip"] }));
  });

  it("omits auth for health checks", async () => {
    saveAdminToken("secret");
    const calls: FetchOptions[] = [];
    const client = new ApiClient({
      fetcher: async (_input, init) => {
        calls.push(init ?? {});
        return new Response("ok");
      },
    });

    await client.getHealth();

    expect(calls[0]?.headers).toEqual({});
  });

  it("loads version information without bearer auth", async () => {
    saveAdminToken("secret");
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(JSON.stringify({
          build_time: "2026-08-30T03:15:42Z",
          name: "sandrone",
          version: "0.1.0",
          revision: "0123456789abcdef",
        }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    const version = await client.getVersion();

    expect(version).toEqual({
      build_time: "2026-08-30T03:15:42Z",
      name: "sandrone",
      version: "0.1.0",
      revision: "0123456789abcdef",
    });
    expect(String(calls[0]?.input)).toBe("/version");
    expect(calls[0]?.init?.headers).toEqual({});
  });

  it("decodes structured API errors", async () => {
    const client = new ApiClient({
      fetcher: async () =>
        new Response(JSON.stringify({ error: { code: "unauthorized", message: "token required" } }), {
          status: 401,
          headers: { "content-type": "application/json" },
        }),
    });

    await expect(client.listSubscriptions()).rejects.toMatchObject({
      status: 401,
      code: "unauthorized",
      message: "token required",
    });
  });

  it("calls the unauthorized callback for protected 401 responses", async () => {
    const onUnauthorized = vi.fn();
    const client = new ApiClient({
      onUnauthorized,
      fetcher: async () =>
        new Response(JSON.stringify({ error: { code: "unauthorized", message: "token required" } }), {
          status: 401,
          headers: { "content-type": "application/json" },
        }),
    });

    await expect(client.listSubscriptions()).rejects.toMatchObject({ status: 401 });

    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });

  it("does not call the unauthorized callback for auth-free requests", async () => {
    const onUnauthorized = vi.fn();
    const client = new ApiClient({
      onUnauthorized,
      fetcher: async () =>
        new Response(JSON.stringify({ error: { code: "unauthorized", message: "token required" } }), {
          status: 401,
          headers: { "content-type": "application/json" },
        }),
    });

    await expect(client.getHealth()).rejects.toMatchObject({ status: 401 });

    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it("downloads an authenticated backup as a raw blob with the server filename", async () => {
    saveAdminToken("secret");
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(new Uint8Array([80, 75, 3, 4]), {
          headers: {
            "content-disposition": 'attachment; filename="sandrone-backup-20260721T120000Z.zip"',
            "content-type": "application/zip",
          },
        });
      },
    });

    const result = await client.downloadBackup();

    expect(String(calls[0]?.input)).toBe("/v1/backup");
    expect(calls[0]?.init?.method).toBe("GET");
    expect(calls[0]?.init?.headers).toEqual({ Authorization: "Bearer secret" });
    expect(result.filename).toBe("sandrone-backup-20260721T120000Z.zip");
    expect(Array.from(new Uint8Array(await result.blob.arrayBuffer()))).toEqual([80, 75, 3, 4]);
  });

  it.each([
    ["a missing header", undefined],
    ["an unsafe filename", 'attachment; filename="../private.zip"'],
    ["a non-ZIP filename", 'attachment; filename="backup.txt"'],
  ])("uses the stable backup filename fallback for %s", async (_case, contentDisposition) => {
    const client = new ApiClient({
      fetcher: async () => new Response(new Uint8Array([80, 75]), {
        headers: contentDisposition ? { "content-disposition": contentDisposition } : undefined,
      }),
    });

    const result = await client.downloadBackup();

    expect(result.filename).toBe("sandrone-backup.zip");
  });

  it("uploads an authenticated backup as the raw ZIP blob", async () => {
    saveAdminToken("secret");
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const archive = new Blob([new Uint8Array([80, 75, 3, 4])], { type: "application/zip" });
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(JSON.stringify({ ok: true }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    await client.restoreBackup(archive);

    expect(String(calls[0]?.input)).toBe("/v1/backup/restore");
    expect(calls[0]?.init?.method).toBe("POST");
    expect(calls[0]?.init?.headers).toEqual({ Authorization: "Bearer secret" });
    expect(calls[0]?.init?.body).toBe(archive);
  });

  it("decodes structured errors from raw backup responses", async () => {
    const client = new ApiClient({
      fetcher: async () => new Response(JSON.stringify({
        error: { code: "backup_incompatible", message: "storage schema version is incompatible" },
      }), {
        status: 422,
        headers: { "content-type": "application/json" },
      }),
    });

    await expect(client.downloadBackup()).rejects.toMatchObject({
      status: 422,
      code: "backup_incompatible",
      message: "storage schema version is incompatible",
    });
  });

  it("clears cache through the protected management API", async () => {
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(null, { status: 204 });
      },
    });

    await expect(client.clearCache()).resolves.toBeUndefined();
    expect(String(calls[0]?.input)).toBe("/v1/cache");
    expect(calls[0]?.init?.method).toBe("DELETE");
  });

  it.each(["download", "restore"] as const)("calls the unauthorized callback when backup %s returns 401", async (operation) => {
    const onUnauthorized = vi.fn();
    const client = new ApiClient({
      onUnauthorized,
      fetcher: async () => new Response(JSON.stringify({
        error: { code: "unauthorized", message: "token required" },
      }), {
        status: 401,
        headers: { "content-type": "application/json" },
      }),
    });

    const request = operation === "download"
      ? client.downloadBackup()
      : client.restoreBackup(new Blob([], { type: "application/zip" }));
    await expect(request).rejects.toMatchObject({ status: 401, code: "unauthorized" });

    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });

  it("requests each resource list endpoint", async () => {
    const calls: string[] = [];
    const client = new ApiClient({
      fetcher: async (input) => {
        calls.push(String(input));
        return new Response(JSON.stringify({ items: [], shares: [] }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    await client.listSubscriptions();
    await client.listFiles();
    await client.listShares();

    expect(calls).toEqual([
      "/v1/subscriptions",
      "/v1/files",
      "/v1/shares",
    ]);
  });

  it("preserves the ordered subscription request flow", async () => {
    const calls: Array<{ input: string; method: string; body?: FetchBody }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({
          input: String(input),
          method: init?.method ?? "GET",
          body: init?.body,
        });
        return new Response(JSON.stringify({ ok: true }), {
          headers: { "content-type": "application/json" },
        });
      },
    });
    const subscription = {
      name: "team/provider",
      type: "local" as const,
      content: "ss://example",
    };

    await client.createSubscription(subscription);
    await client.getSubscription(subscription.name);
    await client.previewSubscription(subscription.name);

    expect(calls).toEqual([
      {
        input: "/v1/subscriptions",
        method: "POST",
        body: JSON.stringify(subscription),
      },
      {
        input: "/v1/subscriptions/team%2Fprovider",
        method: "GET",
        body: undefined,
      },
      {
        input: "/v1/subscriptions/team%2Fprovider/preview",
        method: "POST",
        body: undefined,
      },
    ]);
  });

  it("preserves the ordered file request flow", async () => {
    const calls: Array<{ input: string; method: string; body?: FetchBody }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({
          input: String(input),
          method: init?.method ?? "GET",
          body: init?.body,
        });
        return new Response(JSON.stringify({ ok: true }), {
          headers: { "content-type": "application/json" },
        });
      },
    });
    const file = {
      name: "team/default.yaml",
      kind: "static",
      source: { type: "inline", content: "mixed-port: 7890\n" },
    };

    await client.createFile(file);
    await client.getFileSpec(file.name);
    await client.getFileSource(file.name);
    await client.previewFile(file.name);

    expect(calls).toEqual([
      {
        input: "/v1/files",
        method: "POST",
        body: JSON.stringify(file),
      },
      {
        input: "/v1/files/team%2Fdefault.yaml?mode=spec",
        method: "GET",
        body: undefined,
      },
      {
        input: "/v1/files/team%2Fdefault.yaml?mode=source&response=json",
        method: "GET",
        body: undefined,
      },
      {
        input: "/v1/files/team%2Fdefault.yaml?response=json",
        method: "GET",
        body: undefined,
      },
    ]);
  });

  it("sends force-refresh flags only for explicit preview refreshes", async () => {
    const calls: Array<{ input: string; method: string; body?: FetchBody }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({
          input: String(input),
          method: init?.method ?? "GET",
          body: init?.body,
        });
        return new Response(JSON.stringify({ ok: true }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    await client.previewSubscription("provider");
    await client.previewSubscription("provider", { refresh: true });
    await client.previewFile("client.yaml");
    await client.previewFile("client.yaml", { refresh: true });
    await client.getScheduledRefreshStatus();
    await client.runScheduledRefresh();

    expect(calls).toEqual([
      { input: "/v1/subscriptions/provider/preview", method: "POST", body: undefined },
      { input: "/v1/subscriptions/provider/preview", method: "POST", body: JSON.stringify({ refresh: true }) },
      { input: "/v1/files/client.yaml?response=json", method: "GET", body: undefined },
      { input: "/v1/files/client.yaml?response=json&refresh=true", method: "GET", body: undefined },
      { input: "/v1/settings/scheduled-refresh-status", method: "GET", body: undefined },
      { input: "/v1/settings/scheduled-refresh/run", method: "POST", body: undefined },
    ]);
  });

  it("coalesces concurrent subscription list requests", async () => {
    let resolveResponse: (() => void) | undefined;
    const responseReady = new Promise<void>((resolve) => {
      resolveResponse = resolve;
    });
    const fetcher = vi.fn(async () => {
      await responseReady;
      return new Response(JSON.stringify({ items: [] }), {
        headers: { "content-type": "application/json" },
      });
    });
    const client = new ApiClient({ fetcher });

    const first = client.listSubscriptions();
    const second = client.listSubscriptions();
    resolveResponse?.();
    await Promise.all([first, second]);

    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("coalesces concurrent file list requests", async () => {
    let resolveResponse: (() => void) | undefined;
    const responseReady = new Promise<void>((resolve) => {
      resolveResponse = resolve;
    });
    const fetcher = vi.fn(async () => {
      await responseReady;
      return new Response(JSON.stringify({ items: [] }), {
        headers: { "content-type": "application/json" },
      });
    });
    const client = new ApiClient({ fetcher });

    const first = client.listFiles();
    const second = client.listFiles();
    resolveResponse?.();
    await Promise.all([first, second]);

    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("loads format capabilities with bearer auth and coalesces concurrent requests", async () => {
    saveAdminToken("secret");
    let resolveResponse: (() => void) | undefined;
    const responseReady = new Promise<void>((resolve) => {
      resolveResponse = resolve;
    });
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const capabilityList = {
      items: [{
        direction: "render",
        field_counts: { lossy: 0, raw_only: 0, supported: 1 },
        format: "base64",
        href: "/v1/capabilities/formats/render/base64",
        node_types: ["ss"],
        reversible: false,
        revisions: [],
      }],
    } as const;
    const fetcher = vi.fn(async (input: FetchInput, init?: FetchOptions) => {
      calls.push({ input, init });
      await responseReady;
      return new Response(JSON.stringify(capabilityList), {
        headers: { "content-type": "application/json" },
      });
    });
    const client = new ApiClient({ fetcher });

    const first = client.listFormatCapabilities();
    const second = client.listFormatCapabilities();
    resolveResponse?.();
    const [firstResult, secondResult] = await Promise.all([first, second]);

    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(firstResult).toEqual(capabilityList);
    expect(secondResult).toBe(firstResult);
    expect(String(calls[0]?.input)).toBe("/v1/capabilities/formats");
    expect(calls[0]?.init?.headers).toEqual({ Authorization: "Bearer secret" });
  });

  it("bypasses a pending project settings request when a fresh read is requested", async () => {
    let resolveStale: ((response: Response) => void) | undefined;
    let resolveFresh: ((response: Response) => void) | undefined;
    const staleResponse = new Promise<Response>((resolve) => {
      resolveStale = resolve;
    });
    const freshResponse = new Promise<Response>((resolve) => {
      resolveFresh = resolve;
    });
    const staleSettings = { remote_defaults: { user_agent: "stale-agent" } };
    const restoredSettings = { remote_defaults: { user_agent: "restored-agent" } };
    const fetcher = vi.fn()
      .mockImplementationOnce(() => staleResponse)
      .mockImplementationOnce(() => freshResponse)
      .mockResolvedValueOnce(new Response(JSON.stringify(restoredSettings), {
        headers: { "content-type": "application/json" },
      }));
    const client = new ApiClient({ fetcher });

    const stale = client.getSettings();
    const fresh = client.getSettings({ fresh: true });

    expect(fetcher).toHaveBeenCalledTimes(2);
    resolveStale?.(new Response(JSON.stringify(staleSettings), {
      headers: { "content-type": "application/json" },
    }));
    await expect(stale).resolves.toEqual(staleSettings);

    const afterStale = client.getSettings();
    expect(fetcher).toHaveBeenCalledTimes(2);

    resolveFresh?.(new Response(JSON.stringify(restoredSettings), {
      headers: { "content-type": "application/json" },
    }));
    await expect(fresh).resolves.toEqual(restoredSettings);
    await expect(afterStale).resolves.toEqual(restoredSettings);

    const next = client.getSettings();
    expect(fetcher).toHaveBeenCalledTimes(3);
    await expect(next).resolves.toEqual(restoredSettings);
  });

  it("uses the unified settings endpoint for reads and updates", async () => {
    const envelope = { settings: {}, effective: {}, overrides: {}, restart_required: [] };
    const fetcher = vi.fn(async () => new Response(JSON.stringify(envelope), {
      headers: { "content-type": "application/json" },
    }));
    const client = new ApiClient({ fetcher });

    await client.getSettings();
    await client.updateSettings({} as never);

    const calls = fetcher.mock.calls as unknown as Array<[FetchInput, FetchOptions?]>;
    expect(calls.map(([input, init]) => [String(input), init?.method ?? "GET"])).toEqual([
      ["/v1/settings", "GET"],
      ["/v1/settings", "PUT"],
    ]);
  });

  it("requests subscription previews with encoded resource names", async () => {
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(JSON.stringify({ subscription_name: "provider", nodes: [] }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    await client.previewSubscription("provider");

    expect(String(calls[0]?.input)).toBe("/v1/subscriptions/provider/preview");
    expect(calls[0]?.init?.method).toBe("POST");
    expect(calls[0]?.init?.body).toBeUndefined();
  });

  it("requests subscription traffic with refresh control", async () => {
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(JSON.stringify({ subscription_name: "provider", traffic: { used_bytes: 0 } }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    await client.subscriptionTraffic("provider", { refresh: true });

    expect(String(calls[0]?.input)).toBe("/v1/subscriptions/provider/traffic");
    expect(calls[0]?.init?.method).toBe("POST");
    expect(calls[0]?.init?.body).toBe(JSON.stringify({ refresh: true }));
  });

  it("requests the complete target-specific rule-set catalog", async () => {
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(JSON.stringify({ items: [] }), { headers: { "content-type": "application/json" } });
      },
    });

    await client.listRuleSetCatalog("mihomo");

    expect(String(calls[0]?.input)).toBe("/v1/rule-set-catalog?target=mihomo");
    expect(calls[0]?.init?.method).toBe("GET");
    expect(calls[0]?.init?.body).toBeUndefined();
  });

  it("previews saved files as JSON", async () => {
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(JSON.stringify({ body: "ok" }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    await client.previewFile("default.yaml");

    expect(String(calls[0]?.input)).toBe("/v1/files/default.yaml?response=json");
    expect(calls[0]?.init?.method).toBe("GET");
  });

  it("loads saved file source before compilation", async () => {
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(JSON.stringify({ content_type: "application/yaml", body: "mixed-port: 7890\n" }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    await client.getFileSource("default.yaml");

    expect(String(calls[0]?.input)).toBe("/v1/files/default.yaml?mode=source&response=json");
    expect(calls[0]?.init?.method).toBe("GET");
  });

  it("creates subscription shares with target format and valid range", async () => {
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(JSON.stringify({ share: {} }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    await client.createShare({
      name: "mobile",
      target_kind: "subscription",
      target_name: "nodes",
      target_format: "mihomo-proxies",
      valid_from: "2026-06-21T00:00:00.000Z",
      valid_until: "2026-07-01T00:00:00.000Z",
    });

    expect(String(calls[0]?.input)).toBe("/v1/shares");
    expect(calls[0]?.init?.method).toBe("POST");
    expect(calls[0]?.init?.body).toBe(JSON.stringify({
      name: "mobile",
      target_kind: "subscription",
      target_name: "nodes",
      target_format: "mihomo-proxies",
      valid_from: "2026-06-21T00:00:00.000Z",
      valid_until: "2026-07-01T00:00:00.000Z",
    }));
  });

  it("deletes shares with DELETE", async () => {
    const calls: Array<{ input: FetchInput; init?: FetchOptions }> = [];
    const client = new ApiClient({
      fetcher: async (input, init) => {
        calls.push({ input, init });
        return new Response(JSON.stringify({ ok: true }), {
          headers: { "content-type": "application/json" },
        });
      },
    });

    await client.deleteShare("sh_123");

    expect(String(calls[0]?.input)).toBe("/v1/shares/sh_123");
    expect(calls[0]?.init?.method).toBe("DELETE");
  });

  it("coalesces concurrent subscription definition requests", async () => {
    let resolveResponse: (() => void) | undefined;
    const responseReady = new Promise<void>((resolve) => {
      resolveResponse = resolve;
    });
    const fetcher = vi.fn(async () => {
      await responseReady;
      return new Response(JSON.stringify({ name: "provider" }), {
        headers: { "content-type": "application/json" },
      });
    });
    const client = new ApiClient({ fetcher });

    const first = client.getSubscription("provider");
    const second = client.getSubscription("provider");
    resolveResponse?.();
    await Promise.all([first, second]);

    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("coalesces concurrent subscription preview requests", async () => {
    let resolveResponse: (() => void) | undefined;
    const responseReady = new Promise<void>((resolve) => {
      resolveResponse = resolve;
    });
    const fetcher = vi.fn(async () => {
      await responseReady;
      return new Response(JSON.stringify({ subscription_name: "provider", nodes: [] }), {
        headers: { "content-type": "application/json" },
      });
    });
    const client = new ApiClient({ fetcher });

    const first = client.previewSubscription("provider");
    const second = client.previewSubscription("provider");
    resolveResponse?.();
    await Promise.all([first, second]);

    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("coalesces concurrent subscription traffic requests", async () => {
    let resolveResponse: (() => void) | undefined;
    const responseReady = new Promise<void>((resolve) => {
      resolveResponse = resolve;
    });
    const fetcher = vi.fn(async () => {
      await responseReady;
      return new Response(JSON.stringify({ subscription_name: "provider", traffic: { used_bytes: 0 } }), {
        headers: { "content-type": "application/json" },
      });
    });
    const client = new ApiClient({ fetcher });

    const first = client.subscriptionTraffic("provider");
    const second = client.subscriptionTraffic("provider");
    resolveResponse?.();
    await Promise.all([first, second]);

    expect(fetcher).toHaveBeenCalledTimes(1);
  });
});
