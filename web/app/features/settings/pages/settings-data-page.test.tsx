import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SettingsDataPage } from "./settings-data-page";

describe("settings data page", () => {
  it("shows the focused heading and plaintext warning and returns", async () => {
    const user = userEvent.setup();
    const onBack = vi.fn();

    renderDataPage({ onBack });

    const pageHeader = screen.getByRole("heading", { name: "数据管理" }).closest("header");
    expect(pageHeader).toHaveClass("MuiPaper-root", "MuiPaper-outlined");
    expect(pageHeader?.parentElement).toHaveClass("grid", "gap-6");
    expect(screen.getByRole("note")).toHaveTextContent("备份是未加密的明文");
    await user.click(screen.getByRole("button", { name: "返回" }));

    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it("shows the selected ZIP", async () => {
    const user = userEvent.setup();
    renderDataPage();

    const input = screen.getByLabelText("选择备份 ZIP 文件");
    expect(input).toHaveClass("sr-only");
    expect(input).toHaveAttribute("accept", ".zip,application/zip");
    await user.upload(input, backupFile());

    expect(screen.getByText("已选择：nightly.zip")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "恢复备份" })).toBeEnabled();
  });

  it("cancels restore without uploading and clears the selection after confirmation succeeds", async () => {
    const user = userEvent.setup();
    const onRestoreBackup = vi.fn().mockResolvedValue(undefined);
    renderDataPage({ onRestoreBackup });
    await user.upload(screen.getByLabelText("选择备份 ZIP 文件"), backupFile());
    const restoreButton = screen.getByRole("button", { name: "恢复备份" });

    await user.click(restoreButton);
    let dialog = screen.getByRole("dialog", { name: "恢复此备份？" });
    expect(dialog).toHaveTextContent("所有服务器数据（缓存除外）将被所选备份替换");
    await user.click(within(dialog).getByRole("button", { name: "取消" }));

    expect(onRestoreBackup).not.toHaveBeenCalled();
    expect(screen.getByText("已选择：nightly.zip")).toBeInTheDocument();
    await waitFor(() => expect(dialog).not.toBeInTheDocument());

    await user.click(restoreButton);
    dialog = screen.getByRole("dialog", { name: "恢复此备份？" });
    await user.click(within(dialog).getByRole("button", { name: "替换服务器数据" }));

    expect(onRestoreBackup).toHaveBeenCalledWith(expect.objectContaining({ name: "nightly.zip" }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(restoreButton).toBeDisabled();
    expect(screen.queryByText("已选择：nightly.zip")).not.toBeInTheDocument();
  });

  it("disables all backup controls while an operation is pending", async () => {
    const user = userEvent.setup();
    let finishDownload: (() => void) | undefined;
    const onDownloadBackup = vi.fn(() => new Promise<void>((resolve) => {
      finishDownload = resolve;
    }));
    renderDataPage({ onDownloadBackup });
    const input = screen.getByLabelText("选择备份 ZIP 文件");
    await user.upload(input, backupFile());

    await user.click(screen.getByRole("button", { name: "下载备份" }));

    expect(screen.getByRole("button", { name: "正在下载" })).toBeDisabled();
    expect(input).toBeDisabled();
    expect(screen.getByRole("button", { name: "恢复备份" })).toBeDisabled();
    expect(onDownloadBackup).toHaveBeenCalledTimes(1);

    finishDownload?.();
    expect(await screen.findByRole("button", { name: "下载备份" })).toBeEnabled();
    expect(input).toBeEnabled();
    expect(screen.getByRole("button", { name: "恢复备份" })).toBeEnabled();
  });

  it("keeps the restore confirmation locked while the upload is pending", async () => {
    const user = userEvent.setup();
    let finishRestore: (() => void) | undefined;
    const onRestoreBackup = vi.fn(() => new Promise<void>((resolve) => {
      finishRestore = resolve;
    }));
    renderDataPage({ onRestoreBackup });
    const input = screen.getByLabelText("选择备份 ZIP 文件");
    await user.upload(input, backupFile());
    const downloadButton = screen.getByRole("button", { name: "下载备份" });
    const restoreButton = screen.getByRole("button", { name: "恢复备份" });
    await user.click(restoreButton);
    const dialog = screen.getByRole("dialog", { name: "恢复此备份？" });

    await user.click(within(dialog).getByRole("button", { name: "替换服务器数据" }));

    expect(within(dialog).getByRole("button", { name: "正在恢复" })).toBeDisabled();
    expect(within(dialog).getByRole("button", { name: "取消" })).toBeDisabled();
    expect(downloadButton).toBeDisabled();
    expect(input).toBeDisabled();
    expect(onRestoreBackup).toHaveBeenCalledTimes(1);

    finishRestore?.();
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(restoreButton).toBeDisabled();
  });

  it("preserves a failed restore selection and lets the administrator retry", async () => {
    const user = userEvent.setup();
    const onRestoreBackup = vi.fn()
      .mockRejectedValueOnce(new Error("invalid archive"))
      .mockResolvedValueOnce(undefined);
    renderDataPage({ onRestoreBackup });
    await user.upload(screen.getByLabelText("选择备份 ZIP 文件"), backupFile());
    const restoreButton = screen.getByRole("button", { name: "恢复备份" });
    await user.click(restoreButton);
    const dialog = screen.getByRole("dialog", { name: "恢复此备份？" });

    await user.click(within(dialog).getByRole("button", { name: "替换服务器数据" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("恢复失败。所选文件已保留，请重试。");
    expect(screen.getByText("已选择：nightly.zip")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "替换服务器数据" })).toBeEnabled();

    await user.click(within(dialog).getByRole("button", { name: "替换服务器数据" }));

    expect(onRestoreBackup).toHaveBeenCalledTimes(2);
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(restoreButton).toBeDisabled();
    expect(screen.queryByText("已选择：nightly.zip")).not.toBeInTheDocument();
  });
});

function renderDataPage({
  onBack = vi.fn(),
  onDownloadBackup = vi.fn().mockResolvedValue(undefined),
  onRestoreBackup = vi.fn().mockResolvedValue(undefined),
}: {
  onBack?: () => void;
  onDownloadBackup?: () => Promise<void>;
  onRestoreBackup?: (file: Blob) => Promise<void>;
} = {}) {
  return render(
    <SettingsDataPage
      onBack={onBack}
      onDownloadBackup={onDownloadBackup}
      onRestoreBackup={onRestoreBackup}
    />,
  );
}

function backupFile() {
  return new File([new Uint8Array([80, 75, 3, 4])], "nightly.zip", { type: "application/zip" });
}
