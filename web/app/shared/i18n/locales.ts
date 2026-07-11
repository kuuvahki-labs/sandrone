export const supportedLocales = ["zh-CN", "en-US"] as const;

export type Locale = (typeof supportedLocales)[number];

export const fallbackLocale: Locale = "en-US";

export function isLocale(value: unknown): value is Locale {
  return value === "zh-CN" || value === "en-US";
}

export function detectPreferredLocale(languages: readonly string[] = browserLanguages()): Locale {
  const first = languages[0]?.toLowerCase() ?? "";
  return first === "zh" || first.startsWith("zh-") ? "zh-CN" : fallbackLocale;
}

function browserLanguages(): readonly string[] {
  if (typeof navigator === "undefined") {
    return [];
  }
  if (navigator.languages.length > 0) {
    return navigator.languages;
  }
  return navigator.language ? [navigator.language] : [];
}
