import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { defaultProjectSettings } from "~/features/settings/model/project-settings";
import { UICapabilityProvider } from "~/shared/capabilities/context";

import { SettingsDataPage } from "./settings-data-page";
import { SettingsServicePage } from "./settings-service-page";

describe("settings service page", () => {
  it("hides probe defaults and scheduled refresh when capabilities are unavailable", () => {
    render(
      <UICapabilityProvider value={{
        capabilities: [
          { key: "probe.enabled", enabled: false },
          { key: "scheduler.enabled", enabled: false },
        ],
        loaded: true,
        hasFeature: () => false,
        getFeature: (key) => ({ key, enabled: false }),
      }}>
        <SettingsServicePage
          overrides={{}}
          restartRequired={[]}
          scheduledRefreshResources={[]}
          settings={defaultProjectSettings}
          onBack={vi.fn()}
          onSave={vi.fn()}
        />
      </UICapabilityProvider>,
    );

    expect(screen.queryByRole("heading", { name: "测活" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "定时更新" })).not.toBeInTheDocument();
  });

  it("edits the complete service settings page and saves every group before returning", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const onBack = vi.fn();

    renderRuntimePage(
      <SettingsServicePage
        defaultUserAgent="sandrone/0.2.0"
        overrides={{ "http.listen": "environment" }}
        restartRequired={[]}
        scheduledRefreshResources={[]}
        settings={defaultProjectSettings}
        onBack={onBack}
        onSave={onSave}
      />,
    );

    expect(screen.getByRole("heading", { name: "订阅流量" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "运行默认值" })).toBeInTheDocument();
    const automaticTraffic = screen.getByRole("switch", { name: "在订阅列表显示" });
    expect(automaticTraffic).not.toBeChecked();
    expect(screen.getByRole("heading", { name: "远程请求", level: 4 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "脚本", level: 4 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "测活", level: 4 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "缓存", level: 4 })).toBeInTheDocument();
    expect(screen.getAllByText("缓存时间设为 0 时关闭。")).toHaveLength(1);

    expect(screen.getByRole("textbox", { name: "监听地址" })).toHaveValue("127.0.0.1:1137");
    expect(screen.queryByRole("textbox", { name: "Web UI 静态目录" })).not.toBeInTheDocument();
    expect(screen.getByText("当前由 environment 覆盖")).toBeInTheDocument();
    fireEvent.change(screen.getByRole("textbox", { name: "MCP 路径" }), {
      target: { value: "/agent" },
    });
    await user.click(automaticTraffic);
    const remoteGroup = screen.getByRole("region", { name: "远程请求" });
    expect(within(remoteGroup).getByRole("textbox", { name: "User-Agent" })).toHaveValue("");
    expect(within(remoteGroup).getByRole("textbox", { name: "User-Agent" })).toHaveAttribute("placeholder", "sandrone/0.2.0");
    fireEvent.change(within(remoteGroup).getByRole("textbox", { name: "User-Agent" }), {
      target: { value: "Sandrone Global" },
    });
    fireEvent.change(within(remoteGroup).getByRole("spinbutton", { name: "缓存（秒）" }), {
      target: { value: "120" },
    });

    const scriptGroup = screen.getByRole("region", { name: "脚本" });
    fireEvent.change(within(scriptGroup).getByRole("spinbutton", { name: "超时（秒）" }), {
      target: { value: "3.5" },
    });

    const probeGroup = screen.getByRole("region", { name: "测活" });
    expect(within(probeGroup).getAllByRole("combobox")).toHaveLength(2);
    expect(within(probeGroup).getByRole("combobox", { name: "测活方式" })).toHaveTextContent("URL 测试");
    fireEvent.change(within(probeGroup).getByRole("spinbutton", { name: "结果缓存（秒）" }), {
      target: { value: "300" },
    });
    const probeURL = within(probeGroup).getByRole("combobox", { name: "URL" });
    await user.click(probeURL);
    await user.keyboard("{ArrowDown}");
    await user.click(await screen.findByRole("option", {
      name: "Cloudflare http://cp.cloudflare.com/generate_204",
    }));
    const probeMethod = within(probeGroup).getByRole("combobox", { name: "测活方式" });
    await user.click(probeMethod);
    await user.click(await screen.findByRole("option", { name: "UDP NTP" }));
    expect(within(probeGroup).queryByRole("combobox", { name: "URL" })).not.toBeInTheDocument();
    expect(within(probeGroup).getByRole("textbox", { name: "NTP 服务器" })).toBeInTheDocument();
    await user.click(probeMethod);
    await user.click(await screen.findByRole("option", { name: "URL 测试" }));
    expect(within(probeGroup).getByRole("combobox", { name: "URL" })).toHaveValue("http://cp.cloudflare.com/generate_204");

    const cacheGroup = screen.getByRole("region", { name: "缓存" });
    fireEvent.change(within(cacheGroup).getByRole("spinbutton", { name: "订阅快照（秒）" }), {
      target: { value: "300" },
    });

    const pageHeader = screen.getByRole("heading", { name: "服务设置" }).closest("header");
    expect(pageHeader).not.toBeNull();
    const saveRuntimeDefaults = within(pageHeader as HTMLElement).getByRole("button", { name: "保存设置" });
    await user.click(saveRuntimeDefaults);

    expect(onSave).toHaveBeenCalledWith(expect.objectContaining({
      remote_defaults: expect.objectContaining({
        user_agent: "Sandrone Global",
        timeout_ms: 15000,
      }),
      probe_defaults: expect.objectContaining({
        core: "sing-box",
        url: "http://cp.cloudflare.com/generate_204",
      }),
      script_defaults: { timeout_ms: 3500 },
      cache_defaults: expect.objectContaining({
        remote_fetch_ttl_seconds: 120,
        probe_ttl_seconds: 300,
        subscription_snapshot_ttl_seconds: 300,
      }),
      mcp: expect.objectContaining({ path: "/agent" }),
      subscriptions: { auto_load_traffic: true },
    }));

    await user.click(screen.getByRole("button", { name: "返回" }));
    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it("does not replace an unsaved schedule when status polling updates", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const baseProps = {
      overrides: {},
      restartRequired: [],
      scheduledRefreshResources: [],
      settings: defaultProjectSettings,
      onBack: vi.fn(),
      onSave,
    };
    const { rerender } = renderRuntimePage(
      <SettingsServicePage
        {...baseProps}
        scheduledRefreshStatus={{ enabled: false, running: false, last_success_count: 0, last_failure_count: 0, skipped_count: 0 }}
      />,
    );
    await user.click(screen.getByRole("switch", { name: "启用定时更新" }));
    const schedule = screen.getByRole("textbox", { name: "Cron 计划" });
    await user.clear(schedule);
    await user.type(schedule, "@daily");

    rerender(
      <UICapabilityProvider value={runtimeCapabilityValue}>
        <SettingsServicePage
          {...baseProps}
          scheduledRefreshStatus={{ enabled: true, running: true, last_success_count: 2, last_failure_count: 1, skipped_count: 3 }}
        />
      </UICapabilityProvider>,
    );
    await user.click(screen.getByRole("button", { name: "保存设置" }));

    expect(onSave).toHaveBeenCalledWith(expect.objectContaining({
      scheduled_refresh: expect.objectContaining({ schedule: "@daily" }),
    }));
  });

  it("selects grouped resources and preserves a configured missing target", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const settings = {
      ...defaultProjectSettings,
      scheduled_refresh: {
        ...defaultProjectSettings.scheduled_refresh,
        enabled: true,
        targets: [{ kind: "subscription" as const, name: "missing-provider" }],
      },
    };
    renderRuntimePage(
      <SettingsServicePage
        overrides={{}}
        restartRequired={[]}
        scheduledRefreshResources={[
          { kind: "subscription", name: "provider", label: "Provider" },
          { kind: "file", name: "client.yaml", label: "Client config" },
        ]}
        settings={settings}
        onBack={vi.fn()}
        onSave={onSave}
      />,
    );

    expect(screen.getByText("missing-provider (资源已缺失)")).toBeInTheDocument();
    await user.click(screen.getByRole("combobox", { name: "更新目标" }));
    await user.click(await screen.findByRole("option", { name: "Provider" }));
    await user.click(screen.getByRole("button", { name: "保存设置" }));

    expect(onSave).toHaveBeenCalledWith(expect.objectContaining({
      scheduled_refresh: expect.objectContaining({
        targets: [
          { kind: "subscription", name: "missing-provider" },
          { kind: "subscription", name: "provider" },
        ],
      }),
    }));
  });
});

