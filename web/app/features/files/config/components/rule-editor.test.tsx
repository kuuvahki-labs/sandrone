import { useState } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { RuleDraft, RuleSetDraft } from "~/features/files/config/model/editor-model";
import type { ConfigNodeSummary } from "~/features/files/config/model/node-source";
import { requireFileDriver } from "~/features/files/drivers/registry";
import { requireFileDriverUI } from "~/features/files/editor/file-driver-ui-registry";
import type { FileConfigDraft } from "~/features/files/model/types";

import { RuleListEditor, RuleSetListEditor } from "./rule-editor";

describe("config rule editors", () => {
  it("adds a rule set from the footer and focuses its name", async () => {
    const user = userEvent.setup();
    render(<RuleSetHarness initial={[]} />);

    const section = screen.getByRole("group", { name: "规则集" });
    const add = within(section).getByRole("button", { name: "添加规则集" });
    expect(add.closest('[data-slot="config-list-actions"]')).not.toBeNull();

    await user.click(add);

    expect(within(section).getByRole("textbox", { name: "名称" })).toHaveFocus();
    expect(within(section).getByRole("button", { name: /收起规则集/ }))
      .toHaveAttribute("aria-expanded", "true");
  });

  it("uses short visible library copy with a complete accessible name", async () => {
    const user = userEvent.setup();
    const onOpenCatalog = vi.fn();
    render(<RuleSetHarness initial={[]} onOpenCatalog={onOpenCatalog} />);

    const open = screen.getByRole("button", { name: "从规则集库添加" });
    expect(open).toHaveTextContent("从库添加");
    await user.click(open);
    expect(onOpenCatalog).toHaveBeenCalledOnce();
  });

  it("adds a rule from the footer and focuses its type", async () => {
    const user = userEvent.setup();
    render(<RuleHarness initial={[]} ruleSets={[]} />);

    const section = screen.getByRole("group", { name: "规则策略" });
    const add = within(section).getByRole("button", { name: "添加规则" });
    expect(add.closest('[data-slot="config-list-actions"]')).not.toBeNull();

    await user.click(add);

    expect(within(section).getByRole("combobox", { name: "类型" })).toHaveFocus();
    expect(within(section).getByRole("button", { name: "收起规则 1" }))
      .toHaveAttribute("aria-expanded", "true");
  });

  it("forwards live rule-set, group, and node references into rule fields", async () => {
    const user = userEvent.setup();
    const config = initialConfig("mihomo", {
      groups: [{ name: "Proxy", type: "select", proxies: ["$nodes", "DIRECT"] }],
      rule_sets: [
        { name: "private", type: "inline", behavior: "classical", payload: ["DOMAIN-SUFFIX,local"] },
        { name: "ads", type: "inline", behavior: "domain", payload: ["DOMAIN-SUFFIX,ads.example"] },
      ],
      rules: ["RULE-SET,private,DIRECT"],
    });
    render(
      <RuleHarness
        initial={config.rules}
        nodes={[{ key: "hk", name: "HK Node", type: "ss", endpoint: "hk.example:8388" }]}
        ruleSets={config.ruleSets}
      />,
    );

    await user.click(screen.getByRole("button", { name: "展开规则 1" }));
    const rule = screen.getByRole("group", { name: "规则 1" });
    await user.click(within(rule).getByRole("combobox", { name: "匹配值" }));
    expect(await screen.findByRole("option", { name: "ads" })).toBeInTheDocument();
    await user.keyboard("{Escape}");
    await user.click(within(rule).getByRole("combobox", { name: "策略" }));
    expect(await screen.findByRole("option", { name: "Proxy" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /HK Node.*ss/ })).toBeInTheDocument();
    await user.keyboard("{Escape}");
  });

  it("keeps one indexed rule-set editor open and distinguishes duplicate names", async () => {
    const user = userEvent.setup();
    const drafts = initialConfig("mihomo", {
      rule_sets: [
        { name: "private", type: "inline", behavior: "classical", payload: ["DOMAIN-SUFFIX,local"] },
        { name: "private", type: "inline", behavior: "domain", payload: ["DOMAIN-SUFFIX,example.com"] },
      ],
    }).ruleSets;
    render(<RuleSetHarness initial={drafts} />);

    const first = screen.getByRole("button", { name: "展开规则集 1 private" });
    const second = screen.getByRole("button", { name: "展开规则集 2 private" });
    expect(first).toHaveAttribute("aria-expanded", "false");
    const firstRow = screen.getByRole("group", { name: "规则集 1 private" });
    const secondRow = screen.getByRole("group", { name: "规则集 2 private" });
    await user.click(within(firstRow).getByText("private", { exact: true }));
    expect(within(firstRow).getByRole("textbox", { name: "名称" })).toHaveValue("private");
    await user.click(within(secondRow).getByText("private", { exact: true }));
    expect(within(firstRow).queryByRole("textbox", { name: "名称" })).not.toBeInTheDocument();
    expect(within(secondRow).getByRole("textbox", { name: "名称" })).toHaveValue("private");
    expect(second).toHaveAttribute("aria-expanded", "true");
  });

  it("edits no-resolve and closes an expanded rule after reordering", async () => {
    const user = userEvent.setup();
    const config = initialConfig("mihomo", {
      groups: [{ name: "Proxy", type: "select", proxies: ["$nodes", "DIRECT"] }],
      rule_sets: [{ name: "private", type: "inline", behavior: "classical", payload: ["DOMAIN-SUFFIX,local"] }],
      rules: ["RULE-SET,private,DIRECT", "MATCH,Proxy"],
    });
    render(<RuleHarness initial={config.rules} ruleSets={config.ruleSets} />);

    await user.click(screen.getByRole("button", { name: "展开规则 1" }));
    const first = screen.getByRole("group", { name: "规则 1" });
    await user.click(within(first).getByRole("checkbox", { name: "no-resolve" }));
    expect(within(first).getByRole("checkbox", { name: "no-resolve" })).toBeChecked();
    await user.click(screen.getByRole("button", { name: "下移规则 1" }));

    expect(screen.queryByRole("button", { name: "收起规则 1" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /展开规则/ })).toHaveLength(2);
  });

  it("keeps Shadowrocket rule sets remote-only with exact reference types", async () => {
    const user = userEvent.setup();
    const config = initialConfig("shadowrocket", {
      rule_sets: [{ name: "ads", type: "domain-set", url: "https://example.com/ads.list" }],
      rules: ["DOMAIN-SET,ads,REJECT", "FINAL,Proxy"],
    });
    render(<RuleSetHarness initial={config.ruleSets} kind="shadowrocket" />);

    await user.click(screen.getByRole("button", { name: "展开规则集 1 ads" }));
    const row = screen.getByRole("group", { name: "规则集 1 ads" });
    const behavior = within(row).getByRole("combobox", { name: "类型" });
    await user.click(behavior);
    expect((await screen.findAllByRole("option")).map((option) => option.textContent)).toEqual(["RULE-SET", "DOMAIN-SET"]);
    await user.keyboard("{Escape}");
    expect(within(row).getByRole("textbox", { name: "URL" })).toHaveValue("https://example.com/ads.list");
    expect(within(row).queryByRole("button", { name: /内联|远程/ })).not.toBeInTheDocument();
    expect(within(row).queryByRole("combobox", { name: "格式" })).not.toBeInTheDocument();
    expect(within(row).queryByRole("textbox", { name: /更新间隔/ })).not.toBeInTheDocument();
  });

  it("offers common Shadowrocket string rules and only meaningful no-resolve", async () => {
    const user = userEvent.setup();
    const config = initialConfig("shadowrocket", {
      groups: [{ name: "Proxy", type: "select", proxies: ["$nodes", "DIRECT"] }],
      rule_sets: [{ name: "ads", type: "domain-set", url: "https://example.com/ads.list" }],
      rules: ["DOMAIN-SET,ads,REJECT", "FINAL,Proxy"],
    });
    render(<RuleHarness initial={config.rules} ruleSets={config.ruleSets} kind="shadowrocket" />);

    await user.click(screen.getByRole("button", { name: "展开规则 1" }));
    const first = screen.getByRole("group", { name: "规则 1" });
    await user.click(within(first).getByRole("combobox", { name: "类型" }));
    const ruleTypes = (await screen.findAllByRole("option")).map((option) => option.textContent);
    expect(ruleTypes).toEqual(expect.arrayContaining([
      "DOMAIN-SUFFIX", "RULE-SET", "DOMAIN-SET", "GEOIP", "IP-CIDR", "FINAL",
    ]));
    expect(ruleTypes).not.toEqual(expect.arrayContaining(["PROCESS-NAME", "IP-CIDR6"]));
    await user.click(screen.getByRole("option", { name: "IP-CIDR" }));
    expect(within(first).getByRole("checkbox", { name: "no-resolve" })).toBeInTheDocument();

    await user.click(within(first).getByRole("combobox", { name: "类型" }));
    await user.click(screen.getByRole("option", { name: "FINAL" }));
    expect(within(first).queryByRole("checkbox", { name: "no-resolve" })).not.toBeInTheDocument();
    expect(within(first).getByRole("textbox", { name: /匹配值|Rule value/i })).toBeDisabled();
  });
});

