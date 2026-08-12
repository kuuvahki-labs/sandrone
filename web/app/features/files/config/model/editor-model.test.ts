import { describe, expect, it } from "vitest";

import { FILE_DRIVER_REGISTRY } from "~/features/files/drivers/registry";
import type { FileConfigDraft, FileKind, RuleSetCatalogItem } from "~/features/files/model/types";
import { zhCN } from "~/shared/i18n/translations/zh-CN";

import type { ConfigMap, RuleDraft, RuleSetDraft } from "./editor-model";
import type { ConfigTemplateID } from "./templates";

describe("file config model", () => {
  const domainItem: RuleSetCatalogItem = {
    name: "geosite-cn",
    ruleKind: "domain",
    url: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs",
  };

  it.each(["(?i)", "[", "(HK)\\1", "`HK`"])('accepts backend-safe Shadowrocket filter %j', (filter) => {
    expect(validShadowrocketGroupFilter(filter)).toBe(true);
  });

  it.each(["", "HK,hidden=1", "HK\nTW", "HK\rTW"])('rejects backend-unsafe Shadowrocket filter %j', (filter) => {
    expect(validShadowrocketGroupFilter(filter)).toBe(false);
  });

  it.each([
    ["example.com", true],
    [" Proxy ", true],
    ["", false],
    ["example.com,DIRECT", false],
    ["Proxy\nDIRECT", false],
    ["Proxy\rDIRECT", false],
  ] as const)("validates Shadowrocket structured rule component %j", (value, expected) => {
    expect(validShadowrocketRuleComponent(value)).toBe(expected);
  });

  it("does not serialize unsafe Shadowrocket structured rule fields", () => {
    expect(serializeRules("shadowrocket", [
      { id: "valid", type: "domain", value: "example.com", policy: "Proxy" },
      { id: "value-comma", type: "domain", value: "example.org,DIRECT", policy: "Proxy" },
      { id: "policy-comma", type: "domain", value: "example.net", policy: "Proxy,DIRECT" },
      { id: "policy-newline", type: "final", value: "", policy: "Proxy\nDIRECT" },
    ])).toEqual(["DOMAIN,example.com,Proxy"]);
  });

  it("uses backend runtime defaults for omitted Shadowrocket sections", () => {
    const config = initialConfig("shadowrocket", {}, "zh-CN");

    expect(config.groups).toEqual([{ name: "Proxy", type: "select", proxies: ["$nodes", "DIRECT"] }]);
    expect(config.ruleSets).toEqual([]);
    expect(serializeRules("shadowrocket", config.rules)).toEqual([
      "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
      "IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
      "IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
      "GEOIP,CN,DIRECT",
      "FINAL,Proxy",
    ]);
  });

  it.each(["mihomo", "sing-box"] as const)("uses localized defaults for new %s entries", (kind) => {
    const key = kind === "sing-box" ? "tag" : "name";
    expect(defaultGroups(kind, "minimal", "zh-CN")[0][key]).toBe("🚀 节点选择");
    expect(defaultRuleDraft(kind, 0, "zh-CN")).toMatchObject({ value: "custom", policy: "🚀 节点选择" });
  });

  it.each([
    ["mihomo", "geoip"],
    ["shadowrocket", "geoip"],
    ["shadowrocket", "ip-cidr"],
  ] as const)("defaults no-resolve on when changing a %s rule to %s", (kind, type) => {
    const adapter = structuredAdapter(kind);
    const rule = adapter.rules.transitionType(defaultRuleDraft(kind, 0), type);

    expect(rule.noResolve).toBe(true);
  });

  it.each(["mihomo", "sing-box"] as const)("reads preset-only %s group names from the shared locale catalog", (kind) => {
    const fallbackKey = "files.config.outputNames.group.fallback";
    const otherKey = "files.config.outputNames.group.other";
    const catalog = zhCN as unknown as Record<typeof fallbackKey | typeof otherKey, string>;
    const originalFallback = catalog[fallbackKey];
    const originalOther = catalog[otherKey];
    catalog[fallbackKey] = "临时故障转移";
    catalog[otherKey] = "临时其他";

    try {
      const nameKey = kind === "sing-box" ? "tag" : "name";
      expect(defaultGroups(kind, "fallback", "zh-CN").map((group) => group[nameKey])).toContain("临时故障转移");
      expect(defaultGroups(kind, "region", "zh-CN").map((group) => group[nameKey])).toContain("临时其他");
    } finally {
      catalog[fallbackKey] = originalFallback;
      catalog[otherKey] = originalOther;
    }
  });

  it.each([
    [domainItem, "domain"],
    [{ ...domainItem, name: "geoip-private", ruleKind: "ip" as const }, "ipcidr"],
  ] as const)("maps a Mihomo catalog item to an MRS provider", (entry, behavior) => {
    const result = addCatalogRuleSet({ kind: "mihomo", entry }, []);
    expect(result.status).toBe("added");
    if (result.status !== "added") return;
    expect(result.ruleSets).toMatchObject([{
      name: entry.name,
      source: "remote",
      behavior,
      format: "mrs",
      interval: "86400",
      url: entry.url,
    }]);
  });

  it("maps a sing-box catalog item to a binary remote rule set", () => {
    const entry = {
      ...domainItem,
      url: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.srs",
    };
    const result = addCatalogRuleSet({ kind: "sing-box", entry }, []);
    expect(result.status).toBe("added");
    if (result.status !== "added") return;
    expect(result.ruleSets[0]).toMatchObject({ format: "binary", interval: "1d", url: entry.url });
  });

  it.each([
    ["RULE-SET", "rule-set"],
    ["DOMAIN-SET", "domain-set"],
  ] as const)("maps a Shadowrocket catalog %s reference to its lowercase type", (referenceType, behavior) => {
    const entry: RuleSetCatalogItem = {
      ...domainItem,
      referenceType,
      url: `https://example.com/${behavior}.list`,
    };
    const result = addCatalogRuleSet({ kind: "shadowrocket", entry }, []);

    expect(result.status).toBe("added");
    if (result.status !== "added") return;
    expect(result.ruleSets[0]).toMatchObject({
      name: entry.name,
      source: "remote",
      behavior,
      url: entry.url,
    });
  });

  it("rejects catalog URL and name conflicts", () => {
    const existing = {
      id: "existing",
      name: "GeoSite-CN",
      source: "remote" as const,
      behavior: "domain",
      format: "mrs",
      interval: "86400",
      payloadText: "",
      url: domainItem.url,
    };
    expect(addCatalogRuleSet({ kind: "mihomo", entry: domainItem }, [existing])).toEqual({
      status: "duplicate-url",
      existingName: "GeoSite-CN",
    });
    expect(addCatalogRuleSet({ kind: "mihomo", entry: { ...domainItem, url: "https://example.com/other.mrs" } }, [existing])).toEqual({
      status: "name-conflict",
      existingName: "GeoSite-CN",
    });
  });

  it.each([
    ["mihomo", "yaml", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.yaml"],
    ["mihomo", "text", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/cn.yaml", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/cn.list"],
    ["mihomo", "mrs", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.list", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs"],
    ["sing-box", "source", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.srs", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.json"],
    ["sing-box", "binary", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/cn.json", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/cn.srs"],
  ] as const)("links %s official catalog URLs to the selected %s format", (kind, format, url, expectedURL) => {
    expect(ruleSetFormatPatch(kind, url, format)).toEqual({ format, url: expectedURL });
  });

  it("preserves encoded official rule-set names while changing format", () => {
    const url = "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/geolocation-%21cn.srs";
    expect(ruleSetFormatPatch("sing-box", url, "source")).toEqual({
      format: "source",
      url: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/geolocation-%21cn.json",
    });
  });

  it.each([
    ["mihomo", "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs", "yaml"],
    ["mihomo", "https://example.com/geo/geosite/cn.mrs", "text"],
    ["mihomo", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.srs", "mrs"],
    ["sing-box", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs", "binary"],
    ["sing-box", "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.srs?download=1", "source"],
  ] as const)("leaves non-matching %s URLs unchanged", (kind, url, format) => {
    expect(ruleSetFormatPatch(kind, url, format)).toEqual({ format, url });
  });

  it("round-trips a Mihomo HTTP rule set through wizard mode", () => {
    const config = initialConfig("mihomo", {
      rule_sets: [{
        name: "reject",
        type: "http",
        behavior: "domain",
        format: "text",
        interval: 3600,
        url: "https://example.com/reject.list",
      }],
      rules: ["RULE-SET,reject,REJECT", "MATCH,Proxy"],
    });

    expect(config.mode).toBe("wizard");
    expect(config.ruleSets[0]).toMatchObject({
      source: "remote",
      name: "reject",
      behavior: "domain",
      format: "text",
      interval: "3600",
      url: "https://example.com/reject.list",
    });
    expect(serializeRuleSets("mihomo", config.ruleSets)).toEqual([{
      name: "reject",
      type: "http",
      behavior: "domain",
      format: "text",
      interval: 3600,
      url: "https://example.com/reject.list",
    }]);
  });

  it("round-trips a sing-box remote rule set through wizard mode", () => {
    const config = initialConfig("sing-box", {
      rule_sets: [{
        type: "remote",
        tag: "geosite-cn",
        format: "binary",
        update_interval: "12h",
        url: "https://example.com/geosite-cn.srs",
      }],
      rules: [{ rule_set: ["geosite-cn"], outbound: "direct" }],
    });

    expect(config.mode).toBe("wizard");
    expect(config.ruleSets[0]).toMatchObject({
      source: "remote",
      name: "geosite-cn",
      format: "binary",
      interval: "12h",
      url: "https://example.com/geosite-cn.srs",
    });
    expect(serializeRuleSets("sing-box", config.ruleSets)).toEqual([{
      type: "remote",
      tag: "geosite-cn",
      format: "binary",
      update_interval: "12h",
      url: "https://example.com/geosite-cn.srs",
    }]);
  });

  it("round-trips sing-box DNS transport and resolving fallback rules through wizard mode", () => {
    const rules = [
      { port: 853, outbound: "Proxy" },
      { action: "resolve" },
      { rule_set: ["cn-ip"], outbound: "direct" },
      { outbound: "Proxy" },
    ];
    const config = initialConfig("sing-box", { rules });

    expect(config.mode).toBe("wizard");
    expect(config.rules.map((rule) => rule.type)).toEqual(["port", "resolve", "rule-set", "match"]);
    expect(config.rules[1]).toMatchObject({ type: "resolve", value: "", policy: "" });
    expect(ruleRequiresPolicy("sing-box", "resolve")).toBe(false);
    expect(serializeRules("sing-box", config.rules)).toEqual(rules);
  });

  it("round-trips strict Shadowrocket rule sets and ordered string rules", () => {
    const settings = {
      rule_sets: [
        { name: "ads", type: "domain-set", url: "https://example.com/ads.list" },
        { name: "lan", type: "rule-set", url: "https://example.com/lan.list" },
      ],
      rules: [
        "DOMAIN-SUFFIX,example.com,Proxy",
        "DST-PORT,853,Proxy",
        "DOMAIN-SET,ads,REJECT",
        "RULE-SET,lan,DIRECT,no-resolve",
        "GEOIP,CN,DIRECT,no-resolve",
        "FINAL,Proxy",
      ],
    };
    const config = initialConfig("shadowrocket", settings);

    expect(config.mode).toBe("wizard");
    expect(config.ruleSets.map((item) => item.behavior)).toEqual(["domain-set", "rule-set"]);
    expect(serializeRuleSets("shadowrocket", config.ruleSets)).toEqual(settings.rule_sets);
    expect(serializeRules("shadowrocket", config.rules)).toEqual(settings.rules);
    expect(config.rules.map((rule) => rule.type)).toEqual([
      "domain-suffix", "dst-port", "domain-set", "rule-set", "geoip", "final",
    ]);
  });

  it("falls back to advanced mode for unsupported Shadowrocket rule-set fields", () => {
    expect(initialConfig("shadowrocket", {
      rule_sets: [{ name: "ads", type: "domain-set", url: "https://example.com/ads.list", interval: 3600 }],
      rules: ["FINAL,DIRECT"],
    }).mode).toBe("advanced");
  });

  it.each(["PROCESS-NAME,Safari,DIRECT", "IP-CIDR6,2001:db8::/32,DIRECT"]) (
    "keeps undocumented Shadowrocket rule syntax in advanced mode: %s",
    (rule) => {
      expect(initialConfig("shadowrocket", { rules: [rule] }).mode).toBe("advanced");
    },
  );

  it("recognizes Shadowrocket fixed and runtime-filter group membership without mixing fields", () => {
    expect(proxyGroupMemberMode("shadowrocket", {
      name: "HK",
      type: "url-test",
      "policy-regex-filter": "(?i)HK",
    })).toBe("runtime-filter");
    expect(proxyGroupHasMemberSource("shadowrocket", {
      name: "HK",
      type: "url-test",
      "policy-regex-filter": "(?i)HK",
    })).toBe(true);
    expect(proxyGroupHasMemberSource("shadowrocket", {
      name: "Proxy",
      type: "select",
      proxies: ["$nodes", "DIRECT"],
    })).toBe(true);
  });

  it.each([
    ["domain-suffix", false],
    ["rule-set", true],
    ["domain-set", false],
    ["geoip", true],
    ["ip-cidr", true],
    ["final", false],
  ] as const)("marks Shadowrocket no-resolve support for %s", (type, expected) => {
    expect(ruleSupportsNoResolve("shadowrocket", type)).toBe(expected);
  });

  it.each(["minimal", "standard", "full"] satisfies ConfigTemplateID[])(
    "round-trips the Mihomo %s template without dropping no-resolve",
    (templateID) => {
      const template = createConfigFromTemplate("mihomo", templateID);
      const config = initialConfig("mihomo", template);

      expect(config.mode).toBe("wizard");
      expect(config.rules.some((rule) => rule.noResolve)).toBe(true);
      expect(serializeRules("mihomo", config.rules)).toEqual(template.rules);
    },
  );

  it.each(["sing-box", "shadowrocket"] as const)(
    "round-trips every generated %s template through wizard mode",
    (kind) => {
      for (const templateID of ["minimal", "standard", "full"] satisfies ConfigTemplateID[]) {
        const template = createConfigFromTemplate(kind, templateID);
        const config = initialConfig(kind, template);

        expect(config.mode).toBe("wizard");
        expect(serializeRules(kind, config.rules)).toEqual(template.rules);
      }
    },
  );

  it("keeps unsupported remote rule set fields in advanced mode", () => {
    const config = initialConfig("sing-box", {
      rule_sets: [{
        type: "remote",
        tag: "private",
        format: "source",
        url: "https://example.com/private.json",
        http_client: "rules-client",
      }],
      rules: [{ rule_set: ["private"], outbound: "direct" }],
    });

    expect(config.mode).toBe("advanced");
  });

  it("accepts a valid include-all-proxies runtime filter as the member source", () => {
    const group = {
      name: "香港节点",
      type: "url-test",
      "include-all-proxies": true,
      filter: String.raw`(?i)(?:香港|\bHK\b)`,
      "exclude-filter": "(?i)(?:家宽|实验)",
    };

    expect(proxyGroupMemberMode("mihomo", group)).toBe("runtime-filter");
    expect(proxyGroupHasMemberSource("mihomo", group)).toBe(true);
    expect(validMihomoGroupFilter(group.filter)).toBe(true);
    expect(validMihomoGroupFilter(group["exclude-filter"])).toBe(true);
  });

  it.each(["", "(?i)", "(?<=HK)", "(?=HK)", "(?!HK)", "(HK)\\1", "HK`US", "["])(
    "rejects the unsupported Mihomo filter %j",
    (filter) => expect(validMihomoGroupFilter(filter)).toBe(false),
  );

  it("does not treat provider-wide or empty filter fields as a valid member source", () => {
    expect(proxyGroupHasMemberSource("mihomo", { "include-all": true, filter: "(?i)HK" })).toBe(false);
    expect(proxyGroupHasMemberSource("mihomo", { "include-all-proxies": true, filter: "" })).toBe(false);
    expect(proxyGroupHasMemberSource("mihomo", {
      "include-all-proxies": true,
      filter: "(?i)HK",
      "exclude-filter": { invalid: true },
    })).toBe(false);
  });
});

function initialConfig(kind: FileKind, value?: FileConfigDraft, locale: "en-US" | "zh-CN" = "en-US") {
  const adapter = structuredAdapter(kind);
  const draft = adapter.initialize(value, locale);
  return { ...draft, groups: adapter.groups.serialize(draft.groups) };
}

function serializeRuleSets(kind: FileKind, values: RuleSetDraft[]) {
  return structuredAdapter(kind).ruleSets.serialize(values);
}

function serializeRules(kind: FileKind, values: RuleDraft[]) {
  return structuredAdapter(kind).rules.serialize(values);
}

function defaultGroups(kind: FileKind, preset: string, locale: "en-US" | "zh-CN" = "en-US") {
  const adapter = structuredAdapter(kind);
  return adapter.groups.serialize(adapter.groups.defaults(preset, locale));
}

function defaultRuleDraft(kind: FileKind, index: number, locale: "en-US" | "zh-CN" = "en-US") {
  return structuredAdapter(kind).rules.create(index, locale);
}

function addCatalogRuleSet(
  request: { entry: RuleSetCatalogItem; kind: FileKind },
  current: RuleSetDraft[],
) {
  return structuredAdapter(request.kind).ruleSets.fromCatalog(request.entry, current);
}

function ruleSetFormatPatch(kind: FileKind, url: string, format: string) {
  return structuredAdapter(kind).ruleSets.formatPatch(url, format);
}

function ruleSupportsNoResolve(kind: FileKind, type: string) {
  return structuredAdapter(kind).rules.supportsNoResolve(type);
}

function ruleRequiresPolicy(kind: FileKind, type: string) {
  return structuredAdapter(kind).rules.requiresPolicy(type);
}

function proxyGroupMemberMode(kind: FileKind, group: ConfigMap) {
  return (kind === "mihomo" && group["include-all-proxies"] === true)
    || (kind === "shadowrocket" && typeof group["policy-regex-filter"] === "string")
    ? "runtime-filter"
    : "fixed";
}

function proxyGroupHasMemberSource(kind: FileKind, group: ConfigMap) {
  const adapter = structuredAdapter(kind);
  const targets = kind === "sing-box" ? group.outbounds : group.proxies;
  if (Array.isArray(targets) && targets.length > 0) return true;
  if (proxyGroupMemberMode(kind, group) === "fixed") return false;
  const filter = kind === "shadowrocket" ? group["policy-regex-filter"] : group.filter;
  const nativeExcludeFilter = group["exclude-filter"];
  return adapter.groups.validateFilter(filter)
    && (!adapter.groups.supportsExcludeFilter
      || nativeExcludeFilter === undefined
      || nativeExcludeFilter === ""
      || adapter.groups.validateFilter(nativeExcludeFilter));
}

function validMihomoGroupFilter(value: unknown) {
  return structuredAdapter("mihomo").groups.validateFilter(value);
}

function validShadowrocketGroupFilter(value: unknown) {
  return structuredAdapter("shadowrocket").groups.validateFilter(value);
}

function validShadowrocketRuleComponent(value: unknown) {
  return structuredAdapter("shadowrocket").rules.validateComponent(value);
}

function structuredAdapter(kind: FileKind) {
  const configuration = FILE_DRIVER_REGISTRY.get(kind)?.configuration;
  if (configuration?.mode !== "structured") throw new Error(`expected structured driver: ${kind}`);
  return configuration.adapter;
}

function createConfigFromTemplate(kind: FileKind, templateID: ConfigTemplateID) {
  return structuredAdapter(kind).templates.create(templateID);
}
