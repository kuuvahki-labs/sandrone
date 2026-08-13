# 格式与能力参考

本页是节点输入 parser、节点输出 renderer、协议覆盖和字段兼容状态的现行契约。
它不描述完整客户端文件；`mihomo-proxies`、`sing-box-outbounds` 和
`shadowrocket-proxies` 都是节点片段，完整文件由 [FileSpec](file-spec.md)
及对应 typed-file driver 生成。

## 运行时能力发现

`sandrone inspect` 返回当前二进制的轻量运行时摘要：

- `formats.parse`、`formats.render`：已注册输入、输出格式；
- `processors.nodes`、`processors.file`：公开 processor 名称；
- `file_kinds`、`probe.methods`、`probe.backends`：typed-file 与实际 probe
  backend 摘要；
- `store`：是否配置 store；配置时包含 subscription、file 数量。

字段级 format capability 不再内嵌到 inspect。调用方先读取格式索引，再按
`direction` 与 `format` 读取单条详情：

| 入口 | 格式索引 | 单条详情 |
| --- | --- | --- |
| CLI | `sandrone capability formats` | `sandrone capability format <parse|render> <format>` |
| HTTP | `GET /v1/capabilities/formats` | `GET /v1/capabilities/formats/{direction}/{format}` |
| MCP | `sandrone://capabilities/formats` | `sandrone://capabilities/formats/{direction}/{format}` |

索引中的每项包含 `direction`、`format`、`node_types`、`reversible`、
`field_counts`、`revisions` 和协议入口链接；字段级详情才包含 `types`、
`fields`、`lossy` 与 `raw_only`。

面向 Web UI 的功能显示能力通过 `GET /v1/capabilities/ui` 获取。它只返回
后端计算后的稳定 feature 状态，不包含固定的产品导航入口。当前 feature 包括：

- `probe.enabled`：是否有可执行的测活 backend；
- `scheduler.enabled`：服务是否提供调度器能力，与
  `settings.scheduled_refresh.enabled`（用户是否开启定时刷新）不同；
- `core.mihomo`、`core.sing_box`：对应测活 core backend 是否可用。

格式集合当前由前后端同版本发布，继续使用现有格式 capability 和前端固定资源
配置，不纳入 UI feature catalog。未来出现独立发布或动态变化的格式能力时，
再增加对应的 UI feature。

Web 在能力未加载或请求失败时，只隐藏依赖后端能力的控件；固定导航和基础资源
页面仍然可用。feature 的 `enabled` 和 `reason` 由后端计算，前端不根据底层
`probe.backends` 或编译标签自行推导。

`probe_methods` 的封闭集合是 `tcp_connect`、`udp_ntp` 和 `url_test`；
具体可用 core/backend 仍以当前进程返回的 `probe.backends` 为准。

每条 adapter capability 包含：

| 字段 | 含义 |
| --- | --- |
| `format` | canonical 输入或输出格式名 |
| `direction` | `parse` 或 `render` |
| `types` | 声明支持的 canonical `NodeIR.type` |
| `fields` | `supported` 字段 |
| `lossy` | renderer 已知不能完整表达的字段 |
| `raw_only` | parser 只保留到 `NodeIR.raw` 的字段 |
| `reversible` | 是否声明可逆 |

字段项还带 `protocol`、`ir_field`、`status`、`source_ref`，必要时有
`notes`。Mihomo 字段依据固定到 v1.19.25，sing-box 字段依据固定到
v1.13.14；Shadowrocket 字段引用固定的上游 revision。调用方需要精确、可审计
的字段清单时，应读取对应的单条详情，而不是根据目标名称猜测。

## 输入格式

| format | 接受的输入 | 可逆 |
| --- | --- | --- |
| `uri` | 单个分享 URI | 否 |
| `uri-list` | 逐行分享 URI；空行和 `#` 注释行忽略 | 否 |
| `base64` | Base64 包装的 URI 列表 | 否 |
| `mihomo` | `proxies` 列表或单个带 `type` 的 Mihomo YAML/JSON 对象 | 否 |
| `sing-box` | 带 `outbounds`、`endpoints` 的 JSON，或单个带 `type` 的对象 | 否 |
| `json-nodes` | `NodeIR` JSON 数组或 `{ "nodes": [...] }` | 是 |

