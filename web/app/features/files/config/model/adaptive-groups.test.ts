import { describe, expect, it } from "vitest";

import type { ConfigAdaptiveStrategy } from "~/features/files/drivers/core/file-driver";
import { requireFileDriver } from "~/features/files/drivers/registry";
import type { FileAdaptiveGroupConfigDetail, FileConfigDraft } from "~/features/files/model/types";

import { adaptiveGenerationDisabledReasonKey } from "./adaptive-availability";
import type {
  AdaptiveGroupGeneration,
  AdaptiveGroupOptions,
  AdaptiveGroupType,
} from "./adaptive-groups";
import type { ConfigMap } from "./editor-model";
import type { ConfigNamingLocale } from "./naming";
import type { ConfigTemplateID } from "./templates";

type StructuredTarget = "mihomo" | "sing-box" | "shadowrocket";

function structuredAdapter(target: StructuredTarget) {
  const driver = requireFileDriver(target);
  if (driver.configuration.mode !== "structured") {
    throw new Error(`expected ${target} to use structured configuration`);
 }
  return driver.configuration.adapter;
}

function adaptiveStrategy(target: StructuredTarget = "mihomo"): ConfigAdaptiveStrategy {
  return structuredAdapter(target).adaptive;
}

function createConfigFromTemplate(
  target: StructuredTarget,
  templateID: ConfigTemplateID,
  namingLocale?: ConfigNamingLocale,
): FileConfigDraft {
  return structuredAdapter(target).templates.create(templateID, namingLocale);
}

function adaptiveGroupAnchorProblem(
  config: Readonly<FileConfigDraft>,
  target: StructuredTarget = "mihomo",
) {
  return adaptiveStrategy(target).anchorProblem(config);
}

function adaptiveGroupConfigFromOptions(
  options: Readonly<AdaptiveGroupOptions>,
  target: StructuredTarget = "mihomo",
): FileAdaptiveGroupConfigDetail | undefined {
  return adaptiveStrategy(target).configFromOptions(options);
}

function adaptiveGroupOptionsFromConfig(
  config: FileAdaptiveGroupConfigDetail | undefined,
  target: StructuredTarget = "mihomo",
): AdaptiveGroupOptions {
  return adaptiveStrategy(target).optionsFromConfig(config);
}

function canonicalAdaptiveGroupNames(
  groups: readonly ConfigMap[],
  target: StructuredTarget = "mihomo",
): string[] {
  return adaptiveStrategy(target).canonicalNames(groups);
}

function defaultAdaptiveGroupOptions(target: StructuredTarget = "mihomo"): AdaptiveGroupOptions {
  return adaptiveStrategy(target).defaultOptions();
}

function generateAdaptiveGroups(
  nodeNames: readonly string[],
  options: Readonly<AdaptiveGroupOptions>,
  target: StructuredTarget = "mihomo",
  namingLocale: ConfigNamingLocale = "en-US",
): AdaptiveGroupGeneration {
  return adaptiveStrategy(target).generate(nodeNames, options, namingLocale);
}

function mergeAdaptiveGroups(
  config: Readonly<FileConfigDraft>,
  generation: Readonly<AdaptiveGroupGeneration>,
  target: StructuredTarget = "mihomo",
) {
  return adaptiveStrategy(target).merge(config, generation);
}

function stripCanonicalAdaptiveGroups(
  config: Readonly<FileConfigDraft>,
  target: StructuredTarget = "mihomo",
) {
  return adaptiveStrategy(target).strip(config);
}

describe("localized adaptive group naming", () => {
  it.each(["mihomo", "sing-box", "shadowrocket"] as const)("uses localized region names and anchors for %s", (target) => {
    const base = createConfigFromTemplate(target, "minimal", "zh-CN");
    const generation = generateAdaptiveGroups(
      ["HK-01"],
      { type: target === "sing-box" ? "selector" : "select" },
      target,
      "zh-CN",
    );
    const result = mergeAdaptiveGroups(base, generation, target);
    const nameKey = target === "sing-box" ? "tag" : "name";
    const memberKey = target === "sing-box" ? "outbounds" : "proxies";
    const anchor = result.config.groups?.find((group) => group[nameKey] === "🚀 节点选择");

    expect(generation.candidates.find((candidate) => candidate.id === "hk")?.name).toBe("🇭🇰 香港");
    expect(result.generatedGroupNames).toEqual(["🇭🇰 香港"]);
    expect(anchor?.[memberKey]).toEqual(expect.arrayContaining(["🇭🇰 香港"]));
    expect(adaptiveGroupAnchorProblem(base, target)).toBeNull();
 });

  it("treats English and Chinese anchors together as an ambiguous duplicate", () => {
    const base = createConfigFromTemplate("sing-box", "minimal", "zh-CN");
    const config = {
      ...base,
      groups: [...(base.groups ?? []), { type: "selector", tag: "Proxy", outbounds: ["$nodes"] }],
   };

    expect(adaptiveGroupAnchorProblem(config, "sing-box")).toEqual({ code: "anchor_duplicate", count: 2 });
 });
});

