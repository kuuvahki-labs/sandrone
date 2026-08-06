import { describe, expect, it } from "vitest";

import { requireFileDriver } from "~/features/files/drivers/registry";
import type { FileConfigDraft } from "~/features/files/model/types";

import type { AdaptiveGroupGeneration, AdaptiveGroupOptions } from "./adaptive-groups";
import type { ConfigNamingLocale } from "./naming";
import type { ConfigTemplateID } from "./templates";

const CONFIG_KINDS = ["mihomo", "sing-box", "shadowrocket"] as const;
type ConfigKind = typeof CONFIG_KINDS[number];
const TEMPLATE_IDS = ["minimal", "standard", "full"] as const satisfies readonly ConfigTemplateID[];
const SHADOWROCKET_RULE_BASE = "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket";
const SHADOWROCKET_TEMPLATE_ARTIFACTS = new Set([
  "Abema", "Amazon", "AmazonPrimeVideo", "Anthropic", "Apple", "AppleTV", "Atlassian", "BBC",
  "Bahamut", "BiliBili", "BiliBiliIntl", "Binance", "Blizzard", "Bloomberg", "CNN", "China", "Cloudflare", "DAZN",
  "DigitalOcean", "Discord", "Disney", "Docker", "Dropbox", "EA", "Epic", "Facebook", "GitHub",
  "GitLab", "Global", "Gog", "Google", "HBO", "Hulu", "Instagram", "Jetbrains", "KKTV", "Lan", "Line",
  "LinkedIn", "Microsoft", "NYTimes", "Netflix", "Niconico", "Nintendo", "Notion", "Npmjs", "OneDrive",
  "OpenAI", "PayPal", "Pinterest", "PlayStation", "Reddit", "Riot", "Scholar", "Snap", "Spotify",
  "Stackexchange", "Steam", "SteamCN", "Stripe", "Telegram", "TikTok", "Tumblr", "Twitch", "Twitter", "Ubisoft", "Vercel",
  "ViuTV", "Whatsapp", "Wikimedia", "Xbox", "YouTube", "eBay", "iCloud",
]);

const MINIMAL_MODULES = ["select", "auto", "ad", "private", "cn", "global", "final"];
const STANDARD_MODULES = [
  ...MINIMAL_MODULES.slice(0, -1),
  "ai",
  "youtube",
  "google",
  "microsoft",
  "apple",
  "netflix",
  "telegram",
  "final",
];
const FULL_ONLY_MODULES = [
  "bilibili",
  "twitter",
  "meta",
  "discord",
  "social-other",
  "disney",
  "streaming-west",
  "streaming-asia",
  "steam",
  "gaming-pc",
  "gaming-console",
  "github",
  "cloud",
  "dev-tools",
  "storage",
  "payment",
  "crypto",
  "education",
  "news",
  "shopping",
];

