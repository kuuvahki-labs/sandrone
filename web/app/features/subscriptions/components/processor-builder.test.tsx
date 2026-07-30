import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ProcessorBuilder } from "~/features/subscriptions/components/processor-builder";
import {
  remoteSubscriptionDefinition,
  scriptFiles,
  selectMuiOption,
} from "~/features/subscriptions/test-data";
import type { ProcessorDetail, ResourceOption } from "~/shared/resources/types";

function renderProcessorBuilder({
  defaultValue = [],
  onDirty,
  scriptFiles: availableScriptFiles = [],
}: {
  defaultValue?: ProcessorDetail[];
  onDirty?: () => void;
  scriptFiles?: ResourceOption[];
} = {}) {
  const { container } = render(
    <ProcessorBuilder
      defaultValue={defaultValue}
      onDirty={onDirty}
      scriptFiles={availableScriptFiles}
    />,
  );
  const processorsInput = container.querySelector<HTMLInputElement>(
    'input[name="processors"][type="hidden"]',
  );
  if (!processorsInput) {
    throw new Error("ProcessorBuilder did not render its processors input");
  }
  return {
    processorsInput,
    serializedProcessors: () => JSON.parse(processorsInput.value) as ProcessorDetail[],
  };
}

describe("ProcessorBuilder", () => {
  it("serializes all probe processor parameters from the subscription processing chain", () => {
    const { serializedProcessors } = renderProcessorBuilder({
      defaultValue: [{
        type: "probe",
        stage: "nodes",
        params: {
          method: "url_test",
          core: "sing-box",
          url: "https://www.gstatic.com/generate_204",
          expected_status: "204",
          timeout_ms: 5000,
          attempts: 2,
          concurrency: 3,
          cache_ttl_seconds: 300,
          annotate: true,
          sort: "duration",
          fail_mode: "drop",
        },
      }],
    });

    const probeGroup = screen.getByRole("group", { name: "处理器 测活" });
    expect(within(probeGroup).getByRole("combobox", { name: "方式" })).toHaveTextContent("url_test");
    expect(within(probeGroup).getAllByRole("combobox")).toHaveLength(4);
    expect(probeGroup).not.toHaveTextContent(/sing-box|mihomo/);
    expect(within(probeGroup).getByRole("combobox", { name: "URL" })).toHaveValue("https://www.gstatic.com/generate_204");
    expect(within(probeGroup).getByRole("textbox", { name: "期望状态" })).toHaveValue("204");
    expect(within(probeGroup).getByRole("spinbutton", { name: "超时毫秒" })).toHaveValue(5000);
    expect(within(probeGroup).getByRole("spinbutton", { name: "尝试次数" })).toHaveValue(2);
    expect(within(probeGroup).getByRole("spinbutton", { name: "并发数" })).toHaveValue(3);
    expect(within(probeGroup).getByRole("spinbutton", { name: "缓存秒数" })).toHaveValue(300);
    expect(within(probeGroup).getByRole("checkbox", { name: "写入测活元数据" })).toBeChecked();
    expect(within(probeGroup).getByRole("combobox", { name: "排序" })).toHaveTextContent("按耗时");
    expect(within(probeGroup).getByRole("combobox", { name: "失败处理" })).toHaveTextContent("丢弃");

    expect(serializedProcessors()).toEqual([
      { type: "quick_settings", stage: "nodes" },
      {
        type: "probe",
        stage: "nodes",
        params: {
          method: "url_test",
          core: "sing-box",
          url: "https://www.gstatic.com/generate_204",
          expected_status: "204",
          timeout_ms: 5000,
          attempts: 2,
          concurrency: 3,
          cache_ttl_seconds: 300,
          annotate: true,
          sort: "duration",
          fail_mode: "drop",
        },
      },
    ]);
  });
  it("fills probe runtime defaults into inputs and serializes them", async () => {
    const user = userEvent.setup();
    const { serializedProcessors } = renderProcessorBuilder();

    await user.click(screen.getByRole("combobox", { name: "类型" }));
    await user.click(screen.getByRole("option", { name: "测活" }));
    await user.click(screen.getByRole("button", { name: "添加处理器" }));

    const probeGroup = screen.getByRole("group", { name: "处理器 测活" });
    expect(within(probeGroup).getByRole("combobox", { name: "方式" })).toHaveTextContent("url_test");
    expect(within(probeGroup).getAllByRole("combobox")).toHaveLength(4);
    expect(probeGroup).not.toHaveTextContent(/sing-box|mihomo/);
    expect(within(probeGroup).getByRole("combobox", { name: "失败处理" })).toHaveTextContent("保留");
    expect(within(probeGroup).queryByRole("textbox", { name: "NTP 服务器" })).not.toBeInTheDocument();
    const probeURL = within(probeGroup).getByRole("combobox", { name: "URL" });
    expect(probeURL).toHaveValue("http://www.gstatic.com/generate_204");
    await user.click(probeURL);
    await user.keyboard("{ArrowDown}");
    await user.click(await screen.findByRole("option", {
      name: "华为 http://connectivitycheck.platform.hicloud.com/generate_204",
    }));
    expect(within(probeGroup).getByRole("spinbutton", { name: "超时毫秒" })).toHaveValue(5000);
    expect(within(probeGroup).getByRole("spinbutton", { name: "尝试次数" })).toHaveValue(1);
    expect(within(probeGroup).getByRole("spinbutton", { name: "并发数" })).toHaveValue(10);
    expect(within(probeGroup).getByRole("spinbutton", { name: "缓存秒数" })).toHaveValue(0);
    expect(within(probeGroup).queryByText(/默认/)).not.toBeInTheDocument();

    expect(serializedProcessors()).toEqual([
      { type: "quick_settings", stage: "nodes" },
      {
        type: "probe",
        stage: "nodes",
        params: {
          method: "url_test",
          core: "sing-box",
          url: "http://connectivitycheck.platform.hicloud.com/generate_204",
          timeout_ms: 5000,
          attempts: 1,
          concurrency: 10,
          cache_ttl_seconds: 0,
          fail_mode: "keep",
        },
      },
    ]);
  });
  it("serializes visual processor edits into the hidden form field", async () => {
    const user = userEvent.setup();
    const onDirty = vi.fn();
    const { serializedProcessors } = renderProcessorBuilder({ onDirty });

    expect(screen.getByRole("combobox", { name: "类型" })).toHaveTextContent("过滤");
    const quickSettingsGroup = screen.getByRole("group", { name: "处理器 快捷设置" });
    expect(within(quickSettingsGroup).getByText("处理器 1")).toBeInTheDocument();
    expect(within(quickSettingsGroup).queryByRole("textbox", { name: "名称" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "添加处理器" }));
    expect(onDirty).toHaveBeenCalled();
    expect(screen.queryByRole("combobox", { name: "第 1 个处理器阶段" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "第 2 个处理器类型" })).not.toBeInTheDocument();
    const addedProcessorGroup = screen.getByRole("group", { name: "处理器 过滤" });
    expect(within(addedProcessorGroup).getByText("处理器 2")).toBeInTheDocument();
    await user.click(within(addedProcessorGroup).getByRole("button", { name: "编辑名称" }));
    const addedProcessorName = within(addedProcessorGroup).getByRole("textbox", { name: "名称" });
    expect(addedProcessorName).toHaveValue("");
    fireEvent.change(addedProcessorName, { target: { value: "按 server 过滤" } });
    expect(screen.getByRole("combobox", { name: "过滤动作" })).toHaveTextContent("保留");
    await user.click(screen.getByRole("combobox", { name: "匹配字段" }));
    const fieldListbox = await screen.findByRole("listbox");
    expect(within(fieldListbox).getByRole("option", { name: "name" })).toBeInTheDocument();
    expect(within(fieldListbox).getByRole("option", { name: "type" })).toBeInTheDocument();
    await user.click(within(fieldListbox).getByRole("option", { name: "server" }));
    expect(screen.queryByRole("option", { name: "source_format" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "tags" })).not.toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "匹配方式" })).toHaveTextContent("正则");
    fireEvent.change(screen.getByRole("textbox", { name: "正则表达式" }), {
      target: { value: "example\\.com" },
    });

    expect(serializedProcessors()).toEqual([
      { type: "quick_settings", stage: "nodes" },
      { name: "按 server 过滤", type: "filter", stage: "nodes", params: { action: "keep", field: "server", match: "regex", pattern: "example\\.com" } },
    ]);
  });
  it("adds the optional information-node filter preset before health checks", async () => {
    const user = userEvent.setup();
    const { serializedProcessors } = renderProcessorBuilder({
      defaultValue: [{
        type: "probe",
        stage: "nodes",
        params: { method: "url_test", core: "sing-box" },
      }],
    });

    await selectMuiOption(user, screen.getByRole("combobox", { name: "类型" }), "过滤信息节点（预设）");
    await user.click(screen.getByRole("button", { name: "添加处理器" }));

    const saved = serializedProcessors();
    expect(saved.map((processor: { type: string }) => processor.type)).toEqual(["quick_settings", "filter", "probe"]);
    expect(saved[1]).toEqual({
      name: "过滤信息节点",
      type: "filter",
      stage: "nodes",
      params: {
        action: "drop",
        field: "name",
        match: "regex",
        pattern: "(?i)(网址|流量|时间|应急|过期|bandwidth|expire)",
      },
    });
  });
  it("uses the shared script params contract for subscription processors", async () => {
    const user = userEvent.setup();
    const { serializedProcessors } = renderProcessorBuilder({
      defaultValue: [
        {
          type: "script",
          stage: "nodes",
          params: {
            engine: "js",
            args: { flag: true, in: "zh" },
            id: "legacy-script",
            path: "rename.js",
            content: "return input;",
            timeout_ms: 5000,
          },
        },
      ],
      scriptFiles,
    });

    const scriptGroup = screen.getByRole("group", { name: "处理器 脚本处理" });
    expect(within(scriptGroup).queryByRole("combobox", { name: "第 2 个处理器类型" })).not.toBeInTheDocument();
    expect(within(scriptGroup).getByRole("group", { name: "脚本来源" })).toBeInTheDocument();
    expect(within(scriptGroup).getByRole("button", { name: "内联脚本" })).toHaveAttribute("aria-pressed", "true");
    const contentInput = within(scriptGroup).getByRole("textbox", { name: "内联脚本" });
    expect(contentInput).toHaveValue("return input;");
    const contentEditor = contentInput.closest("[data-highlighted-textarea]");
    expect(contentEditor).toHaveAttribute("data-highlighted-textarea", "javascript");
    expect(contentEditor?.querySelector('[data-line-number="1"]')).toHaveTextContent("1");
    expect(contentInput.closest(".md\\:col-span-2")).toBeInTheDocument();
    expect(within(scriptGroup).queryByRole("textbox", { name: "脚本路径" })).not.toBeInTheDocument();
    const argsInput = within(scriptGroup).getByRole("textbox", { name: "脚本参数" });
    expect(argsInput.closest("[data-highlighted-textarea]")).toHaveAttribute("data-highlighted-textarea", "text");
    expect(argsInput).toHaveValue("flag=true\nin=zh");
    expect(within(scriptGroup).getByRole("spinbutton", { name: "超时毫秒" })).toHaveValue(5000);
    expect(within(scriptGroup).queryByRole("textbox", { name: "脚本 ID" })).not.toBeInTheDocument();

    await user.click(within(scriptGroup).getByRole("button", { name: "文件脚本" }));
    expect(within(scriptGroup).queryByRole("textbox", { name: "内联脚本" })).not.toBeInTheDocument();
    const scriptFileSelect = within(scriptGroup).getByRole("combobox", { name: "脚本文件" });
    expect(scriptFileSelect).toHaveValue("rename.js");
    expect(within(scriptGroup).queryByRole("textbox", { name: "脚本路径" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "default.yaml" })).not.toBeInTheDocument();
    await user.selectOptions(scriptFileSelect, "other.js");
    fireEvent.change(argsInput, { target: { value: "in=zh\nflag=true\nthreshold=2" } });

    expect(serializedProcessors()).toEqual([
      { type: "quick_settings", stage: "nodes" },
      {
        type: "script",
        stage: "nodes",
        params: {
          engine: "js",
          args: { in: "zh", flag: true, threshold: 2 },
          id: "legacy-script",
          source: { type: "file", name: "other.js" },
          timeout_ms: 5000,
        },
      },
    ]);
  });
  it("uses a highlighted key-value editor for custom processor params", () => {
    const { serializedProcessors } = renderProcessorBuilder({
      defaultValue: [
        {
          type: "custom",
          stage: "nodes",
          params: { enabled: true, threshold: 2 },
        },
      ],
    });

    const customGroup = screen.getByRole("group", { name: "处理器 custom" });
    const paramsInput = within(customGroup).getByRole("textbox", { name: "参数键值" });
    expect(paramsInput).toHaveValue("enabled=true\nthreshold=2");
    expect(paramsInput.closest("[data-highlighted-textarea]")).toHaveAttribute("data-highlighted-textarea", "text");

    fireEvent.change(paramsInput, { target: { value: "enabled=false\nthreshold=3" } });

    expect(serializedProcessors()).toEqual([
      { type: "quick_settings", stage: "nodes" },
      { type: "custom", stage: "nodes", params: { enabled: false, threshold: 3 } },
    ]);
  });
  it("opens the processor name editor with the saved name as the input value", async () => {
    const user = userEvent.setup();
    renderProcessorBuilder({
      defaultValue: remoteSubscriptionDefinition.processors,
    });

    const processorGroup = screen.getByRole("group", { name: "处理器 入口重命名" });
    await user.click(within(processorGroup).getByRole("button", { name: "编辑名称" }));
    const nameInput = within(processorGroup).getByRole("textbox", { name: "名称" });

    expect(nameInput).toHaveValue("入口重命名");
    expect(nameInput).toHaveAttribute("placeholder", "留空使用默认名称");
  });
  it("seeds quick settings as the first processor before later additions", async () => {
    const user = userEvent.setup();
    const { serializedProcessors } = renderProcessorBuilder();

    const newType = screen.getByRole("combobox", { name: "类型" });
    expect(newType).toHaveTextContent("过滤");
    expect(screen.getByRole("button", { name: "添加处理器" })).toHaveTextContent("添加");
    expect(screen.getByRole("button", { name: "添加处理器" })).toHaveClass("shrink-0", "whitespace-nowrap");
    await user.click(newType);
    const listbox = await screen.findByRole("listbox");
    const options = within(listbox).getAllByRole("option");
    expect(options[0]).toHaveTextContent("过滤");
    expect(within(listbox).queryByRole("option", { name: "排除匹配节点" })).not.toBeInTheDocument();
    expect(within(listbox).queryByRole("option", { name: "按类型过滤" })).not.toBeInTheDocument();
    expect(within(listbox).queryByRole("option", { name: "清理名称" })).not.toBeInTheDocument();
    await user.click(options[0]);
    const quickSettingsGroup = screen.getByRole("group", { name: "处理器 快捷设置" });
    expect(within(quickSettingsGroup).getByText("处理器 1")).toBeInTheDocument();
    expect(within(quickSettingsGroup).queryByRole("textbox", { name: "名称" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "第 1 个处理器类型" })).not.toBeInTheDocument();
    expect(serializedProcessors()).toEqual([
      { type: "quick_settings", stage: "nodes" },
    ]);

    await user.click(screen.getByRole("button", { name: "添加处理器" }));

    const addedProcessorGroup = screen.getByRole("group", { name: "处理器 过滤" });
    expect(within(addedProcessorGroup).getByText("处理器 2")).toBeInTheDocument();
    await user.click(within(addedProcessorGroup).getByRole("button", { name: "编辑名称" }));
    expect(within(addedProcessorGroup).getByRole("textbox", { name: "名称" })).toHaveValue("");
    expect(screen.queryByRole("combobox", { name: "第 2 个处理器类型" })).not.toBeInTheDocument();
    expect(serializedProcessors()).toEqual([
      { type: "quick_settings", stage: "nodes" },
      { type: "filter", stage: "nodes", params: { action: "keep", field: "name", match: "regex" } },
    ]);
  });
  it("serializes added name processing as flat params", async () => {
    const user = userEvent.setup();
    const { serializedProcessors } = renderProcessorBuilder();

    await selectMuiOption(user, screen.getByRole("combobox", { name: "类型" }), "名称处理");
    await user.click(screen.getByRole("button", { name: "添加处理器" }));

    const addedProcessorGroup = screen.getByRole("group", { name: "处理器 名称处理" });
    expect(within(addedProcessorGroup).getByRole("checkbox", { name: "去除首尾空白" })).toBeChecked();
    expect(within(addedProcessorGroup).getByRole("combobox", { name: "重命名方式" })).toHaveTextContent("不重命名");
    expect(within(addedProcessorGroup).queryByRole("combobox", { name: /名称操作/ })).not.toBeInTheDocument();
    expect(serializedProcessors()).toEqual([
      { type: "quick_settings", stage: "nodes" },
      { type: "rename", stage: "nodes", params: { trim: true } },
    ]);
  });
});
