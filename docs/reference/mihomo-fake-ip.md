# Mihomo fake-IP 默认与边界

本页说明 Sandrone Web 新建 Mihomo 文件的 fake-IP 默认、匹配语法与可选扩展。
排查步骤见[排查 Mihomo fake-IP 问题](../how-to/troubleshoot-mihomo-fake-ip.md)。

## 适用范围

Web 新建表单会把 [Mihomo base](../../web/app/features/files/drivers/mihomo/base.ts)
复制到 `FileSpec.source`，并添加默认 Sniffer processor；TUN 需另行选择。
编辑已有文件不会回填这些默认值。API/CLI 省略 source 时使用服务端内建 base，
与 Web base 不同，因此应查看保存的 source 和最终产物，不能只依据 `kind: mihomo`。

## Web 新建 base

Web base 使用 fake-IP，开启 `store-selected`、`store-fake-ip`，DNS 设置
`ipv6: false`、`prefer-h3: true`。地址池和过滤模式沿用当前核心的
`198.18.0.1/16`、`blacklist` 默认值。完整配置与过滤项以
[base 源码](../../web/app/features/files/drivers/mihomo/base.ts)为准。

基础过滤项覆盖单标签、本地/私有名称、联网检测，以及部分设备、登录、NTP 和
STUN 兼容场景；不包含 Tailscale。resolver 与出站路径见
[客户端 DNS 路径](client-routing-dns.md#mihomo)。

## `fake-ip-filter` 的效果

在 blacklist 模式下，匹配域名返回真实解析结果，其余域名可以获得 fake IP。
这只决定 DNS 答复，不会自动选择 `DIRECT`、切换 resolver、绕开 TUN 或放宽
LAN 入站。真实 IP 连接仍按路由规则处理；若 DNS 查询经过 Mihomo、连接却绕过
其 fake-IP 映射链路，扩大过滤列表可能掩盖路径错误。

`fake-ip-filter-mode: rule` 使用另一套有序规则语法，不能直接套用本页的域名列表。
上游依据见 [Mihomo DNS](https://wiki.metacubex.one/en/config/dns/)。

## 匹配语法

DNS 域名通配符与路由 `DOMAIN-WILDCARD` 语法不同：

| 写法 | 匹配范围 |
| --- | --- |
| `"*"` | `nas`、`printer`、`localhost` 等不含点的单标签名称 |
| `"*.example.com"` | 一层子域名；不匹配根域名或 `b.a.example.com` |
| `"+.example.com"` | 根域名及任意层级子域名 |
| `"time.*.com"` | `*` 只匹配一层 |
| `geosite:name` | 运行环境的 geosite 域名集合，内容随 geodata 更新 |

YAML 通配符应加引号。完整规则见
[Mihomo 域名通配符语法](https://wiki.metacubex.one/en/handbook/syntax/)。

Sniffer 的 `skip-domain` 控制嗅探排除项，不是 fake-IP 过滤；当前
[Sniffer 预设](../../web/app/features/files/drivers/mihomo/preset-content/sniffer.yaml)
设置 `override-destination: false`，不替换实际连接目标。TUN、Tailscale 的影响见
[社区配置预设](community-config-presets.md)。

## Fake-IP 规则分层

基础过滤来自 Web base，以下三个可选扩展默认关闭且互斥。选择另一来源会替换
已识别的旧扩展；基础过滤不会因此被移除。

### 稳定兼容扩展

此预设用 `yaml_override` 的 `fake-ip-filter+` 追加静态列表，离线、自包含。
完整条目见 [fake-ip-compat.yaml](../../web/app/features/files/drivers/mihomo/preset-content/fake-ip-compat.yaml)，
主要覆盖额外校时、软件更新、媒体、银行、P2P 和远控场景。

追加用户自己的例外也可使用同样的 merge 内容：

```yaml
dns:
  fake-ip-filter+:
    - "device.example.com"
```

这只改变命中域名的 DNS 答复，不改变 resolver 或路由。排查时先验证是否为
fake-IP 问题，新增条目优先采用满足需求的最窄范围；用户可编辑复制后的 YAML。

### OpenClash 与 ShellCrash 上游规则

这两个预设生成 Mihomo 原生 `domain`/`text` HTTP rule-provider，再通过
`fake-ip-filter+` 引用它。当前配置见
[OpenClash 预设](../../web/app/features/files/drivers/mihomo/preset-content/fake-ip-openclash.yaml)和
[ShellCrash 预设](../../web/app/features/files/drivers/mihomo/preset-content/fake-ip-shellcrash.yaml)。
客户端每 86400 秒通过 jsDelivr 检查对应上游分支；首次下载失败且无缓存时，
当次扩展不生效，基础列表仍可用。远程列表可能包含宽泛后缀，且会随上游更新；
需要离线或固定列表时可使用稳定扩展或保存自己的副本。

上游来源为 [OpenClash custom fake filter](https://github.com/vernesong/OpenClash/blob/master/luci-app-openclash/root/etc/openclash/custom/openclash_custom_fake_filter.list)
与 [ShellCrash fake_ip_filter.list](https://github.com/juewuy/ShellCrash/blob/dev/public/fake_ip_filter.list)。

## Tailscale 与 MagicDNS

`+.ts.net` 例外只决定 Tailnet FQDN 是否得到真实 DNS 结果，不单独选择 TAILSCALE
路由或排除 TUN 地址。模式、MagicDNS、地址段和依赖见
[社区配置预设](community-config-presets.md#tailscale-三态与安全边界)。
