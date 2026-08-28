import { useState } from "react";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CodeBlock, HighlightedTextarea } from "./code-editor";
import { SelectField } from "./form-fields";
import { PageHeader } from "./page";
import { ProbeURLField } from "./probe-url-field";
import { CreateSpeedDial } from "./resource-list";
import { SnapshotCachePolicyField } from "./snapshot-cache-policy-field";

const originalClipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");

afterEach(() => {
  if (originalClipboardDescriptor) {
    Object.defineProperty(navigator, "clipboard", originalClipboardDescriptor);
  } else {
    Reflect.deleteProperty(navigator, "clipboard");
  }
  vi.restoreAllMocks();
});

describe("shared UI primitives", () => {
  it("renders select group headers in option order without changing ungrouped selects", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <SelectField
        label="Preset"
        onChange={vi.fn()}
        options={[
          { value: "script", label: "Script" },
          { value: "block-stun", label: "Block STUN", group: "Privacy" },
          { value: "block-quic", label: "Block QUIC", group: "Privacy" },
          { value: "tailscale", label: "Tailscale", group: "Platform" },
        ]}
        value="script"
      />,
    );

    await user.click(screen.getByRole("combobox", { name: "Preset" }));
    const groupedListbox = screen.getByRole("listbox");
    expect([...groupedListbox.children].map((child) => child.textContent)).toEqual([
      "Script",
      "Privacy",
      "Block STUN",
      "Block QUIC",
      "Platform",
      "Tailscale",
    ]);
    await user.keyboard("{Escape}");

    rerender(
      <SelectField
        label="Preset"
        onChange={vi.fn()}
        options={[
          { value: "script", label: "Script" },
          { value: "merge", label: "Merge" },
        ]}
        value="merge"
      />,
    );
    expect(screen.getByRole("combobox", { name: "Preset" })).toHaveTextContent("Merge");
    await user.click(screen.getByRole("combobox", { name: "Preset" }));
    const ungroupedListbox = screen.getByRole("listbox");
    expect([...ungroupedListbox.children].map((child) => child.textContent)).toEqual(["Script", "Merge"]);
  });

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

    expect(screen.getByRole("button", { name: "预览" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "更多操作" })).not.toBeInTheDocument();
  });

  it("explains why a disabled page-header action is unavailable", async () => {
    const user = userEvent.setup();
    const onPreview = vi.fn();
    render(
      <PageHeader
        label=""
        secondaryActions={[{
          accessibleLabel: "预览文件",
          disabled: true,
          disabledReason: "请先保存修改，再预览已保存版本",
          label: "预览",
          onSelect: onPreview,
        }]}
        title="编辑文件"
      />,
    );

    const preview = screen.getByRole("button", { name: "预览文件" });
    expect(preview).toHaveAttribute("aria-disabled", "true");
    expect(preview.querySelector("button")).toBeDisabled();

    await user.tab();
    expect(preview).toHaveFocus();
    expect(preview).toHaveAccessibleDescription("请先保存修改，再预览已保存版本");
    await user.hover(preview);
    expect(await screen.findByRole("tooltip")).toHaveTextContent("请先保存修改，再预览已保存版本");
    await user.click(preview);
    expect(onPreview).not.toHaveBeenCalled();
  });

  it("puts multiple secondary actions in the mobile compact overflow menu", async () => {
    vi.stubGlobal("matchMedia", vi.fn((query: string) => ({
      matches: query === "(max-width:819px)",
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })));
    const user = userEvent.setup();
    const onPreview = vi.fn();

    try {
      render(
        <PageHeader
          label=""
          primaryAction={{ label: "保存", variant: "contained" }}
          secondaryActions={[
            {
              accessibleLabel: "预览文件",
              disabled: true,
              disabledReason: "请先保存修改，再预览已保存版本",
              label: "预览",
              onSelect: onPreview,
            },
            { label: "分享", onSelect: vi.fn() },
          ]}
          sticky
          title="编辑文件"
        />,
      );

      await user.click(screen.getByRole("button", { name: "更多操作" }));
      const preview = screen.getByRole("menuitem", { name: "预览文件" });
      expect(preview).toHaveAttribute("aria-disabled", "true");
      expect(preview).toHaveAccessibleDescription("请先保存修改，再预览已保存版本");
      await user.hover(preview);
      expect(await screen.findByRole("tooltip")).toHaveTextContent("请先保存修改，再预览已保存版本");
      await user.click(preview);
      expect(onPreview).not.toHaveBeenCalled();
      expect(screen.getByRole("menuitem", { name: "分享" })).toBeInTheDocument();
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("renders long code content with Prism tokens and a copy action", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn(async (_value: string) => undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    render(<CodeBlock label="最终文件内容" language="json" value={"{\n  \"name\": \"node\"\n}\n"} />);

    expect(screen.getByText("json")).toBeInTheDocument();
    const code = screen.getByRole("region", { name: "最终文件内容" }).querySelector("code");
    expect(code).toBeInTheDocument();
    expect(code).toHaveTextContent("\"name\": \"node\"");
    expect(code?.querySelector(".token.property")).toHaveTextContent("\"name\"");
    expect(code!.tagName.toLowerCase()).toBe("code");

    await user.click(screen.getByRole("button", { name: "复制最终文件内容" }));

    expect(writeText).toHaveBeenCalledWith("{\n  \"name\": \"node\"\n}\n");
    expect(await screen.findByRole("button", { name: "已复制最终文件内容" })).toBeInTheDocument();
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

  it("syncs highlight translation to native textarea scrolling", () => {
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

    expect(textarea).toHaveAttribute("wrap", "off");

    Object.defineProperty(textarea, "scrollLeft", { configurable: true, value: 96, writable: true });
    Object.defineProperty(textarea, "scrollTop", { configurable: true, value: 120, writable: true });
    fireEvent.scroll(textarea);

    expect(highlightLayer?.scrollLeft).toBe(0);
    expect(highlightLayer?.scrollTop).toBe(0);
    expect((highlightContent as HTMLElement | null)?.style.transform).toBe("translate(-96px, -120px)");
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

  it("selects a create action from the speed dial", async () => {
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
  });
});

describe("ProbeURLField", () => {
  it("offers the ordered probe providers and selects their complete URL", async () => {
    const user = userEvent.setup();
    render(<ProbeURLHarness initialValue="http://www.gstatic.com/generate_204" />);

    const input = screen.getByRole("combobox", { name: "URL" });
    await user.click(input);
    await user.keyboard("{ArrowDown}");

    const listbox = await screen.findByRole("listbox");
    expect(within(listbox).getAllByRole("option").map((option) => option.textContent)).toEqual([
      "Googlehttp://www.gstatic.com/generate_204",
      "Applehttp://captive.apple.com/hotspot-detect.html",
      "Cloudflarehttp://cp.cloudflare.com/generate_204",
      "Microsofthttp://www.msftconnecttest.com/connecttest.txt",
      "华为http://connectivitycheck.platform.hicloud.com/generate_204",
    ]);

    await user.click(within(listbox).getByRole("option", {
      name: "Apple http://captive.apple.com/hotspot-detect.html",
    }));
    expect(input).toHaveValue("http://captive.apple.com/hotspot-detect.html");
    expect(screen.getByRole("status", { name: "当前 URL" }))
      .toHaveTextContent("http://captive.apple.com/hotspot-detect.html");
  });

  it("keeps a URL outside the preset catalog", async () => {
    const user = userEvent.setup();
    render(<ProbeURLHarness initialValue="https://probe.example.test/ok" />);

    const input = screen.getByRole("combobox", { name: "URL" });
    expect(input).toHaveValue("https://probe.example.test/ok");
    await user.clear(input);
    await user.type(input, "https://custom.example.test/health");
    await user.tab();

    expect(input).toHaveValue("https://custom.example.test/health");
    expect(screen.getByRole("status", { name: "当前 URL" }))
      .toHaveTextContent("https://custom.example.test/health");
  });
});

describe("SnapshotCachePolicyField", () => {
  it("preserves explicit disable and exposes custom TTL input on demand", () => {
    render(<SnapshotCachePolicyField defaultValue={0} />);

    const policy = screen.getByRole("combobox", { name: "快照缓存" });
    expect(policy).toHaveValue("disabled");
    expect(screen.queryByRole("spinbutton", { name: "缓存时间（秒）" })).not.toBeInTheDocument();

    fireEvent.change(policy, { target: { value: "custom" } });
    expect(screen.getByRole("spinbutton", { name: "缓存时间（秒）" })).toBeInTheDocument();
  });
});

function ProbeURLHarness({ initialValue }: { initialValue: string }) {
  const [value, setValue] = useState(initialValue);
  return (
    <>
      <ProbeURLField label="URL" value={value} onChange={setValue} />
      <output aria-label="当前 URL">{value}</output>
    </>
  );
}
