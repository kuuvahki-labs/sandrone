import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { defaultProjectSettings } from "~/features/settings/model/project-settings";

import { StartupSettingsSection } from "./startup-settings-section";

describe("StartupSettingsSection", () => {
  it("shows override sources without prefilling the redacted token", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const onTokenChange = vi.fn();
    render(
      <StartupSettingsSection
        overrides={{ "http.listen": "environment" }}
        token={undefined}
        value={{ ...defaultProjectSettings, http: { ...defaultProjectSettings.http, token_configured: true } }}
        onChange={onChange}
        onTokenChange={onTokenChange}
      />,
    );

    expect(screen.getByRole("textbox", { name: "监听地址" })).toHaveValue("127.0.0.1:1137");
    expect(screen.getByText("当前由 environment 覆盖")).toBeInTheDocument();
    expect(screen.getByLabelText("新管理 token")).toHaveValue("");

    fireEvent.change(screen.getByLabelText("新管理 token"), { target: { value: "replacement" } });
    expect(onTokenChange).toHaveBeenCalledWith("replacement");

    await user.click(screen.getByRole("switch", { name: "清除已保存的管理 token" }));
    expect(onTokenChange).toHaveBeenCalledWith("");
  });
});
