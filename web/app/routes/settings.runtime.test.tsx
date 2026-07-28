import { createMemoryRouter, RouterProvider } from "react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSandrone } from "~/core/provider/context";
import type { SandroneContextValue } from "~/core/provider/types";
import type { RuntimeSettingsInput } from "~/shared/api/client";
import { I18nProvider } from "~/shared/i18n/context";

import SettingsRuntimeRoute from "./settings.runtime";

vi.mock("~/core/provider/context", () => ({
  useSandrone: vi.fn(),
}));

const runtimeSettings: RuntimeSettingsInput = {
  remote_defaults: { user_agent: "sandrone/0.1.0", timeout_ms: 15000 },
  probe_defaults: {},
  cache_defaults: {},
};

describe("SettingsRuntimeRoute", () => {
  beforeEach(() => {
    vi.mocked(useSandrone).mockReset();
  });

  it("loads and saves runtime defaults on the runtime route", async () => {
    const user = userEvent.setup();
    const client = mockApp();

    renderSettingsRuntimeRoute();

    await waitFor(() => expect(client.getRuntimeSettings).toHaveBeenCalledTimes(1));
    expect(client.getVersion).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "保存运行默认值" }));
    await waitFor(() => expect(client.updateRuntimeSettings).toHaveBeenCalledTimes(1));
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
    getRuntimeSettings: vi.fn().mockResolvedValue(runtimeSettings),
    getVersion: vi.fn(),
    restoreBackup: vi.fn(),
    updateRuntimeSettings: vi.fn().mockResolvedValue(undefined),
  };
  vi.mocked(useSandrone).mockReturnValue({
    client,
    showNotice: vi.fn(),
  } as unknown as SandroneContextValue);
  return client;
}
