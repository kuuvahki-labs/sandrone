import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ConfigNodePreviewInput } from "~/features/files/config/model/node-source";
import { requireFileDriver } from "~/features/files/drivers/registry";
import { requireFileDriverUI } from "~/features/files/editor/file-driver-ui-registry";
import type { FileConfigDraft, FileDetail, FileItem, RuleSetCatalogResult } from "~/features/files/model/types";
import { FileEditPage } from "~/features/files/pages/file-edit-page";
import type { ResourceOption } from "~/shared/resources/types";

import { FileConfigEditor } from "./editor";

const subscriptions: ResourceOption[] = [
  { name: "provider", title: "provider" },
];

const item: FileItem = {
  name: "default.yaml",
  title: "default.yaml",
  kind: "mihomo",
};

const minimalConfig: FileConfigDraft = {
    subscriptions: ["provider"],
    group_preset: "minimal",
    ruleset_preset: "default",
    groups: [{ name: "Proxy", type: "select", proxies: ["$nodes", "DIRECT"] }],
    rule_sets: [{ name: "private", type: "inline", behavior: "classical", payload: ["DOMAIN-SUFFIX,local"] }],
    rules: ["RULE-SET,private,DIRECT", "MATCH,Proxy"],
};

const minimalDetail: FileDetail = {
  name: "default.yaml",
	kind: "mihomo",
  source: {},
  processors: [],
  config: {
    subscriptions: minimalConfig.subscriptions,
    settingsPresent: true,
    settings: {
      groups: minimalConfig.groups,
      rule_sets: minimalConfig.rule_sets,
      rules: minimalConfig.rules,
    },
  },
  rawSpec: { name: "default.yaml", kind: "mihomo" },
};

afterEach(() => {
  localStorage.removeItem("sandrone.locale");
  vi.restoreAllMocks();
});

