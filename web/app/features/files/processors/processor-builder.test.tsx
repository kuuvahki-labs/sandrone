import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { ProcessorDetail } from "~/shared/resources/types";

import { FileProcessorBuilder } from "./processor-builder";

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

  it("adds preset dependencies once and removes Mihomo presets for another kind", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<FileProcessorBuilder kind="mihomo" />);

    await selectMuiOption(user, screen.getByRole("combobox", { name: "类型" }), "Tailnet 代理共享");
    await user.click(screen.getByRole("button", { name: "添加处理器" }));
    expect(currentProcessors().map((processor) => processor.name)).toEqual([
      "TUN",
      "Tailscale 共存",
      "Tailnet 代理共享",
    ]);

    await user.click(screen.getByRole("button", { name: "添加处理器" }));
    expect(currentProcessors()).toHaveLength(3);

    const presets = currentProcessors();
    rerender(<FileProcessorBuilder key="sing-box" kind="sing-box" defaultValue={presets} />);
    expect(currentProcessors()).toEqual([]);
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
