import { fireEvent, render, screen, waitForElementToBeRemoved, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "~/shared/i18n/context";

import { SettingsPage } from "./settings-page";

const noop = () => undefined;

describe("settings page", () => {
  it("uses outlined floating labels for overview fields", () => {
    render(
      <SettingsPage
        localeMode="auto"
        publicBaseUrl="https://example.com"
        themeMode="system"
        onOpenData={noop}
        onOpenRuntime={noop}
        onLocaleMode={noop}
        onSaveBaseUrl={noop}
        onSignOut={noop}
        onThemeMode={noop}
      />,
    );

    expect(screen.getByText("主题模式", { selector: "label.MuiInputLabel-root" })).toBeInTheDocument();
    expect(screen.getByText("语言", { selector: "label.MuiInputLabel-root" })).toBeInTheDocument();
    expect(screen.getByText("Public Base URL", { selector: "label.MuiInputLabel-root" })).toBeInTheDocument();
  });

  it("renders a progressive settings overview with working form and navigation actions", async () => {
    const user = userEvent.setup();
    const onOpenData = vi.fn();
    const onOpenRuntime = vi.fn();
    const onSaveBaseUrl = vi.fn();
    const onThemeMode = vi.fn();
    render(
      <SettingsPage
        localeMode="auto"
        publicBaseUrl="https://example.com"
        themeMode="system"
        onOpenData={onOpenData}
        onOpenRuntime={onOpenRuntime}
        onLocaleMode={noop}
        onSaveBaseUrl={onSaveBaseUrl}
        onSignOut={noop}
        onThemeMode={onThemeMode}
      />,
    );

    expect(screen.getByRole("heading", { name: "设置" })).toBeInTheDocument();
    expect(screen.getByText("管理界面偏好与服务配置")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "外观与语言" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "服务连接" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "数据与账户" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "关于 Sandrone" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "GitHub" })).toHaveAttribute("href", "https://github.com/kuuvahki-labs/sandrone");
    const pageRoot = screen.getByRole("heading", { name: "设置" }).closest("section");
    expect(pageRoot).toHaveClass("grid", "gap-6");
    expect(pageRoot).not.toHaveClass("max-w-[760px]");
    const pageHeader = screen.getByRole("heading", { name: "设置" }).closest("header");
    expect(pageHeader).toHaveClass("MuiPaper-root", "MuiPaper-outlined");
    expect(pageHeader?.parentElement).toBe(pageRoot);
    expect(screen.queryByTestId("PaletteOutlinedIcon")).not.toBeInTheDocument();
    expect(screen.queryByTestId("StorageOutlinedIcon")).not.toBeInTheDocument();

    const advanced = screen.getByRole("button", { name: "打开高级设置" });
    expect(advanced).toHaveTextContent("高级设置");
    expect(advanced).not.toHaveTextContent(/远程请求|缓存|测活/);
    expect(screen.queryByRole("heading", { name: "运行默认值" })).not.toBeInTheDocument();
    expect(screen.queryByRole("note")).not.toBeInTheDocument();

    const saveBaseUrl = screen.getByRole("button", { name: "保存服务地址" });
    expect(saveBaseUrl).toHaveTextContent("保存");
    expect(screen.getByRole("combobox", { name: "主题模式" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Public Base URL" })).toHaveValue("https://example.com");
    expect(screen.getByText("仅保存在当前浏览器。")).toBeInTheDocument();

    await user.click(advanced);
    await user.click(screen.getByRole("button", { name: "管理备份与恢复" }));
    await user.click(screen.getByRole("combobox", { name: "主题模式" }));
    await user.click(screen.getByRole("option", { name: "浅色" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Public Base URL" }), {
      target: { value: "https://public.example.test" },
    });
    await user.click(saveBaseUrl);

    expect(onOpenRuntime).toHaveBeenCalledTimes(1);
    expect(onOpenData).toHaveBeenCalledTimes(1);
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
        localeMode="auto"
        publicBaseUrl="https://example.com"
        revision={revision}
        themeMode="system"
        version={version}
        onOpenData={noop}
        onOpenRuntime={noop}
        onLocaleMode={noop}
        onSaveBaseUrl={noop}
        onSignOut={noop}
        onThemeMode={noop}
      />,
    );

    expect(screen.getByText(expected)).toBeInTheDocument();
  });
  it("renders settings language controls in English", () => {
    localStorage.setItem("sandrone.locale", "en-US");

    render(
      <SettingsPage
        localeMode="en-US"
        publicBaseUrl="https://example.com"
        themeMode="system"
        onOpenData={noop}
        onOpenRuntime={noop}
        onLocaleMode={noop}
        onSaveBaseUrl={noop}
        onSignOut={noop}
        onThemeMode={noop}
      />,
    );

    expect(screen.getByRole("heading", { name: "Settings" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Language" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Theme mode" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Data and account" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Manage backup and restore" })).toBeInTheDocument();
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
          onOpenRuntime={noop}
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
  it("keeps the advanced heading outside native button content and supports keyboard activation", async () => {
    const user = userEvent.setup();
    const onOpenRuntime = vi.fn();
    render(
      <SettingsPage
        localeMode="auto"
        publicBaseUrl="https://example.com"
        themeMode="system"
        onOpenData={noop}
        onOpenRuntime={onOpenRuntime}
        onLocaleMode={noop}
        onSaveBaseUrl={noop}
        onSignOut={noop}
        onThemeMode={noop}
      />,
    );

    const advanced = screen.getByRole("button", { name: "打开高级设置" });
    const heading = screen.getByRole("heading", { name: "高级设置", level: 3 });
    expect(advanced.tagName).toBe("DIV");
    expect(heading.closest("button")).toBeNull();

    advanced.focus();
    await user.keyboard("{Enter}");

    expect(onOpenRuntime).toHaveBeenCalledTimes(1);
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
        onOpenRuntime={noop}
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