describe("config file workbench integration", { timeout: 20_000 }, () => {
  it("shows the customized template state only once", () => {
    renderEditor({
      defaultValue: {
        ...minimalConfig,
        groups: [{ name: "Custom", type: "select", proxies: ["$nodes", "DIRECT"] }],
      },
    });

    expect(screen.getAllByText("已自定义")).toHaveLength(1);
  });

  it("describes unsupported advanced structures without restating the selected mode", () => {
    renderEditor({
      adapter: structuredAdapter("sing-box"),
      defaultValue: {
        subscriptions: ["provider"],
        rule_sets: [{
          type: "remote",
          tag: "private",
          format: "source",
          url: "https://example.com/private.json",
          http_client: "rules-client",
        }],
        rules: [{ rule_set: ["private"], outbound: "direct" }],
      },
    });

    expect(screen.getByText("检测到向导暂时无法安全编辑的自定义结构。")).toBeInTheDocument();
  });

  it("keeps the captured Chinese naming language across template replacement", async () => {
    localStorage.setItem("sandrone.locale", "zh-CN");
    const user = userEvent.setup();
    renderEditor({ createNamingLocale: "zh-CN", defaultValue: undefined, mode: "create" });

    expect(currentConfig().groups[0].name).toBe("🚀 节点选择");
    expect(currentConfig().rules.at(-1)).toBe("MATCH,🐟 漏网之鱼");

    await user.click(screen.getByRole("radio", { name: /标准/ }));
    expect(currentConfig().groups.map((group) => group.name)).toContain("🤖 AI 服务");
    expect(currentConfig().rules.at(-1)).toBe("MATCH,🐟 漏网之鱼");

    const addGroup = screen.getByRole("button", { name: "添加代理组" });
    expect(addGroup).toHaveTextContent("添加");
    await user.click(addGroup);
    expect(currentConfig().groups.at(-1)?.name).toBe("自定义");
    await user.click(screen.getByRole("button", { name: /^规则策略/ }));
    const addRule = screen.getByRole("button", { name: "添加规则" });
    expect(addRule).toHaveTextContent("添加");
    await user.click(addRule);
    expect(currentConfig().rules.at(-1)).toBe("RULE-SET,custom,🚀 节点选择");
  });
  it("adds a catalog provider without changing routing rules", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    const onDirty = vi.fn();
    const catalog: RuleSetCatalogResult = {
      items: [{
        name: "geoip-test",
        ruleKind: "ip",
        url: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/test.mrs",
      }],
    };
    renderEditor({
      loadRuleSetCatalog: vi.fn().mockResolvedValue(catalog),
      onDirty,
    });
    const rulesBefore = currentConfig().rules;

    const openCatalog = screen.getByRole("button", { name: "Add from catalog" });
    expect(openCatalog).toHaveTextContent("From catalog");
    await user.click(openCatalog);
    const row = await screen.findByRole("listitem");
    await user.click(within(row).getByRole("button", { name: "Add rule set “geoip-test”" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Rule set catalog" })).not.toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: /Expand rule set \d+ geoip-test/ }));
    const added = screen.getByRole("group", { name: /Rule set \d+ geoip-test/ });
    await user.click(within(added).getByRole("combobox", { name: "Format" }));
    await user.click(screen.getByRole("option", { name: "yaml" }));

    expect(currentConfig().rule_sets.at(-1)).toEqual({
      name: "geoip-test",
      type: "http",
      behavior: "ipcidr",
      format: "yaml",
      interval: 86400,
      url: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/test.yaml",
    });
    expect(currentConfig().rules).toEqual(rulesBefore);
    expect(onDirty).toHaveBeenCalled();
  });

  it("wires preview nodes and live rule-set names into reference fields", async () => {
    const user = userEvent.setup();
    renderEditor({
      defaultValue: {
        ...minimalConfig,
        rule_sets: [
          { name: "private", type: "inline", behavior: "classical", payload: ["DOMAIN-SUFFIX,local"] },
          { name: "ads", type: "inline", behavior: "domain", payload: ["DOMAIN-SUFFIX,ads.example"] },
        ],
      },
      loadSubscriptionPreview: vi.fn().mockResolvedValue(subscriptionPreview("provider", ["HK Node"])),
    });

    expect(await screen.findByText("已加载 1 个节点")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "展开代理组 Proxy" }));
    await user.click(screen.getByRole("combobox", { name: "成员 1" }));
    expect(await screen.findByRole("option", { name: /HK Node.*ss/ })).toBeInTheDocument();
    await user.keyboard("{Escape}");

    await user.click(screen.getByRole("button", { name: /^规则策略/ }));
    await user.click(screen.getByRole("button", { name: "展开规则 1" }));
    const rule = screen.getByRole("group", { name: "规则 1" });
    await user.click(within(rule).getByRole("combobox", { name: "匹配值" }));
    expect(await screen.findByRole("option", { name: "ads" })).toBeInTheDocument();
    await user.keyboard("{Escape}");
    await user.click(within(rule).getByRole("combobox", { name: "策略" }));
    expect(await screen.findByRole("option", { name: /HK Node.*ss/ })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Proxy" })).toBeInTheDocument();
  });

  it("uses Shadowrocket rendered node names for references, including normalized duplicates", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    const onValidityChange = vi.fn();
    const preview = subscriptionPreview(
      "provider",
      ["US,1", "US,1"],
      [{ shadowrocket: "US，1" }, { shadowrocket: "US，1 (2)" }],
    );
    renderEditor({
      adapter: structuredAdapter("shadowrocket"),
      defaultValue: {
        subscriptions: ["provider"],
        groups: [{ name: "Proxy", type: "select", proxies: ["US，1", "US，1 (2)"] }],
        rule_sets: [],
        rules: ["FINAL,US，1 (2)"],
      },
      loadSubscriptionPreview: vi.fn().mockResolvedValue(preview),
      onValidityChange,
    });

    expect(await screen.findByText("Loaded 2 nodes")).toBeInTheDocument();
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true));
    await user.click(screen.getByRole("button", { name: "Expand proxy group Proxy" }));
    await user.click(screen.getByRole("combobox", { name: "Members 1" }));
    const renderedNodeOptions = (await screen.findAllByRole("option"))
      .map((option) => option.textContent)
      .filter((text) => text?.startsWith("US，1"));
    expect(renderedNodeOptions).toEqual(expect.arrayContaining([
      expect.stringMatching(/^US，1ss/),
      expect.stringMatching(/^US，1 \(2\)ss/),
    ]));
  });

  it("uses only authoritative Shadowrocket names from a partial target mapping", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    const onValidityChange = vi.fn();
    const preview = subscriptionPreview(
      "provider",
      ["US,1", "raw unsupported"],
      [{ shadowrocket: "US，1" }, undefined],
    );
    renderEditor({
      adapter: structuredAdapter("shadowrocket"),
      defaultValue: {
        subscriptions: ["provider"],
        groups: [{ name: "Proxy", type: "select", proxies: ["US，1"] }],
        rule_sets: [],
        rules: ["FINAL,US，1"],
      },
      loadSubscriptionPreview: vi.fn().mockResolvedValue(preview),
      onValidityChange,
    });

    expect(await screen.findByText("Loaded 2 nodes")).toBeInTheDocument();
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true));
    await user.click(screen.getByRole("button", { name: "Expand proxy group Proxy" }));
    await user.click(screen.getByRole("combobox", { name: "Members 1" }));
    const nodeOptions = (await screen.findAllByRole("option")).map((option) => option.textContent ?? "");
    expect(nodeOptions).toContainEqual(expect.stringMatching(/^US，1ss/));
    expect(nodeOptions.some((name) => name.includes("raw unsupported"))).toBe(false);
    expect(nodeOptions.some((name) => name.startsWith("US,1"))).toBe(false);
  });

  it("uses server-realized Shadowrocket names when an earlier duplicate is skipped", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    const onValidityChange = vi.fn();
    const preview = subscriptionPreview(
      "provider",
      ["dup", "dup"],
      [{ shadowrocket: "" }, { shadowrocket: "dup" }],
    );
    renderEditor({
      adapter: structuredAdapter("shadowrocket"),
      defaultValue: {
        subscriptions: ["provider"],
        groups: [{ name: "Proxy", type: "select", proxies: ["dup"] }],
        rule_sets: [],
        rules: ["FINAL,dup"],
      },
      loadSubscriptionPreview: vi.fn().mockResolvedValue(preview),
      onValidityChange,
    });

    expect(await screen.findByText("Loaded 2 nodes")).toBeInTheDocument();
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true));
    await user.click(screen.getByRole("button", { name: "Expand proxy group Proxy" }));
    await user.click(screen.getByRole("combobox", { name: "Members 1" }));
    const renderedNames = (await screen.findAllByRole("option")).map((option) => option.textContent ?? "");
    expect(renderedNames.filter((name) => name.startsWith("dup"))).toEqual([expect.stringMatching(/^dupss/)]);
    expect(renderedNames.some((name) => name.includes("dup (2)"))).toBe(false);
  });

  it("blocks Shadowrocket save when a non-empty preview has no renderable nodes", async () => {
    const onValidityChange = vi.fn();
    const preview = subscriptionPreview(
      "provider",
      ["unsupported"],
      [{ shadowrocket: "" }],
    );
    renderEditor({
      adapter: structuredAdapter("shadowrocket"),
      defaultValue: {
        subscriptions: ["provider"],
        groups: [{ name: "Proxy", type: "select", proxies: ["$nodes", "DIRECT"] }],
        rule_sets: [],
        rules: ["FINAL,Proxy"],
      },
      loadSubscriptionPreview: vi.fn().mockResolvedValue(preview),
      onValidityChange,
    });

    expect(await screen.findByText("已加载 1 个节点")).toBeInTheDocument();
    expect(screen.getByText("所选订阅包含节点，但没有可供 Shadowrocket 渲染的节点。")).toBeInTheDocument();
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(false));
  });

  it("rejects and defensively omits unsafe Shadowrocket structured rule fields", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    const onValidityChange = vi.fn();
    renderEditor({
      adapter: structuredAdapter("shadowrocket"),
      defaultValue: {
        groups: [{ name: "Proxy", type: "select", proxies: ["DIRECT"] }],
        rule_sets: [],
        rules: ["DOMAIN,example.com,Proxy"],
      },
      onValidityChange,
    });

    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true));
    await user.click(screen.getByRole("button", { name: /^Rules/ }));
    await user.click(screen.getByRole("button", { name: "Expand rule 1" }));
    const value = within(screen.getByRole("group", { name: "Rule 1" })).getByRole("textbox", { name: "Match value" });
    await user.type(value, ",DIRECT");

    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(false));
    expect(currentConfig().rules).toEqual([]);
  });

  it("rejects a Shadowrocket $nodes-only group when no subscription is selected", async () => {
    const onValidityChange = vi.fn();
    renderEditor({
      adapter: structuredAdapter("shadowrocket"),
      defaultValue: {
        groups: [{ name: "Proxy", type: "select", proxies: ["$nodes"] }],
        rule_sets: [],
        rules: [],
      },
      onValidityChange,
    });

    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(false));
  });

  it("keeps the default Shadowrocket template valid without a Sandrone subscription", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const onValidityChange = vi.fn();
    renderEditor({
      adapter: structuredAdapter("shadowrocket"),
      defaultValue: undefined,
      mode: "create",
      onValidityChange,
    });

    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true));
    expect(currentEnvelope().subscriptions ?? []).toEqual([]);
    expect(currentConfig().groups.find((group) => group.name === "Proxy")).toEqual({
      name: "Proxy",
      type: "select",
      proxies: ["PROXY", "$nodes", "DIRECT", "REJECT"],
    });
    expect(currentConfig().groups.some((group) => group.name === "Auto")).toBe(false);
  });

  it("keeps collapsed remote rule-set validation active", async () => {
    const onValidityChange = vi.fn();
    renderEditor({
      defaultValue: {
        ...minimalConfig,
        rule_sets: [{
          name: "remote",
          type: "http",
          behavior: "domain",
          format: "mrs",
          interval: 86400,
          url: "not-a-url",
        }],
      },
      onValidityChange,
    });

    expect(screen.getByRole("button", { name: "展开规则集 1 remote" })).toHaveAttribute("aria-expanded", "false");
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(false));
  });

  it("updates adaptive groups only after explicit regeneration", async () => {
    const user = userEvent.setup();
    const loadPreview = vi.fn()
      .mockResolvedValueOnce(subscriptionPreview("provider", ["HK-01", "香港-02"]))
      .mockResolvedValueOnce(subscriptionPreview("provider", ["JP-01", "東京-02"]));
    renderEditor({ loadSubscriptionPreview: loadPreview });

    expect(await screen.findByText("已加载 2 个节点")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "生成自适应分组" }));
    expect(screen.getByRole("button", { name: "展开代理组 Hong Kong" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "刷新节点" }));
    await waitFor(() => expect(loadPreview).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.getByRole("button", { name: "重新生成自适应分组" })).toBeEnabled());
    expect(screen.getByRole("button", { name: "展开代理组 Hong Kong" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "展开代理组 Japan" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "重新生成自适应分组" }));
    expect(screen.queryByRole("button", { name: "展开代理组 Hong Kong" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "展开代理组 Japan" })).toBeInTheDocument();
  });

  it("uses displayed Shadowrocket runtime defaults as the adaptive anchor and materializes them on generation", async () => {
    const user = userEvent.setup();
    renderEditor({
      defaultValue: { subscriptions: ["provider"], settingsMode: "structured" },
      adapter: structuredAdapter("shadowrocket"),
      loadSubscriptionPreview: vi.fn().mockResolvedValue(subscriptionPreview("provider", ["HK-01", "香港-02"])),
    });

    expect(currentConfig()).toEqual({});
    expect(await screen.findByText("已加载 2 个节点")).toBeInTheDocument();
    const generate = screen.getByRole("button", { name: "生成自适应分组" });
    expect(generate).toBeEnabled();
    await user.click(generate);

    expect(currentConfig().groups).toEqual(expect.arrayContaining([
      expect.objectContaining({ name: "Proxy", type: "select" }),
      expect.objectContaining({ name: "Hong Kong", "policy-regex-filter": expect.any(String) }),
    ]));
  });

  it("accepts a Shadowrocket health group with omitted optional timing fields", async () => {
    const onValidityChange = vi.fn();
    renderEditor({
      defaultValue: {
        settingsMode: "structured",
        groups: [{ name: "Auto", type: "url-test", proxies: ["DIRECT"] }],
        rule_sets: [],
        rules: ["FINAL,DIRECT"],
      },
      adapter: structuredAdapter("shadowrocket"),
      onValidityChange,
    });

    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true));
  });

  it("keeps sortable group identities aligned after adaptive generation", async () => {
    const user = userEvent.setup();
    renderEditor({
      defaultValue: {
        ...minimalConfig,
        groups: [
          { name: "Proxy", type: "select", proxies: ["$nodes", "DIRECT"] },
          { name: "Custom", type: "select", proxies: ["DIRECT"] },
        ],
      },
      loadSubscriptionPreview: vi.fn().mockResolvedValue(subscriptionPreview("provider", ["HK-01", "香港-02"])),
    });

    expect(await screen.findByText("已加载 2 个节点")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "生成自适应分组" }));
    await user.click(screen.getByRole("button", { name: "添加代理组" }));
    await user.click(screen.getByRole("button", { name: "下移代理组 Hong Kong" }));
    await user.click(screen.getAllByRole("button", { name: "删除代理组 Custom" })[0]);

    const renderedNames = within(screen.getByRole("group", { name: "代理组" }))
      .getAllByRole("button", { name: /展开代理组/ })
      .map((button) => button.getAttribute("aria-label")?.replace("展开代理组 ", ""));
    expect(renderedNames).toEqual(currentConfig().groups.map((group) => group.name));
  });

  it("removes adaptive groups with a template and restores them on undo", async () => {
    const user = userEvent.setup();
    renderEditor({
      loadSubscriptionPreview: vi.fn().mockResolvedValue(subscriptionPreview("provider", ["HK-01", "香港-02"])),
    });

    expect(await screen.findByText("已加载 2 个节点")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "生成自适应分组" }));
    const beforeApply = structuredClone(currentConfig().groups);
    await user.click(screen.getByRole("radio", { name: /标准/ }));
    const dialog = screen.getByRole("dialog", { name: "替换当前配置？" });
    expect(dialog).toHaveTextContent("并移除自适应分组");
    const replace = within(dialog).getByRole("button", { name: "替换当前配置" });
    expect(replace).toHaveTextContent("替换");
    await user.click(replace);
    expect(screen.queryByRole("button", { name: "展开代理组 Hong Kong" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "撤销" }));
    expect(currentConfig().groups).toEqual(beforeApply);
    expect(screen.getByRole("button", { name: "展开代理组 Hong Kong" })).toBeInTheDocument();
  });

  it("marks persisted adaptive option changes dirty at the page boundary", async () => {
    const user = userEvent.setup();
    const onBack = vi.fn();
    render(
      <FileEditPage
        detail={minimalDetail}
        item={item}
        loadSubscriptionPreview={vi.fn().mockResolvedValue(subscriptionPreview("provider", ["HK-01", "香港-02"]))}
        onBack={onBack}
        onPreview={vi.fn()}
        onSave={vi.fn()}
        subscriptions={subscriptions}
      />,
    );

    expect(await screen.findByText("已加载 2 个节点")).toBeInTheDocument();
    await user.click(screen.getByRole("combobox", { name: "代理组类型" }));
    await user.click(await screen.findByRole("option", { name: "load-balance" }));
    fireEvent.change(screen.getByRole("spinbutton", { name: "地区最少节点数" }), { target: { value: "3" } });
    await user.click(screen.getByRole("button", { name: "返回" }));

    expect(onBack).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "放弃修改？" })).toBeInTheDocument();
  });

  it("does not mark the page dirty when template replacement is canceled", async () => {
    const user = userEvent.setup();
    const onBack = vi.fn();
    render(
      <FileEditPage
        detail={minimalDetail}
        item={item}
        onBack={onBack}
        onPreview={vi.fn()}
        onSave={vi.fn()}
        subscriptions={subscriptions}
      />,
    );

    await user.click(screen.getByRole("radio", { name: /完整/ }));
    await user.click(within(screen.getByRole("dialog", { name: "替换当前配置？" })).getByRole("button", { name: "取消" }));
    await user.click(screen.getByRole("button", { name: "返回" }));

    expect(onBack).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("dialog", { name: "放弃修改？" })).not.toBeInTheDocument();
  });

  it("renders one English editor smoke contract", () => {
    localStorage.setItem("sandrone.locale", "en-US");
    renderEditor();

    expect(screen.getByRole("group", { name: "Configuration template" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Rules/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Expand rule set 1 private" })).toBeInTheDocument();
  });
});

function renderEditor(overrides: Partial<Parameters<typeof FileConfigEditor>[0]> = {}) {
  const adapter = overrides.adapter ?? structuredAdapter("mihomo");
  const ui = overrides.ui ?? requireFileDriverUI(adapter.kind);
  return render(
    <FileConfigEditor
      baseEditor={<textarea aria-label="base config" defaultValue="" />}
      defaultValue={minimalConfig}
      mode="edit"
      subscriptions={subscriptions}
      {...overrides}
      adapter={adapter}
      ui={ui}
    />,
  );
}

function structuredAdapter(kind: string) {
  const configuration = requireFileDriver(kind).configuration;
  if (configuration.mode !== "structured") throw new Error(`expected structured driver: ${kind}`);
  return configuration.adapter;
}

interface SerializedConfigEnvelope {
  subscriptions?: string[];
  settings: {
    groups: Array<Record<string, unknown>>;
    rule_sets: Array<Record<string, unknown>>;
    rules: unknown[];
  };
}

function currentEnvelope(): SerializedConfigEnvelope {
  const input = document.querySelector<HTMLInputElement>('input[name="config"]');
  if (!input) throw new Error("expected serialized config input");
  return JSON.parse(input.value) as SerializedConfigEnvelope;
}

function currentConfig(): {
  groups: Array<Record<string, unknown>>;
  rule_sets: Array<Record<string, unknown>>;
  rules: unknown[];
} {
  return currentEnvelope().settings;
}

function subscriptionPreview(
  subscriptionName: string,
  nodeNames: string[],
  targetNames: Array<Record<string, string> | undefined> = [],
): ConfigNodePreviewInput {
  return {
    subscriptionName,
    nodes: nodeNames.map((name, index) => ({
      identity: `node-${index + 1}`,
      after: { name, type: "ss", endpoint: `node-${index + 1}.example:8388` },
      ...(targetNames[index] ? { targetNames: targetNames[index] } : {}),
    })),
    warnings: [],
  };
}
