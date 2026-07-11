import { screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { jsonResponse, renderApp, resourceListResponse, resources } from "./app-routing.test-data";

describe("React Router app resource sorting", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it("renders subscriptions newest first from unsorted API data", async () => {
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const response = resourceListResponse(url, {
        ...resources,
        subscriptions: [
          { name: "old", type: "remote", format: "uri-list", created_at: "2026-06-25T01:00:00.000Z" },
          { name: "fallback", type: "remote", format: "uri-list", updated_at: "2026-06-27T01:00:00.000Z" },
          { name: "new", type: "remote", format: "uri-list", created_at: "2026-06-26T01:00:00.000Z" },
        ],
      }, init);
      return response ?? jsonResponse({ ok: true });
    }));

    renderApp("/subscriptions");

    await screen.findByRole("heading", { name: "我的订阅" });
    expect(editButtonTitles("订阅列表")).toEqual(["fallback", "new", "old"]);
  });

  it("renders files newest first from unsorted API data", async () => {
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const response = resourceListResponse(url, {
        ...resources,
        files: [
          { name: "old.yaml", type: "remote", created_at: "2026-06-25T01:00:00.000Z" },
          { name: "fallback.yaml", type: "remote", updated_at: "2026-06-27T01:00:00.000Z" },
          { name: "new.yaml", type: "remote", created_at: "2026-06-26T01:00:00.000Z" },
        ],
      }, init);
      return response ?? jsonResponse({ ok: true });
    }));

    renderApp("/files");

    await screen.findByRole("heading", { name: "我的文件" });
    expect(editButtonTitles("文件列表")).toEqual(["fallback.yaml", "new.yaml", "old.yaml"]);
  });

});

function editButtonTitles(listName: string): string[] {
  return within(screen.getByRole("list", { name: listName }))
    .getAllByRole("button", { name: /^编辑：/ })
    .map((button) => button.getAttribute("aria-label")?.replace(/^编辑：/, "") ?? "");
}
