import { describe, expect, it } from "vitest";

import { FILE_DRIVER_REGISTRY, requireFileDriver } from "./registry";

describe("file driver codecs", () => {
  it("keeps Mihomo remote source URL validation in its driver", () => {
    const driver = FILE_DRIVER_REGISTRY.get("mihomo");

    expect(driver?.source.validate({ type: "remote", remote: { url: "ftp://example.com/base.yaml" } }))
      .toBe("source_remote_url_invalid");
    expect(driver?.source.validate({ type: "inline", content: "mixed-port: 7890" })).toBeNull();
  });

  it("retains the exact missing-driver errors", () => {
    expect(errorMessage(() => requireFileDriver("future-client")))
      .toBe("unregistered file kind: future-client");
    expect(errorMessage(() => requireFileDriver("")))
      .toBe("unregistered file kind: (missing)");
  });

  it("registers the exact four definitions and five creation presets in creation order", () => {
    expect(FILE_DRIVER_REGISTRY.drivers.map((driver) => [driver.kind, driver.configuration.mode])).toEqual([
      ["static", "none"],
      ["mihomo", "structured"],
      ["sing-box", "structured"],
      ["shadowrocket", "structured"],
    ]);
    expect(FILE_DRIVER_REGISTRY.drivers.map((driver) => [driver.kind, driver.targetRendererFormat])).toEqual([
      ["static", undefined],
      ["mihomo", "mihomo-proxies"],
      ["sing-box", "sing-box-outbounds"],
      ["shadowrocket", "shadowrocket-proxies"],
    ]);
    expect(FILE_DRIVER_REGISTRY.get("static")).not.toHaveProperty("targetRendererFormat");
    expect(FILE_DRIVER_REGISTRY.createPresets).toEqual([
      {
        kind: "static",
        source: "local",
        sourceType: "inline",
        order: 0,
        initialName: "",
        icon: "file",
        labelKey: "model.fileSource.local",
        accessibleLabelKey: "files.create.local",
      },
      {
        kind: "static",
        source: "remote",
        sourceType: "remote",
        order: 1,
        initialName: "",
        icon: "remote",
        labelKey: "model.fileSource.remote",
        accessibleLabelKey: "files.create.remote",
      },
      {
        kind: "mihomo",
        source: "mihomo",
        sourceType: "inline",
        order: 2,
        initialName: "mihomo.yaml",
        icon: "mihomo",
        labelKey: "files.kind.mihomo",
        accessibleLabelKey: "files.create.mihomo",
      },
      {
        kind: "sing-box",
        source: "sing-box",
        sourceType: "inline",
        order: 3,
        initialName: "sing-box.json",
        icon: "sing-box",
        labelKey: "files.kind.singBox",
        accessibleLabelKey: "files.create.singBox",
      },
      {
        kind: "shadowrocket",
        source: "shadowrocket",
        sourceType: "inline",
        order: 4,
        initialName: "shadowrocket.conf",
        icon: "rocket",
        labelKey: "files.kind.shadowrocket",
        accessibleLabelKey: "files.create.shadowrocket",
      },
    ]);
    expect(FILE_DRIVER_REGISTRY.get("mihomo")).toBe(FILE_DRIVER_REGISTRY.drivers[1]);
    expect(FILE_DRIVER_REGISTRY.resolveCreatePreset("mihomo")).toBe(FILE_DRIVER_REGISTRY.createPresets[2]);
    expect(FILE_DRIVER_REGISTRY.get("missing")).toBeUndefined();
    expect(Object.isFrozen(FILE_DRIVER_REGISTRY.drivers)).toBe(true);
    expect(Object.isFrozen(FILE_DRIVER_REGISTRY.createPresets)).toBe(true);
    for (const driver of FILE_DRIVER_REGISTRY.drivers) {
      expect(Object.isFrozen(driver)).toBe(true);
      expect(Object.isFrozen(driver.configuration)).toBe(true);
      expect(Object.isFrozen(driver.createPresets)).toBe(true);
      expect(Object.isFrozen(driver.processors.mergeModes)).toBe(true);
    }
    for (const preset of FILE_DRIVER_REGISTRY.createPresets) {
      expect(Object.isFrozen(preset)).toBe(true);
    }
    expect(FILE_DRIVER_REGISTRY.drivers[0]).not.toHaveProperty("preview");
  });

  it("delegates settings decoding and encoding to each registered adapter", () => {
    const mihomo = structuredAdapter("mihomo");
    const singBox = structuredAdapter("sing-box");

    expect(mihomo?.decode({
      subscriptions: ["provider"],
      settingsPresent: true,
      settings: { adaptive_groups: { type: "url-test" }, groups: [], rule_sets: [], rules: [] },
    })).toMatchObject({
      subscriptions: ["provider"],
      settingsMode: "structured",
      adaptiveGroups: { type: "url-test" },
      groups: [],
      ruleSets: [],
      rules: [],
    });
    expect(singBox?.decode({
      settingsPresent: true,
      settings: { groups: [], future_nested: true },
    })).toMatchObject({ settingsMode: "raw", rawSettings: { groups: [], future_nested: true } });
    expect(singBox?.encode(singBox.initialize({
      subscriptions: ["provider"],
      settingsMode: "structured",
      groups: [],
      rule_sets: [],
      rules: [],
    }))).toEqual({
      subscriptions: ["provider"],
      settings: { groups: [], rule_sets: [], rules: [] },
    });
  });

  it.each([
    ["mihomo", {}],
    ["mihomo", { type: "url-test" }],
    ["mihomo", { regions: ["us", "hk"] }],
    ["shadowrocket", {}],
    ["shadowrocket", { type: "url-test" }],
    ["shadowrocket", { regions: ["us", "hk"] }],
  ] as const)("round-trips partial %s adaptive settings without filling or reordering fields", (kind, adaptiveGroups) => {
    const adapter = structuredAdapter(kind);
    const settings = { adaptive_groups: adaptiveGroups };

    const decoded = adapter.decode({ settingsPresent: true, settings }, "en-US");

    expect(decoded).toMatchObject({ settingsMode: "structured", adaptiveGroups });
    expect(adapter.encode(decoded!)).toEqual({ settings });
  });

  it.each([
    { adaptive_groups: { future: true } },
    { adaptive_groups: { type: 7 } },
    { adaptive_groups: { regions: ["hk", 7] } },
  ])("falls back to raw mode for backend-unrepresentable Mihomo adaptive settings %#", (settings) => {
    expect(structuredAdapter("mihomo").decode({ settingsPresent: true, settings })).toMatchObject({
      settingsMode: "raw",
      rawSettings: settings,
    });
  });

  it.each(["mihomo", "shadowrocket"] as const)(
    "falls back to raw mode when %s receives removed minimum_node_count",
    (kind) => {
      const settings = { adaptive_groups: { minimum_node_count: 2 } };

      expect(structuredAdapter(kind).decode({ settingsPresent: true, settings })).toMatchObject({
        settingsMode: "raw",
        rawSettings: settings,
      });
    },
  );

  it("registers Shadowrocket as an INI typed-file target without exposing extra merge modes", () => {
    const driver = FILE_DRIVER_REGISTRY.get("shadowrocket");

    expect(driver).toMatchObject({
      kind: "shadowrocket",
      presentation: { icon: "rocket" },
      configuration: { mode: "structured" },
      source: { syntax: "ini", strategy: "optional-base" },
      processors: { mergeModes: ["ini_override"] },
    });
    expect(driver?.createPresets).toEqual([expect.objectContaining({
      kind: "shadowrocket",
      source: "shadowrocket",
      sourceType: "inline",
      order: 4,
      initialName: "shadowrocket.conf",
    })]);
    expect(structuredAdapter("shadowrocket").catalogTarget).toBe("shadowrocket");
    expect(driver?.source.defaultBase("en-US")).toContain("[General]");
    expect(driver?.source.defaultBase("en-US")).toContain("[Proxy Group]");
    expect(driver?.source.validate({ type: "remote", remote: { url: "ftp://example.com/base.conf" } }))
      .toBe("source_remote_url_invalid");
  });

  it("round-trips strict Shadowrocket settings and preserves omitted versus explicit empty arrays", () => {
    const adapter = structuredAdapter("shadowrocket");
    const settings = {
      adaptive_groups: { type: "url-test", regions: [] },
      groups: [{
        name: "Proxy",
        type: "select",
        proxies: ["$nodes", "DIRECT"],
        hidden: false,
      }],
      rule_sets: [{ name: "ads", type: "domain-set", url: "https://example.com/ads.list" }],
      rules: ["DOMAIN-SET,ads,REJECT", "FINAL,Proxy"],
    };

    const decoded = adapter.decode({ settingsPresent: true, settings });
    expect(decoded).toMatchObject({
      settingsMode: "structured",
      groups: [{ name: "Proxy", members: ["$nodes", "DIRECT"] }],
      ruleSets: [{ name: "ads", behavior: "domain-set" }],
    });
    expect(adapter.encode(decoded!)).toEqual({ settings });
    expect(adapter.encode(adapter.initialize({ settingsMode: "structured", groups: [], rules: [] }))).toEqual({
      settings: { groups: [], rules: [] },
    });
    expect(adapter.encode(adapter.initialize({ settingsMode: "structured" }))).toEqual({ settings: {} });
  });

  it.each([
    ["policy-select-name", "DIRECT"],
    ["select", 0],
  ] as const)("falls back to raw mode for removed Shadowrocket group field %s", (key, value) => {
    const settings = {
      groups: [{ name: "Proxy", type: "select", proxies: ["DIRECT"], [key]: value }],
    };
    expect(structuredAdapter("shadowrocket").decode({ settingsPresent: true, settings })).toMatchObject({
      settingsMode: "raw",
      rawSettings: settings,
    });
  });

  it("round-trips Mihomo hidden as a normalized group field", () => {
    const adapter = structuredAdapter("mihomo");
    const settings = {
      groups: [{ name: "Proxy", type: "select", proxies: ["DIRECT"], hidden: true }],
    };
    const decoded = adapter.decode({ settingsPresent: true, settings })!;
    expect(decoded.groups[0]).toMatchObject({ hidden: true });
    expect(adapter.encode(decoded)).toEqual({ settings });
  });

  it("defaults only new Mihomo url-test groups to tolerance 50", () => {
    const adapter = structuredAdapter("mihomo");
    const transitioned = adapter.groups.transitionType(adapter.groups.create("en-US"), "url-test");
    const generated = adapter.groups.serialize(adapter.groups.defaults("standard", "en-US"));

    expect(adapter.groups.serialize([transitioned])).toEqual([
      expect.objectContaining({ type: "url-test", tolerance: 50 }),
    ]);
    expect(generated.find((group) => group.type === "url-test"))
      .toEqual(expect.objectContaining({ tolerance: 50 }));
  });

  it("does not backfill omitted Mihomo tolerance and preserves explicit values", () => {
    const adapter = structuredAdapter("mihomo");
    const withoutTolerance = {
      name: "Auto",
      type: "url-test",
      proxies: ["$nodes"],
      url: "https://cp.cloudflare.com",
      interval: 300,
    };
    const explicitTolerance = { ...withoutTolerance, tolerance: 75 };

    const withoutDraft = adapter.groups.project([withoutTolerance]);
    const explicitDraft = adapter.groups.project([explicitTolerance]);

    expect(withoutDraft).not.toBeNull();
    expect(withoutDraft?.[0].healthCheckTolerance).toBeUndefined();
    expect(adapter.groups.serialize(withoutDraft!)).toEqual([withoutTolerance]);
    expect(explicitDraft).not.toBeNull();
    expect(explicitDraft?.[0].healthCheckTolerance).toBe(75);
    expect(explicitDraft?.[0].adapterState).not.toHaveProperty("tolerance");
    expect(adapter.groups.serialize(explicitDraft!)).toEqual([explicitTolerance]);
  });

  it.each([
    ["selector", true],
    ["selector", false],
    ["urltest", true],
    ["urltest", false],
  ] as const)(
    "round-trips sing-box %s interrupt_exist_connections=%s as semantic state",
    (nativeType, interruptExistingConnections) => {
      const adapter = structuredAdapter("sing-box");
      const nativeGroup = {
        type: nativeType,
        tag: "Proxy",
        outbounds: ["$nodes", "direct"],
        ...(nativeType === "urltest"
          ? { url: "https://cp.cloudflare.com", interval: "5m" }
          : {}),
        interrupt_exist_connections: interruptExistingConnections,
      };

      const drafts = adapter.groups.project([nativeGroup]);

      expect(drafts).not.toBeNull();
      expect(drafts?.[0]).toMatchObject({ interruptExistingConnections });
      expect(drafts?.[0].adapterState).not.toHaveProperty("interrupt_exist_connections");
      const serialized = adapter.groups.serialize(drafts!);
      expect(serialized).toEqual([nativeGroup]);
      expect(Object.keys(serialized[0])).not.toContain("__sandroneInterruptExistingConnectionsExplicit");
    },
  );

  it("omits unchecked interrupt_exist_connections from new sing-box groups", () => {
    const adapter = structuredAdapter("sing-box");
    const selector = { ...adapter.groups.create("en-US"), interruptExistingConnections: false };
    const urltest = adapter.groups.transitionType(selector, "url-test");

    expect(adapter.groups.serialize([selector])[0]).not.toHaveProperty("interrupt_exist_connections");
    expect(adapter.groups.serialize([urltest])[0]).not.toHaveProperty("interrupt_exist_connections");
  });

  it("accepts trimmed Shadowrocket rule-set types and URLs", () => {
    const adapter = structuredAdapter("shadowrocket");
    const settings = {
      rule_sets: [{ name: "ads", type: " domain-set ", url: " https://example.com/ads.list " }],
    };

    const decoded = adapter.decode({ settingsPresent: true, settings });
    expect(decoded).toMatchObject({ settingsMode: "structured", mode: "advanced" });
    expect(adapter.encode(decoded!)).toEqual({ settings });
  });

  it.each([
    { groups: [{ name: "Proxy", type: "select", proxies: ["$nodes"], future: true }] },
    { groups: [{ name: "Proxy", type: "unsupported", proxies: ["$nodes"] }] },
    { groups: [{ name: " DIRECT ", type: "select", proxies: ["$nodes"] }] },
    { groups: [{ name: " #comment ", type: "select", proxies: ["$nodes"] }] },
    { groups: [{ name: ";comment", type: "select", proxies: ["$nodes"] }] },
    { groups: [{ name: "[section]", type: "select", proxies: ["$nodes"] }] },
    { groups: [{ name: "Proxy", type: "select", proxies: ["DIRECT", "DIRECT"] }] },
    { groups: [{ name: "Proxy", type: "select", proxies: ["DIRECT"], select: true }] },
    { groups: [{ name: "Proxy", type: "select", proxies: ["DIRECT"], select: -1 }] },
    { groups: [{ name: "Proxy", type: "select", "policy-regex-filter": "HK,hidden=1" }] },
    { groups: [{ name: "Proxy", type: "select", "policy-regex-filter": "HK\n[Rule]" }] },
    { rule_sets: [{ name: "ads", type: "domain-set", url: "ftp://example.com/ads.list" }] },
    { rule_sets: [{ name: "ads", type: "domain-set", url: "https://example.com/ads,tracking.list" }] },
    { rules: [{ type: "FINAL", policy: "Proxy" }] },
    { future_nested: true },
  ])("falls back to raw mode for unsupported Shadowrocket settings %#", (settings) => {
    expect(structuredAdapter("shadowrocket").decode({
      settingsPresent: true,
      settings,
    })).toMatchObject({ settingsMode: "raw", rawSettings: settings });
  });
});

function structuredAdapter(kind: string) {
  const configuration = FILE_DRIVER_REGISTRY.get(kind)?.configuration;
  if (configuration?.mode !== "structured") throw new Error(`expected structured file driver: ${kind}`);
  return configuration.adapter;
}

function errorMessage(action: () => unknown): string | undefined {
  try {
    action();
  } catch (error) {
    expect(error).toBeInstanceOf(Error);
    return (error as Error).message;
  }
  return undefined;
}
