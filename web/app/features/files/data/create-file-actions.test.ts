import { afterEach, describe, expect, it, vi } from "vitest";

import type { FileDetail, FileItem } from "~/features/files/model/types";
import type { ApiClient } from "~/shared/api/client";

import { createFileActions } from "./create-file-actions";

describe("file actions", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("rejects empty file names before creating a file", async () => {
    const { client, createFile, navigate, refreshResources } = setupActions();
    const form = new FormData();

    await expect(createFile("static", form)).rejects.toThrow("file name is required");

    expect(client.createFile).not.toHaveBeenCalled();
    expect(refreshResources).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
  });

  it("creates files with frontend-owned created and updated timestamps", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-27T01:02:03.000Z"));
    const { client, createFile } = setupActions();
    const form = new FormData();
    form.set("name", "default.yaml");

    await createFile("static", form);

    expect(client.createFile).toHaveBeenCalledWith(expect.objectContaining({
      created_at: "2026-06-27T01:02:03.000Z",
      updated_at: "2026-06-27T01:02:03.000Z",
    }));
  });

  it("creates an exact minimal file payload", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-27T01:02:03.000Z"));
    const { client, createFile } = setupActions();
    const form = new FormData();
    form.set("name", "default.yaml");

    await createFile("static", form);

    expect(client.createFile).toHaveBeenCalledWith({
      name: "default.yaml",
      display_name: undefined,
      created_at: "2026-06-27T01:02:03.000Z",
      updated_at: "2026-06-27T01:02:03.000Z",
      kind: "static",
      source: { type: "inline", content: "" },
      config: undefined,
      processors: [],
      meta: { ui: "web" },
    });
    expect(Object.keys(client.createFile.mock.calls[0]?.[0] ?? {})).toEqual([
      "name",
      "display_name",
      "created_at",
      "updated_at",
      "kind",
      "source",
      "config",
      "processors",
      "meta",
    ]);
  });

  it("awaits create API and refresh completion before later effects", async () => {
    const api = deferred<unknown>();
    const refresh = deferred<void>();
    const events: string[] = [];
    const { client, closeSheet, createFile, navigate, refreshResources, showNotice } = setupActions();
    client.createFile.mockImplementation(async () => {
      events.push("api:start");
      await api.promise;
      events.push("api:done");
      return {};
    });
    refreshResources.mockImplementation(async () => {
      events.push("refresh:start");
      await refresh.promise;
      events.push("refresh:done");
    });
    closeSheet.mockImplementation(() => events.push("close"));
    showNotice.mockImplementation(() => events.push("notice"));
    navigate.mockImplementation(() => events.push("navigate"));

    const pending = createFile("static", validCreateForm());
    expect(events).toEqual(["api:start"]);
    expect(refreshResources).not.toHaveBeenCalled();
    expect(closeSheet).not.toHaveBeenCalled();
    expect(showNotice).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();

    api.resolve({});
    await vi.waitFor(() => expect(refreshResources).toHaveBeenCalledTimes(1));
    expect(events).toEqual(["api:start", "api:done", "refresh:start"]);
    expect(closeSheet).not.toHaveBeenCalled();
    expect(showNotice).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();

    refresh.resolve();
    await pending;
    expect(events).toEqual([
      "api:start",
      "api:done",
      "refresh:start",
      "refresh:done",
      "close",
      "notice",
      "navigate",
    ]);
  });

  it("stops create effects when the API rejects", async () => {
    const { client, closeSheet, createFile, navigate, refreshResources, showNotice } = setupActions();
    client.createFile.mockRejectedValue(new Error("create failed"));

    await expect(createFile("static", validCreateForm())).rejects.toThrow("create failed");

    expect(refreshResources).not.toHaveBeenCalled();
    expect(closeSheet).not.toHaveBeenCalled();
    expect(showNotice).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
  });

  it("stops create effects after a rejected refresh", async () => {
    const { closeSheet, createFile, navigate, refreshResources, showNotice } = setupActions();
    refreshResources.mockRejectedValue(new Error("refresh failed"));

    await expect(createFile("static", validCreateForm())).rejects.toThrow("refresh failed");

    expect(closeSheet).not.toHaveBeenCalled();
    expect(showNotice).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
  });

  it("uses the explicit create kind and ignores a submitted kind field", async () => {
    const { client, createFile } = setupActions();
    const form = new FormData();
    form.set("name", "default.yaml");
    form.set("kind", "static");

    await createFile("mihomo", form);

    expect(client.createFile).toHaveBeenCalledWith(expect.objectContaining({
      kind: "mihomo",
    }));
  });

  it("creates files with display name and multiline description payload", async () => {
    const { client, createFile } = setupActions();
    const form = new FormData();
    form.set("name", "default.yaml");
    form.set("display_name", "  Mobile Config  ");
    form.set("description", "  main config\nfor mobile clients  ");

    await createFile("static", form);

    expect(client.createFile).toHaveBeenCalledWith(expect.objectContaining({
      display_name: "Mobile Config",
      meta: {
        description: "main config\nfor mobile clients",
        ui: "web",
      },
    }));
  });

  it("preserves the existing file name and current route when saving edits", async () => {
    const { client, navigate, refreshResources, saveFileEdit } = setupActions();
    const item: FileItem = { name: "default.yaml", title: "default.yaml", kind: "static" };
    const form = new FormData();
    form.set("name", "renamed.yaml");
    form.set("kind", "sing-box");
    form.set("source", JSON.stringify({ type: "inline", content: "body" }));

    await saveFileEdit(item, form);

    expect(client.createFile).toHaveBeenCalledWith(expect.objectContaining({ kind: "static", name: "default.yaml", source: { type: "inline", content: "body" } }));
    expect(refreshResources).toHaveBeenCalledWith();
    expect(navigate).not.toHaveBeenCalled();
  });

  it("does not fall back to the summary kind when the detail kind is missing", async () => {
    const { client, saveFileEdit } = setupActions();
    const item: FileItem = { name: "default.yaml", title: "default.yaml", kind: "static" };
    const detail: FileDetail = {
      name: "default.yaml",
      kind: "",
      source: { type: "inline", content: "old" },
      processors: [],
      rawSpec: {},
    };

    await expect(saveFileEdit(item, new FormData(), detail)).rejects.toThrow("unregistered file kind");
    expect(client.createFile).not.toHaveBeenCalled();
  });

  it("preserves created_at and refreshes updated_at when saving file edits", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-27T04:05:06.000Z"));
    const { client, saveFileEdit } = setupActions();
    const item: FileItem = { name: "default.yaml", title: "default.yaml", kind: "static" };
    const form = new FormData();
    form.set("source", JSON.stringify({ type: "inline", content: "body" }));

    await saveFileEdit(item, form, {
      name: "default.yaml",
      kind: "static",
      source: { type: "inline", content: "old" },
      processors: [],
      createdAt: "2026-06-27T01:02:03.000Z",
      updatedAt: "2026-06-27T02:03:04.000Z",
      rawSpec: {},
    });

    expect(client.createFile).toHaveBeenCalledWith(expect.objectContaining({
      created_at: "2026-06-27T01:02:03.000Z",
      updated_at: "2026-06-27T04:05:06.000Z",
    }));
  });

  it("saves an exact minimal file edit payload", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-27T04:05:06.000Z"));
    const { client, saveFileEdit } = setupActions();
    const item: FileItem = { name: "default.yaml", title: "default.yaml", kind: "static" };

    await saveFileEdit(item, new FormData(), {
      name: "default.yaml",
      kind: "static",
      source: { type: "inline", content: "old" },
      processors: [],
      createdAt: "2026-06-27T01:02:03.000Z",
      rawSpec: {},
    });

    expect(client.createFile).toHaveBeenCalledWith({
      name: "default.yaml",
      display_name: undefined,
      created_at: "2026-06-27T01:02:03.000Z",
      updated_at: "2026-06-27T04:05:06.000Z",
      kind: "static",
      source: { type: "inline", content: "" },
      config: undefined,
      processors: [],
      meta: { ui: "web" },
    });
    expect(Object.keys(client.createFile.mock.calls[0]?.[0] ?? {})).toEqual([
      "name",
      "display_name",
      "created_at",
      "updated_at",
      "kind",
      "source",
      "config",
      "processors",
      "meta",
    ]);
  });

  it("awaits edit API and refresh completion before notifying", async () => {
    const api = deferred<unknown>();
    const refresh = deferred<void>();
    const events: string[] = [];
    const { client, refreshResources, saveFileEdit, showNotice } = setupActions();
    const item: FileItem = { name: "default.yaml", title: "default.yaml", kind: "static" };
    client.createFile.mockImplementation(async () => {
      events.push("api:start");
      await api.promise;
      events.push("api:done");
      return {};
    });
    refreshResources.mockImplementation(async () => {
      events.push("refresh:start");
      await refresh.promise;
      events.push("refresh:done");
    });
    showNotice.mockImplementation(() => events.push("notice"));

    const pending = saveFileEdit(item, new FormData());
    expect(events).toEqual(["api:start"]);
    expect(refreshResources).not.toHaveBeenCalled();
    expect(showNotice).not.toHaveBeenCalled();

    api.resolve({});
    await vi.waitFor(() => expect(refreshResources).toHaveBeenCalledTimes(1));
    expect(events).toEqual(["api:start", "api:done", "refresh:start"]);
    expect(showNotice).not.toHaveBeenCalled();

    refresh.resolve();
    await pending;

    expect(events).toEqual(["api:start", "api:done", "refresh:start", "refresh:done", "notice"]);
  });

  it("stops edit effects when the API rejects", async () => {
    const { client, refreshResources, saveFileEdit, showNotice } = setupActions();
    const item: FileItem = { name: "default.yaml", title: "default.yaml", kind: "static" };
    client.createFile.mockRejectedValue(new Error("edit failed"));

    await expect(saveFileEdit(item, new FormData())).rejects.toThrow("edit failed");

    expect(refreshResources).not.toHaveBeenCalled();
    expect(showNotice).not.toHaveBeenCalled();
  });

  it("stops edit effects after a rejected refresh", async () => {
    const { refreshResources, saveFileEdit, showNotice } = setupActions();
    const item: FileItem = { name: "default.yaml", title: "default.yaml", kind: "static" };
    refreshResources.mockRejectedValue(new Error("refresh failed"));

    await expect(saveFileEdit(item, new FormData())).rejects.toThrow("refresh failed");

    expect(showNotice).not.toHaveBeenCalled();
  });

  it("falls back to updated_at as created_at when saving older file edits", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-27T04:05:06.000Z"));
    const { client, saveFileEdit } = setupActions();
    const item: FileItem = { name: "default.yaml", title: "default.yaml", kind: "static" };
    const form = new FormData();
    form.set("source", JSON.stringify({ type: "inline", content: "body" }));

    await saveFileEdit(item, form, {
      name: "default.yaml",
      kind: "static",
      source: { type: "inline", content: "old" },
      processors: [],
      updatedAt: "2026-06-27T02:03:04.000Z",
      rawSpec: {},
    });

    expect(client.createFile).toHaveBeenCalledWith(expect.objectContaining({
      created_at: "2026-06-27T02:03:04.000Z",
      updated_at: "2026-06-27T04:05:06.000Z",
    }));
  });

  it("uses the submitted source JSON while keeping the edited file name", async () => {
    const { client, saveFileEdit } = setupActions();
    const item: FileItem = { name: "default.yaml", title: "default.yaml", kind: "static" };
    const form = new FormData();
    form.set("name", "renamed.yaml");
    form.set("source", JSON.stringify({ type: "remote", remote: { url: "https://example.com/base.yaml" } }));

    await saveFileEdit(item, form);

    expect(client.createFile).toHaveBeenCalledWith(expect.objectContaining({
      name: "default.yaml",
      source: { type: "remote", remote: { url: "https://example.com/base.yaml" } },
    }));
  });

  it("saves file edits with submitted display name", async () => {
    const { client, saveFileEdit } = setupActions();
    const item: FileItem = { name: "default.yaml", title: "Default Profile", displayName: "Default Profile", kind: "static" };
    const form = new FormData();
    form.set("display_name", "  Mobile Config  ");
    form.set("source", JSON.stringify({ type: "inline", content: "body" }));

    await saveFileEdit(item, form);

    expect(client.createFile).toHaveBeenCalledWith(expect.objectContaining({
      name: "default.yaml",
      display_name: "Mobile Config",
    }));
  });

  it("submits the typed settings envelope without legacy config fields", async () => {
    const { client, createFile } = setupActions();
    const form = new FormData();
    form.set("name", "default.yaml");
    form.set("config", JSON.stringify({ subscriptions: ["provider"], settings: { groups: [], rule_sets: [], rules: [] } }));

    await createFile("mihomo", form);

    expect(client.createFile).toHaveBeenCalledWith(expect.objectContaining({
      kind: "mihomo",
      config: { subscriptions: ["provider"], settings: { groups: [], rule_sets: [], rules: [] } },
    }));
  });

  it("does not POST edits that would truncate multiple subscriptions", async () => {
    const { client, saveFileEdit } = setupActions();
    const item: FileItem = { name: "client.yaml", title: "client.yaml", kind: "mihomo" };
    const detail: FileDetail = {
      name: "client.yaml", kind: "mihomo",
      source: { type: "inline", content: "" }, processors: [],
      config: { subscriptions: ["one", "two"], settingsPresent: false },
      rawSpec: {},
    };

    await expect(saveFileEdit(item, new FormData(), detail)).rejects.toThrow("multiple subscriptions");
    expect(client.createFile).not.toHaveBeenCalled();
  });

  it("does not POST creates with multiple subscriptions", async () => {
    const { client, createFile } = setupActions();
    const form = new FormData();
    form.set("name", "default.yaml");
    form.set("config", JSON.stringify({ subscriptions: ["one", "two"], settings: {} }));

    await expect(createFile("mihomo", form)).rejects.toThrow("multiple subscriptions");
    expect(client.createFile).not.toHaveBeenCalled();
  });

  it.each(["", "future-client"])("does not POST an unregistered '%s' file kind", async (kind) => {
    const { client, createFile, saveFileEdit } = setupActions();
    const createForm = new FormData();
    createForm.set("name", "future.json");
    const item: FileItem = { name: "future.json", title: "future.json", kind };

    await expect(createFile(kind, createForm)).rejects.toThrow("unregistered file kind");
    await expect(saveFileEdit(item, new FormData())).rejects.toThrow("unregistered file kind");
    expect(client.createFile).not.toHaveBeenCalled();
  });

});

function setupActions() {
  const events: string[] = [];
  const client = {
    createFile: vi.fn(async () => {
      events.push("api");
      return {};
    }),
  } as unknown as ApiClient;
  const closeSheet = vi.fn(() => {
    events.push("close");
  });
  const navigate = vi.fn(() => {
    events.push("navigate");
  });
  const refreshResources = vi.fn(async () => {
    events.push("refresh");
  });
  const showNotice = vi.fn(() => {
    events.push("notice");
  });
  return {
    client: client as ApiClient & { createFile: ReturnType<typeof vi.fn> },
    closeSheet,
    events,
    navigate,
    refreshResources,
    showNotice,
    ...createFileActions({
      client,
      closeSheet,
      navigate,
      refreshResources,
      showNotice,
    }),
  };
}

function validCreateForm(): FormData {
  const form = new FormData();
  form.set("name", "default.yaml");
  return form;
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, reject, resolve };
}
