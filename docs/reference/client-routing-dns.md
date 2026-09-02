# 客户端路由、DNS 与 IP 兜底

本页是 Sandrone Web 新建 Mihomo、sing-box 和 Shadowrocket 文件时，规则模板、
DNS 路径与 IP 兜底语义的现行契约。它描述默认生成结果，不承诺自动改写已经保存
的 source、settings 或 processors。

## 隐私边界

默认目标不是“任何解析器都看不到域名”，而是避免普通公网域名通过明文 DNS 或
不受控的系统 fallback 离开客户端：

- canonical 中国域名可以交给直连的中国加密 DNS；DNS 服务商仍会知道查询名，
  但本地网络只能看到加密 DNS 连接。这是为国内 CDN 和可用性保留的显式取舍。
- 已知境外域名和应用自带的已知 DoH/DoT 流量优先进入代理保护的路径。
- 未知域名在 Mihomo 中使用不绑定路由规则的境外加密 DNS，sing-box 使用经代理
  转发的境外加密 DNS；Shadowrocket 为执行中国 IP 兜底，可能先使用中国加密 DNS，
  具体限制见下文。
- LAN、私有域名可以使用本地/系统 resolver。
- Shadowrocket 的 `*.apple.com`、`*.icloud.com = server:system` 是有意保留的
  iOS 兼容例外。系统 resolver 是否使用明文传输由当前网络决定；该例外不扩展成
  全局 system fallback。

因此，“国内 DNS 可见中国域名”在此模型中可接受；“系统 DNS 可见任意未知或境外
域名”不可作为默认行为。客户端、操作系统或应用绕过 TUN、使用未被规则集识别的
自定义加密 DNS，仍可能形成契约之外的旁路。

## canonical 规则顺序

生成模板按下列层次匹配：

1. 目标端口 `853` 和 `category-doh` 进入主代理策略；
2. 广告和私有域名；
3. 服务域名按具体子服务优先、宽泛父服务在后的顺序匹配；
4. `cn` 域名和 `geolocation-!cn` 域名；
5. 解析边界；
6. 专用服务 IP、私有 IP；
7. 中国 IP 兜底；
8. 最终策略。

服务策略组完整拥有其命名服务的路由控制权。模板不在服务规则之前加入绕过策略组
的国内直连例外；需要直连时由对应服务策略组选择 `DIRECT`。Microsoft 和 Apple
策略组默认以 `DIRECT` 为首选项，因此默认路径仍然直连，但用户切换策略后会作用于
完整服务。

同一策略下优先使用上游维护的聚合集合：AI 使用 `category-ai-!cn`，Microsoft 和
Apple 分别直接使用其父集合，Meta 使用 `meta`，新闻使用 `category-media`。这能
避免重复下载同义 provider。不同策略间即使存在包含关系，具体子服务仍必须先于
宽泛父列表，例如 Xbox 先于 Microsoft、Apple TV+ 先于 Apple、Hulu 先于 Disney、
npm 先于 GitHub；这样各子服务策略组不会被父列表提前截流。

三个客户端的标准和完整模板都提供 `Static/CDN Resources`；静态/CDN 域名使用
Sukka 的 Mihomo text、sing-box source JSON，以及 Shadowrocket 可解析的
DOMAIN-SET/RULE-SET 文本，并位于私有网络之后、普通服务规则之前。

Mihomo 和 Shadowrocket 还输出原生 `Fallback`，并让主代理组可以选择它；sing-box
没有有序 fallback outbound，因此不生成一个实际只是普通 selector 的同名替代品。
Mihomo 与 Shadowrocket 的广告组同时提供 `REJECT`、`REJECT-DROP` 和 `DIRECT`；
sing-box 使用其原生 `block` outbound，不伪造丢包拒绝策略。

Mihomo 和 sing-box 分别使用 MetaCubeX 的 `cn-ip` MRS/SRS 规则集；
Shadowrocket 使用 `GEOIP,CN`。三者都是域名规则之后的**解析型兜底**：它们能
覆盖没有收录在域名规则集、但解析结果属于中国 IP 的主机。它们不能替代域名规则，
也受 IP 数据库更新及时性、CDN 地址和 anycast 归属准确性的限制。

## `no-resolve` 如何保证

`no-resolve` 只控制“评估 IP 类规则时是否为了当前域名额外发起解析”，不是 DNS
加密或防泄漏开关：

- Mihomo 的专用 `*-ip` 规则带 `no-resolve`，但 `cn-ip` 故意不带；
- Shadowrocket 的私有/专用 IP 规则可带 `no-resolve`，但最终
  `GEOIP,CN` 故意不带；
- sing-box 不使用这个字符串参数，而是在所有域名规则后插入
  `{ "action": "resolve" }`，再评估 IP 规则集。

因此，请求以域名进入时，专用 IP 规则不会抢先触发本地解析；只有到达最终中国 IP
兜底前才执行受控解析。请求本来就是 IP 地址时，`no-resolve` 不妨碍 IP 规则匹配。

## TUN 与 Fake-IP 能力边界

三种产物只对齐路由和 DNS 的结果语义，不强求客户端字段相同：

- Mihomo base 不含 `tun`；用户显式选择 TUN processor 后才生成并开启完整 TUN
  块。base 默认使用 fake-IP，并由基础 `fake-ip-filter` 和三个互斥的可选扩展控制
  哪些域名返回真实 IP。