远程输入省略格式时，只在 `base64`、`uri-list`、`mihomo`、`sing-box`
之间自动检测；`uri` 和 `json-nodes` 不属于自动检测候选。结构化顶层键会让
`sing-box` 或 `mihomo` 优先，其余候选依次尝试，必须实际解析出至少一个节点。

显式 `uri-list`/`base64` 解析会对每行先尝试 URI，并兼容单行 JSON/YAML
节点；远程自动检测使用严格 URI-list，不接受该单行结构化回退。

VMess URI 输入同时接受 legacy `vmess://Base64(JSON)` 与 Discussion #716
风格的 `vmess://UUID@host:port?...` URL；URL 输入只提升当前 `NodeIR` 可表达
的字段，未支持参数保留为 Raw 并产生 warning。VMess URI 输出仍为 legacy
Base64 JSON，不承诺对 #716 的完整或无损往返。

VMess 与 VLESS 的非空用户 ID 遵循上游核心兼容语义：能解析为 UUID 时规范化为
canonical UUID 字符串，否则静默映射为以 nil UUID 为 namespace、原字符串为 name
的 UUIDv5。映射后的 UUID 写入 `NodeIR.UUID`，使所有目标 renderer 使用同一线级
身份；这项语义保持规范化不产生 warning。空用户 ID 仍无效。TUIC 不使用该映射，
其 UUID/password 凭据中的 UUID 必须能被标准 UUID parser 接受。

## 输出格式

| format | 产物 |
| --- | --- |
| `json-nodes` | 缩进的 `NodeIR` JSON 数组 |
| `mihomo-proxies` | 带顶层 `proxies` 的 YAML 节点文档 |
| `sing-box-outbounds` | 带 `outbounds` 和/或 `endpoints` 的 JSON 节点文档 |
| `shadowrocket-proxies` | 完整的 Shadowrocket `[Proxy]` section |
| `uri-list` | 未做 Base64 包装的逐行分享 URI |

`shadowrocket-proxies` 没有同名或其它 Shadowrocket parser；它是纯输出
adapter，不参与输入解析或自动检测。

## 协议矩阵

下表的“是”表示 adapter 声明该节点类型；具体字段仍须服从后续的
`supported`/`lossy`/`raw_only` 规则。

| `NodeIR.type` | URI 输入/输出 | Mihomo 输入/输出 | sing-box 输入/输出 | JSON 输入/输出 | Shadowrocket 输出 |
| --- | --- | --- | --- | --- | --- |
| `ss` | 是 | 是 | 是 | 是 | 是 |
| `ssr` | 是 | 是 | 否 | 是 | 否 |
| `vmess` | 是 | 是 | 是 | 是 | 是 |
| `vless` | 是 | 是 | 是 | 是 | 是 |
| `trojan` | 是 | 是 | 是 | 是 | 是 |
| `hysteria` | 是 | 是 | 是 | 是 | 是 |
| `hysteria2` | 是 | 是 | 是 | 是 | 是 |
| `tuic` | 是 | 是 | 是 | 是 | 是 |
| `mieru` | 是 | 是 | 否 | 是 | 否 |
| `socks` | 是 | 是 | 是 | 是 | 是 |
| `http` | 是 | 是 | 是 | 是 | 是 |
| `wireguard` | 否 | 是 | 是 | 是 | 是 |
| `snell` | 否 | 是 | 否 | 是 | 是 |
| `anytls` | 是 | 是 | 是 | 是 | 否 |

URI parser 接受常用别名，例如 `hy://`、`hy2://`、`socks5://`；Mieru
使用 `mierus://`。这些语法别名最终都规范化为表中的 canonical type。

## 字段状态语义

### `supported`

parser 能将该源字段提升到稳定 `NodeIR` 字段，或 renderer 能按目标 schema
输出该字段。`supported` 不是“任意字段组合都有效”：连接所需字段缺失、目标
语法不允许的认证组合或不受支持的 transport 仍可能使单节点被跳过。

