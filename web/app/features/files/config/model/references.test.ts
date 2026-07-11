import { describe, expect, it } from "vitest";

import { requireFileDriver } from "~/features/files/drivers/registry";

import type { ConfigNodeSummary } from "./node-source";
import {
  memberReferenceOptions as memberOptions,
  policyReferenceOptions as policyOptions,
  ruleSetReferenceOptions,
} from "./references";

type ConfigKind = "mihomo" | "sing-box" | "shadowrocket";

const nodes: ConfigNodeSummary[] = [
  { key: "hk", name: "HK Node", type: "ss", endpoint: "hk.example:8388" },
  { key: "jp", name: "JP Node", type: "vmess", endpoint: "jp.example:443" },
];

describe("config reference options", () => {
  it("builds ordered Mihomo member and policy suggestions from live definitions", () => {
    const groups = [
      { name: "Proxy", type: "select", proxies: ["$nodes"] },
      { name: "Auto", type: "url-test", proxies: ["$nodes"] },
    ];

    expect(memberReferenceOptions("mihomo", nodes, groups, "Proxy").map(optionKey)).toEqual([
      "macro:$nodes",
      "node:HK Node",
      "node:JP Node",
      "group:Auto",
      "builtin:DIRECT",
      "builtin:REJECT",
      "builtin:REJECT-DROP",
      "builtin:PASS",
      "builtin:PASS-RULE",
      "builtin:COMPATIBLE",
      "builtin:GLOBAL",
    ]);
    expect(policyReferenceOptions("mihomo", nodes, groups).map(optionKey)).toEqual([
      "node:HK Node",
      "node:JP Node",
      "group:Proxy",
      "group:Auto",
      "builtin:DIRECT",
      "builtin:REJECT",
      "builtin:REJECT-DROP",
      "builtin:PASS",
      "builtin:PASS-RULE",
      "builtin:COMPATIBLE",
      "builtin:GLOBAL",
    ]);
  });

  it("uses sing-box names and deduplicates colliding suggestions by value", () => {
    const groups = [
      { tag: "Proxy", type: "selector", outbounds: ["$nodes"] },
      { tag: "HK Node", type: "selector", outbounds: ["direct"] },
    ];

    expect(memberReferenceOptions("sing-box", nodes, groups, "Proxy").map(optionKey)).toEqual([
      "macro:$nodes",
      "node:HK Node",
      "node:JP Node",
      "builtin:direct",
      "builtin:block",
    ]);
    expect(ruleSetReferenceOptions([
      { name: " private " },
      { name: "private" },
      { name: "" },
      { name: "ads" },
    ])).toEqual(["private", "ads"]);
  });

  it("does not offer Shadowrocket nodes whose rendered names shadow built-in policies", () => {
    const shadowrocketNodes = [
      { key: "direct-upper", name: "DIRECT", type: "ss", endpoint: "one.example:443" },
      { key: "direct-lower", name: "direct", type: "ss", endpoint: "two.example:443" },
      { key: "reject-mixed", name: "ReJeCt", type: "ss", endpoint: "three.example:443" },
      { key: "proxy", name: "PROXY", type: "ss", endpoint: "four.example:443" },
      { key: "tailscale", name: "TAILSCALE", type: "ss", endpoint: "five.example:443" },
      { key: "reject-drop", name: "REJECT-DROP", type: "ss", endpoint: "six.example:443" },
      { key: "hk", name: "HK Node", type: "ss", endpoint: "hk.example:443" },
    ];
    const groups = [{ name: "Proxy", type: "select", proxies: ["$nodes"] }];

    expect(memberReferenceOptions("shadowrocket", shadowrocketNodes, groups, "Proxy").map(optionKey)).toEqual([
      "macro:$nodes",
      "node:HK Node",
      "builtin:PROXY",
      "builtin:DIRECT",
      "builtin:REJECT",
    ]);
    expect(policyReferenceOptions("shadowrocket", shadowrocketNodes, groups).map(optionKey)).toEqual([
      "node:HK Node",
      "group:Proxy",
      "builtin:PROXY",
      "builtin:DIRECT",
      "builtin:TAILSCALE",
      "builtin:REJECT",
      "builtin:REJECT-DICT",
      "builtin:REJECT-ARRAY",
      "builtin:REJECT-200",
      "builtin:REJECT-IMG",
      "builtin:REJECT-TINYGIF",
      "builtin:REJECT-DROP",
      "builtin:REJECT-NO-DROP",
    ]);
  });
});

function optionKey(option: { kind: string; value: string }): string {
  return `${option.kind}:${option.value}`;
}

function memberReferenceOptions(
  kind: ConfigKind,
  values: readonly ConfigNodeSummary[],
  nativeGroups: Record<string, unknown>[],
  currentGroup: string,
) {
  const adapter = structuredAdapter(kind);
  const groups = nativeGroups.map((group) => ({ name: String(kind === "sing-box" ? group.tag ?? "" : group.name ?? "") }));
  return memberOptions(adapter.references, values, groups, currentGroup);
}

function policyReferenceOptions(
  kind: ConfigKind,
  values: readonly ConfigNodeSummary[],
  nativeGroups: Record<string, unknown>[],
) {
  const adapter = structuredAdapter(kind);
  const groups = nativeGroups.map((group) => ({ name: String(kind === "sing-box" ? group.tag ?? "" : group.name ?? "") }));
  return policyOptions(adapter.references, values, groups);
}

function structuredAdapter(kind: ConfigKind) {
  const driver = requireFileDriver(kind);
  if (driver.configuration.mode !== "structured") throw new Error(`${kind} is not structured`);
  return driver.configuration.adapter;
}
