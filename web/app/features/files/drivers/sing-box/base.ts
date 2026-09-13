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
      { "type": "local", "tag": "dns-local" },
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
        "domain_regex": ["^[^.]+$"],
        "domain_suffix": ["lan", "local"],
        "action": "route",
        "server": "dns-local"
      },
      {
        "domain": [
          "www.gstatic.com",
          "captive.apple.com",
          "cp.cloudflare.com",
          "www.msftconnecttest.com",
          "connectivitycheck.platform.hicloud.com",
          "Mijia Cloud",
          "dig.io.mi.com",
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
    "strategy": "prefer_ipv4"
  },
  "inbounds": [
    { "type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": 2080 }
  ],
  "outbounds": [],
  "route": {
    "auto_detect_interface": true,
    "default_domain_resolver": "dns-cn",
    "default_http_client": "rule-set-direct",
    "rule_set": [],
    "rules": []
  },
  "experimental": {
    "cache_file": { "enabled": true, "store_fakeip": true }
  }
}`;
}
