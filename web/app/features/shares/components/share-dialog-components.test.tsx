import { useState } from "react";
import { fireEvent, render, renderHook, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { CopyShareResult } from "~/features/shares/data/create-share-actions";
import type { ApiClient } from "~/shared/api/client";
import { I18nProvider } from "~/shared/i18n/context";

import { ManualCopyDialog } from "./manual-copy-dialog";
import { ShareDialogProvider, useShareDialog } from "./share-dialog-context";

const manualCopyUrl = "https://example.com/s/share/mobile.yaml?token=abc&format=mihomo-proxies#install";
let originalClipboardDescriptor: PropertyDescriptor | undefined;

beforeEach(() => {
  localStorage.clear();
  originalClipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
});

afterEach(() => {
  if (originalClipboardDescriptor) {
    Object.defineProperty(navigator, "clipboard", originalClipboardDescriptor);
  } else {
    Reflect.deleteProperty(navigator, "clipboard");
  }
  originalClipboardDescriptor = undefined;
  window.getSelection()?.removeAllRanges();
});

describe("share dialog context", () => {
  it("rejects consumers outside ShareDialogProvider", () => {
    expect(() => renderHook(() => useShareDialog())).toThrowError(
      "useShareDialog must be used inside ShareDialogProvider",
    );
  });

  it("keeps its private target behind a stable open and close command API", async () => {
    const user = userEvent.setup();
    const observe = vi.fn<(value: ReturnType<typeof useShareDialog>) => void>();
    const client = { createShare: vi.fn() } as unknown as ApiClient;

    render(
      <I18nProvider>
        <ShareDialogProvider client={client} publicBaseUrl="https://public.example" showNotice={vi.fn()}>
          <DialogHarness />
          <IdentityHarness observe={observe} />
        </ShareDialogProvider>
      </I18nProvider>,
    );

    const firstValue = observe.mock.calls[0]?.[0];
    expect(firstValue).toBeDefined();
    expect(observe).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("share-dialog-keys")).toHaveTextContent("close,open");
    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));

    const dialog = screen.getByRole("dialog", { name: "创建分享链接" });
    expect(within(dialog).getByRole("textbox", { name: "名称" })).toHaveValue("provider");
    expect(within(dialog).getByRole("textbox", { name: "分享目标" })).toHaveValue("provider");
    expect(within(dialog).getByRole("combobox", { name: "默认输出格式" })).toHaveValue("base64");
    expect(observe).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Read identity", hidden: true }));

    const secondValue = observe.mock.calls[1]?.[0];
    expect(secondValue).toBe(firstValue);
    expect(secondValue?.open).toBe(firstValue?.open);
    expect(secondValue?.close).toBe(firstValue?.close);

    await user.click(screen.getByRole("button", { name: "Close route share dialog", hidden: true }));
    expect(screen.queryByRole("dialog", { name: "创建分享链接" })).not.toBeInTheDocument();
    expect(observe).toHaveBeenCalledTimes(2);
    await user.click(screen.getByRole("button", { name: "Read identity" }));
    expect(observe.mock.calls[2]?.[0]).toBe(firstValue);
  });

  it("starts a fresh keyed form for every result lifecycle", async () => {
    const user = userEvent.setup();
    const client = {
      createShare: vi.fn()
        .mockResolvedValueOnce(createResponse)
        .mockResolvedValueOnce(createResponse),
    } as unknown as ApiClient;

    render(
      <I18nProvider>
        <ShareDialogProvider client={client} publicBaseUrl="https://public.example" showNotice={vi.fn()}>
          <DialogHarness />
        </ShareDialogProvider>
      </I18nProvider>,
    );

    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));
    await user.click(screen.getByRole("button", { name: "保存分享链接" }));

    const result = await screen.findByRole("dialog", { name: "分享链接已创建" });
    expect(within(result).getByText(createdPublicUrl)).toBeInTheDocument();
    expect(within(result).queryByRole("textbox", { name: "名称" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Open another share dialog", hidden: true }));

    const dialog = screen.getByRole("dialog", { name: "创建分享链接" });
    expect(within(dialog).getByRole("textbox", { name: "名称" })).toHaveValue("second.yaml");
    expect(within(dialog).getByRole("textbox", { name: "分享目标" })).toHaveValue("second.yaml");
    expect(within(dialog).queryByText(createdPublicUrl)).not.toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "保存分享链接" }));
    const secondResult = await screen.findByRole("dialog", { name: "分享链接已创建" });
    await user.click(within(secondResult).getByRole("button", { name: "完成" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));
    const reopened = screen.getByRole("dialog", { name: "创建分享链接" });
    expect(within(reopened).getByRole("textbox", { name: "名称" })).toHaveValue("provider");
    expect(within(reopened).getByRole("textbox", { name: "分享目标" })).toHaveValue("provider");
    expect(within(reopened).getByRole("combobox", { name: "默认输出格式" })).toHaveValue("base64");
    expect(within(reopened).queryByText(createdPublicUrl)).not.toBeInTheDocument();
  });

  it("locks every user close path while submitting, then preserves the form for a retry", async () => {
    const user = userEvent.setup();
    const pendingCreate = deferred<unknown>();
    const showNotice = vi.fn();
    const client = {
      createShare: vi.fn()
        .mockImplementationOnce(() => pendingCreate.promise)
        .mockResolvedValueOnce(createResponse),
    } as unknown as ApiClient;

    render(
      <I18nProvider>
        <ShareDialogProvider client={client} publicBaseUrl="https://public.example" showNotice={showNotice}>
          <DialogHarness />
        </ShareDialogProvider>
      </I18nProvider>,
    );

    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));
    const name = screen.getByRole("textbox", { name: "名称" });
    await user.clear(name);
    await user.type(name, "retry-mobile");
    await user.click(screen.getByRole("button", { name: "保存分享链接" }));

    expect(screen.getByRole("button", { name: "保存分享链接" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "保存分享链接" })).toHaveTextContent("保存中…");
    expect(screen.getByRole("button", { name: "取消" })).toBeDisabled();
    await user.keyboard("{Escape}");
    const dialogContainer = document.querySelector(".MuiDialog-container")!;
    fireEvent.mouseDown(dialogContainer);
    fireEvent.click(dialogContainer);
    expect(screen.getByRole("dialog", { name: "创建分享链接" })).toBeInTheDocument();

    pendingCreate.reject(new Error("share request failed"));
    expect(await screen.findByRole("alert")).toHaveTextContent("share request failed");
    expect(screen.getByRole("dialog", { name: "创建分享链接" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "名称" })).toHaveValue("retry-mobile");
    expect(showNotice).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "保存分享链接" }));
    expect(await screen.findByRole("dialog", { name: "分享链接已创建" })).toBeInTheDocument();
  });

  it("preserves link selections across clipboard success and fallback", async () => {
    const user = userEvent.setup();
    const writeText = installClipboardMock();
    const showNotice = vi.fn();
    renderDialog({ createShare: vi.fn().mockResolvedValue(createResponse), showNotice });

    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));
    await user.click(screen.getByRole("button", { name: "保存分享链接" }));
    const result = await screen.findByRole("dialog", { name: "分享链接已创建" });
    const publicUrl = within(result).getByText(createdPublicUrl);

    await user.click(publicUrl);
    expect(window.getSelection()?.toString()).toBe(createdPublicUrl);

    const range = document.createRange();
    range.setStart(publicUrl.firstChild!, 0);
    range.setEnd(publicUrl.firstChild!, 5);
    window.getSelection()?.removeAllRanges();
    window.getSelection()?.addRange(range);
    fireEvent.click(publicUrl);

    expect(window.getSelection()?.toString()).toBe("https");

    await user.click(within(result).getByRole("button", { name: "复制链接" }));
    expect(writeText).toHaveBeenCalledWith(createdPublicUrl);

    Reflect.deleteProperty(navigator, "clipboard");
    await user.click(within(result).getByRole("button", { name: "复制链接" }));
    expect(showNotice).toHaveBeenCalledWith("无法自动复制，请手动复制链接", "warning");
    expect(window.getSelection()?.toString()).toBe(createdPublicUrl);
  });

  function installClipboardMock() {
    const writeText = vi.fn(async () => undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    return writeText;
  }
});

