import { createMemoryRouter, RouterProvider } from "react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSandrone } from "~/core/provider/context";
import type { SandroneContextValue } from "~/core/provider/types";
import { defaultProjectSettings } from "~/features/settings/model/project-settings";
import { I18nProvider } from "~/shared/i18n/context";

import SettingsRoute from "./settings";
import SettingsDataRoute from "./settings.data";
import SettingsRuntimeRoute from "./settings.runtime";

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
    ["打开高级设置", "/settings/runtime", "runtime destination"],
    ["管理备份与恢复", "/settings/data", "data destination"],
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
  it("does not fetch settings resources before a data operation", () => {
    const client = mockSettingsDataApp();

    renderSettingsDataRoute();

    expect(client.getVersion).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "下载备份" })).toBeInTheDocument();
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

describe("SettingsRuntimeRoute", () => {
  it("saves unified settings on the advanced route", async () => {
    const user = userEvent.setup();
    const { client, updateSettings } = mockSettingsRuntimeApp();

    renderSettingsRuntimeRoute();

    await waitFor(() => expect(client.getVersion).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("textbox", { name: "User-Agent" })).toHaveAttribute("placeholder", "sandrone/0.2.0");
    await user.click(screen.getByRole("switch", { name: "自动获取流量" }));
    await user.click(screen.getByRole("button", { name: "保存设置" }));
    await waitFor(() => expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
      subscriptions: { auto_load_traffic: true },
    })));
    expect(client.downloadBackup).not.toHaveBeenCalled();
  });

  it("returns to the settings overview", async () => {
    const user = userEvent.setup();
    mockSettingsRuntimeApp();
    const router = createMemoryRouter([
      { path: "/settings", element: <p>settings destination</p> },
      { path: "/settings/runtime", element: <SettingsRuntimeRoute /> },
    ], { initialEntries: ["/settings/runtime"] });
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

function renderSettingsRuntimeRoute() {
  const router = createMemoryRouter([
    { path: "/settings/runtime", element: <SettingsRuntimeRoute /> },
  ], { initialEntries: ["/settings/runtime"] });
  return render(
    <I18nProvider>
      <RouterProvider router={router} />
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

function mockSettingsRuntimeApp() {
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
  };
  const updateSettings = vi.fn().mockResolvedValue(undefined);
  vi.mocked(useSandrone).mockReturnValue({
    client,
    restartRequired: [],
    settings: defaultProjectSettings,
    settingsOverrides: {},
    showNotice: vi.fn(),
    updateSettings,
  } as unknown as SandroneContextValue);
  return { client, updateSettings };
}
