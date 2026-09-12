import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { createProcessorDraftId } from "~/shared/processors/model";
import type { ProcessorDetail } from "~/shared/resources/types";

import { ProcessorEditorList } from "./processor-editor-list";

const originalClipboard = Object.getOwnPropertyDescriptor(navigator, "clipboard");
const existing: ProcessorDetail = { type: "script", stage: "file", params: { code: "existing" } };
const incoming: ProcessorDetail[] = [
  { name: "Imported", type: "script", stage: "nodes", enabled: false, params: { code: "external", empty: [], nested: { keep: true } }, future: { value: 1 } },
  { name: "Imported", type: "script", enabled: true, params: {} },
];

afterEach(() => {
  if (originalClipboard) Object.defineProperty(navigator, "clipboard", originalClipboard);
  else Reflect.deleteProperty(navigator, "clipboard");
});

describe("processor clipboard transfer", () => {
  it("copies the current unsaved list and immediately appends imports without changing their definitions", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    setClipboard({ writeText, readText: vi.fn().mockResolvedValue(JSON.stringify(incoming)) });
    const onDirty = vi.fn();
    render(<Harness onDirty={onDirty} />);

    await user.click(screen.getByRole("button", { name: "导入处理器" }));
    expect(currentProcessors()).toEqual([existing, ...incoming]);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "导入处理器" }));
    expect(currentProcessors()).toEqual([existing, ...incoming, ...incoming]);
    await user.click(screen.getByRole("button", { name: "复制处理器" }));
    expect(JSON.parse(writeText.mock.calls[0][0])).toEqual(currentProcessors());
    expect(onDirty).toHaveBeenCalledTimes(2);

    const importedCard = screen.getAllByRole("group", { name: "处理器 Imported" })[0];
    fireEvent.change(within(importedCard).getByRole("textbox", { name: "Code" }), { target: { value: "edited" } });
    expect(currentProcessors()[1]).toMatchObject({ stage: "nodes", enabled: false, future: { value: 1 }, params: { code: "edited", nested: { keep: true } } });
  });

  it("appends to the latest list when editing continues during clipboard reading", async () => {
    const user = userEvent.setup();
    let resolveRead!: (value: string) => void;
    setClipboard({ readText: vi.fn(() => new Promise<string>((resolve) => { resolveRead = resolve; })) });
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "导入处理器" }));
    await user.click(screen.getByRole("button", { name: "添加处理器" }));
    const before = currentProcessors();
    await act(async () => resolveRead(JSON.stringify(incoming)));
    expect(currentProcessors()).toEqual([...before, ...incoming]);
  });

  it("offers selectable JSON only when automatic copying fails", async () => {
    const user = userEvent.setup();
    setClipboard({ writeText: vi.fn().mockRejectedValue(new Error("denied")) });
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "复制处理器" }));
    const dialog = screen.getByRole("dialog", { name: "复制处理器" });
    const text = within(dialog).getByRole("textbox", { name: "处理器 JSON" });
    expect(text).toHaveAttribute("readonly");
    expect(JSON.parse((text as HTMLTextAreaElement).value)).toEqual([existing]);
  });

  it("falls back to manual paste and rejects malformed batches without appending a partial result", async () => {
    const user = userEvent.setup();
    setClipboard(undefined);
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "导入处理器" }));
    const dialog = screen.getByRole("dialog", { name: "导入处理器" });
    const text = within(dialog).getByRole("textbox", { name: "处理器 JSON" });
    fireEvent.change(text, { target: { value: JSON.stringify([...incoming, { type: 7 }]) } });
    await user.click(within(dialog).getByRole("button", { name: /^导入$/ }));
    expect(currentProcessors()).toEqual([existing]);
    expect(within(dialog).getByRole("alert")).toBeInTheDocument();
    fireEvent.change(text, { target: { value: JSON.stringify({ resource_type: "file", resource: { name: "external", processors: incoming } }) } });
    await user.click(within(dialog).getByRole("button", { name: /^导入$/ }));
    expect(currentProcessors()).toEqual([existing, ...incoming]);
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });
});

function setClipboard(value: unknown) {
  Object.defineProperty(navigator, "clipboard", { configurable: true, value });
}

function currentProcessors(): ProcessorDetail[] {
  return JSON.parse(document.querySelector<HTMLInputElement>('input[name="processors"]')!.value) as ProcessorDetail[];
}

function Harness({ onDirty }: { onDirty?: () => void }) {
  return <ProcessorEditorList
    createDraftId={() => createProcessorDraftId("test")}
    defaultParams={() => ({ code: "new" })}
    defaultType="script"
    defaultValue={[existing]}
    paramsEditor={({ draft, onChange }) => <label>Code<input value={String(draft.params.code ?? "")} onChange={(event) => onChange({ code: event.target.value })} /></label>}
    processorOptions={[{ value: "script", label: "脚本" }]}
    serializeDraft={(draft) => ({ type: draft.type, stage: "file", params: draft.params })}
    onDirty={onDirty}
  />;
}
