import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { HighlightedTextarea } from "./code-editor";

const source = 'const first = "node";\nconst second = "NODE";\nconst literal = "[node]";';
const dialogMethods = ["show", "showModal", "close"] as const;
const originalDialogDescriptors = new Map(dialogMethods.map((method) => [
  method,
  Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, method),
]));

beforeEach(() => {
  // jsdom has no dialog top layer; browser coverage verifies its native behavior.
  for (const method of dialogMethods) {
    Object.defineProperty(HTMLDialogElement.prototype, method, {
      configurable: true,
      value(this: HTMLDialogElement) {
        this.toggleAttribute("open", method !== "close");
      },
    });
  }
});

afterEach(() => {
  for (const method of dialogMethods) {
    const descriptor = originalDialogDescriptors.get(method);
    if (descriptor) Object.defineProperty(HTMLDialogElement.prototype, method, descriptor);
    else Reflect.deleteProperty(HTMLDialogElement.prototype, method);
  }
});

function renderEditor() {
  const onDirty = vi.fn();
  const onSave = vi.fn();
  render(
    <form
      aria-label="文件编辑"
      onChange={onDirty}
      onSubmit={(event) => {
        event.preventDefault();
        onSave();
      }}
    >
      <HighlightedTextarea
        defaultValue={source}
        label="脚本内容"
        language="javascript"
        minRows={8}
        name="content"
        showLineNumbers
      />
      <button type="submit">保存</button>
    </form>,
  );

  return {
    form: screen.getByRole("form", { name: "文件编辑" }) as HTMLFormElement,
    onDirty,
    onSave,
    textarea: screen.getByRole("textbox", { name: "脚本内容" }) as HTMLTextAreaElement,
  };
}

