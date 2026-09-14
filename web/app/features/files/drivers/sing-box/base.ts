import { configAnchorName, type ConfigNamingLocale } from "~/features/files/config/model/naming";

export function singBoxDefaultBase(namingLocale: ConfigNamingLocale): string {
  const anchor = configAnchorName(namingLocale);
  return `{
  "log": { "level": "info" },
  "http_clients": [
    { "tag": "rule-set-direct" }
  ],
  "dns": {
    "servers": [
      { "type": "local", "tag": "dns-local", "neighbor_domain": [".", ".lan"] },
      { "type": "https", "tag": "dns-cn", "server": "223.5.5.5" },
      { "type": "https", "tag": "dns-remote", "server": "1.1.1.1", "detour": "${anchor}" },
      {
        "type": "fakeip",
        "tag": "dns-fakeip",
        "inet4_range": "198.18.0.0/15",
        "inet6_range": "fc00::/18"
      }
    ],
    "rules": [
      {
        "preferred_by": ["dns-local"],
        "action": "route",
        "server": "dns-local"
      },
      {
        "domain_regex": ["^[^.]+$"],
        "domain_suffix": [
          "lan",
          "localdomain",
          "localhost",
          "local",
          "home.arpa",
          "internal",
          "example",
          "invalid",
          "test"
        ],
        "action": "route",
        "server": "dns-local"
      },
      {
        "domain": [
          "Mijia Cloud",
          "dlg.io.mi.com",
          "localhost.ptlogin2.qq.com",
          "localhost.sec.qq.com"
        ],
        "domain_suffix": ["market.xiaomi.com", "pool.ntp.org"],
        "domain_regex": [
          "^[^.]+\\\\.icloud\\\\.com$",
          "^localhost\\\\.[^.]+\\\\.weixin\\\\.qq\\\\.com$",
          "^time\\\\.[^.]+\\\\.com$",
          "^ntp\\\\.[^.]+\\\\.com$",
          "^stun\\\\.[^.]+\\\\.[^.]+$",
          "^stun\\\\.[^.]+\\\\.[^.]+\\\\.[^.]+$"
        ],
        "action": "route",
        "server": "dns-remote"
      },
      { "query_type": ["A", "AAAA"], "action": "route", "server": "dns-fakeip" },
      { "rule_set": ["cn"], "action": "route", "server": "dns-cn" }
    ],
    "final": "dns-remote",
    "strategy": "ipv4_only"
  },
  "inbounds": [
    { "type": "mixed", "tag": "mixed-in", "listen": "0.0.0.0", "listen_port": 2080 },
    {
      "type": "tun",
      "tag": "tun-in",
      "address": ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
      "auto_route": true,
      "strict_route": true,
      "route_exclude_address": [
        "10.0.0.0/8",
        "172.16.0.0/12",
        "192.168.0.0/16",
        "169.254.0.0/16",
        "fe80::/10",
        "fc00::/7",
        "224.0.0.251/32",
        "ff02::fb/128"
      ]
    }
  ],
  "outbounds": [],
  "route": {
    "auto_detect_interface": true,
    "default_domain_resolver": "dns-cn",
    "default_http_client": "rule-set-direct",
    "rule_set": [],
    "rules": [
      {
        "type": "logical",
        "mode": "and",
        "rules": [
          { "inbound": ["mixed-in"] },
          {
            "source_ip_cidr": ["127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"],
            "invert": true
          }
        ],
        "action": "reject"
      },
      { "action": "sniff" },
      {
        "type": "logical",
        "mode": "or",
        "rules": [
          { "domain_regex": ["^[^.]+$"] },
          {
            "domain_suffix": [
              "lan",
              "localdomain",
              "localhost",
              "local",
              "home.arpa",
              "internal",
              "example",
              "invalid",
              "test"
            ]
          }
        ],
        "action": "resolve",
        "server": "dns-local",
        "strategy": "ipv4_only"
      },
      {
        "type": "logical",
        "mode": "or",
        "rules": [{ "protocol": "dns" }, { "port": 53 }],
        "action": "hijack-dns"
      },
      { "clash_mode": "direct", "outbound": "direct" },
      { "clash_mode": "global", "outbound": "${anchor}" }
    ]
  },
  "experimental": {
    "cache_file": { "enabled": true, "store_fakeip": true },
    "clash_api": { "external_controller": "127.0.0.1:9090" }
  }
}`;
}
