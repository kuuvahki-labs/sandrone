import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ConfigNodePreviewInput } from "~/features/files/config/model/node-source";
import { requireFileDriver } from "~/features/files/drivers/registry";
import { requireFileDriverUI } from "~/features/files/editor/file-driver-ui-registry";
import type {
  FileConfigDraft,
  FileDetail,
  FileItem,
  RuleSetCatalogResult,
} from "~/features/files/model/types";
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
  });

  it("wires preview nodes and live references into nested editors", async () => {
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
    await user.keyboard("{Escape}");
  });

  it("adds a catalog rule set without changing routing rules", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    const entry = {
      name: "geosite-cn",
      ruleKind: "domain",
      url: "https://raw.githubusercontent.com/example/catalog/main/geosite-cn.mrs",
    } as const;
    const catalog: RuleSetCatalogResult = { items: [entry] };
    renderEditor({
      loadRuleSetCatalog: vi.fn().mockResolvedValue(catalog),
    });

    const rulesBefore = currentConfig().rules;
    await user.click(screen.getByRole("button", { name: "Add from catalog" }));
    const dialog = await screen.findByRole("dialog", { name: "Rule set catalog" });
    await user.click(within(dialog).getByRole("button", { name: "Add rule set “geosite-cn”" }));

    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: "Rule set catalog" })).not.toBeInTheDocument();
    });
    expect(currentConfig().rule_sets).toContainEqual({
      name: "geosite-cn",
      type: "http",
      behavior: "domain",
      format: "mrs",
      interval: 86400,
      url: entry.url,
    });
    expect(currentConfig().rules).toEqual(rulesBefore);
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
    fireEvent.change(value, { target: { value: "example.com,DIRECT" } });

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

  it("keeps runtime-filter groups stable when the subscription preview changes", async () => {
    const user = userEvent.setup();
    const loadPreview = vi.fn()
      .mockResolvedValueOnce(subscriptionPreview("provider", ["HK-01", "香港-02"]))
      .mockResolvedValueOnce(subscriptionPreview("provider", ["JP-01", "東京-02"]));
    renderEditor({ loadSubscriptionPreview: loadPreview });

    expect(await screen.findByText("已加载 2 个节点")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "生成自适应分组" }));
    expect(screen.getByRole("button", { name: "展开代理组 Hong Kong" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "展开代理组 Japan" })).toBeInTheDocument();
    const generated = structuredClone(currentConfig().groups);

    await user.click(screen.getByRole("button", { name: "刷新节点" }));
    await waitFor(() => expect(loadPreview).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.getByRole("button", { name: "重新生成自适应分组" })).toBeEnabled());
    expect(currentConfig().groups).toEqual(generated);
  });

  it.each(["mihomo", "shadowrocket"] as const)(
    "generates the five default runtime-filter groups for %s without selecting a subscription",
    async (kind) => {
      const user = userEvent.setup();
      const adapter = structuredAdapter(kind);
      renderEditor({
        adapter,
        defaultValue: adapter.templates.create("minimal"),
      });

      const generate = screen.getByRole("button", { name: "生成自适应分组" });
      expect(generate).toBeEnabled();
      await user.click(generate);

      const nameKey = kind === "shadowrocket" ? "policy-regex-filter" : "include-all-proxies";
      expect(currentConfig().groups.filter((group) => group[nameKey] !== undefined).map((group) => group.name)).toEqual([
        "Hong Kong",
        "Taiwan",
        "Singapore",
        "Japan",
        "United States",
      ]);
    },
  );

  it("keeps sing-box adaptive generation disabled without a subscription preview", () => {
    const adapter = structuredAdapter("sing-box");
    renderEditor({
      adapter,
      defaultValue: adapter.templates.create("minimal"),
    });

    expect(screen.getByRole("button", { name: "生成自适应分组" })).toBeDisabled();
    expect(screen.getByText("请先选择订阅。")).toBeInTheDocument();
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

  it("removes adaptive groups with a template and restores them on undo", async () => {
    const user = userEvent.setup();
    const adapter = structuredAdapter("mihomo");
    const generation = adapter.adaptive.generate(
      ["HK-01", "香港-02"],
      adapter.adaptive.defaultOptions(),
    );
    const startingDraft = adapter.adaptive.merge(minimalConfig, generation).config;
    renderEditor({ adapter, defaultValue: startingDraft });

    const startingGroups = structuredClone(currentConfig().groups);
    await user.click(screen.getByRole("radio", { name: /标准/ }));
    const dialog = screen.getByRole("dialog", { name: "替换当前配置？" });
    expect(dialog).toHaveTextContent("并移除自适应分组");
    const replace = within(dialog).getByRole("button", { name: "替换当前配置" });
    expect(replace).toHaveTextContent("替换");
    await user.click(replace);
    expect(screen.queryByRole("button", { name: "展开代理组 Hong Kong" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "撤销" }));
    expect(currentConfig().groups).toEqual(startingGroups);
    expect(screen.getByRole("button", { name: "展开代理组 Hong Kong" })).toBeInTheDocument();
  });

  it("marks persisted adaptive option changes dirty at the page boundary", async () => {
    const user = userEvent.setup();
    const adapter = structuredAdapter("mihomo");
    const options = adapter.adaptive.defaultOptions();
    const generation = adapter.adaptive.generate(
      ["HK-01", "香港-02"],
      options,
    );
    const persistedDraft = {
      ...adapter.adaptive.merge(minimalConfig, generation).config,
      adaptive_groups: adapter.adaptive.configFromOptions(options),
    };
    const onDirty = vi.fn();
    render(
      <FileConfigEditor
        adapter={adapter}
        baseEditor={<textarea aria-label="base config" defaultValue="" />}
        defaultValue={persistedDraft}
        mode="edit"
        onDirty={onDirty}
        subscriptions={subscriptions}
        ui={requireFileDriverUI("mihomo")}
      />,
    );

    await user.click(screen.getByRole("combobox", { name: "代理组类型" }));
    await user.click(await screen.findByRole("option", { name: "load-balance" }));

    expect(onDirty).toHaveBeenCalled();
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

    const template = screen.getByRole("group", { name: "Configuration template" });
    const nodeSource = screen.getByRole("group", { name: "Node source" });

    expect(screen.getByRole("heading", { name: "Configuration content" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Base configuration content" })).toBeInTheDocument();
    expect(template.compareDocumentPosition(nodeSource)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(screen.queryByRole("heading", { name: "Configuration details" })).not.toBeInTheDocument();
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
): ConfigNodePreviewInput {
  return {
    subscriptionName,
    nodes: nodeNames.map((name, index) => ({
      identity: `node-${index + 1}`,
      after: { name, type: "ss", endpoint: `node-${index + 1}.example:8388` },
    })),
    warnings: [],
  };
}
