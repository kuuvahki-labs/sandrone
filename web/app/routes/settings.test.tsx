import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useSandrone } from "~/core/provider/context";
import type { SandroneContextValue } from "~/core/provider/types";
import type { ApiClient, RuntimeSettingsInput } from "~/shared/api/client";
import { I18nProvider, useI18n } from "~/shared/i18n/context";

import SettingsRoute from "./settings";

vi.mock("~/core/provider/context", () => ({
  useSandrone: vi.fn(),
}));

const runtimeSettings: RuntimeSettingsInput = {
  remote_defaults: { user_agent: "sandrone/0.1.0", timeout_ms: 15000 },
  probe_defaults: {},
  cache_defaults: {},
};

describe("SettingsRoute", () => {
  beforeEach(() => {
    vi.mocked(useSandrone).mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("loads version independently while runtime settings are pending", async () => {
    const getRuntimeSettings = vi.fn(() => new Promise<RuntimeSettingsInput>(() => undefined));
    const getVersion = vi.fn().mockResolvedValue({ name: "sandrone", version: "0.1.0" });
    mockApp({ getRuntimeSettings, getVersion });

    renderSettingsRoute();

    await waitFor(() => {
      expect(getRuntimeSettings).toHaveBeenCalledTimes(1);
      expect(getVersion).toHaveBeenCalledTimes(1);
    });
    expect(await screen.findByText("v0.1.0")).toBeInTheDocument();
  });

  it("silently falls back when version loading fails", async () => {
    const showNotice = vi.fn();
    const getVersion = vi.fn().mockRejectedValue(new Error("version unavailable"));
    mockApp({
      getRuntimeSettings: vi.fn().mockResolvedValue(runtimeSettings),
      getVersion,
    }, showNotice);

    renderSettingsRoute();

    expect(await screen.findByText("暂不可用")).toBeInTheDocument();
    await waitFor(() => expect(getVersion).toHaveBeenCalledTimes(1));
    expect(showNotice).not.toHaveBeenCalled();
  });

  it("keeps the existing notification when runtime settings loading fails", async () => {
    const showNotice = vi.fn();
    mockApp({
      getRuntimeSettings: vi.fn().mockRejectedValue(new Error("runtime unavailable")),
      getVersion: vi.fn().mockRejectedValue(new Error("version unavailable")),
    }, showNotice);

    renderSettingsRoute();

    await waitFor(() => {
      expect(showNotice).toHaveBeenCalledTimes(1);
      expect(showNotice).toHaveBeenCalledWith("运行默认值加载失败", "error");
    });
  });

  it("loads version only once when the locale changes", async () => {
    const getVersion = vi.fn().mockResolvedValue({ name: "sandrone", version: "0.1.0" });
    mockApp({
      getRuntimeSettings: vi.fn().mockResolvedValue(runtimeSettings),
      getVersion,
    });

    render(
      <I18nProvider>
        <LocaleSwitchHarness />
      </I18nProvider>,
    );

    await waitFor(() => expect(getVersion).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole("button", { name: "Switch locale" }));
    await screen.findByText("About Sandrone");
    expect(getVersion).toHaveBeenCalledTimes(1);
  });

  it("downloads through a temporary anchor and cleans up its object URL", async () => {
    const user = userEvent.setup();
    const blob = new Blob([new Uint8Array([80, 75, 3, 4])], { type: "application/zip" });
    const downloadBackup = vi.fn().mockResolvedValue({ blob, filename: "server-backup.zip" });
    const createObjectURL = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:sandrone-backup");
    const revokeObjectURL = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => undefined);
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    mockApp({
      downloadBackup,
      getRuntimeSettings: vi.fn().mockResolvedValue(runtimeSettings),
      getVersion: vi.fn().mockResolvedValue({ name: "sandrone", version: "0.1.0" }),
    });
    renderSettingsRoute();

    await user.click(screen.getByRole("button", { name: "下载备份" }));

    await waitFor(() => expect(downloadBackup).toHaveBeenCalledTimes(1));
    const anchor = click.mock.instances[0] as HTMLAnchorElement;
    expect(createObjectURL).toHaveBeenCalledWith(blob);
    expect(anchor.download).toBe("server-backup.zip");
    expect(anchor.href).toBe("blob:sandrone-backup");
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:sandrone-backup");
    expect(document.body).not.toContainElement(anchor);
  });

  it("cleans up the temporary backup link when triggering the download fails", async () => {
    const user = userEvent.setup();
    const showNotice = vi.fn();
    const downloadBackup = vi.fn().mockResolvedValue({
      blob: new Blob(["backup"], { type: "application/zip" }),
      filename: "server-backup.zip",
    });
    vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:sandrone-backup");
    const revokeObjectURL = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => undefined);
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {
      throw new Error("download blocked");
    });
    mockApp({
      downloadBackup,
      getRuntimeSettings: vi.fn().mockResolvedValue(runtimeSettings),
      getVersion: vi.fn().mockResolvedValue({ name: "sandrone", version: "0.1.0" }),
    }, showNotice);
    renderSettingsRoute();

    await user.click(screen.getByRole("button", { name: "下载备份" }));

    await waitFor(() => expect(showNotice).toHaveBeenCalledWith("备份下载失败", "error"));
    const anchor = click.mock.instances[0] as HTMLAnchorElement;
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:sandrone-backup");
    expect(document.body).not.toContainElement(anchor);
  });

  it("reloads runtime settings and shows success after restoring a backup", async () => {
    const user = userEvent.setup();
    const showNotice = vi.fn();
    const restoredSettings: RuntimeSettingsInput = {
      ...runtimeSettings,
      remote_defaults: { user_agent: "restored-agent", timeout_ms: 30000 },
    };
    const getRuntimeSettings = vi.fn()
      .mockResolvedValueOnce(runtimeSettings)
      .mockResolvedValueOnce(restoredSettings);
    const restoreBackup = vi.fn().mockResolvedValue(undefined);
    mockApp({
      getRuntimeSettings,
      getVersion: vi.fn().mockResolvedValue({ name: "sandrone", version: "0.1.0" }),
      restoreBackup,
    }, showNotice);
    renderSettingsRoute();
    await waitFor(() => expect(getRuntimeSettings).toHaveBeenCalledTimes(1));

    await chooseAndConfirmBackup(user);

    await waitFor(() => expect(getRuntimeSettings).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "恢复此备份？" })).not.toBeInTheDocument());
    expect(restoreBackup).toHaveBeenCalledWith(expect.objectContaining({ name: "nightly.zip" }));
    expect(screen.getByRole("textbox", { name: "User-Agent" })).toHaveValue("restored-agent");
    expect(showNotice).toHaveBeenCalledWith("备份恢复成功");
    expect(screen.queryByText("已选择：nightly.zip")).not.toBeInTheDocument();
  });

  it("keeps fresh restored settings when the initial load resolves later", async () => {
    const user = userEvent.setup();
    let resolveInitial: ((settings: RuntimeSettingsInput) => void) | undefined;
    const initial = new Promise<RuntimeSettingsInput>((resolve) => {
      resolveInitial = resolve;
    });
    const staleSettings: RuntimeSettingsInput = {
      ...runtimeSettings,
      remote_defaults: { user_agent: "stale-agent", timeout_ms: 15000 },
    };
    const restoredSettings: RuntimeSettingsInput = {
      ...runtimeSettings,
      remote_defaults: { user_agent: "restored-agent", timeout_ms: 30000 },
    };
    const getRuntimeSettings = vi.fn()
      .mockReturnValueOnce(initial)
      .mockResolvedValueOnce(restoredSettings);
    mockApp({
      getRuntimeSettings,
      getVersion: vi.fn().mockResolvedValue({ name: "sandrone", version: "0.1.0" }),
      restoreBackup: vi.fn().mockResolvedValue(undefined),
    });
    renderSettingsRoute();
    await waitFor(() => expect(getRuntimeSettings).toHaveBeenCalledTimes(1));

    await chooseAndConfirmBackup(user);

    await waitFor(() => expect(getRuntimeSettings).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.getByRole("textbox", { name: "User-Agent" })).toHaveValue("restored-agent"));
    const freshCall = getRuntimeSettings.mock.calls[1];
    await act(async () => {
      resolveInitial?.(staleSettings);
      await initial;
    });

    expect(freshCall).toEqual([{ fresh: true }]);
    expect(screen.getByRole("textbox", { name: "User-Agent" })).toHaveValue("restored-agent");
  });

  it("ignores an initial settings failure after restored settings load", async () => {
    const user = userEvent.setup();
    const showNotice = vi.fn();
    let rejectInitial: ((reason: unknown) => void) | undefined;
    const initial = new Promise<RuntimeSettingsInput>((_resolve, reject) => {
      rejectInitial = reject;
    });
    const restoredSettings: RuntimeSettingsInput = {
      ...runtimeSettings,
      remote_defaults: { user_agent: "restored-agent", timeout_ms: 30000 },
    };
    const getRuntimeSettings = vi.fn()
      .mockReturnValueOnce(initial)
      .mockResolvedValueOnce(restoredSettings);
    mockApp({
      getRuntimeSettings,
      getVersion: vi.fn().mockResolvedValue({ name: "sandrone", version: "0.1.0" }),
      restoreBackup: vi.fn().mockResolvedValue(undefined),
    }, showNotice);
    renderSettingsRoute();
    await waitFor(() => expect(getRuntimeSettings).toHaveBeenCalledTimes(1));

    await chooseAndConfirmBackup(user);

    await waitFor(() => expect(screen.getByRole("textbox", { name: "User-Agent" })).toHaveValue("restored-agent"));
    await act(async () => {
      rejectInitial?.(new Error("stale initial load failed"));
      await initial.catch(() => undefined);
    });

    expect(showNotice).not.toHaveBeenCalledWith("运行默认值加载失败", "error");
  });

  it("keeps a failed restore selected and does not reload runtime settings", async () => {
    const user = userEvent.setup();
    const showNotice = vi.fn();
    const getRuntimeSettings = vi.fn().mockResolvedValue(runtimeSettings);
    const restoreBackup = vi.fn().mockRejectedValue(new Error("archive invalid"));
    mockApp({
      getRuntimeSettings,
      getVersion: vi.fn().mockResolvedValue({ name: "sandrone", version: "0.1.0" }),
      restoreBackup,
    }, showNotice);
    renderSettingsRoute();
    await waitFor(() => expect(getRuntimeSettings).toHaveBeenCalledTimes(1));

    await chooseAndConfirmBackup(user);

    expect(await screen.findByRole("alert")).toHaveTextContent("恢复失败。所选文件已保留，请重试。");
    expect(restoreBackup).toHaveBeenCalledTimes(1);
    expect(getRuntimeSettings).toHaveBeenCalledTimes(1);
    expect(showNotice).toHaveBeenCalledWith("archive invalid", "error");
    expect(screen.getByText("已选择：nightly.zip")).toBeInTheDocument();
  });

  it("still completes restore when the post-restore settings reload fails", async () => {
    const user = userEvent.setup();
    const showNotice = vi.fn();
    const getRuntimeSettings = vi.fn()
      .mockResolvedValueOnce(runtimeSettings)
      .mockRejectedValueOnce(new Error("reload unavailable"));
    const restoreBackup = vi.fn().mockResolvedValue(undefined);
    mockApp({
      getRuntimeSettings,
      getVersion: vi.fn().mockResolvedValue({ name: "sandrone", version: "0.1.0" }),
      restoreBackup,
    }, showNotice);
    renderSettingsRoute();
    await waitFor(() => expect(getRuntimeSettings).toHaveBeenCalledTimes(1));

    await chooseAndConfirmBackup(user);

    await waitFor(() => expect(getRuntimeSettings).toHaveBeenCalledTimes(2));
    expect(restoreBackup).toHaveBeenCalledTimes(1);
    expect(showNotice).toHaveBeenCalledWith("运行默认值加载失败", "error");
    expect(showNotice).toHaveBeenCalledWith("备份恢复成功");
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "恢复此备份？" })).not.toBeInTheDocument());
    expect(screen.queryByText("已选择：nightly.zip")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "恢复备份" })).toBeDisabled();
  });
});

