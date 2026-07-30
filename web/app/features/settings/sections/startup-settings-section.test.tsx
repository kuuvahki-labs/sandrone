import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { defaultProjectSettings } from "~/features/settings/model/project-settings";

import { StartupSettingsSection } from "./startup-settings-section";

describe("StartupSettingsSection", () => {
  it("groups service and MCP settings without persistent authentication controls", () => {
    const onChange = vi.fn();
    render(
      <StartupSettingsSection
        overrides={{ "http.listen": "environment" }}
        value={defaultProjectSettings}
        onChange={onChange}
      />,
    );

    expect(screen.getByRole("heading", { name: "服务" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "MCP" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "监听地址" })).toHaveValue("127.0.0.1:1137");
    expect(screen.getByText("当前由 environment 覆盖")).toBeInTheDocument();
    expect(screen.queryByLabelText("新管理 token")).not.toBeInTheDocument();
    expect(screen.queryByRole("switch", { name: "强制要求管理 token" })).not.toBeInTheDocument();
    expect(screen.queryByRole("switch", { name: "清除已保存的管理 token" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "MCP 传输方式" })).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole("textbox", { name: "MCP 路径" }), { target: { value: "/agent" } });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({
      mcp: expect.objectContaining({ path: "/agent" }),
    }));
  });
});
