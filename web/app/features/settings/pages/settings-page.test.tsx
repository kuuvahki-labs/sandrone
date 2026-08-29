import { fireEvent, render, screen, waitForElementToBeRemoved, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "~/shared/i18n/context";

import { SettingsPage } from "./settings-page";

const noop = () => undefined;

describe("settings page", () => {
  it("shows RFC3339 build time for development and release builds", () => {
    const { rerender } = render(
      <SettingsPage
        buildTime="2026-08-30T03:15:42Z"
        localeMode="auto"
        publicBaseUrl="https://example.com"
        themeMode="system"
        version="dev"
        onOpenData={noop}
        onOpenService={noop}
        onLocaleMode={noop}
        onSaveBaseUrl={noop}
        onSignOut={noop}
        onThemeMode={noop}
      />,
    );
    expect(screen.getByText("dev (2026-08-30T03:15:42Z)")).toBeInTheDocument();

    rerender(
      <SettingsPage
        buildTime="2026-08-30T03:15:42Z"
        localeMode="auto"
        publicBaseUrl="https://example.com"
        revision="0123456789abcdef"
        themeMode="system"
        version="0.1.12"
        onOpenData={noop}
        onOpenService={noop}
        onLocaleMode={noop}
        onSaveBaseUrl={noop}
        onSignOut={noop}
        onThemeMode={noop}
      />,
    );
    expect(screen.getByText("v0.1.12 (0123456789ab; 2026-08-30T03:15:42Z)")).toBeInTheDocument();
  });

  it("updates theme mode and the public base URL", async () => {
    const user = userEvent.setup();
    const onSaveBaseUrl = vi.fn();
    const onThemeMode = vi.fn();
    render(
      <SettingsPage
        localeMode="auto"
        publicBaseUrl="https://example.com"
        themeMode="system"
        onOpenData={noop}
        onOpenService={noop}
        onLocaleMode={noop}
        onSaveBaseUrl={onSaveBaseUrl}
        onSignOut={noop}
        onThemeMode={onThemeMode}
      />,
    );

    const saveBaseUrl = screen.getByRole("button", { name: "保存服务地址" });

    await user.click(screen.getByRole("combobox", { name: "主题模式" }));
    await user.click(screen.getByRole("option", { name: "浅色" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Public Base URL" }), {
      target: { value: "https://public.example.test" },
    });
    await user.click(saveBaseUrl);

    expect(onThemeMode).toHaveBeenCalledWith("light");
    expect(onSaveBaseUrl).toHaveBeenCalledWith("https://public.example.test");
  });
  it("requests a locale-mode update through the language control", async () => {
    const user = userEvent.setup();
    const onLocaleMode = vi.fn();
    render(
      <I18nProvider>
        <SettingsPage
          localeMode="auto"
          publicBaseUrl="https://example.com"
          themeMode="system"
          onOpenData={noop}
          onOpenService={noop}
          onLocaleMode={onLocaleMode}
          onSaveBaseUrl={noop}
          onSignOut={noop}
          onThemeMode={noop}
        />
      </I18nProvider>,
    );

    await user.click(screen.getByRole("combobox", { name: "语言" }));
    await user.click(screen.getByRole("option", { name: "English" }));

    expect(onLocaleMode).toHaveBeenCalledWith("en-US");
  });
  it("keeps the service settings heading outside native button content and supports keyboard activation", async () => {
    const user = userEvent.setup();
    const onOpenService = vi.fn();
    render(
      <SettingsPage
        localeMode="auto"
        publicBaseUrl="https://example.com"
        themeMode="system"
        onOpenData={noop}
        onOpenService={onOpenService}
        onLocaleMode={noop}
        onSaveBaseUrl={noop}
        onSignOut={noop}
        onThemeMode={noop}
      />,
    );

    const advanced = screen.getByRole("button", { name: "打开服务设置" });
    const heading = screen.getByRole("heading", { name: "服务设置", level: 3 });
    expect(advanced.tagName).toBe("DIV");
    expect(heading.closest("button")).toBeNull();

    advanced.focus();
    await user.keyboard("{Enter}");

    expect(onOpenService).toHaveBeenCalledTimes(1);
  });
  it("cancels sign-out before confirming it exactly once", async () => {
    const user = userEvent.setup();
    const onSignOut = vi.fn();
    render(
      <SettingsPage
        localeMode="auto"
        publicBaseUrl="https://example.com"
        themeMode="system"
        onOpenData={noop}
        onOpenService={noop}
        onLocaleMode={noop}
        onSaveBaseUrl={noop}
        onSignOut={onSignOut}
        onThemeMode={noop}
      />,
    );

    await user.click(screen.getByRole("button", { name: "退出登录" }));

    let dialog = screen.getByRole("dialog", { name: "退出登录？" });
    await user.click(within(dialog).getByRole("button", { name: "取消" }));
    expect(onSignOut).not.toHaveBeenCalled();
    await waitForElementToBeRemoved(dialog);

    await user.click(screen.getByRole("button", { name: "退出登录" }));
    dialog = screen.getByRole("dialog", { name: "退出登录？" });
    await user.click(within(dialog).getByRole("button", { name: "退出登录" }));

    expect(onSignOut).toHaveBeenCalledTimes(1);
  });
});