describe("config templates", () => {
  it.each(CONFIG_KINDS)("exposes the module tiers with computed %s counts", (kind) => {
    const templates = getConfigTemplates(kind);
    const expectedMinimal = kind === "shadowrocket"
      ? MINIMAL_MODULES.filter((moduleID) => moduleID !== "auto" && moduleID !== "ad")
      : MINIMAL_MODULES;
    const expectedStandard = kind === "shadowrocket"
      ? STANDARD_MODULES.filter((moduleID) => moduleID !== "auto" && moduleID !== "ad")
      : STANDARD_MODULES;

    expect(templates.map((template) => template.id)).toEqual(TEMPLATE_IDS);
    expect(templates[0].modules).toEqual(expectedMinimal);
    expect(templates[1].modules).toEqual(expectedStandard);
    expect(templates[2].modules).toEqual(expect.arrayContaining(FULL_ONLY_MODULES));
    expect(templates[2].modules).not.toEqual(expect.arrayContaining(["adult", "gemini", "google-scholar"]));
    expect(templates[2].modules.includes("auto")).toBe(kind !== "shadowrocket");
    if (kind === "shadowrocket") {
      expect(templates.every((template) => !template.modules.includes("ad"))).toBe(true);
      expect(templates.map((template) => template.groupCount)).toEqual([5, 12, 32]);
    }

    for (const template of templates) {
      const config = createConfigFromTemplate(kind, template.id);
      expect(template.name).not.toBe("");
      expect(template.description).not.toBe("");
      expect(template.groupCount).toBe(config.groups?.length);
      expect(template.ruleSetCount).toBe(config.rule_sets?.length);
      expect(template.ruleCount).toBe(config.rules?.length);
   }
 });

  it.each(CONFIG_KINDS)("creates explicit, isolated %s configs for every template", (kind) => {
    for (const templateID of TEMPLATE_IDS) {
      const first = createConfigFromTemplate(kind, templateID);
      const second = createConfigFromTemplate(kind, templateID);

      expect(first).toMatchObject({
        group_preset: "basic",
        ruleset_preset: "default",
        groups: expect.any(Array),
        rule_sets: expect.any(Array),
        rules: expect.any(Array),
     });
      expect(first).not.toHaveProperty("subscriptions");
      expect(first).not.toBe(second);
      expect(first.groups).not.toBe(second.groups);
      expect(first.rule_sets).not.toBe(second.rule_sets);
      expect(first.rules).not.toBe(second.rules);
   }
 });

  it.each(["mihomo", "sing-box"] as const)("uses the Cloudflare probe URL in the %s standard template", (kind) => {
    const config = createConfigFromTemplate(kind, "standard");
    const autoGroup = config.groups?.find((group) =>
      group.type === (kind === "mihomo" ? "url-test" : "urltest"));

    expect(autoGroup?.url).toBe("https://cp.cloudflare.com");
  });

  it.each(CONFIG_KINDS)("defaults Microsoft and Apple service groups to direct in %s templates", (kind) => {
    for (const templateID of ["standard", "full"] satisfies ConfigTemplateID[]) {
      const groups = createConfigFromTemplate(kind, templateID).groups ?? [];
      const memberKey = kind === "sing-box" ? "outbounds" : "proxies";
      const direct = kind === "sing-box" ? "direct" : "DIRECT";

      for (const name of ["Microsoft", "Apple"]) {
        const group = groups.find((item) => item.name === name || item.tag === name);
        const members = group?.[memberKey];
        expect(Array.isArray(members) ? members[0] : undefined).toBe(direct);
      }
    }
  });

  it.each(CONFIG_KINDS)("adds a direct-first Bilibili group only to the %s full template", (kind) => {
    const standard = createConfigFromTemplate(kind, "standard");
    const full = createConfigFromTemplate(kind, "full");
    const memberKey = kind === "sing-box" ? "outbounds" : "proxies";
    const direct = kind === "sing-box" ? "direct" : "DIRECT";
    const groupName = (group: Record<string, unknown>): string => String(group.name ?? group.tag ?? "");
    const fullGroup = full.groups?.find((group) => groupName(group) === "Bilibili");
    const members = fullGroup?.[memberKey];

    expect(standard.groups?.some((group) => groupName(group) === "Bilibili")).toBe(false);
    expect(Array.isArray(members) ? members[0] : undefined).toBe(direct);
    expect(full.rule_sets).toEqual(expect.arrayContaining([
      expect.objectContaining(kind === "sing-box" ? { tag: "bilibili" } : { name: "bilibili" }),
    ]));
    expect(full.rules).toEqual(expect.arrayContaining([
      kind === "sing-box"
        ? expect.objectContaining({ rule_set: ["bilibili"], outbound: "Bilibili" })
        : "RULE-SET,bilibili,Bilibili",
    ]));
  });

  it("uses MetaCubeX MRS rule providers for Mihomo", () => {
    const config = createConfigFromTemplate("mihomo", "full");

    expect(config.rule_sets).not.toHaveLength(0);
    for (const ruleSet of config.rule_sets ?? []) {
      expect(ruleSet).toMatchObject({
        name: expect.any(String),
        type: "http",
        behavior: expect.stringMatching(/^(domain|ipcidr)$/),
        format: "mrs",
        interval: 86400,
        url: expect.stringMatching(
          /^https:\/\/raw\.githubusercontent\.com\/MetaCubeX\/meta-rules-dat\/meta\/geo\/(?:geosite|geoip)\/.+\.mrs$/,
        ),
     });
   }
 });

  it("uses equivalent MetaCubeX binary SRS rule sets for sing-box", () => {
    const config = createConfigFromTemplate("sing-box", "full");

    expect(config.rule_sets).not.toHaveLength(0);
    for (const ruleSet of config.rule_sets ?? []) {
      expect(ruleSet).toEqual({
        type: "remote",
        tag: expect.any(String),
        format: "binary",
        update_interval: "1d",
        url: expect.stringMatching(
          /^https:\/\/raw\.githubusercontent\.com\/MetaCubeX\/meta-rules-dat\/sing\/geo\/(?:geosite|geoip)\/.+\.srs$/,
        ),
     });
   }
 });

  it.each(TEMPLATE_IDS)("uses the complete China domain set in every %s template", (templateID) => {
    const mihomo = createConfigFromTemplate("mihomo", templateID);
    expect(mihomo.rule_sets).toEqual(expect.arrayContaining([
      expect.objectContaining({
        name: "cn",
        behavior: "domain",
        url: expect.stringMatching(/\/geosite\/cn\.mrs$/),
      }),
      expect.objectContaining({
        name: "cn-ip",
        behavior: "ipcidr",
        url: expect.stringMatching(/\/geoip\/cn\.mrs$/),
      }),
    ]));
    expect(mihomo.rule_sets?.some((ruleSet) => ruleSet.name === "geolocation-cn")).toBe(false);

    const singBox = createConfigFromTemplate("sing-box", templateID);
    expect(singBox.rule_sets).toEqual(expect.arrayContaining([
      expect.objectContaining({ tag: "cn", url: expect.stringMatching(/\/geosite\/cn\.srs$/) }),
      expect.objectContaining({ tag: "cn-ip", url: expect.stringMatching(/\/geoip\/cn\.srs$/) }),
    ]));
    expect(singBox.rule_sets?.some((ruleSet) => ruleSet.tag === "geolocation-cn")).toBe(false);

    const shadowrocket = createConfigFromTemplate("shadowrocket", templateID);
    expect(shadowrocket.rule_sets).toEqual(expect.arrayContaining([
      {
        name: "cn-domain",
        type: "domain-set",
        url: `${SHADOWROCKET_RULE_BASE}/China/China_Domain.list`,
      },
      {
        name: "cn",
        type: "rule-set",
        url: `${SHADOWROCKET_RULE_BASE}/China/China.list`,
      },
    ]));
    expect(shadowrocket.rules).toEqual(expect.arrayContaining([
      "DOMAIN-SET,cn-domain,China",
      "RULE-SET,cn,China",
    ]));
  });

  it.each(TEMPLATE_IDS)("keeps China IP matching as a resolving fallback in every %s Mihomo template", (templateID) => {
    const rules = createConfigFromTemplate("mihomo", templateID).rules ?? [];

    expect(rules).toContain("RULE-SET,cn-ip,China");
    expect(rules).not.toContain("RULE-SET,cn-ip,China,no-resolve");
    expect(rules).toContain("RULE-SET,private-ip,Private,no-resolve");
  });

  it.each(["standard", "full"] satisfies ConfigTemplateID[])("keeps dedicated service IP rules non-resolving in the %s Mihomo template", (templateID) => {
    expect(createConfigFromTemplate("mihomo", templateID).rules).toContain(
      "RULE-SET,telegram-ip,Telegram,no-resolve",
    );
  });

  it.each(CONFIG_KINDS)("places broad China routing after specific services in the %s standard template", (kind) => {
    const config = createConfigFromTemplate(kind, "standard");
    const rules = config.rules ?? [];
    const policy = (rule: Record<string, unknown>): string => kind === "sing-box"
      ? String(rule.outbound ?? "")
      : String(rule).split(",")[2] ?? "";
    const ruleSetID = (rule: Record<string, unknown>): string => kind === "sing-box"
      ? String(Array.isArray(rule.rule_set) ? rule.rule_set[0] : "")
      : String(rule).split(",")[1] ?? "";
    const firstPolicy = (name: string): number => rules.findIndex((rule) =>
      policy(rule as Record<string, unknown>) === name);
    const broadChina = rules.findIndex((rule) =>
      ruleSetID(rule as Record<string, unknown>) === "cn");

    expect(firstPolicy("YouTube")).toBeGreaterThanOrEqual(0);
    expect(broadChina).toBeGreaterThan(firstPolicy("YouTube"));
    expect(firstPolicy("Global")).toBeGreaterThan(broadChina);
  });

  it.each(["mihomo", "sing-box"] as const)("places domestic service exceptions before broad %s service rules", (kind) => {
    const standard = createConfigFromTemplate(kind, "standard");
    const full = createConfigFromTemplate(kind, "full");
    const ruleSetID = (rule: unknown): string => {
      if (kind !== "sing-box") return String(rule).split(",")[1] ?? "";
      const value = (rule as Record<string, unknown>).rule_set;
      return String(Array.isArray(value) ? value[0] : "");
    };
    const indexOf = (config: FileConfigDraft, id: string): number =>
      (config.rules ?? []).findIndex((rule) => ruleSetID(rule) === id);

    expect(indexOf(standard, "category-ads-all")).toBeLessThan(indexOf(standard, "microsoft@cn"));
    expect(indexOf(standard, "private-ip")).toBeLessThan(indexOf(standard, "microsoft@cn"));
    expect(indexOf(standard, "microsoft@cn")).toBeLessThan(indexOf(standard, "microsoft"));
    expect(indexOf(standard, "apple-cn")).toBeLessThan(indexOf(standard, "apple"));
    expect(indexOf(standard, "steam@cn")).toBe(-1);
    expect(indexOf(standard, "category-games@cn")).toBe(-1);
    expect(indexOf(full, "steam@cn")).toBeLessThan(indexOf(full, "steam"));
    expect(indexOf(full, "category-games@cn")).toBeLessThan(indexOf(full, "epicgames"));
  });

  it("uses the available SteamCN exception before Steam in the full Shadowrocket template", () => {
    const config = createConfigFromTemplate("shadowrocket", "full");
    const rules = config.rules as string[];
    const steamCN = rules.indexOf("RULE-SET,steam@cn,China");
    const steam = rules.indexOf("RULE-SET,steam,Steam");

    expect(config.rule_sets).toEqual(expect.arrayContaining([
      {
        name: "steam@cn",
        type: "rule-set",
        url: `${SHADOWROCKET_RULE_BASE}/SteamCN/SteamCN.list`,
      },
    ]));
    expect(steamCN).toBeGreaterThanOrEqual(0);
    expect(steamCN).toBeLessThan(steam);
  });

  it.each(TEMPLATE_IDS)("uses live Blackmatrix rule lists for the %s Shadowrocket template", (templateID) => {
    const config = createConfigFromTemplate("shadowrocket", templateID);
    const ruleTypesByName = new Map((config.rule_sets ?? []).map((ruleSet) => [ruleSet.name, ruleSet.type]));
    const groupsByName = new Map((config.groups ?? []).map((group) => [group.name, group]));

    expect(config.groups).not.toHaveLength(0);
    expect(groupsByName.get("Proxy")).toEqual({
      name: "Proxy",
      type: "select",
      proxies: ["PROXY", "$nodes", "DIRECT", "REJECT"],
   });
    expect(groupsByName.has("Auto")).toBe(false);
    for (const group of config.groups ?? []) {
      if (Array.isArray(group.proxies)) expect(group.proxies).not.toContain("Auto");
   }
    expect(config.rules?.at(-1)).toBe("FINAL,Final");
    for (const ruleSet of config.rule_sets ?? []) {
      expect(ruleSet).toEqual({
        name: expect.any(String),
        type: expect.stringMatching(/^(rule-set|domain-set)$/),
        url: expect.stringMatching(
          /^https:\/\/raw\.githubusercontent\.com\/blackmatrix7\/ios_rule_script\/master\/rule\/Shadowrocket\//,
        ),
     });
      expect(String(ruleSet.url)).not.toContain("MetaCubeX");
   }
    for (const rule of config.rules ?? []) {
      if (typeof rule !== "string") continue;
      const [type, name] = rule.split(",");
      if (type !== "RULE-SET" && type !== "DOMAIN-SET") continue;
      expect(type).toBe(ruleTypesByName.get(name) === "domain-set" ? "DOMAIN-SET" : "RULE-SET");
   }
 });

  it.each(TEMPLATE_IDS)("omits ad blocking from the %s Shadowrocket template", (templateID) => {
    const config = createConfigFromTemplate("shadowrocket", templateID);

    expect(config.groups?.map((group) => group.name)).not.toContain("Ad Block");
    expect(config.rule_sets?.map((ruleSet) => ruleSet.name)).not.toContain("category-ads-all");
    expect(config.rules?.filter((rule): rule is string => typeof rule === "string")
      .some((rule) => rule.includes("category-ads-all") || rule.includes(",Ad Block"))).toBe(false);
  });

  it.each(TEMPLATE_IDS)("uses each fixed Blackmatrix catalog URL once in the %s Shadowrocket template", (templateID) => {
    const config = createConfigFromTemplate("shadowrocket", templateID);
    const urls = (config.rule_sets ?? []).map((ruleSet) => String(ruleSet.url));
    const artifacts: string[] = [];

    expect(new Set(urls).size).toBe(urls.length);
    for (const url of urls) {
      const match = url.match(new RegExp(`^${SHADOWROCKET_RULE_BASE}/([^/]+)/\\1(?:_Domain)?\\.list$`));
      expect(match?.[1]).toEqual(expect.any(String));
      expect(SHADOWROCKET_TEMPLATE_ARTIFACTS.has(match?.[1] ?? "")).toBe(true);
      if (match?.[1]) artifacts.push(match[1]);
   }
    if (templateID === "full") expect(new Set(artifacts)).toEqual(SHADOWROCKET_TEMPLATE_ARTIFACTS);
 });

  it.each(TEMPLATE_IDS)("never assigns one Blackmatrix URL to different policies in the %s Shadowrocket template", (templateID) => {
    const config = createConfigFromTemplate("shadowrocket", templateID);
    const urlsByRuleSet = new Map((config.rule_sets ?? []).map((ruleSet) => [String(ruleSet.name), String(ruleSet.url)]));
    const policiesByURL = new Map<string, Set<string>>();

    for (const rule of config.rules ?? []) {
      if (typeof rule !== "string") continue;
      const [type, ruleSetName, policy] = rule.split(",");
      if (type !== "RULE-SET" && type !== "DOMAIN-SET") continue;
      const url = urlsByRuleSet.get(ruleSetName);
      if (!url) throw new Error(`missing URL for ${ruleSetName}`);
      const policies = policiesByURL.get(url) ?? new Set<string>();
      policies.add(policy);
      policiesByURL.set(url, policies);
   }

    expect([...policiesByURL.entries()].filter(([, policies]) => policies.size > 1)).toEqual([]);
    if (templateID === "full") {
      expect(policiesByURL.get(`${SHADOWROCKET_RULE_BASE}/Amazon/Amazon.list`)).toEqual(new Set(["Shopping"]));
      expect(policiesByURL.get(`${SHADOWROCKET_RULE_BASE}/Microsoft/Microsoft.list`)).toEqual(new Set(["Microsoft"]));
   }
 });

  it.each(CONFIG_KINDS)("keeps every generated %s group, rule-set, and policy reference closed", (kind) => {
    for (const templateID of TEMPLATE_IDS) {
      expect(referenceProblems(kind, createConfigFromTemplate(kind, templateID))).toEqual([]);
   }
 });

  it.each(CONFIG_KINDS)("materializes approved Chinese names throughout %s templates", (kind) => {
    const config = createConfigFromTemplate(kind, "full", "zh-CN");
    const groupKey = kind === "sing-box" ? "tag" : "name";
    const names = config.groups?.map((group) => group[groupKey]);

    expect(names).toEqual(expect.arrayContaining([
      "🚀 节点选择",
      ...(kind === "shadowrocket" ? [] : ["⚡ 自动选择"]),
      ...(kind === "shadowrocket" ? [] : ["🛑 广告拦截"]),
      "🐦 推特/X",
      "🐟 漏网之鱼",
    ]));
    if (kind === "shadowrocket") expect(names).not.toContain("🛑 广告拦截");
    expect(referenceProblems(kind, config)).toEqual([]);
    const ruleSetNames = config.rule_sets?.map((ruleSet) => ruleSet[kind === "sing-box" ? "tag" : "name"]);
    if (kind === "shadowrocket") {
      expect(ruleSetNames).not.toContain("category-ads-all");
    } else {
      expect(ruleSetNames).toContain("category-ads-all");
    }
    expect(recognizeConfigTemplate(kind, config)).toEqual({
      adaptive: false,
      match: "full",
      namingLocale: "zh-CN",
   });
 });

  it.each(CONFIG_KINDS)("matches exact %s templates while ignoring subscriptions", (kind) => {
    for (const templateID of TEMPLATE_IDS) {
      const config = createConfigFromTemplate(kind, templateID);
      const withSubscriptions = { ...config, subscriptions: ["primary", "backup"] };

      expect(recognizeConfigTemplate(kind, config).match).toBe(templateID);
      expect(recognizeConfigTemplate(kind, withSubscriptions).match).toBe(templateID);
   }
 });

  it.each(CONFIG_KINDS)("recognizes structural changes to a %s template as custom", (kind) => {
    const config = createConfigFromTemplate(kind, "standard");
    const groups = structuredClone(config.groups ?? []);
    groups[0] = { ...groups[0], unexpected: true };

    expect(recognizeConfigTemplate(kind, { ...config, groups }).match).toBe("custom");
 });

  it("does not migrate a legacy Shadowrocket Auto template during recognition", () => {
    const config = createConfigFromTemplate("shadowrocket", "minimal");
    const groups = structuredClone(config.groups ?? []);
    const proxy = groups.find((group) => group.name === "Proxy");
    if (!proxy) throw new Error("expected Proxy group");
    proxy.proxies = ["Auto", "$nodes", "DIRECT", "REJECT"];
    groups.splice(1, 0, {
      name: "Auto",
      type: "url-test",
      proxies: ["$nodes"],
      interval: 300,
      timeout: 5,
      tolerance: 50,
   });

    expect(recognizeConfigTemplate("shadowrocket", { ...config, groups }).match).toBe("custom");
 });

  it.each(CONFIG_KINDS)("keeps the detected Chinese naming language for customized %s configs", (kind) => {
    const config = createConfigFromTemplate(kind, "minimal", "zh-CN");
    const groups = structuredClone(config.groups ?? []);
    groups[0] = { ...groups[0], custom: true };

    expect(recognizeConfigTemplate(kind, { ...config, groups })).toMatchObject({
      match: "custom",
      namingLocale: "zh-CN",
   });
 });

  it.each(TEMPLATE_IDS)("recognizes the %s Mihomo template beneath a canonical adaptive layer", (templateID) => {
    const config = createConfigFromTemplate("mihomo", templateID);
    const merged = mergeAdaptiveGroups(
      config,
      generateAdaptiveGroups(["HK-01"], { type: "url-test" }),
    );

    expect(recognizeConfigTemplate("mihomo", merged.config)).toEqual({
      adaptive: true,
      match: templateID,
      namingLocale: "en-US",
   });
    expect(recognizeConfigTemplate("mihomo", merged.config).match).toBe(templateID);
 });

  it.each(TEMPLATE_IDS)("recognizes the %s Shadowrocket template beneath a canonical adaptive layer", (templateID) => {
    const config = createConfigFromTemplate("shadowrocket", templateID);
    const merged = mergeAdaptiveGroups(
      config,
      generateAdaptiveGroups(
        ["HK-01"],
        { type: "url-test" },
        "shadowrocket",
      ),
      "shadowrocket",
    );

    expect(recognizeConfigTemplate("shadowrocket", merged.config)).toEqual({
      adaptive: true,
      match: templateID,
      namingLocale: "en-US",
   });
 });

  it("keeps a custom adaptive-like group classified as custom", () => {
    const config = createConfigFromTemplate("mihomo", "minimal");
    const merged = mergeAdaptiveGroups(
      config,
      generateAdaptiveGroups(["HK-01"], { type: "select" }),
    );
    const customized = {
      ...merged.config,
      groups: merged.config.groups?.map((group) => group.name === "Hong Kong"
        ? { ...group, icon: "custom" }
        : group),
   };

    expect(stripCanonicalAdaptiveGroups(customized).changed).toBe(false);
    expect(recognizeConfigTemplate("mihomo", customized)).toEqual({ adaptive: false, match: "custom", namingLocale: "en-US" });
 });

  it("detects adaptive groups but keeps invalid anchor structures custom", () => {
    const config = createConfigFromTemplate("mihomo", "minimal");
    const merged = mergeAdaptiveGroups(
      config,
      generateAdaptiveGroups(["HK-01"], { type: "select" }),
    );
    const duplicateProxy = {
      ...merged.config,
      groups: [...(merged.config.groups ?? []), { name: "Proxy", type: "select", proxies: ["$nodes"] }],
   };

    expect(recognizeConfigTemplate("mihomo", duplicateProxy)).toEqual({ adaptive: true, match: "custom", namingLocale: "en-US" });
 });

  it("detects adaptive groups but keeps externally referenced layers custom", () => {
    const config = createConfigFromTemplate("mihomo", "minimal");
    const merged = mergeAdaptiveGroups(
      config,
      generateAdaptiveGroups(["HK-01"], { type: "select" }),
    );
    const referenced = {
      ...merged.config,
      groups: [...(merged.config.groups ?? []), {
        name: "Dependent",
        type: "select",
        proxies: ["Hong Kong"],
     }],
   };

    expect(recognizeConfigTemplate("mihomo", referenced)).toEqual({ adaptive: true, match: "custom", namingLocale: "en-US" });
 });

  it("detects duplicate canonical groups but keeps the conflict custom", () => {
    const config = createConfigFromTemplate("mihomo", "minimal");
    const merged = mergeAdaptiveGroups(
      config,
      generateAdaptiveGroups(["HK-01"], { type: "select" }),
    );
    const canonical = merged.config.groups?.find((group) => group.name === "Hong Kong");
    if (!canonical) throw new Error("expected generated canonical group");
    const duplicated = {
      ...merged.config,
      groups: [...(merged.config.groups ?? []), structuredClone(canonical)],
   };

    expect(recognizeConfigTemplate("mihomo", duplicated)).toEqual({ adaptive: true, match: "custom", namingLocale: "en-US" });
 });

  it("does not offer config templates for static files", () => {
    expect(requireFileDriver("static").configuration.mode).toBe("none");
 });
});

