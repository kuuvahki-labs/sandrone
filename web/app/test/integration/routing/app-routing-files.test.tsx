import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  filePreview,
  fileSpec,
  installDefaultFetchMock,
  jsonResponse,
  renderApp,
  resourceListResponse,
  resources,
  subscriptionPreview,
} from "./app-routing.test-data";

describe("React Router app file workflows", () => {
  beforeEach(installDefaultFetchMock);

  it("shows files as a generic file workbench and creates remote file specs", async () => {
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
    const { router } = renderApp("/files");

    await screen.findByRole("heading", { name: "我的文件" });
    await user.click(screen.getByRole("button", { name: "新建文件" }));
    await user.click(await screen.findByRole("menuitem", { name: "远程文件" }));
    expect(await screen.findByRole("heading", { name: "新建文件" })).toBeInTheDocument();
    await user.clear(screen.getByRole("textbox", { name: "名称" }));
    await user.type(screen.getByRole("textbox", { name: "名称" }), "remote.yaml");
    const sourceMode = screen.getByRole("group", { name: "来源方式" });
    expect(within(sourceMode).getByRole("button", { name: "远程" })).toHaveAttribute("aria-pressed", "true");
    await user.type(screen.getByRole("textbox", { name: "远程地址" }), "https://example.com/base.yaml");
    await user.click(screen.getByRole("button", { name: "保存文件" }));

    const post = requests.find((request) => request.url.endsWith("/v1/files") && request.init?.method === "POST");
    expect(post).toBeDefined();
    const body = JSON.parse(String(post?.init?.body));
    expect(body).toMatchObject({
      name: "remote.yaml",
      source: { type: "remote", remote: { url: "https://example.com/base.yaml" } },
      processors: [],
      meta: { ui: "web" },
    });
    expect(body.target).toBeUndefined();
    expect(body.inputs).toBeUndefined();
    expect(body.node_inputs).toBeUndefined();
    expect(router.state.location.pathname).toBe("/files/remote.yaml/edit");
  });

  it("creates mihomo config files from subscriptions", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/subscriptions/provider/preview")) return jsonResponse(subscriptionPreview);
      return jsonResponse({ ok: true }, { status: init?.method === "POST" ? 201 : 200 });
    }));
    renderApp("/files");

    await screen.findByRole("heading", { name: "我的文件" });
    await user.click(screen.getByRole("button", { name: "新建文件" }));
    await user.click(await screen.findByRole("menuitem", { name: "mihomo 配置" }));
    expect(await screen.findByRole("heading", { name: "新建文件" })).toBeInTheDocument();
    await user.click(screen.getByRole("combobox", { name: "订阅" }));
    await user.click(await screen.findByRole("option", { name: "provider" }));
    expect(await screen.findByText("已加载 1 个节点")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "保存文件" }));

    expect(requests.filter((request) => request.url.includes("/v1/subscriptions/provider/preview") && request.init?.method === "POST")).toHaveLength(1);

    const post = requests.find((request) => request.url.endsWith("/v1/files") && request.init?.method === "POST");
    expect(post).toBeDefined();
    const previewRequestIndex = requests.findIndex((request) => request.url.includes("/v1/subscriptions/provider/preview") && request.init?.method === "POST");
    const filePostIndex = requests.findIndex((request) => request.url.endsWith("/v1/files") && request.init?.method === "POST");
    expect(previewRequestIndex).toBeGreaterThanOrEqual(0);
    expect(filePostIndex).toBeGreaterThan(previewRequestIndex);
    const body = JSON.parse(String(post?.init?.body));
    expect(body).toMatchObject({
      name: "mihomo.yaml",
      kind: "mihomo",
      source: { type: "inline", content: expect.stringContaining("mixed-port: 7890") },
      config: {
        subscriptions: ["provider"],
        settings: {
          adaptive_groups: expect.any(Object),
          groups: expect.any(Array),
          rule_sets: expect.any(Array),
          rules: expect.any(Array),
        },
      },
      processors: [
        { name: "Sniffer", type: "merge", stage: "file", params: { mode: "yaml_override", content: expect.stringContaining("# sandrone:mihomo-preset=sniffer") } },
        { name: "TUN", type: "merge", stage: "file", params: { mode: "yaml_override", content: expect.stringContaining("# sandrone:mihomo-preset=tun") } },
      ],
      meta: { ui: "web" },
    });
  });

  it("hydrates a typed config source and snapshots it on save", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const typedResources = {
      ...resources,
      files: [{ name: "default.yaml", type: "mihomo", target: "mihomo" }],
    };
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, typedResources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/files/default.yaml?mode=spec")) {
        return jsonResponse({
          name: "default.yaml",
          kind: "mihomo",
          source: {},
          config: {
            settings: {
              groups: [{ name: "Proxy", type: "select", proxies: ["DIRECT"] }],
              rule_sets: [{ name: "private", type: "inline", behavior: "classical", payload: ["DOMAIN-SUFFIX,local"] }],
              rules: ["RULE-SET,private,DIRECT", "MATCH,Proxy"],
            },
          },
          processors: [],
        });
      }
      if (url.includes("/v1/files/default.yaml?mode=source&response=json")) {
        return jsonResponse({ content_type: "application/yaml", body: "mixed-port: 7890\nallow-lan: false\n" });
      }
      return jsonResponse({ ok: true }, { status: init?.method === "POST" ? 201 : 200 });
    }));

    renderApp("/files/default.yaml/edit");

    expect(await screen.findByRole("textbox", { name: "内容" })).toHaveValue("mixed-port: 7890\nallow-lan: false\n");
    await user.click(screen.getByRole("button", { name: "保存文件" }));

    await waitFor(() => {
      const post = requests.find((request) => request.url.endsWith("/v1/files") && request.init?.method === "POST");
      expect(post).toBeDefined();
      expect(JSON.parse(String(post?.init?.body))).toMatchObject({
        kind: "mihomo",
        source: { type: "inline", content: "mixed-port: 7890\nallow-lan: false\n" },
      });
    });
  });

  it("blocks editing when a typed config source cannot be loaded", async () => {
    const typedResources = {
      ...resources,
      files: [{ name: "default.yaml", type: "mihomo", target: "mihomo" }],
    };
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const resourceResponse = resourceListResponse(url, typedResources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/files/default.yaml?mode=spec")) {
        return jsonResponse({ name: "default.yaml", kind: "mihomo", source: {}, processors: [] });
      }
      if (url.includes("/v1/files/default.yaml?mode=source&response=json")) {
        return jsonResponse({ error: { code: "internal_error", message: "source unavailable" } }, { status: 500 });
      }
      return jsonResponse({ ok: true });
    }));

    renderApp("/files/default.yaml/edit");

    expect(await screen.findByRole("heading", { name: "文件定义读取失败" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "保存文件" })).not.toBeInTheDocument();
  });

  it("hydrates and preserves an ordinary local file source on save", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/files/default.yaml?mode=spec")) {
        return jsonResponse({
          ...fileSpec,
          source: { type: "local", path: "files/default.yaml" },
        });
      }
      if (url.includes("/v1/files/default.yaml?mode=source&response=json")) {
        return jsonResponse({ content_type: "text/plain; charset=utf-8", body: "port: 7890" });
      }
      return jsonResponse({ ok: true }, { status: init?.method === "POST" ? 201 : 200 });
    }));
    const { router } = renderApp("/files/default.yaml/edit");

    const content = await screen.findByRole("textbox", { name: "内容" });
    expect(content).toHaveValue("port: 7890");
    expect(screen.getByRole("group", { name: "处理器 文件脚本" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "保存文件" }));

    await waitFor(() => {
      const post = requests.find((request) => request.url.endsWith("/v1/files") && request.init?.method === "POST");
      expect(post).toBeDefined();
      expect(JSON.parse(String(post?.init?.body))).toMatchObject({
        name: "default.yaml",
        source: { type: "local", path: "files/default.yaml" },
        processors: fileSpec.processors,
        meta: { ui: "web" },
      });
    });
    await screen.findByText("文件已保存");
    expect(router.state.location.pathname).toBe("/files/default.yaml/edit");
    expect(screen.getByRole("heading", { name: "编辑文件" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "我的文件" })).not.toBeInTheDocument();
  });

  it("saves empty file text without leaving the editor route", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/files/default.yaml?mode=spec")) {
        return jsonResponse({ ...fileSpec, source: { type: "inline", content: "port: 7890" } });
      }
      if (url.includes("/v1/files/default.yaml?mode=source&response=json")) {
        return jsonResponse({ content_type: "text/plain; charset=utf-8", body: "port: 7890" });
      }
      return jsonResponse({ ok: true }, { status: init?.method === "POST" ? 201 : 200 });
    }));
    const { router } = renderApp("/files/default.yaml/edit");

    const content = await screen.findByRole("textbox", { name: "内容" });
    expect(content).toHaveValue("port: 7890");
    await user.clear(content);
    await user.click(screen.getByRole("button", { name: "保存文件" }));

    await waitFor(() => {
      const post = requests.find((request) => request.url.endsWith("/v1/files") && request.init?.method === "POST");
      expect(post).toBeDefined();
      expect(JSON.parse(String(post?.init?.body))).toMatchObject({
        name: "default.yaml",
        source: { type: "inline", content: "" },
      });
    });
    expect(router.state.location.pathname).toBe("/files/default.yaml/edit");
  });

  it("hydrates remote file content while preserving its request metadata", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/files/default.yaml?mode=spec")) {
        return jsonResponse({
          ...fileSpec,
          source: {
            type: "remote",
            remote: {
              url: "https://example.com/base.yaml",
              user_agent: "Sandrone Tests",
              proxy: "http://127.0.0.1:7890",
              timeout_ms: 2500,
              cache_ttl_seconds: 60,
            },
          },
        });
      }
      if (url.includes("/v1/files/default.yaml?mode=source&response=json")) {
        return jsonResponse({ content_type: "text/plain; charset=utf-8", body: "remote: source\n" });
      }
      return jsonResponse({ ok: true });
    }));

    renderApp("/files/default.yaml/edit");

    const contentSource = await screen.findByRole("group", { name: "内容来源" });
    expect(within(contentSource).getByRole("textbox", { name: "远程地址" })).toHaveValue("https://example.com/base.yaml");
    expect(within(contentSource).getByRole("textbox", { name: "User-Agent" })).toHaveValue("Sandrone Tests");
    expect(within(contentSource).getByRole("textbox", { name: "代理" })).toHaveValue("http://127.0.0.1:7890");
    expect(within(contentSource).getByRole("spinbutton", { name: "超时毫秒" })).toHaveValue(2500);

    await user.click(screen.getByRole("button", { name: "保存文件" }));
    await waitFor(() => {
      const post = requests.find((request) => request.url.endsWith("/v1/files") && request.init?.method === "POST");
      expect(post).toBeDefined();
      expect(JSON.parse(String(post?.init?.body))).toMatchObject({
        source: {
          type: "remote",
          remote: {
            url: "https://example.com/base.yaml",
            user_agent: "Sandrone Tests",
            proxy: "http://127.0.0.1:7890",
            timeout_ms: 2500,
            cache_ttl_seconds: 60,
          },
        },
      });
    });

    await user.click(within(contentSource).getByRole("button", { name: "本地" }));

    expect(within(contentSource).getByRole("textbox", { name: "内容" })).toHaveValue("remote: source\n");
    expect(requests.some((request) => request.url.includes("?mode=source&response=json"))).toBe(true);
  });

  it("opens a saved file preview and returns to the editor", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/files/default.yaml?mode=spec")) return jsonResponse(fileSpec);
      if (url.includes("/v1/files/default.yaml?response=json")) return jsonResponse(filePreview);
      return jsonResponse({ ok: true });
    }));
    renderApp("/files/default.yaml/edit");

    await user.click(await screen.findByRole("button", { name: "预览文件" }));

    expect(await screen.findByRole("heading", { name: "文件预览" })).toBeInTheDocument();
    const previewBlock = screen.getByRole("region", { name: "最终文件内容" });
    expect(previewBlock).toHaveTextContent("proxies: []");
    const previewRequest = requests.find((request) => request.url.endsWith("/v1/files/default.yaml?response=json"));
    expect(previewRequest).toBeDefined();
    expect(previewRequest?.init?.method).toBe("GET");
    await user.click(screen.getByRole("button", { name: "返回" }));

    expect(await screen.findByRole("heading", { name: "编辑文件" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "远程地址" })).toHaveValue("https://example.com/base.yaml");
  });

  it.each([
    ["future-client", "future-client"],
    [undefined, ""],
  ])("opens an unregistered %s file read-only without posting and still previews it", async (specKind, expectedKind) => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const unknownResources = {
      ...resources,
      files: [{ name: "future.json", target: specKind, type: "remote" }],
    };
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, unknownResources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/files/future.json?mode=spec")) {
        return jsonResponse({
          name: "future.json",
          ...(specKind === undefined ? {} : { kind: specKind }),
          source: { type: "remote", remote: { url: "https://example.com/future.json" } },
          config: { settings: { future: true } },
          future_field: { keep: true },
          processors: [],
        });
      }
      if (url.includes("/v1/files/future.json?mode=source&response=json")) {
        return jsonResponse({ content_type: "application/json", body: "{}" });
      }
      if (url.includes("/v1/files/future.json?response=json")) {
        return jsonResponse({ content_type: "application/json", body: "{\"future\":true}", warnings: [] });
      }
      return jsonResponse({ ok: true });
    }));

    renderApp("/files/future.json/edit");

    expect(await screen.findByText("此文件类型未注册，只能查看定义和预览。")).toBeInTheDocument();
    const rawDefinition = screen.getByRole("region", { name: "原始文件定义" });
    if (expectedKind) {
      expect(rawDefinition).toHaveTextContent(`"kind": "${expectedKind}"`);
    } else {
      expect(rawDefinition).not.toHaveTextContent('"kind"');
    }
    expect(rawDefinition).toHaveTextContent("\"future_field\"");
    expect(screen.queryByRole("button", { name: "保存文件" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "预览文件" }));
    expect(await screen.findByRole("heading", { name: "文件预览" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "最终文件内容" })).toHaveTextContent("future");
    expect(requests.filter((request) => request.init?.method === "POST")).toHaveLength(0);
  });

  it("does not treat encoded slashes as file name separators in routes", async () => {
    const slashResources = {
      ...resources,
      files: [{ name: "files/default.txt", type: "inline", meta: { ui: "legacy" } }],
    };
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const resourceResponse = resourceListResponse(url, slashResources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({
        name: "files/default.txt",
        source: { type: "inline", content: "" },
        processors: [],
      });
    }));

    renderApp("/files/files%2Fdefault.txt/edit");

    expect(await screen.findByRole("heading", { name: "文件不存在" })).toBeInTheDocument();
    expect(screen.getByText("可能已被删除或重命名。")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "编辑文件" })).not.toBeInTheDocument();
  });

  it("labels the file definition loading state without repeating its context", async () => {
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const resourceResponse = resourceListResponse(String(input), resources, init);
      if (resourceResponse) return resourceResponse;
      return new Promise<Response>(() => undefined);
    }));

    renderApp("/files/default.yaml/edit");

    expect(await screen.findByRole("heading", { name: "正在读取文件定义" })).toBeInTheDocument();
  });

});
