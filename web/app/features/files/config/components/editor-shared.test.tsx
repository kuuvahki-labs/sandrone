import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ConfigRowSummary, RowOrderActions, SectionIssues } from "./editor-shared";

afterEach(() => {
  localStorage.removeItem("sandrone.locale");
});

describe("shared config editor rows", () => {
  it("renders long summaries as wrapping plain text", () => {
    const { container } = render(
      <ConfigRowSummary
        primary="Developer Tools With A Very Long Name"
        secondary={["select", "Auto", "订阅节点", "DIRECT"]}
      />,
    );

    expect(container.querySelector(".MuiChip-root")).not.toBeInTheDocument();
    expect(screen.getByText("Developer Tools With A Very Long Name")).toHaveClass("break-words", "whitespace-normal");
    expect(container).toHaveTextContent("select · Auto · 订阅节点 · DIRECT");
  });

  it("keeps desktop controls and mobile menu callbacks accessible", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    const onDown = vi.fn();
    const onUp = vi.fn();
    render(
      <RowOrderActions
        deleteLabel="删除代理组 Proxy"
        downDisabled={false}
        downLabel="下移代理组 Proxy"
        mobileMenuLabel="Proxy 更多操作"
        onDelete={onDelete}
        onDown={onDown}
        onUp={onUp}
        upDisabled
        upLabel="上移代理组 Proxy"
      />,
    );

    expect(screen.getByRole("button", { name: "上移代理组 Proxy" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "下移代理组 Proxy" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "Proxy 更多操作" }));
    expect(screen.getByRole("menuitem", { name: "上移代理组 Proxy" })).toHaveAttribute("aria-disabled", "true");
    await user.click(screen.getByRole("menuitem", { name: "下移代理组 Proxy" }));
    expect(onDown).toHaveBeenCalledTimes(1);
    expect(onUp).not.toHaveBeenCalled();
    expect(onDelete).not.toHaveBeenCalled();
  });

  it("renders a driver-owned issue message descriptor without knowing its code", () => {
    localStorage.setItem("sandrone.locale", "en-US");
    render(
      <SectionIssues issues={[{
        code: "driver_reserved_name",
        itemId: "group-0",
        message: "untranslated fallback",
        messageKey: "files.config.issueShadowrocketNodeReserved",
        messageParams: { reference: "DIRECT" },
        section: "groups",
        severity: "error",
      }]} />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Rendered Shadowrocket node name “DIRECT” conflicts with a built-in policy.",
    );
    expect(screen.getByRole("alert")).not.toHaveTextContent("untranslated fallback");
  });
});
