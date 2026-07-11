import { useState } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { RuleDraft, RuleSetDraft } from "~/features/files/config/model/editor-model";
import { requireFileDriver } from "~/features/files/drivers/registry";
import { requireFileDriverUI } from "~/features/files/editor/file-driver-ui-registry";
import type { FileConfigDraft } from "~/features/files/model/types";

import { RuleListEditor, RuleSetListEditor } from "./rule-editor";

describe("config rule editors", () => {
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
    await user.click(first);
    expect(within(screen.getByRole("group", { name: "规则集 1 private" })).getByRole("textbox", { name: "名称" })).toHaveValue("private");
    await user.click(second);
    expect(within(screen.getByRole("group", { name: "规则集 1 private" })).queryByRole("textbox", { name: "名称" })).not.toBeInTheDocument();
    expect(within(screen.getByRole("group", { name: "规则集 2 private" })).getByRole("textbox", { name: "名称" })).toHaveValue("private");
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

function RuleSetHarness({ initial, kind = "mihomo" }: { initial: RuleSetDraft[]; kind?: string }) {
  const [ruleSets, setRuleSets] = useState(initial);
  const adapter = structuredAdapter(kind);
  return (
    <RuleSetListEditor
      adapter={adapter}
      inboundReferences={{ private: 2 }}
      issues={[]}
      ruleSets={ruleSets}
      ui={requireFileDriverUI(kind)}
      onChange={setRuleSets}
    />
  );
}

function RuleHarness({ initial, kind = "mihomo", ruleSets }: { initial: RuleDraft[]; kind?: string; ruleSets: RuleSetDraft[] }) {
  const [rules, setRules] = useState(initial);
  const adapter = structuredAdapter(kind);
  return (
    <RuleListEditor
      adapter={adapter}
      defaultExpanded
      groups={adapter.groups.project([{ name: "Proxy", type: "select", proxies: ["$nodes", "DIRECT"] }]) ?? []}
      issues={[]}
      nodes={[]}
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
