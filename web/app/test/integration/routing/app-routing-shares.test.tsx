import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  installDefaultFetchMock,
  jsonResponse,
  remoteSubscriptionDefinition,
  renderApp,
  resourceListResponse,
  resources,
  uiCapabilities,
} from "./app-routing.test-data";

describe("React Router app share and delete workflows", () => {
  let originalClipboardDescriptor: PropertyDescriptor | undefined;

  beforeEach(installDefaultFetchMock);
  afterEach(() => {
    window.history.replaceState({}, "", "/");
    if (originalClipboardDescriptor) {
      Object.defineProperty(navigator, "clipboard", originalClipboardDescriptor);
    } else {
      Reflect.deleteProperty(navigator, "clipboard");
    }
    originalClipboardDescriptor = undefined;
  });

  it("uses a confirmation dialog for destructive resource deletes", async () => {
    const user = userEvent.setup();
    renderApp("/subscriptions");

    await screen.findByRole("heading", { name: "我的订阅" });
    await user.click(screen.getByRole("button", { name: "provider 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "删除" }));

    expect(document.querySelector(".dialog-backdrop")).not.toBeInTheDocument();
    expect(document.querySelector(".dialog")).not.toBeInTheDocument();
    const dialog = screen.getByRole("dialog", { name: "删除“provider”？" });
    expect(dialog).toHaveTextContent("不会级联删除");
    expect(within(dialog).getByRole("button", { name: "删除“provider”" })).toHaveTextContent("删除");
  });

  it("shows full share URLs using the current origin when no public base URL is configured", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn(async () => undefined);
    originalClipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    renderApp("/shares");

    await screen.findByRole("heading", { level: 2, name: "分享" });
    const fullUrl = `${window.location.origin}/s/sh_123/mobile`;
    expect(screen.getByText(fullUrl)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "查看详情：mobile" }));
    const details = screen.getByRole("dialog", { name: "分享详情" });
    await user.click(within(details).getByRole("button", { name: "复制链接" }));

    expect(writeText).toHaveBeenCalledWith(fullUrl);
  });

  it("creates a subscription share after navigating from the list to its editor", async () => {
    const user = userEvent.setup();
    window.history.replaceState({}, "", "/subscriptions");
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.endsWith("/v1/shares") && init?.method === "POST") {
        return jsonResponse({
          share: {
            id: "sh_provider",
            name: "provider",
            target_kind: "subscription",
            target_name: "provider",
            target_format: "base64",
            public_filename: "provider.txt",
          },
        }, { status: 201 });
      }
      if (url.includes("/v1/subscriptions/provider")) {
        return jsonResponse(remoteSubscriptionDefinition);
      }
      if (url === "/v1/capabilities/ui") {
        return jsonResponse(uiCapabilities);
      }
      return jsonResponse({ ok: true }, { status: init?.method === "POST" ? 201 : 200 });
    }));
    const { router } = renderApp("/subscriptions");

    await screen.findByRole("heading", { name: "我的订阅" });
    await user.click(screen.getByRole("button", { name: "编辑：provider" }));
    expect(await screen.findByRole("heading", { name: "编辑订阅" })).toBeInTheDocument();
    expect(await screen.findByRole("group", { name: "处理器 入口重命名" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/subscriptions/remote/provider/edit");

    await user.click(screen.getByRole("button", { name: "分享订阅" }));
    const sheet = screen.getByRole("dialog", { name: "创建分享链接" });
    expect(within(sheet).getByRole("textbox", { name: "名称" })).toHaveValue("provider");
    expect(within(sheet).getByRole("textbox", { name: "分享目标" })).toHaveValue("provider");
    const formatSelect = within(sheet).getByRole("combobox", { name: "默认输出格式" });
    expect(formatSelect).toHaveValue("base64");
    expect(within(formatSelect).getAllByRole("option").map((option) => option.getAttribute("value"))).toEqual([
      "base64",
      "uri-list",
      "mihomo-proxies",
      "sing-box-outbounds",
      "shadowrocket-proxies",
    ]);
    expect(within(formatSelect).queryByRole("option", { name: "json-nodes" })).not.toBeInTheDocument();
    await user.click(within(sheet).getByRole("button", { name: "保存分享链接" }));

    const post = requests.find((request) => request.url.endsWith("/v1/shares") && request.init?.method === "POST");
    expect(post).toBeDefined();
    const body = JSON.parse(String(post?.init?.body));
    expect(body).toMatchObject({
      name: "provider",
      target_kind: "subscription",
      target_name: "provider",
      target_format: "base64",
      meta: { ui: "web" },
    });
    expect(window.location.pathname).toBe("/subscriptions");
    expect(router.state.location.pathname).toBe("/subscriptions/remote/provider/edit");
    const result = await screen.findByRole("dialog", { name: "分享链接已创建" });
    expect(within(result).getByText(
      `${window.location.origin}/s/sh_provider/provider.txt?format=base64`,
    )).toBeInTheDocument();
  });

  it("opens a prefilled share sheet from file list actions", async () => {
    const user = userEvent.setup();
    window.history.replaceState({}, "", "/files");
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.endsWith("/v1/shares") && init?.method === "POST") {
        return jsonResponse({
          share: {
            id: "sh_default",
            name: "default.yaml",
            target_kind: "file",
            target_name: "default.yaml",
            public_filename: "default.yaml",
          },
        }, { status: 201 });
      }
      return jsonResponse({ ok: true }, { status: init?.method === "POST" ? 201 : 200 });
    }));
    const { router } = renderApp("/files");

    await screen.findByRole("heading", { name: "我的文件" });
    await user.click(screen.getByRole("button", { name: "default.yaml 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "分享文件" }));
    const sheet = screen.getByRole("dialog", { name: "创建分享链接" });
    expect(within(sheet).getByRole("textbox", { name: "名称" })).toHaveValue("default.yaml");
    expect(within(sheet).getByRole("textbox", { name: "分享目标" })).toHaveValue("default.yaml");
    expect(within(sheet).queryByRole("combobox", { name: "默认输出格式" })).not.toBeInTheDocument();
    expect(within(sheet).queryByText("此分享支持多种格式，这里只设置默认格式。")).not.toBeInTheDocument();

    await user.click(within(sheet).getByRole("button", { name: "1天" }));
    await user.click(within(sheet).getByRole("button", { name: "保存分享链接" }));

    const post = requests.find((request) => request.url.endsWith("/v1/shares") && request.init?.method === "POST");
    expect(post).toBeDefined();
    const body = JSON.parse(String(post?.init?.body));
    expect(body).toMatchObject({
      name: "default.yaml",
      target_kind: "file",
      target_name: "default.yaml",
      content_type: "application/octet-stream",
      meta: { ui: "web" },
    });
    expect(body).not.toHaveProperty("target_format");
    expect(Date.parse(String(body.valid_until))).toBeGreaterThan(Date.now());
    expect(window.location.pathname).toBe("/files");
    expect(router.state.location.pathname).toBe("/files");
    const result = await screen.findByRole("dialog", { name: "分享链接已创建" });
    expect(within(result).getByText(
      `${window.location.origin}/s/sh_default/default.yaml`,
    )).toBeInTheDocument();
  });

});
