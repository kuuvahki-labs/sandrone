# Mihomo fake-IP 默认与边界

本页记录 Sandrone Web 新建 Mihomo 文件时使用的 fake-IP 配置、过滤列表和相关 processor 边界。它不是通用 Mihomo 推荐配置，也不是需要持续扩大的域名清单。

遇到具体解析或连接故障时，按 [排查 Mihomo fake-IP 问题](../how-to/troubleshoot-mihomo-fake-ip.md) 操作；本页不重复修改、重载和验证步骤。

## 适用范围

下文默认值只用于 **Web 新建** Mihomo 文件：

- 新建表单把 Web driver 的 inline base 写入 `FileSpec.source`；
- 新文件默认获得 `Sniffer` 与传统 NTP 直连 processor；base 不输出 TUN，只有显式添加 TUN processor 才生成并开启；
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
  prefer-h3: true
  enhanced-mode: fake-ip
  fake-ip-filter:
    - "*"
    - geosite:private
    - geosite:connectivity-check
    - "+.lan"
    - "+.local"
    - "+.market.xiaomi.com"
    - "Mijia Cloud"
    - "dig.io.mi.com"
    - "localhost.ptlogin2.qq.com"
    - "localhost.sec.qq.com"
    - "localhost.*.weixin.qq.com"
    - "*.icloud.com"
    - "time.*.com"
    - "ntp.*.com"
    - "+.pool.ntp.org"
    - "stun.*.*"
    - "stun.*.*.*"
```

列表及顺序是当前 Web source 的完整默认。Tailscale 域名和地址不在其中。

同一 base 的 bootstrap、国内、境外、节点域名 resolver 及其出站路径由
[客户端路由、DNS 与 IP 兜底](client-routing-dns.md#mihomo)统一说明。这些
resolver 设置参与实际 DNS 路径，但不改变 `fake-ip-filter` 的匹配语义。

## `fake-ip-filter` 的效果

Web base 显式设置 DNS `ipv6: false` 和 `prefer-h3: true`，仅省略 Mihomo 已提供的
`fake-ip-range: 198.18.0.1/16` 与 `fake-ip-filter-mode: blacklist` 默认值。按当前核心
默认语义，匹配过滤项的域名不获得
fake-IP 映射，而走真实解析；未匹配的域名可以从默认 fake-IP 地址池获得 fake IP。
Mihomo 官方说明见 [DNS 配置](https://wiki.metacubex.one/en/config/dns/)。

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
| QQ / 微信登录 | `localhost.ptlogin2.qq.com`、`localhost.sec.qq.com`、`localhost.*.weixin.qq.com` | 属于远程扩展下载失败时也必须保留的登录兼容保障；不放宽为整个 `qq.com` |
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

这控制域名嗅探是否改写连接目标，不是 fake-IP 过滤。可选 `TUN` processor 则启用 strict route 与 DNS hijack，并绕开 RFC 1918、IPv4/IPv6 link-local、IPv6 ULA，以及 `224.0.0.251/32`、`ff02::fb/128` 两个 mDNS 组播目标；它同样不会把域名加入 `fake-ip-filter`。

## Fake-IP 规则分层

基础保障始终来自 Web base。下列三个扩展规则源都默认关闭且互相冲突；用户选择新来源
时会移除已识别的旧来源。远程来源下载失败不会阻止基础 QQ/微信登录、NTP、STUN、
局域网和设备兼容条目继续生效。

### 稳定兼容扩展

`Fake-IP 兼容扩展（稳定）` 是 Web 中默认关闭的可选 file-stage processor preset，不属于
Web base，也不在新建文件默认处理链中。选择后，它通过
`yaml_override` 的 `fake-ip-filter+` 运算追加以下静态清单：

```yaml
dns:
  fake-ip-filter:
    # 扩展校时与 NTP 匹配
    - "time-ios.apple.com"
    - "time.*.gov"
    - "time.*.edu.cn"
    - "time.*.apple.com"
    - "time1.*.com"
    - "time2.*.com"
    - "time3.*.com"
    - "time4.*.com"
    - "time5.*.com"
    - "time6.*.com"
    - "time7.*.com"
    - "ntp1.*.com"
    - "ntp2.*.com"
    - "ntp3.*.com"
    - "ntp4.*.com"
    - "ntp5.*.com"
    - "ntp6.*.com"
    - "ntp7.*.com"
    - "*.time.edu.cn"
    - "*.ntp.org.cn"
    - "ntp.ntsc.ac.cn"

    # Apple 软件更新
    - "mesu.apple.com"
    - "swscan.apple.com"
    - "swquery.apple.com"
    - "swdownload.apple.com"
    - "swcdn.apple.com"
    - "swdist.apple.com"

    # 媒体与本地登录兼容
    - "music.163.com"
    - "*.music.163.com"
    - "y.qq.com"
    - "*.y.qq.com"
    - "streamoc.music.tc.qq.com"
    - "mobileoc.music.tc.qq.com"
    - "isure.stream.qqmusic.qq.com"
    - "dl.stream.qqmusic.qq.com"
    - "aqqmusic.tc.qq.com"
    - "amobile.music.tc.qq.com"
    - "songsearch.kugou.com"
    - "trackercdn.kugou.com"
    - "*.kuwo.cn"
    - "music.migu.cn"
    - "*.music.migu.cn"
    - "*.mcdn.bilivideo.cn"

    # 银行、P2P、加速器与远控兼容
    - "+.cmbchina.com"
    - "+.cmbimg.com"
    - "+.sandai.net"
    - "+.n0808.com"
    - "+.uu.163.com"
    - "ps.res.netease.com"
    - "+.oray.com"
    - "+.orayimg.com"
