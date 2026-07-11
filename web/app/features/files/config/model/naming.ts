import { translate, type TranslationKey } from "~/shared/i18n/context";
import type { Locale } from "~/shared/i18n/locales";

export type ConfigNamingLocale = Locale;

// IDs and approved locale-catalog names are adapted from SubBoost's proxy-group modules (AGPL-3.0), pinned at:
// https://github.com/SubBoost/subboost/blob/ebc40a202adeaca25c88ca3bbbf085412f6e08f5/packages/core/src/generator/proxy-group-modules.ts
export const CONFIG_GROUP_IDS = [
  "select", "auto", "fallback", "other", "ad", "private", "cn", "global", "final", "ai", "youtube", "google",
  "microsoft", "apple", "telegram", "twitter", "meta", "discord", "social-other", "netflix",
  "disney", "streaming-west", "streaming-asia", "steam", "gaming-pc", "gaming-console", "github",
  "cloud", "dev-tools", "storage", "payment", "crypto", "education", "news", "shopping",
] as const;

export type ConfigGroupID = (typeof CONFIG_GROUP_IDS)[number];

export const CONFIG_REGION_IDS = [
  "hk", "tw", "sg", "jp", "kr", "us", "ca", "uk", "de", "fr", "mo", "au", "ru", "th",
  "in", "my", "ph", "tr", "ua", "fi", "ar", "eg",
] as const;

export type ConfigRegionID = (typeof CONFIG_REGION_IDS)[number];

function configGroupNameKey(id: ConfigGroupID): TranslationKey {
  return `files.config.outputNames.group.${id}`;
}

function configRegionNameKey(id: ConfigRegionID): TranslationKey {
  return `files.config.outputNames.region.${id}`;
}

export function configGroupName(id: ConfigGroupID, locale: ConfigNamingLocale): string {
  return translate(locale, configGroupNameKey(id));
}

export function configRegionName(id: ConfigRegionID, locale: ConfigNamingLocale): string {
  return translate(locale, configRegionNameKey(id));
}

export function configAnchorName(locale: ConfigNamingLocale): string {
  return configGroupName("select", locale);
}

export function configAutoName(locale: ConfigNamingLocale): string {
  return configGroupName("auto", locale);
}

export function configCustomGroupName(locale: ConfigNamingLocale): string {
  return translate(locale, "files.config.outputNames.customGroup");
}

const KNOWN_NAMES = {
  "en-US": new Set([
    ...CONFIG_GROUP_IDS.map((id) => configGroupName(id, "en-US")),
    ...CONFIG_REGION_IDS.map((id) => configRegionName(id, "en-US")),
  ]),
  "zh-CN": new Set([
    ...CONFIG_GROUP_IDS.map((id) => configGroupName(id, "zh-CN")),
    ...CONFIG_REGION_IDS.map((id) => configRegionName(id, "zh-CN")),
  ]),
} satisfies Record<ConfigNamingLocale, ReadonlySet<string>>;

export function detectConfigNamingLocale(
	names: readonly string[],
): ConfigNamingLocale {
	const englishAnchorCount = names.filter((name) => name === configAnchorName("en-US")).length;
  const chineseAnchorCount = names.filter((name) => name === configAnchorName("zh-CN")).length;
  if (englishAnchorCount === 1 && chineseAnchorCount === 0) return "en-US";
  if (chineseAnchorCount === 1 && englishAnchorCount === 0) return "zh-CN";

  const englishCount = names.filter((name) => KNOWN_NAMES["en-US"].has(name)).length;
  const chineseCount = names.filter((name) => KNOWN_NAMES["zh-CN"].has(name)).length;
  return chineseCount > englishCount ? "zh-CN" : "en-US";
}

export function configNamesForLocale(locale: ConfigNamingLocale): ReadonlySet<string> {
  return KNOWN_NAMES[locale];
}