const createResponse = {
  share: {
    id: "sh_new",
    name: "mobile",
    target_kind: "subscription",
    target_name: "provider",
    target_format: "uri-list",
    public_filename: "mobile.txt",
  },
};
const createdPublicUrl = "https://public.example/s/sh_new/mobile.txt?format=uri-list";

function renderDialog({
  createShare,
  showNotice,
}: {
  createShare: ReturnType<typeof vi.fn>;
  showNotice: (message: string, severity?: "success" | "error" | "warning") => void;
}) {
  const client = { createShare } as unknown as ApiClient;
  return render(
    <I18nProvider>
      <ShareDialogProvider client={client} publicBaseUrl="https://public.example" showNotice={showNotice}>
        <DialogHarness />
      </ShareDialogProvider>
    </I18nProvider>,
  );
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve;
    reject = promiseReject;
  });
  return { promise, reject, resolve };
}

function DialogHarness() {
  const shareDialog = useShareDialog();
  return (
    <>
      <output data-testid="share-dialog-keys">{Object.keys(shareDialog).sort().join(",")}</output>
      <button type="button" onClick={() => shareDialog.open({ kind: "subscription", name: "provider" })}>
        Open route share dialog
      </button>
      <button type="button" onClick={() => shareDialog.open({ kind: "file", name: "second.yaml" })}>
        Open another share dialog
      </button>
      <button type="button" onClick={shareDialog.close}>
        Close route share dialog
      </button>
    </>
  );
}

