import { readFileSync } from "node:fs";

import { describe, expect, expectTypeOf, it } from "vitest";

import type {
  ConfigEditorDraft,
  GroupDraft,
  RuleDraft,
  RuleSetDraft,
} from "~/features/files/config/model/editor-model";
import type { RuleSetCatalogItem } from "~/features/files/model/types";

import { FILE_DRIVER_REGISTRY } from "./registry";

describe("structured file driver editor models", () => {
  it("keeps the shared editor model free of registered target selection", () => {
    const source = readFileSync(new URL("../config/model/editor-model.ts", import.meta.url), "utf8");

    expect(source).not.toMatch(/\bFileKind\b/);
    expect(source).not.toMatch(/["'](?:mihomo|sing-box|shadowrocket)["']/);
  });

  it.each(["mihomo", "sing-box", "shadowrocket"])(
    "%s keeps the minimal template canonical across the editor round-trip",
    (kind) => {
      const adapter = structuredAdapter(kind);
      const template = adapter.templates.create("minimal", "zh-CN");
      const native = adapter.toNativeDraft(adapter.initialize(template, "zh-CN"));

      expect(native).not.toHaveProperty("settingsMode");
      expect(adapter.templates.recognize(native)).toEqual({
        adaptive: false,
        match: "minimal",
        namingLocale: "zh-CN",
      });
    },
  );

  it.each(["mihomo", "sing-box", "shadowrocket"])(
    "%s materializes missing sections and preserves explicit empty sections",
    (kind) => {
      const adapter = structuredAdapter(kind);
      const missing = adapter.decode({ subscriptions: ["one", "two"], settingsPresent: false }, "en-US");
      const empty = adapter.decode({ settingsPresent: true, settings: {} }, "en-US");
      const explicit = adapter.decode({
        settingsPresent: true,
        settings: { groups: [], rule_sets: [], rules: [] },
      }, "en-US");

      expect(missing).toMatchObject({ subscriptions: ["one", "two"], settingsMode: "structured" });
      expect(empty).toMatchObject({ settingsMode: "structured" });
      expect(missing?.groups.length).toBeGreaterThan(0);
      expect(missing?.rules.length).toBeGreaterThan(0);
      expect(empty?.groups.length).toBeGreaterThan(0);
      expect(empty?.rules.length).toBeGreaterThan(0);
      expect(explicit).toMatchObject({
        settingsMode: "structured",
        mode: "wizard",
        groups: [],
        ruleSets: [],
        rules: [],
      });
      expect(adapter.encode(missing!)).toMatchObject({
        subscriptions: ["one", "two"],
        settings: { groups: expect.any(Array), rule_sets: expect.any(Array), rules: expect.any(Array) },
      });
      expect(adapter.encode(empty!)).toMatchObject({
        settings: { groups: expect.any(Array), rule_sets: expect.any(Array), rules: expect.any(Array) },
      });
      expect(adapter.encode(explicit!)).toEqual({ settings: { groups: [], rule_sets: [], rules: [] } });
      expect(adapter.validateSettings({ groups: [], rule_sets: [], rules: [] })).toBe(true);
      expect(adapter.validateSettings({ groups: [], rules: [] })).toBe(false);
    },
  );

  it("keeps raw fallback and advanced native-list fallback as different lifecycles", () => {
    const adapter = structuredAdapter("sing-box");
    const rawSettings = { groups: [], future_nested: { keep: true } };
    const advancedSettings = {
      groups: [],
      rule_sets: [{
        type: "remote",
        tag: "private",
        format: "source",
        url: "https://example.com/private.json",
        http_client: "rules-client",
      }],
      rules: [{ rule_set: ["private"], outbound: "direct" }],
    };

    const raw = adapter.decode({ settingsPresent: true, settings: rawSettings }, "en-US");
    const advanced = adapter.decode({ settingsPresent: true, settings: advancedSettings }, "en-US");

    expect(raw).toMatchObject({ settingsMode: "raw", rawSettings });
    expect(advanced).toMatchObject({
      settingsMode: "structured",
      mode: "advanced",
      advancedRuleSetsText: JSON.stringify(advancedSettings.rule_sets, null, 2),
    });
    expect(adapter.encode(raw!)).toEqual({ settings: rawSettings });
    expect(adapter.encode(advanced!)).toEqual({ settings: advancedSettings });
  });

  it.each([
    ["multi-rule-set route", {
      groups: [],
      rule_sets: [],
      rules: [{ rule_set: ["one", "two"], outbound: "block", invert: true }],
    }],
    ["route with an extra matcher", {
      groups: [],
      rule_sets: [],
      rules: [{ domain_suffix: ["example.com"], outbound: "block" }],
    }],
    ["inline rule-set with unsupported matchers", {
      groups: [],
      rule_sets: [{
        type: "inline",
        tag: "applications",
        rules: [{
          domain: ["example.com"],
          domain_suffix: ["example.org"],
          process_name: ["curl"],
        }],
      }],
      rules: [],
    }],
  ] as const)("keeps a sing-box %s in advanced mode without semantic loss", (_label, settings) => {
    const adapter = structuredAdapter("sing-box");

    const draft = adapter.decode({ settingsPresent: true, settings }, "en-US")!;

    expect(draft).toMatchObject({ settingsMode: "structured", mode: "advanced" });
    expect(adapter.encode(draft)).toEqual({ settings });
  });

  it.each([
    ["mihomo", {
      name: "Proxy",
      type: "select",
      proxies: ["$nodes", "DIRECT"],
      icon: "https://example.com/icon.png",
      future_nested: { keep: true },
    }],
    ["sing-box", {
      type: "selector",
      tag: "Proxy",
      outbounds: ["$nodes", "direct"],
      interrupt_exist_connections: true,
      future_nested: { keep: true },
    }],
  ] as const)("round-trips opaque %s group fields through the normalized wizard", (kind, nativeGroup) => {
    const adapter = structuredAdapter(kind);
    const draft = adapter.decode({
      settingsPresent: true,
      settings: { groups: [nativeGroup], rule_sets: [], rules: [] },
    }, "en-US")!;

    expect(draft).toMatchObject({
      mode: "wizard",
      groups: [{ name: "Proxy", memberMode: "fixed" }],
    });
    expect(draft.groups[0]).not.toHaveProperty("tag");
    expect(draft.groups[0]).not.toHaveProperty("proxies");
    expect(draft.groups[0]).not.toHaveProperty("outbounds");
    expect(adapter.encode(draft)).toEqual({
      settings: { groups: [nativeGroup], rule_sets: [], rules: [] },
    });
  });

  it.each([
    ["mihomo provider-only membership", "mihomo", {
      name: "Provider",
      type: "select",
      use: ["airport"],
    }],
    ["mihomo string interval", "mihomo", {
      name: "Auto",
      type: "url-test",
      proxies: ["$nodes"],
      url: "https://cp.cloudflare.com",
      interval: "300",
    }],
    ["sing-box numeric interval", "sing-box", {
      type: "urltest",
      tag: "Auto",
      outbounds: ["$nodes"],
      url: "https://example.com/generate_204",
      interval: 300,
    }],
    ["sing-box future type", "sing-box", {
      type: "future-selector",
      tag: "Future",
      outbounds: ["$nodes"],
    }],
    ["Shadowrocket select timing fields", "shadowrocket", {
      name: "Proxy",
      type: "select",
      proxies: ["$nodes", "DIRECT"],
      interval: 300,
      timeout: 5,
      tolerance: 50,
    }],
  ] as const)("keeps an unrepresentable %s group in advanced mode", (_label, kind, nativeGroup) => {
    const adapter = structuredAdapter(kind);
    const settings = { groups: [nativeGroup], rule_sets: [], rules: [] };

    const draft = adapter.decode({ settingsPresent: true, settings }, "en-US")!;

    expect(draft).toMatchObject({
      settingsMode: "structured",
      mode: "advanced",
      advancedGroupsText: JSON.stringify(settings.groups, null, 2),
    });
    expect(adapter.encode(draft)).toEqual({ settings });
  });

  it.each([
    ["rule with an extra token", { rule_sets: [], rules: ["MATCH,Proxy,EXTRA"] }],
    ["remote rule-set with a string interval", {
      rule_sets: [{
        name: "private",
        type: "http",
        behavior: "classical",
        format: "yaml",
        interval: "300",
        url: "https://example.com/private.yaml",
      }],
      rules: [],
    }],
    ["inline rule-set with significant payload whitespace", {
      rule_sets: [{
        name: "private",
        type: "inline",
        behavior: "classical",
        payload: [" DOMAIN-SUFFIX,example.com "],
      }],
      rules: [],
    }],
  ] as const)("keeps a Mihomo %s in advanced mode without semantic loss", (_label, partialSettings) => {
    const adapter = structuredAdapter("mihomo");
    const settings = { groups: [], ...partialSettings };

    const draft = adapter.decode({ settingsPresent: true, settings }, "en-US")!;

    expect(draft).toMatchObject({ settingsMode: "structured", mode: "advanced" });
    expect(adapter.encode(draft)).toEqual({ settings });
  });

  it.each([
    ["rule-set fields with significant whitespace", {
      rule_sets: [{
        name: " private ",
        type: "rule-set",
        url: " https://example.com/private.list ",
      }],
      rules: [],
    }],
    ["rule with significant whitespace", {
      rule_sets: [],
      rules: [" DOMAIN-SUFFIX, example.com , Proxy "],
    }],
  ] as const)("keeps a backend-accepted Shadowrocket %s in advanced mode without semantic loss", (_label, partialSettings) => {
    const adapter = structuredAdapter("shadowrocket");
    const settings = { groups: [], ...partialSettings };

    const draft = adapter.decode({ settingsPresent: true, settings }, "en-US")!;

    expect(draft).toMatchObject({ settingsMode: "structured", mode: "advanced" });
    expect(adapter.encode(draft)).toEqual({ settings });
  });

  it("normalizes sing-box group types and delegates transitions to its adapter", () => {
    const adapter = structuredAdapter("sing-box");
    const projected = adapter.groups.project([{
      type: "urltest",
      tag: "Auto",
      outbounds: ["$nodes"],
      url: "http://www.gstatic.com/generate_204",
      interval: "5m",
      default: "Node A",
    }]);
    if (!projected) throw new Error("expected representable sing-box group");
    const group = projected[0];

    expect(group).toMatchObject({
      name: "Auto",
      type: "url-test",
      members: ["$nodes"],
      healthCheckURL: "http://www.gstatic.com/generate_204",
      healthCheckInterval: "5m",
    });
    expect(adapter.groups.transitionType(group, "select")).toMatchObject({
      type: "select",
      healthCheckURL: "",
      healthCheckInterval: "",
    });
    expect(adapter.groups.typeOptions.map((option) => option.value)).toEqual(["select", "url-test"]);
  });

  it("preserves malformed opaque Mihomo filters while blocking wizard submission", () => {
    const adapter = structuredAdapter("mihomo");
    const settings = {
      groups: [{
        name: "Hong Kong",
        type: "select",
        "include-all-proxies": true,
        filter: "(?i)HK",
        "exclude-filter": { future: true },
      }],
      rule_sets: [],
      rules: [],
    };
    const draft = adapter.decode({ settingsPresent: true, settings }, "en-US")!;

    expect(adapter.validate(draft)).toContainEqual(expect.objectContaining({ code: "group_filter_invalid" }));
    expect(adapter.encode(draft)).toEqual({ settings });
  });

  it("owns rule-set catalog conversion and native syntax validation per driver", () => {
    const entry: RuleSetCatalogItem = {
      name: "geosite-cn",
      ruleKind: "domain",
      url: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs",
    };
    const mihomo = structuredAdapter("mihomo");
    const shadowrocket = structuredAdapter("shadowrocket");

    expect(mihomo.ruleSets.fromCatalog(entry, [])).toMatchObject({
      status: "added",
      ruleSets: [{ behavior: "domain", format: "mrs", interval: "86400" }],
    });
    expect(mihomo.groups.validateFilter("(?i)HK")).toBe(true);
    expect(mihomo.groups.validateFilter("(HK)\\1")).toBe(false);
    expect(shadowrocket.groups.validateFilter("HK,hidden=1")).toBe(false);
    expect(shadowrocket.rules.validateComponent("Proxy,DIRECT")).toBe(false);
  });

  it("keeps Shadowrocket settings strict before projecting normalized drafts", () => {
    const adapter = structuredAdapter("shadowrocket");
    const validSettings = {
      groups: [{ name: "Proxy", type: "select", proxies: ["$nodes", "DIRECT"] }],
      rule_sets: [{ name: "ads", type: "domain-set", url: "https://example.com/ads.list" }],
      rules: ["DOMAIN-SET,ads,REJECT", "FINAL,Proxy"],
    };

    const valid = adapter.decode({ settingsPresent: true, settings: validSettings }, "en-US")!;
    expect(valid).toMatchObject({
      settingsMode: "structured",
      mode: "wizard",
      groups: [{ name: "Proxy", type: "select", members: ["$nodes", "DIRECT"] }],
      ruleSets: [{ name: "ads", behavior: "domain-set" }],
      rules: [{ type: "domain-set", value: "ads", policy: "REJECT" }, { type: "final", policy: "Proxy" }],
    });
    expect(adapter.encode(valid)).toEqual({ settings: validSettings });

    const unsupported = { groups: [{ ...validSettings.groups[0], future: true }] };
    expect(adapter.decode({ settingsPresent: true, settings: unsupported }, "en-US")).toMatchObject({
      settingsMode: "raw",
      rawSettings: unsupported,
    });
  });

  it("exports target-neutral editor draft types", () => {
    expectTypeOf<ConfigEditorDraft["groups"]>().toEqualTypeOf<GroupDraft[]>();
    expectTypeOf<ConfigEditorDraft["ruleSets"]>().toEqualTypeOf<RuleSetDraft[]>();
    expectTypeOf<ConfigEditorDraft["rules"]>().toEqualTypeOf<RuleDraft[]>();
    expectTypeOf<GroupDraft>().not.toHaveProperty("tag");
    expectTypeOf<GroupDraft>().not.toHaveProperty("proxies");
    expectTypeOf<GroupDraft>().not.toHaveProperty("outbounds");
  });
});

function structuredAdapter(kind: string) {
  const configuration = FILE_DRIVER_REGISTRY.get(kind)?.configuration;
  if (configuration?.mode !== "structured") throw new Error(`expected structured driver: ${kind}`);
  return configuration.adapter;
}
