import { readFileSync } from "node:fs";

import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { CodeBlock, HighlightedTextarea } from "./code-editor";
import { DiscardChangesDialog } from "./dialogs";
import { EmptyState, ResourceLoadingCard, SnackbarStack } from "./feedback";
import { Metric, PageHeader } from "./page";
import { CreateSpeedDial } from "./resource-list";

describe("shared UI primitives", () => {
  it("collapses sticky page headers after their scroll sentinel leaves the viewport", () => {
    const onBack = vi.fn();
    const { container } = render(
      <PageHeader
        backAction={{ label: "返回", onSelect: onBack }}
        description="very-long-resource-name.yaml"
        label="文件"
        primaryAction={{ label: "保存", type: "submit", variant: "contained" }}
        sticky
        title="编辑文件"
      />,
    );
    const sentinel = container.querySelector("[aria-hidden]");
    const header = screen.getByRole("banner");
    expect(header).toHaveAttribute("data-page-header-compact", "false");
    expect(screen.getByText("very-long-resource-name.yaml")).toBeInTheDocument();

    vi.spyOn(sentinel!, "getBoundingClientRect").mockReturnValue({ bottom: -1 } as DOMRect);
    fireEvent.scroll(window);

    expect(header).toHaveAttribute("data-page-header-compact", "true");
    expect(screen.queryByText("very-long-resource-name.yaml")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "返回" }));
    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it("keeps one secondary action beside the primary action when compact", () => {
    const onPreview = vi.fn();
    const { container } = render(
      <PageHeader
        label="文件"
        primaryAction={{ label: "保存", variant: "contained" }}
        secondaryActions={[{ label: "预览", onSelect: onPreview }]}
        sticky
        title="编辑文件"
      />,
    );

    expect(screen.getByRole("button", { name: "保存" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "预览" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "更多操作" })).not.toBeInTheDocument();

    const sentinel = container.querySelector("[aria-hidden]");
    vi.spyOn(sentinel!, "getBoundingClientRect").mockReturnValue({ bottom: -1 } as DOMRect);
    fireEvent.scroll(window);

    expect(screen.getByRole("button", { name: "预览" })).toHaveClass("MuiButton-sizeSmall");
    expect(screen.getByRole("button", { name: "保存" })).toHaveClass("MuiButton-sizeSmall");
    expect(screen.queryByRole("button", { name: "更多操作" })).not.toBeInTheDocument();
  });

  it("renders metrics, empty states, and notices through MUI components", () => {
    render(
      <>
        <Metric label="订阅" value={3} />
        <EmptyState title="还没有订阅" />
        <SnackbarStack notices={[
          { id: 1, message: "已保存", severity: "success" },
          { id: 2, message: "请求失败", severity: "error" },
          { id: 3, message: "需要先创建文件", severity: "warning" },
        ]} />
      </>,
    );

    expect(screen.getByText("订阅")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "还没有订阅" })).toBeInTheDocument();
    expect(screen.getByText("已保存").closest('[role="status"]')).not.toBeNull();

    expect(screen.getByText("订阅").closest(".MuiPaper-root")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "还没有订阅" }).closest(".MuiCard-root")).toBeInTheDocument();
    expect(screen.getByText("已保存")).toBeInTheDocument();
    expect(screen.getByText("请求失败").closest(".MuiAlert-root")).toHaveClass("MuiAlert-colorError");
    expect(screen.getByText("需要先创建文件").closest(".MuiAlert-root")).toHaveClass("MuiAlert-colorWarning");
  });

  it("renders the shared resource loading card with the edit-route markup contract", () => {
    const { container } = render(<ResourceLoadingCard title="正在加载文件定义" />);

    expect(container.firstElementChild).toHaveClass("grid", "gap-6");
    expect(screen.getByRole("article")).toHaveClass("MuiCard-root");
    expect(screen.getByRole("heading", { name: "正在加载文件定义", level: 2 })).toHaveClass("MuiTypography-h5");
  });

  it("renders long code content with Prism tokens and a copy action", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn(async (_value: string) => undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    render(<CodeBlock label="最终文件内容" language="json" value={"{\n  \"name\": \"node\"\n}\n"} />);

    expect(screen.getByRole("region", { name: "最终文件内容" })).toHaveClass("MuiPaper-root", "overflow-hidden");
    expect(screen.getByText("json")).toBeInTheDocument();
    const code = screen.getByRole("region", { name: "最终文件内容" }).querySelector("code");
    expect(code).toBeInTheDocument();
    expect(code).toHaveTextContent("\"name\": \"node\"");
    expect(code?.querySelector(".token.property")).toHaveTextContent("\"name\"");
    expect(code!.tagName.toLowerCase()).toBe("code");
    expect(code!.closest("pre")).toHaveClass("max-h-[min(70vh,640px)]", "overflow-auto", "whitespace-pre");
    expect(code!.closest("pre")).not.toHaveClass("whitespace-pre-wrap");
    expect(code!.closest("pre")).not.toHaveClass("break-words");

    await user.click(screen.getByRole("button", { name: "复制最终文件内容" }));

    expect(writeText).toHaveBeenCalledWith("{\n  \"name\": \"node\"\n}\n");
    expect(await screen.findByRole("button", { name: "已复制最终文件内容" })).toBeInTheDocument();
  });

  it("fills available height only when requested", () => {
    render(<CodeBlock fillHeight label="最终文件内容" language="text" value={"line 1\nline 2"} />);

    const block = screen.getByRole("region", { name: "最终文件内容" });
    const pre = block.querySelector("pre");
    expect(block).toHaveClass("flex", "min-h-0", "flex-1", "flex-col");
    expect(pre).toHaveClass("min-h-0", "flex-1", "overflow-auto");
    expect(pre).not.toHaveClass("max-h-[min(70vh,640px)]");
  });

  it("renders json-diff lines with visible prefixes, line state, and JSON tokens", () => {
    render(<CodeBlock label="节点对比 Diff" language="json-diff" value={'-   "name": "old"\n+   "name": "new"\n  "type": "ss"'} />);

    const diffBlock = screen.getByRole("region", { name: "节点对比 Diff" });
    const removed = diffBlock.querySelector('[data-diff-line="removed"]');
    const added = diffBlock.querySelector('[data-diff-line="added"]');
    const unchanged = diffBlock.querySelector('[data-diff-line="unchanged"]');

    expect(removed?.textContent).toContain('-   "name": "old"');
    expect(added?.textContent).toContain('+   "name": "new"');
    expect(unchanged?.textContent).toContain('  "type": "ss"');
    expect(removed).toHaveClass("code-diff-line-removed");
    expect(added).toHaveClass("code-diff-line-added");
    expect(removed).toHaveClass("min-w-full", "w-max");
    expect(added).toHaveClass("min-w-full", "w-max");
    expect(unchanged).toHaveClass("min-w-full", "w-max");
    expect(added?.querySelector(".token.property")).toHaveTextContent('"name"');
  });

  it("tokenizes INI sections, keys, and values instead of only labeling the code block", () => {
    render(<CodeBlock label="Shadowrocket 配置" language="ini" value={"[General]\nipv6 = false\n# note"} />);

    const code = screen.getByRole("region", { name: "Shadowrocket 配置" }).querySelector("code");

    expect(code).toHaveClass("language-ini");
    expect(code?.querySelector(".token.selector")).toHaveTextContent("[General]");
    expect(code?.querySelector(".token.constant")).toHaveTextContent("ipv6");
    expect([...code!.querySelectorAll(".token.attr-value")].map((token) => token.textContent).join(""))
      .toBe("= false");
    expect(code?.querySelector(".token.comment")).toHaveTextContent("# note");
  });

  it("keeps highlighted textarea input, highlight layer, and controlled updates in sync", async () => {
    const user = userEvent.setup();
    function Harness() {
      const [value, setValue] = useState('const input = {"name":"node"};');
      return <HighlightedTextarea label="脚本内容" language="javascript" minRows={4} value={value} onChange={(event) => setValue(event.target.value)} />;
    }

    const { container } = render(<Harness />);
    const textarea = screen.getByRole("textbox", { name: "脚本内容" });
    const highlightLayer = container.querySelector("[data-highlighted-textarea-layer]");

    expect(textarea).toHaveValue('const input = {"name":"node"};');
    expect(highlightLayer).toHaveTextContent('const input = {"name":"node"};');
    expect(highlightLayer?.querySelector(".token.keyword")).toHaveTextContent("const");

    await user.clear(textarea);
    await user.type(textarea, "return input;");

    expect(textarea).toHaveValue("return input;");
    expect(highlightLayer).toHaveTextContent("return input;");
  });

  it("renders synchronized line numbers when requested", async () => {
    const user = userEvent.setup();
    function Harness() {
      const [value, setValue] = useState("line 1\nline 2");
      return <HighlightedTextarea showLineNumbers label="内容" language="text" minRows={3} value={value} onChange={(event) => setValue(event.target.value)} />;
    }

    const { container } = render(<Harness />);
    const textarea = screen.getByRole("textbox", { name: "内容" });
    const lineNumbers = container.querySelector("[data-highlighted-textarea-lines]");

    expect(lineNumbers).toBeInTheDocument();
    expect(lineNumbers?.querySelector('[data-line-number="1"]')).toHaveTextContent("1");
    expect(lineNumbers?.querySelector('[data-line-number="2"]')).toHaveTextContent("2");
    expect(lineNumbers?.querySelector('[data-line-number="3"]')).not.toBeInTheDocument();

    await user.clear(textarea);
    await user.type(textarea, "first\nsecond\nthird");

    expect(lineNumbers?.querySelector('[data-line-number="3"]')).toHaveTextContent("3");
  });

  it("keeps highlighted textarea label actions on the label row", () => {
    render(
      <HighlightedTextarea
        label="内容"
        labelAction={<button type="button">复制内容</button>}
        language="text"
        minRows={3}
        value="ss://example"
      />,
    );

    const label = screen.getByText("内容");
    const action = screen.getByRole("button", { name: "复制内容" });

    expect(label.closest("[data-highlighted-textarea-label-row]")).toContainElement(action);
  });

  it("keeps the native textarea as the only horizontal scrollbar surface", () => {
    const { container } = render(
      <HighlightedTextarea
        showLineNumbers
        label="内容"
        language="javascript"
        minRows={3}
        value={"const veryLongValue = 'abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz';"}
      />,
    );

    const textarea = screen.getByRole("textbox", { name: "内容" });
    const highlightLayer = container.querySelector("[data-highlighted-textarea-layer]");
    const highlightContent = container.querySelector("[data-highlighted-textarea-content]");
    const highlightedCode = highlightLayer?.querySelector("code");

    expect(textarea).toHaveAttribute("wrap", "off");
    expect(textarea).toHaveClass("highlighted-textarea-input", "overflow-auto", "whitespace-pre", "font-mono", "leading-6");
    expect(highlightLayer).toHaveClass("pointer-events-none", "overflow-hidden", "whitespace-pre", "font-mono", "leading-6", "bottom-0");
    expect(highlightContent).toHaveClass("min-w-max");
    expect(highlightedCode).toHaveClass("max-w-none", "overflow-visible", "font-mono", "leading-6");
    expect(container.querySelector("[data-highlighted-textarea-scrollbar]")).not.toBeInTheDocument();

    Object.defineProperty(textarea, "scrollLeft", { configurable: true, value: 96, writable: true });
    Object.defineProperty(textarea, "scrollTop", { configurable: true, value: 120, writable: true });
    fireEvent.scroll(textarea);

    expect(highlightLayer?.scrollLeft).toBe(0);
    expect(highlightLayer?.scrollTop).toBe(0);
    expect((highlightContent as HTMLElement | null)?.style.transform).toBe("translate(-96px, -120px)");
  });

  it("keeps selected textarea text transparent while preserving selection feedback", () => {
    render(
      <HighlightedTextarea
        showLineNumbers
        label="内容"
        language="text"
        minRows={3}
        value={"ss://selected-text-should-not-double-render.example"}
      />,
    );

    const textarea = screen.getByRole("textbox", { name: "内容" });
    const baseCss = readFileSync("app/styles/base.css", "utf8");

    expect(textarea).toHaveClass("highlighted-textarea-input");
    expect(baseCss).toContain(".highlighted-textarea-input {");
    expect(baseCss).toContain("-webkit-text-fill-color: transparent;");
    expect(baseCss).toContain(".highlighted-textarea-input::selection");
    expect(baseCss).toContain("background-color:");
  });

  it("submits a named highlighted textarea through native form data", () => {
    render(
      <form aria-label="local source">
        <HighlightedTextarea defaultValue="ss://example" label="订阅内容" language="text" name="source_input" minRows={3} />
      </form>,
    );

    const form = screen.getByRole("form", { name: "local source" }) as HTMLFormElement;

    expect(screen.getByRole("textbox", { name: "订阅内容" })).toHaveAttribute("name", "source_input");
    expect(new FormData(form).get("source_input")).toBe("ss://example");
  });

  it("renders a MUI speed dial for create actions", async () => {
    const user = userEvent.setup();
    const onRemote = vi.fn();
    const onLocal = vi.fn();

    render(
      <CreateSpeedDial
        actions={[
          { ariaLabel: "新建远程订阅", icon: <span aria-hidden>R</span>, label: "远程", onSelect: onRemote },
          { ariaLabel: "新建本地订阅", icon: <span aria-hidden>L</span>, label: "本地", onSelect: onLocal },
        ]}
        ariaLabel="新建订阅"
      />,
    );

    await user.click(screen.getByRole("button", { name: "新建订阅" }));
    await user.click(await screen.findByRole("menuitem", { name: "新建远程订阅" }));

    expect(onRemote).toHaveBeenCalledTimes(1);
    expect(onLocal).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "新建订阅" }).closest(".MuiSpeedDial-root")).toBeInTheDocument();
  });

  it("renders discard confirmation through a MUI dialog", () => {
    render(<DiscardChangesDialog onCancel={() => undefined} onConfirm={() => undefined} />);

    expect(screen.getByRole("dialog", { name: "放弃修改？" })).toHaveClass("MuiDialog-paper");
    expect(screen.getByRole("button", { name: "继续编辑" })).toHaveClass("MuiButton-root");
    expect(screen.getByRole("button", { name: "放弃修改" })).toHaveTextContent("放弃");
  });
});
