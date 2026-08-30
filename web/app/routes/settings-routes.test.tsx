import { createMemoryRouter, RouterProvider } from "react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSandrone } from "~/core/provider/context";
import type { SandroneContextValue } from "~/core/provider/types";
import { defaultProjectSettings } from "~/features/settings/model/project-settings";
import { UICapabilityProvider } from "~/shared/capabilities/context";
import { I18nProvider } from "~/shared/i18n/context";

import SettingsRoute from "./settings";
import SettingsDataRoute from "./settings.data";
import SettingsServiceRoute from "./settings.service";

vi.mock("~/core/provider/context", () => ({
  useSandrone: vi.fn(),
}));

beforeEach(() => {
  vi.mocked(useSandrone).mockReset();
});

describe("SettingsRoute", () => {
  it("loads only version information on the settings overview", async () => {
    const client = mockSettingsOverviewApp();

    renderSettingsOverviewRoute();

    await waitFor(() => expect(client.getVersion).toHaveBeenCalledTimes(1));
    expect(client.downloadBackup).not.toHaveBeenCalled();
  });

  it.each([
    ["打开服务设置", "/settings/service", "service destination"],
    ["打开数据管理", "/settings/data", "data destination"],
  ])("navigates with %s", async (buttonName, destination, destinationText) => {
    const user = userEvent.setup();
    mockSettingsOverviewApp();
    const router = createMemoryRouter([
      { path: "/settings", element: <SettingsRoute /> },
      { path: destination, element: <p>{destinationText}</p> },
    ], { initialEntries: ["/settings"] });

    render(
      <I18nProvider>
        <RouterProvider router={router} />
      </I18nProvider>,
    );
    await user.click(screen.getByRole("button", { name: buttonName }));

    expect(await screen.findByText(destinationText)).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(destination);
  });
});

describe("SettingsDataRoute", () => {
  it("does not fetch unrelated settings resources on load", () => {
    const client = mockSettingsDataApp();

    renderSettingsDataRoute();

    expect(client.getVersion).not.toHaveBeenCalled();
    expect(client.clearCache).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "下载备份" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "清空缓存" })).toBeInTheDocument();
  });

  it("returns to the settings overview", async () => {
    const user = userEvent.setup();
    mockSettingsDataApp();
    const router = createMemoryRouter([
      { path: "/settings", element: <p>settings destination</p> },
      { path: "/settings/data", element: <SettingsDataRoute /> },
    ], { initialEntries: ["/settings/data"] });
    render(
      <I18nProvider>
        <RouterProvider router={router} />
      </I18nProvider>,
    );

    await user.click(screen.getByRole("button", { name: "返回" }));

    expect(await screen.findByText("settings destination")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/settings");
  });
});

describe("SettingsServiceRoute", () => {
  it("saves unified settings on the service settings route", async () => {
    const user = userEvent.setup();
    const { client, updateSettings } = mockSettingsServiceApp();

    renderSettingsServiceRoute();

    await waitFor(() => expect(client.getVersion).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("textbox", { name: "User-Agent" })).toHaveAttribute("placeholder", "sandrone/0.2.0");
    await user.click(screen.getByRole("switch", { name: "在订阅列表显示" }));
    await user.click(screen.getByRole("button", { name: "保存设置" }));
    await waitFor(() => expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
      subscriptions: { auto_load_traffic: true },
    })));
    expect(client.downloadBackup).not.toHaveBeenCalled();
  });

  it("runs scheduled refresh after saving when requested", async () => {
    const user = userEvent.setup();
    const settings = {
      ...defaultProjectSettings,
      scheduled_refresh: {
        ...defaultProjectSettings.scheduled_refresh,
        enabled: true,
        targets: [{ kind: "subscription" as const, name: "provider" }],
      },
    };
    const { client, showNotice, updateSettings } = mockSettingsServiceApp(settings);

    renderSettingsServiceRoute(true);
    const runAfterSave = await screen.findByRole("checkbox", { name: "保存后立即执行一次" });
    await user.click(runAfterSave);
    await user.click(screen.getByRole("button", { name: "保存设置" }));

    await waitFor(() => expect(client.runScheduledRefresh).toHaveBeenCalledTimes(1));
    expect(updateSettings.mock.invocationCallOrder[0]).toBeLessThan(client.runScheduledRefresh.mock.invocationCallOrder[0]);
    expect(client.getScheduledRefreshStatus).toHaveBeenCalledWith({ fresh: true });
    expect(showNotice).toHaveBeenCalledWith("已启动一次更新");
  });

  it("reports a saved setting when the immediate run cannot start", async () => {
    const user = userEvent.setup();
    const settings = {
      ...defaultProjectSettings,
      scheduled_refresh: {
        ...defaultProjectSettings.scheduled_refresh,
        enabled: true,
        targets: [{ kind: "file" as const, name: "client.yaml" }],
      },
    };
    const { client, showNotice, updateSettings } = mockSettingsServiceApp(settings);
    client.runScheduledRefresh.mockRejectedValueOnce(new Error("unavailable"));

    renderSettingsServiceRoute(true);
    const runAfterSave = await screen.findByRole("checkbox", { name: "保存后立即执行一次" });
    await user.click(runAfterSave);
    await user.click(screen.getByRole("button", { name: "保存设置" }));

    await waitFor(() => expect(showNotice).toHaveBeenCalledWith("设置已保存，但立即更新启动失败", "error"));
    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(runAfterSave).toBeChecked();
  });

  it("returns to the settings overview", async () => {
    const user = userEvent.setup();
    mockSettingsServiceApp();
    const router = createMemoryRouter([
      { path: "/settings", element: <p>settings destination</p> },
      { path: "/settings/service", element: <SettingsServiceRoute /> },
    ], { initialEntries: ["/settings/service"] });
    render(
      <I18nProvider>
        <RouterProvider router={router} />
      </I18nProvider>,
    );

    await user.click(screen.getByRole("button", { name: "返回" }));

    expect(await screen.findByText("settings destination")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/settings");
  });
});

