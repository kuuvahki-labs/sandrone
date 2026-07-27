import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  collectionDefinition,
  installDefaultFetchMock,
  jsonResponse,
  remoteSubscriptionDefinition,
  renderApp,
  resourceListResponse,
  resources,
  subscriptionPreview,
} from "./app-routing.test-data";

describe("React Router app subscription workflows", () => {
  beforeEach(installDefaultFetchMock);

  it("loads remote subscription traffic automatically and skips local or collection items", async () => {
    const listResources = {
      ...resources,
      subscriptions: [
        ...resources.subscriptions,
        { name: "local", type: "local", format: "uri-list", meta: { description: "scratch" } },
      ],
    };
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, listResources, init);
      if (resourceResponse) return resourceResponse;
      if (url.endsWith("/v1/subscriptions/provider/traffic")) {
        return jsonResponse({
          subscription_name: "provider",
          type: "remote",
          format: "uri-list",
          traffic: { upload_bytes: 1024, download_bytes: 2048, used_bytes: 3072, total_bytes: 10240, plan_name: "VIP 1" },
        });
      }
      if (url.endsWith("/v1/subscriptions/warn/traffic")) {
        return jsonResponse({
          subscription_name: "warn",
          type: "remote",
          format: "uri-list",
        });
      }
      return jsonResponse(remoteSubscriptionDefinition);
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions");

    await screen.findByRole("heading", { name: "我的订阅" });
    await waitFor(() => {
      expect(requests.some((request) => request.url.endsWith("/v1/subscriptions/provider/traffic"))).toBe(true);
      expect(requests.some((request) => request.url.endsWith("/v1/subscriptions/warn/traffic"))).toBe(true);
    });
    const list = screen.getByRole("list", { name: "订阅列表" });
    expect(await within(list).findByText("↑ 1 KiB · ↓ 2 KiB · TOT 10 KiB")).toBeInTheDocument();
    const trafficRequests = requests.filter((request) => request.url.includes("/v1/subscriptions/") && request.url.endsWith("/traffic"));
    expect(trafficRequests.map((request) => request.url.split("/v1/subscriptions/")[1])).toEqual([
      "provider/traffic",
      "warn/traffic",
    ]);
    expect(trafficRequests.every((request) => request.init?.method === "POST")).toBe(true);
  });

  it("identifies subscriptions in concurrent traffic failure notices", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.endsWith("/traffic")) {
        return jsonResponse({
          error: {
            code: "file_input_not_found",
            message: "remote returned status 503",
          },
        }, { status: 400 });
      }
      return jsonResponse(remoteSubscriptionDefinition);
    });
    vi.stubGlobal("fetch", fetchMock);

    renderApp("/subscriptions");

    expect(await screen.findByText("订阅「provider」的用量加载失败：remote returned status 503")).toBeInTheDocument();
    expect(await screen.findByText("订阅「warn」的用量加载失败：remote returned status 503")).toBeInTheDocument();
  });

  it("does not load subscription traffic while opening the edit route", async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/subscriptions/provider")) return jsonResponse(remoteSubscriptionDefinition);
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/remote/provider/edit");

    expect(await screen.findByRole("heading", { name: "编辑订阅" })).toBeInTheDocument();
    expect(requests.some((request) => request.url.endsWith("/v1/subscriptions/provider/traffic"))).toBe(false);
  });

  it("opens subscription new pages from the speed dial", async () => {
    const user = userEvent.setup();
    renderApp("/subscriptions");

    await screen.findByRole("heading", { name: "我的订阅" });
    await user.click(screen.getByRole("button", { name: "新建订阅" }));
    await user.click(await screen.findByRole("menuitem", { name: "新建组合订阅" }));

    expect(await screen.findByRole("heading", { name: "新建订阅" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "组合" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("textbox", { name: "名称" })).toHaveValue("default");
    expect(screen.getByRole("group", { name: "包含订阅" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "远程" }));

    expect(await screen.findByRole("textbox", { name: "订阅地址" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "远程" })).toHaveAttribute("aria-pressed", "true");
  });

  it("creates collections without binding a target format", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/new?type=collection");

    await screen.findByRole("heading", { name: "新建订阅" });
    expect(screen.queryByRole("textbox", { name: "输出格式" })).not.toBeInTheDocument();
    const providerSource = screen.getByRole("checkbox", { name: "provider 远程订阅 · uri-list" });
    const warnSource = screen.getByRole("checkbox", { name: "warn 远程订阅 · uri-list" });
    expect(providerSource).not.toBeChecked();
    expect(warnSource).not.toBeChecked();
    await user.click(providerSource);
    await user.click(warnSource);
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const post = requests.find((request) => request.url.endsWith("/v1/subscriptions") && request.init?.method === "POST");
    expect(post).toBeDefined();
    const body = JSON.parse(String(post?.init?.body));
    expect(body.target).toBeUndefined();
    expect(body.type).toBe("collection");
    expect(body.inputs).toEqual([
      { name: "provider", type: "subscription", ref: { kind: "subscription", name: "provider" } },
      { name: "warn", type: "subscription", ref: { kind: "subscription", name: "warn" } },
    ]);
  });

  it("opens the remote editor after creating a subscription", async () => {
    const user = userEvent.setup();
    let created = false;
    let postedBody: unknown;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/v1/subscriptions/example.com")) {
        return jsonResponse({ name: "example.com", type: "remote", remote: { url: "https://example.com/sub" }, meta: { ui: "web" } });
      }
      if (url.includes("/v1/subscriptions") && init?.method === "POST") {
        postedBody = JSON.parse(String(init.body));
        created = true;
        return jsonResponse({ ok: true }, { status: 201 });
      }
      const resourceResponse = resourceListResponse(url, created ? {
          ...resources,
          subscriptions: [...resources.subscriptions, { name: "example.com", type: "remote", format: "auto", meta: { ui: "web" } }],
        } : resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/new?type=remote");

    await screen.findByRole("heading", { name: "新建订阅" });
    fireEvent.change(screen.getByRole("textbox", { name: "订阅地址" }), {
      target: { value: "https://example.com/sub" },
    });
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    expect(postedBody).toMatchObject({
      type: "remote",
      remote: { url: "https://example.com/sub" },
    });
    expect(await screen.findByRole("heading", { name: "编辑订阅" }, { timeout: 3000 })).toBeInTheDocument();
    expect(await screen.findByRole("textbox", { name: "订阅地址" }, { timeout: 3000 })).toHaveValue("https://example.com/sub");
  });

  it("does not treat encoded slashes as subscription name separators in routes", async () => {
    const slashResources = {
      ...resources,
      subscriptions: [{ name: "remote/provider", type: "remote", format: "uri-list", meta: { ui: "legacy" } }],
    };
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const resourceResponse = resourceListResponse(url, slashResources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse(remoteSubscriptionDefinition);
    }));

    renderApp("/subscriptions/remote/remote%2Fprovider/edit");

    expect(await screen.findByRole("heading", { name: "订阅不存在" })).toBeInTheDocument();
    expect(screen.getByText("可能已被删除或重命名。")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "编辑订阅" })).not.toBeInTheDocument();
  });

  it("opens the collection editor after creating a combined subscription", async () => {
    const user = userEvent.setup();
    let created = false;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/v1/subscriptions") && init?.method === "POST") {
        created = true;
        return jsonResponse({ ok: true }, { status: 201 });
      }
      const resourceResponse = resourceListResponse(url, created ? {
          ...resources,
          subscriptions: [...resources.subscriptions, { name: "private", type: "collection", meta: { node_count: "0", source_count: "2" } }],
        } : resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/subscriptions/private")) {
        return jsonResponse({ name: "private", type: "collection", inputs: [] });
      }
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/new?type=collection");

    await screen.findByRole("heading", { name: "新建订阅" });
    fireEvent.change(screen.getByRole("textbox", { name: "名称" }), {
      target: { value: "private" },
    });
    await user.click(screen.getByRole("checkbox", { name: "provider 远程订阅 · uri-list" }));
    await user.click(screen.getByRole("checkbox", { name: "warn 远程订阅 · uri-list" }));
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    expect(await screen.findByRole("heading", { name: "编辑订阅" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "名称" })).toHaveValue("private");
  });

  it("loads stored collection inputs before editing a combined subscription", async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/subscriptions/default")) return jsonResponse(collectionDefinition);
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/collection/default/edit");

    expect(await screen.findByRole("heading", { name: "编辑订阅" })).toBeInTheDocument();
    await waitFor(() => {
      expect(requests.some((request) => request.url.includes("/v1/subscriptions/default"))).toBe(true);
    });
    const sourcePicker = screen.getByRole("group", { name: "包含订阅" });
    expect(within(sourcePicker).getByRole("checkbox", { name: "provider 远程订阅 · uri-list" })).not.toBeChecked();
    expect(within(sourcePicker).getByRole("checkbox", { name: "warn 远程订阅 · uri-list" })).toBeChecked();
    expect(screen.getByRole("group", { name: "处理器 只保留香港" })).toBeInTheDocument();
  });

  it("keeps new page input and shows an alert when creating a subscription fails", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/subscriptions") && init?.method === "POST") {
        return jsonResponse(
          { error: { code: "invalid_argument", message: "remote fetch failed" } },
          { status: 400 },
        );
      }
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/new?type=remote");

    await screen.findByRole("heading", { name: "新建订阅" });
    fireEvent.change(screen.getByRole("textbox", { name: "订阅地址" }), {
      target: { value: "https://example.com/sub" },
    });
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("remote fetch failed");
    expect(screen.getByRole("textbox", { name: "订阅地址" })).toHaveValue("https://example.com/sub");
  });

  it("blocks empty remote creation before sending a request", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/new?type=remote");

    await screen.findByRole("heading", { name: "新建订阅" });
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("请输入远程订阅 URL");
    expect(requests.some((request) => request.url.endsWith("/v1/subscriptions") && request.init?.method === "POST")).toBe(false);
  });

  it("blocks empty collection creation before sending a request", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/new?type=collection");

    await screen.findByRole("heading", { name: "新建订阅" });
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("请输入组合名称并选择至少一个包含订阅");
    expect(requests.some((request) => request.url.endsWith("/v1/subscriptions") && request.init?.method === "POST")).toBe(false);
  });

  it("creates local subscription content from multi-line share links as plain text", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true }, { status: init?.method === "POST" ? 201 : 200 });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/new?type=local");

    const content = "ss://YWVzLTEyOC1nY206c2VjcmV0@example.com:8388#node-a\nvmess://example";
    await screen.findByRole("heading", { name: "新建订阅" });
    const subscriptionInput = screen.getByRole("textbox", { name: "内容" });
    expect(subscriptionInput.tagName).toBe("TEXTAREA");
    fireEvent.change(subscriptionInput, { target: { value: content } });
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const post = requests.find((request) => request.url.endsWith("/v1/subscriptions") && request.init?.method === "POST");
    expect(post).toBeDefined();
    const body = JSON.parse(String(post?.init?.body));
    expect(body).toMatchObject({
      name: "manual",
      type: "local",
      content,
      meta: { ui: "web" },
    });
    expect(body.format).toBeUndefined();
    expect(body.remote).toBeUndefined();
  });

  it("opens subscription cards directly in the editor", async () => {
    const user = userEvent.setup();
    renderApp("/subscriptions");

    await screen.findByRole("heading", { name: "我的订阅" });
    await user.click(screen.getByRole("button", { name: "编辑：provider" }));

    expect(await screen.findByRole("heading", { name: "编辑订阅" })).toBeInTheDocument();
    expect(await screen.findByRole("group", { name: "处理器 入口重命名" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "订阅地址" })).toHaveValue("https://example.com/sub");
  });

  it("saves subscription edits after switching type and navigates to the new type", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const localResources = {
      ...resources,
      subscriptions: [
        { name: "provider", type: "local", format: "uri-list", meta: { description: "daily" } },
        ...resources.subscriptions.filter((item) => item.name !== "provider"),
      ],
    };
    const localSubscriptionDefinition = {
      name: "provider",
      type: "local",
      format: "uri-list",
      content: "ss://converted",
      meta: { description: "daily" },
    };
    let saved = false;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      if (url.includes("/v1/subscriptions") && init?.method === "POST") {
        saved = true;
        return jsonResponse({ ok: true }, { status: 201 });
      }
      const resourceResponse = resourceListResponse(url, saved ? localResources : resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/subscriptions/provider")) {
        return jsonResponse(saved ? localSubscriptionDefinition : remoteSubscriptionDefinition);
      }
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/remote/provider/edit");

    await screen.findByRole("textbox", { name: "订阅地址" });
    await user.click(screen.getByRole("button", { name: "本地" }));
    fireEvent.change(
      within(screen.getByRole("group", { name: "基本信息" })).getByRole("textbox", { name: "内容" }),
      { target: { value: "ss://converted" } },
    );
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const post = requests.find((request) => request.url.endsWith("/v1/subscriptions") && request.init?.method === "POST");
    expect(post).toBeDefined();
    const body = JSON.parse(String(post?.init?.body));
    expect(body).toMatchObject({ name: "provider", type: "local", content: "ss://converted" });
    expect(body.remote).toBeUndefined();
    expect(await screen.findByRole("button", { name: "本地" })).toHaveAttribute("aria-pressed", "true");
    expect(await screen.findByRole("textbox", { name: "内容" })).toHaveValue("ss://converted");
  });

  it("preserves processors and remote fields when saving remote edits", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/subscriptions/provider") && init?.method !== "POST") {
        return jsonResponse(remoteSubscriptionDefinition);
      }
      return jsonResponse({ ok: true }, { status: init?.method === "POST" ? 201 : 200 });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/remote/provider/edit");

    expect(await screen.findByRole("group", { name: "处理器 入口重命名" })).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "处理器 JSON" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "第 2 个处理器类型" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const post = requests.find((request) => request.url.endsWith("/v1/subscriptions") && request.init?.method === "POST");
    expect(post).toBeDefined();
    expect(JSON.parse(String(post?.init?.body))).toMatchObject({
      name: "provider",
      type: "remote",
      format: "base64",
      remote: {
        url: "https://example.com/sub",
        timeout_ms: 10000,
      },
      processors: [
        { type: "quick_settings", stage: "nodes" },
        ...remoteSubscriptionDefinition.processors,
      ],
      meta: { description: "daily", owner: "ops", ui: "web" },
    });
  });

  it("does not persist display-only auto format when saving remote edits", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const autoResources = {
      ...resources,
      subscriptions: [{ name: "auto", type: "remote", format: "auto", meta: { ui: "web" } }],
    };
    const autoSubscriptionDefinition = {
      name: "auto",
      type: "remote",
      remote: {
        url: "https://example.com/auto",
        timeout_ms: 10000,
      },
      meta: { ui: "web" },
    };
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, autoResources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/subscriptions/auto") && init?.method !== "POST") {
        return jsonResponse(autoSubscriptionDefinition);
      }
      return jsonResponse({ ok: true }, { status: init?.method === "POST" ? 201 : 200 });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/remote/auto/edit");

    expect(await screen.findByRole("textbox", { name: "订阅地址" })).toHaveValue("https://example.com/auto");
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const post = requests.find((request) => request.url.endsWith("/v1/subscriptions") && request.init?.method === "POST");
    expect(post).toBeDefined();
    const body = JSON.parse(String(post?.init?.body));
    expect(body.name).toBe("auto");
    expect(body.format).toBeUndefined();
  });

  it("opens a subscription processor preview from the editor", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/subscriptions/provider/preview")) return jsonResponse(subscriptionPreview);
      if (url.includes("/v1/subscriptions/provider")) return jsonResponse(remoteSubscriptionDefinition);
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/remote/provider/edit");

    await user.click(await screen.findByRole("button", { name: "预览订阅" }));

    expect(await screen.findByRole("heading", { name: "节点预览" })).toBeInTheDocument();
    expect(screen.getByText("source-node-a")).toBeInTheDocument();
    const previewRequest = requests.find((request) => request.url.endsWith("/v1/subscriptions/provider/preview"));
    expect(previewRequest?.init?.method).toBe("POST");
  });

  it("reloads the subscription definition after returning from preview", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    let saved = false;
    const updatedSubscriptionDefinition = {
      ...remoteSubscriptionDefinition,
      remote: {
        ...remoteSubscriptionDefinition.remote,
        url: "https://example.com/updated",
      },
    };
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/subscriptions") && init?.method === "POST") {
        saved = true;
        return jsonResponse({ ok: true }, { status: 201 });
      }
      if (url.includes("/v1/subscriptions/provider/preview")) return jsonResponse(subscriptionPreview);
      if (url.includes("/v1/subscriptions/provider")) return jsonResponse(saved ? updatedSubscriptionDefinition : remoteSubscriptionDefinition);
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderApp("/subscriptions/remote/provider/edit");

    const sourceInput = await screen.findByRole("textbox", { name: "订阅地址" });
    expect(sourceInput).toHaveValue("https://example.com/sub");
    fireEvent.change(sourceInput, { target: { value: "https://example.com/updated" } });
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    await waitFor(() => {
      expect(requests.some((request) => request.url.endsWith("/v1/subscriptions") && request.init?.method === "POST")).toBe(true);
    });
    const post = requests.find((request) => request.url.endsWith("/v1/subscriptions") && request.init?.method === "POST");
    expect(post).toBeDefined();
    expect(JSON.parse(String(post?.init?.body)).remote.url).toBe("https://example.com/updated");
    await user.click(screen.getByRole("button", { name: "预览订阅" }));

    expect(await screen.findByRole("heading", { name: "节点预览" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "返回" }));

    expect(await screen.findByRole("textbox", { name: "订阅地址" })).toHaveValue("https://example.com/updated");
    expect(requests.filter((request) => request.url.endsWith("/v1/subscriptions/provider") && request.init?.method === "GET").length).toBeGreaterThanOrEqual(2);
  });

});