describe("adaptive generation availability", () => {
  const config = createConfigFromTemplate("mihomo", "minimal");
  const ready = {
    anchorProblem: null,
    editorMode: "wizard" as const,
    hasCurrentPreview: true,
    nodeCount: 2,
    previewStatus: "ready" as const,
    requiresNodePreview: true,
    selected: true,
 };

  it.each([
    ["advanced editor", { editorMode: "advanced" }, "files.config.adaptiveAdvancedUnsupported"],
    ["missing subscription", { selected: false }, "files.config.adaptiveSelectSubscription"],
    ["loading preview", { previewStatus: "loading" }, "files.config.adaptivePreviewLoading"],
    ["failed preview", { previewStatus: "error" }, "files.config.adaptivePreviewUnavailable"],
    ["missing current preview", { hasCurrentPreview: false }, "files.config.adaptivePreviewUnavailable"],
    ["empty preview", { nodeCount: 0 }, "files.config.adaptiveNoNodes"],
  ] as const)("returns the translation key for %s", (_name, overrides, expected) => {
    expect(adaptiveGenerationDisabledReasonKey({ ...ready, ...overrides })).toBe(expected);
 });

  it.each([
    ["missing", [], "files.config.adaptiveProxyMissing"],
    [
      "duplicate",
      [
        { name: "Proxy", type: "select", proxies: ["$nodes"] },
        { name: "Proxy", type: "select", proxies: ["$nodes"] },
      ],
      "files.config.adaptiveProxyDuplicate",
    ],
    ["invalid members", [{ name: "Proxy", type: "select", proxies: [1] }], "files.config.adaptiveProxyMembersInvalid"],
  ] as const)("returns the translation key for a %s anchor", (_name, groups, expected) => {
    const current = { ...config, groups: [...groups] };
    expect(adaptiveGenerationDisabledReasonKey({
      ...ready,
      anchorProblem: adaptiveGroupAnchorProblem(current),
   })).toBe(expected);
 });

  it("returns the sing-box anchor type key and allows a valid config", () => {
    const singBox = createConfigFromTemplate("sing-box", "minimal");

    expect(adaptiveGenerationDisabledReasonKey({
      ...ready,
      anchorProblem: adaptiveGroupAnchorProblem({
        ...singBox,
        groups: singBox.groups?.map((group) => group.tag === "Proxy"
          ? { ...group, type: "urltest" }
          : group),
     }, "sing-box"),
   })).toBe("files.config.adaptiveProxyTypeInvalid");
    expect(adaptiveGenerationDisabledReasonKey({
      ...ready,
      anchorProblem: adaptiveGroupAnchorProblem(singBox, "sing-box"),
   })).toBeUndefined();
 });
});