### `raw_only`

parser 识别到字段但没有稳定 IR 抽象时，将原始值保存在 `NodeIR.raw`，并用
`parse_unknown_field` 报告。当前显式声明的重点包括：

- Mihomo VLESS xHTTP padding，以及 VMess/VLESS/Trojan 的部分 gRPC
  私有字段；
- sing-box Hysteria2 的 `realm`、`bbr_profile`、`initial_packet_size`；
- sing-box WireGuard 的部分旧式扁平 endpoint 字段。

`raw` 不是跨目标透传通道。除非目标 renderer 明确消费某个 raw key，未输出的
raw 字段会产生 `render_lossy_field`。`json-nodes` 会原样承载 `raw`，适合诊断
或保存规范化 IR。

少数上游字段在严格值域内与 Sandrone 当前语义等价，因此 parser 会把它们作为
无操作兼容项消费而不产生 `parse_unknown_field`：

- URI TLS 查询参数 `disable_sni` 映射到 `TLS.DisableSNI`，包括显式
  `0`/`false`；`allowInsecure`、`allowinsecure`、`allow_insecure`、
  `allow-insecure`、`skip-cert-verify` 和 `insecure` 均映射到
  `TLS.InsecureSkipVerify`；
- VMess AEAD、VLESS 和 Trojan WebSocket URI 接受正整数 `ed` 与可选非空
  `eh`，分别映射到 `transport.max_early_data` 和
  `transport.early_data_header_name`；缺少 `eh` 时使用
  `Sec-WebSocket-Protocol`，非法或冲突值仍保留到 `raw` 并告警；
- VLESS/TCP 仅接受 `quicSecurity=none`；其它值以及 TCP 上的 `mode`、`spx`
  仍保留到 `raw` 并告警；
- Mihomo Hysteria2 仅接受布尔值 `udp: true`，gRPC 仅接受
  `grpc-mode: gun`；`udp: false`、非法 UDP 值、其它 gRPC mode 和
  `dialer-proxy` 仍保留到 `raw` 并告警；
- sing-box SOCKS 仅接受字符串或数值 `version: 5`；`4`、`4a` 和其它值仍
  保留到 `raw` 并告警。

这些边界只表示当前输入与既有 IR 语义等价，不新增 IR 字段，也不代表 parser
支持同名上游能力的其它取值。

### `lossy`

字段已进入稳定 IR，但目标 renderer 无法等价表达。可选字段通常保留节点并产生
`render_lossy_field`；若缺失的是安全连接所需信息，或现有组合会改变连接语义，
renderer 跳过整个节点并产生 `render_node_skipped`。

只有 `json-nodes` 的 parse/render capability 声明 `reversible: true`。其它
格式即使某个样本没有 warning，也不承诺往返完全等价。

## 共同字段边界

- `name`、`type`、`server`、`port` 是普通代理的共同核心字段；认证字段按协议
  分别使用 `password`、`uuid`、`username`、`cipher` 等。
- `tls.client_fingerprint` 表示 uTLS client fingerprint；
  `tls.fingerprint`、URI `pinSHA256`/`pcs` 表示证书 fingerprint 或 pin，
  两者不可互换。
- `dialer.udp_relay` 是 Sandrone 的显式 UDP 转发策略。sing-box 的
  `network` 是 TCP/UDP 协议选择器，不是等价字段。
- canonical `NodeIR.network` 只接受 `tcp`/`udp`，与
  `NodeIR.transport.type` 相互独立。Mihomo 的 VMess/VLESS/Trojan
  `network` 和 URI 的 `type`/`net`/`transport` 都解析到后者；输出到无法表达
  TCP/UDP 协议选择器的格式时报告有损字段，不借用同名字段改写语义。
- WebSocket early-data、gRPC、xHTTP、Reality、ECH、multiplex、UDP-over-TCP
  都是独立字段族；某个 renderer 支持 transport 基础字段，并不表示它支持该族
  的所有扩展。
