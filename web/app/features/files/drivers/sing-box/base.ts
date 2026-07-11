import { configAnchorName, type ConfigNamingLocale } from "~/features/files/config/model/naming";

export function singBoxDefaultBase(namingLocale: ConfigNamingLocale): string {
  const anchor = configAnchorName(namingLocale);
  return `{
  "log": { "level": "info" },
  "dns": {
    "servers": [
      { "type": "local", "tag": "dns-local" },
      { "type": "https", "tag": "dns-remote", "server": "1.1.1.1", "detour": "${anchor}" }
    ],
    "final": "dns-remote",
    "strategy": "prefer_ipv4"
  },
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "address": ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
      "auto_route": true,
      "route_exclude_address": [
        "10.0.0.0/8",
        "172.16.0.0/12",
        "192.168.0.0/16",
        "169.254.0.0/16",
        "fe80::/10",
        "fc00::/7"
      ]
    },
    {
      "type": "mixed",
      "tag": "mixed-in",
      "listen": "127.0.0.1",
      "listen_port": 2080
    }
  ],
  "outbounds": [],
  "route": {
    "auto_detect_interface": true,
    "default_domain_resolver": "dns-local",
    "final": "${anchor}",
    "rule_set": [],
    "rules": []
  },
  "experimental": {
    "cache_file": { "enabled": true }
  }
}`;
}
