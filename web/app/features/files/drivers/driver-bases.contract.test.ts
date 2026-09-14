import { load } from "js-yaml";
import { describe, expect, it } from "vitest";

import type { FileConfigDraft } from "~/features/files/model/types";

import type { ConfigTemplateID } from "../config/model/templates";
import { requireFileDriver } from "./registry";

const TEMPLATE_IDS = ["minimal", "standard", "full"] as const satisfies readonly ConfigTemplateID[];
const MIHOMO_ONLY_TEMPLATE_GROUPS = new Set(["Fallback"]);
const MIHOMO_ONLY_GROUP_MEMBERS = new Set(["REJECT-DROP"]);

interface NormalizedTemplateRule {
  type: string;
  value?: string;
  policy?: string;
}

describe("file driver default bases", () => {
  it("enables the approved Mihomo base for Web creation", () => {
    const base = driverBase("mihomo");
    const parsed = load(base) as Record<string, unknown>;

    expect(parsed).toMatchObject({
      "mixed-port": 7890,
      "external-controller": "127.0.0.1:9090",
      "geo-auto-update": true,
      "geo-update-interval": 24,
      "allow-lan": true,
      "bind-address": "*",
      "lan-allowed-ips": ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"],
      mode: "rule",
      "log-level": "info",
      ipv6: false,
      "unified-delay": true,
      "tcp-concurrent": true,
      "disable-keep-alive": true,
      profile: { "store-selected": true, "store-fake-ip": true },
      sniffer: {
        enable: true,
        "override-destination": false,
        "skip-domain": ["Mijia Cloud", "dlg.io.mi.com", "+.push.apple.com", "+.akadns.net"],
        sniff: {
          HTTP: { ports: [80, 8080, 8880] },
          TLS: { ports: [443, 8443] },
          QUIC: { ports: [443, 8443] },
        },
      },
      tun: {
        enable: true,
        stack: "mixed",
        "auto-route": true,
        "strict-route": true,
        "auto-detect-interface": true,
        "dns-hijack": ["any:53", "tcp://any:53"],
        "route-exclude-address": [
          "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16",
          "17.249.0.0/16", "17.252.0.0/16", "17.57.144.0/22", "17.188.128.0/18",
          "17.188.20.0/23", "fe80::/10", "fc00::/7", "2620:149:a44::/48",
          "2403:300:a42::/48", "2403:300:a51::/48", "2a01:b740:a42::/48",
          "224.0.0.251/32", "ff02::fb/128",
        ],
      },
      dns: {
        enable: true,
        ipv6: false,
        "prefer-h3": true,
        "enhanced-mode": "fake-ip",
        "fake-ip-filter": [
          "*",
          "geosite:private",
          "geosite:connectivity-check",
          "+.lan",
          "+.local",
          "+.market.xiaomi.com",
          "Mijia Cloud",
          "dlg.io.mi.com",
          "localhost.ptlogin2.qq.com",
          "localhost.sec.qq.com",
          "localhost.*.weixin.qq.com",
          "*.icloud.com",
          "+.push.apple.com",
          "+.akadns.net",
          "time.*.com",
          "ntp.*.com",
          "+.pool.ntp.org",
          "stun.*.*",
          "stun.*.*.*",
        ],
        "default-nameserver": [
          "https://223.5.5.5/dns-query#DIRECT",
          "https://223.6.6.6/dns-query#DIRECT",
        ],
        "nameserver-policy": {
          "geosite:private": ["system"],
          "+.push.apple.com": ["system"],
          "+.akadns.net": ["system"],
          "rule-set:cn": [
            "https://223.5.5.5/dns-query#DIRECT",
            "https://223.6.6.6/dns-query#DIRECT",
          ],
        },
        nameserver: [
          "https://cloudflare-dns.com/dns-query",
          "https://dns.google/dns-query",
        ],
        "proxy-server-nameserver": [
          "https://223.5.5.5/dns-query#DIRECT",
          "https://223.6.6.6/dns-query#DIRECT",
        ],
        "direct-nameserver": [
          "https://223.5.5.5/dns-query#DIRECT",
          "https://223.6.6.6/dns-query#DIRECT",
        ],
      },
      proxies: [],
      "proxy-groups": [],
      "rule-providers": {},
      rules: [],
    });
    expect(parsed).not.toHaveProperty("secret");
    expect(parsed).not.toHaveProperty("auto-redirect");
    expect(parsed).not.toHaveProperty("geox-url");
    expect(parsed).not.toHaveProperty("dns.fake-ip-range");
    expect(parsed).not.toHaveProperty("dns.fake-ip-filter-mode");
    expect(parsed).not.toHaveProperty("dns.direct-nameserver-follow-policy");
    expect(base).not.toContain("tailscale");
  });

  it("exposes the explicit sing-box base used by new files", () => {
    const base = driverBase("sing-box");

    expect(JSON.parse(base)).toEqual({
      log: { level: "info" },
      http_clients: [{ tag: "rule-set-direct" }],
      dns: {
        servers: [
          { type: "local", tag: "dns-local", neighbor_domain: [".", ".lan"] },
          { type: "https", tag: "dns-cn", server: "223.5.5.5" },
          { type: "https", tag: "dns-remote", server: "1.1.1.1", detour: "Proxy" },
          {
            type: "fakeip",
            tag: "dns-fakeip",
            inet4_range: "198.18.0.0/15",
            inet6_range: "fc00::/18",
          },
        ],
        rules: [
          {
            preferred_by: ["dns-local"],
            action: "route",
            server: "dns-local",
          },
          {
            domain_regex: ["^[^.]+$"],
            domain_suffix: [
              "lan",
              "localdomain",
              "localhost",
              "local",
              "home.arpa",
              "internal",
              "example",
              "invalid",
              "test",
            ],
            action: "route",
            server: "dns-local",
          },
          {
            domain_suffix: ["push.apple.com", "akadns.net"],
            action: "route",
            server: "dns-local",
          },
          {
            domain: [
              "Mijia Cloud",
              "dlg.io.mi.com",
              "localhost.ptlogin2.qq.com",
              "localhost.sec.qq.com",
            ],
            domain_suffix: ["market.xiaomi.com", "pool.ntp.org"],
            domain_regex: [
              "^[^.]+\\.icloud\\.com$",
              "^localhost\\.[^.]+\\.weixin\\.qq\\.com$",
              "^time\\.[^.]+\\.com$",
              "^ntp\\.[^.]+\\.com$",
              "^stun\\.[^.]+\\.[^.]+$",
              "^stun\\.[^.]+\\.[^.]+\\.[^.]+$",
            ],
            action: "route",
            server: "dns-remote",
          },
          { query_type: ["A", "AAAA"], action: "route", server: "dns-fakeip" },
          { rule_set: ["cn"], action: "route", server: "dns-cn" },
        ],
        final: "dns-remote",
        strategy: "ipv4_only",
      },
      inbounds: [
        { type: "mixed", tag: "mixed-in", listen: "0.0.0.0", listen_port: 2080 },
        {
          type: "tun",
          tag: "tun-in",
          address: ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
          auto_route: true,
          strict_route: true,
          route_exclude_address: [
            "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16",
            "17.249.0.0/16", "17.252.0.0/16", "17.57.144.0/22", "17.188.128.0/18",
            "17.188.20.0/23", "fe80::/10", "fc00::/7", "2620:149:a44::/48",
            "2403:300:a42::/48", "2403:300:a51::/48", "2a01:b740:a42::/48",
            "224.0.0.251/32", "ff02::fb/128",
          ],
        },
      ],
      outbounds: [],
      route: {
        auto_detect_interface: true,
        default_domain_resolver: "dns-cn",
        default_http_client: "rule-set-direct",
        rule_set: [],
        rules: [
          {
            type: "logical",
            mode: "and",
            rules: [
              { inbound: ["mixed-in"] },
              {
                source_ip_cidr: ["127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"],
                invert: true,
              },
            ],
            action: "reject",
          },
          { action: "sniff" },
          {
            type: "logical",
            mode: "or",
            rules: [
              { domain_regex: ["^[^.]+$"] },
              {
                domain_suffix: [
                  "lan",
                  "localdomain",
                  "localhost",
                  "local",
                  "home.arpa",
                  "internal",
                  "example",
                  "invalid",
                  "test",
                ],
              },
            ],
            action: "resolve",
            server: "dns-local",
            strategy: "ipv4_only",
          },
          {
            type: "logical",
            mode: "or",
            rules: [{ protocol: "dns" }, { port: 53 }],
            action: "hijack-dns",
          },
          {
            domain_suffix: ["push.apple.com", "akadns.net"],
            outbound: "direct",
          },
          { clash_mode: "direct", outbound: "direct" },
          { clash_mode: "global", outbound: "Proxy" },
        ],
      },
      experimental: {
        cache_file: { enabled: true, store_fakeip: true },
        clash_api: { external_controller: "127.0.0.1:9090" },
      },
    });
    expect(base).not.toMatch(/auto_redirect|"stack"|"path"|cache_id/);
    expect(JSON.parse(base).dns.rules.findIndex((rule: Record<string, unknown>) =>
      Array.isArray(rule.domain_suffix) && rule.domain_suffix.includes("push.apple.com"))).toBeLessThan(
      JSON.parse(base).dns.rules.findIndex((rule: Record<string, unknown>) =>
        rule.server === "dns-fakeip"),
    );
    expect(JSON.parse(base).dns.rules.findIndex((rule: Record<string, unknown>) =>
      rule.server === "dns-fakeip")).toBeLessThan(
      JSON.parse(base).dns.rules.findIndex((rule: Record<string, unknown>) =>
        rule.server === "dns-cn"),
    );
  });

  it("keeps localized base references on the registered sing-box driver", () => {
    const base = JSON.parse(driverBase("sing-box", "zh-CN")) as {
      dns: { servers: Array<{ detour?: string }> };
      route: { final?: string; rules: Array<Record<string, unknown>> };
    };

    expect(base.dns.servers[2]?.detour).toBe("🚀 节点选择");
    expect(base.route.rules.at(-1)).toEqual({ clash_mode: "global", outbound: "🚀 节点选择" });
    expect(base.route).not.toHaveProperty("final");
  });

  it("keeps Mihomo and sing-box default capabilities aligned", () => {
    const mihomo = asRecord(load(driverBase("mihomo")));
    const singBox = asRecord(JSON.parse(driverBase("sing-box")));

    expect(mihomoDefaultCapabilities(mihomo)).toEqual(singBoxDefaultCapabilities(singBox));
    expect(singBoxDefaultCapabilities(singBox)).toEqual({
      cacheFakeIP: true,
      controlAPI: true,
      dnsHijack: true,
      fakeIPv4: true,
      ipv4OnlyByDefault: true,
      lanMixedProxy: true,
      localDNS: true,
      modeRules: true,
      tun: true,
    });
  });

  it.each(TEMPLATE_IDS)("keeps the %s Mihomo and sing-box template semantics aligned", (templateID) => {
    expect(normalizeTemplate("mihomo", templateID)).toEqual(normalizeTemplate("sing-box", templateID));
  });

  it("uses the approved portable Shadowrocket Web base", () => {
    const base = driverBase("shadowrocket");

    expect(base.match(/^\[General\]$/gmu)).toHaveLength(1);
    expect(base).not.toMatch(/^\s*bypass-system\s*=/mu);
    expect(base).toContain("dns-server = https://223.5.5.5/dns-query,https://223.6.6.6/dns-query");
    expect(base).toContain("fallback-dns-server = https://1.1.1.1/dns-query#proxy,https://8.8.8.8/dns-query#proxy");
    expect(base).toContain("proxy-dns-server = https://223.5.5.5/dns-query,https://223.6.6.6/dns-query");
    expect(base).toContain("hijack-dns = :53");
    expect(base).not.toMatch(/^dns-server\s*=.*(?:^|,)\s*(?:223\.5\.5\.5|119\.29\.29\.29)(?:,|$)/mu);
    expect(base).not.toContain("fallback-dns-server = system");
    expect(base).toMatch(/^dns-direct-fallback-proxy = false$/mu);
    expect(base).toMatch(/^close-if-proxy-chain-missing = true$/mu);
    expect(base).toMatch(/^udp-policy-not-supported-behaviour = REJECT$/mu);
    expect(base).toMatch(/^block-quic = all-proxy$/mu);
    expect(base).toMatch(/^ipv6 = true$/mu);
    expect(base).toMatch(/^prefer-ipv6 = false$/mu);
    expect(base).toMatch(/^\[Proxy\]$[\s\S]*^\[Proxy Group\]$[\s\S]*^\[Rule\]$[\s\S]*^\[Host\]$/mu);
    expect(base).toContain("*.apple.com = server:system");
    expect(base).toContain("*.icloud.com = server:system");
    expect(base).toContain("localhost = 127.0.0.1");
    expect(base).toContain("# always-real-ip =");
    expect(base).toMatch(/^\[Host\]$/mu);
    expect(base).not.toMatch(/^\s*always-real-ip\s*=/mu);
  });
});

