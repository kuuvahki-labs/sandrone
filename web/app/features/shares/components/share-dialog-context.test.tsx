import { useState } from "react";
import { fireEvent, render, renderHook, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { ApiClient } from "~/shared/api/client";
import { I18nProvider } from "~/shared/i18n/context";

import { ShareDialogProvider, useShareDialog } from "./share-dialog-context";

describe("share dialog context", () => {
  let originalClipboardDescriptor: PropertyDescriptor | undefined;

  beforeEach(() => localStorage.clear());

  afterEach(() => {
    if (originalClipboardDescriptor) {
      Object.defineProperty(navigator, "clipboard", originalClipboardDescriptor);
    } else {
      Reflect.deleteProperty(navigator, "clipboard");
    }
    originalClipboardDescriptor = undefined;
    window.getSelection()?.removeAllRanges();
  });

  it("rejects consumers outside ShareDialogProvider", () => {
    expect(() => renderHook(() => useShareDialog())).toThrowError(
      "useShareDialog must be used inside ShareDialogProvider",
    );
  });

  it("exposes only memoized open and close commands and owns the target privately", async () => {
    const user = userEvent.setup();
    const client = { createShare: vi.fn() } as unknown as ApiClient;

    render(
      <I18nProvider>
        <ShareDialogProvider client={client} publicBaseUrl="https://public.example" showNotice={vi.fn()}>
          <DialogHarness />
        </ShareDialogProvider>
      </I18nProvider>,
    );

    expect(screen.getByTestId("share-dialog-keys")).toHaveTextContent("close,open");
    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));

    const dialog = screen.getByRole("dialog", { name: "创建分享链接" });
    expect(within(dialog).getByRole("textbox", { name: "名称" })).toHaveValue("provider");
    expect(within(dialog).getByRole("textbox", { name: "分享目标" })).toHaveValue("provider");
    expect(within(dialog).getByRole("combobox", { name: "默认输出格式" })).toHaveValue("base64");

    await user.click(screen.getByRole("button", { name: "Close route share dialog", hidden: true }));
    expect(screen.queryByRole("dialog", { name: "创建分享链接" })).not.toBeInTheDocument();
  });

  it("keeps the context value and commands stable across target changes", async () => {
    const user = userEvent.setup();
    const observe = vi.fn<(value: ReturnType<typeof useShareDialog>) => void>();
    const client = { createShare: vi.fn() } as unknown as ApiClient;

    render(
      <I18nProvider>
        <ShareDialogProvider client={client} publicBaseUrl="https://public.example" showNotice={vi.fn()}>
          <IdentityHarness observe={observe} />
        </ShareDialogProvider>
      </I18nProvider>,
    );

    const firstValue = observe.mock.calls[0]?.[0];
    expect(firstValue).toBeDefined();
    expect(observe).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Open identity dialog" }));
    expect(observe).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "Read identity", hidden: true }));

    const secondValue = observe.mock.calls[1]?.[0];
    expect(secondValue).toBe(firstValue);
    expect(secondValue?.open).toBe(firstValue?.open);
    expect(secondValue?.close).toBe(firstValue?.close);

    await user.click(screen.getByRole("button", { name: "Close identity dialog", hidden: true }));
    expect(observe).toHaveBeenCalledTimes(2);
    await user.click(screen.getByRole("button", { name: "Read identity" }));
    expect(observe.mock.calls[2]?.[0]).toBe(firstValue);
  });

  it("keeps a rejected submission open and renders the error", async () => {
    const user = userEvent.setup();
    const showNotice = vi.fn();
    const client = {
      createShare: vi.fn(async () => {
        throw new Error("share request failed");
      }),
    } as unknown as ApiClient;

    render(
      <I18nProvider>
        <ShareDialogProvider client={client} publicBaseUrl="https://public.example" showNotice={showNotice}>
          <DialogHarness />
        </ShareDialogProvider>
      </I18nProvider>,
    );

    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));
    await user.click(screen.getByRole("button", { name: "保存分享链接" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("share request failed");
    expect(screen.getByRole("dialog", { name: "创建分享链接" })).toBeInTheDocument();
    expect(showNotice).not.toHaveBeenCalled();
  });

  it("shows the created default link, closes from the result, and resets on reopen", async () => {
    const user = userEvent.setup();
    const client = {
      createShare: vi.fn().mockResolvedValue(createResponse),
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

    await user.click(within(result).getByRole("button", { name: "完成" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));
    expect(screen.getByRole("dialog", { name: "创建分享链接" })).toBeInTheDocument();
  });

  it("starts a fresh form for a new open command without requiring the result dialog to close", async () => {
    const user = userEvent.setup();
    const client = {
      createShare: vi.fn().mockResolvedValue(createResponse),
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
    expect(await screen.findByRole("dialog", { name: "分享链接已创建" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Open another share dialog", hidden: true }));

    const dialog = screen.getByRole("dialog", { name: "创建分享链接" });
    expect(within(dialog).getByRole("textbox", { name: "名称" })).toHaveValue("second.yaml");
    expect(within(dialog).getByRole("textbox", { name: "分享目标" })).toHaveValue("second.yaml");
    expect(within(dialog).queryByText(createdPublicUrl)).not.toBeInTheDocument();
  });

  it("locks every user close path while submitting, then preserves the form for a retry", async () => {
    const user = userEvent.setup();
    const pendingCreate = deferred<unknown>();
    const client = {
      createShare: vi.fn()
        .mockImplementationOnce(() => pendingCreate.promise)
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
    expect(screen.getByRole("textbox", { name: "名称" })).toHaveValue("retry-mobile");

    await user.click(screen.getByRole("button", { name: "保存分享链接" }));
    expect(await screen.findByRole("dialog", { name: "分享链接已创建" })).toBeInTheDocument();
  });

  it("copies the visible result link with the primary action", async () => {
    const user = userEvent.setup();
    const writeText = installClipboardMock();
    renderDialog({ createShare: vi.fn().mockResolvedValue(createResponse), showNotice: vi.fn() });

    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));
    await user.click(screen.getByRole("button", { name: "保存分享链接" }));
    const result = await screen.findByRole("dialog", { name: "分享链接已创建" });
    await user.click(within(result).getByRole("button", { name: "复制链接" }));

    expect(writeText).toHaveBeenCalledWith(createdPublicUrl);
  });

  it("selects the whole visible result link and warns when automatic copy is unavailable", async () => {
    const user = userEvent.setup();
    const showNotice = vi.fn();
    renderDialog({ createShare: vi.fn().mockResolvedValue(createResponse), showNotice });

    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));
    await user.click(screen.getByRole("button", { name: "保存分享链接" }));
    const result = await screen.findByRole("dialog", { name: "分享链接已创建" });
    originalClipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Reflect.deleteProperty(navigator, "clipboard");
    await user.click(within(result).getByRole("button", { name: "复制链接" }));

    expect(showNotice).toHaveBeenCalledWith("无法自动复制，请手动复制链接", "warning");
    expect(window.getSelection()?.toString()).toBe(createdPublicUrl);
  });

  it("selects a result link on click without replacing a partial drag selection", async () => {
    const user = userEvent.setup();
    renderDialog({ createShare: vi.fn().mockResolvedValue(createResponse), showNotice: vi.fn() });

    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));
    await user.click(screen.getByRole("button", { name: "保存分享链接" }));
    const publicUrl = await screen.findByText(createdPublicUrl);

    await user.click(publicUrl);
    expect(window.getSelection()?.toString()).toBe(createdPublicUrl);

    const range = document.createRange();
    range.setStart(publicUrl.firstChild!, 0);
    range.setEnd(publicUrl.firstChild!, 5);
    window.getSelection()?.removeAllRanges();
    window.getSelection()?.addRange(range);
    fireEvent.click(publicUrl);

    expect(window.getSelection()?.toString()).toBe("https");
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
