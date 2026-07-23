import { describe, expect, it } from "vitest";

import { requireFileDriver } from "~/features/files/drivers/registry";
import type { FileConfigDraft, FileKind } from "~/features/files/model/types";

import { buildConfigRelationModel as buildProjectedConfigRelationModel } from "./relations";
import type { ConfigTemplateID } from "./templates";

const CONFIG_KINDS = ["mihomo", "sing-box", "shadowrocket"] as const satisfies readonly FileKind[];
const TEMPLATE_IDS = ["minimal", "standard", "full"] as const;
type ConfigKind = typeof CONFIG_KINDS[number];

describe("config relation model", () => {
  it.each(CONFIG_KINDS)("accepts every generated %s template and counts inbound references", (kind) => {
    for (const templateID of TEMPLATE_IDS) {
      const config = createConfigFromTemplate(kind, templateID);
      const model = buildConfigRelationModel(kind, config.groups ?? [], config.rule_sets ?? [], config.rules ?? []);

      expect(model.issues).toEqual([]);
      expect(Object.keys(model.groupInboundReferences)).toHaveLength(config.groups?.length ?? 0);
      expect(Object.keys(model.ruleSetInboundReferences)).toHaveLength(config.rule_sets?.length ?? 0);
      expect(Object.values(model.ruleSetInboundReferences).every((count) => count === 1)).toBe(true);
      expect(model.groupInboundReferences.Final).toBeGreaterThan(0);
    }
  });

  it.each([
    {
      kind: "mihomo" as const,
      groups: [
        { name: "A", type: "select", proxies: ["B", "$nodes", "DIRECT"] },
        { name: "B", type: "select", proxies: ["Node 1"] },
      ],
      ruleSets: [{ name: "known", type: "inline", behavior: "domain", payload: [] }],
      rules: ["RULE-SET,known,B", "MATCH,A"],
    },
    {
      kind: "sing-box" as const,
      groups: [
        { type: "selector", tag: "A", outbounds: ["B", "$nodes", "direct"] },
        { type: "selector", tag: "B", outbounds: ["Node 1"] },
      ],
      ruleSets: [{ type: "inline", tag: "known", rules: [] }],
      rules: [{ rule_set: ["known"], outbound: "B" }, { outbound: "A" }],
    },
  ])("counts group and rule-set inbound references for $kind", ({ kind, groups, ruleSets, rules }) => {
    const model = buildConfigRelationModel(kind, groups, ruleSets, rules);

    expect(model.groupInboundReferences).toEqual({ A: 1, B: 2 });
    expect(model.ruleSetInboundReferences).toEqual({ known: 1 });
    expect(model.issues).toEqual([]);
  });

  it.each(CONFIG_KINDS)("reports empty and duplicate %s group and rule-set names on every affected item", (kind) => {
    const groups = kind === "sing-box"
      ? [{ tag: "", outbounds: [] }, { tag: "A", outbounds: [] }, { tag: " A ", outbounds: [] }]
      : [{ name: "", proxies: [] }, { name: "A", proxies: [] }, { name: " A ", proxies: [] }];
    const ruleSets = kind === "sing-box"
      ? [{ tag: "" }, { tag: "known" }, { tag: " known " }]
      : kind === "shadowrocket"
        ? [
          { name: "", type: "rule-set", url: "https://example.com/empty.list" },
          { name: "known", type: "rule-set", url: "https://example.com/known.list" },
          { name: " known ", type: "rule-set", url: "https://example.com/known-copy.list" },
        ]
        : [{ name: "" }, { name: "known" }, { name: " known " }];
    const model = buildConfigRelationModel(kind, groups, ruleSets, []);

    expect(issueKeys(model)).toEqual([
      "error:group_name_empty:groups:group-0",
      "error:group_name_duplicate:groups:group-1",
      "error:group_name_duplicate:groups:group-2",
      "error:rule_set_name_empty:rule_sets:ruleset-0",
      "error:rule_set_name_duplicate:rule_sets:ruleset-1",
      "error:rule_set_name_duplicate:rule_sets:ruleset-2",
    ]);
    expect(model.issues.every((issue) => issue.message.length > 0)).toBe(true);
  });

  it.each([
    {
      kind: "mihomo" as const,
      groups: [{ name: "Proxy", proxies: ["$nodes", "DIRECT"] }],
      rules: ["RULE-SET,missing,Missing Group", "MATCH,DIRECT"],
    },
    {
      kind: "sing-box" as const,
      groups: [{ tag: "Proxy", outbounds: ["$nodes", "direct"] }],
      rules: [{ rule_set: ["missing"], outbound: "Missing Group" }, { outbound: "direct" }],
    },
  ])("reports unknown $kind rule-set references and warns about unresolved policies", ({ kind, groups, rules }) => {
    const model = buildConfigRelationModel(kind, groups, [], rules);

    expect(issueKeys(model)).toEqual([
      "error:unknown_rule_set:rules:rule-0",
      "warning:unknown_rule_policy:rules:rule-0",
    ]);
    expect(model.issues.map((issue) => issue.reference)).toEqual(["missing", "Missing Group"]);
  });

  it.each([
    {
      kind: "mihomo" as const,
      groups: [{ name: "Proxy", proxies: ["$nodes"] }],
      rules: ["RULE-SET, ,Proxy"],
    },
    {
      kind: "sing-box" as const,
      groups: [{ tag: "Proxy", outbounds: ["$nodes"] }],
      rules: [{ rule_set: [" "], outbound: "Proxy" }],
    },
    {
      kind: "sing-box" as const,
      groups: [{ tag: "Proxy", outbounds: ["$nodes"] }],
      rules: [{ rule_set: [], outbound: "Proxy" }],
    },
  ])("reports an empty $kind rule-set reference as an error", ({ kind, groups, rules }) => {
    const model = buildConfigRelationModel(kind, groups, [], rules);

    expect(model.issues).toEqual([{
      severity: "error",
      code: "rule_set_reference_empty",
      section: "rules",
      itemId: "rule-0",
      message: "Rule-set reference is required.",
    }]);
  });

  it.each([
    {
      kind: "mihomo" as const,
      groups: [{ name: "Proxy", proxies: ["$nodes"] }],
      ruleSets: [{ name: "known" }],
      rules: ["RULE-SET,known, "],
    },
    {
      kind: "mihomo" as const,
      groups: [{ name: "Proxy", proxies: ["$nodes"] }],
      ruleSets: [{ name: "known" }],
      rules: ["RULE-SET,known"],
    },
    {
      kind: "sing-box" as const,
      groups: [{ tag: "Proxy", outbounds: ["$nodes"] }],
      ruleSets: [{ tag: "known" }],
      rules: [{ rule_set: ["known"], outbound: " " }],
    },
  ])("reports an empty $kind rule policy as an error", ({ kind, groups, ruleSets, rules }) => {
    const model = buildConfigRelationModel(kind, groups, ruleSets, rules);

    expect(model.issues).toEqual([{
      severity: "error",
      code: "rule_policy_empty",
      section: "rules",
      itemId: "rule-0",
      message: "Rule policy is required.",
    }]);
  });

  it("treats Mihomo GEOIP as a normal rule and accepts target literals", () => {
    const model = buildConfigRelationModel(
      "mihomo",
      [{ name: "Proxy", proxies: ["Named Node", "$nodes", "DIRECT", "REJECT"] }],
      [],
      ["GEOIP,CN,DIRECT", "MATCH,REJECT"],
    );

    expect(model.issues).toEqual([]);
    expect(model.ruleSetInboundReferences).toEqual({});
  });

  it.each(["REJECT-DROP", "PASS", "PASS-RULE", "COMPATIBLE"])(
    "accepts the Mihomo %s built-in policy",
    (policy) => {
      const model = buildConfigRelationModel(
        "mihomo",
        [{ name: "Proxy", proxies: ["$nodes"] }],
        [],
        [`MATCH,${policy}`],
      );

      expect(model.issues).toEqual([]);
    },
  );

  it("accepts sing-box direct/block literals and concrete node names in groups", () => {
    const model = buildConfigRelationModel(
      "sing-box",
      [{ tag: "Proxy", outbounds: ["Named Node", "$nodes", "direct", "block"] }],
      [],
      [{ outbound: "block" }],
    );

    expect(model.issues).toEqual([]);
  });

  it("counts Shadowrocket DOMAIN-SET and RULE-SET references and treats FINAL as final", () => {
    const model = buildConfigRelationModel(
      "shadowrocket",
      [{ name: "Proxy", type: "select", proxies: ["Node 1", "$nodes", "DIRECT", "REJECT"] }],
      [
        { name: "domains", type: "domain-set", url: "https://example.com/domains.list" },
        { name: "mixed", type: "rule-set", url: "https://example.com/mixed.list" },
      ],
      ["DOMAIN-SET,domains,Proxy", "RULE-SET,mixed,DIRECT,no-resolve", "FINAL,Proxy"],
      ["Node 1"],
    );

    expect(model.issues).toEqual([]);
    expect(model.ruleSetInboundReferences).toEqual({ domains: 1, mixed: 1 });
    expect(model.groupInboundReferences.Proxy).toBe(2);
  });

  it.each([
    "PROXY", "DIRECT", "TAILSCALE", "REJECT", "REJECT-DICT", "REJECT-ARRAY",
    "REJECT-200", "REJECT-IMG", "REJECT-TINYGIF", "REJECT-DROP", "REJECT-NO-DROP",
  ])("accepts the Shadowrocket %s built-in rule policy", (policy) => {
    const model = buildConfigRelationModel(
      "shadowrocket",
      [{ name: "Proxy", type: "select", proxies: ["DIRECT"] }],
      [],
      [`AND,((PROTOCOL,UDP),(DST-PORT,443)),${policy}`],
      [],
    );

    expect(model.issues).toEqual([]);
  });

  it("keeps Shadowrocket group policies separate from rule-only actions", () => {
    const model = buildConfigRelationModel(
      "shadowrocket",
      [{ name: "Proxy", type: "select", proxies: ["PROXY", "DIRECT", "REJECT", "TAILSCALE"] }],
      [],
      [],
      [],
    );

    expect(model.issues).toContainEqual(expect.objectContaining({
      severity: "error",
      code: "unknown_group_target",
      reference: "TAILSCALE",
    }));
    expect(model.issues).not.toContainEqual(expect.objectContaining({ reference: "PROXY" }));
  });

  it("makes unresolved Shadowrocket targets blocking only when current preview names are available", () => {
    const groups = [{ name: "Proxy", type: "select", proxies: ["Missing Node", "DIRECT"] }];
    const rules = ["FINAL,Missing Policy"];

    expect(buildConfigRelationModel("shadowrocket", groups, [], rules).issues).toEqual([
      expect.objectContaining({ severity: "warning", code: "unknown_rule_policy" }),
    ]);
    expect(buildConfigRelationModel("shadowrocket", groups, [], rules, ["Known Node"]).issues).toEqual([
      expect.objectContaining({ severity: "error", code: "unknown_group_target", reference: "Missing Node" }),
      expect.objectContaining({ severity: "error", code: "unknown_rule_policy", reference: "Missing Policy" }),
    ]);
  });

  it("rejects Shadowrocket structured values that violate backend field constraints", () => {
    const model = buildConfigRelationModel(
      "shadowrocket",
      [
        { name: "DIRECT", type: "select", proxies: ["$nodes"] },
        { name: "Node 1", type: "select", proxies: ["DIRECT"] },
        {
          name: "Ranges",
          type: "url-test",
          proxies: ["DIRECT", "DIRECT"],
          interval: 86401,
          timeout: 301,
          tolerance: 65536,
        },
      ],
      [{ name: "bad,name", type: "rule-set", url: "https://example.com/rules,tracking.list" }],
      [],
      ["Node 1"],
    );

    expect(model.issues).toEqual(expect.arrayContaining([
      expect.objectContaining({ code: "shadowrocket_group_name_reserved", itemId: "group-0" }),
      expect.objectContaining({ code: "shadowrocket_group_node_collision", itemId: "group-1" }),
      expect.objectContaining({ code: "shadowrocket_group_member_duplicate", itemId: "group-2" }),
      expect.objectContaining({ code: "shadowrocket_group_interval_invalid", itemId: "group-2" }),
      expect.objectContaining({ code: "shadowrocket_group_timeout_invalid", itemId: "group-2" }),
      expect.objectContaining({ code: "shadowrocket_group_tolerance_invalid", itemId: "group-2" }),
      expect.objectContaining({ code: "shadowrocket_rule_set_name_invalid", itemId: "ruleset-0" }),
      expect.objectContaining({ code: "rule_set_url_invalid", itemId: "ruleset-0" }),
    ]));
  });

  it.each(["DIRECT", "direct", "ReJeCt", "PROXY", "TAILSCALE", "REJECT-DROP"])(
    "rejects a Shadowrocket rendered node name that shadows a built-in policy: %s",
    (name) => {
      const model = buildConfigRelationModel(
        "shadowrocket",
        [{ name: "Proxy", type: "select", proxies: ["DIRECT"] }],
        [],
        ["FINAL,Proxy"],
        [name],
      );

      expect(model.issues).toContainEqual(expect.objectContaining({
        severity: "error",
        code: "shadowrocket_node_reserved_collision",
        reference: name,
      }));
    },
  );

  it.each(["PROXY", "TAILSCALE", "REJECT-DROP"])(
    "rejects a Shadowrocket group name that shadows a built-in rule policy: %s",
    (name) => {
      const model = buildConfigRelationModel(
        "shadowrocket",
        [{ name, type: "select", proxies: ["DIRECT"] }],
        [],
        [],
      );

      expect(model.issues).toContainEqual(expect.objectContaining({
        severity: "error",
        code: "shadowrocket_group_name_reserved",
        itemId: "group-0",
      }));
    },
  );

  it.each(["#comment", ";comment", "[section]"])(
    "rejects a Shadowrocket group name that starts with an INI control character: %s",
    (name) => {
      const model = buildConfigRelationModel(
        "shadowrocket",
        [{ name, type: "select", proxies: ["DIRECT"] }],
        [],
        [],
      );

      expect(model.issues).toContainEqual(expect.objectContaining({
        severity: "error",
        code: "shadowrocket_group_name_invalid",
        itemId: "group-0",
      }));
    },
  );

  it("rejects a fixed Shadowrocket group that is empty after expanding subscription nodes", () => {
    const model = buildConfigRelationModel(
      "shadowrocket",
      [{ name: "Empty", type: "select", proxies: ["$nodes"] }],
      [],
      [],
      [],
    );

    expect(model.issues).toContainEqual(expect.objectContaining({
      severity: "error",
      code: "shadowrocket_group_members_empty",
      itemId: "group-0",
    }));
  });

  it("allows Shadowrocket health groups to omit interval and timeout", () => {
    const model = buildConfigRelationModel(
      "shadowrocket",
      [{ name: "Auto", type: "url-test", proxies: ["DIRECT"] }],
      [],
      [],
    );

    expect(model.issues).toEqual([]);
  });

  it("requires Shadowrocket symbolic rules to match the declared rule-set type", () => {
    const model = buildConfigRelationModel(
      "shadowrocket",
      [{ name: "Proxy", type: "select", proxies: ["DIRECT"] }],
      [
        { name: "domains", type: "domain-set", url: "https://example.com/domains.list" },
        { name: "mixed", type: "rule-set", url: "https://example.com/mixed.list" },
      ],
      ["RULE-SET,domains,Proxy", "DOMAIN-SET,mixed,Proxy"],
    );

    expect(model.issues).toEqual([
      expect.objectContaining({ code: "rule_set_type_mismatch", itemId: "rule-0", reference: "domains" }),
      expect.objectContaining({ code: "rule_set_type_mismatch", itemId: "rule-1", reference: "mixed" }),
    ]);
  });

  it("keeps Shadowrocket driver issues in their baseline relation order", () => {
    const model = buildConfigRelationModel(
      "shadowrocket",
      [{ name: "Proxy", type: "select", proxies: ["Missing Node"] }],
      [{ name: "known", type: "domain-set", url: "https://example.com/domains.list" }],
      ["RULE-SET,known,Proxy"],
      ["Known Node"],
    );

    expect(model.issues.map((issue) => issue.code)).toEqual([
      "unknown_group_target",
      "rule_set_type_mismatch",
    ]);
  });

  it("accepts the Chinese sing-box anchor and rejects mixed anchor identities", () => {
    const localized = buildConfigRelationModel(
      "sing-box",
      [{ type: "selector", tag: "🚀 节点选择", outbounds: ["Node 1", "direct"] }],
      [],
      [{ outbound: "🚀 节点选择" }],
      ["Node 1"],
    );
    expect(localized.issues).toEqual([]);

    const mixed = buildConfigRelationModel(
      "sing-box",
      [
        { type: "selector", tag: "Proxy", outbounds: ["Node 1"] },
        { type: "selector", tag: "🚀 节点选择", outbounds: ["direct"] },
      ],
      [],
      [],
      ["Node 1"],
    );
    expect(mixed.issues).toContainEqual(expect.objectContaining({ code: "singbox_proxy_duplicate" }));
  });

  it.each([
    { name: "missing", groups: [{ type: "urltest", tag: "Auto", outbounds: ["Node 1"] }], code: "singbox_proxy_missing" },
    {
      name: "duplicate",
      groups: [
        { type: "selector", tag: "Proxy", outbounds: ["Node 1"] },
        { type: "selector", tag: "Proxy", outbounds: ["direct"] },
      ],
      code: "singbox_proxy_duplicate",
    },
  ])("rejects a $name sing-box Proxy anchor when preview validation is active", ({ groups, code }) => {
    const model = buildConfigRelationModel("sing-box", groups, [], [], ["Node 1"]);

    expect(model.issues).toContainEqual(expect.objectContaining({
      severity: "error",
      code,
      section: "groups",
    }));
  });

  it("rejects sing-box group tags that collide with nodes or reserved outbounds", () => {
    const model = buildConfigRelationModel(
      "sing-box",
      [
        { type: "selector", tag: "Proxy", outbounds: ["Node 1"] },
        { type: "selector", tag: "Node 1", outbounds: ["direct"] },
        { type: "selector", tag: "direct", outbounds: ["Node 1"] },
      ],
      [],
      [],
      ["Node 1"],
    );

    expect(model.issues).toEqual(expect.arrayContaining([
      expect.objectContaining({ severity: "error", code: "singbox_group_node_collision", itemId: "group-1" }),
      expect.objectContaining({ severity: "error", code: "singbox_group_reserved_collision", itemId: "group-2" }),
    ]));
  });

  it("rejects duplicate sing-box preview node tags", () => {
    const model = buildConfigRelationModel(
      "sing-box",
      [{ type: "selector", tag: "Proxy", outbounds: ["Node 1"] }],
      [],
      [],
      ["Node 1", "Node 1"],
    );

    expect(model.issues).toContainEqual(expect.objectContaining({
      severity: "error",
      code: "singbox_node_tag_duplicate",
      reference: "Node 1",
    }));
  });

  it("rejects sing-box Proxy groups with a non-selector type", () => {
    const model = buildConfigRelationModel(
      "sing-box",
      [{ type: "urltest", tag: "Proxy", outbounds: ["Node 1"], url: "https://example.com", interval: "5m" }],
      [],
      [],
      ["Node 1"],
    );

    expect(model.issues).toContainEqual(expect.objectContaining({
      severity: "error",
      code: "singbox_proxy_type_invalid",
      itemId: "group-0",
    }));
  });

  it("rejects sing-box node tags that shadow built-in outbounds", () => {
    const model = buildConfigRelationModel(
      "sing-box",
      [{ type: "selector", tag: "Proxy", outbounds: ["direct"] }],
      [],
      [],
      ["direct"],
    );

    expect(model.issues).toContainEqual(expect.objectContaining({
      severity: "error",
      code: "singbox_node_reserved_collision",
      reference: "direct",
    }));
  });

  it("rejects empty sing-box urltest groups after expanding $nodes", () => {
    const model = buildConfigRelationModel(
      "sing-box",
      [
        { type: "selector", tag: "Proxy", outbounds: ["Auto", "direct"] },
        { type: "urltest", tag: "Auto", outbounds: ["$nodes"], url: "https://example.com", interval: "5m" },
      ],
      [],
      [],
      [],
    );

    expect(model.issues).toContainEqual(expect.objectContaining({
      severity: "error",
      code: "singbox_urltest_empty",
      itemId: "group-1",
    }));
  });

  it("keeps sing-box driver issues in their baseline relation order", () => {
    const model = buildConfigRelationModel(
      "sing-box",
      [
        { type: "selector", tag: "Proxy", outbounds: ["Missing Node"] },
        { type: "urltest", tag: "Auto", outbounds: [], url: "https://example.com", interval: "5m" },
      ],
      [],
      [],
      ["Known Node"],
    );

    expect(model.issues.map((issue) => issue.code)).toEqual([
      "unknown_group_target",
      "singbox_urltest_empty",
    ]);
  });

  it("treats unresolved sing-box group targets as blocking when preview validation is active", () => {
    const model = buildConfigRelationModel(
      "sing-box",
      [{ type: "selector", tag: "Proxy", outbounds: ["Missing Node", "direct"] }],
      [],
      [],
      ["Known Node"],
    );

    expect(model.issues).toContainEqual(expect.objectContaining({
      severity: "error",
      code: "unknown_group_target",
      reference: "Missing Node",
    }));
  });

  it("treats unresolved sing-box rule outbounds as blocking when preview validation is active", () => {
    const model = buildConfigRelationModel(
      "sing-box",
      [{ type: "selector", tag: "Proxy", outbounds: ["Node 1"] }],
      [],
      [{ outbound: "Missing Node" }],
      ["Node 1"],
    );

    expect(model.issues).toContainEqual(expect.objectContaining({
      severity: "error",
      code: "unknown_rule_policy",
      reference: "Missing Node",
    }));
  });

  it.each([
    {
      kind: "mihomo" as const,
      groups: [{ name: "Proxy", proxies: ["Known Node", "Missing Node", "$nodes", "DIRECT"] }],
      rules: ["MATCH,Known Node"],
    },
    {
      kind: "sing-box" as const,
      groups: [{ type: "selector", tag: "Proxy", outbounds: ["Known Node", "Missing Node", "$nodes", "direct"] }],
      rules: [{ outbound: "Known Node" }],
    },
  ])("uses preview node names when validating $kind references", ({ kind, groups, rules }) => {
    const model = buildConfigRelationModel(kind, groups, [], rules, ["Known Node"]);

    expect(model.issues).toEqual([expect.objectContaining({
      severity: kind === "sing-box" ? "error" : "warning",
      code: "unknown_group_target",
      section: "groups",
      itemId: "group-0",
      reference: "Missing Node",
    })]);
  });

  it("accepts every Mihomo built-in target exposed by the reference picker", () => {
    const model = buildConfigRelationModel(
      "mihomo",
      [{ name: "Proxy", proxies: ["$nodes"] }],
      [],
      ["MATCH,GLOBAL"],
    );

    expect(model.issues).toEqual([]);
  });

  it("counts names that collide with object prototype keys", () => {
    const model = buildConfigRelationModel(
      "mihomo",
      [{ name: "toString", proxies: ["$nodes"] }, { name: "__proto__", proxies: ["toString"] }],
      [{ name: "constructor" }],
      ["RULE-SET,constructor,__proto__", "MATCH,toString"],
    );

    expect(Object.entries(model.groupInboundReferences)).toEqual([["toString", 2], ["__proto__", 1]]);
    expect(Object.entries(model.ruleSetInboundReferences)).toEqual([["constructor", 1]]);
    expect(model.issues).toEqual([]);
  });

  it.each([
    {
      kind: "mihomo" as const,
      groups: [
        { name: "A", proxies: ["B"] },
        { name: "B", proxies: ["C"] },
        { name: "C", proxies: ["A"] },
      ],
      rules: ["MATCH,A"],
    },
    {
      kind: "sing-box" as const,
      groups: [
        { tag: "A", outbounds: ["B"] },
        { tag: "B", outbounds: ["C"] },
        { tag: "C", outbounds: ["A"] },
      ],
      rules: [{ outbound: "A" }],
    },
  ])("reports every group in a definite $kind reference cycle", ({ kind, groups, rules }) => {
    const model = buildConfigRelationModel(kind, groups, [], rules);

    expect(model.issues.filter((issue) => issue.code === "group_reference_cycle")).toEqual([
      expect.objectContaining({ severity: "error", section: "groups", itemId: "group-0" }),
      expect.objectContaining({ severity: "error", section: "groups", itemId: "group-1" }),
      expect.objectContaining({ severity: "error", section: "groups", itemId: "group-2" }),
    ]);
  });

  it.each([
    {
      kind: "mihomo" as const,
      groups: [{ name: "Proxy", proxies: ["$nodes"] }],
      ruleSets: [{ name: "known" }],
      rules: ["MATCH,Proxy", "RULE-SET,known,Proxy"],
    },
    {
      kind: "sing-box" as const,
      groups: [{ tag: "Proxy", outbounds: ["$nodes"] }],
      ruleSets: [{ tag: "known" }],
      rules: [{ outbound: "Proxy" }, { rule_set: ["known"], outbound: "Proxy" }],
    },
  ])("warns only when a $kind final route is not last", ({ kind, groups, ruleSets, rules }) => {
    const misplaced = buildConfigRelationModel(kind, groups, ruleSets, rules);
    const corrected = buildConfigRelationModel(kind, groups, ruleSets, [...rules].reverse());

    expect(misplaced.issues).toContainEqual({
      severity: "warning",
      code: "final_rule_not_last",
      section: "rules",
      itemId: "rule-0",
      message: "Final routing rule must be last.",
    });
    expect(corrected.issues.some((issue) => issue.code === "final_rule_not_last")).toBe(false);
  });

  it("returns an empty model for static files", () => {
    expect(buildProjectedConfigRelationModel({ events: [], groups: [], ruleSets: [], issues: [] })).toEqual({
      groupInboundReferences: {},
      ruleSetInboundReferences: {},
      issues: [],
    });
  });
});

function issueKeys(model: ReturnType<typeof buildConfigRelationModel>): string[] {
  return model.issues.map((issue) => `${issue.severity}:${issue.code}:${issue.section}:${issue.itemId}`);
}

function buildConfigRelationModel(
  kind: ConfigKind,
  groups: Record<string, unknown>[],
  ruleSets: Record<string, unknown>[],
  rules: unknown[],
  nodeNames?: string[],
) {
  const adapter = structuredAdapter(kind);
  return buildProjectedConfigRelationModel(adapter.relations.project(groups, ruleSets, rules, nodeNames));
}

function createConfigFromTemplate(
  kind: ConfigKind,
  templateID: ConfigTemplateID,
): FileConfigDraft {
  return structuredAdapter(kind).templates.create(templateID);
}

function structuredAdapter(kind: ConfigKind) {
  const driver = requireFileDriver(kind);
  if (driver.configuration.mode !== "structured") throw new Error(`${kind} is not structured`);
  return driver.configuration.adapter;
}
