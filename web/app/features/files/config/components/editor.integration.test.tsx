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
  groups: [{
    name: "Proxy",
    type: "select",
    proxies: ["$nodes", "DIRECT"],
  }],
  rule_sets: [{
    name: "private",
    type: "inline",
    behavior: "classical",
    payload: ["DOMAIN-SUFFIX,local"],
  }],
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
  it("projects subscription preview nodes into a nested editor", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    renderEditor({
      loadSubscriptionPreview: vi.fn().mockResolvedValue(
        subscriptionPreview("provider", ["HK Node"]),
      ),
    });

    const template = screen.getByRole("group", {
      name: "Configuration template",
    });
    const nodeSource = screen.getByRole("group", { name: "Node source" });
    expect(screen.getByRole("heading", { name: "Configuration content" }))
      .toBeInTheDocument();
    expect(template.compareDocumentPosition(nodeSource))
      .toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(await screen.findByText("Loaded 1 nodes")).toBeInTheDocument();

    await user.click(screen.getByRole("button", {
      name: "Expand proxy group Proxy",
    }));
    await user.click(screen.getByRole("combobox", { name: "Members 1" }));
    expect(await screen.findByRole("option", { name: /HK Node.*ss/ }))
      .toBeInTheDocument();
  });

  it("projects a catalog choice into the hidden serialized output", async () => {
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

    await user.click(screen.getByRole("button", { name: "Add from catalog" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Rule set catalog",
    });
    await user.click(within(dialog).getByRole("button", {
      name: "Add rule set “geosite-cn”",
    }));

    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: "Rule set catalog" }))
        .not.toBeInTheDocument();
    });
    expect(currentConfig().rule_sets).toContainEqual({
      name: "geosite-cn",
      type: "http",
      behavior: "domain",
      format: "mrs",
      interval: 86400,
      url: entry.url,
    });
  });

  it("applies a template and restores the serialized structure on undo", async () => {
    const user = userEvent.setup();
    const adapter = structuredAdapter("mihomo");
    const generation = adapter.adaptive.generate(
      ["HK-01", "香港-02"],
      adapter.adaptive.defaultOptions(),
    );
    const startingDraft = adapter.adaptive.merge(
      minimalConfig,
      generation,
    ).config;
    renderEditor({ adapter, defaultValue: startingDraft });

    const startingGroups = structuredClone(currentConfig().groups);
    await user.click(screen.getByRole("radio", { name: /标准/ }));
    const dialog = screen.getByRole("dialog", { name: "替换当前配置？" });
    await user.click(within(dialog).getByRole("button", {
      name: "替换当前配置",
    }));
    expect(screen.queryByRole("button", {
      name: "展开代理组 Hong Kong",
    })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "撤销" }));
    expect(currentConfig().groups).toEqual(startingGroups);
  });

  it("updates hidden output and validity after a structured edit", async () => {
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

    await waitFor(() => {
      expect(onValidityChange).toHaveBeenLastCalledWith(true);
    });
    await user.click(screen.getByRole("button", { name: /^Rules/ }));
    await user.click(screen.getByRole("button", { name: "Expand rule 1" }));
    const rule = screen.getByRole("group", { name: "Rule 1" });
    fireEvent.change(within(rule).getByRole("textbox", {
      name: "Match value",
    }), {
      target: { value: "example.com,DIRECT" },
    });

    await waitFor(() => {
      expect(onValidityChange).toHaveBeenLastCalledWith(false);
    });
    expect(currentConfig().rules).toEqual([]);
  });

  it("calls the explicit dirty binding for adaptive option changes", async () => {
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
    await user.click(await screen.findByRole("option", {
      name: "load-balance",
    }));

    expect(onDirty).toHaveBeenCalled();
  });

  it("keeps an invalid persisted file shareable at the edit-page boundary", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    render(
      <FileEditPage
        detail={{
          ...minimalDetail,
          config: {
            ...minimalDetail.config!,
            subscriptions: ["provider", "legacy-provider"],
          },
        }}
        item={item}
        onBack={vi.fn()}
        onPreview={vi.fn()}
        onSave={vi.fn()}
        onShare={vi.fn()}
        subscriptions={subscriptions}
      />,
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Save file" })).toBeDisabled();
    });
    const share = screen.getByRole("button", { name: "Share file" });
    expect(share).toBeEnabled();
    expect(share).toHaveTextContent(/^Share$/);
  });

  it("updates file edit sharing across source and processor changes with a successful save", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    const onSave = vi.fn(async () => undefined);
    render(
      <FileEditPage
        detail={{
          ...minimalDetail,
          source: {
            type: "remote",
            remote: { url: "https://example.com/base.yaml" },
          },
          processors: [
            {
              name: "Inline script",
              type: "script",
              stage: "file",
              params: {
                source: {
                  type: "inline",
                  content: "function main(input) { return input; }",
                },
              },
            },
          ],
        }}
        item={item}
        onBack={vi.fn()}
        onPreview={vi.fn()}
        onSave={onSave}
        onShare={vi.fn()}
        subscriptions={subscriptions}
      />,
    );

    const share = screen.getByRole("button", { name: "Share file" });
    expect(share).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "Local" }));
    expect(share).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Save file" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(share).toBeEnabled());

    await user.click(screen.getByRole("button", { name: "Delete processor" }));
    expect(share).toBeDisabled();
  });
});

function renderEditor(
  overrides: Partial<Parameters<typeof FileConfigEditor>[0]> = {},
) {
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
  if (configuration.mode !== "structured") {
    throw new Error(`expected structured driver: ${kind}`);
  }
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

function currentConfig(): SerializedConfigEnvelope["settings"] {
  const input = document.querySelector<HTMLInputElement>(
    'input[name="config"]',
  );
  if (!input) throw new Error("expected serialized config input");
  return (JSON.parse(input.value) as SerializedConfigEnvelope).settings;
}

function subscriptionPreview(
  subscriptionName: string,
  nodeNames: string[],
): ConfigNodePreviewInput {
  return {
    subscriptionName,
    nodes: nodeNames.map((name, index) => ({
      identity: `node-${index + 1}`,
      after: {
        name,
        type: "ss",
        endpoint: `node-${index + 1}.example:8388`,
      },
    })),
    warnings: [],
  };
}
