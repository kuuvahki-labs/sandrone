import { createMemoryRouter, RouterProvider } from "react-router";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSandrone } from "~/core/provider/context";
import type { SandroneContextValue } from "~/core/provider/types";
import { I18nProvider } from "~/shared/i18n/context";

import SettingsDataRoute from "./settings.data";

vi.mock("~/core/provider/context", () => ({
  useSandrone: vi.fn(),
}));

describe("SettingsDataRoute", () => {
  beforeEach(() => {
    vi.mocked(useSandrone).mockReset();
  });

  it("does not fetch settings resources before a data operation", () => {
    const client = mockApp();

    renderSettingsDataRoute();

    expect(client.getVersion).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "下载备份" })).toBeInTheDocument();
  });

  it("returns to the settings overview", async () => {
    const user = userEvent.setup();
    mockApp();
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

function mockApp() {
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
