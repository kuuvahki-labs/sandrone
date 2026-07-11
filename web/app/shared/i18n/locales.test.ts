import { describe, expect, it } from "vitest";

import { detectPreferredLocale, isLocale } from "./locales";

describe("locale helpers", () => {
  it("accepts only supported locales", () => {
    expect(isLocale("zh-CN")).toBe(true);
    expect(isLocale("en-US")).toBe(true);
    expect(isLocale("zh-HK")).toBe(false);
    expect(isLocale("fr-FR")).toBe(false);
  });

  it("maps Chinese browser language variants to zh-CN", () => {
    expect(detectPreferredLocale(["zh-HK", "en-US"])).toBe("zh-CN");
    expect(detectPreferredLocale(["zh-TW"])).toBe("zh-CN");
    expect(detectPreferredLocale(["zh"])).toBe("zh-CN");
  });

  it("defaults non-Chinese browser languages to en-US", () => {
    expect(detectPreferredLocale(["en-GB", "zh-CN"])).toBe("en-US");
    expect(detectPreferredLocale(["fr-FR"])).toBe("en-US");
    expect(detectPreferredLocale([])).toBe("en-US");
  });
});
