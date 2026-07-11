import { useState } from "react";
import { render, renderHook, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ApiClient } from "~/shared/api/client";
import { I18nProvider } from "~/shared/i18n/context";

import { ShareDialogProvider, useShareDialog } from "./share-dialog-context";

describe("share dialog context", () => {
  beforeEach(() => localStorage.clear());

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
        <ShareDialogProvider client={client} showNotice={vi.fn()}>
          <DialogHarness />
        </ShareDialogProvider>
      </I18nProvider>,
    );

    expect(screen.getByTestId("share-dialog-keys")).toHaveTextContent("close,open");
    await user.click(screen.getByRole("button", { name: "Open route share dialog" }));

    const dialog = screen.getByRole("dialog", { name: "创建分享链接" });
    expect(within(dialog).getByRole("textbox", { name: "名称" })).toHaveValue("provider");
    expect(within(dialog).getByRole("textbox", { name: "分享目标" })).toHaveValue("provider");

    await user.click(screen.getByRole("button", { name: "Close route share dialog", hidden: true }));
    expect(screen.queryByRole("dialog", { name: "创建分享链接" })).not.toBeInTheDocument();
  });

  it("keeps the context value and commands stable across target changes", async () => {
    const user = userEvent.setup();
    const observe = vi.fn<(value: ReturnType<typeof useShareDialog>) => void>();
    const client = { createShare: vi.fn() } as unknown as ApiClient;

    render(
      <I18nProvider>
        <ShareDialogProvider client={client} showNotice={vi.fn()}>
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
        <ShareDialogProvider client={client} showNotice={showNotice}>
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
});

function DialogHarness() {
  const shareDialog = useShareDialog();
  return (
    <>
      <output data-testid="share-dialog-keys">{Object.keys(shareDialog).sort().join(",")}</output>
      <button type="button" onClick={() => shareDialog.open({ kind: "subscription", name: "provider" })}>
        Open route share dialog
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
