import { load } from "js-yaml";
import { describe, expect, it } from "vitest";

import { requireFileDriver } from "./registry";

describe("file driver default bases", () => {
  it("enables the approved Mihomo base for Web creation", () => {
    const base = driverBase("mihomo");
    const parsed = load(base) as Record<string, unknown>;

    expect(parsed).toMatchObject({
      "mixed-port": 7890,
      "allow-lan": true,
      "bind-address": "*",
      "lan-allowed-ips": ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"],
      mode: "rule",
      "log-level": "info",
      ipv6: false,
      "unified-delay": true,
      "tcp-concurrent": true,
      profile: { "store-selected": true, "store-fake-ip": true },
      dns: {
        enable: true,
        ipv6: false,
        "enhanced-mode": "fake-ip",
        "fake-ip-range": "198.18.0.1/16",
        "fake-ip-filter-mode": "blacklist",
        "fake-ip-filter": [
          "*",
          "geosite:private",
          "geosite:connectivity-check",
          "+.lan",
          "+.local",
          "+.market.xiaomi.com",
          "Mijia Cloud",
          "dig.io.mi.com",
          "*.icloud.com",
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
          "rule-set:cn": [
            "https://223.5.5.5/dns-query#DIRECT",
            "https://223.6.6.6/dns-query#DIRECT",
          ],
        },
        nameserver: [
          "https://cloudflare-dns.com/dns-query#RULES",
          "https://dns.google/dns-query#RULES",
        ],
        "proxy-server-nameserver": [
          "https://223.5.5.5/dns-query#DIRECT",
          "https://223.6.6.6/dns-query#DIRECT",
        ],
        "direct-nameserver": [
          "https://223.5.5.5/dns-query#DIRECT",
          "https://223.6.6.6/dns-query#DIRECT",
        ],
        "direct-nameserver-follow-policy": false,
        "respect-rules": true,
      },
      proxies: [],
      "proxy-groups": [],
      "rule-providers": {},
      rules: [],
    });
    expect(parsed).not.toHaveProperty("external-controller");
    expect(parsed).not.toHaveProperty("secret");
    expect(parsed).not.toHaveProperty("auto-redirect");
    expect(parsed).not.toHaveProperty("sniffer");
    expect(parsed).not.toHaveProperty("tun");
    expect(base).not.toContain("tailscale");
    expect(base).not.toContain("100.64.0.0/10");
  });

  it("exposes the explicit sing-box base used by new files", () => {
    const base = driverBase("sing-box");

    expect(JSON.parse(base)).toEqual({
      log: { level: "info" },
      dns: {
        servers: [
          { type: "local", tag: "dns-local" },
          { type: "https", tag: "dns-cn", server: "223.5.5.5", detour: "direct" },
          { type: "https", tag: "dns-remote", server: "1.1.1.1", detour: "Proxy" },
        ],
        rules: [
          { rule_set: ["private"], action: "route", server: "dns-local" },
          { rule_set: ["cn"], action: "route", server: "dns-cn" },
        ],
        final: "dns-remote",
        strategy: "prefer_ipv4",
      },
      inbounds: [
        {
          type: "tun",
          tag: "tun-in",
          address: ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
          auto_route: true,
          strict_route: true,
          route_exclude_address: [
            "10.0.0.0/8",
            "172.16.0.0/12",
            "192.168.0.0/16",
            "169.254.0.0/16",
            "fe80::/10",
            "fc00::/7",
            "224.0.0.251/32",
            "ff02::fb/128",
          ],
        },
        { type: "mixed", tag: "mixed-in", listen: "127.0.0.1", listen_port: 2080 },
      ],
      outbounds: [],
      route: {
        auto_detect_interface: true,
        default_domain_resolver: "dns-cn",
        final: "Proxy",
        rule_set: [],
        rules: [],
      },
      experimental: { cache_file: { enabled: true } },
    });
    expect(base).not.toMatch(/auto_redirect|"stack"|"path"|cache_id|fakeip|clash_api/);
  });

  it("keeps localized base references on the registered sing-box driver", () => {
    const base = JSON.parse(driverBase("sing-box", "zh-CN")) as {
      dns: { servers: Array<{ detour?: string }> };
      route: { final?: string };
    };

    expect(base.dns.servers[2]?.detour).toBe("🚀 节点选择");
    expect(base.route.final).toBe("🚀 节点选择");
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
    expect(base).toContain("block-quic = all-proxy");
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
