import type { FileConfigDraft } from "~/features/files/model/types";

import {
  type ConfigGroupID,
  type ConfigNamingLocale,
  detectConfigNamingLocale,
} from "./naming";

export type ConfigTemplateID = "minimal" | "standard" | "full";

export interface ConfigTemplateDefinition {
  id: ConfigTemplateID;
  name: string;
  description: string;
  modules: string[];
  groupCount: number;
  ruleSetCount: number;
  ruleCount: number;
}

export type ConfigTemplateMatch = ConfigTemplateID | "custom" | null;

export interface ConfigTemplateRecognition {
  adaptive: boolean;
  match: ConfigTemplateMatch;
  namingLocale: ConfigNamingLocale;
}

export type TemplateModuleGroupMode = "select" | "url-test" | "direct-first" | "reject-first";

export interface ConfigTemplateModule {
  id: ConfigGroupID;
  groupMode: TemplateModuleGroupMode;
  rules: readonly string[];
}

export interface ConfigTemplateRuleEntry {
  module: ConfigTemplateModule;
  ruleID: string;
}

export interface ConfigTemplateBlueprint {
  enabled: ReadonlySet<string>;
  groups: readonly ConfigTemplateModule[];
  namingLocale: ConfigNamingLocale;
  ruleEntries: readonly ConfigTemplateRuleEntry[];
}

export interface ConfigTemplateDialect {
  groupNames: (groups: readonly Record<string, unknown>[]) => string[];
  materialize: (blueprint: Readonly<ConfigTemplateBlueprint>) => FileConfigDraft;
  moduleIDs: (
    id: ConfigTemplateID,
    catalogModuleIDs: readonly string[],
  ) => readonly string[];
  normalizeRecognition: (
    config: Readonly<FileConfigDraft>,
  ) => { adaptive: boolean; config: FileConfigDraft };
}

export interface ConfigTemplateStrategy {
  create: (id: ConfigTemplateID, namingLocale?: ConfigNamingLocale) => FileConfigDraft;
  list: () => ConfigTemplateDefinition[];
  recognize: (config: FileConfigDraft) => ConfigTemplateRecognition;
  resolveNamingLocale: (
    config: FileConfigDraft | undefined,
    fallback: ConfigNamingLocale,
  ) => ConfigNamingLocale;
}

const MODULES: readonly ConfigTemplateModule[] = [
  module("select"), module("auto", "url-test"), module("ad", "reject-first", ["category-ads-all"]),
  module("private", "direct-first", ["private", "private-ip"]), module("cn", "direct-first", ["geolocation-cn", "cn-ip"]),
  module("global", "select", ["geolocation-!cn"]), module("final"), module("ai", "select", ["openai", "anthropic", "category-ai-chat-!cn"]),
  module("youtube", "select", ["youtube"]), module("google", "select", ["google", "google-ip"]), module("microsoft", "select", ["microsoft", "onedrive"]),
  module("apple", "select", ["apple", "icloud"]), module("telegram", "select", ["telegram", "telegram-ip"]), module("twitter", "select", ["twitter", "twitter-ip"]),
  module("meta", "select", ["facebook", "instagram", "whatsapp", "facebook-ip"]), module("discord", "select", ["discord"]),
  module("social-other", "select", ["tiktok", "line", "reddit", "linkedin", "snap", "pinterest", "tumblr"]), module("netflix", "select", ["netflix", "netflix-ip"]),
  module("disney", "select", ["disney"]), module("streaming-west", "select", ["hbo", "hulu", "primevideo", "apple-tvplus", "spotify", "twitch", "dazn"]),
  module("streaming-asia", "select", ["bahamut", "biliintl", "niconico", "abema", "viu", "kktv"]), module("steam", "select", ["steam"]),
  module("gaming-pc", "select", ["epicgames", "ea", "ubisoft", "blizzard", "gog", "riot"]), module("gaming-console", "select", ["playstation", "xbox", "nintendo"]),
  module("github", "select", ["github", "gitlab", "atlassian"]), module("cloud", "select", ["aws", "azure", "cloudflare", "digitalocean", "vercel", "netlify", "cloudflare-ip"]),
  module("dev-tools", "select", ["docker", "npmjs", "jetbrains", "stackexchange"]), module("storage", "select", ["dropbox", "notion"]),
  module("payment", "select", ["paypal", "stripe", "wise"]), module("crypto", "select", ["binance"]),
  module("education", "select", ["category-scholar-!cn", "coursera", "udemy", "edx", "khanacademy", "wikimedia"]),
  module("news", "select", ["bbc", "cnn", "nytimes", "wsj", "bloomberg"]), module("shopping", "select", ["amazon", "ebay"]),
];

const MODULE_BY_ID = new Map(MODULES.map((item) => [item.id, item]));
const MINIMAL_MODULES = ["select", "auto", "ad", "private", "cn", "global", "final"];
const STANDARD_MODULES = [
  "select", "auto", "ad", "private", "cn", "global", "ai", "youtube", "google", "microsoft", "apple", "netflix", "telegram", "final",
];
const TEMPLATE_IDS = ["minimal", "standard", "full"] as const satisfies readonly ConfigTemplateID[];
const TEMPLATE_MODULES: Record<ConfigTemplateID, readonly string[]> = {
  minimal: MINIMAL_MODULES,
  standard: STANDARD_MODULES,
  full: MODULES.map((item) => item.id),
};
const TEMPLATE_METADATA: Record<ConfigTemplateID, { name: string; description: string }> = {
  minimal: { name: "Minimal", description: "Core selection, blocking, private, China, global, and final routing." },
  standard: { name: "Standard", description: "Core routing plus frequently used and region-restricted services." },
  full: { name: "Full", description: "Broad social, media, gaming, cloud, finance, education, news, and shopping coverage." },
};

