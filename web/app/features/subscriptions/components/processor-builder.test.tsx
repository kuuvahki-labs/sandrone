import { createHash } from "node:crypto";

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ProcessorBuilder } from "~/features/subscriptions/components/processor-builder";
import {
  remoteSubscriptionDefinition,
  scriptFiles,
  selectMuiOption,
} from "~/features/subscriptions/test-data";
import { UICapabilityProvider } from "~/shared/capabilities/context";
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
    <UICapabilityProvider value={{
      capabilities: [{ key: "probe.enabled", enabled: true }],
      loaded: true,
      hasFeature: (key) => key === "probe.enabled",
      getFeature: (key) => key === "probe.enabled" ? { key, enabled: true } : undefined,
    }}>
      <ProcessorBuilder
        defaultValue={defaultValue}
        onDirty={onDirty}
        scriptFiles={availableScriptFiles}
      />
    </UICapabilityProvider>,
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
  it("keeps an empty processor chain empty", () => {
    const { serializedProcessors } = renderProcessorBuilder();

    expect(screen.queryByRole("group", { name: "处理器 快捷设置" })).not.toBeInTheDocument();
    expect(screen.queryByText("未配置处理器。")).not.toBeInTheDocument();
    expect(serializedProcessors()).toEqual([]);
  });

  it("does not append quick settings to persisted processors", async () => {
    const user = userEvent.setup();
    const { serializedProcessors } = renderProcessorBuilder({
      defaultValue: [
        ...(remoteSubscriptionDefinition.processors ?? []),
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
      ],
    });

    const renameGroup = screen.getByRole("group", { name: "处理器 入口重命名" });
    await user.click(within(renameGroup).getByRole("button", { name: "编辑名称" }));
    const nameInput = within(renameGroup).getByRole("textbox", { name: "名称" });
    expect(nameInput).toHaveValue("入口重命名");
    expect(nameInput).toHaveAttribute("placeholder", "留空使用默认名称");

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
      {
        name: "入口重命名",
        type: "rename",
        stage: "nodes",
        params: { mode: "prefix", value: "source-" },
      },
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

  it("preserves the persisted position of quick settings", () => {
    const { serializedProcessors } = renderProcessorBuilder({
      defaultValue: [
        { type: "rename", stage: "nodes", params: { mode: "prefix", value: "source-" } },
        { type: "quick_settings", stage: "nodes", params: { udp: "enabled" } },
        { type: "sort", stage: "nodes", params: { by: "+name" } },
      ],
    });

    expect(serializedProcessors()).toEqual([
      { type: "rename", stage: "nodes", params: { mode: "prefix", value: "source-" } },
      { type: "quick_settings", stage: "nodes", params: { udp: "enabled" } },
      { type: "sort", stage: "nodes", params: { by: "+name" } },
    ]);
  });

  it("appends the information-node filter preset after an existing probe", async () => {
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
    expect(probeURL).toHaveValue("https://cp.cloudflare.com");
    expect(within(probeGroup).getByRole("spinbutton", { name: "超时毫秒" })).toHaveValue(5000);
    expect(within(probeGroup).getByRole("spinbutton", { name: "尝试次数" })).toHaveValue(1);
    expect(within(probeGroup).getByRole("spinbutton", { name: "并发数" })).toHaveValue(10);
    expect(within(probeGroup).getByRole("spinbutton", { name: "缓存秒数" })).toHaveValue(0);
    expect(within(probeGroup).queryByText(/默认/)).not.toBeInTheDocument();

    expect(serializedProcessors()).toEqual([
      {
        type: "probe",
        stage: "nodes",
        params: {
          method: "url_test",
          core: "sing-box",
          url: "https://cp.cloudflare.com",
          timeout_ms: 5000,
          attempts: 1,
          concurrency: 10,
          cache_ttl_seconds: 0,
          fail_mode: "keep",
        },
      },
    ]);

    await selectMuiOption(user, screen.getByRole("combobox", { name: "类型" }), "过滤信息节点");
    await user.click(screen.getByRole("button", { name: "添加处理器" }));

    expect(serializedProcessors()).toEqual([
      {
        type: "probe",
        stage: "nodes",
        params: {
          method: "url_test",
          core: "sing-box",
          url: "https://cp.cloudflare.com",
          timeout_ms: 5000,
          attempts: 1,
          concurrency: 10,
          cache_ttl_seconds: 0,
          fail_mode: "keep",
        },
      },
      {
        name: "过滤信息节点",
        type: "filter",
        stage: "nodes",
        params: {
          action: "drop",
          field: "name",
          match: "regex",
          pattern: "(?i)(网址|官网|流量|剩余|时间|应急|套餐|订阅|公告|重置|过期|到期|bandwidth|traffic|quota|reset|expire|expiry|expiration)",
        },
      },
    ]);
  });

  it("appends the inline node-name normalization preset without a file dependency", async () => {
    const user = userEvent.setup();
    const { serializedProcessors } = renderProcessorBuilder({
      defaultValue: [{ type: "sort", stage: "nodes", params: { by: "+name" } }],
    });

    await selectMuiOption(
      user,
      screen.getByRole("combobox", { name: "类型" }),
      "节点名称规范化",
    );
    await user.click(screen.getByRole("button", { name: "添加处理器" }));

    const processors = serializedProcessors();
    expect(processors[0]).toEqual({
      type: "sort",
      stage: "nodes",
      params: { by: "+name" },
    });
    expect(processors[1]).toMatchObject({
      name: "节点名称规范化",
      type: "script",
      stage: "nodes",
      params: { source: { type: "inline" } },
    });
    expect(processors[1].params).not.toHaveProperty("args");
    const source = processors[1].params?.source as { content?: unknown };
    expect(typeof source.content).toBe("string");
    expect(createHash("sha256").update(String(source.content)).digest("hex"))
      .toBe("218f3bbeb8b6e176e501f7d4b236e7a573196c7126d4b13dc286c763491a0ab9");
  });

  it("starts without quick settings and appends visual processors in order", async () => {
    const user = userEvent.setup();
    const onDirty = vi.fn();
    const { serializedProcessors } = renderProcessorBuilder({ onDirty });

    const newType = screen.getByRole("combobox", { name: "类型" });
    expect(newType).toHaveTextContent("过滤");
    const addProcessor = screen.getByRole("button", { name: "添加处理器" });
    expect(addProcessor).toHaveTextContent("添加");
    await user.click(newType);
    const listbox = await screen.findByRole("listbox");
    const options = within(listbox).getAllByRole("option");
    expect(options[0]).toHaveTextContent("过滤");
    expect(within(listbox).queryByRole("option", { name: "排除匹配节点" })).not.toBeInTheDocument();
    expect(within(listbox).queryByRole("option", { name: "按类型过滤" })).not.toBeInTheDocument();
    expect(within(listbox).queryByRole("option", { name: "清理名称" })).not.toBeInTheDocument();
    await user.click(options[0]);

    expect(screen.queryByRole("group", { name: "处理器 快捷设置" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "第 1 个处理器类型" })).not.toBeInTheDocument();
    expect(serializedProcessors()).toEqual([]);

    await user.click(addProcessor);
    expect(onDirty).toHaveBeenCalled();
    expect(screen.queryByRole("combobox", { name: "第 1 个处理器阶段" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "第 2 个处理器类型" })).not.toBeInTheDocument();
    const filterGroup = screen.getByRole("group", { name: "处理器 过滤" });
    expect(within(filterGroup).getByText("处理器 1")).toBeInTheDocument();
    await user.click(within(filterGroup).getByRole("button", { name: "编辑名称" }));
    const filterName = within(filterGroup).getByRole("textbox", { name: "名称" });
    expect(filterName).toHaveValue("");
    fireEvent.change(filterName, { target: { value: "按 server 过滤" } });
    expect(within(filterGroup).getByRole("combobox", { name: "过滤动作" })).toHaveTextContent("保留");
    await user.click(within(filterGroup).getByRole("combobox", { name: "匹配字段" }));
    const fieldListbox = await screen.findByRole("listbox");
    expect(within(fieldListbox).getByRole("option", { name: "name" })).toBeInTheDocument();
    expect(within(fieldListbox).getByRole("option", { name: "type" })).toBeInTheDocument();
    await user.click(within(fieldListbox).getByRole("option", { name: "server" }));
    expect(screen.queryByRole("option", { name: "source_format" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "tags" })).not.toBeInTheDocument();
    expect(within(filterGroup).getByRole("combobox", { name: "匹配方式" })).toHaveTextContent("正则");
    fireEvent.change(within(filterGroup).getByRole("textbox", { name: "正则表达式" }), {
      target: { value: "example\\.com" },
    });

    expect(serializedProcessors()).toEqual([
      { name: "按 server 过滤", type: "filter", stage: "nodes", params: { action: "keep", field: "server", match: "regex", pattern: "example\\.com" } },
    ]);

    await selectMuiOption(user, newType, "名称处理");
    await user.click(addProcessor);
    const renameGroup = screen.getByRole("group", { name: "处理器 名称处理" });
    expect(within(renameGroup).getByRole("checkbox", { name: "去除首尾空白" })).toBeChecked();
    expect(within(renameGroup).getByRole("combobox", { name: "重命名方式" })).toHaveTextContent("不重命名");
    expect(within(renameGroup).queryByRole("combobox", { name: /名称操作/ })).not.toBeInTheDocument();

    expect(serializedProcessors()).toEqual([
      { name: "按 server 过滤", type: "filter", stage: "nodes", params: { action: "keep", field: "server", match: "regex", pattern: "example\\.com" } },
      { type: "rename", stage: "nodes", params: { trim: true } },
    ]);

    await selectMuiOption(user, newType, "快捷设置");
    await user.click(addProcessor);
    const quickSettingsGroup = screen.getByRole("group", { name: "处理器 快捷设置" });
    expect(within(quickSettingsGroup).getByText("处理器 3")).toBeInTheDocument();
    expect(serializedProcessors()).toEqual([
      { name: "按 server 过滤", type: "filter", stage: "nodes", params: { action: "keep", field: "server", match: "regex", pattern: "example\\.com" } },
      { type: "rename", stage: "nodes", params: { trim: true } },
      { type: "quick_settings", stage: "nodes" },
    ]);
  });

  it("binds shared script and custom key-value editors into one ordered chain", async () => {
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
        {
          type: "custom",
          stage: "nodes",
          params: { enabled: true, threshold: 2 },
        },
      ],
      scriptFiles: scriptFiles.map((file) => file.name === "rename.js" ? { ...file, title: "Rename Nodes" } : file),
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

    const customGroup = screen.getByRole("group", { name: "处理器 custom" });
    const paramsInput = within(customGroup).getByRole("textbox", { name: "参数键值" });
    expect(paramsInput).toHaveValue("enabled=true\nthreshold=2");
    expect(paramsInput.closest("[data-highlighted-textarea]")).toHaveAttribute("data-highlighted-textarea", "text");
    fireEvent.change(paramsInput, { target: { value: "enabled=false\nthreshold=3" } });

    await user.click(within(scriptGroup).getByRole("button", { name: "文件脚本" }));
    expect(within(scriptGroup).queryByRole("textbox", { name: "内联脚本" })).not.toBeInTheDocument();
    const scriptFileSelect = within(scriptGroup).getByRole("combobox", { name: "脚本文件" });
    expect(scriptFileSelect).toHaveValue("rename.js");
    expect(within(scriptFileSelect).getByRole("option", { name: "Rename Nodes (rename.js)" })).toBeInTheDocument();
    expect(within(scriptGroup).queryByRole("textbox", { name: "脚本路径" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "default.yaml" })).not.toBeInTheDocument();
    await user.selectOptions(scriptFileSelect, "other.js");
    fireEvent.change(argsInput, { target: { value: "in=zh\nflag=true\nthreshold=2" } });

    expect(serializedProcessors()).toEqual([
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
      { type: "custom", stage: "nodes", params: { enabled: false, threshold: 3 } },
    ]);
  });
});
