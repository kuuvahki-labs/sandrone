import { fireEvent, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  installDefaultFetchMock,
  jsonResponse,
  renderApp,
  resourceListResponse,
  resources,
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

  it("confirms before deleting a public share", async () => {
    const user = userEvent.setup();
    renderApp("/shares");

    await screen.findByRole("heading", { level: 2, name: "分享" });
    await user.click(screen.getByRole("button", { name: "mobile 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "删除" }));

    const dialog = screen.getByRole("dialog", { name: "删除“sh_123”？" });
    expect(within(dialog).getByRole("button", { name: "删除“sh_123”" })).toHaveTextContent("删除");
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

    await user.click(screen.getByRole("button", { name: "复制链接：mobile" }));

    expect(writeText).toHaveBeenCalledWith(fullUrl);
  });

  it("copies each client-oriented format from a subscription share menu", async () => {
    const user = userEvent.setup();
    originalClipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    const writeText = installClipboardMock();
    installShareResources([
      {
        id: "sh_nodes",
        name: "nodes",
        target_kind: "subscription",
        target_name: "provider",
        target_format: "uri-list",
        public_filename: "nodes.txt",
        format_filenames: {
          "base64": "nodes.txt",
          "uri-list": "nodes.txt",
          "mihomo-proxies": "nodes.yaml",
          "sing-box-outbounds": "nodes.json",
          "shadowrocket-proxies": "nodes.conf",
          "json-nodes": "nodes.json",
        },
      },
    ]);
    renderApp("/shares");

    await screen.findByRole("heading", { level: 2, name: "分享" });
    const more = screen.getByRole("button", { name: "nodes 更多操作" });
    const cases = [
      ["复制为通用订阅（Base64）", "base64", "nodes.txt"],
      ["复制为 URI list", "uri-list", "nodes.txt"],
      ["复制为 Mihomo", "mihomo-proxies", "nodes.yaml"],
      ["复制为 sing-box", "sing-box-outbounds", "nodes.json"],
      ["复制为 Shadowrocket", "shadowrocket-proxies", "nodes.conf"],
    ] as const;

    for (const [label, format, filename] of cases) {
      await user.click(more);
      await user.click(screen.getByRole("menuitem", { name: label }));
      expect(writeText).toHaveBeenLastCalledWith(
        `${window.location.origin}/s/sh_nodes/${filename}?format=${format}`,
      );
    }

    expect(writeText).toHaveBeenCalledTimes(5);
  });

  it("shows the attempted menu URL and closes after copy becomes available", async () => {
    const user = userEvent.setup();
    originalClipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: undefined,
    });
    installShareResources([
      {
        id: "sh_nodes",
        name: "nodes",
        target_kind: "subscription",
        target_name: "provider",
        target_format: "uri-list",
        public_filename: "nodes.txt",
        format_filenames: {
          "base64": "nodes.txt",
          "uri-list": "nodes.txt",
          "mihomo-proxies": "nodes.yaml",
        },
      },
    ]);
    renderApp("/shares");

    await screen.findByRole("heading", { level: 2, name: "分享" });
    await user.click(screen.getByRole("button", { name: "nodes 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "复制为通用订阅（Base64）" }));

    const attemptedUrl = `${window.location.origin}/s/sh_nodes/nodes.txt?format=base64`;
    const dialog = await screen.findByRole("dialog", { name: "请手动复制链接" });
    expect(within(dialog).getByText(attemptedUrl)).toBeInTheDocument();

    const writeText = installClipboardMock();
    await user.click(within(dialog).getByRole("button", { name: "重试复制" }));

    expect(writeText).toHaveBeenCalledWith(attemptedUrl);
    expect(screen.queryByRole("dialog", { name: "请手动复制链接" })).not.toBeInTheDocument();
  });

  it("keeps a historical JSON share as the primary URL without adding a JSON menu action", async () => {
    const user = userEvent.setup();
    originalClipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    const writeText = installClipboardMock();
    installShareResources([
      {
        id: "sh_json",
        name: "json",
        target_kind: "subscription",
        target_name: "provider",
        target_format: "json-nodes",
        public_filename: "json.json",
        format_filenames: {
          "base64": "json.txt",
          "uri-list": "json.txt",
          "mihomo-proxies": "json.yaml",
          "sing-box-outbounds": "json.json",
          "shadowrocket-proxies": "json.conf",
          "json-nodes": "json.json",
        },
      },
    ]);
    renderApp("/shares");

    await screen.findByRole("heading", { level: 2, name: "分享" });
    await user.click(screen.getByRole("button", { name: "复制链接：json" }));
    expect(writeText).toHaveBeenCalledWith(
      `${window.location.origin}/s/sh_json/json.json?format=json-nodes`,
    );

    await user.click(screen.getByRole("button", { name: "json 更多操作" }));
    expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual([
      "复制为通用订阅（Base64）",
      "复制为 URI list",
      "复制为 Mihomo",
      "复制为 sing-box",
      "复制为 Shadowrocket",
      "删除",
    ]);
    expect(screen.queryByRole("menuitem", { name: /json-nodes/i })).not.toBeInTheDocument();
  });

  it("opens a prefilled share sheet from subscription list actions", async () => {
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
            target_format: "mihomo-proxies",
            public_filename: "provider.yaml",
          },
        }, { status: 201 });
      }
      return jsonResponse({ ok: true }, { status: init?.method === "POST" ? 201 : 200 });
    }));
    const { router } = renderApp("/subscriptions");

    await screen.findByRole("heading", { name: "我的订阅" });
    await user.click(screen.getByRole("button", { name: "provider 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "分享" }));
    const sheet = screen.getByRole("dialog", { name: "创建分享链接" });
    expect(within(sheet).getByRole("textbox", { name: "名称" })).toHaveValue("provider");
    expect(within(sheet).getByRole("textbox", { name: "分享目标" })).toHaveValue("provider");
    expect(within(sheet).getByText("目标", { selector: "label" })).toBeInTheDocument();
    const ageRecipientInput = within(sheet).getByRole("textbox", { name: "age X25519 加密公钥" });
    fireEvent.change(ageRecipientInput, { target: { value: "age1example" } });
    const maxVisitsInput = within(sheet).getByRole("spinbutton", { name: "最大访问次数（0 为不限）" });
    fireEvent.change(maxVisitsInput, { target: { value: "2" } });

    expect(within(sheet).getByText("默认输出格式", { selector: "label" })).toBeInTheDocument();
    expect(within(sheet).getByText("此分享支持多种格式，这里只设置默认格式。")).toBeInTheDocument();
    const formatSelect = within(sheet).getByRole("combobox", { name: "默认输出格式" });
    expect(within(formatSelect).getAllByRole("option").map((option) => option.getAttribute("value"))).toEqual([
      "base64",
      "uri-list",
      "mihomo-proxies",
      "sing-box-outbounds",
      "shadowrocket-proxies",
    ]);
    expect(within(formatSelect).queryByRole("option", { name: "json-nodes" })).not.toBeInTheDocument();
    await user.selectOptions(formatSelect, "mihomo-proxies");
    const before = Date.now();
    await user.click(within(sheet).getByRole("button", { name: "7天" }));
    const saveShare = within(sheet).getByRole("button", { name: "保存分享链接" });
    expect(saveShare).toHaveTextContent("保存");
    await user.click(saveShare);

    const post = requests.find((request) => request.url.endsWith("/v1/shares") && request.init?.method === "POST");
    expect(post).toBeDefined();
    const body = JSON.parse(String(post?.init?.body));
    expect(body).toMatchObject({
      name: "provider",
      target_kind: "subscription",
      target_name: "provider",
      target_format: "mihomo-proxies",
	  age_recipient: "age1example",
	  max_uses: 2,
      meta: { ui: "web" },
    });
    expect(Date.parse(String(body.valid_until))).toBeGreaterThanOrEqual(before + 7 * 24 * 60 * 60 * 1000 - 60_000);
    expect(Date.parse(String(body.valid_until))).toBeLessThanOrEqual(Date.now() + 7 * 24 * 60 * 60 * 1000 + 60_000);
    expect(window.location.pathname).toBe("/subscriptions");
    expect(router.state.location.pathname).toBe("/subscriptions");
    const result = await screen.findByRole("dialog", { name: "分享链接已创建" });
    expect(within(result).getByText(
      `${window.location.origin}/s/sh_provider/provider.yaml?format=mihomo-proxies`,
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
    await user.click(screen.getByRole("menuitem", { name: "分享" }));
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
    expect(Date.parse(String(body.valid_until))).toBeGreaterThan(Date.now());
    expect(window.location.pathname).toBe("/files");
    expect(router.state.location.pathname).toBe("/files");
    const result = await screen.findByRole("dialog", { name: "分享链接已创建" });
    expect(within(result).getByText(
      `${window.location.origin}/s/sh_default/default.yaml`,
    )).toBeInTheDocument();
  });

});

function installClipboardMock() {
  const writeText = vi.fn(async () => undefined);
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText },
  });
  return writeText;
}

function installShareResources(shares: Array<Record<string, unknown>>) {
  vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const resourceResponse = resourceListResponse(url, { ...resources, shares }, init);
    if (resourceResponse) return resourceResponse;
    return jsonResponse({ ok: true });
  }));
}
