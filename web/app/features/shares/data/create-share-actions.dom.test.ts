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
    const { client, createShare, showNotice } = setupActions();
    client.createShare.mockResolvedValue({
      share: {
        id: "sh_new",
        name: "mobile",
        target_kind: "subscription",
        target_name: "provider",
        target_format: "mihomo-proxies",
        public_filename: "mobile.yaml",
      },
    });
    const form = new FormData();
    form.set("name", "mobile");
    form.set("target_kind", "subscription");
    form.set("target", "provider");
    form.set("target_format", "mihomo-proxies");
    form.set("valid_from", "2026-06-21T10:00");
    form.set("valid_until", "2026-07-01T10:00");
	form.set("age_recipient", "age1example");
	form.set("max_uses", "5");

    const created = await createShare(form);

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
    expect(created).toEqual(expect.objectContaining({
      id: "sh_new",
      publicUrl: "https://public.example/s/sh_new/mobile.yaml?format=mihomo-proxies",
    }));
    expect(showNotice).not.toHaveBeenCalledWith("分享链接已创建");
  });

  it("shows a subscription-specific notice when no subscription target exists", async () => {
    const { client, createShare, showNotice } = setupActions();
    const form = new FormData();
    form.set("target_kind", "subscription");

    const created = await createShare(form);

    expect(created).toBeNull();
    expect(client.createShare).not.toHaveBeenCalled();
    expect(showNotice).toHaveBeenCalledWith("需要先创建一个订阅", "warning");
  });

  it("defaults a new subscription share to Base64", async () => {
    const { client, createShare } = setupActions();

    await createShare(validShareForm());

    expect(client.createShare).toHaveBeenCalledWith(expect.objectContaining({
      target_kind: "subscription",
      target_name: "provider",
      target_format: "base64",
    }));
  });

  it("propagates API errors without showing a success notice", async () => {
    const { client, createShare, showNotice } = setupActions();
    client.createShare.mockRejectedValue(new Error("create failed"));

    await expect(createShare(validShareForm())).rejects.toThrow("create failed");

    expect(showNotice).not.toHaveBeenCalled();
  });

  it("rejects a create response missing presentation fields", async () => {
    const { client, createShare } = setupActions();
    client.createShare.mockResolvedValue({ share: { id: "sh_new" } });

    await expect(createShare(validShareForm())).rejects.toThrow("Invalid create share response");
  });

  it("copies the stored default public URL unchanged", async () => {
    const { copyShare, showNotice } = setupActions();
    const writeText = installClipboardMock();
    const item = shareItem("/s/share?format=json-nodes#install");

    const copied = await copyShare(item);

    expect(copied).toEqual({ copied: true });
    expect(writeText).toHaveBeenCalledWith("/s/share?format=json-nodes#install");
    expect(showNotice).toHaveBeenCalledWith("已复制链接");
  });

  it("returns the exact URL and warns when the clipboard is unavailable", async () => {
    const { copyShare, showNotice } = setupActions();

    const copied = await copyShare(shareItem("http://example.test/s/share"));

    expect(copied).toEqual({
      copied: false,
      url: "http://example.test/s/share",
    });
    expect(showNotice).toHaveBeenCalledWith("无法自动复制，请手动复制链接", "warning");
  });

  it("returns the exact URL and warns when the clipboard rejects the write", async () => {
    const { copyShare, showNotice } = setupActions();
    const writeText = installClipboardMock();
    writeText.mockRejectedValue(new Error("clipboard denied"));

    const copied = await copyShare(shareItem("http://example.test/s/share"));

    expect(copied).toEqual({
      copied: false,
      url: "http://example.test/s/share",
    });
    expect(showNotice).toHaveBeenCalledWith("无法自动复制，请手动复制链接", "warning");
  });

  it("copies an explicitly selected format URL", async () => {
    const { copyShare } = setupActions();
    const writeText = installClipboardMock();
    const item = shareItem("https://example.com/s/share?token=abc&format=json-nodes#install");

    await copyShare(item, "sing-box-outbounds");

    expect(writeText).toHaveBeenCalledWith(
      "https://example.com/s/share/mobile.json?token=abc&format=sing-box-outbounds#install",
    );
  });

  it("retries an already-derived URL without changing it", async () => {
    const { copyShareUrl } = setupActions();
    const writeText = installClipboardMock();
    const url = "https://example.com/s/share/mobile.json?token=abc&format=sing-box-outbounds#install";

    const copied = await copyShareUrl(url);

    expect(copied).toEqual({ copied: true });
    expect(writeText).toHaveBeenCalledWith(url);
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
    formatFilenames: {
      "sing-box-outbounds": "mobile.json",
    },
    title: "mobile",
  };
}

function validShareForm(): FormData {
  const form = new FormData();
  form.set("target_kind", "subscription");
  form.set("target", "provider");
  return form;
}

function setupActions() {
  const client = {
    createShare: vi.fn(async () => ({
      share: {
        id: "sh_new",
        name: "provider",
        target_kind: "subscription",
        target_name: "provider",
        target_format: "uri-list",
        public_filename: "provider.txt",
      },
    })),
  } as unknown as ApiClient;
  const showNotice = vi.fn();
  return {
    client: client as ApiClient & { createShare: ReturnType<typeof vi.fn> },
    showNotice,
    ...createShareActions({
      client,
      publicBaseUrl: "https://public.example",
      showNotice,
      t: createTranslator("zh-CN"),
    }),
  };
}
