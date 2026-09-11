import { describe, expect, it } from "vitest";

import { singBoxConfigurationAdapter } from "./configuration";
import { singBoxGroupMembers, validSingBoxGroupFilter } from "./group-filter";

describe("sing-box group regex", () => {
  it("matches case sensitively unless the leading flag enables case folding", () => {
    const group = { outbounds: ["$nodes"], filter: "^HK", "exclude-filter": "home" };
    const nodes = ["HK-01", "hk-02", "HK-home", "JP-01", "HK-01"];
    expect(singBoxGroupMembers(group, nodes)).toEqual(["HK-01"]);
    expect(singBoxGroupMembers({ ...group, filter: "(?i)^HK" }, nodes)).toEqual(["HK-01", "hk-02"]);
    expect(singBoxGroupMembers({ outbounds: ["$nodes"], filter: " HK " }, ["HK", " HK "])).toEqual([" HK "]);
  });

  it.each(["", " ", "(?i)", "(?i) ", "[", "(?i)HK(?i)", undefined, 42])("rejects invalid pattern %j", (pattern) => {
    expect(validSingBoxGroupFilter(pattern)).toBe(false);
  });

  it("round-trips persistent regex fields and restores ordered fixed members on a mode switch", () => {
    const groups = singBoxConfigurationAdapter.groups;
    const native = { type: "selector", tag: "HK", outbounds: ["$nodes"], filter: "(?i)^HK", "exclude-filter": "home", default: "HK-01" };
    const [projected] = groups.project([native])!;
    expect(projected).toMatchObject({ memberMode: "regex-filter", filter: "(?i)^HK", excludeFilter: "home" });
    expect(groups.serialize([projected])).toEqual([native]);
    const fixed = groups.transitionMemberMode(projected, "fixed", ["HK-02", "direct"]);
    expect(groups.serialize([fixed])).toEqual([{ type: "selector", tag: "HK", outbounds: ["HK-02", "direct"], default: "HK-01" }]);
    const filtered = groups.transitionMemberMode(fixed, "regex-filter");
    expect(filtered.filter).toBe(".*");
    expect(groups.serialize([filtered])[0]).toMatchObject({ filter: ".*", outbounds: ["$nodes"] });
  });

  it("preserves unsupported filter shapes in advanced mode", () => {
    const groups = singBoxConfigurationAdapter.groups;
    const native = { type: "selector", tag: "HK", outbounds: ["$nodes"], filter: "HK" };
    expect(groups.project([{ ...native, filter: 42 }])).toBeNull();
    expect(groups.project([{ ...native, "exclude-filter": 42 }])).toBeNull();
    expect(groups.project([{ ...native, outbounds: ["HK-01"] }])).toBeNull();
  });

  it("validates regexes when projecting the selected subscriptions into urltest members", () => {
    const adapter = singBoxConfigurationAdapter;
    const groups = [
      { type: "selector", tag: "Proxy", outbounds: ["HK"] },
      { type: "urltest", tag: "HK", outbounds: ["$nodes"], filter: "^HK", "exclude-filter": "home", url: "https://example.com", interval: "5m" },
    ];
    const populated = adapter.relations.project(groups, [], [], ["HK-01", "JP-01"]);
    expect(populated.events).toContainEqual({ type: "reference", reference: expect.objectContaining({ target: "HK-01" }) });
    expect(populated.events).not.toContainEqual({ type: "reference", reference: expect.objectContaining({ target: "JP-01" }) });
    expect(populated.events).not.toContainEqual({ type: "reference", reference: expect.objectContaining({ target: "$nodes" }) });
    const empty = adapter.relations.project(groups, [], [], ["HK-home", "hk-02", "JP-01"]);
    expect(empty.events).toContainEqual({ type: "issue", issue: expect.objectContaining({ code: "singbox_urltest_empty" }) });
    const draft = adapter.initialize({ groups: [{ ...groups[1], filter: "[" }], rule_sets: [], rules: [] });
    expect(adapter.validate(draft)).toContainEqual(expect.objectContaining({ code: "group_filter_invalid" }));
  });

  it("blocks an empty filtered selector only after a subscription preview is available", () => {
    const groups = [{ type: "selector", tag: "Proxy", outbounds: ["$nodes"], filter: "^HK" }];
    const project = singBoxConfigurationAdapter.relations.project;
    const noPreview = project(groups, [], []);
    expect(noPreview.events).toEqual([]);
    const empty = project(groups, [], [], ["JP-01"]);
    expect(empty.events).toContainEqual({ type: "issue", issue: expect.objectContaining({ code: "group_members_empty" }) });
    const fixed = project([{ type: "selector", tag: "Proxy", outbounds: [] }], [], [], []);
    expect(fixed.events).toEqual([]);
  });
});
