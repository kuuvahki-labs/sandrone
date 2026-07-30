import { MemoryRouter } from "react-router";
import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ShellFrame } from "./shell";

describe("ShellFrame", () => {
  it("renders accessible desktop and mobile destination navigation", () => {
    render(
      <MemoryRouter>
        <ShellFrame activePath="/subscriptions">
          <p>content</p>
        </ShellFrame>
      </MemoryRouter>,
    );

    expect(screen.getByText("content")).toBeInTheDocument();
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

});
