# Mihomo fake-IP 默认与边界

本页记录 Sandrone Web 新建 Mihomo 文件时使用的 fake-IP 配置、过滤列表和相关 processor 边界。它不是通用 Mihomo 推荐配置，也不是需要持续扩大的域名清单。

遇到具体解析或连接故障时，按 [排查 Mihomo fake-IP 问题](../how-to/troubleshoot-mihomo-fake-ip.md) 操作；本页不重复修改、重载和验证步骤。

## 适用范围

下文默认值只用于 **Web 新建** Mihomo 文件：

- 新建表单把 Web driver 的 inline base 写入 `FileSpec.source`；
- 新文件同时获得 `Sniffer`、`TUN` 两个 file-stage processors，顺序固定为先 Sniffer、后 TUN；
- 编辑已有文件时沿用已保存的 source 和 processors，不自动回填此默认；
- 通过 API、CLI 或嵌入式 service 创建文件时，若调用方未提交同一 source，不应假定会得到 Web 默认。服务端 Mihomo driver 的内置空基线不是本页所列配置。

因此，判断某个文件是否采用这些值时，应查看它保存的 source 和最终渲染结果，而不是仅依据 `kind: mihomo`。

## Web 新建 base

Web 默认开启 Mihomo DNS 的 fake-IP 模式：

```yaml
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
```

列表及顺序是当前 Web source 的完整默认。Tailscale 域名和地址不在其中。

同一 base 还设置：

```yaml
dns:
  default-nameserver:
    - 223.5.5.5
    - 1.1.1.1
  nameserver:
    - https://dns.alidns.com/dns-query
    - https://cloudflare-dns.com/dns-query
  proxy-server-nameserver:
    - https://dns.alidns.com/dns-query
    - https://cloudflare-dns.com/dns-query
  respect-rules: true
```

这些 resolver 设置参与实际 DNS 路径，但不改变 `fake-ip-filter` 的匹配语义。

## `fake-ip-filter` 的效果

