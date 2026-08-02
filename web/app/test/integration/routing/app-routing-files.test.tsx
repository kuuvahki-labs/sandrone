import { fireEvent, screen, waitFor, within } from "@testing-library/react";
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
    const nameInput = screen.getByRole("textbox", { name: "名称" });
    fireEvent.change(nameInput, { target: { value: "remote.yaml" } });
    const sourceMode = screen.getByRole("group", { name: "来源方式" });
    expect(within(sourceMode).getByRole("button", { name: "远程" })).toHaveAttribute("aria-pressed", "true");
    const remoteURLInput = screen.getByRole("textbox", { name: "远程地址" });
    fireEvent.change(remoteURLInput, { target: { value: "https://example.com/base.yaml" } });
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

  it("keeps saved file text in the editor after refreshing the file list", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    let resolveRefresh!: () => void;
    const refreshPending = new Promise<void>((resolve) => {
      resolveRefresh = resolve;
    });
    let fileListRequests = 0;
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      if (url.endsWith("/v1/files") && (init?.method ?? "GET") === "GET") {
        fileListRequests += 1;
        if (fileListRequests > 1) await refreshPending;
      }
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/files/default.yaml?mode=spec")) {
        return jsonResponse({ ...fileSpec, source: { type: "inline", content: "port: 7890" } });
      }
      return jsonResponse({ ok: true }, { status: init?.method === "POST" ? 201 : 200 });
    }));
    const { router } = renderApp("/files/default.yaml/edit");

    const content = await screen.findByRole("textbox", { name: "内容" });
    expect(content).toHaveValue("port: 7890");
    fireEvent.change(content, { target: { value: "" } });
    await user.click(screen.getByRole("button", { name: "保存文件" }));

    await waitFor(() => {
      const post = requests.find((request) => request.url.endsWith("/v1/files") && request.init?.method === "POST");
      expect(post).toBeDefined();
      expect(JSON.parse(String(post?.init?.body))).toMatchObject({
        name: "default.yaml",
        source: { type: "inline", content: "" },
      });
      expect(requests.filter((request) => request.url.endsWith("/v1/files") && (request.init?.method ?? "GET") === "GET").length).toBeGreaterThanOrEqual(2);
    });
    expect(router.state.location.pathname).toBe("/files/default.yaml/edit");
    expect(screen.getByRole("textbox", { name: "内容" })).toHaveValue("");
    resolveRefresh();
    await screen.findByText("文件已保存");
    expect(screen.getByRole("textbox", { name: "内容" })).toHaveValue("");
  });

  it("round-trips a remote descriptor without fetching remote content", async () => {
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
      expect(JSON.parse(String(post?.init?.body)).source).toEqual({
          type: "remote",
          remote: {
            url: "https://example.com/base.yaml",
            user_agent: "Sandrone Tests",
            proxy: "http://127.0.0.1:7890",
            timeout_ms: 2500,
            cache_ttl_seconds: 60,
          },
      });
    });

    expect(requests.some((request) => request.url.includes("?mode=source"))).toBe(false);
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
    const { router } = renderApp("/files/default.yaml/edit");

    await user.click(await screen.findByRole("button", { name: "预览文件" }));

    expect(await screen.findByRole("heading", { name: "文件预览" })).toBeInTheDocument();
    expect(router.state.location.search).toBe("?from=edit");
    const previewBlock = screen.getByRole("region", { name: "最终文件内容" });
    expect(previewBlock).toHaveTextContent("proxies: []");
    const previewRequest = requests.find((request) => request.url.endsWith("/v1/files/default.yaml?response=json"));
    expect(previewRequest).toBeDefined();
    expect(previewRequest?.init?.method).toBe("GET");
    await user.click(screen.getByRole("button", { name: "返回编辑" }));

    expect(await screen.findByRole("heading", { name: "编辑文件" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "远程地址" })).toHaveValue("https://example.com/base.yaml");
    expect(requests.filter((request) => (
      request.url.endsWith("/v1/files/default.yaml?mode=spec")
      && (request.init?.method ?? "GET") === "GET"
    )).length).toBeGreaterThanOrEqual(2);
  });

  it("opens a file preview from the list and returns to the list", async () => {
    const user = userEvent.setup();
    const { router } = renderApp("/files");

    await screen.findByRole("heading", { name: "我的文件" });
    await user.click(screen.getByRole("button", { name: "default.yaml 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "预览文件" }));

    expect(await screen.findByRole("heading", { name: "文件预览" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/files/default.yaml/preview");
    expect(router.state.location.search).toBe("?from=list");

    await user.click(screen.getByRole("button", { name: "返回文件列表" }));
    expect(await screen.findByRole("heading", { name: "我的文件" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/files");
  });

  it("does not preview unsaved file edits", async () => {
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/files/default.yaml?mode=spec")) return jsonResponse(fileSpec);
      return jsonResponse({ ok: true });
    }));
    const { router } = renderApp("/files/default.yaml/edit");

    const remoteURL = await screen.findByRole("textbox", { name: "远程地址" });
    fireEvent.change(remoteURL, { target: { value: "https://example.com/changed.yaml" } });

    const preview = screen.getByRole("button", { name: "预览文件" });
    expect(preview).toHaveAttribute("aria-disabled", "true");
    expect(preview).toHaveAccessibleDescription("请先保存修改，再预览已保存版本");
    expect(router.state.location.pathname).toBe("/files/default.yaml/edit");
  });

  it("opens an unregistered future-client file read-only without posting and still previews it", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const unknownResources = {
      ...resources,
      files: [{ name: "future.json", target: "future-client", type: "remote" }],
    };
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, unknownResources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/files/future.json?mode=spec")) {
        return jsonResponse({
          name: "future.json",
          kind: "future-client",
          source: { type: "remote", remote: { url: "https://example.com/future.json" } },
          config: { settings: { future: true } },
          future_field: { keep: true },
          processors: [],
        });
      }
      if (url.includes("/v1/files/future.json?response=json")) {
        return jsonResponse({ content_type: "application/json", body: "{\"future\":true}", warnings: [] });
      }
      return jsonResponse({ ok: true });
    }));

    renderApp("/files/future.json/edit");

    expect(await screen.findByText("此文件类型未注册，只能查看定义和预览。")).toBeInTheDocument();
    const rawDefinition = screen.getByRole("region", { name: "原始文件定义" });
    expect(rawDefinition).toHaveTextContent('"kind": "future-client"');
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

});
