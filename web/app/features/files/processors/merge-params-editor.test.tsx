import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";

import { FileMergeParamsEditor } from "./merge-params-editor";

it("keeps yaml_override syntax help keyboard focusable", async () => {
  const user = userEvent.setup();
  render(
    <FileMergeParamsEditor
      kind="mihomo"
      params={{ mode: "yaml_override", content: "rules+:\n  - MATCH,DIRECT" }}
      onChange={vi.fn()}
    />,
  );

  await user.tab();
  expect(screen.getByRole("combobox", { name: "合并模式" })).toHaveFocus();
  await user.tab();
  expect(screen.getByRole("button", { name: "覆写语法说明" })).toHaveFocus();
});

it("lets sing-box merge processors switch between JSON override modes", async () => {
  const user = userEvent.setup();
  const onChange = vi.fn();
  render(
    <FileMergeParamsEditor
      kind="sing-box"
      params={{ mode: "json_override", content: "{\"route\":{}}" }}
      onChange={onChange}
    />,
  );

  const mode = screen.getByRole("combobox", { name: "合并模式" });
  expect(mode).toHaveTextContent("JSON 可组合覆写");
  await user.click(mode);
  await user.click(screen.getByRole("option", { name: "JSON 覆盖" }));
  expect(onChange).toHaveBeenCalledWith({ mode: "json_overlay" });
});

it("offers only INI override with INI highlighting for Shadowrocket", () => {
  const onChange = vi.fn();
  const { container } = render(
    <FileMergeParamsEditor
      kind="shadowrocket"
      params={{ mode: "ini_override", content: "[Rule+]\nFINAL,DIRECT" }}
      onChange={onChange}
    />,
  );

  const mode = screen.getByRole("combobox", { name: "合并模式" });
  expect(mode).toHaveTextContent("INI 覆写");
  expect(screen.queryByText("YAML 覆盖")).not.toBeInTheDocument();
  expect(screen.getByRole("textbox", { name: "内容" })).toHaveAttribute("placeholder", expect.stringContaining("[General]"));
  expect(container.querySelector('[data-highlighted-textarea="ini"]')).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "覆写语法说明" })).toHaveAttribute("aria-label", "覆写语法说明");
});