function IdentityHarness({
  observe,
}: {
  observe: (value: ReturnType<typeof useShareDialog>) => void;
}) {
  const shareDialog = useShareDialog();
  const [, setVersion] = useState(0);
  observe(shareDialog);
  return (
    <>
      <button type="button" onClick={() => shareDialog.open({ kind: "subscription", name: "provider" })}>
        Open identity dialog
      </button>
      <button type="button" onClick={shareDialog.close}>
        Close identity dialog
      </button>
      <button type="button" onClick={() => setVersion((version) => version + 1)}>
        Read identity
      </button>
    </>
  );
}

describe("manual copy dialog", () => {
  it("shows and selects the exact URL when opened", async () => {
    renderManualCopyDialog();

    expect(screen.getByRole("dialog", { name: "请手动复制链接" })).toBeInTheDocument();
    expect(screen.getByText(manualCopyUrl)).toBeInTheDocument();
    await waitFor(() => expect(window.getSelection()?.toString()).toBe(manualCopyUrl));
  });

  it("closes after a successful retry", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onRetry = vi.fn().mockResolvedValue({ copied: true });
    renderManualCopyDialog({ onClose, onRetry });

    await user.click(screen.getByRole("button", { name: "重试复制" }));

    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("stays open and reselects the URL after a failed retry", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onRetry = vi.fn().mockResolvedValue({ copied: false, url: manualCopyUrl });
    renderManualCopyDialog({ onClose, onRetry });
    window.getSelection()?.removeAllRanges();

    await user.click(screen.getByRole("button", { name: "重试复制" }));

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "请手动复制链接" })).toBeInTheDocument();
    expect(window.getSelection()?.toString()).toBe(manualCopyUrl);
  });

  it("closes from Done, Escape, and the backdrop", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const { rerender } = renderManualCopyDialog({ onClose });

    await user.click(screen.getByRole("button", { name: "完成" }));
    expect(onClose).toHaveBeenCalledTimes(1);

    rerender(manualCopyDialogElement(onClose));
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledTimes(2);

    rerender(manualCopyDialogElement(onClose));
    const dialogContainer = document.querySelector(".MuiDialog-container")!;
    fireEvent.mouseDown(dialogContainer);
    fireEvent.click(dialogContainer);
    expect(onClose).toHaveBeenCalledTimes(3);
  });

  it("does not replace a partial selection when the URL is clicked", () => {
    renderManualCopyDialog();
    const link = screen.getByText(manualCopyUrl);
    const range = document.createRange();
    range.setStart(link.firstChild!, 0);
    range.setEnd(link.firstChild!, 5);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);

    fireEvent.click(link);

    expect(selection?.toString()).toBe("https");
  });
});

function renderManualCopyDialog({
  onClose = vi.fn(),
  onRetry = vi.fn().mockResolvedValue({ copied: true }),
}: {
  onClose?: () => void;
  onRetry?: () => Promise<{ copied: true } | { copied: false; url: string }>;
} = {}) {
  return render(manualCopyDialogElement(onClose, onRetry));
}

function manualCopyDialogElement(
  onClose: () => void,
  onRetry: () => Promise<CopyShareResult> = vi.fn().mockResolvedValue({ copied: true }),
) {
  return (
    <I18nProvider>
      <ManualCopyDialog url={manualCopyUrl} onClose={onClose} onRetry={onRetry} />
    </I18nProvider>
  );
}
