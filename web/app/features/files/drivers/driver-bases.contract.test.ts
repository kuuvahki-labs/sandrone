import { load } from "js-yaml";
import { describe, expect, it } from "vitest";

import { requireFileDriver } from "./registry";

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
        "skip-domain": ["Mijia Cloud", "dlg.io.mi.com", "+.push.apple.com"],
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
          "fe80::/10", "fc00::/7", "224.0.0.251/32", "ff02::fb/128",
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
          "dig.io.mi.com",
          "localhost.ptlogin2.qq.com",
          "localhost.sec.qq.com",
          "localhost.*.weixin.qq.com",
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
          { type: "local", tag: "dns-local" },
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
            domain_regex: ["^[^.]+$"],
            domain_suffix: ["lan", "local"],
            action: "route",
            server: "dns-local",
          },
          {
            domain: [
              "www.gstatic.com",
              "captive.apple.com",
              "cp.cloudflare.com",
              "www.msftconnecttest.com",
              "connectivitycheck.platform.hicloud.com",
              "Mijia Cloud",
              "dig.io.mi.com",
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
        strategy: "prefer_ipv4",
      },
      inbounds: [
        { type: "mixed", tag: "mixed-in", listen: "127.0.0.1", listen_port: 2080 },
        {
          type: "tun",
          tag: "tun-in",
          address: ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
          auto_route: true,
          strict_route: true,
          platform: {
            http_proxy: { enabled: true, server: "127.0.0.1", server_port: 2080 },
          },
          route_exclude_address: [
            "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16",
            "fe80::/10", "fc00::/7", "224.0.0.251/32", "ff02::fb/128",
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
          { action: "sniff" },
          {
            type: "logical",
            mode: "or",
            rules: [{ protocol: "dns" }, { port: 53 }],
            action: "hijack-dns",
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
    expect(base.route.rules[3]).toEqual({ clash_mode: "global", outbound: "🚀 节点选择" });
    expect(base.route).not.toHaveProperty("final");
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
