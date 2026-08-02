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
    const onPreview = vi.fn();
    const onShare = vi.fn();
    render(
      <SubscriptionEditPage
        definition={remoteSubscriptionDefinition}
        item={subscriptions[0]}
        onBack={noop}
        onPreview={onPreview}
        onSave={onSave}
        onShare={onShare}
        sources={subscriptions}
      />,
    );

    const share = screen.getByRole("button", { name: "分享订阅" });
    const preview = screen.getByRole("button", { name: "预览订阅" });
    expect(share).toHaveTextContent(/^分享$/);
    expect(share).toBeEnabled();
    expect(preview).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "远程" }));
    expect(share).toBeEnabled();

    const displayName = screen.getByRole("textbox", { name: "显示名称" });
    fireEvent.change(displayName, { target: { value: "updated" } });
    expect(screen.getByRole("button", { name: "分享订阅" })).toHaveAttribute("aria-disabled", "true");
    const dirtyPreview = screen.getByRole("button", { name: "预览订阅" });
    expect(dirtyPreview).toHaveAttribute("aria-disabled", "true");
    expect(dirtyPreview).toHaveAccessibleDescription("请先保存修改，再预览已保存版本");

    const form = displayName.closest("form");
    if (!form) throw new Error("expected subscription edit form");
    fireEvent.submit(form);
    fireEvent.submit(form);
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "保存订阅" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "预览订阅" }))
      .toHaveAccessibleDescription("保存完成后即可预览");
    await act(async () => resolveSave(true));
    expect(screen.getByRole("button", { name: "分享订阅" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "预览订阅" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "分享订阅" }));
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

    fireEvent.change(screen.getByRole("textbox", { name: "显示名称" }), {
      target: { value: "invalid" },
    });
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    expect(screen.getByRole("button", { name: "分享订阅" })).toHaveAttribute("aria-disabled", "true");
  });
  it("switches remote edits across local and collection modes without stale fields", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn(saveSuccess);
    render(<SubscriptionEditPage item={subscriptions[0]} onBack={noop} onSave={onSave} onShare={noop} definition={remoteSubscriptionDefinition} sources={subscriptions} />);

    expect(screen.getByRole("button", { name: "远程" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("textbox", { name: "名称" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "本地" }));

    expect(screen.getByRole("button", { name: "分享订阅" })).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByRole("button", { name: "本地" })).toHaveAttribute("aria-pressed", "true");
    await user.click(screen.getByRole("button", { name: "返回" }));
    const dialog = screen.getByRole("dialog", { name: "放弃修改？" });
    expect(dialog).toHaveTextContent("离开后当前编辑内容不会保存");
    const continueEditing = within(dialog).getByRole("button", { name: "继续编辑" });
    expect(continueEditing).toBeInTheDocument();
    await user.click(continueEditing);
    expect(screen.queryByRole("dialog", { name: "放弃修改？" })).not.toBeInTheDocument();

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

    const savedLocal = onSave.mock.calls[0]?.[0] as FormData;
    expect(savedLocal.get("subscription_type")).toBe("local");
    expect(savedLocal.get("source_input")).toBe("ss://converted");
    expect(savedLocal.get("user_agent")).toBeNull();
    expect(savedLocal.get("proxy")).toBeNull();
    expect(savedLocal.get("timeout_ms")).toBeNull();
    expect(savedLocal.getAll("subscriptions")).toEqual([]);
    expect(JSON.parse(String(savedLocal.get("processors")))).toEqual([
      {
        name: "入口重命名",
        type: "rename",
        stage: "nodes",
        params: { mode: "prefix", value: "source-" },
      },
      { type: "quick_settings", stage: "nodes" },
    ]);

    await user.click(screen.getByRole("button", { name: "组合" }));

    expect(screen.getByRole("button", { name: "组合" })).toHaveAttribute("aria-pressed", "true");
    const sourcePicker = screen.getByRole("group", { name: "包含订阅" });
    const warnSource = within(sourcePicker).getByRole("checkbox", { name: "warn 远程订阅 · uri-list" });
    expect(within(sourcePicker).queryByRole("checkbox", { name: "provider 远程订阅 · uri-list" })).not.toBeInTheDocument();
    expect(warnSource).not.toBeChecked();

    await user.click(warnSource);
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const savedCollection = onSave.mock.calls[1]?.[0] as FormData;
    expect(savedCollection.get("subscription_type")).toBe("collection");
    expect(savedCollection.getAll("subscriptions")).toEqual(["warn"]);
    expect(savedCollection.get("source_input")).toBeNull();
    expect(savedCollection.get("format")).toBeNull();
    expect(savedCollection.get("user_agent")).toBeNull();
    expect(savedCollection.get("proxy")).toBeNull();
    expect(savedCollection.get("timeout_ms")).toBeNull();
    expect(JSON.parse(String(savedCollection.get("processors")))).toEqual([
      {
        name: "入口重命名",
        type: "rename",
        stage: "nodes",
        params: { mode: "prefix", value: "source-" },
      },
      { type: "quick_settings", stage: "nodes" },
    ]);
  });
  it("prefills and orders the full remote-source editing workflow", async () => {
    const user = userEvent.setup();
    const { container } = render(<SubscriptionEditPage item={subscriptions[0]} onBack={noop} onSave={saveSuccess} definition={remoteSubscriptionDefinition} sources={subscriptions} />);

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
    expect(within(quickSettingsGroup).getByText("处理器 2")).toBeInTheDocument();
    expect(within(quickSettingsGroup).queryByRole("textbox", { name: "名称" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "第 1 个处理器类型" })).not.toBeInTheDocument();
    const renamedProcessorGroup = screen.getByRole("group", { name: "处理器 入口重命名" });
    expect(within(renamedProcessorGroup).getByRole("heading", { name: "入口重命名" })).toBeInTheDocument();
    expect(within(renamedProcessorGroup).getByText("名称处理")).toBeInTheDocument();
    expect(within(renamedProcessorGroup).queryByRole("textbox", { name: "名称" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "第 2 个处理器类型" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "第 1 个处理器阶段" })).not.toBeInTheDocument();
    await user.click(screen.getAllByRole("button", { name: "编辑名称" })[0]);
    expect(within(renamedProcessorGroup).getByRole("textbox", { name: "名称" })).toHaveValue("入口重命名");
    expect(screen.getByRole("combobox", { name: "重命名方式" })).toHaveTextContent("添加前缀");
    expect(screen.getByRole("textbox", { name: "内容" })).toHaveValue("source-");
    expect(screen.queryByRole("combobox", { name: /名称操作/ })).not.toBeInTheDocument();

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