describe("adaptive Mihomo group generation", () => {
  it("generates the five selected runtime-filter regions without preview nodes", () => {
    const result = generateAdaptiveGroups(
      [],
      defaultAdaptiveGroupOptions(),
    );

    expect(result.groups.map((group) => group.name)).toEqual([
      "Hong Kong",
      "Taiwan",
      "Singapore",
      "Japan",
      "United States",
    ]);
 });

  it("classifies unique names with exclusions, overlaps, and registry order without low-cost groups", () => {
    const result = generateAdaptiveGroups(
      [
        "HK-01 0.5",
        "香港-02",
        "HK-01 0.5",
        "US-日本-01",
        "US-日本-02",
        "亚美尼亚 US-03",
        "Unnamed Elsewhere",
        " ",
      ],
      { type: "url-test" },
    );

    expect(result.uniqueNodeCount).toBe(6);
    expect(result.groups.map((group) => group.name)).toEqual([
      "Hong Kong",
      "Japan",
      "United States",
    ]);
    expect(result.candidates.find((item) => item.name === "United States")).toMatchObject({
      matchedNodeCount: 2,
      active: true,
   });
 });

  it("skips a generated group whose name collides with a concrete proxy", () => {
    const result = generateAdaptiveGroups(
      ["Hong Kong", "HK-02"],
      defaultAdaptiveGroupOptions(),
    );

    expect(result.groups.some((group) => group.name === "Hong Kong")).toBe(false);
    expect(result.warnings).toContainEqual({ code: "node_name_conflict", groupName: "Hong Kong" });
 });

  it("exposes all 22 regions in stable order", () => {
    expect(generateAdaptiveGroups([], { type: "select" })
      .candidates.map((item) => item.name)).toEqual([
      "Hong Kong", "Taiwan", "Singapore", "Japan", "South Korea",
      "United States", "Canada", "United Kingdom", "Germany", "France", "Macau",
      "Australia", "Russia", "Thailand", "India", "Malaysia", "Philippines",
      "Turkey", "Ukraine", "Finland", "Argentina", "Egypt",
    ]);
 });

  it.each([
    ["🇭🇰 edge", "Hong Kong"],
    ["tw-01", "Taiwan"],
    ["Singapore SIN", "Singapore"],
    ["東京 HND", "Japan"],
    ["Seoul ICN", "South Korea"],
    ["lax usa", "United States"],
    ["YYZ-01", "Canada"],
    ["United Kingdom LHR", "United Kingdom"],
    ["Germany BER", "Germany"],
    ["France CDG", "France"],
    ["Macao MAC", "Macau"],
    ["Sydney AUS", "Australia"],
    ["Moscow SVO", "Russia"],
    ["Bangkok BKK", "Thailand"],
    ["Mumbai BOM", "India"],
    ["Kuala Lumpur KUL", "Malaysia"],
    ["Manila MNL", "Philippines"],
    ["Türkiye IST", "Turkey"],
    ["Kyiv UKR", "Ukraine"],
    ["Helsinki HEL", "Finland"],
    ["Buenos Aires EZE", "Argentina"],
    ["Cairo EGY", "Egypt"],
  ])("matches representative node %s as %s", (nodeName, groupName) => {
    const result = generateAdaptiveGroups([nodeName], { type: "select" });

    expect(result.candidates.find((item) => item.name === groupName)).toMatchObject({
      active: true,
      matchedNodeCount: 1,
   });
 });

  const groupShapes: Array<[AdaptiveGroupType, Record<string, unknown>]> = [
    ["select", { type: "select" }],
    ["url-test", {
      type: "url-test",
      url: "https://cp.cloudflare.com",
      interval: 300,
      lazy: true,
   }],
    ["load-balance", {
      type: "load-balance",
      url: "https://cp.cloudflare.com",
      interval: 300,
      lazy: true,
      strategy: "sticky-sessions",
   }],
  ];

  it.each(groupShapes)("emits the complete %s canonical shape", (type, expected) => {
    const result = generateAdaptiveGroups(["US-01"], { type });
    const group = result.groups.find((item) => item.name === "United States");
    const candidate = result.candidates.find((item) => item.name === "United States");

    expect(candidate).toBeDefined();
    expect(candidate?.filter).toMatch(/^\(\?i\)/);
    expect(candidate?.excludeFilter).toMatch(/^\(\?i\)/);
    expect(group).toEqual({
      name: "United States",
      "include-all-proxies": true,
      filter: candidate?.filter,
      "exclude-filter": candidate?.excludeFilter,
      ...expected,
   });
 });

  it("omits exclude-filter completely when the region has no exclusion terms", () => {
    const result = generateAdaptiveGroups(["HK-01"], { type: "select" });
    const candidate = result.candidates.find((item) => item.name === "Hong Kong");

    expect(candidate?.filter).toMatch(/^\(\?i\)/);
    expect(candidate?.excludeFilter).toBeUndefined();
    expect(result.groups.find((item) => item.name === "Hong Kong")).toEqual({
      name: "Hong Kong",
      type: "select",
      "include-all-proxies": true,
      filter: candidate?.filter,
   });
 });

  it("uses browser-compatible filters without unsupported regex extensions", () => {
    const candidates = generateAdaptiveGroups([], { type: "select" }).candidates;

    for (const candidate of candidates) {
      for (const pattern of [candidate.filter, candidate.excludeFilter]) {
        if (!pattern) continue;
        const source = pattern.replace(/^\(\?i\)/, "");
        expect(() => new RegExp(source, "i"), candidate.name).not.toThrow();
        expect(source, candidate.name).not.toMatch(/\(\?(?!:)/);
        expect(source, candidate.name).not.toMatch(/\\[1-9]/);
        expect(source, candidate.name).not.toContain("`");
     }
   }
 });

  it("generates only selected regions", () => {
    const result = generateAdaptiveGroups(
      ["HK-01 0.5", "香港-02", "JP-01", "東京-02"],
      {
        type: "select",
        enabledRegionIds: ["jp"],
     },
    );

    expect(result.groups.map((group) => group.name)).toEqual(["Japan"]);
    expect(result.candidates.find((candidate) => candidate.id === "hk")).toMatchObject({
      active: true,
      matchedNodeCount: 2,
   });
 });

  it("normalizes persisted options with stable region ids and round-trips an empty selection", () => {
    const options = adaptiveGroupOptionsFromConfig({
      type: "load-balance",
      regions: ["us", "unknown", "hk", "us"],
   });

    expect(options).toEqual({
      type: "load-balance",
      enabledRegionIds: ["hk", "us"],
   });
    expect(adaptiveGroupConfigFromOptions({ ...options, enabledRegionIds: [] })).toEqual({
      type: "load-balance",
      regions: [],
   });
 });

  it("defaults to Hong Kong, Taiwan, Japan, United States, and Singapore", () => {
    expect(defaultAdaptiveGroupOptions().enabledRegionIds).toEqual(["hk", "tw", "jp", "us", "sg"]);
 });

  it("uses only the five default regions from the default generation options", () => {
    const result = generateAdaptiveGroups(
      ["HK-01", "香港-02", "CA-01", "Canada-02"],
      defaultAdaptiveGroupOptions(),
    );

    expect(result.groups.map((group) => group.name)).toEqual([
      "Hong Kong",
      "Taiwan",
      "Singapore",
      "Japan",
      "United States",
    ]);
    expect(result.candidates.find((candidate) => candidate.id === "ca")).toMatchObject({
      active: true,
      matchedNodeCount: 2,
   });
 });
});

describe("adaptive sing-box group generation", () => {
  it("skips selected regions without matching nodes and reports them", () => {
    const result = generateAdaptiveGroups(
      ["HK-01"],
      defaultAdaptiveGroupOptions("sing-box"),
      "sing-box",
    );

    expect(result.groups.map((group) => group.tag)).toEqual(["Hong Kong"]);
    expect(result.warnings).toContainEqual({
      code: "empty_regions_skipped",
      groupNames: ["Taiwan", "Singapore", "Japan", "United States"],
   });
 });

  it("uses urltest defaults and rejects persisted Mihomo-only types", () => {
    expect(defaultAdaptiveGroupOptions("sing-box")).toMatchObject({ type: "urltest" });
    expect(adaptiveGroupOptionsFromConfig({
      type: "load-balance",
   }, "sing-box")).toMatchObject({
      type: "urltest",
   });
 });

  it("emits selector groups with only unique matching preview node tags", () => {
    const result = generateAdaptiveGroups(
      ["HK-02", "HK-01", "HK-02", "US-日本-01", "亚美尼亚 US-03", "", " "],
      { type: "selector" },
      "sing-box",
    );

    expect(result.uniqueNodeCount).toBe(4);
    expect(result.groups).toEqual([
      { tag: "Hong Kong", type: "selector", outbounds: ["HK-02", "HK-01"] },
      { tag: "Japan", type: "selector", outbounds: ["US-日本-01"] },
      { tag: "United States", type: "selector", outbounds: ["US-日本-01"] },
    ]);
 });

  it("emits the complete urltest shape with HTTPS health checks", () => {
    const result = generateAdaptiveGroups(
      ["JP-01", "東京-02"],
      { type: "urltest" },
      "sing-box",
    );

    expect(result.groups).toEqual([{
      tag: "Japan",
      type: "urltest",
      outbounds: ["JP-01", "東京-02"],
      url: "https://cp.cloudflare.com",
      interval: "5m",
      tolerance: 50,
   }]);
 });

  it.each(["select", "url-test", "load-balance"] as const)(
    "rejects unsupported sing-box group type %s",
    (type) => {
      expect(() => generateAdaptiveGroups(
        ["HK-01"],
        { type },
        "sing-box",
      )).toThrow(RangeError);
   },
  );

  it("skips a generated tag that collides with a concrete node", () => {
    const result = generateAdaptiveGroups(
      ["Hong Kong", "HK-02"],
      { type: "selector" },
      "sing-box",
    );

    expect(result.groups.some((group) => group.tag === "Hong Kong")).toBe(false);
    expect(result.warnings).toContainEqual({ code: "node_name_conflict", groupName: "Hong Kong" });
 });

  it("recognizes only strict canonical shapes with unique region-matching members", () => {
    const canonical = generateAdaptiveGroups(
      ["HK-02", "HK-01"],
      { type: "urltest" },
      "sing-box",
    ).groups[0];

    expect(canonicalAdaptiveGroupNames([canonical], "sing-box")).toEqual(["Hong Kong"]);
    expect(canonicalAdaptiveGroupNames([{ ...canonical, icon: "custom" }], "sing-box")).toEqual([]);
    expect(canonicalAdaptiveGroupNames([{ ...canonical, outbounds: ["HK-02", "direct"] }], "sing-box"))
      .toEqual([]);
    expect(canonicalAdaptiveGroupNames([{ ...canonical, outbounds: ["HK-02", "HK-02"] }], "sing-box"))
      .toEqual([]);
    expect(canonicalAdaptiveGroupNames([{ ...canonical, url: "http://www.gstatic.com/generate_204" }], "sing-box"))
      .toEqual([]);
 });
});

describe("adaptive Shadowrocket group generation", () => {
  it("uses only Shadowrocket adaptive types", () => {
    expect(defaultAdaptiveGroupOptions("shadowrocket")).toMatchObject({ type: "url-test" });
    expect(adaptiveGroupOptionsFromConfig({ type: "urltest" }, "shadowrocket")).toMatchObject({ type: "url-test" });
    expect(() => generateAdaptiveGroups(
      ["HK-01"],
      { type: "urltest" },
      "shadowrocket",
    )).toThrow(RangeError);
 });

  it.each(["select", "url-test", "load-balance"] as const)(
    "emits a strict %s runtime-filter group",
    (type) => {
      const result = generateAdaptiveGroups(
        ["HK-01"],
        { type },
        "shadowrocket",
      );
      const candidate = result.candidates.find((item) => item.name === "Hong Kong");
      const group = result.groups.find((item) => item.name === "Hong Kong");

      expect(group).toEqual({
        name: "Hong Kong",
        type,
        "policy-regex-filter": candidate?.filter,
        ...(type === "url-test" || type === "load-balance"
          ? { interval: 300, timeout: 5, tolerance: 50 }
          : {}),
     });
      expect(group).not.toHaveProperty("proxies");
      expect(group).not.toHaveProperty("url");
      expect(group).not.toHaveProperty("include-all-proxies");
   },
  );

  it("recognizes and reconciles strict canonical runtime-filter groups", () => {
    const base = createConfigFromTemplate("shadowrocket", "minimal");
    const generation = generateAdaptiveGroups(
      ["HK-01", "HK-02"],
      { type: "url-test" },
      "shadowrocket",
    );
    const first = mergeAdaptiveGroups(base, generation, "shadowrocket");
    const second = mergeAdaptiveGroups(first.config, generation, "shadowrocket");

    expect(canonicalAdaptiveGroupNames(first.config.groups ?? [], "shadowrocket")).toEqual(["Hong Kong"]);
    expect(first.generatedGroupNames).toEqual(["Hong Kong"]);
    expect(first.config.groups?.find((group) => group.name === "Proxy")?.proxies)
      .toEqual(["PROXY", "Hong Kong", "$nodes", "DIRECT", "REJECT"]);
    expect(second.changed).toBe(false);
 });
});

it.each(["selector", "urltest"] as const)(
  "rejects sing-box-only adaptive type %s for Mihomo",
  (type) => {
    expect(() => generateAdaptiveGroups(
      ["HK-01"],
      { type },
    )).toThrow(RangeError);
 },
);

it("rejects duplicate sing-box Proxy tags even when only one is a selector", () => {
  const config = createConfigFromTemplate("sing-box", "minimal");
  config.groups = [
    ...(config.groups ?? []),
    { type: "urltest", tag: "Proxy", outbounds: ["direct"], url: "https://example.com", interval: "5m" },
  ];

  expect(adaptiveGroupAnchorProblem(config, "sing-box")).toEqual({ code: "anchor_duplicate", count: 2 });
});

it("rejects a sing-box Proxy anchor that is not a selector", () => {
  const config = createConfigFromTemplate("sing-box", "minimal");
  config.groups = (config.groups ?? []).map((group) => group.tag === "Proxy"
    ? { ...group, type: "urltest" }
    : group);

  expect(adaptiveGroupAnchorProblem(config, "sing-box")).toEqual({ code: "anchor_type_invalid" });
});

describe("adaptive sing-box group reconciliation", () => {
  it("appends static groups and inserts tags before $nodes without mutating the template", () => {
    const base = createConfigFromTemplate("sing-box", "minimal");
    const snapshot = structuredClone(base);
    deepFreeze(base);
    const generation = generateAdaptiveGroups(
      ["HK-02", "HK-01"],
      { type: "urltest" },
      "sing-box",
    );

    const first = mergeAdaptiveGroups(base, generation, "sing-box");
    const second = mergeAdaptiveGroups(first.config, generation, "sing-box");

    expect(base).toEqual(snapshot);
    expect(first.changed).toBe(true);
    expect(first.generatedGroupNames).toEqual(["Hong Kong"]);
    expect(first.config.groups?.map((group) => group.tag)).toEqual([
      ...(snapshot.groups ?? []).map((group) => group.tag),
      "Hong Kong",
    ]);
    expect(namedSingBoxGroup(first.config, "Hong Kong")).toEqual({
      tag: "Hong Kong",
      type: "urltest",
      outbounds: ["HK-02", "HK-01"],
      url: "https://cp.cloudflare.com",
      interval: "5m",
      tolerance: 50,
   });
    expect(singBoxProxyMembers(first.config)).toEqual([
      "Auto", "Hong Kong", "$nodes", "direct", "block",
    ]);
    expect(second.changed).toBe(false);
    expect(second.config).toEqual(first.config);
 });

  it("updates canonical static members to the current preview order and values", () => {
    const base = createConfigFromTemplate("sing-box", "minimal");
    const first = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(
        ["HK-01", "HK-02"],
        { type: "selector" },
        "sing-box",
      ),
      "sing-box",
    );

    const result = mergeAdaptiveGroups(
      first.config,
      generateAdaptiveGroups(
        ["HK-02", "HK-03"],
        { type: "selector" },
        "sing-box",
      ),
      "sing-box",
    );

    expect(result.warnings).not.toContainEqual({ code: "group_name_conflict", groupName: "Hong Kong" });
    expect(namedSingBoxGroup(result.config, "Hong Kong")?.outbounds).toEqual(["HK-02", "HK-03"]);
 });

  it.each([
    { type: "selector", tag: "Hong Kong", outbounds: ["direct"] },
    { type: "selector", tag: "Hong Kong", outbounds: ["HK-01"], icon: "custom" },
    { type: "selector", tag: "Hong Kong" },
  ])("preserves a same-tag custom group %#", (custom) => {
    const base = createConfigFromTemplate("sing-box", "minimal");
    const groups = structuredClone(base.groups ?? []);
    const proxy = groups.find((group) => group.tag === "Proxy");
    if (proxy) proxy.outbounds = ["Auto", "Hong Kong", "$nodes", "direct", "block"];
    groups.splice(2, 0, custom);
    const config = { ...base, groups };

    const result = mergeAdaptiveGroups(
      config,
      generateAdaptiveGroups(
        ["HK-01", "HK-02"],
        { type: "selector" },
        "sing-box",
      ),
      "sing-box",
    );

    expect(result.warnings).toContainEqual({ code: "group_name_conflict", groupName: "Hong Kong" });
    expect(result.config.groups?.filter((group) => group.tag === "Hong Kong")).toEqual([custom]);
    expect(singBoxProxyMembers(result.config)).toContain("Hong Kong");
 });

  it("removes an unreferenced stale canonical group and its Proxy tag", () => {
    const base = createConfigFromTemplate("sing-box", "minimal");
    const first = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(
        ["HK-01", "HK-02"],
        { type: "selector" },
        "sing-box",
      ),
      "sing-box",
    );

    const result = mergeAdaptiveGroups(
      first.config,
      generateAdaptiveGroups(["HK-01"], { type: "selector", enabledRegionIds: [] }, "sing-box"),
      "sing-box",
    );

    expect(result.removedGroupNames).toContain("Hong Kong");
    expect(namedSingBoxGroup(result.config, "Hong Kong")).toBeUndefined();
    expect(singBoxProxyMembers(result.config)).not.toContain("Hong Kong");
 });

  it.each(["group", "rule"] as const)(
    "preserves a stale canonical referenced by another %s but removes its Proxy tag",
    (referenceKind) => {
      const base = createConfigFromTemplate("sing-box", "minimal");
      const first = mergeAdaptiveGroups(
        base,
        generateAdaptiveGroups(["HK-01"], { type: "selector" }, "sing-box"),
        "sing-box",
      );
      const config = referenceKind === "group"
        ? {
            ...first.config,
            groups: [...(first.config.groups ?? []), {
              type: "selector",
              tag: "Dependent",
              outbounds: ["Hong Kong"],
           }],
         }
        : {
            ...first.config,
            rules: [...(first.config.rules ?? []).slice(0, -1), { outbound: "Hong Kong" }],
         };

      const result = mergeAdaptiveGroups(
        config,
        generateAdaptiveGroups([], { type: "selector" }, "sing-box"),
        "sing-box",
      );

      expect(result.warnings).toContainEqual({ code: "referenced_stale_group", groupName: "Hong Kong" });
      expect(namedSingBoxGroup(result.config, "Hong Kong")).toBeDefined();
      expect(singBoxProxyMembers(result.config)).not.toContain("Hong Kong");
      expect(result.preservedGroupNames).toContain("Hong Kong");
   },
  );

  it("removes an old canonical on node/tag collision but preserves the Proxy string for the node", () => {
    const base = createConfigFromTemplate("sing-box", "minimal");
    const first = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01"], { type: "selector" }, "sing-box"),
      "sing-box",
    );
    const collision = generateAdaptiveGroups(
      ["Hong Kong", "HK-02"],
      { type: "selector" },
      "sing-box",
    );

    const result = mergeAdaptiveGroups(first.config, collision, "sing-box");

    expect(result.warnings).toContainEqual({ code: "node_name_conflict", groupName: "Hong Kong" });
    expect(namedSingBoxGroup(result.config, "Hong Kong")).toBeUndefined();
    expect(singBoxProxyMembers(result.config)).toContain("Hong Kong");
 });

  it.each([
    ["missing", { code: "anchor_missing" }],
    ["duplicate", { code: "anchor_duplicate", count: 2 }],
    ["invalid-members", { code: "anchor_members_invalid" }],
  ] as const)("leaves a config with a %s Proxy selector anchor unchanged", (fixture, problem) => {
    const base = createConfigFromTemplate("sing-box", "minimal");
    const groups = structuredClone(base.groups ?? []);
    const config = {
      ...base,
      groups: fixture === "missing"
        ? groups.filter((group) => group.tag !== "Proxy")
        : fixture === "duplicate"
          ? [...groups, { type: "selector", tag: "Proxy", outbounds: ["$nodes"] }]
          : groups.map((group) => group.tag === "Proxy" ? { ...group, outbounds: [1] } : group),
   };
    const snapshot = structuredClone(config);
    const generation = generateAdaptiveGroups(
      ["HK-01"],
      { type: "selector" },
      "sing-box",
    );

    expect(adaptiveGroupAnchorProblem(config, "sing-box")).toEqual(problem);
    expect(mergeAdaptiveGroups(config, generation, "sing-box"))
      .toMatchObject({ changed: false, config: snapshot, warnings: [problem] });
    expect(config).toEqual(snapshot);
 });

  it("strips a safe canonical layer back to its sing-box routing template", () => {
    const base = createConfigFromTemplate("sing-box", "minimal");
    const merged = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01"], { type: "selector" }, "sing-box"),
      "sing-box",
    );

    expect(stripCanonicalAdaptiveGroups(merged.config, "sing-box")).toMatchObject({
      changed: true,
      config: base,
      strippedGroupNames: ["Hong Kong"],
   });
 });
});

