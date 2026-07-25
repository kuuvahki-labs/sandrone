import { act, screen, waitFor, within } from "@testing-library/react";
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

  it("loads only settings resources on the settings route", async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true });
    }));

    renderApp("/settings");

    expect(await screen.findByRole("heading", { name: "设置" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Public Base URL" })).toHaveValue(window.location.origin);
    expect(resourceListUrls(requests)).toEqual([]);
  });

  it("loads only file resources on the files route", async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true });
    }));

    renderApp("/files");

    expect(await screen.findByRole("heading", { name: "我的文件" })).toBeInTheDocument();
    expect(resourceListUrls(requests)).toEqual(["/v1/files"]);
  });

  it("coalesces duplicate subscription list loads during one page entry", async () => {
    let resolveList: (() => void) | undefined;
    const listReady = new Promise<void>((resolve) => {
      resolveList = resolve;
    });
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      if (url.endsWith("/v1/subscriptions") && (init?.method ?? "GET") === "GET") {
        await listReady;
      }
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true });
    }));

    renderApp("/subscriptions", { strictMode: true });
    resolveList?.();

    expect(await screen.findByRole("heading", { name: "我的订阅" })).toBeInTheDocument();
    await waitFor(() => {
      expect(resourceListUrls(requests)).toEqual(["/v1/subscriptions"]);
    });
  });

  it("preserves root subscription requests across StrictMode alias navigation", async () => {
    let resolveList: (() => void) | undefined;
    const listReady = new Promise<void>((resolve) => {
      resolveList = resolve;
    });
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      if (url.endsWith("/v1/subscriptions") && (init?.method ?? "GET") === "GET") {
        await listReady;
      }
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.endsWith("/traffic")) {
        return jsonResponse({ subscription_name: url.split("/").at(-2), type: "remote" });
      }
      return jsonResponse({ ok: true });
    }));

    const { router } = renderApp("/", { strictMode: true });
    resolveList?.();

    expect(await screen.findByRole("heading", { name: "我的订阅" })).toBeInTheDocument();
    await waitFor(() => {
      expect(trafficRequestUrls(requests)).toEqual([
        "/v1/subscriptions/provider/traffic",
        "/v1/subscriptions/warn/traffic",
      ]);
    });
    expect(resourceListUrls(requests)).toEqual(["/v1/subscriptions"]);

    await act(async () => {
      await router.navigate("/subscriptions");
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(router.state.location.pathname).toBe("/subscriptions");
    expect(screen.getByRole("heading", { name: "我的订阅" })).toBeInTheDocument();
    expect(resourceListUrls(requests)).toEqual(["/v1/subscriptions"]);
    expect(trafficRequestUrls(requests)).toEqual([
      "/v1/subscriptions/provider/traffic",
      "/v1/subscriptions/warn/traffic",
    ]);
  });

  it("coalesces duplicate file list loads during one page entry", async () => {
    let resolveList: (() => void) | undefined;
    const listReady = new Promise<void>((resolve) => {
      resolveList = resolve;
    });
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      if (url.endsWith("/v1/files") && (init?.method ?? "GET") === "GET") {
        await listReady;
      }
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true });
    }));

    renderApp("/files", { strictMode: true });
    resolveList?.();

    expect(await screen.findByRole("heading", { name: "我的文件" })).toBeInTheDocument();
    await waitFor(() => {
      expect(resourceListUrls(requests)).toEqual(["/v1/files"]);
    });
  });

  it("loads only share resources on the shares route", async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      return jsonResponse({ ok: true });
    }));

    renderApp("/shares");

    expect(await screen.findByRole("heading", { level: 2, name: "分享" })).toBeInTheDocument();
    expect(resourceListUrls(requests)).toEqual(["/v1/shares"]);
  });

  it("loads subscriptions and files on subscription editor routes", async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, init });
      const resourceResponse = resourceListResponse(url, resources, init);
      if (resourceResponse) return resourceResponse;
      if (url.includes("/v1/subscriptions/provider")) return jsonResponse(remoteSubscriptionDefinition);
      return jsonResponse({ ok: true });
    }));

    renderApp("/subscriptions/remote/provider/edit");

    expect(await screen.findByRole("textbox", { name: "订阅地址" })).toHaveValue("https://example.com/sub");
    expect(resourceListUrls(requests)).toEqual(["/v1/subscriptions", "/v1/files"]);
  });

  it("shows the token form when the active route resource list is unauthorized", async () => {
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/v1/subscriptions")) {
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
    expect(screen.getByRole("img", { name: "Sandrone logo" })).toHaveAttribute("src", "/brand/sandrone-logo-64.png");
    expect(screen.getByLabelText("管理员 token")).toBeInTheDocument();
    expect(screen.getByLabelText("认证品牌")).toHaveClass("justify-items-center");
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

  it("renders the token form in English when the locale is en-US", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/v1/subscriptions")) {
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

    expect(await screen.findByRole("heading", { name: "Authentication required" })).toBeInTheDocument();
    expect(screen.getByLabelText("Admin token")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Enter" })).toBeInTheDocument();
  });

  it("shows the Sandrone logo while resource lists are loading", async () => {
    let resolveResponse: ((response: Response) => void) | undefined;
    const responseReady = new Promise<Response>((resolve) => {
      resolveResponse = resolve;
    });
    vi.stubGlobal("fetch", vi.fn(() => responseReady));

    renderApp("/subscriptions");

    const loadingLogo = screen.getAllByRole("img", { name: "Sandrone logo" }).find((image) => image.getAttribute("src") === "/brand/sandrone-logo-64.png");
    expect(loadingLogo).toBeTruthy();
    expect(loadingLogo?.parentElement).toHaveClass("justify-items-center");
    expect(screen.getByRole("heading", { name: "正在连接 Sandrone" })).toBeInTheDocument();
    resolveResponse?.(jsonResponse({ items: resources.subscriptions }));
    expect(await screen.findByRole("heading", { name: "我的订阅" })).toBeInTheDocument();
  });

  it("focuses the admin token input when the token form opens", async () => {
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/v1/subscriptions")) {
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

    const tokenInput = await screen.findByLabelText("管理员 token");
    await waitFor(() => expect(tokenInput).toHaveFocus());
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

    await user.type(await screen.findByLabelText("管理员 token"), "secret");
    await user.click(screen.getByRole("button", { name: "进入" }));

    expect(await screen.findByRole("heading", { name: "我的订阅" })).toBeInTheDocument();
    const authorizedListRequest = requests.find((request) => request.url.endsWith("/v1/subscriptions") && asHeaders(request.init?.headers).Authorization === "Bearer secret");
    expect(authorizedListRequest).toBeDefined();
  });

  it("reloads resources when confirming the admin token with Enter", async () => {
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

    await user.type(await screen.findByLabelText("管理员 token"), "secret");
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

function trafficRequestUrls(requests: Array<{ url: string; init?: RequestInit }>): string[] {
  return requests
    .filter((request) => request.init?.method === "POST" && request.url.endsWith("/traffic"))
    .map((request) => request.url);
}