function renderSettingsRoute() {
  return render(
    <I18nProvider>
      <SettingsRoute />
    </I18nProvider>,
  );
}

function LocaleSwitchHarness() {
  const { setLocale } = useI18n();
  return (
    <>
      <button type="button" onClick={() => setLocale("en-US")}>Switch locale</button>
      <SettingsRoute />
    </>
  );
}

function mockApp(
  client: Pick<ApiClient, "getRuntimeSettings" | "getVersion"> & Partial<Pick<ApiClient, "downloadBackup" | "restoreBackup">>,
  showNotice = vi.fn(),
) {
  vi.mocked(useSandrone).mockReturnValue({
    client,
    publicBaseUrl: "https://example.com",
    saveBaseUrl: vi.fn(),
    showNotice,
    signOut: vi.fn(),
    themeMode: "system",
    updateThemeMode: vi.fn(),
  } as unknown as SandroneContextValue);
}

async function chooseAndConfirmBackup(user: ReturnType<typeof userEvent.setup>) {
  const file = new File([new Uint8Array([80, 75, 3, 4])], "nightly.zip", { type: "application/zip" });
  await user.upload(screen.getByLabelText("选择备份 ZIP 文件"), file);
  await user.click(screen.getByRole("button", { name: "恢复备份" }));
  const dialog = screen.getByRole("dialog", { name: "恢复此备份？" });
  await user.click(within(dialog).getByRole("button", { name: "替换服务器数据" }));
}