function driverBase(kind: string, locale: "en-US" | "zh-CN" = "en-US"): string {
  return requireFileDriver(kind).source.defaultBase(locale);
}

function mihomoDefaultCapabilities(base: Record<string, unknown>) {
  const dns = asRecord(base.dns);
  const tun = asRecord(base.tun);
  return {
    cacheFakeIP: asRecord(base.profile)["store-fake-ip"] === true,
    controlAPI: base["external-controller"] === "127.0.0.1:9090",
    dnsHijack: stringList(tun["dns-hijack"]).includes("any:53"),
    fakeIPv4: dns["enhanced-mode"] === "fake-ip",
    ipv4OnlyByDefault: base.ipv6 === false && dns.ipv6 === false,
    lanMixedProxy: base["allow-lan"] === true
      && stringList(base["lan-allowed-ips"]).slice(0, 3).join(",") === "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16",
    localDNS: stringList(dns["fake-ip-filter"]).includes("geosite:private"),
    modeRules: base.mode === "rule",
    tun: tun.enable === true,
  };
}

function singBoxDefaultCapabilities(base: Record<string, unknown>) {
  const dns = asRecord(base.dns);
  const dnsServers = recordList(dns.servers);
  const inbounds = recordList(base.inbounds);
  const route = asRecord(base.route);
  const routeRules = recordList(route.rules);
  const mixed = inbounds.find((inbound) => inbound.tag === "mixed-in") ?? {};
  const tun = inbounds.find((inbound) => inbound.tag === "tun-in") ?? {};
  const local = dnsServers.find((server) => server.tag === "dns-local") ?? {};
  const fakeIP = dnsServers.find((server) => server.tag === "dns-fakeip") ?? {};
  return {
    cacheFakeIP: asRecord(asRecord(base.experimental).cache_file).store_fakeip === true,
    controlAPI: asRecord(asRecord(base.experimental).clash_api).external_controller === "127.0.0.1:9090",
    dnsHijack: routeRules.some((rule) => rule.action === "hijack-dns"),
    fakeIPv4: fakeIP.type === "fakeip" && fakeIP.inet4_range === "198.18.0.0/15",
    ipv4OnlyByDefault: dns.strategy === "ipv4_only",
    lanMixedProxy: mixed.listen === "0.0.0.0" && routeRules.some((rule) => rule.action === "reject"),
    localDNS: stringList(local.neighbor_domain).join(",") === ".,.lan"
      && recordList(dns.rules).some((rule) => stringList(rule.preferred_by).includes("dns-local")),
    modeRules: routeRules.some((rule) => rule.clash_mode === "direct")
      && routeRules.some((rule) => rule.clash_mode === "global"),
    tun: tun.auto_route === true && tun.strict_route === true,
  };
}