- parser 保留的 `NodeIR.warnings`、`source_format`、`tags`、`meta` 和 `raw`
  只有 `json-nodes` 能完整表达；其它 renderer 按字段能力产生 warning。

### Hysteria v1 带宽规范化

可渲染的 canonical Hysteria v1 节点对上传、下载各使用且只使用一种表示：

| 方向 | 显式单位字符串 | 整数 Mbps |
| --- | --- | --- |
| 上传 | `hysteria.up` | `hysteria.up_mbps` |
| 下载 | `hysteria.down` | `hysteria.down_mbps` |

同一方向不能同时填两个字段，也不能两个都为空。`*_mbps` 必须是正整数，且不能
超过 platform `int` 与 `math.MaxUint64 / 1,000,000` 两者中的较小值；这个界限保证
后续换算为 bit/s 不会溢出。字符串必须是“正整数、一个 ASCII 空格、单位”的
canonical 形式，例如 `55 Bps` 或 `640 KBps`。单位区分大小写，使用十进制倍率：

| 单位 | 每单位 bit rate |
| --- | --- |
| `bps`、`Kbps`、`Mbps`、`Gbps`、`Tbps` | `1`、`1,000`、`1,000,000`、`1,000,000,000`、`1,000,000,000,000` bit/s |
| `Bps`、`KBps`、`MBps`、`GBps`、`TBps` | 对应 byte/s，换算为上一行同级单位的 8 倍 bit/s |

能够无损换成整数 Mbps 的显式速率会提升到 `*_mbps`，例如 `125 KBps` 变成
`1` Mbps；不能整除的速率保留显式单位字符串。裸数的含义只由输入格式决定：

| 输入字段 | 裸数假定 | 诊断 |
| --- | --- | --- |
| Mihomo Hysteria `up` / `down` | Mbps | 无；`up-speed` / `down-speed` 是显式 Mbps 兼容字段，只有正整数值才优先；`0` 视为未设置，非法值保留到 `raw` 后回退到原生字段 |
| sing-box Hysteria JSON-number `up` / `down` | Bps | 无；`up_mbps` / `down_mbps` 是显式 Mbps 兼容字段，仅在原生字段为空时使用 |
| Hysteria URI `up` / `down` | Mbps | `parse_implicit_bandwidth_unit`；正整数 `upmbps` / `downmbps` 优先且不需要假定 |
| legacy `json-nodes` 或 inline node | `source_format` 为 sing-box 时是 Bps；为 Mihomo/URI、`json-nodes`、缺失或未知时是 Mbps | 只有 `source_format` 为 `json-nodes`、缺失或未知，且实际采用裸数字符串时产生 `parse_implicit_bandwidth_unit` |

legacy JSON/inline 同一方向同时带字符串和 `*_mbps` 时也按来源消歧：sing-box
provenance 的非空字符串优先；其它 provenance 的正整数 `*_mbps` 优先，没有正整数
兼容值时才采用字符串。

显式单位输入不需要单位假定。不能规范化的源值保留在 `raw` 并产生
`parse_unknown_field`，不会以歧义裸数进入 canonical 字段。

目标输出继续服从同一语义：Mihomo 用带显式单位的原生 `up` / `down`。canonical
层支持 lowercase `bps`，但 locked sing-box rate decoder 不支持这个单位；sing-box
renderer 会把能被 8 整除的值无损转换为 `Bps`（例如 `56 bps` 变成 `7 Bps`），
不能整除时只跳过该节点并产生 `render_node_skipped`。其它可到达 renderer 的显式
单位与整数 Mbps 按目标原生字段输出并保持互斥。URI 和 Shadowrocket 只表达整数
Mbps，因此只转换可精确换算的显式速率，其余方向省略并产生
`render_lossy_field`。Mihomo 和 sing-box renderer 遇到缺方向、双字段、超出安全
Mbps 界限或其它无效 canonical 速率时只跳过该节点，不会使仍可渲染的同批节点
失败。

## 各 renderer 的关键边界

### `mihomo-proxies`