describe("settings data page", () => {
  it("confirms cache clearing", async () => {
    const user = userEvent.setup();
    const onClearCache = vi.fn().mockResolvedValue(undefined);
    renderDataPage({ onClearCache });

    expect(screen.getByRole("heading", { name: "缓存" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "清空缓存" }));
    let dialog = screen.getByRole("dialog", { name: "清空缓存？" });
    expect(dialog).toHaveTextContent("后续请求可能暂时变慢");
    await user.click(within(dialog).getByRole("button", { name: "取消" }));
    expect(onClearCache).not.toHaveBeenCalled();
    await waitFor(() => expect(dialog).not.toBeInTheDocument());

    await user.click(screen.getByRole("button", { name: "清空缓存" }));
    dialog = screen.getByRole("dialog", { name: "清空缓存？" });
    await user.click(within(dialog).getByRole("button", { name: "清空缓存" }));
    expect(onClearCache).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
  });

  it("disables cache actions while clearing", () => {
    renderDataPage({ cacheClearing: true });

    expect(screen.getByRole("button", { name: "正在清理" })).toBeDisabled();
  });

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
  cacheClearing = false,
  onBack = vi.fn(),
  onClearCache = vi.fn().mockResolvedValue(undefined),
  onDownloadBackup = vi.fn().mockResolvedValue(undefined),
  onRestoreBackup = vi.fn().mockResolvedValue(undefined),
}: {
  cacheClearing?: boolean;
  onBack?: () => void;
  onClearCache?: () => Promise<void>;
  onDownloadBackup?: () => Promise<void>;
  onRestoreBackup?: (file: Blob) => Promise<void>;
} = {}) {
  return render(
    <SettingsDataPage
      cacheClearing={cacheClearing}
      onBack={onBack}
      onClearCache={onClearCache}
      onDownloadBackup={onDownloadBackup}
      onRestoreBackup={onRestoreBackup}
    />,
  );
}

const runtimeCapabilityValue = {
  capabilities: [
    { key: "probe.enabled", enabled: true },
    { key: "scheduler.enabled", enabled: true },
  ],
  loaded: true,
  hasFeature: (key: string) => key === "probe.enabled" || key === "scheduler.enabled",
  getFeature: (key: string) => key === "probe.enabled" || key === "scheduler.enabled"
    ? { key, enabled: true }
    : undefined,
};

function renderRuntimePage(page: React.ReactElement) {
  return render(
    <UICapabilityProvider value={runtimeCapabilityValue}>
      {page}
    </UICapabilityProvider>,
  );
}

function backupFile() {
  return new File([new Uint8Array([80, 75, 3, 4])], "nightly.zip", { type: "application/zip" });
}