- sing-box base 直接包含 TUN inbound 和仅监听本机的 mixed inbound。sing-box 没有
  Mihomo 式的 `tun.enable` 开关；删除这个 inbound 会把产物改成仅供显式设置系统
  代理的本地端口配置。当前 DNS 没有配置 FakeIP server，所有查询都返回上游真实
  IP，因此不需要再生成 fake-IP 例外列表。
- Shadowrocket 是否以 TUN 接管流量由 App 的代理类型或 `compatibility-mode` 决定；
  `tun-excluded-routes` 和 `hijack-dns` 只描述启用后的处理方式。其 TUN DNS 支持
  fake IP，也可用 `always-real-ip` 指定真实 IP 例外；当前 base 不默认扩大该列表，
  Apple/iCloud 的系统解析例外仍由 `[Host]` 承担。

所以，“Mihomo 默认不开 TUN”不能机械转换成删除 sing-box TUN inbound，也不能转换
成删除 Shadowrocket 的 TUN 旁路参数；“Mihomo 有 fake-IP-filter”也不意味着另外两端
必须生成同名或等长列表。

## 各客户端 DNS 路径

### Mihomo

- bootstrap、节点域名和直连域名使用直连的阿里 IP DoH；不配置明文 bootstrap；
- `rule-set:cn` 使用同一组中国 DoH，`geosite:private` 是唯一常规 system 例外；
- 默认 resolver 是不绑定路由规则的 Cloudflare/Google DoH；
- `category-doh` 与端口 `853` 先进入主代理策略；
- 显式选择 TUN processor 后，它开启 `strict-route` 并劫持 TCP/UDP 53，同时把 mDNS 目标
  `224.0.0.251/32`、`ff02::fb/128` 排除在 TUN 自动路由之外。

这里的 `#DIRECT` 明确固定中国 DNS 请求自身的出站路径。默认境外 DoH URL 不携带
`#RULES`，base 也不启用 `respect-rules`；这避免与 `prefer-h3: true` 组成上游不建议的
组合，但不保证境外 DoH 连接本身经过代理。查询内容仍由 HTTPS 加密，不会交给中国
或系统 resolver。`fake-ip-filter` 只决定返回真实 IP
还是 fake IP，不决定查询应交给哪个 resolver。fake-IP 的独立边界见
[Mihomo fake-IP 默认与边界](mihomo-fake-ip.md)。

### sing-box

- `private` 交给 `dns-local`；
- `cn` 交给直连的中国 HTTPS DNS；
- 其余查询交给通过主代理 detour 的境外 HTTPS DNS；
- `default_domain_resolver` 使用中国加密 DNS，避免 detour 建立前依赖全局 system
  resolver；
- TUN 启用 `strict_route`，默认 processor 以逻辑规则劫持 protocol DNS 或目标
  端口 53；`224.0.0.251/32`、`ff02::fb/128` 则在进入该识别规则前绕开 TUN，
  让 mDNS 留在本地链路，同时保留对其他非标准端口明文 DNS 的协议识别。

`strict_route` 是配置层的跨平台默认；sing-box for Android 的 VpnService 当前不实现
该选项，不能把桌面端的 fail-closed 行为直接视为 Android 保证。

sing-box 新版不依赖已弃用的 legacy GeoIP 数据库；中国 IP 兜底由远程 IP-CIDR
SRS 规则集完成。

### Shadowrocket

- 主 resolver 是中国 IP DoH；节点域名也使用这组 DoH；
- fallback 是带 `#proxy` 的 Cloudflare/Google IP DoH，不回退到 system；
- `hijack-dns = :53` 劫持任意目标的 53 端口；
- Apple/iCloud 的 Host system 覆盖和 localhost 映射继续保留。

Shadowrocket 当前配置语义不能像 sing-box 那样按 canonical rule-set 为每个普通
查询选择 resolver。已被境外域名规则命中的连接可由代理端解析；为了执行末尾的
`GEOIP,CN`，未命中域名规则的未知主机则可能先由中国 DoH 解析。fallback 只在
主查询失败或超时时接管。这是该客户端默认在国内解析成功率、iOS 兼容性和
resolver 最小暴露之间的折中，不应描述为严格的按域名双路 DNS。

## 上游语义依据

- Mihomo：[DNS](https://wiki.metacubex.one/en/config/dns/)、
  [规则](https://wiki.metacubex.one/en/config/rules/)与
  [TUN](https://wiki.metacubex.one/en/config/inbound/tun/)；
- sing-box：[DNS](https://sing-box.sagernet.org/configuration/dns/)、
  [规则动作](https://sing-box.sagernet.org/configuration/route/rule_action/)、
  [TUN](https://sing-box.sagernet.org/configuration/inbound/tun/)与
  [废弃项](https://sing-box.sagernet.org/deprecated/)；
- Shadowrocket：[LOWERTOP 配置与手册](https://github.com/LOWERTOP/Shadowrocket)；
- DoH 传输语义：[RFC 8484](https://www.rfc-editor.org/rfc/rfc8484)。

## 验证最终文件

保存后应检查最终渲染结果，而不是只看模板或 base：

- 所有域名规则位于 IP 规则之前；
- 中国 IP 兜底位于境外域名规则之后，且没有 `no-resolve`；
- 没有明文公网 nameserver 或全局 system fallback；
- 应用 DoH/DoT 规则位于普通服务规则之前；
- 若选择了 TUN，其 DNS hijack 与路由配置仍存在且顺序正确。

自定义 source、手动删除模板 rule set、改变 processor 顺序或使用全局路由模式，
都可能改变上述结果。