function renderSettingsOverviewRoute() {
  const router = createMemoryRouter([
    { path: "/settings", element: <SettingsRoute /> },
  ], { initialEntries: ["/settings"] });
  return render(
    <I18nProvider>
      <RouterProvider router={router} />
    </I18nProvider>,
  );
}

function renderSettingsDataRoute() {
  const router = createMemoryRouter([
    { path: "/settings/data", element: <SettingsDataRoute /> },
  ], { initialEntries: ["/settings/data"] });
  return render(
    <I18nProvider>
      <RouterProvider router={router} />
    </I18nProvider>,
  );
}

function renderSettingsServiceRoute(schedulerEnabled = false) {
  const router = createMemoryRouter([
    { path: "/settings/service", element: <SettingsServiceRoute /> },
  ], { initialEntries: ["/settings/service"] });
  return render(
    <I18nProvider>
      <UICapabilityProvider value={{
        capabilities: [{ key: "scheduler.enabled", enabled: schedulerEnabled }],
        loaded: true,
        hasFeature: (key) => key === "scheduler.enabled" && schedulerEnabled,
        getFeature: (key) => key === "scheduler.enabled" ? { key, enabled: schedulerEnabled } : undefined,
      }}>
        <RouterProvider router={router} />
      </UICapabilityProvider>
    </I18nProvider>,
  );
}

function mockSettingsOverviewApp() {
  const client = {
    downloadBackup: vi.fn(),
    getVersion: vi.fn().mockResolvedValue({
      name: "sandrone",
      revision: "0123456789abcdef",
      version: "0.1.0",
    }),
    restoreBackup: vi.fn(),
  };
  vi.mocked(useSandrone).mockReturnValue({
    client,
    effectiveSettings: defaultProjectSettings,
    publicBaseUrl: "https://example.com",
    saveBaseUrl: vi.fn(),
    showNotice: vi.fn(),
    signOut: vi.fn(),
    settings: defaultProjectSettings,
    updateSettings: vi.fn(),
  } as unknown as SandroneContextValue);
  return client;
}

function mockSettingsDataApp() {
  const client = {
    clearCache: vi.fn().mockResolvedValue(undefined),
    downloadBackup: vi.fn(),
    getVersion: vi.fn(),
    restoreBackup: vi.fn(),
  };
  vi.mocked(useSandrone).mockReturnValue({
    client,
    reloadSettings: vi.fn(),
    showNotice: vi.fn(),
  } as unknown as SandroneContextValue);
  return client;
}

function mockSettingsServiceApp(settings = defaultProjectSettings) {
  const client = {
    downloadBackup: vi.fn(),
    getScheduledRefreshStatus: vi.fn().mockResolvedValue({
      enabled: false,
      running: false,
      last_success_count: 0,
      last_failure_count: 0,
      skipped_count: 0,
    }),
    getVersion: vi.fn().mockResolvedValue({
      name: "sandrone",
      revision: "fedcba9876543210",
      version: "0.2.0",
    }),
    listFiles: vi.fn().mockResolvedValue({ files: [] }),
    listSubscriptions: vi.fn().mockResolvedValue({ subscriptions: [] }),
    restoreBackup: vi.fn(),
    runScheduledRefresh: vi.fn().mockResolvedValue({ accepted: true }),
  };
  const updateSettings = vi.fn().mockResolvedValue(undefined);
  const showNotice = vi.fn();
  vi.mocked(useSandrone).mockReturnValue({
    client,
    restartRequired: [],
    settings,
    settingsOverrides: {},
    showNotice,
    updateSettings,
  } as unknown as SandroneContextValue);
  return { client, showNotice, updateSettings };
}
