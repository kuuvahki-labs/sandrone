import { createMemoryRouter, RouterProvider } from "react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSandrone } from "~/core/provider/context";
import type { SandroneContextValue } from "~/core/provider/types";
import { defaultProjectSettings } from "~/features/settings/model/project-settings";
import { I18nProvider } from "~/shared/i18n/context";

import SettingsRuntimeRoute from "./settings.runtime";

vi.mock("~/core/provider/context", () => ({
  useSandrone: vi.fn(),
}));

describe("SettingsRuntimeRoute", () => {
  beforeEach(() => {
    vi.mocked(useSandrone).mockReset();
  });

  it("saves unified settings on the advanced route", async () => {
    const user = userEvent.setup();
    const { client, updateSettings } = mockApp();

    renderSettingsRuntimeRoute();

    expect(client.getVersion).not.toHaveBeenCalled();
    await user.click(screen.getByRole("switch", { name: "自动获取流量" }));
    await user.click(screen.getByRole("button", { name: "保存设置" }));
    await waitFor(() => expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
      subscriptions: { auto_load_traffic: true },
    })));
    expect(client.downloadBackup).not.toHaveBeenCalled();
  });

  it("returns to the settings overview", async () => {
    const user = userEvent.setup();
    mockApp();
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

function mockApp() {
  const client = {
    downloadBackup: vi.fn(),
    getVersion: vi.fn(),
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