当前 mode 是 `blacklist`。匹配过滤项的域名不获得 fake-IP 映射，而走真实解析；未匹配的域名可以从 `fake-ip-range` 获得 fake IP。Mihomo 官方说明见 [DNS 配置](https://wiki.metacubex.one/en/config/dns/)。

该结果只回答“DNS 返回真实 IP 还是 fake IP”，不等价于：

- 路由规则选择 `DIRECT`；
- 指定某个 `nameserver` 或 `nameserver-policy`；
- 绕过 TUN；
- 允许某个来源访问 LAN 入站；
- 修复局域网、mDNS、split DNS 或应用自身的 resolver 行为。

域名获得真实 IP 后，后续连接仍按 Mihomo 路由、代理组、TUN 和系统网络配置处理。反过来，若 DNS 查询经过 Mihomo，而后续连接绕过同一 fake-IP 映射链路，单纯扩大过滤列表也可能只是掩盖路径错误。

`fake-ip-filter-mode: rule` 使用另一套自上而下的规则语法；本页的默认列表是 `blacklist` 域名匹配项，不能原样解释为 rule mode 规则。

## 匹配语法

Mihomo 的 DNS 域名通配符与路由规则中的 `DOMAIN-WILDCARD` 不是同一语法。当前默认涉及：

| 写法 | 匹配范围 |
| --- | --- |
| `"*"` | 只匹配 `nas`、`printer`、`localhost` 这类不含 `.` 的单标签名称 |
| `*.example.com` | 只匹配一层子域名，如 `a.example.com`；不匹配 `example.com` 或 `b.a.example.com` |
| `+.example.com` | 匹配根域名及任意层级子域名 |
| `time.*.com` | `*` 在该位置只匹配一层 |
| `geosite:name` | 导入运行环境 geosite 域名集合；实际内容随 geodata 版本而变 |

通配符应加引号。完整规则见 Mihomo [域名通配符语法](https://wiki.metacubex.one/en/handbook/syntax/)。

## 默认过滤项的意图与限度

| 类别 | 当前条目 | 边界 |
| --- | --- | --- |
| 本地名称 | `"*"`、`geosite:private`、`+.lan`、`+.local` | 让单标签、私有或本地域名更容易交给真实 resolver；不保证对应 LAN/mDNS resolver 可达 |
| 连通性探测 | `geosite:connectivity-check` | 覆盖范围由 geodata 决定；它不是强制门户或系统联网问题的完整修复 |
| Xiaomi / 米家 | `+.market.xiaomi.com`、`Mijia Cloud`、`dig.io.mi.com` | 针对具体兼容场景，不放宽为整个 `mi.com` |
| Apple | `*.icloud.com` | 只匹配一层 `icloud.com` 子域名；不是 Apple 服务全量清单 |
| NTP | `time.*.com`、`ntp.*.com`、`+.pool.ntp.org` | 只影响这些名称的 DNS 答复，不保证 UDP、系统时间服务或 TUN 路径 |
| STUN | `stun.*.*`、`stun.*.*.*` | 是有限的层级模式，不等于匹配所有 STUN 服务，也不配置 UDP 路由 |

默认 `Sniffer` processor 另行配置 `skip-domain`：

```yaml
sniffer:
  skip-domain:
    - "Mijia Cloud"
    - "dlg.io.mi.com"
    - "+.push.apple.com"
```

这控制域名嗅探是否改写连接目标，不是 fake-IP 过滤。默认 `TUN` processor 则启用 DNS hijack，并绕开 RFC 1918、IPv4/IPv6 link-local 和 IPv6 ULA 等地址；它同样不会把域名加入 `fake-ip-filter`。

## Tailscale 与 MagicDNS

Tailscale 兼容是 Web 中的可选 processor preset，不属于新建默认。添加 `Tailscale 共存` 时，Web 会确保 `TUN` 在前，并追加：

```yaml
dns:
  fake-ip-filter:
    - "+.tailscale.com"
    - "+.ts.net"
  nameserver-policy:
    "<+.ts.net>": 100.100.100.100

tun:
  route-exclude-address:
    - 100.64.0.0/10
    - fd7a:115c:a1e0::/48
```

preset 使用 `yaml_override` 的 `+` 运算追加数组，实际保存内容中的键为 `fake-ip-filter+` 与 `route-exclude-address+`；上面展示的是 merge 后的语义。

这些设置分别处理三个边界：

1. `+.tailscale.com` 与 `+.ts.net` 请求真实解析；
2. `<host>.<tailnet>.ts.net` 的查询由 `nameserver-policy` 指向 Tailscale resolver；
3. Tailscale IPv4/IPv6 地址从 Mihomo TUN 自动路由中排除。

`+.tailscale.com` 不匹配 `*.ts.net`，而 `+.ts.net` 也不能单独保证系统已接受 Tailscale DNS 设置。MagicDNS 同时支持单标签 machine name 和 `<machine>.<tailnet>.ts.net` FQDN，并依赖客户端/系统 resolver 与 search domain；见 Tailscale [MagicDNS](https://tailscale.com/docs/features/magicdns)和 [DNS 参考](https://tailscale.com/docs/reference/dns-in-tailscale)。

若还要从 tailnet 设备访问 Mihomo 的 LAN 入站，需要另加 `Tailnet 代理共享`。它在上述依赖之后把 `100.64.0.0/10`、`fd7a:115c:a1e0::/48` 追加到 `lan-allowed-ips`。DNS 解析、TUN 目标绕行和入站来源 ACL 是彼此独立的三项配置。

## 何时修改

只有在确认应用确实需要真实 DNS 结果、且 DNS 查询与连接路径已经厘清后，才应添加过滤项。优先使用精确主机名，其次使用必要的最窄后缀；不要把社区大清单直接复制为公共默认。

具体检查、追加与验证步骤见 [排查 Mihomo fake-IP 问题](../how-to/troubleshoot-mihomo-fake-ip.md)。