function normalizeTemplate(kind: "mihomo" | "sing-box", templateID: ConfigTemplateID) {
  const config = createTemplate(kind, templateID);
  const groups = recordList(config.groups)
    .filter((group) => !MIHOMO_ONLY_TEMPLATE_GROUPS.has(String(group.name ?? group.tag)))
    .map((group) => ({
      id: String(group.name ?? group.tag),
      intervalSeconds: group.interval === "5m" ? 300 : group.interval,
      members: stringList(group.proxies ?? group.outbounds)
        .filter((member) => !MIHOMO_ONLY_TEMPLATE_GROUPS.has(member) && !MIHOMO_ONLY_GROUP_MEMBERS.has(member))
        .map(normalizePolicy),
      probeURL: group.url,
      tolerance: group.tolerance,
      type: group.type === "urltest" ? "url-test" : group.type === "selector" ? "select" : group.type,
    }));
  const ruleSets = recordList(config.rule_sets).map((ruleSet) => ({
    family: String(ruleSet.url).includes("/geoip/") ? "ip" : "domain",
    id: String(ruleSet.name ?? ruleSet.tag),
  }));
  const rules = (config.rules ?? []).flatMap((rule) => normalizeTemplateRule(kind, rule));
  return { groups, ruleSets, rules };
}

