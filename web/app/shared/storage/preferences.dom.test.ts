import { afterEach, describe, expect, it } from "vitest";

import {
  clearAdminToken,
  getAdminToken,
  getLocaleModePreference,
  getLocalePreference,
  getPublicBaseUrl,
  loadThemePreference,
  saveAdminToken,
  saveLocalePreference,
  savePublicBaseUrl,
  saveThemePreference,
} from "./preferences";

describe("browser storage helpers", () => {
  afterEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute("data-theme-mode");
    document.documentElement.removeAttribute("data-theme-preset");
  });

  it("trims and persists the admin token", () => {
    saveAdminToken("  secret-token  ");

    expect(getAdminToken()).toBe("secret-token");

    clearAdminToken();
    expect(getAdminToken()).toBe("");
  });

  it("normalizes the public base URL", () => {
    savePublicBaseUrl(" https://example.com/subs/ ");

    expect(getPublicBaseUrl()).toBe("https://example.com/subs");
  });

  it("defaults to the approved dark Ocean theme", () => {
    expect(loadThemePreference()).toEqual({ mode: "dark", preset: "ocean" });
  });

  it("falls back to Ocean when stored theme data is invalid", () => {
    localStorage.setItem("sandrone.theme", JSON.stringify({ mode: "dark", preset: "unknown" }));

    expect(loadThemePreference()).toEqual({ mode: "dark", preset: "ocean" });
  });

  it("applies saved theme data to the document element", () => {
    saveThemePreference({ mode: "dark", preset: "ocean" });

    expect(document.documentElement).toHaveAttribute("data-theme-mode", "dark");
    expect(document.documentElement).toHaveAttribute("data-theme-preset", "ocean");
    expect(loadThemePreference()).toEqual({ mode: "dark", preset: "ocean" });
  });

  it("notifies the app when the theme preference changes", () => {
    const events: unknown[] = [];
    window.addEventListener("sandrone:theme-preference-change", (event) => events.push((event as CustomEvent).detail), { once: true });

    saveThemePreference({ mode: "light", preset: "ocean" });

    expect(events).toEqual([{ mode: "light", preset: "ocean" }]);
  });

  it("persists a supported locale preference", () => {
    saveLocalePreference("en-US");

    expect(localStorage.getItem("sandrone.locale")).toBe("en-US");
    expect(getLocalePreference()).toBe("en-US");
  });

  it("uses auto as the non-authoritative locale cache default", () => {
    expect(getLocaleModePreference()).toBe("auto");

    saveLocalePreference("auto");

    expect(getLocaleModePreference()).toBe("auto");
  });

  it("falls back to browser locale when stored locale data is invalid", () => {
    localStorage.setItem("sandrone.locale", "fr-FR");

    expect(getLocalePreference(["zh-HK"])).toBe("zh-CN");
  });
});
