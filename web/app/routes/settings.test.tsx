import { createMemoryRouter, RouterProvider } from "react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSandrone } from "~/core/provider/context";
import type { SandroneContextValue } from "~/core/provider/types";
import { I18nProvider } from "~/shared/i18n/context";

import SettingsRoute from "./settings";

vi.mock("~/core/provider/context", () => ({
  useSandrone: vi.fn(),
}));

describe("SettingsRoute", () => {
  beforeEach(() => {
    vi.mocked(useSandrone).mockReset();
  });

  it("loads only version information on the settings overview", async () => {
    const client = mockApp();

    renderSettingsRoute();

    await waitFor(() => expect(client.getVersion).toHaveBeenCalledTimes(1));
    expect(client.getRuntimeSettings).not.toHaveBeenCalled();
    expect(client.downloadBackup).not.toHaveBeenCalled();
  });

  it.each([
    ["打开高级设置", "/settings/runtime", "runtime destination"],
    ["管理备份与恢复", "/settings/data", "data destination"],
  ])("navigates with %s", async (buttonName, destination, destinationText) => {
    const user = userEvent.setup();
    mockApp();
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

function renderSettingsRoute() {
  const router = createMemoryRouter([
    { path: "/settings", element: <SettingsRoute /> },
  ], { initialEntries: ["/settings"] });
  return render(
    <I18nProvider>
      <RouterProvider router={router} />
    </I18nProvider>,
  );
}

function mockApp() {
  const client = {
    downloadBackup: vi.fn(),
    getRuntimeSettings: vi.fn(),
    getVersion: vi.fn().mockResolvedValue({
      name: "sandrone",
      revision: "0123456789abcdef",
      version: "0.1.0",
    }),
    restoreBackup: vi.fn(),
    updateRuntimeSettings: vi.fn(),
  };
  vi.mocked(useSandrone).mockReturnValue({
    client,
    publicBaseUrl: "https://example.com",
    saveBaseUrl: vi.fn(),
    showNotice: vi.fn(),
    signOut: vi.fn(),
    themeMode: "system",
    updateThemeMode: vi.fn(),
  } as unknown as SandroneContextValue);
  return client;
}
