import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { CopyShareResult } from "~/features/shares/data/create-share-actions";
import { I18nProvider } from "~/shared/i18n/context";

import { ManualCopyDialog } from "./manual-copy-dialog";

const url = "https://example.com/s/share/mobile.yaml?token=abc&format=mihomo-proxies#install";

describe("manual copy dialog", () => {
  it("shows and selects the exact URL when opened", async () => {
    renderDialog();

    expect(screen.getByRole("dialog", { name: "请手动复制链接" })).toBeInTheDocument();
    expect(screen.getByText(url)).toBeInTheDocument();
    await waitFor(() => expect(window.getSelection()?.toString()).toBe(url));
  });

  it("closes after a successful retry", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onRetry = vi.fn().mockResolvedValue({ copied: true });
    renderDialog({ onClose, onRetry });

    await user.click(screen.getByRole("button", { name: "重试复制" }));

    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("stays open and reselects the URL after a failed retry", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onRetry = vi.fn().mockResolvedValue({ copied: false, url });
    renderDialog({ onClose, onRetry });
    window.getSelection()?.removeAllRanges();

    await user.click(screen.getByRole("button", { name: "重试复制" }));

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "请手动复制链接" })).toBeInTheDocument();
    expect(window.getSelection()?.toString()).toBe(url);
  });

  it("closes from Done, Escape, and the backdrop", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const { rerender } = renderDialog({ onClose });

    await user.click(screen.getByRole("button", { name: "完成" }));
    expect(onClose).toHaveBeenCalledTimes(1);

    rerender(dialogElement(onClose));
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledTimes(2);

    rerender(dialogElement(onClose));
    const dialogContainer = document.querySelector(".MuiDialog-container")!;
    fireEvent.mouseDown(dialogContainer);
    fireEvent.click(dialogContainer);
    expect(onClose).toHaveBeenCalledTimes(3);
  });

  it("does not replace a partial selection when the URL is clicked", () => {
    renderDialog();
    const link = screen.getByText(url);
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

function renderDialog({
  onClose = vi.fn(),
  onRetry = vi.fn().mockResolvedValue({ copied: true }),
}: {
  onClose?: () => void;
  onRetry?: () => Promise<{ copied: true } | { copied: false; url: string }>;
} = {}) {
  return render(dialogElement(onClose, onRetry));
}

function dialogElement(
  onClose: () => void,
  onRetry: () => Promise<CopyShareResult> = vi.fn().mockResolvedValue({ copied: true }),
) {
  return (
    <I18nProvider>
      <ManualCopyDialog url={url} onClose={onClose} onRetry={onRetry} />
    </I18nProvider>
  );
}