describe("adaptive Mihomo group reconciliation", () => {
  it("appends canonical groups and inserts references before $nodes without mutating the template", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const snapshot = structuredClone(base);
    deepFreeze(base);
    const generation = generateAdaptiveGroups(
      ["HK-01", "香港-02"],
      { type: "url-test" },
    );

    const first = mergeAdaptiveGroups(base, generation);
    const second = mergeAdaptiveGroups(first.config, generation);

    expect(base).toEqual(snapshot);
    expect(first.changed).toBe(true);
    expect(first.generatedGroupNames).toEqual(["Hong Kong"]);
    expect(first.config.groups?.map((group) => group.name)).toEqual([
      ...(snapshot.groups ?? []).map((group) => group.name),
      "Hong Kong",
    ]);
    expect(proxyMembers(first.config)).toEqual([
      "Auto", "Hong Kong", "$nodes", "DIRECT", "REJECT",
    ]);
    expect(second.changed).toBe(false);
    expect(second.config).toEqual(first.config);
 });

  it("appends multiple canonical groups when the config has no final fallback group", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const groups = (base.groups ?? []).slice(0, -1);
    const config = { ...base, groups };
    const generation = generateAdaptiveGroups(
      ["HK-01", "JP-01"],
      { type: "select", enabledRegionIds: ["hk", "jp"] },
    );

    const result = mergeAdaptiveGroups(config, generation);

    expect(result.config.groups?.map((group) => group.name)).toEqual([
      ...groups.map((group) => group.name),
      "Hong Kong",
      "Japan",
    ]);
  });

  it("replaces an active canonical group when its generated type changes", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const first = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01", "HK-02"], { type: "url-test" }),
    );

    const result = mergeAdaptiveGroups(
      first.config,
      generateAdaptiveGroups(["HK-01", "HK-02"], { type: "select" }),
    );

    expect(namedGroup(result.config, "Hong Kong")).toEqual(expect.objectContaining({ type: "select" }));
    expect(namedGroup(result.config, "Hong Kong")).not.toHaveProperty("url");
    expect(proxyMembers(result.config).filter((name) => name === "Hong Kong")).toHaveLength(1);
 });

  it("recognizes and replaces a legacy Chinese canonical group on regeneration", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const generation = generateAdaptiveGroups(
      ["HK-01", "香港-02"],
      { type: "select" },
    );
    const generated = mergeAdaptiveGroups(base, generation);
    const legacy = {
      ...generated.config,
      groups: generated.config.groups?.map((group) => {
        if (group.name === "Hong Kong") return { ...group, name: "香港节点" };
        if (group.name !== "Proxy" || !Array.isArray(group.proxies)) return group;
        return {
          ...group,
          proxies: group.proxies.map((target) => target === "Hong Kong" ? "香港节点" : target),
       };
     }),
   };

    expect(canonicalAdaptiveGroupNames(legacy.groups ?? [])).toEqual(["香港节点"]);

    const result = mergeAdaptiveGroups(legacy, generation);

    expect(result.changed).toBe(true);
    expect(namedGroup(result.config, "香港节点")).toBeUndefined();
    expect(namedGroup(result.config, "Hong Kong")).toBeDefined();
    expect(proxyMembers(result.config)).toEqual(["Auto", "Hong Kong", "$nodes", "DIRECT", "REJECT"]);
 });

  it("preserves a concrete Proxy member that only trims to a managed group name", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const generation = generateAdaptiveGroups(
      ["HK-01", " Hong Kong "],
      { type: "select" },
    );
    const first = mergeAdaptiveGroups(base, generation);
    const config = {
      ...first.config,
      groups: first.config.groups?.map((group) => group.name === "Proxy"
        ? {
            ...group,
            proxies: proxyMembers(first.config).flatMap((target) => (
              target === "Hong Kong" ? [" Hong Kong ", target] : [target]
            )),
         }
        : group),
   };

    const result = mergeAdaptiveGroups(config, generation);

    expect(proxyMembers(result.config)).toEqual([
      "Auto", " Hong Kong ", "Hong Kong", "$nodes", "DIRECT", "REJECT",
    ]);
 });

  it("preserves a same-name custom group and its Proxy reference", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const groups = structuredClone(base.groups ?? []);
    const proxy = groups.find((group) => group.name === "Proxy");
    if (proxy) proxy.proxies = ["Auto", "Hong Kong", "$nodes", "DIRECT", "REJECT"];
    groups.splice(2, 0, { name: "Hong Kong", type: "select", proxies: ["DIRECT"], icon: "custom" });
    const config = { ...base, groups };

    const result = mergeAdaptiveGroups(
      config,
      generateAdaptiveGroups(["HK-01", "HK-02"], { type: "select" }),
    );

    expect(result.warnings).toContainEqual({ code: "group_name_conflict", groupName: "Hong Kong" });
    expect(result.config.groups?.filter((group) => group.name === "Hong Kong")).toEqual([
      { name: "Hong Kong", type: "select", proxies: ["DIRECT"], icon: "custom" },
    ]);
    expect(proxyMembers(result.config)).toEqual(["Auto", "Hong Kong", "$nodes", "DIRECT", "REJECT"]);
 });

  it.each(["group", "rule"] as const)(
    "preserves a stale canonical referenced by another %s but removes its Proxy entry",
    (referenceKind) => {
      const base = createConfigFromTemplate("mihomo", "minimal");
      const first = mergeAdaptiveGroups(
        base,
        generateAdaptiveGroups(["HK-01", "HK-02"], { type: "select" }),
      );
      const config = referenceKind === "group"
        ? {
            ...first.config,
            groups: [...(first.config.groups ?? []), {
              name: "Dependent",
              type: "select",
              proxies: ["Hong Kong"],
           }],
         }
        : {
            ...first.config,
            rules: [...(first.config.rules ?? []).slice(0, -1), "MATCH,Hong Kong"],
         };

      const result = mergeAdaptiveGroups(
        config,
        generateAdaptiveGroups([], { type: "select" }),
      );

      expect(result.warnings).toContainEqual({ code: "referenced_stale_group", groupName: "Hong Kong" });
      expect(namedGroup(result.config, "Hong Kong")).toBeDefined();
      expect(proxyMembers(result.config)).not.toContain("Hong Kong");
      expect(result.preservedGroupNames).toContain("Hong Kong");
   },
  );

  it("removes an unreferenced stale canonical and its Proxy reference", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const first = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01", "HK-02"], { type: "select" }),
    );

    const result = mergeAdaptiveGroups(
      first.config,
      generateAdaptiveGroups(["HK-01"], { type: "select", enabledRegionIds: [] }),
    );

    expect(result.removedGroupNames).toContain("Hong Kong");
    expect(namedGroup(result.config, "Hong Kong")).toBeUndefined();
    expect(proxyMembers(result.config)).not.toContain("Hong Kong");
 });

  it("removes an old canonical on node-name collision but preserves the Proxy string for the node", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const first = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01", "HK-02"], { type: "select" }),
    );
    const collision = generateAdaptiveGroups(
      ["Hong Kong", "HK-02"],
      { type: "select" },
    );

    const result = mergeAdaptiveGroups(first.config, collision);

    expect(result.warnings).toContainEqual({ code: "node_name_conflict", groupName: "Hong Kong" });
    expect(namedGroup(result.config, "Hong Kong")).toBeUndefined();
    expect(proxyMembers(result.config)).toContain("Hong Kong");
 });

  it("preserves a below-threshold concrete node that collides with an old canonical name", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const first = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01"], { type: "select" }),
    );
    const collision = generateAdaptiveGroups(
      ["Hong Kong"],
      { type: "select" },
    );

    const result = mergeAdaptiveGroups(first.config, collision);

    expect(result.warnings).toContainEqual({ code: "node_name_conflict", groupName: "Hong Kong" });
    expect(namedGroup(result.config, "Hong Kong")).toBeUndefined();
    expect(proxyMembers(result.config)).toContain("Hong Kong");
 });

  it("preserves duplicate canonical groups and their existing Proxy reference", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const first = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01", "HK-02"], { type: "select" }),
    );
    const canonical = namedGroup(first.config, "Hong Kong");
    if (!canonical) throw new Error("expected generated canonical group");
    const config = {
      ...first.config,
      groups: [...(first.config.groups ?? []), structuredClone(canonical)],
   };

    const result = mergeAdaptiveGroups(
      config,
      generateAdaptiveGroups(["HK-01", "HK-02"], { type: "url-test" }),
    );

    expect(result.warnings).toContainEqual({ code: "group_name_conflict", groupName: "Hong Kong" });
    expect(result.config.groups?.filter((group) => group.name === "Hong Kong")).toHaveLength(2);
    expect(proxyMembers(result.config)).toEqual(proxyMembers(config));
 });

  it.each(["duplicate-canonical", "canonical-with-custom"] as const)(
    "reports a stale %s name conflict without changing the existing groups",
    (fixture) => {
      const base = createConfigFromTemplate("mihomo", "minimal");
      const first = mergeAdaptiveGroups(
        base,
        generateAdaptiveGroups(["HK-01"], { type: "select" }),
      );
      const canonical = namedGroup(first.config, "Hong Kong");
      if (!canonical) throw new Error("expected generated canonical group");
      const conflicting = fixture === "duplicate-canonical"
        ? structuredClone(canonical)
        : { name: "Hong Kong", type: "select", proxies: ["DIRECT"], icon: "custom" };
      const config = { ...first.config, groups: [...(first.config.groups ?? []), conflicting] };

      const result = mergeAdaptiveGroups(
        config,
        generateAdaptiveGroups([], { type: "select" }),
      );

      expect(result.changed).toBe(false);
      expect(result.config).toEqual(config);
      expect(result.warnings).toContainEqual({ code: "group_name_conflict", groupName: "Hong Kong" });
      expect(proxyMembers(result.config)).toEqual(proxyMembers(config));
   },
  );

  it.each([
    ["missing", { code: "anchor_missing" }],
    ["duplicate", { code: "anchor_duplicate", count: 2 }],
    ["invalid-members", { code: "anchor_members_invalid" }],
  ] as const)("leaves a config with a %s Proxy anchor unchanged", (fixture, problem) => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const groups = structuredClone(base.groups ?? []);
    const config = {
      ...base,
      groups: fixture === "missing"
        ? groups.filter((group) => group.name !== "Proxy")
        : fixture === "duplicate"
          ? [...groups, { name: "Proxy", type: "select", proxies: ["$nodes"] }]
          : groups.map((group) => group.name === "Proxy" ? { ...group, proxies: [1] } : group),
   };
    const snapshot = structuredClone(config);

    expect(adaptiveGroupAnchorProblem(config)).toEqual(problem);
    expect(mergeAdaptiveGroups(
      config,
      generateAdaptiveGroups(["HK-01"], { type: "select" }),
    )).toMatchObject({ changed: false, config: snapshot, warnings: [problem] });
    expect(config).toEqual(snapshot);
 });

  it.each(["select", "url-test", "load-balance"] as const)(
    "strips a safe %s canonical layer back to its routing template",
    (type) => {
      const base = createConfigFromTemplate("mihomo", "minimal");
      const merged = mergeAdaptiveGroups(
        base,
        generateAdaptiveGroups(["HK-01"], { type }),
      );

      const stripped = stripCanonicalAdaptiveGroups(merged.config);

      expect(canonicalAdaptiveGroupNames(merged.config.groups ?? [])).toEqual(["Hong Kong"]);
      expect(stripped).toMatchObject({ changed: true, config: base, strippedGroupNames: ["Hong Kong"] });
   },
  );

  it("does not strip a same-name group with any custom field", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const merged = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01"], { type: "select" }),
    );
    const config = {
      ...merged.config,
      groups: merged.config.groups?.map((group) => group.name === "Hong Kong"
        ? { ...group, icon: "custom" }
        : group),
   };
    const snapshot = structuredClone(config);

    deepFreeze(config);
    expect(canonicalAdaptiveGroupNames(config.groups ?? [])).toEqual([]);
    expect(stripCanonicalAdaptiveGroups(config)).toEqual({
      changed: false,
      config: snapshot,
      strippedGroupNames: [],
   });
 });

  it("strips a canonical duplicate of a custom group without deleting their shared Proxy reference", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const merged = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01"], { type: "select" }),
    );
    const config = {
      ...merged.config,
      groups: [...(merged.config.groups ?? []), {
        name: "Hong Kong",
        type: "select",
        proxies: ["DIRECT"],
        icon: "custom",
     }],
   };

    const stripped = stripCanonicalAdaptiveGroups(config);

    expect(stripped.changed).toBe(true);
    expect(stripped.config.groups?.filter((group) => group.name === "Hong Kong")).toEqual([
      { name: "Hong Kong", type: "select", proxies: ["DIRECT"], icon: "custom" },
    ]);
    expect(proxyMembers(stripped.config)).toContain("Hong Kong");
 });

  it("does not strip canonical groups when the Proxy anchor is duplicated", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const merged = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01"], { type: "select" }),
    );
    const config = {
      ...merged.config,
      groups: [...(merged.config.groups ?? []), { name: "Proxy", type: "select", proxies: ["$nodes"] }],
   };

    expect(canonicalAdaptiveGroupNames(config.groups ?? [])).toEqual(["Hong Kong"]);
    expect(stripCanonicalAdaptiveGroups(config)).toEqual({
      changed: false,
      config,
      strippedGroupNames: [],
   });
 });

  it("does not strip a canonical group referenced outside Proxy", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const merged = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01"], { type: "select" }),
    );
    const config = {
      ...merged.config,
      groups: [...(merged.config.groups ?? []), {
        name: "Dependent",
        type: "select",
        proxies: ["Hong Kong"],
     }],
   };

    expect(stripCanonicalAdaptiveGroups(config)).toEqual({
      changed: false,
      config,
      strippedGroupNames: [],
   });
 });

  it("does not strip duplicate exact canonical groups", () => {
    const base = createConfigFromTemplate("mihomo", "minimal");
    const merged = mergeAdaptiveGroups(
      base,
      generateAdaptiveGroups(["HK-01"], { type: "select" }),
    );
    const canonical = namedGroup(merged.config, "Hong Kong");
    if (!canonical) throw new Error("expected generated canonical group");
    const config = {
      ...merged.config,
      groups: [...(merged.config.groups ?? []), structuredClone(canonical)],
   };

    expect(stripCanonicalAdaptiveGroups(config)).toEqual({
      changed: false,
      config,
      strippedGroupNames: [],
   });
 });
});

function proxyMembers(config: FileConfigDraft): string[] {
  const proxies = namedGroup(config, "Proxy")?.proxies;
  return Array.isArray(proxies) ? proxies.filter((item): item is string => typeof item === "string") : [];
}

function namedGroup(config: FileConfigDraft, name: string): Record<string, unknown> | undefined {
  return config.groups?.find((group) => group.name === name);
}

function singBoxProxyMembers(config: FileConfigDraft): string[] {
  const outbounds = namedSingBoxGroup(config, "Proxy")?.outbounds;
  return Array.isArray(outbounds) ? outbounds.filter((item): item is string => typeof item === "string") : [];
}

function namedSingBoxGroup(config: FileConfigDraft, tag: string): Record<string, unknown> | undefined {
  return config.groups?.find((group) => group.tag === tag);
}

function deepFreeze<T>(value: T): T {
  if (!value || typeof value !== "object" || Object.isFrozen(value)) return value;
  Object.freeze(value);
  for (const child of Object.values(value)) deepFreeze(child);
  return value;
}
