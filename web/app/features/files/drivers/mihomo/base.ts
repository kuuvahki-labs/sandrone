const MIHOMO_BASE = `mixed-port: 7890
geo-auto-update: true
geo-update-interval: 24
allow-lan: true
bind-address: "*"
lan-allowed-ips:
  - 10.0.0.0/8
  - 172.16.0.0/12
  - 192.168.0.0/16
  - fc00::/7

mode: rule
log-level: info
ipv6: false
unified-delay: true
tcp-concurrent: true
profile:
  store-selected: true
  store-fake-ip: true

dns:
  enable: true
  ipv6: false
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  fake-ip-filter-mode: blacklist
  fake-ip-filter:
    - "*"
    - geosite:private
    - geosite:connectivity-check
    - "+.lan"
    - "+.local"
    - "+.market.xiaomi.com"
    - "Mijia Cloud"
    - "dig.io.mi.com"
    - "*.icloud.com"
    - "time.*.com"
    - "ntp.*.com"
    - "+.pool.ntp.org"
    - "stun.*.*"
    - "stun.*.*.*"
  default-nameserver:
    - "https://223.5.5.5/dns-query#DIRECT"
    - "https://223.6.6.6/dns-query#DIRECT"
  nameserver-policy:
    "geosite:private":
      - system
    "rule-set:cn":
      - "https://223.5.5.5/dns-query#DIRECT"
      - "https://223.6.6.6/dns-query#DIRECT"
  nameserver:
    - "https://cloudflare-dns.com/dns-query#RULES"
    - "https://dns.google/dns-query#RULES"
  proxy-server-nameserver:
    - "https://223.5.5.5/dns-query#DIRECT"
    - "https://223.6.6.6/dns-query#DIRECT"
  direct-nameserver:
    - "https://223.5.5.5/dns-query#DIRECT"
    - "https://223.6.6.6/dns-query#DIRECT"
  direct-nameserver-follow-policy: false
  respect-rules: true

proxies: []
proxy-groups: []
rule-providers: {}
rules: []`;

export function mihomoDefaultBase(): string {
  return MIHOMO_BASE;
}