function referenceProblems(kind: ConfigKind, config: FileConfigDraft): string[] {
  const groups = config.groups ?? [];
  const ruleSets = config.rule_sets ?? [];
  const rules = config.rules ?? [];
  const groupNameKey = kind === "sing-box" ? "tag" : "name";
  const groupTargetKey = kind === "sing-box" ? "outbounds" : "proxies";
  const ruleSetNameKey = kind === "sing-box" ? "tag" : "name";
  const literals = new Set(kind === "sing-box"
    ? ["direct", "block", "$nodes"]
    : kind === "shadowrocket"
      ? ["PROXY", "DIRECT", "REJECT", "$nodes"]
      : ["DIRECT", "REJECT", "$nodes"]);
  const groupNames = new Set(groups.map((group) => String(group[groupNameKey])));
  const ruleSetNames = new Set(ruleSets.map((ruleSet) => String(ruleSet[ruleSetNameKey])));
  const problems: string[] = [];

  for (const group of groups) {
    for (const target of stringList(group[groupTargetKey])) {
      if (!groupNames.has(target) && !literals.has(target)) problems.push(`unknown group target: ${target}`);
   }
 }

  for (const rule of rules) {
    if (kind === "sing-box") {
      if (!isRecord(rule)) {
        problems.push("non-object sing-box rule");
        continue;
     }
      for (const ruleSet of stringList(rule.rule_set)) {
        if (!ruleSetNames.has(ruleSet)) problems.push(`unknown rule-set: ${ruleSet}`);
     }
      const policy = stringValue(rule.outbound);
      if (!groupNames.has(policy) && !literals.has(policy)) problems.push(`unknown policy: ${policy}`);
      continue;
   }

    if (typeof rule !== "string") {
      problems.push("non-string INI/YAML rule");
      continue;
   }
    const parts = rule.split(",").map((part) => part.trim());
    if ((parts[0] === "RULE-SET" || parts[0] === "DOMAIN-SET") && !ruleSetNames.has(parts[1])) {
      problems.push(`unknown rule-set: ${parts[1]}`);
   }
    const policy = parts[0] === "MATCH" || parts[0] === "FINAL" ? parts[1] : parts[2];
    if (!groupNames.has(policy) && !literals.has(policy)) problems.push(`unknown policy: ${policy}`);
 }

  return problems;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function createConfigFromTemplate(
  kind: ConfigKind,
  templateID: ConfigTemplateID,
  namingLocale?: ConfigNamingLocale,
): FileConfigDraft {
  return structuredAdapter(kind).templates.create(templateID, namingLocale);
}

function getConfigTemplates(kind: ConfigKind) {
  return structuredAdapter(kind).templates.list();
}

function recognizeConfigTemplate(kind: ConfigKind, config: FileConfigDraft) {
  return structuredAdapter(kind).templates.recognize(config);
}

function generateAdaptiveGroups(
  nodeNames: readonly string[],
  options: Readonly<AdaptiveGroupOptions>,
  kind: ConfigKind = "mihomo",
  namingLocale?: ConfigNamingLocale,
) {
  return structuredAdapter(kind).adaptive.generate(nodeNames, options, namingLocale);
}

function mergeAdaptiveGroups(
  config: Readonly<FileConfigDraft>,
  generation: Readonly<AdaptiveGroupGeneration>,
  kind: ConfigKind = "mihomo",
) {
  return structuredAdapter(kind).adaptive.merge(config, generation);
}

function stripCanonicalAdaptiveGroups(
  config: Readonly<FileConfigDraft>,
  kind: ConfigKind = "mihomo",
) {
  return structuredAdapter(kind).adaptive.strip(config);
}

function structuredAdapter(kind: ConfigKind) {
  const driver = requireFileDriver(kind);
  if (driver.configuration.mode !== "structured") throw new Error(`${kind} is not structured`);
  return driver.configuration.adapter;
}
