# 社区配置预设

本页说明 Web 社区配置预设的默认状态、用途、可编辑参数和选择后果。
预设是普通 processor 的可编辑副本，完整生成内容可从链接的源码查看。

## 适用范围与所有权

- 目标客户端的依据版本来自 `GET /v1/capabilities/formats`，显示在文件预览中；
  这是当前实现的验证基线，不限制用户编辑配置。
- 基础默认和默认开启的 processor 只影响之后创建的新文件；分组默认只影响之后
  创建的新分组。编辑已有文件或分组不会自动回填或迁移。
- 选择预设会复制一个普通、可编辑的 file-stage `merge` 或 `script` processor。
  副本不会随 Sandrone 升级自动更新。受管身份由各预设的识别规则决定；只有仍被
  识别的副本才能被冲突规划器移除，未识别的自定义 processor 始终归用户所有。三个 Mihomo
  Fake-IP 规则源是例外：它们通过内容首行的 Sandrone marker 保留预设身份；用户再次
  选择同一来源时会显式刷新为当前版本，选择另一来源时会替换旧来源。删除 marker
  后，该副本恢复为普通用户 processor，不再被自动刷新或冲突移除。
- 依赖补齐和冲突移除在一次明确的添加操作中完成，并保留所有非冲突 processor 的
  相对顺序。界面只列出新增依赖和被移除的冲突项。
- 有序规则预设按自己的插入模式放置规则，并保留最终策略。选择通用规则前插入的
  预设会寻找锚点，找不到时整步失败；Shadowrocket Tailscale 预设插在规则顶部。
  调整 processor 顺序会改变后续步骤看到的配置。

版本与字段能力查询见[格式与能力](capabilities.md)。

## 新文件与新分组基础默认

这些是创建行为，不是可追加到已有文件的迁移：

