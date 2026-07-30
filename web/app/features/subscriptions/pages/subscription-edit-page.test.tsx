import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  noop,
  remoteSubscriptionDefinition,
  subscriptions,
} from "~/features/subscriptions/test-data";

import { SubscriptionEditPage } from "./subscription-edit-page";

const saveSuccess = (_form: FormData) => true;

describe("SubscriptionEditPage", () => {
  it("wires form changes, saves, and shares while rejecting duplicate submissions", async () => {
    const user = userEvent.setup();
    let resolveSave!: (persisted: boolean) => void;
    const onSave = vi.fn((_form: FormData) => new Promise<boolean>((resolve) => {
      resolveSave = resolve;
    }));
    const onShare = vi.fn();
    render(
      <SubscriptionEditPage
        definition={remoteSubscriptionDefinition}
        item={subscriptions[0]}
        onBack={noop}
        onSave={onSave}
        onShare={onShare}
        sources={subscriptions}
      />,
    );

    const share = screen.getByRole("button", { name: "分享订阅" });
    expect(share).toHaveTextContent(/^分享$/);
    expect(share).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "远程" }));
    expect(share).toBeEnabled();

    const displayName = screen.getByRole("textbox", { name: "显示名称" });
    fireEvent.change(displayName, { target: { value: "updated" } });
    expect(share).toBeDisabled();

    const form = displayName.closest("form");
    if (!form) throw new Error("expected subscription edit form");
    fireEvent.submit(form);
    fireEvent.submit(form);
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "保存订阅" })).toBeDisabled();
    await act(async () => resolveSave(true));
    expect(share).toBeEnabled();

    await user.click(share);
    expect(onShare).toHaveBeenCalledTimes(1);
  });
  it("keeps sharing disabled when save reports an in-band failure", async () => {
    const user = userEvent.setup();
    render(
      <SubscriptionEditPage
        definition={remoteSubscriptionDefinition}
        item={subscriptions[0]}
        onBack={noop}
        onSave={() => false}
        onShare={noop}
        sources={subscriptions}
      />,
    );

    const share = screen.getByRole("button", { name: "分享订阅" });
    fireEvent.change(screen.getByRole("textbox", { name: "显示名称" }), {
      target: { value: "invalid" },
    });
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    expect(share).toBeDisabled();
  });
  it("switches remote subscription edits to local content without stale remote fields", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn(saveSuccess);
    render(<SubscriptionEditPage item={subscriptions[0]} onBack={noop} onSave={onSave} onShare={noop} definition={remoteSubscriptionDefinition} sources={subscriptions} />);

    expect(screen.getByRole("button", { name: "远程" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("textbox", { name: "名称" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "本地" }));

    expect(screen.getByRole("button", { name: "分享订阅" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "本地" })).toHaveAttribute("aria-pressed", "true");
    const localContentInput = within(screen.getByRole("group", { name: "基本信息" })).getByRole("textbox", { name: "内容" });
    const localContentEditor = localContentInput.closest("[data-highlighted-textarea]");
    expect(localContentEditor).toHaveAttribute("data-highlighted-textarea", "text");
    expect(localContentEditor?.querySelector('[data-line-number="1"]')).toHaveTextContent("1");
    expect(screen.queryByRole("textbox", { name: "订阅地址" })).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "User-Agent" })).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "代理" })).not.toBeInTheDocument();
    expect(screen.queryByRole("spinbutton", { name: "超时毫秒" })).not.toBeInTheDocument();

    fireEvent.change(localContentInput, { target: { value: "ss://converted" } });
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const saved = onSave.mock.calls[0]?.[0] as FormData;
    expect(saved.get("subscription_type")).toBe("local");
    expect(saved.get("source_input")).toBe("ss://converted");
    expect(saved.get("user_agent")).toBeNull();
    expect(saved.get("proxy")).toBeNull();
    expect(saved.get("timeout_ms")).toBeNull();
    expect(saved.getAll("subscriptions")).toEqual([]);
    expect(JSON.parse(String(saved.get("processors")))).toEqual([
      { type: "quick_settings", stage: "nodes" },
      {
        name: "入口重命名",
        type: "rename",
        stage: "nodes",
        params: { mode: "prefix", value: "source-" },
      },
    ]);
  });
  it("switches remote subscription edits to a collection without stale source fields", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn(saveSuccess);
    render(<SubscriptionEditPage item={subscriptions[0]} onBack={noop} onSave={onSave} definition={remoteSubscriptionDefinition} sources={subscriptions} />);

    await user.click(screen.getByRole("button", { name: "组合" }));

    expect(screen.getByRole("button", { name: "组合" })).toHaveAttribute("aria-pressed", "true");
    const sourcePicker = screen.getByRole("group", { name: "包含订阅" });
    const warnSource = within(sourcePicker).getByRole("checkbox", { name: "warn 远程订阅 · uri-list" });
    expect(within(sourcePicker).queryByRole("checkbox", { name: "provider 远程订阅 · uri-list" })).not.toBeInTheDocument();
    expect(warnSource).not.toBeChecked();

    await user.click(warnSource);
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const saved = onSave.mock.calls[0]?.[0] as FormData;
    expect(saved.get("subscription_type")).toBe("collection");
    expect(saved.getAll("subscriptions")).toEqual(["warn"]);
    expect(saved.get("source_input")).toBeNull();
    expect(saved.get("format")).toBeNull();
    expect(saved.get("user_agent")).toBeNull();
    expect(saved.get("proxy")).toBeNull();
    expect(saved.get("timeout_ms")).toBeNull();
  });
  it("renders subscription editing as a segmented full page form", () => {
    render(
      <SubscriptionEditPage
        item={{ ...subscriptions[2], displayName: "默认组合", title: "默认组合" }}
        definition={{
          name: "default",
          displayName: "默认组合",
          kind: "collection",
          sourceRefs: ["provider", "warn"],
          processors: [],
          meta: { description: "main group\nbackup group" },
        }}
        onBack={noop}
        onSave={saveSuccess}
        sources={subscriptions}
      />,
    );

    expect(screen.getByRole("heading", { name: "编辑订阅" })).toBeInTheDocument();
    expect(screen.getByText("组合信息")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "名称" })).toHaveValue("default");
    expect(screen.getByRole("textbox", { name: "名称" })).toBeDisabled();
    expect(screen.getByRole("textbox", { name: "显示名称" })).toHaveValue("默认组合");
    expect(screen.getByRole("textbox", { name: "描述" })).toHaveValue("main group\nbackup group");
    const sourcePicker = screen.getByRole("group", { name: "包含订阅" });
    expect(within(sourcePicker).getByRole("checkbox", { name: "provider 远程订阅 · uri-list" })).toBeChecked();
    expect(within(sourcePicker).getByRole("checkbox", { name: "warn 远程订阅 · uri-list" })).toBeChecked();
    expect(screen.getByText("处理链")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存订阅" })).toBeInTheDocument();
  });
  it("confirms before leaving a dirty subscription edit page", async () => {
    const user = userEvent.setup();
    render(<SubscriptionEditPage item={subscriptions[2]} onBack={noop} onSave={saveSuccess} sources={subscriptions} />);

    await user.type(screen.getByRole("textbox", { name: "描述" }), "private");
    await user.click(screen.getByRole("button", { name: "返回" }));

    const dialog = screen.getByRole("dialog", { name: "放弃修改？" });
    expect(dialog).toHaveTextContent("离开后当前编辑内容不会保存");
    expect(within(dialog).getByRole("button", { name: "继续编辑" })).toBeInTheDocument();
  });
  it("prefills source editing with full source fields", async () => {
    const user = userEvent.setup();
    render(<SubscriptionEditPage item={subscriptions[0]} onBack={noop} onSave={saveSuccess} definition={remoteSubscriptionDefinition} sources={subscriptions} />);

    expect(screen.getByRole("textbox", { name: "订阅地址" })).toHaveValue("https://example.com/sub");
    expect(screen.getByRole("textbox", { name: "User-Agent" })).toHaveValue("Sandrone Test");
    expect(screen.getByRole("textbox", { name: "代理" })).toHaveValue("http://127.0.0.1:7890");
    expect(screen.getByRole("spinbutton", { name: "超时毫秒" })).toHaveValue(10000);
    expect(screen.getByRole("spinbutton", { name: "远程请求缓存（秒）" })).toHaveValue(45);
    expect(screen.getByRole("combobox", { name: "渲染结果缓存" })).toHaveValue("disabled");
    expect(screen.queryByRole("textbox", { name: "SHA-256" })).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "期望状态" })).not.toBeInTheDocument();
    expect(screen.queryByRole("spinbutton", { name: "最大字节" })).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "内容快照（base64）" })).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "处理器 JSON" })).not.toBeInTheDocument();
    expect(screen.getByText("基本信息")).toBeInTheDocument();
    expect(screen.getByText("处理链")).toBeInTheDocument();
    const quickSettingsGroup = screen.getByRole("group", { name: "处理器 快捷设置" });
    expect(within(quickSettingsGroup).getByText("处理器 1")).toBeInTheDocument();
    expect(within(quickSettingsGroup).queryByRole("textbox", { name: "名称" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "第 1 个处理器类型" })).not.toBeInTheDocument();
    const renamedProcessorGroup = screen.getByRole("group", { name: "处理器 入口重命名" });
    expect(within(renamedProcessorGroup).getByRole("heading", { name: "入口重命名" })).toBeInTheDocument();
    expect(within(renamedProcessorGroup).getByText("名称处理")).toBeInTheDocument();
    expect(within(renamedProcessorGroup).queryByRole("textbox", { name: "名称" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "第 2 个处理器类型" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "第 1 个处理器阶段" })).not.toBeInTheDocument();
    await user.click(screen.getAllByRole("button", { name: "编辑名称" })[1]);
    expect(within(renamedProcessorGroup).getByRole("textbox", { name: "名称" })).toHaveValue("入口重命名");
    expect(screen.getByRole("combobox", { name: "重命名方式" })).toHaveTextContent("添加前缀");
    expect(screen.getByRole("textbox", { name: "内容" })).toHaveValue("source-");
    expect(screen.queryByRole("combobox", { name: /名称操作/ })).not.toBeInTheDocument();
  });
  it("orders source editing sections by the remote subscription workflow", () => {
    const { container } = render(<SubscriptionEditPage item={subscriptions[0]} onBack={noop} onSave={saveSuccess} definition={remoteSubscriptionDefinition} sources={subscriptions} />);

    const sourceInfo = screen.getByRole("group", { name: "基本信息" });
    const processorRules = screen.getByRole("group", { name: "处理链" });
    expect(Boolean(sourceInfo.compareDocumentPosition(processorRules) & Node.DOCUMENT_POSITION_FOLLOWING)).toBe(true);
    expect(within(sourceInfo).getByRole("textbox", { name: "名称" })).toHaveValue("provider");
    expect(within(sourceInfo).getByRole("textbox", { name: "名称" })).toBeDisabled();
    expect(within(sourceInfo).getByRole("textbox", { name: "订阅地址" })).toHaveValue("https://example.com/sub");
    expect(within(sourceInfo).getByRole("spinbutton", { name: "超时毫秒" })).toHaveValue(10000);
    const formatSelect = within(sourceInfo).getByRole("combobox", { name: "格式" });
    expect(formatSelect).toHaveValue("base64");
    expect(within(formatSelect).getByRole("option", { name: "自动" })).toHaveValue("auto");
    expect(within(formatSelect).getByRole("option", { name: "URI List" })).toHaveValue("uri-list");
    expect(within(formatSelect).queryByRole("option", { name: /Shadowrocket/i })).not.toBeInTheDocument();
    expect(within(sourceInfo).queryByRole("textbox", { name: "类型" })).not.toBeInTheDocument();
    expect(within(sourceInfo).getByRole("textbox", { name: "描述" })).toHaveValue("daily");
    expect(within(sourceInfo).queryByRole("textbox", { name: "元数据 JSON" })).not.toBeInTheDocument();
    expect(container.querySelector('input[name="meta"][type="hidden"]')).toHaveValue(JSON.stringify(remoteSubscriptionDefinition.meta, null, 2));
    expect(screen.queryByRole("group", { name: "解析设置" })).not.toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "元数据" })).not.toBeInTheDocument();

    expect(processorRules).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "远程抓取设置" })).not.toBeInTheDocument();
  });
});