describe("code editor interactions", () => {
  it("searches literal text without modifying the document, dirtying the form, or submitting it", async () => {
    const user = userEvent.setup();
    const { form, onDirty, onSave, textarea } = renderEditor();

    await user.click(screen.getByRole("button", { name: "查找脚本内容" }));
    const search = screen.getByRole("searchbox", { name: "查找脚本内容" });
    await user.type(search, "[[node]");

    expect(search).toHaveValue("[node]");
    expect(screen.getByText("1 / 1")).toBeInTheDocument();
    await user.keyboard("{Enter}");
    expect(textarea).toHaveValue(source);
    expect(onDirty).not.toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
    expect([...new FormData(form).entries()]).toEqual([["content", source]]);
  });

  it("finds text without case sensitivity and cycles in both directions", async () => {
    const user = userEvent.setup();
    renderEditor();

    await user.click(screen.getByRole("button", { name: "查找脚本内容" }));
    const search = screen.getByRole("searchbox", { name: "查找脚本内容" });
    await user.type(search, "node");
    expect(screen.getByText("1 / 3")).toBeInTheDocument();

    await user.keyboard("{Shift>}{Enter}{/Shift}");
    expect(screen.getByText("3 / 3")).toBeInTheDocument();
    await user.keyboard("{Enter}");
    expect(screen.getByText("1 / 3")).toBeInTheDocument();
    await user.clear(search);
    await user.type(search, "missing");
    expect(screen.getByText("0 / 0")).toBeInTheDocument();
    await user.keyboard("{Enter}");
    expect(screen.getByText("0 / 0")).toBeInTheDocument();
  });

  it("refocuses an open search and selects its existing query when invoked from the textarea", async () => {
    const user = userEvent.setup();
    const { onDirty, onSave, textarea } = renderEditor();

    await user.click(screen.getByRole("button", { name: "查找脚本内容" }));
    const search = screen.getByRole("searchbox", { name: "查找脚本内容" }) as HTMLInputElement;
    await user.type(search, "node");
    await user.click(textarea);
    expect(textarea).toHaveFocus();

    await user.keyboard("{Control>}f{/Control}");

    expect(screen.getByRole("searchbox", { name: "查找脚本内容" })).toBe(search);
    expect(search).toHaveFocus();
    expect(search).toHaveValue("node");
    expect(search.selectionStart).toBe(0);
    expect(search.selectionEnd).toBe(4);
    await user.type(search, "first", { skipClick: true });
    expect(search).toHaveValue("first");
    expect(textarea).toHaveValue(source);
    expect(onDirty).not.toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });

  it.each(["\r\n", "\r"])("keeps search selections aligned with native textarea line endings for %j without dirtying the source", async (lineEnding) => {
    const user = userEvent.setup();
    const originalSource = ["const first = 1;", "const target = 2;", "const targetTwo = target;"].join(lineEnding);
    const onChange = vi.fn();
    const onDirty = vi.fn();
    render(
      <form aria-label="文件编辑" onChange={onDirty}>
        <HighlightedTextarea
          label="脚本内容"
          language="javascript"
          minRows={8}
          onChange={onChange}
          showLineNumbers
          value={originalSource}
        />
      </form>,
    );
    const textarea = screen.getByRole("textbox", { name: "脚本内容" }) as HTMLTextAreaElement;
    expect(onChange).not.toHaveBeenCalled();
    expect(onDirty).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "查找脚本内容" }));
    await user.type(screen.getByRole("searchbox", { name: "查找脚本内容" }), "target");
    expect(screen.getByRole("status")).toHaveTextContent("1 / 3");
    expect(textarea.value.slice(textarea.selectionStart, textarea.selectionEnd)).toBe("target");

    for (const [keys, count] of [
      ["{Enter}", "2 / 3"],
      ["{Enter}", "3 / 3"],
      ["{Enter}", "1 / 3"],
      ["{Shift>}{Enter}{/Shift}", "3 / 3"],
      ["{Shift>}{Enter}{/Shift}", "2 / 3"],
      ["{Shift>}{Enter}{/Shift}", "1 / 3"],
    ]) {
      await user.keyboard(keys);
      expect(screen.getByRole("status")).toHaveTextContent(count);
      expect(textarea.value.slice(textarea.selectionStart, textarea.selectionEnd)).toBe("target");
    }

    expect(onChange).not.toHaveBeenCalled();
    expect(onDirty).not.toHaveBeenCalled();
  });

  it("preserves the textarea and native form ownership when expanding and collapsing edited content", async () => {
    const user = userEvent.setup();
    const { form, onDirty, onSave, textarea } = renderEditor();
    textarea.focus();
    textarea.setSelectionRange(6, 11);

    await user.click(screen.getByRole("button", { name: "放大编辑" }));

    expect(screen.getByRole("textbox", { name: "脚本内容" })).toBe(textarea);
    expect(textarea.selectionStart).toBe(6);
    expect(textarea.selectionEnd).toBe(11);
    expect(textarea.form).toBe(form);
    expect([...new FormData(form).entries()]).toEqual([["content", source]]);
    expect(onDirty).not.toHaveBeenCalled();

    fireEvent.change(textarea, { target: { value: `${source}\nreturn first;` } });
    expect(onDirty).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "收起编辑" }));

    expect(screen.getByRole("textbox", { name: "脚本内容" })).toBe(textarea);
    expect(textarea).toHaveValue(`${source}\nreturn first;`);
    expect([...new FormData(form).entries()]).toEqual([["content", `${source}\nreturn first;`]]);
    expect(onSave).not.toHaveBeenCalled();
  });

  it("uses Escape to close search before collapsing and ignores IME composition Escape", async () => {
    const user = userEvent.setup();
    const { textarea } = renderEditor();

    await user.click(screen.getByRole("button", { name: "放大编辑" }));
    await user.click(screen.getByRole("button", { name: "查找脚本内容" }));
    const search = screen.getByRole("searchbox", { name: "查找脚本内容" });

    fireEvent.keyDown(search, { key: "Escape", isComposing: true, keyCode: 229 });
    expect(search).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "收起编辑" })).toBeInTheDocument();

    fireEvent.keyDown(search, { key: "Escape" });
    expect(screen.queryByRole("searchbox", { name: "查找脚本内容" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "收起编辑" })).toBeInTheDocument();

    fireEvent.keyDown(textarea, { key: "Escape", isComposing: true, keyCode: 229 });
    expect(screen.getByRole("button", { name: "收起编辑" })).toBeInTheDocument();
    fireEvent.keyDown(textarea, { key: "Escape" });
    expect(screen.getByRole("button", { name: "放大编辑" })).toBeInTheDocument();
  });

  it("keeps short parameter fields free of editing tools by default", () => {
    render(<HighlightedTextarea defaultValue="{}" label="参数" language="json" minRows={4} />);

    expect(screen.getByRole("textbox", { name: "参数" })).toHaveValue("{}");
    expect(screen.queryByRole("button", { name: "查找参数" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "放大编辑" })).not.toBeInTheDocument();
  });
});