| 客户端 | 只对新对象生效的默认 | 风险/边界 | 来源 |
| --- | --- | --- | --- |
| Mihomo | `allow-lan: true`；`lan-allowed-ips` 只含 RFC1918 IPv4 与 `fc00::/7`；Geo 数据自动更新且间隔 24 小时；显式设置 `disable-keep-alive: true`；不输出 TUN；新建 `url-test` 分组写入 `tolerance: 50`。 | LAN 监听仍需配合端口和访问控制；只有显式添加 TUN 预设才生成并开启 TUN；不会给已有分组补写 tolerance。 | [Mihomo 全局配置](https://wiki.metacubex.one/en/config/general/)、[Mihomo TUN](https://wiki.metacubex.one/en/config/inbound/tun/) |
| sing-box | 新建 selector/urltest 分组可表达 `interrupt_exist_connections`；默认关闭且 false 在序列化时省略，已有值原样保留。 | urltest 开启后，自动切换出站会中断现有连接。 | [sing-box Selector](https://sing-box.sagernet.org/configuration/outbound/selector/)、[sing-box URLTest](https://sing-box.sagernet.org/configuration/outbound/urltest/) |
| Shadowrocket | `close-if-proxy-chain-missing=true`、`dns-direct-fallback-proxy=false`、`udp-policy-not-supported-behaviour=REJECT`、`block-quic=all-proxy`、`ipv6=true`、`prefer-ipv6=false`。 | 可编辑 source 或通过后置处理器调整。 | [Shadowrocket 社区配置](https://github.com/LOWERTOP/Shadowrocket/blob/5f1916b5897fc59fb7172aca59ae52050a3532fe/lazy.conf) |

## 通用与 Mihomo 预设

表中的“默认开”仍只表示新建文件会复制该 processor。

| 预设 | 动机与客户端 | 默认 | 生成行为 | 风险 | 依赖 / 冲突 | 主要来源 |
| --- | --- | --- | --- | --- | --- | --- |
| Sniffer | 从 HTTP/TLS/QUIC 恢复目标域名；Mihomo。 | 开 | 替换 `sniffer` 为启用状态，不替换实际连接目标；端口与排除项见 [Sniffer 内容](../../web/app/features/files/drivers/mihomo/preset-content/sniffer.yaml)。 | 会检查连接元数据。 | 无。 | [Mihomo Sniffer](https://wiki.metacubex.one/en/config/sniff/) |
| TUN | 接管 Mihomo 系统路由和 DNS；Mihomo。 | 关 | 启用 mixed stack、auto-route、strict-route、auto-detect-interface 与 UDP/TCP 53 DNS hijack；保留预置私网、link-local、ULA 和 mDNS exclusions。 | 平台路由或 DNS 不匹配会中断连接。 | Tailscale 相关预设可依赖它。 | [Mihomo TUN](https://wiki.metacubex.one/en/config/inbound/tun/) |
| Fake-IP 兼容扩展（稳定） | 为常见校时、软件更新、媒体、本地登录、银行、P2P、加速器和远控端点返回真实 IP；Mihomo。 | 关 | 仅通过 `fake-ip-filter+` 追加项目维护的静态域名与通配清单；离线、自包含，不改规则、resolver 或 TUN。列表与配置入口见 [Mihomo fake-IP](mihomo-fake-ip.md#稳定兼容扩展)。 | 例外域名的 DNS 与路由行为会改变；它们不会因此自动变成 `DIRECT`。 | 与 OpenClash、ShellCrash Fake-IP 规则互斥。 | [Mihomo DNS](https://wiki.metacubex.one/en/config/dns/)、[社区配置索引](https://gui-for-cores.github.io/guide/gfs/community) |
| OpenClash Fake-IP 规则（跟随上游） | 使用 OpenClash 维护的完整兼容列表；Mihomo。 | 关 | 添加唯一 `domain`/`text` HTTP rule-provider，并通过 `fake-ip-filter+` 引用；每 86400 秒检查 jsDelivr 上的 OpenClash `master`。 | 初次下载失败时扩展列表不生效；列表包含 `+.qq.com` 等较宽规则，真实 IP 范围更大。 | 与稳定、ShellCrash Fake-IP 规则互斥。 | [OpenClash 列表](https://github.com/vernesong/OpenClash/blob/master/luci-app-openclash/root/etc/openclash/custom/openclash_custom_fake_filter.list) |
| ShellCrash Fake-IP 规则（跟随上游） | 使用 ShellCrash 维护的完整兼容列表；Mihomo。 | 关 | 添加唯一 `domain`/`text` HTTP rule-provider，并通过 `fake-ip-filter+` 引用；每 86400 秒检查 jsDelivr 上的 ShellCrash `dev`。 | 初次下载失败时扩展列表不生效；上游变更会在用户未修改 FileSpec 时改变实际命中集合。 | 与稳定、OpenClash Fake-IP 规则互斥。 | [ShellCrash 列表](https://github.com/juewuy/ShellCrash/blob/dev/public/fake_ip_filter.list) |
| QUIC 强制回退 | 绕开质量差或受限的 UDP/443 路径；Mihomo。 | 关 | 在通用规则前拒绝 UDP 目标端口 443。 | 强制 TCP 回退、失去 HTTP/3 优势；依赖 UDP/443 的应用可能失败。 | 无。 | [RFC 9000](https://www.rfc-editor.org/rfc/rfc9000)、[社区配置索引](https://gui-for-cores.github.io/guide/gfs/community) |
| Tailscale 原生接管 | 由 Mihomo 自身建立 Tailnet 端点；Mihomo。 | 关 | 创建 Tailscale proxy、配置 MagicDNS，移除标准共存 exclusions，并在通用规则前加入 Tailnet 路由；参数见下文，完整内容见[脚本](../../web/app/features/files/processors/scripts/mihomo-tailscale-native.js)。 | 未填写 Auth Key 时由目标核心在日志打印交互式登录 URL；端点启动时首次访问可能超时。 | 依赖 TUN；与外部共存。 | [Mihomo Tailscale](https://wiki.metacubex.one/en/config/proxies/tailscale/)、[Tailscale MagicDNS](https://tailscale.com/docs/features/magicdns) |
| Tailscale 共存 | 让系统中独立运行的 Tailscale 与 Mihomo TUN 共存；Mihomo。 | 关 | 让 Tailscale 域名返回真实 IP，Tailnet 域名使用 MagicDNS，标准 Tailnet 地址绕开 TUN；完整内容见[共存配置](../../web/app/features/files/drivers/mihomo/preset-content/tailscale-external.yaml)。 | 系统 Tailscale 必须已运行并正确配置。 | 依赖 TUN；与原生接管。 | [Tailscale MagicDNS](https://tailscale.com/docs/features/magicdns)、[Tailscale DNS](https://tailscale.com/docs/reference/dns-in-tailscale) |
| Tailnet 代理共享 | 允许 Tailnet 设备访问 Mihomo LAN listener；Mihomo。 | 关 | 把标准 Tailnet IPv4/IPv6 段追加到 `lan-allowed-ips`。 | 扩大入站来源范围；必须核对监听端口和访问控制。 | 依赖 TUN 与 Tailscale 外部共存。 | [Mihomo 全局配置](https://wiki.metacubex.one/en/config/general/)、[Tailscale CGNAT](https://tailscale.com/kb/1015/100.x-addresses) |

## sing-box 预设

### 出站配置适配

此 sing-box JS 预设负责正则分组转换和显式默认出口设置。新建文件默认在处理链
首位启用，并将当前命名语言下的默认锚点组名保存到 `params.args.default_outbound`；
基础模板不重复声明 `route.final`。副本是可编辑、可排序、可禁用和可删除的普通
file-stage processor。

在组编辑器选择“正则筛选”，填写包含正则和可选排除正则。本次编辑首次出现正则组
且缺少该预设时，前端添加一次。手动或配置触发添加到已有文件时，默认出口参数
留空，不覆盖已有 base。自动和手动添加共用依赖、冲突与去重规则。
重新打开已有文件不补回缺失的处理器。仍有正则组但缺少启用的内置处理器时，
界面只提示、仍允许保存，调用方可以提供自定义替代脚本。

在处理器“参数”中使用 `default_outbound=Proxy` 指定默认出口；中文配置的新建值
为 `default_outbound=🚀 节点选择`。非空值覆盖 `route.final`，缺失或空白保持原值
或缺失状态，不推断替代默认。目标须在执行时的 `outbounds` 和 `endpoints` 中按
完整 tag 唯一存在；不存在、重复或需要修改的 route 结构非法时，生成失败。若目标
由另一处理器生成，应将其排在适配之前。请求参数不能覆盖保存的 `default_outbound`。
分组改名后需要同步该参数；当前分组里找不到目标时界面非阻断提示，最终以执行时
产物为准。没有设置参数时，不额外校验原 base 的 `route.final`。
路由规则仍按原顺序优先匹配，`default_outbound` 只控制 `route.final` 字段。

调整名称、超时或业务参数不改变内置身份；编辑过脚本正文的副本归用户管理。

组定义保存 `filter`、可选 `exclude-filter` 和 `outbounds: ["$nodes"]`。
driver 先展开成功渲染的订阅节点名（包括 endpoint），JS 再筛选组成员、保序去重，
并删除两个扩展字段；不删除节点定义。JS 只处理执行到该步时的 selector/urltest
成员名单，因此调整处理器顺序会改变它看到的输入。重复执行已展开结果保持不变。
这两个正则字段属于此脚本的输入，不是 sing-box 原生字段；API/CLI 调用也必须
显式携带[脚本](../../web/app/features/files/processors/scripts/sing-box-outbound-adapter.js)
或等价处理器，后端不会自动添加。仅需固定默认出口时，也可以在显式 base 中配置。

表达式采用 JavaScript 正则语法，默认区分大小写，支持开头的 `(?i)` 标记。
例如 `(?i)HK|香港` 可匹配香港节点，排除正则 `Home` 仅排除大小写一致的名称。
正则为空、语法错误、筛选后无成员，或显式 `default` 不在筛选结果中时，生成失败
并指出组名；不会自动回退到全部节点或直连。失败不会发布部分配置。

新生成的自适应地区组使用相同正则与处理器，已有固定成员组保持原样。
每次 Sandrone 生成文件都会重新筛选当次订阅结果，订阅快照缓存仍遵守原有策略；
客户端已经下载的文件不会自行重新匹配。Mihomo、Shadowrocket 继续使用各自原生
正则字段，统一编辑入口不代表三种正则引擎支持所有相同语法。

### 其他预设

| 预设 | 动机 | 默认 | 生成行为 | 风险 | 依赖 / 冲突 | 主要来源 |
| --- | --- | --- | --- | --- | --- | --- |
| Sniff & DNS Hijack | 在路由前识别协议并接管 DNS。 | 开 | 通过 JSON override 前插 `{action:"sniff"}`，再前插匹配 DNS 协议或端口 53 的 `hijack-dns` logical rule。 | 检查连接元数据并改变 resolver 路径。 | QUIC 预设依赖它。 | [sing-box Sniff](https://sing-box.sagernet.org/configuration/route/rule_action/#sniff)、[DNS Hijack](https://sing-box.sagernet.org/configuration/route/rule_action/#hijack-dns) |
| QUIC 强制回退 | 迫使兼容流量回退至 TCP。 | 关 | 依赖 sniff，在通用规则前插入 `{protocol:"quic",action:"reject"}`。 | 失去 HTTP/3 优势，必须使用 QUIC 的应用可能失败。 | 依赖 Sniff & DNS Hijack。 | [RFC 9000](https://www.rfc-editor.org/rfc/rfc9000)、[sing-box Protocol](https://sing-box.sagernet.org/configuration/route/rule/#protocol) |
| Tailscale 原生接管 | 由 sing-box 自身建立 Tailnet endpoint。 | 关 | 创建 Tailscale endpoint 与 MagicDNS server，移除标准共存 exclusions，并在通用规则前加入 endpoint 路由；保留 DNS/路由最终策略。完整内容见[脚本](../../web/app/features/files/processors/scripts/sing-box-tailscale-native.js)。 | 未填写 Auth Key 时由目标核心在日志打印交互式登录 URL；端点启动时首次访问可能超时。 | 需要当前配置已有唯一 TUN；与外部共存。 | [sing-box Tailscale endpoint](https://sing-box.sagernet.org/configuration/endpoint/tailscale/)、[Tailscale DNS server](https://sing-box.sagernet.org/configuration/dns/server/tailscale/)、[preferred_by](https://sing-box.sagernet.org/configuration/route/rule/#preferred_by) |
| Tailscale 共存 | 让系统 Tailscale 与 sing-box TUN 共存。 | 关 | 标准 Tailnet 地址绕开 TUN，`ts.net` 使用 MagicDNS；FakeIP 下让 `tailscale.com` 走原默认真实 DNS（条件见下文）。不创建 endpoint，不改最终策略；完整内容见[脚本](../../web/app/features/files/processors/scripts/sing-box-tailscale-external.js)。 | 系统 Tailscale 必须已运行并正确配置；FakeIP 配置的默认 DNS 不符合复用条件时处理器报错。 | 需要当前配置已有唯一 TUN；与原生接管。 | [Tailscale MagicDNS](https://tailscale.com/docs/features/magicdns)、[Tailscale DNS](https://tailscale.com/docs/reference/dns-in-tailscale) |

## Shadowrocket 预设

| 预设 | 动机 | 默认 | 生成行为 | 风险 | 依赖 / 冲突 | 主要来源 |
| --- | --- | --- | --- | --- | --- | --- |
| Tailscale 原生接管 | 使用 Shadowrocket 自身的 TAILSCALE policy。 | 关 | 在第一段规则顶部把 Tailnet 域名及标准 IPv4/IPv6 地址交给内建 `TAILSCALE`。 | Tailscale 的可用性和认证由 Shadowrocket 自身控制。 | 与外部共存冲突。 | [Shadowrocket 社区配置](https://github.com/LOWERTOP/Shadowrocket/blob/5f1916b5897fc59fb7172aca59ae52050a3532fe/lazy.conf) |
| Tailscale 共存 | 把 Tailnet 流量交给当前 LAN 中运行 Tailscale 的路由器；Shadowrocket。 | 关 | 将标准 Tailnet 地址加入 `skip-proxy`、`tun-excluded-routes`，并在规则顶部加入 Tailnet `DIRECT` 规则；DNS 仍由现有配置处理。 | LAN 网关必须拥有对应 Tailnet 路由；MagicDNS 需要用户现有 DNS 或独立 Host 配置支持。离开该网络后继续使用此配置会把 Tailnet 流量旁路到错误的物理网关。 | 与原生接管冲突。 | [Shadowrocket Tailscale 与通用参数](https://github.com/LOWERTOP/Shadowrocket/wiki/) |

## Tailscale 三态与安全边界

三种客户端的状态都是：没有 Tailscale processor 即关闭；外部共存表示独立于
目标客户端的路由所有者负责 Tailnet；原生接管表示目标客户端自身处理 Tailnet。
两种模式互斥且默认都关闭。Mihomo 与 sing-box 的外部所有者是本机系统
Tailscale；Shadowrocket 共存的外部所有者是当前 LAN 网关上的 Tailscale。

Shadowrocket 共存会把 Tailnet 流量旁路到物理网络，不支持同机双 VPN 共存语义；
离开有 Tailnet 路由的 LAN 后，需要切换模式或重新配置。

Mihomo 与 sing-box 原生预设都在 processor `args` 中提供可编辑 `auth_key`。非空值
会随文件配置保存并写入目标核心；空值则省略该字段，由目标核心提供交互式登录。
认证由目标客户端完成。预设不接管全局 DNS 或默认路由，`accept_routes` 默认
为 false。标准 `100.64.0.0/10` 与 `fd7a:115c:a1e0::/48` 不代表用户发布的
subnet routes；额外子网、Exit Node 或其他高级选项可按目标客户端语义编辑副本。

sing-box 的固定目标 v1.13.14 在 route rule 使用 `preferred_by` 匹配 endpoint
提供的 MagicDNS 域名和 allowed IP；该版本的 MagicDNS-only DNS rule 使用 legacy
`ip_accept_any`，不是 v1.14 才支持的 DNS-rule `preferred_by`，并明确关闭
`accept_default_resolvers`，因此普通查询不会把 Tailscale resolver 当作全局
fallback。

sing-box 共存预设识别 `type:"fakeip"` 和 legacy `address:"fakeip"` server；
仅在存在 FakeIP server 时添加 `domain_suffix:["tailscale.com"]` 真实 DNS 规则。
该规则复用原 `dns.final` 指向的 server；未指定 `dns.final` 时按核心语义取原首个
server。所选 server 必须有非空 tag 且为通用真实 DNS；默认引用无效、指向 FakeIP、
MagicDNS 或其他非通用 resolver 时明确报错，不另选上游。两条域名规则均优先于原有
规则，避免通配 FakeIP 抢先匹配；`tailscale.com` 不使用 MagicDNS。参见
[sing-box DNS 默认 server](https://sing-box.sagernet.org/configuration/dns/#final)。

## 可编辑 GitHub 加速快捷项

Mihomo、sing-box 和 Shadowrocket 新建文件默认启用此项。添加时复制一个普通
file-stage inline script；删除该步骤即停止正文改写，结构化规则集字段不迁移。

| `params.args` | 含义 |
| --- | --- |
| `preset_id` | `github-rule-source-mirror`，用于识别内置副本 |
| `replacements` | 有序的 `[source, destination]` 字符串二元组数组 |

例如可在 processor 参数中保存：

```json
{
  "preset_id": "github-rule-source-mirror",
  "replacements": [["https://source.example.com/", "https://mirror.example.com/"]]
}
```

[替换脚本](../../web/app/features/files/processors/scripts/replace-strings.js)按数组顺序
替换全部字面匹配，不使用正则；参数格式错误会使该步骤失败，无匹配则保持正文。
这两个保存参数不接受请求参数覆盖。用户可直接编辑保存的映射，脚本不绑定特定镜像。

当前[默认映射](../../web/app/features/files/processors/github-rule-source-mirror-preset.ts)
把 MetaCubeX、blackmatrix7 和 iab0x00 的已知 GitHub Raw 前缀改写为 jsDelivr；
分发语义见 [jsDelivr GitHub 文档](https://www.jsdelivr.com/documentation#id-github)。
旧 marker `sandrone:file-preset=github-rule-source-rewrite` 仍用于识别已有副本、
避免重复添加，不会自动改写其正文。

新建文件与新添加预设的名称使用当时的界面语言；已保存名称保持原样。
