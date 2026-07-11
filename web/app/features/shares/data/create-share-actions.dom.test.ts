import { afterEach, describe, expect, it, vi } from "vitest";

import type { ShareItem } from "~/features/shares/model/types";
import type { ApiClient } from "~/shared/api/client";
import { createTranslator } from "~/shared/i18n/context";

import { createShareActions } from "./create-share-actions";

describe("share actions", () => {
  let originalClipboardDescriptor: PropertyDescriptor | undefined;

  afterEach(() => {
    if (originalClipboardDescriptor) {
      Object.defineProperty(navigator, "clipboard", originalClipboardDescriptor);
    } else {
      Reflect.deleteProperty(navigator, "clipboard");
    }
    originalClipboardDescriptor = undefined;
  });

  it("creates subscription shares with target format and valid range", async () => {
    const { client, closeSheet, createShare, onShareCreated, showNotice } = setupActions();
    const form = new FormData();
    form.set("name", "mobile");
    form.set("target_kind", "subscription");
    form.set("target", "provider");
    form.set("target_format", "mihomo-proxies");
    form.set("valid_from", "2026-06-21T10:00");
    form.set("valid_until", "2026-07-01T10:00");
	form.set("age_recipient", "age1example");
	form.set("max_uses", "5");

    await createShare(form);

    expect(client.createShare).toHaveBeenCalledWith({
      name: "mobile",
      target_kind: "subscription",
      target_name: "provider",
      target_format: "mihomo-proxies",
      valid_from: new Date("2026-06-21T10:00").toISOString(),
      valid_until: new Date("2026-07-01T10:00").toISOString(),
	  age_recipient: "age1example",
	  max_uses: 5,
      meta: { ui: "web" },
    });
    expect(onShareCreated).toHaveBeenCalledWith();
    expect(closeSheet).toHaveBeenCalledTimes(1);
    expect(showNotice).toHaveBeenCalledWith("分享链接已创建");
  });

  it("awaits the API and refresh callback before closing and notifying", async () => {
    const api = deferred<unknown>();
    const refreshed = deferred<void>();
    const events: string[] = [];
    const { client, closeSheet, createShare, onShareCreated, showNotice } = setupActions();
    client.createShare.mockImplementation(async () => {
      events.push("api:start");
      await api.promise;
      events.push("api:done");
      return {};
    });
    onShareCreated.mockImplementation(async () => {
      events.push("refresh:start");
      await refreshed.promise;
      events.push("refresh:done");
    });
    closeSheet.mockImplementation(() => events.push("close"));
    showNotice.mockImplementation(() => events.push("notice"));

    const pending = createShare(validShareForm());
    expect(events).toEqual(["api:start"]);
    expect(onShareCreated).not.toHaveBeenCalled();
    expect(closeSheet).not.toHaveBeenCalled();
    expect(showNotice).not.toHaveBeenCalled();

    api.resolve({});
    await vi.waitFor(() => expect(onShareCreated).toHaveBeenCalledTimes(1));
    expect(events).toEqual(["api:start", "api:done", "refresh:start"]);
    expect(closeSheet).not.toHaveBeenCalled();
    expect(showNotice).not.toHaveBeenCalled();

    refreshed.resolve();
    await pending;
    expect(events).toEqual([
      "api:start",
      "api:done",
      "refresh:start",
      "refresh:done",
      "close",
      "notice",
    ]);
  });

  it("shows a subscription-specific notice when no subscription target exists", async () => {
    const { client, closeSheet, createShare, onShareCreated, showNotice } = setupActions();
    const form = new FormData();
    form.set("target_kind", "subscription");

    await createShare(form);

    expect(client.createShare).not.toHaveBeenCalled();
    expect(onShareCreated).not.toHaveBeenCalled();
    expect(closeSheet).not.toHaveBeenCalled();
    expect(showNotice).toHaveBeenCalledWith("需要先创建一个订阅", "warning");
  });

  it("stops before refresh, close, and notice when the API rejects", async () => {
    const { client, closeSheet, createShare, onShareCreated, showNotice } = setupActions();
    client.createShare.mockRejectedValue(new Error("create failed"));

    await expect(createShare(validShareForm())).rejects.toThrow("create failed");

    expect(onShareCreated).not.toHaveBeenCalled();
    expect(closeSheet).not.toHaveBeenCalled();
    expect(showNotice).not.toHaveBeenCalled();
  });

  it("stops before close and notice when the refresh callback rejects", async () => {
    const { closeSheet, createShare, onShareCreated, showNotice } = setupActions();
    onShareCreated.mockRejectedValue(new Error("refresh failed"));

    await expect(createShare(validShareForm())).rejects.toThrow("refresh failed");

    expect(closeSheet).not.toHaveBeenCalled();
    expect(showNotice).not.toHaveBeenCalled();
  });

  it("copies the stored default public URL unchanged", async () => {
    const { copyShare, showNotice } = setupActions();
    const writeText = installClipboardMock();
    const item = shareItem("/s/share?format=json-nodes#install");

    await copyShare(item);

    expect(writeText).toHaveBeenCalledWith("/s/share?format=json-nodes#install");
    expect(showNotice).toHaveBeenCalledWith("已复制链接");
  });

  it("copies an explicitly selected format URL", async () => {
    const { copyShare } = setupActions();
    const writeText = installClipboardMock();
    const item = shareItem("https://example.com/s/share?token=abc&format=json-nodes#install");

    await copyShare(item, "sing-box-outbounds");

    expect(writeText).toHaveBeenCalledWith(
      "https://example.com/s/share?token=abc&format=sing-box-outbounds#install",
    );
  });

  function installClipboardMock() {
    const writeText = vi.fn(async () => undefined);
    originalClipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    return writeText;
  }
});

function shareItem(publicUrl: string): ShareItem {
  return {
    id: "share",
    publicUrl,
    status: "valid",
    targetKind: "subscription",
    targetName: "provider",
    targetFormat: "json-nodes",
    title: "mobile",
  };
}

function validShareForm(): FormData {
  const form = new FormData();
  form.set("target_kind", "subscription");
  form.set("target", "provider");
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

function setupActions() {
  const client = {
    createShare: vi.fn(async () => ({})),
  } as unknown as ApiClient;
  const closeSheet = vi.fn();
  const onShareCreated = vi.fn(async () => undefined);
  const showNotice = vi.fn();
  return {
    client: client as ApiClient & { createShare: ReturnType<typeof vi.fn> },
    closeSheet,
    onShareCreated,
    showNotice,
    ...createShareActions({
      client,
      closeSheet,
      onShareCreated,
      showNotice,
      t: createTranslator("zh-CN"),
    }),
  };
}
