# 社区配置预设

本页是 Sandrone Web 社区配置预设的唯一完整参考。它记录每个预设的动机、支持
客户端、默认状态、精确生成行为、风险、依赖/冲突和来源；其它文档只链接到这里，
不复制预设矩阵。

## 适用范围与所有权

- Mihomo 产物固定面向 v1.19.25，sing-box 固定面向 v1.13.14，Shadowrocket
  配置基线固定到源码 revision `5f1916b5897fc59fb7172aca59ae52050a3532fe`。
  这些 revision 来自已鉴权的 `GET /v1/capabilities/formats` 索引，并显示在 Web
  文件预览中；不写入 `FileSpec`，也不在 Web 预设目录另存一份版本号。
- 基础默认和默认开启的 processor 只影响之后创建的新文件；分组默认只影响之后
  创建的新分组。编辑已有文件或分组不会自动回填或迁移。
- 选择预设会复制一个普通、可编辑的 file-stage `merge` 或 `script` processor。
  副本不会随 Sandrone 升级自动更新。只有内容仍与受管预设完全一致的副本才能被
  冲突规划器识别并移除；编辑过或未知的 processor 始终归用户所有。三个 Mihomo
  Fake-IP 规则源是例外：它们通过内容首行的 Sandrone marker 保留预设身份；用户再次
  选择同一来源时会显式刷新为当前版本，选择另一来源时会替换旧来源。删除 marker
  后，该副本恢复为普通用户 processor，不再被自动刷新或冲突移除。
- 依赖补齐和冲突移除在一次明确的添加操作中完成，并保留所有非冲突 processor 的
  相对顺序。界面只列出新增依赖和被移除的冲突项。
- 有序规则固定为“用户/服务规则 → 按 processor 声明顺序生成的场景规则 → 私网、
  地区等通用规则 → `MATCH`/`FINAL`”。预设不改写最终策略；无法找到安全锚点时
  整步失败，不发布半成品。

目标 revision 的构建来源还可从[构建身份](build-info.md)和
[格式与能力](capabilities.md)核对。

## 新文件与新分组基础默认

这些是创建行为，不是可追加到已有文件的迁移：

