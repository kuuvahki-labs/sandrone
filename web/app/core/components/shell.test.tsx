import { MemoryRouter } from "react-router";
import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { I18nProvider } from "~/shared/i18n/context";

import { ShellFrame } from "./shell";

describe("ShellFrame", () => {
  it("renders MUI drawer destination navigation without a top app bar", () => {
    const { container } = render(
      <MemoryRouter>
        <ShellFrame activePath="/subscriptions">
          <p>content</p>
        </ShellFrame>
      </MemoryRouter>,
    );

    expect(container.querySelector(".app-shell")).not.toBeInTheDocument();
    expect(container.querySelector(".nav-rail")).not.toBeInTheDocument();
    expect(container.querySelector(".desktop-drawer")).toBeInTheDocument();
    expect(container.querySelector(".bottom-nav")).toBeInTheDocument();
    expect(container.querySelector(".top-app-bar")).not.toBeInTheDocument();
    expect(screen.getByText("content")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Sandrone logo" })).toHaveAttribute("src", "/brand/sandrone-logo-48.png");
    const drawer = screen.getByRole("navigation", { name: "桌面导航" });
    expect(within(drawer).getByRole("link", { name: "订阅" })).toHaveAttribute("href", "/subscriptions");
    expect(within(drawer).getByRole("link", { name: "文件" })).toHaveAttribute("href", "/files");
    expect(within(drawer).getByRole("link", { name: "分享" })).toHaveAttribute("href", "/shares");
    expect(within(drawer).getByRole("link", { name: "我的" })).toHaveAttribute("href", "/settings");

    const bottomNav = screen.getByRole("navigation", { name: "底部导航" });
    const bottomLink = within(bottomNav).getByRole("link", { name: "订阅" });
    expect(bottomLink).toHaveAttribute("href", "/subscriptions");
    expect(bottomLink).not.toHaveAttribute("role", "button");
  });

  it("keeps edit and preview flows focused by omitting the mobile bottom navigation", () => {
    render(
      <MemoryRouter>
        <ShellFrame activePath="/subscriptions/remote/provider/edit">
          <p>edit content</p>
        </ShellFrame>
      </MemoryRouter>,
    );

    expect(screen.getByText("edit content")).toBeInTheDocument();
    expect(screen.queryByLabelText("底部导航")).not.toBeInTheDocument();

    render(
      <MemoryRouter>
        <ShellFrame activePath="/subscriptions/remote/provider/preview">
          <p>preview content</p>
        </ShellFrame>
      </MemoryRouter>,
    );

    expect(screen.getByText("preview content")).toBeInTheDocument();
    expect(screen.queryByLabelText("底部导航")).not.toBeInTheDocument();
  });

  it("renders English navigation labels when the locale is en-US", () => {
    localStorage.setItem("sandrone.locale", "en-US");

    render(
      <MemoryRouter>
        <I18nProvider>
          <ShellFrame activePath="/files">
            <p>content</p>
          </ShellFrame>
        </I18nProvider>
      </MemoryRouter>,
    );

    const drawer = screen.getByRole("navigation", { name: "Desktop navigation" });
    expect(within(drawer).getByRole("link", { name: "Subscriptions" })).toHaveAttribute("href", "/subscriptions");
    expect(within(drawer).getByRole("link", { name: "Files" })).toHaveAttribute("href", "/files");
    expect(within(drawer).getByRole("link", { name: "Shares" })).toHaveAttribute("href", "/shares");
    expect(within(drawer).getByRole("link", { name: "Me" })).toHaveAttribute("href", "/settings");
    expect(screen.getByRole("navigation", { name: "Bottom navigation" })).toBeInTheDocument();
  });
});
