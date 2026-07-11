import { describe, expect, it, vi } from "vitest";

import { zhCN } from "~/shared/i18n/translations/zh-CN";

import {
  configGroupName,
  configRegionName,
  detectConfigNamingLocale,
} from "./naming";

describe("file config materialized naming", () => {
  it("resolves names from the shared locale catalog", () => {
    const key = "files.config.outputNames.group.select";
    const catalog = zhCN as unknown as Record<typeof key, string>;
    const original = catalog[key];
    catalog[key] = "临时节点选择";

    try {
      expect(configGroupName("select", "zh-CN")).toBe("临时节点选择");
    } finally {
      catalog[key] = original;
    }
  });

  it("provides the approved Chinese module and region names", () => {
    expect(configGroupName("select", "zh-CN")).toBe("🚀 节点选择");
    expect(configGroupName("auto", "zh-CN")).toBe("⚡ 自动选择");
    expect(configGroupName("fallback", "zh-CN")).toBe("故障转移");
    expect(configGroupName("other", "zh-CN")).toBe("其他");
    expect(configGroupName("final", "zh-CN")).toBe("🐟 漏网之鱼");
    expect(configGroupName("twitter", "zh-CN")).toBe("🐦 推特/X");
    expect(configRegionName("hk", "zh-CN")).toBe("🇭🇰 香港");
    expect(configRegionName("eg", "zh-CN")).toBe("🇪🇬 埃及");
  });

  it("keeps English names unchanged", () => {
    expect(configGroupName("select", "en-US")).toBe("Proxy");
    expect(configGroupName("streaming-west", "en-US")).toBe("Western Streaming");
    expect(configRegionName("us", "en-US")).toBe("United States");
  });

  it("prefers a unique semantic anchor when detecting existing content", () => {
    expect(detectConfigNamingLocale(["🚀 节点选择", "Custom English Name"])).toBe("zh-CN");
    expect(detectConfigNamingLocale(["Proxy", "自定义名称"])).toBe("en-US");
  });

  it("uses known-name counts and falls back to English for ties or unknown content", () => {
    expect(detectConfigNamingLocale(["⚡ 自动选择", "🇭🇰 香港"])).toBe("zh-CN");
    expect(detectConfigNamingLocale(["Auto", "🇭🇰 香港"])).toBe("en-US");
    expect(detectConfigNamingLocale(["Personal"])).toBe("en-US");
  });

  it("resolves canonical adaptive region names from the shared locale catalog", async () => {
    vi.resetModules();
    const { enUS } = await import("~/shared/i18n/translations/en-US");
    const key = "files.config.outputNames.region.hk";
    const catalog = enUS as Record<typeof key, string>;
    const original = catalog[key];
    catalog[key] = "Temporary Hong Kong";

    try {
      const { ADAPTIVE_REGION_GROUPS } = await import("./adaptive-regions");
      expect(ADAPTIVE_REGION_GROUPS.find((region) => region.id === "hk")?.name).toBe("Temporary Hong Kong");
    } finally {
      catalog[key] = original;
      vi.resetModules();
    }
  });
});