const GROUP_ORDER: readonly ConfigGroupID[] = [
  "select", "auto", "ad", "ai", "youtube", "google", "microsoft", "apple", "telegram", "twitter", "meta", "discord", "social-other",
  "netflix", "disney", "streaming-west", "streaming-asia", "steam", "gaming-pc", "gaming-console", "github", "cloud", "dev-tools", "storage",
  "payment", "crypto", "education", "news", "shopping", "private", "cn", "global", "final",
];
const RULE_ORDER: readonly ConfigGroupID[] = [
  "ad", "private", "ai", "cn", "youtube", "education", "cloud", "google", "telegram", "github", "microsoft", "apple", "twitter", "meta",
  "discord", "social-other", "netflix", "disney", "streaming-west", "streaming-asia", "steam", "gaming-pc", "gaming-console", "dev-tools",
  "storage", "payment", "crypto", "news", "shopping", "global",
];

export function createConfigTemplateStrategy(
  dialect: Readonly<ConfigTemplateDialect>,
): ConfigTemplateStrategy {
  const create = (
    id: ConfigTemplateID,
    namingLocale: ConfigNamingLocale = "en-US",
  ): FileConfigDraft => {
    const moduleIDs = dialect.moduleIDs(id, TEMPLATE_MODULES[id]);
    const enabled = new Set(moduleIDs);
    const groups = GROUP_ORDER
      .filter((moduleID) => enabled.has(moduleID))
      .map(requiredModule);
    const ruleEntries = RULE_ORDER
      .filter((moduleID) => enabled.has(moduleID))
      .map(requiredModule)
      .flatMap((item) => item.rules.map((ruleID) => ({ module: item, ruleID })));
    return dialect.materialize({ enabled, groups, namingLocale, ruleEntries });
  };
  const recognize = (config: FileConfigDraft): ConfigTemplateRecognition => {
    const normalized = dialect.normalizeRecognition(config);
    for (const namingLocale of ["zh-CN", "en-US"] as const) {
      const candidate = stableConfigJSON(normalized.config);
      for (const id of TEMPLATE_IDS) {
        if (candidate === stableConfigJSON(create(id, namingLocale))) {
          return { adaptive: normalized.adaptive, match: id, namingLocale };
        }
      }
    }
    return {
      adaptive: normalized.adaptive,
      match: "custom",
      namingLocale: detectConfigNamingLocale(dialect.groupNames(config.groups ?? [])),
    };
  };
  const strategy: ConfigTemplateStrategy = {
    create,
    list: () => TEMPLATE_IDS.map((id) => {
      const config = create(id);
      return {
        id,
        ...TEMPLATE_METADATA[id],
        modules: [...dialect.moduleIDs(id, TEMPLATE_MODULES[id])],
        groupCount: config.groups?.length ?? 0,
        ruleSetCount: config.rule_sets?.length ?? 0,
        ruleCount: config.rules?.length ?? 0,
      };
    }),
    recognize,
    resolveNamingLocale: (config, fallback) => config
      ? recognize(config).namingLocale
      : fallback,
  };
  return Object.freeze(strategy);
}

export function templateGroupTargets(
  item: Readonly<ConfigTemplateModule>,
  selectName: string | undefined,
  autoName: string | undefined,
  direct: string,
  reject: string,
): string[] {
  switch (item.groupMode) {
    case "url-test":
      return ["$nodes"];
    case "reject-first":
      return uniqueStrings([reject, direct, selectName ?? ""]);
    case "direct-first":
      return uniqueStrings([direct, selectName ?? "", autoName ?? "", reject]);
    default:
      return item.id === "select"
        ? uniqueStrings([autoName ?? "", "$nodes", direct, reject])
        : uniqueStrings([selectName ?? "", autoName ?? "", direct, reject]);
  }
}

function module(
  id: ConfigGroupID,
  groupMode: TemplateModuleGroupMode = "select",
  rules: readonly string[] = [],
): ConfigTemplateModule {
  return { id, groupMode, rules };
}

function requiredModule(id: ConfigGroupID): ConfigTemplateModule {
  const item = MODULE_BY_ID.get(id);
  if (!item) throw new Error(`Unknown config template module: ${id}`);
  return item;
}

function stableConfigJSON(config: FileConfigDraft): string {
  return JSON.stringify(stableValue(Object.fromEntries(
    Object.entries(config).filter(([key, value]) => (
      key !== "subscriptions" && value !== undefined
    )),
  )));
}

function stableValue(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(stableValue);
  if (!value || typeof value !== "object") return value;
  return Object.fromEntries(Object.entries(value as Record<string, unknown>)
    .filter(([, entry]) => entry !== undefined)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, entry]) => [key, stableValue(entry)]));
}

function uniqueStrings(values: string[]): string[] {
  return [...new Set(values.filter(Boolean))];
}
