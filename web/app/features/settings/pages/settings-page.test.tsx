import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SettingsPage } from "./settings-page";

const noop = () => undefined;

describe("settings page", () => {
  it("renders settings without diagnostics under My with working form actions", async () => {
    const user = userEvent.setup();
    const onSaveBaseUrl = vi.fn();
    const onThemeMode = vi.fn();
    render(
      <SettingsPage
        publicBaseUrl="https://example.com"
        themeMode="system"
        onSaveBaseUrl={onSaveBaseUrl}
        onSignOut={noop}
        onThemeMode={onThemeMode}
      />,
    );

    expect(screen.getByRole("heading", { name: "设置" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "关于 Sandrone" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "GitHub" })).toHaveAttribute("href", "https://github.com/kuuvahki-labs/sandrone");
    const saveBaseUrl = screen.getByRole("button", { name: "保存服务地址" });
    expect(saveBaseUrl).toHaveTextContent("保存");
    expect(screen.getByRole("combobox", { name: "主题模式" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Public Base URL" })).toHaveValue("https://example.com");
    expect(screen.getByRole("textbox", { name: "User-Agent" })).toHaveValue("");
    expect(screen.getByText("仅保存在当前浏览器。")).toBeInTheDocument();

    await user.click(screen.getByRole("combobox", { name: "主题模式" }));
    await user.click(screen.getByRole("option", { name: "浅色" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Public Base URL" }), {
      target: { value: "https://public.example.test" },
    });
    await user.click(saveBaseUrl);

    expect(onThemeMode).toHaveBeenCalledWith("light");
    expect(onSaveBaseUrl).toHaveBeenCalledWith("https://public.example.test");
  });
  it.each([
    ["0.1.0", "0123456789abcdef", "v0.1.0 (0123456789ab)"],
    ["0.1.0", "", "v0.1.0"],
    ["dev", "", "dev"],
    [undefined, undefined, "暂不可用"],
  ])("shows the project build identity for %s", (version, revision, expected) => {
    render(
      <SettingsPage
        publicBaseUrl="https://example.com"
        revision={revision}
        themeMode="system"
        version={version}
        onSaveBaseUrl={noop}
        onSignOut={noop}
        onThemeMode={noop}
      />,
    );

    expect(screen.getByText(expected)).toBeInTheDocument();
  });
  it("edits runtime defaults from settings", async () => {
    const user = userEvent.setup();
    const onSaveRuntimeSettings = vi.fn();
    render(
      <SettingsPage
        publicBaseUrl="https://example.com"
        runtimeSettings={{
          remote_defaults: {
            user_agent: "sandrone/0.1.0",
            timeout_ms: 15000,
          },
          probe_defaults: {
            method: "url_test",
            core: "sing-box",
            url: "http://www.gstatic.com/generate_204",
            ntp_server: "time.apple.com",
            timeout_ms: 5000,
            attempts: 1,
            concurrency: 10,
            cache_ttl_seconds: 0,
          },
          cache_defaults: {
            remote_fetch_ttl_seconds: 0,
            subscription_traffic_ttl_seconds: 60,
            subscription_render_ttl_seconds: 0,
            file_render_ttl_seconds: 0,
          },
        }}
        themeMode="system"
        onSaveBaseUrl={noop}
        onSaveRuntimeSettings={onSaveRuntimeSettings}
        onSignOut={noop}
        onThemeMode={noop}
      />,
    );

    expect(screen.getByRole("heading", { name: "运行默认值" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "远程请求" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("button", { name: "缓存" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("button", { name: "测活" })).toHaveAttribute("aria-expanded", "true");

    const remoteGroup = screen.getByRole("region", { name: "远程请求" });
    const cacheGroup = screen.getByRole("region", { name: "缓存" });
    const probeGroup = screen.getByRole("region", { name: "测活" });
    expect(within(probeGroup).getAllByRole("combobox")).toHaveLength(2);
    expect(probeGroup).not.toHaveTextContent(/sing-box|mihomo/);
    expect(within(probeGroup).getByRole("combobox", { name: "默认测活方式" })).toHaveTextContent("url_test");
    const probeURL = within(probeGroup).getByRole("combobox", { name: "URL" });
    await user.click(probeURL);
    await user.keyboard("{ArrowDown}");
    await user.click(await screen.findByRole("option", {
      name: "Cloudflare http://cp.cloudflare.com/generate_204",
    }));
    fireEvent.change(within(remoteGroup).getByRole("textbox", { name: "User-Agent" }), {
      target: { value: "Sandrone Global" },
    });
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
    fireEvent.change(within(probeGroup).getByRole("spinbutton", { name: "缓存（秒）" }), {
      target: { value: "300" },
    });
    const saveRuntimeDefaults = screen.getByRole("button", { name: "保存运行默认值" });
    expect(saveRuntimeDefaults).toHaveTextContent("保存");
    await user.click(saveRuntimeDefaults);

    expect(onSaveRuntimeSettings).toHaveBeenCalledWith(expect.objectContaining({
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
    }));
  });
  it("renders settings language controls in English", () => {
    localStorage.setItem("sandrone.locale", "en-US");

    render(
      <SettingsPage
        publicBaseUrl="https://example.com"
        themeMode="system"
        onSaveBaseUrl={noop}
        onSignOut={noop}
        onThemeMode={noop}
      />,
    );

    expect(screen.getByRole("heading", { name: "Settings" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Language" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Theme mode" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Data management" })).toBeInTheDocument();
    expect(screen.getByText(/Backups are unencrypted plaintext/)).toBeInTheDocument();
  });
  it("confirms before signing out of the admin token", async () => {
    const user = userEvent.setup();
    render(
      <SettingsPage
        publicBaseUrl="https://example.com"
        themeMode="system"
        onSaveBaseUrl={noop}
        onSignOut={noop}
        onThemeMode={noop}
      />,
    );

    await user.click(screen.getByRole("button", { name: "退出登录" }));

    const dialog = screen.getByRole("dialog", { name: "退出登录？" });
    expect(dialog).toHaveTextContent("管理员 token 将从当前浏览器移除");
    expect(within(dialog).getByRole("button", { name: "退出登录" })).toHaveTextContent("退出");
  });

  it("places plaintext backup controls after runtime defaults and shows the selected ZIP", async () => {
    const user = userEvent.setup();
    renderBackupSettings();

    const runtimeCard = screen.getByRole("heading", { name: "运行默认值" }).closest("article");
    const dataCard = screen.getByRole("heading", { name: "数据管理" }).closest("article");
    expect(runtimeCard?.nextElementSibling).toBe(dataCard);
    expect(dataCard).toHaveClass("md:col-span-2");
    expect(screen.getByText("备份是未加密的明文，其中可能包含凭据。恢复会替换服务器上的所有数据（缓存除外）。")).toBeInTheDocument();

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
    renderBackupSettings({ onRestoreBackup });
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
    expect(screen.queryByRole("dialog", { name: "恢复此备份？" })).not.toBeInTheDocument();
  });

  it("disables download, selection, and restore while a backup operation is pending", async () => {
    const user = userEvent.setup();
    let finishDownload: (() => void) | undefined;
    const onDownloadBackup = vi.fn(() => new Promise<void>((resolve) => {
      finishDownload = resolve;
    }));
    renderBackupSettings({ onDownloadBackup });
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

  it("keeps restore confirmation locked while the upload is pending", async () => {
    const user = userEvent.setup();
    let finishRestore: (() => void) | undefined;
    const onRestoreBackup = vi.fn(() => new Promise<void>((resolve) => {
      finishRestore = resolve;
    }));
    renderBackupSettings({ onRestoreBackup });
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
    renderBackupSettings({ onRestoreBackup });
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

function renderBackupSettings({
  onDownloadBackup = vi.fn().mockResolvedValue(undefined),
  onRestoreBackup = vi.fn().mockResolvedValue(undefined),
}: {
  onDownloadBackup?: () => Promise<void>;
  onRestoreBackup?: (file: Blob) => Promise<void>;
} = {}) {
  return render(
    <SettingsPage
      publicBaseUrl="https://example.com"
      themeMode="system"
      onDownloadBackup={onDownloadBackup}
      onRestoreBackup={onRestoreBackup}
      onSaveBaseUrl={noop}
      onSignOut={noop}
      onThemeMode={noop}
    />,
  );
}

function backupFile() {
  return new File([new Uint8Array([80, 75, 3, 4])], "nightly.zip", { type: "application/zip" });
}