| 客户端 | 只对新对象生效的默认 | 风险/边界 | 来源 |
| --- | --- | --- | --- |
| Mihomo | `allow-lan: true`；`lan-allowed-ips` 只含 RFC1918 IPv4 与 `fc00::/7`；Geo 数据自动更新且间隔 24 小时；显式设置 `disable-keep-alive: true`；不输出 TUN；新建 `url-test` 分组写入 `tolerance: 50`。 | LAN 监听仍需配合端口和访问控制；只有显式添加 TUN 预设才生成并开启 TUN；不会给已有分组补写 tolerance。 | [Mihomo 全局配置](https://wiki.metacubex.one/en/config/general/)、[Mihomo TUN](https://wiki.metacubex.one/en/config/inbound/tun/) |
| sing-box | 新建 selector/urltest 分组可表达 `interrupt_exist_connections`；默认关闭且 false 在序列化时省略，已有值原样保留。 | urltest 开启后，自动切换出站会中断现有连接。 | [sing-box Selector](https://sing-box.sagernet.org/configuration/outbound/selector/)、[sing-box URLTest](https://sing-box.sagernet.org/configuration/outbound/urltest/) |
| Shadowrocket | `close-if-proxy-chain-missing=true`、`dns-direct-fallback-proxy=false`、`udp-policy-not-supported-behaviour=REJECT`、`block-quic=all-proxy`、`ipv6=true`、`prefer-ipv6=false`。 | 兼容性放宽必须显式选择下表中的可选预设。 | [Shadowrocket 社区配置](https://github.com/LOWERTOP/Shadowrocket/blob/5f1916b5897fc59fb7172aca59ae52050a3532fe/lazy.conf) |

## 通用与 Mihomo 预设

表中的“默认开”仍只表示新建文件会复制该 processor。

| 预设 | 动机与客户端 | 默认 | 精确生成行为 | 风险 | 依赖 / 冲突 | 主要来源 |
| --- | --- | --- | --- | --- | --- | --- |
| Sniffer | 从 HTTP/TLS/QUIC 恢复目标域名；Mihomo。 | 开 | 强制替换 `sniffer` 为启用状态；沿用 Mihomo 默认的 DNS mapping 与 pure-IP 解析，不替换实际连接目标；仅嗅探 HTTP 80/8080/8880、TLS 443/8443、QUIC 443/8443，并保留精确 skip-domain。 | 会检查连接元数据。 | 无。 | [Mihomo Sniffer](https://wiki.metacubex.one/en/config/sniff/) |
| TUN | 接管 Mihomo 系统路由和 DNS；Mihomo。 | 关 | 启用 mixed stack、auto-route、strict-route、auto-detect-interface 与 UDP/TCP 53 DNS hijack；保留预置私网、link-local、ULA 和 mDNS exclusions。 | 平台路由或 DNS 不匹配会中断连接。 | Tailscale 相关预设可依赖它。 | [Mihomo TUN](https://wiki.metacubex.one/en/config/inbound/tun/) |
| Fake-IP 兼容扩展（稳定） | 为常见校时、软件更新、媒体、本地登录、银行、P2P、加速器和远控端点返回真实 IP；Mihomo。 | 关 | 仅通过 `fake-ip-filter+` 追加项目维护的静态域名与通配清单；离线、自包含，不改规则、resolver 或 TUN。完整清单见 [Mihomo fake-IP](mihomo-fake-ip.md#稳定兼容扩展)。 | 例外域名的 DNS 与路由行为会改变；它们不会因此自动变成 `DIRECT`。 | 与 OpenClash、ShellCrash Fake-IP 规则互斥。 | [Mihomo DNS](https://wiki.metacubex.one/en/config/dns/)、[社区配置索引](https://gui-for-cores.github.io/guide/gfs/community) |
| OpenClash Fake-IP 规则（跟随上游） | 使用 OpenClash 维护的完整兼容列表；Mihomo。 | 关 | 添加唯一 `domain`/`text` HTTP rule-provider，并通过 `fake-ip-filter+` 引用；每 86400 秒检查 jsDelivr 上的 OpenClash `master`。 | 初次下载失败时扩展列表不生效；列表包含 `+.qq.com` 等较宽规则，真实 IP 范围更大。 | 与稳定、ShellCrash Fake-IP 规则互斥。 | [OpenClash 列表](https://github.com/vernesong/OpenClash/blob/master/luci-app-openclash/root/etc/openclash/custom/openclash_custom_fake_filter.list) |
| ShellCrash Fake-IP 规则（跟随上游） | 使用 ShellCrash 维护的完整兼容列表；Mihomo。 | 关 | 添加唯一 `domain`/`text` HTTP rule-provider，并通过 `fake-ip-filter+` 引用；每 86400 秒检查 jsDelivr 上的 ShellCrash `dev`。 | 初次下载失败时扩展列表不生效；上游变更会在用户未修改 FileSpec 时改变实际命中集合。 | 与稳定、OpenClash Fake-IP 规则互斥。 | [ShellCrash 列表](https://github.com/juewuy/ShellCrash/blob/dev/public/fake_ip_filter.list) |
| QUIC 强制回退 | 绕开质量差或受限的 UDP/443 路径；Mihomo。 | 关 | 在通用规则前拒绝 UDP 目标端口 443。 | 强制 TCP 回退、失去 HTTP/3 优势；依赖 UDP/443 的应用可能失败。 | 无。 | [RFC 9000](https://www.rfc-editor.org/rfc/rfc9000)、[社区配置索引](https://gui-for-cores.github.io/guide/gfs/community) |
| Tailscale 原生接管 | 由 Mihomo 自身建立 Tailnet 端点；Mihomo。 | 关 | 创建唯一 `{name: TAILSCALE,type: tailscale,ephemeral:false,udp:true,accept-routes:false}` proxy；可编辑 processor 参数 `auth_key` 非空时写入 `auth-key`；为 `+.ts.net` 配置 MagicDNS；移除两个标准外部 exclusions；在通用/最终规则前依次插入域名、IPv4 与 IPv6 TAILSCALE 规则。 | 未填写 Auth Key 时由目标核心在日志打印交互式登录 URL；端点启动时首次访问可能超时。 | 依赖 TUN；与外部共存。 | [Mihomo Tailscale](https://wiki.metacubex.one/en/config/proxies/tailscale/)、[Tailscale MagicDNS](https://tailscale.com/docs/features/magicdns) |
| Tailscale 共存 | 让系统中独立运行的 Tailscale 与 Mihomo TUN 共存；Mihomo。 | 关 | 把 `+.ts.net` 加入 fake-IP 例外并定向到 `100.100.100.100`；把 `100.64.0.0/10`、`fd7a:115c:a1e0::/48` 追加到 TUN route exclusions；不创建 Tailscale proxy。 | 系统 Tailscale 必须已运行并正确配置。 | 依赖 TUN；与原生接管。 | [Tailscale MagicDNS](https://tailscale.com/docs/features/magicdns)、[Tailscale DNS](https://tailscale.com/docs/reference/dns-in-tailscale) |
| Tailnet 代理共享 | 允许 Tailnet 设备访问 Mihomo LAN listener；Mihomo。 | 关 | 把标准 Tailnet IPv4/IPv6 段追加到 `lan-allowed-ips`。 | 扩大入站来源范围；必须核对监听端口和访问控制。 | 依赖 TUN 与 Tailscale 外部共存。 | [Mihomo 全局配置](https://wiki.metacubex.one/en/config/general/)、[Tailscale CGNAT](https://tailscale.com/kb/1015/100.x-addresses) |

## sing-box 预设

| 预设 | 动机 | 默认 | 精确生成行为 | 风险 | 依赖 / 冲突 | 主要来源 |
| --- | --- | --- | --- | --- | --- | --- |
| Sniff & DNS Hijack | 在路由前识别协议并接管 DNS。 | 开 | 通过 JSON override 前插 `{action:"sniff"}`，再前插匹配 DNS 协议或端口 53 的 `hijack-dns` logical rule。 | 检查连接元数据并改变 resolver 路径。 | QUIC 预设依赖它。 | [sing-box Sniff](https://sing-box.sagernet.org/configuration/route/rule_action/#sniff)、[DNS Hijack](https://sing-box.sagernet.org/configuration/route/rule_action/#hijack-dns) |
| QUIC 强制回退 | 迫使兼容流量回退至 TCP。 | 关 | 依赖 sniff，在通用规则前插入 `{protocol:"quic",action:"reject"}`。 | 失去 HTTP/3 优势，必须使用 QUIC 的应用可能失败。 | 依赖 Sniff & DNS Hijack。 | [RFC 9000](https://www.rfc-editor.org/rfc/rfc9000)、[sing-box Protocol](https://sing-box.sagernet.org/configuration/route/rule/#protocol) |
| Tailscale 原生接管 | 由 sing-box v1.13.14 自身建立 Tailnet endpoint。 | 关 | 创建唯一 `{type:"tailscale",tag:"ts-ep",ephemeral:false,accept_routes:false}` endpoint；可编辑 processor 参数 `auth_key` 非空时写入同名 endpoint 字段；移除标准外部 exclusions；添加 `{type:"tailscale",tag:"ts-dns",endpoint:"ts-ep",accept_default_resolvers:false}` DNS server、legacy `{ip_accept_any:true,server:"ts-dns"}` DNS rule，以及在通用/最终规则前的 `{preferred_by:["ts-ep"],action:"route",outbound:"ts-ep"}` route rule；`dns.final`、`route.final` 不变。 | 未填写 Auth Key 时由目标核心在日志打印交互式登录 URL；端点启动时首次访问可能超时。 | 需要当前配置已有唯一 TUN；与外部共存。 | [sing-box Tailscale endpoint](https://sing-box.sagernet.org/configuration/endpoint/tailscale/)、[Tailscale DNS server](https://sing-box.sagernet.org/configuration/dns/server/tailscale/)、[preferred_by](https://sing-box.sagernet.org/configuration/route/rule/#preferred_by) |
| Tailscale 共存 | 让系统 Tailscale 与 sing-box TUN 共存。 | 关 | 精确去重后把两个标准地址段加入选定 TUN `route_exclude_address`；添加 `{type:"udp",tag:"ts-dns",server:"100.100.100.100"}`，并只用 `domain_suffix:["ts.net"]` DNS rule 路由到它；不创建 endpoint；`dns.final`、`route.final` 不变。 | 系统 Tailscale 必须已运行并正确配置。 | 需要当前配置已有唯一 TUN；与原生接管。 | [Tailscale MagicDNS](https://tailscale.com/docs/features/magicdns)、[Tailscale DNS](https://tailscale.com/docs/reference/dns-in-tailscale) |

## Shadowrocket 预设

| 预设 | 动机 | 默认 | 精确生成行为 | 风险 | 依赖 / 冲突 | 主要来源 |
| --- | --- | --- | --- | --- | --- | --- |
| Tailscale 原生接管 | 使用 Shadowrocket 自身的 TAILSCALE policy。 | 关 | 在第一段规则顶部精确插入 `DOMAIN-SUFFIX,ts.net,TAILSCALE`、`IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve`、`IP-CIDR,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve`；不显示模块启用提醒。 | Tailscale 的可用性和认证由 Shadowrocket 自身控制。 | 与外部共存冲突。 | [Shadowrocket 社区配置](https://github.com/LOWERTOP/Shadowrocket/blob/5f1916b5897fc59fb7172aca59ae52050a3532fe/lazy.conf) |
| Tailscale 共存 | 把 Tailnet 流量交给当前 LAN 中运行 Tailscale 的路由器；Shadowrocket。 | 关 | 精确去重后把 `100.64.0.0/10`、`fd7a:115c:a1e0::/48` 同时追加到 `skip-proxy` 与 `tun-excluded-routes`；在第一段规则顶部依次插入域名、IPv4 与 IPv6 的 `DIRECT` 规则；不修改 `[Host]` 或 DNS。 | LAN 网关必须拥有对应 Tailnet 路由；MagicDNS 需要用户现有 DNS 或独立 Host 配置支持。离开该网络后继续使用此配置会把 Tailnet 流量旁路到错误的物理网关。 | 与原生接管冲突。 | [Shadowrocket Tailscale 与通用参数](https://github.com/LOWERTOP/Shadowrocket/wiki/) |

## Tailscale 三态与安全边界

三种客户端的状态都是：没有 Tailscale processor 即关闭；外部共存表示独立于
目标客户端的路由所有者负责 Tailnet；原生接管表示目标客户端自身处理 Tailnet。
两种模式互斥且默认都关闭。Mihomo 与 sing-box 的外部所有者是本机系统
Tailscale；Shadowrocket 共存的外部所有者是当前 LAN 网关上的 Tailscale。

三种客户端的外部共存实现不同。Mihomo 与 sing-box 排除目标 TUN 路由并把
MagicDNS 定向到 `100.100.100.100`；Shadowrocket 使用 `DIRECT`、`skip-proxy`
和 `tun-excluded-routes` 把 Tailnet 流量完整旁路到物理网络，不接管 MagicDNS。
它不支持同机双 VPN 共存语义，只适用于当前默认网关已经提供 Tailnet 路由的网络；
移动设备离开该网络后应切换回原生接管或关闭模式。

Mihomo 与 sing-box 原生预设都在 processor `args` 中提供可编辑 `auth_key`。非空值
会随文件配置保存并写入目标核心；空值则省略该字段，由目标核心提供交互式登录。
Sandrone 不生成 Auth Key，也不提供登录表单、登录 URL、二维码、Headscale、
自定义 control URL 或 Exit Node 选择。原生 endpoint 使用默认 hostname/state
directory，省略 advertise、relay、SSH、MTU 与 system-interface 等高级字段；
`accept_routes` 保持 false。

标准 `100.64.0.0/10` 与 `fd7a:115c:a1e0::/48` 只是可编辑起点，不代表用户
发布的 subnet routes。需要额外子网 CIDR 时，用户应编辑已经复制到文件中的
processor；只有在明确理解目标客户端语义后，才自行修改目标原生内容以开启
`accept_routes` 或选择 Exit Node。预设从不把默认 `MATCH`、`FINAL`、
`route.final` 或 `dns.final` 改成 Tailscale。

sing-box 的固定目标 v1.13.14 在 route rule 使用 `preferred_by` 匹配 endpoint
提供的 MagicDNS 域名和 allowed IP；该版本的 MagicDNS-only DNS rule 使用 legacy
`ip_accept_any`，不是 v1.14 才支持的 DNS-rule `preferred_by`，并明确关闭
`accept_default_resolvers`，因此普通查询不会把 Tailscale resolver 当作全局
fallback。

## 可编辑 GitHub 加速快捷项

Mihomo、sing-box 和 Shadowrocket 还共享“GitHub 加速”快捷项，并在三个客户端的
新建文件中默认启用。它
把项目已知的 MetaCubeX `meta`/`sing`、blackmatrix7 与 Shadowrocket AI 使用的
iab0x00 GitHub Raw 前缀改写为 jsDelivr，未匹配内容保持原样且不产生 warning。
它生成普通 inline script，并把
有序替换表放在可编辑参数中，不是新的 processor type；删除或编辑副本即可改变
行为。精确参数与前缀表见
[Processors](processors.md#web-内置字符串替换预设)，来源为
[jsDelivr GitHub 文档](https://www.jsdelivr.com/documentation#id-github)。

新建文件与新添加预设的 processor 名称使用当时的 Web 界面语言；已保存名称视为
用户数据并保持原样。产品名、协议名和技术缩写保留原文。
