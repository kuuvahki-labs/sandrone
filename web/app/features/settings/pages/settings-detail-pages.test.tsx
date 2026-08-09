import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { defaultProjectSettings } from "~/features/settings/model/project-settings";

import { SettingsDataPage } from "./settings-data-page";
import { SettingsRuntimePage } from "./settings-runtime-page";

describe("settings runtime page", () => {
  it("edits the complete runtime page and saves every nested group before returning", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const onBack = vi.fn();

    render(
      <SettingsRuntimePage
        overrides={{ "http.listen": "environment" }}
        restartRequired={[]}
        settings={defaultProjectSettings}
        onBack={onBack}
        onSave={onSave}
      />,
    );

    expect(screen.getByRole("heading", { name: "订阅流量" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "运行默认值" })).toBeInTheDocument();
    const automaticTraffic = screen.getByRole("switch", { name: "自动获取流量" });
    expect(automaticTraffic).not.toBeChecked();
    expect(screen.getByRole("button", { name: "远程请求" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("button", { name: "缓存" })).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByRole("button", { name: "测活" })).toHaveAttribute("aria-expanded", "false");

    expect(screen.getByRole("textbox", { name: "监听地址" })).toHaveValue("127.0.0.1:1137");
    expect(screen.queryByRole("textbox", { name: "Web UI 静态目录" })).not.toBeInTheDocument();
    expect(screen.getByText("当前由 environment 覆盖")).toBeInTheDocument();
    fireEvent.change(screen.getByRole("textbox", { name: "MCP 路径" }), {
      target: { value: "/agent" },
    });
    await user.click(automaticTraffic);
    const remoteGroup = screen.getByRole("region", { name: "远程请求" });
    fireEvent.change(within(remoteGroup).getByRole("textbox", { name: "User-Agent" }), {
      target: { value: "Sandrone Global" },
    });

    await user.click(screen.getByRole("button", { name: "缓存" }));
    const cacheGroup = screen.getByRole("region", { name: "缓存" });
    fireEvent.change(within(cacheGroup).getByRole("spinbutton", { name: "远程请求缓存（秒）" }), {
      target: { value: "120" },
    });
    fireEvent.change(within(cacheGroup).getByRole("spinbutton", { name: "订阅流量缓存（秒）" }), {
      target: { value: "15" },
    });
    fireEvent.change(within(cacheGroup).getByRole("spinbutton", { name: "订阅渲染缓存（秒）" }), {
      target: { value: "180" },
    });
    fireEvent.change(within(cacheGroup).getByRole("spinbutton", { name: "文件渲染缓存（秒）" }), {
      target: { value: "240" },
    });

    await user.click(screen.getByRole("button", { name: "测活" }));
    const probeGroup = screen.getByRole("region", { name: "测活" });
    expect(within(probeGroup).getAllByRole("combobox")).toHaveLength(2);
    expect(within(probeGroup).getByRole("combobox", { name: "默认测活方式" })).toHaveTextContent("url_test");
    const probeURL = within(probeGroup).getByRole("combobox", { name: "URL" });
    await user.click(probeURL);
    await user.keyboard("{ArrowDown}");
    await user.click(await screen.findByRole("option", {
      name: "Cloudflare http://cp.cloudflare.com/generate_204",
    }));
    fireEvent.change(within(probeGroup).getByRole("spinbutton", { name: "缓存（秒）" }), {
      target: { value: "300" },
    });

    const saveRuntimeDefaults = screen.getByRole("button", { name: "保存设置" });
    await user.click(saveRuntimeDefaults);

    expect(onSave).toHaveBeenCalledWith(expect.objectContaining({
      remote_defaults: expect.objectContaining({
        user_agent: "Sandrone Global",
        timeout_ms: 15000,
      }),
      probe_defaults: expect.objectContaining({
        cache_ttl_seconds: 300,
        core: "sing-box",
        url: "http://cp.cloudflare.com/generate_204",
      }),
      cache_defaults: expect.objectContaining({
        remote_fetch_ttl_seconds: 120,
        subscription_traffic_ttl_seconds: 15,
        subscription_render_ttl_seconds: 180,
        file_render_ttl_seconds: 240,
      }),
      mcp: expect.objectContaining({ path: "/agent" }),
      subscriptions: { auto_load_traffic: true },
    }));

    await user.click(screen.getByRole("button", { name: "返回" }));
    expect(onBack).toHaveBeenCalledTimes(1);
  });
});

describe("settings data page", () => {
  it("shows the selected ZIP", async () => {
    const user = userEvent.setup();
    renderDataPage();

    const input = screen.getByLabelText("选择备份 ZIP 文件");
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