function normalizeTemplateRule(kind: "mihomo" | "sing-box", rule: unknown): NormalizedTemplateRule[] {
  if (kind === "mihomo") {
    const [type, value, policy] = String(rule).split(",");
    return type === "MATCH"
      ? [{ type: "final", value: undefined, policy: normalizePolicy(value) }]
      : [{ type: type.toLowerCase(), value, policy: normalizePolicy(policy) }];
  }
  const value = asRecord(rule);
  if (value.action === "resolve" && Object.keys(value).length === 1) return [];
  const ruleSet = stringList(value.rule_set)[0];
  return [{
    type: ruleSet ? "rule-set" : value.port === 853 ? "dst-port" : "final",
    value: ruleSet ?? (value.port === 853 ? "853" : undefined),
    policy: normalizePolicy(String(value.outbound)),
  }];
}

function normalizePolicy(value: string | undefined): string | undefined {
  if (value === "DIRECT") return "direct";
  if (value === "REJECT") return "block";
  return value;
}

function createTemplate(kind: "mihomo" | "sing-box", templateID: ConfigTemplateID): FileConfigDraft {
  const driver = requireFileDriver(kind);
  if (driver.configuration.mode !== "structured") throw new Error(`${kind} is not structured`);
  return driver.configuration.adapter.templates.create(templateID);
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function recordList(value: unknown): Record<string, unknown>[] {
  return Array.isArray(value) ? value.map(asRecord) : [];
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
}
