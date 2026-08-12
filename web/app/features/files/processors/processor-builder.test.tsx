import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ProcessorDetail } from "~/shared/resources/types";

import { FileMergeParamsEditor } from "./merge-params-editor";
import { FileProcessorBuilder } from "./processor-builder";

const typedRuleSourceKinds = ["mihomo", "sing-box", "shadowrocket"] as const;

describe("FileProcessorBuilder", () => {
  it("serializes script and merge processors through the hidden form contract", () => {
    render(
      <FileProcessorBuilder
        kind="mihomo"
        defaultValue={[
          { name: "Remote script", type: "script", stage: "file", params: { source: { type: "remote", remote: { url: "https://example.com/process.js" } }, timeout_ms: 2000 } },
          { type: "merge", stage: "file", params: { mode: "yaml_override", content: "dns:\n  enable: true" } },
        ]}
      />,
    );

    expect(currentProcessors()).toEqual([
      { name: "Remote script", type: "script", stage: "file", params: { source: { type: "remote", remote: { url: "https://example.com/process.js" } }, timeout_ms: 2000 } },
      { type: "merge", stage: "file", params: { mode: "yaml_override", content: "dns:\n  enable: true" } },
    ]);
  });

  it.each(typedRuleSourceKinds)("offers the rule source rewrite preset for %s", async (kind) => {
    const user = userEvent.setup();
    render(<FileProcessorBuilder kind={kind} />);

    await user.click(screen.getByRole("combobox", { name: "类型" }));
    expect(screen.getByRole("option", { name: "GitHub 规则源地址替换" })).toBeInTheDocument();
  });

  it("does not offer the rule source rewrite shortcut for static files", async () => {
    const user = userEvent.setup();
    render(<FileProcessorBuilder kind="static" />);

    await user.click(screen.getByRole("combobox", { name: "类型" }));
    expect(screen.queryByRole("option", { name: "GitHub 规则源地址替换" })).not.toBeInTheDocument();
  });

  it("appends one editable standard script and preserves it across kinds", async () => {
    const user = userEvent.setup();
    const existing: ProcessorDetail = {
      name: "Existing",
      type: "script",
      stage: "file",
      params: { source: { type: "inline", content: "function main(input) { return input; }" } },
    };
    const { rerender } = render(<FileProcessorBuilder kind="mihomo" defaultValue={[existing]} />);

    await selectMuiOption(
      user,
      screen.getByRole("combobox", { name: "类型" }),
      "GitHub 规则源地址替换",
    );
    await user.click(screen.getByRole("button", { name: "添加处理器" }));

    const added = currentProcessors();
    expect(added).toHaveLength(2);
    expect(added[0]).toEqual(existing);
    expect(added[1]).toMatchObject({
      name: "GitHub Rule Source Rewrite",
      type: "script",
      stage: "file",
      params: { source: { type: "inline" } },
    });
    expect(screen.getAllByRole("textbox", { name: "内联脚本" })).toHaveLength(2);

    await user.click(screen.getByRole("button", { name: "添加处理器" }));
    expect(currentProcessors()).toHaveLength(2);

    rerender(<FileProcessorBuilder key="static" kind="static" defaultValue={[added[1]]} />);
    expect(currentProcessors()).toEqual([added[1]]);
  });

  it("groups managed presets and reports Tailnet Share dependencies in one update", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    render(<FileProcessorBuilder kind="mihomo" />);

    await user.click(screen.getByRole("combobox", { name: "Type" }));
    expect(screen.getByRole("option", { name: "Tailscale coexistence" })).toBeInTheDocument();
    expect(screen.getByText("Tailscale")).toBeInTheDocument();
    await user.click(screen.getByRole("option", { name: "Tailnet proxy sharing" }));
    await user.click(screen.getByRole("button", { name: "Add processor" }));

    expect(currentProcessors().map((processor) => processor.name)).toEqual([
      "TUN",
      "Tailscale 共存",
      "Tailnet 代理共享",
    ]);
    expect(screen.getByRole("alert")).toHaveTextContent("Added dependencies: TUN, Tailscale coexistence");
  });

  it("preserves unsupported processors byte-for-byte in their original order", () => {
    const processors: ProcessorDetail[] = [
      { type: "future", stage: "file", params: { nested: { keep: true }, empty: [] }, future: { version: 2 } },
      { type: "script", stage: "file", params: { source: { type: "inline", content: "function main(input) { return input; }" } } },
    ];

    render(<FileProcessorBuilder kind="mihomo" defaultValue={processors} />);

    expect(currentProcessors()).toEqual(processors);
  });

  it("starts a Shadowrocket merge processor in ini_override mode", async () => {
    const user = userEvent.setup();
    render(<FileProcessorBuilder kind="shadowrocket" />);

    await selectMuiOption(user, screen.getByRole("combobox", { name: "类型" }), "合并");
    await user.click(screen.getByRole("button", { name: "添加处理器" }));

    expect(currentProcessors()).toEqual([{
      type: "merge",
      stage: "file",
      params: { mode: "ini_override" },
    }]);
  });
});

describe("FileMergeParamsEditor", () => {
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
});

function currentProcessors(): ProcessorDetail[] {
  const input = document.querySelector<HTMLInputElement>('input[name="processors"]');
  if (!input) throw new Error("expected serialized processors input");
  return JSON.parse(input.value) as ProcessorDetail[];
}

async function selectMuiOption(
  user: ReturnType<typeof userEvent.setup>,
  combobox: HTMLElement,
  optionName: string,
) {
  await user.click(combobox);
  const listbox = await screen.findByRole("listbox");
  await user.click(within(listbox).getByRole("option", { name: optionName }));
}