function RuleSetHarness({ initial, kind = "mihomo", onOpenCatalog }: {
  initial: RuleSetDraft[];
  kind?: string;
  onOpenCatalog?: () => void;
}) {
  const [ruleSets, setRuleSets] = useState(initial);
  const adapter = structuredAdapter(kind);
  return (
    <RuleSetListEditor
      adapter={adapter}
      inboundReferences={{ private: 2 }}
      issues={[]}
      onOpenCatalog={onOpenCatalog}
      ruleSets={ruleSets}
      ui={requireFileDriverUI(kind)}
      onChange={setRuleSets}
    />
  );
}

function RuleHarness({ initial, kind = "mihomo", nodes = [], ruleSets }: {
  initial: RuleDraft[];
  kind?: string;
  nodes?: ConfigNodeSummary[];
  ruleSets: RuleSetDraft[];
}) {
  const [rules, setRules] = useState(initial);
  const adapter = structuredAdapter(kind);
  return (
    <RuleListEditor
      adapter={adapter}
      defaultExpanded
      groups={adapter.groups.project([{ name: "Proxy", type: "select", proxies: ["$nodes", "DIRECT"] }]) ?? []}
      issues={[]}
      nodes={nodes}
      rules={rules}
      ruleSets={ruleSets}
      ui={requireFileDriverUI(kind)}
      onChange={setRules}
    />
  );
}

function structuredAdapter(kind: string) {
  const configuration = requireFileDriver(kind).configuration;
  if (configuration.mode !== "structured") throw new Error(`expected structured driver: ${kind}`);
  return configuration.adapter;
}

function initialConfig(kind: string, value: FileConfigDraft) {
  return structuredAdapter(kind).initialize(value);
}