```

上面展示的是 merge 后的语义；实际 preset 内容中的键是
`fake-ip-filter+`。它只让匹配域名获得真实 IP，不把它们改为 `DIRECT`，也不改变
`nameserver-policy`、DNS 出站或 TUN 路由。

该清单由项目静态维护，不在运行时下载第三方列表；有意不包含 `geosite:cn`、
`rule-set:cn` 或整个 `qq.com`、`163.com` 等宽泛后缀。某个条目只有在多数环境中
可复现必须使用真实 IP 时，才考虑移入 Web base；单一应用或设备的兼容需求继续
留在此可选 preset，用户也可以直接编辑其 YAML。

### OpenClash 与 ShellCrash 上游规则

另外两个可选 preset 不复制第三方完整清单，而是生成 Mihomo 原生远程
`rule-provider`，再把它作为 `rule-set:` 条目追加到 `fake-ip-filter`：

```yaml
rule-providers:
  sandrone-fakeip-shellcrash:
    type: http
    behavior: domain
    format: text
    path: ./ruleset/sandrone-fakeip-shellcrash.list
    url: https://cdn.jsdelivr.net/gh/juewuy/ShellCrash@dev/public/fake_ip_filter.list
    interval: 86400
dns:
  fake-ip-filter+:
    - rule-set:sandrone-fakeip-shellcrash
```

OpenClash 使用独立的 `sandrone-fakeip-openclash` 名称和缓存路径，来源是
[OpenClash custom fake filter](https://github.com/vernesong/OpenClash/blob/master/luci-app-openclash/root/etc/openclash/custom/openclash_custom_fake_filter.list)；
ShellCrash 来源是
[ShellCrash fake_ip_filter.list](https://github.com/juewuy/ShellCrash/blob/dev/public/fake_ip_filter.list)。
两者通过 jsDelivr 分发，以免依赖另一个 processor 的执行顺序。首次下载失败且没有缓存
时，Mihomo 可以继续使用基础配置，但该上游扩展当次不会命中。

OpenClash 当前包含整个 `+.qq.com` 等较宽规则，ShellCrash 相对保守。选择它们表示接受
上游后续变更；需要完全可复现或离线生成时应选择稳定扩展。

## Tailscale 与 MagicDNS

Tailscale 模式、MagicDNS、规则、标准地址段、依赖与风险统一见
[社区配置预设](community-config-presets.md#tailscale-三态与安全边界)。本页只强调
fake-IP 边界：`+.ts.net` 例外决定 Tailnet FQDN 是否获得真实 DNS 结果，不会单独
选择 TAILSCALE 路由、排除 TUN 地址或放宽 LAN 入站；应检查最终渲染结果中的 DNS
与连接路径，而不是只看 `fake-ip-filter`。

## 何时修改

只有在确认应用确实需要真实 DNS 结果、且 DNS 查询与连接路径已经厘清后，才应添加过滤项。优先使用精确主机名，其次使用必要的最窄后缀；不要把社区大清单直接复制为公共默认。

具体检查、追加与验证步骤见 [排查 Mihomo fake-IP 问题](../how-to/troubleshoot-mihomo-fake-ip.md)。
