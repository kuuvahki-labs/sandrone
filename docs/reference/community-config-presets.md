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
  冲突规划器识别并移除；编辑过或未知的 processor 始终归用户所有。
- 依赖补齐和冲突移除在一次明确的添加操作中完成，并保留所有非冲突 processor 的
  相对顺序。界面会列出新增依赖、风险和被移除的冲突项。
- 有序规则固定为“用户/服务规则 → 按 processor 声明顺序生成的场景规则 → 私网、
  地区等通用规则 → `MATCH`/`FINAL`”。预设不改写最终策略；无法找到安全锚点时
  整步失败，不发布半成品。

目标 revision 的构建来源还可从[构建身份](build-info.md)和
[格式与能力](capabilities.md)核对。

## 新文件与新分组基础默认

这些是创建行为，不是可追加到已有文件的迁移：

| 客户端 | 只对新对象生效的默认 | 风险/边界 | 来源 |
| --- | --- | --- | --- |
| Mihomo | `allow-lan: true`；`lan-allowed-ips` 只含 RFC1918 IPv4 与 `fc00::/7`；Geo 数据自动更新且间隔 24 小时；新建 `url-test` 分组写入 `tolerance: 50`。 | LAN 监听仍需配合端口和访问控制；不会给已有分组补写 tolerance。 | [Mihomo 全局配置](https://wiki.metacubex.one/en/config/general/)、[Mihomo TUN](https://wiki.metacubex.one/en/config/inbound/tun/) |
| sing-box | 新建 selector/urltest 分组可表达 `interrupt_exist_connections`；默认关闭且 false 在序列化时省略，已有值原样保留。 | urltest 开启后，自动切换出站会中断现有连接。 | [sing-box Selector](https://sing-box.sagernet.org/configuration/outbound/selector/)、[sing-box URLTest](https://sing-box.sagernet.org/configuration/outbound/urltest/) |
| Shadowrocket | `close-if-proxy-chain-missing=true`、`dns-direct-fallback-proxy=false`、`udp-policy-not-supported-behaviour=REJECT`、`block-quic=all-proxy`、`ipv6=true`、`prefer-ipv6=false`。 | 兼容性放宽必须显式选择下表中的可选预设。 | [Shadowrocket 社区配置](https://github.com/LOWERTOP/Shadowrocket/blob/5f1916b5897fc59fb7172aca59ae52050a3532fe/lazy.conf) |

## 通用与 Mihomo 预设

表中的“默认开”仍只表示新建文件会复制该 processor。

| 预设 | 动机与客户端 | 默认 | 精确生成行为 | 风险 | 依赖 / 冲突 | 主要来源 |
| --- | --- | --- | --- | --- | --- | --- |
| 传统 NTP 直连 | 让不支持代理的传统 NTP 正常校时；Mihomo、sing-box、Shadowrocket。 | 开 | 在通用规则前分别插入 `AND,((NETWORK,UDP),(DST-PORT,123)),DIRECT`、`{network:"udp",port:123,outbound:"direct"}`、`AND,((PROTOCOL,UDP),(DST-PORT,123)),DIRECT`；只匹配 UDP 目标端口 123。 | 流量绕过代理，暴露真实直连出口。 | 无。 | [RFC 5905](https://www.rfc-editor.org/rfc/rfc5905) |
| Sniffer | 从 HTTP/TLS/QUIC 恢复目标域名；Mihomo。 | 开 | 强制替换 `sniffer` 为启用状态，开启 DNS mapping、pure-IP 解析和 destination override，使用预置 HTTP/TLS/QUIC 端口及精确 skip-domain。 | 会检查连接元数据。 | 无。 | [Mihomo Sniffer](https://wiki.metacubex.one/en/config/sniff/) |
| TUN | 接管 Mihomo 系统路由和 DNS；Mihomo。 | 开 | 启用 mixed stack、auto-route、strict-route、auto-detect-interface 与 UDP/TCP 53 DNS hijack；保留预置私网、link-local、ULA 和 mDNS exclusions。 | 平台路由或 DNS 不匹配会中断连接。 | Tailscale、Linux 加速等预设可依赖它。 | [Mihomo TUN](https://wiki.metacubex.one/en/config/inbound/tun/) |
| Fake-IP 兼容扩展 | 为常见校时、软件更新、媒体、本地登录、银行、P2P、加速器和远控端点返回真实 IP；Mihomo。 | 关 | 仅通过 `fake-ip-filter+` 追加项目维护的静态精确清单；不改规则、resolver 或 TUN。完整清单见 [Mihomo fake-IP](mihomo-fake-ip.md#fake-ip-兼容扩展)。 | 例外域名的 DNS 与路由行为会改变；它们不会因此自动变成 `DIRECT`。 | 无。 | [Mihomo DNS](https://wiki.metacubex.one/en/config/dns/)、[社区配置索引](https://gui-for-cores.github.io/guide/gfs/community) |
| STUN 端口阻断 | 减少应用通过常见 STUN 端口发现公网出口；Mihomo。 | 关 | 在通用规则前拒绝 UDP 目标端口 3478 与 5349。 | 只是常见端口近似；可能破坏 WebRTC、语音/视频、P2P 和 Tailscale 打洞。 | 与 UDP/P2P EIM、Tailscale 原生/外部模式冲突。 | [RFC 8489](https://www.rfc-editor.org/rfc/rfc8489)、[社区配置索引](https://gui-for-cores.github.io/guide/gfs/community) |
| QUIC 强制回退 | 绕开质量差或受限的 UDP/443 路径；Mihomo。 | 关 | 在通用规则前拒绝 UDP 目标端口 443。 | 强制 TCP 回退、失去 HTTP/3 优势；依赖 UDP/443 的应用可能失败。 | 无。 | [RFC 9000](https://www.rfc-editor.org/rfc/rfc9000)、[社区配置索引](https://gui-for-cores.github.io/guide/gfs/community) |
| UDP/P2P 兼容 | 改善部分游戏、语音、P2P 和打洞；Mihomo。 | 关 | 写入 `tun.endpoint-independent-nat: true`。 | 可能轻微降低性能与隐私。 | 与 STUN 阻断冲突。 | [Mihomo TUN](https://wiki.metacubex.one/en/config/inbound/tun/) |
| Linux/OpenWrt TUN 加速 | 在支持平台减少 TUN 路由开销；Mihomo。 | 关 | 写入 `tun.auto-route: true`、`tun.auto-redirect: true`、`find-process-mode: strict`；不会生成 `off` 或 keepalive 字段。 | 仅适合 Linux/OpenWrt；依赖 auto-route，可能与 routing mark 冲突。`strict` 只在规则需要时查询进程数据，转发流量未必能对应本地进程。 | 依赖 TUN。 | [Mihomo TUN](https://wiki.metacubex.one/en/config/inbound/tun/)、[v1.19.25 process mode](https://github.com/MetaCubeX/mihomo/blob/v1.19.25/component/process/find_process_mode.go) |
| Windows 宽松路由 | 兼容虚拟机、多网卡或特殊路由；Mihomo。 | 关 | 写入 `tun.strict-route: false`。 | 降低 Windows 多宿主 DNS 防泄漏与不支持流量的 fail-closed 能力。 | 无。 | [Mihomo TUN](https://wiki.metacubex.one/en/config/inbound/tun/) |
| Tailscale 原生接管 | 由 Mihomo 自身建立 Tailnet 端点；Mihomo。 | 关 | 创建唯一 `{name: TAILSCALE,type: tailscale,ephemeral:false,udp:true,accept-routes:false}` proxy；为 `+.ts.net` 配置 MagicDNS；移除两个标准外部 exclusions；在通用/最终规则前依次插入域名、IPv4 与 IPv6 TAILSCALE 规则。 | 首次启动可能由目标核心在日志打印交互式登录 URL，端点启动时首次访问可能超时。 | 依赖 TUN；与外部共存、STUN 阻断冲突。 | [Mihomo Tailscale](https://wiki.metacubex.one/en/config/proxies/tailscale/)、[Tailscale MagicDNS](https://tailscale.com/docs/features/magicdns) |
| Tailscale 共存 | 让系统中独立运行的 Tailscale 与 Mihomo TUN 共存；Mihomo。 | 关 | 把 `+.ts.net` 加入 fake-IP 例外并定向到 `100.100.100.100`；把 `100.64.0.0/10`、`fd7a:115c:a1e0::/48` 追加到 TUN route exclusions；不创建 Tailscale proxy。 | 系统 Tailscale 必须已运行并正确配置。 | 依赖 TUN；与原生接管、STUN 阻断冲突。 | [Tailscale MagicDNS](https://tailscale.com/docs/features/magicdns)、[Tailscale DNS](https://tailscale.com/docs/reference/dns-in-tailscale) |
| Tailnet 代理共享 | 允许 Tailnet 设备访问 Mihomo LAN listener；Mihomo。 | 关 | 把标准 Tailnet IPv4/IPv6 段追加到 `lan-allowed-ips`。 | 扩大入站来源范围；必须核对监听端口和访问控制。 | 依赖 TUN 与 Tailscale 外部共存。 | [Mihomo 全局配置](https://wiki.metacubex.one/en/config/general/)、[Tailscale CGNAT](https://tailscale.com/kb/1015/100.x-addresses) |

## sing-box 预设

| 预设 | 动机 | 默认 | 精确生成行为 | 风险 | 依赖 / 冲突 | 主要来源 |
| --- | --- | --- | --- | --- | --- | --- |
| Sniff & DNS Hijack | 在路由前识别协议并接管 DNS。 | 开 | 通过 JSON override 前插 `{action:"sniff"}`，再前插匹配 DNS 协议或端口 53 的 `hijack-dns` logical rule。 | 检查连接元数据并改变 resolver 路径。 | STUN/QUIC 预设依赖它。 | [sing-box Sniff](https://sing-box.sagernet.org/configuration/route/rule_action/#sniff)、[DNS Hijack](https://sing-box.sagernet.org/configuration/route/rule_action/#hijack-dns) |
| 确保 TUN 入站 | 为结构预设提供唯一安全 TUN 目标。 | 关 | `tag: tun-in` 优先；否则接受唯一 `type: tun`。没有 TUN 时仅本预设追加标准 `tun-in`（双栈地址、auto_route/strict_route 为 true）；歧义或同名非 TUN 安全失败。 | 新 TUN 会接管系统路由并需要平台权限。 | Linux 加速和两种 Tailscale 模式依赖它。 | [sing-box TUN](https://sing-box.sagernet.org/configuration/inbound/tun/) |
| STUN 阻断 | 减少公网出口地址暴露。 | 关 | 依赖 sniff，在通用规则前插入 `{protocol:"stun",action:"reject"}`。 | 可能使 WebRTC、语音、视频会议或 P2P 降级/失败。 | 依赖 Sniff & DNS Hijack；与 UDP/P2P EIM、Tailscale 原生/外部模式冲突。 | [RFC 8489](https://www.rfc-editor.org/rfc/rfc8489)、[sing-box Protocol](https://sing-box.sagernet.org/configuration/route/rule/#protocol) |
| QUIC 强制回退 | 迫使兼容流量回退至 TCP。 | 关 | 依赖 sniff，在通用规则前插入 `{protocol:"quic",action:"reject"}`。 | 失去 HTTP/3 优势，必须使用 QUIC 的应用可能失败。 | 依赖 Sniff & DNS Hijack。 | [RFC 9000](https://www.rfc-editor.org/rfc/rfc9000)、[sing-box Protocol](https://sing-box.sagernet.org/configuration/route/rule/#protocol) |
| 仅 IPv4 | 修复 IPv6 路由、DNS 或节点能力不完整的网络。 | 关 | 设置 `dns.strategy="ipv4_only"`，仅从选定 TUN 的 `address` 删除 IPv6 前缀；其它 IPv6 配置不变。 | IPv6-only 资源不可达；不是通用性能优化。 | 需要当前配置已有唯一 TUN。 | [sing-box DNS](https://sing-box.sagernet.org/configuration/dns/)、[sing-box TUN](https://sing-box.sagernet.org/configuration/inbound/tun/) |
| UDP/P2P 兼容 | 改善部分游戏、语音、P2P 和打洞。 | 关 | 在选定 TUN 写入 `endpoint_independent_nat: true`。 | 仅 gVisor 栈有额外效果；其它支持栈本身已使用 EIM。可能轻微降低性能，并与 STUN 隐私目标冲突。 | 需要唯一 TUN；与 STUN 阻断冲突。 | [sing-box TUN](https://sing-box.sagernet.org/configuration/inbound/tun/) |
| Linux/OpenWrt TUN 加速 | 在支持平台启用 auto-redirect。 | 关 | 在选定 TUN 写入 `auto_route: true` 与 `auto_redirect: true`。 | 仅适合支持的 Linux/OpenWrt 环境；依赖 auto-route，可能与 routing mark 冲突。 | 依赖确保 TUN。 | [sing-box TUN](https://sing-box.sagernet.org/configuration/inbound/tun/) |
| MPTCP 直连 | 避免 sing-box 无法透明代理的 MPTCP 被错误处理。 | 关 | 在选定 TUN 写入 `exclude_mptcp: true`。 | MPTCP 绕过代理策略并暴露真实直连出口。 | 依赖 Linux/OpenWrt TUN 加速（继而依赖确保 TUN）。 | [sing-box TUN](https://sing-box.sagernet.org/configuration/inbound/tun/) |
| Windows 宽松路由 | 兼容 Windows 虚拟机、多网卡或特殊路由。 | 关 | 在选定 TUN 写入 `strict_route: false`。 | 降低多宿主 DNS 防泄漏与不支持流量的 fail-closed 能力。 | 需要当前配置已有唯一 TUN。 | [sing-box TUN](https://sing-box.sagernet.org/configuration/inbound/tun/) |
| Tailscale 原生接管 | 由 sing-box v1.13.14 自身建立 Tailnet endpoint。 | 关 | 创建唯一 `{type:"tailscale",tag:"ts-ep",ephemeral:false,accept_routes:false}` endpoint；移除标准外部 exclusions；添加 `{type:"tailscale",tag:"ts-dns",endpoint:"ts-ep",accept_default_resolvers:false}` DNS server、legacy `{ip_accept_any:true,server:"ts-dns"}` DNS rule，以及在通用/最终规则前的 `{preferred_by:["ts-ep"],action:"route",outbound:"ts-ep"}` route rule；`dns.final`、`route.final` 不变。 | 首次启动可能由目标核心在日志打印交互式登录 URL，端点启动时首次访问可能超时。 | 依赖确保 TUN；与外部共存、STUN 阻断冲突。 | [sing-box Tailscale endpoint](https://sing-box.sagernet.org/configuration/endpoint/tailscale/)、[Tailscale DNS server](https://sing-box.sagernet.org/configuration/dns/server/tailscale/)、[preferred_by](https://sing-box.sagernet.org/configuration/route/rule/#preferred_by) |
| Tailscale 共存 | 让系统 Tailscale 与 sing-box TUN 共存。 | 关 | 精确去重后把两个标准地址段加入选定 TUN `route_exclude_address`；添加 `{type:"udp",tag:"ts-dns",server:"100.100.100.100"}`，并只用 `domain_suffix:["ts.net"]` DNS rule 路由到它；不创建 endpoint；`dns.final`、`route.final` 不变。 | 系统 Tailscale 必须已运行并正确配置。 | 依赖确保 TUN；与原生接管、STUN 阻断冲突。 | [Tailscale MagicDNS](https://tailscale.com/docs/features/magicdns)、[Tailscale DNS](https://tailscale.com/docs/reference/dns-in-tailscale) |

## Shadowrocket 预设

| 预设 | 动机 | 默认 | 精确生成行为 | 风险 | 依赖 / 冲突 | 主要来源 |
| --- | --- | --- | --- | --- | --- | --- |
| 关闭 IPv6 | 兼容 IPv6 路由、DNS 或策略不完整的网络。 | 关 | 只在 `[General]` 写入 `ipv6=false`、`prefer-ipv6=false`。 | 只控制配置可表达的行为；不保证节点底层传输永远不使用 IPv6。 | 无。 | [Shadowrocket 社区配置](https://github.com/LOWERTOP/Shadowrocket/blob/5f1916b5897fc59fb7172aca59ae52050a3532fe/lazy.conf) |
| 不支持 UDP 时直连 | 兼容不支持 UDP 转发的节点策略。 | 关 | 只写入 `udp-policy-not-supported-behaviour=DIRECT`；基础默认仍是 `REJECT`。 | 流量绕过代理，真实出口、运营商路径和本地 DNS 可能暴露。 | 无。 | [Shadowrocket 社区配置](https://github.com/LOWERTOP/Shadowrocket/blob/5f1916b5897fc59fb7172aca59ae52050a3532fe/lazy.conf) |
| 受限网络 DNS 回退 | 直连 DNS 在受限网络失败时经代理重试。 | 关 | 只写入 `dns-direct-fallback-proxy=true`；基础默认仍为 false。 | 本应直连解析的域名可能改经代理。 | 无。 | [Shadowrocket 社区配置](https://github.com/LOWERTOP/Shadowrocket/blob/5f1916b5897fc59fb7172aca59ae52050a3532fe/lazy.conf) |
| Tailscale 原生接管 | 使用 Shadowrocket 自身的 TAILSCALE policy。 | 关 | 在通用/FINAL 前精确插入 `DOMAIN-SUFFIX,ts.net,TAILSCALE`、`IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve`、`IP-CIDR,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve`。没有外部共存模式，也不显示模块启用提醒。 | Tailscale 的可用性和认证由 Shadowrocket 自身控制。 | 无。 | [Shadowrocket 社区配置](https://github.com/LOWERTOP/Shadowrocket/blob/5f1916b5897fc59fb7172aca59ae52050a3532fe/lazy.conf) |

## Tailscale 三态与安全边界

对 Mihomo 和 sing-box，状态是：没有 Tailscale processor 即关闭；外部共存表示
系统中的独立 Tailscale 拥有隧道；原生接管表示目标核心创建文件局部端点。两种
模式互斥且默认都关闭。Shadowrocket 只有关闭和原生 TAILSCALE policy 两态，
不提供外部共存。

Sandrone 不接收、保存或生成 Auth Key，不提供登录表单、登录 URL、二维码、
Headscale 或自定义 control URL，也不自动选择 Exit Node。原生 endpoint 使用默认
hostname/state directory，省略 advertise、relay、SSH、MTU 与 system-interface
等高级字段；`accept_routes` 保持 false。目标核心自身可能在日志中提供登录信息，
这不是 Sandrone 的登录交互。

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

## 可编辑规则源地址替换快捷项

Mihomo、sing-box 和 Shadowrocket 还共享“GitHub 规则源地址替换”快捷项。它默认
把项目已知的 MetaCubeX `meta`/`sing` 与 blackmatrix7 GitHub Raw 前缀改写为
jsDelivr，未匹配内容保持原样且不产生 warning。它生成普通可编辑 inline script，
不是新的 processor type；删除或编辑副本即可改变行为。精确前缀表见
[Processors](processors.md#web-内置规则源地址替换)，来源为
[jsDelivr GitHub 文档](https://www.jsdelivr.com/documentation#id-github)。
