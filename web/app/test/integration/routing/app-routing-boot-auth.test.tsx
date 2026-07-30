import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  asHeaders,
  installDefaultFetchMock,
  jsonResponse,
  remoteSubscriptionDefinition,
  renderApp,
  resourceListResponse,
  resources,
} from "./app-routing.test-data";

describe("React Router app boot and auth workflows", () => {
  beforeEach(installDefaultFetchMock);

  it("loads global settings and version information on the settings overview route", async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url === "/version") {
        return jsonResponse({ name: "sandrone", version: "0.1.0", revision: "0123456789abcdef" });
      }
      return jsonResponse({ ok: true });
    }));

    renderApp("/settings");

    expect(await screen.findByRole("heading", { name: "设置" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Public Base URL" })).toHaveValue(window.location.origin);
    expect(settingsRequestPaths(requests)).toEqual(expect.arrayContaining(["/v1/settings", "/version"]));
  });

  it("unmounts an open share dialog when creating the share returns unauthorized", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/v1/shares") && init?.method === "POST") {
        return jsonResponse(
          { error: { code: "unauthorized", message: "token required" } },
          { status: 401 },
        );
      }
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true });
    }));

    renderApp("/files");

    await screen.findByRole("heading", { name: "我的文件" });
    await user.click(screen.getByRole("button", { name: "default.yaml 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "分享" }));
    const shareDialog = screen.getByRole("dialog", { name: "创建分享链接" });
    await user.click(within(shareDialog).getByRole("button", { name: "保存分享链接" }));

    expect(await screen.findByRole("heading", { name: "需要认证" })).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("reloads resources and leaves the token form after entering a valid token", async () => {
    const user = userEvent.setup();
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      if (url.startsWith("/v1/") && asHeaders(init?.headers).Authorization !== "Bearer secret") {
        return jsonResponse(
          { error: { code: "unauthorized", message: "token required" } },
          { status: 401 },
        );
      }
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true });
    }));
    renderApp("/subscriptions");

    expect(await screen.findByRole("heading", { name: "需要认证" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "进入" })).toBeInTheDocument();
    const tokenInput = screen.getByLabelText("管理员 token");
    await waitFor(() => expect(tokenInput).toHaveFocus());
    await user.type(tokenInput, "secret");
    await user.keyboard("{Enter}");

    expect(await screen.findByRole("heading", { name: "我的订阅" })).toBeInTheDocument();
    const authorizedListRequest = requests.find((request) => request.url.endsWith("/v1/subscriptions") && asHeaders(request.init?.headers).Authorization === "Bearer secret");
    expect(authorizedListRequest).toBeDefined();
  });

  it("loads the stored token before the first resource list request", async () => {
    localStorage.setItem("sandrone.adminToken", "yang");
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

    expect(await screen.findByRole("textbox", { name: "订阅地址" })).toHaveValue("https://example.com/sub");
    expect(resourceListUrls(requests)).toEqual(["/v1/subscriptions", "/v1/files"]);
    const subscriptionRequests = requests.filter((request) => request.url.endsWith("/v1/subscriptions"));
    expect(subscriptionRequests.length).toBeGreaterThanOrEqual(1);
    expect(subscriptionRequests[0]?.init?.headers).toEqual({ Authorization: "Bearer yang" });
  });
});

function resourceListUrls(requests: Array<{ url: string; init?: RequestInit }>): string[] {
  return requests
    .filter((request) => (request.init?.method ?? "GET") === "GET")
    .map((request) => request.url)
    .filter((url) => [
      "/v1/subscriptions",
      "/v1/files",
      "/v1/shares",
    ].includes(url));
}

function settingsRequestPaths(requests: Array<{ url: string; init?: RequestInit }>): string[] {
  return requests
    .filter((request) => (request.init?.method ?? "GET") === "GET")
    .map((request) => request.url)
    .filter((url) => ["/version", "/v1/settings"].includes(url));
}