- 覆盖矩阵中的全部 14 种类型，包括 SSR、Snell、Mieru 与 AnyTLS。
- 支持 Mihomo 的 TLS/Reality、WS early-data、gRPC 和类型化 xHTTP 主字段；
  未建模的实现私有子字段留在 `raw`。
- `ss`、`vmess`、`vless`、`trojan`、`mieru`、`socks` 和 WireGuard
  可表达 `dialer.udp_relay`；其它协议不能据此推断有等价开关。
- 已声明的典型有损项包括 ECH DNS/force-query 扩展、multiplex、目标不支持的
  transport type、Hysteria/Hysteria2 QUIC 调优、Hysteria2 Mbps 字段、
  TUIC zero-RTT/heartbeat，以及 HTTP path。

### `sing-box-outbounds`

- 不声明 SSR、Snell 或 Mieru；WireGuard 输出到顶层 `endpoints`，其它协议输出到
  `outbounds`。
- 支持 AnyTLS，以及 sing-box 的 TLS、transport、multiplex、network、
  UDP-over-TCP 等协议适用字段。
- 没有通用 `dialer.udp_relay` 等价项；该字段不能用 `network` 代替。
- 已声明的典型有损项包括证书 fingerprint、ECH DNS/force-query 扩展、
  VLESS `encryption`、目标不支持的 transport type、Hysteria `protocol`
  与 QUIC 调优、Hysteria2 字符串速率及 `bbr_profile`/`realm`/`cwnd`/
  `udp_mtu`、TUIC token/reduce-RTT/UDP-over-stream version、SOCKS TLS。

### `uri-list`

- URI profile 不覆盖 WireGuard 与 Snell；这两类节点会被跳过，同时报告
  `uri_profile_unsupported` 和 `render_node_skipped`。
- URI 能稳定表达每种协议的分享字段子集，不等价于客户端完整 schema。
- 常见有损字段包括 TFO、UDP relay、multiplex、UDP-over-TCP、额外 headers、
  path、network、部分 TLS/ECH/Reality 信息、transport 扩展和 QUIC 调优。
- Hysteria v1 的 URI `obfs` 是 mode，`obfsParam` 是密码；Mihomo/sing-box
  的单字段 `obfs` 规范化到密码字段，不能把两种含义混写。

### `shadowrocket-proxies`

- 输出完整 `[Proxy]` section，支持 SS、VMess、VLESS、Trojan、Hysteria、
  Hysteria2、TUIC uuid/password、HTTP/HTTPS、SOCKS5/SOCKS5-TLS、单 peer
  WireGuard 和 Snell v2。
- SSR、Mieru、AnyTLS，以及不符合上述语法约束的协议变体会被跳过。
- WireGuard 必须能归约为一个有效 peer；Snell 只接受 v2 语义，且不接受
  ShadowTLS。Hysteria v1 只接受文档化的 `udp`、`wechat-video`、`faketcp`
  protocol。
- 连接关键的未知 transport、TLS identity、Reality/ECH、认证变体不能安全降级，
  因而跳过节点；可选且不影响连接成立的字段才以 `render_lossy_field` 保留节点。
- 节点名会清理 CR/LF，并替换会破坏 INI 的 `,`、`=`、行首
  `#`/`;`/`[`；空名、重复名和与内建策略冲突的名称也会规范化。任何名称变化
  都进入 report。

## 部分成功与 warning 策略

列表 parser 和 renderer 以节点为隔离边界：

- URI-list 的坏行产生 `parse_line_failed` 或 `parse_line_skipped`；
- Mihomo 的坏列表项产生 `parse_proxy_skipped`；
- sing-box 的坏节点项产生 `parse_outbound_skipped`，明确的非节点 outbound
  会静默忽略；
- 未提升字段产生 `parse_unknown_field`；
- 未表达字段产生 `render_lossy_field`；
- 不可渲染节点产生 `render_node_skipped`。

只要仍有有效节点，操作可以成功并在 report 返回 warnings。非空输入最终没有
任何可解析或可渲染节点时，操作返回 `parse_failed` 或 `render_failed`。
warning 的结构、report 聚合和原始输入敏感性见
[错误与诊断参考](errors.md)。
