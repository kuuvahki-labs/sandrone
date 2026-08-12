# 社区配置优化预设设计

## 背景

Mihomo、sing-box 与 Shadowrocket 社区配置通常同时承担四类工作：

1. 提供一份可以直接启动的基础配置；
2. 针对特定平台修补 TUN、DNS、UDP 或 IPv6 兼容问题；
3. 通过阻断 STUN、QUIC 或其它协议换取隐私或连接稳定性；
4. 集成 Tailscale 等额外网络端点。

这些配置解决的往往是真实问题，但整份复制会把平台假设、个人偏好、过时字段、
互相冲突的规则和未经说明的风险一起带入 Sandrone。当前项目已经具备保守的
基础配置、结构化分组和规则编辑、按顺序执行的 file-stage processor，以及
YAML、JSON、INI override 能力。本设计将社区实践拆成可审阅的受管场景预设，
只纳入有明确使用动机、可描述风险且目标客户端能够表达的部分。

## 目标

- 让用户按场景选择社区优化，而不是复制整套第三方配置。
- 复用现有文件处理器和 FileDriver 边界，不增加新的后端业务模型。
- 每项预设明确说明适用客户端、默认状态、使用动机、风险和冲突。
- 保留用户的处理器顺序、自定义内容和现有文件，不执行隐藏迁移。
- 为 Mihomo、sing-box 和 Shadowrocket 生成各自原生、可编辑的配置。
- 支持 Tailscale 外部共存与原生接管，同时不接触 Auth Key。
- 在 Web 文件预览中显示固定目标核心版本，帮助用户理解兼容性边界。

## 非目标

- 不复制或维护多套完整社区配置模板。
- 不把每个核心参数做成结构化高级设置表单。
- 不扩展 canonical NodeIR 表达 Tailscale。
- 不接收、保存或生成 Tailscale Auth Key。
- 不支持 Headscale 或自定义 control URL。
- 不新增最终静态语义审计。
- 不处理多订阅聚合失败策略。
- 不自动迁移、重写或回填已有文件和已有分组。

## 固定兼容性边界

预设只面向项目当前声明的固定目标：

- Mihomo 字段以 v1.19.25 契约为准；
- sing-box 字段以 v1.13.14 契约为准；
- Shadowrocket 字段以项目能力参考中固定的上游 revision 为准。

目标版本显示在 Web 文件预览中，不提供版本选择器，也不根据用户输入切换渲染
行为。预览从现有 /v1/inspect 能力对象中读取目标 renderer 的 source revision，
不在 Web 预设目录中另行复制，也不把版本持久化到 FileSpec。

## 选择的总体方案

采用“安全基础配置 + 受管场景预设”：

- 基础配置只保留普适且风险较低的默认值；
- 默认启用项只在新建文件时加入；
- 有明显行为代价的实践一律默认关闭；
- 每个 FileDriver 只提供目标客户端真正支持的预设；
- Tailscale 不做专属表单，使用现有可编辑处理器。

没有采用“全部参数结构化设置”，因为它会扩大表单、迁移和版本兼容成本；没有
采用“整套场景模板”，因为模板之间会复制基础配置、规则和默认值并持续漂移。

## 架构

### 预设目录

在现有 FileDriver 处理器扩展点上增加最小的预设描述能力。每个预设描述：

- 稳定 ID；
- 分类；
- 用户可见名称和说明；
- 支持的文件 kind；
- 新文件默认状态；
- 依赖预设；
- 冲突预设；
- 风险提示；
- 处理器构造与识别逻辑。

预设按“隐私”“网络兼容”“平台”“Tailscale”分组展示。具体 driver 拥有其
预设内容；共享 core 只定义纯类型和依赖、冲突计算，不依赖 React 或具体 driver。

### 两类处理器

仅修改设置或结构字段、且 merge 能精确定位而不会覆盖用户数组的预设生成普通
merge 处理器：

- Mihomo 使用 yaml_override；
- sing-box 使用 json_override；
- Shadowrocket 使用 ini_override。

需要把规则放在特定语义层之间，或需要安全修改数组中某个具名元素的预设，生成
现有 file-stage inline script 处理器。后者包括 sing-box 的 TUN inbound、DNS
server 和 Tailscale endpoint 等数组结构；使用 JSON override 整体替换这些数组会
吞掉用户的其它元素。脚本使用 Sandrone sandbox 中已有的 YAML/INI API 和内建
JSON，不引入新的处理器类型。
采用脚本是因为 override 数组操作只能前插、后插或整体替换，无法安全实现：

1. 用户规则；
2. 场景预设规则；
3. 私网、地区等通用规则；
4. MATCH 或 FINAL。

### 规则插入

规则脚本按目标 driver 的 canonical 模板签名寻找通用规则层或最终兜底：

1. 保留已存在的用户规则和服务专用规则；
2. 在首个已知通用规则之前插入预设规则；
3. 没有通用层但存在最终兜底时，在最终兜底之前插入；
4. 后续处理器在同一边界继续追加，因此处理器声明顺序就是预设规则顺序；
5. 无法识别安全边界时停止并返回明确错误，不猜测位置。

脚本不得整体替换用户规则，不得静默改写 MATCH 或 FINAL。

### 受管识别与用户所有权

inline script 使用稳定 script ID 识别。YAML 和 INI merge 可使用不会进入实际语义
的注释标记；JSON merge 使用目标结构签名识别。

添加预设后，处理器是用户文件中的普通副本，可以自由编辑。用户修改到无法继续
明确识别时，该处理器降为普通自定义处理器：

- Sandrone 不再将其视为受管预设；
- 冲突处理不得自动删除它；
- 未来版本不得自动更新其内容。

这保证冲突自动处理只作用于 Sandrone 明确认识的处理器，不吞掉用户修改。

## 用户流程

1. 用户在现有文件处理器菜单中选择一个分类后的预设。
2. 纯逻辑规划函数识别当前受管预设并计算依赖、冲突移除和新增处理器。
3. UI 在一次状态更新中完成替换，保留其它处理器的相对顺序。
4. UI 当场显示风险说明以及自动关闭了哪些冲突预设。
5. 文件被标记为 dirty，保存前可继续编辑生成的 merge 或 inline script。
6. 预览按处理器声明顺序执行并显示固定目标版本。

不支持的客户端不展示相应选项。导入的未知或未来处理器继续按现有 opaque 规则
原样保留。

## 新文件默认变化

### 全部客户端

- 新文件默认加入“传统 NTP 直连”规则处理器；
- 仅匹配 UDP 目标端口 123；
- 用户规则仍优先于该规则；
- 不把其它 TCP 或 UDP 流量扩大为直连；
- 预设说明明确提示 UDP/123 会绕过代理并暴露直连出口，且该预设默认启用。

### Mihomo

- 保持 allow-lan 为 true；
- lan-allowed-ips 保持 RFC1918 IPv4 与 fc00::/7 ULA 白名单；
- 启用 Geo 数据自动更新；
- 更新周期为 24 小时；
- 使用 Mihomo 内置 Geo 来源，不新增 geox-url；
- 新建 url-test 分组写入 tolerance: 50；
- 不新增 tolerance 编辑字段；
- 不为已有分组补写 tolerance。

### Shadowrocket

- close-if-proxy-chain-missing 设为 true，代理链中转丢失时拒绝连接；
- dns-direct-fallback-proxy 设为 false；
- udp-policy-not-supported-behaviour 保持 REJECT；
- block-quic 保持 all-proxy；
- ipv6 保持 true，关闭 IPv6 由可选预设提供。

### sing-box

- selector 与 urltest 分组支持 interrupt_exist_connections；
- 新建分组的开关默认关闭，序列化时省略 false；
- 读取已有配置时保留原值；
- urltest 开启此项时提示自动切换会中断现有连接。

## 场景预设目录

### 隐私

#### Shadowrocket WebRTC 隐私

默认关闭。写入 stun-response-ip 和 stun-response-ipv6，使用当前模板已经说明的
虚假地址示例 1.1.1.1 与 ::1；生成后仍可直接编辑处理器修改。

社区动机是避免应用通过 STUN 获得真实公网出口地址。代价是 WebRTC、语音通话、
视频会议和 P2P 可能降级或失效。

#### Mihomo STUN 端口阻断

默认关闭。阻断 UDP 目标端口 3478 和 5349。

社区动机同样是减少公网出口地址暴露。该方案只是常见端口近似，不承诺覆盖使用
其它端口的 STUN；同时可能破坏 WebRTC、语音、视频会议、P2P 和 Tailscale 打洞。

#### sing-box STUN 阻断

默认关闭。依赖现有 sniff 行为，拒绝识别为 STUN 的连接。

固定警告文案：

> 阻止应用通过 STUN 获取公网出口地址；可能导致 WebRTC、语音通话、视频会议或
> P2P 连接降级或失效。默认关闭。

### 网络兼容

#### QUIC 强制回退

默认关闭，仅提供给 Mihomo 和 sing-box：

- Mihomo 阻断 UDP 目标端口 443；
- sing-box 依赖 sniff，并阻断识别为 QUIC 的流量。

社区使用它规避质量较差或被限速的 UDP 路径，迫使 HTTP/3 回退至 TCP。代价是
失去 QUIC 性能，依赖 UDP/443 的应用可能失败。Shadowrocket 不增加相同预设，
因为基础配置已经使用 block-quic=all-proxy。

#### sing-box IPv4-only

默认关闭。DNS strategy 改为 ipv4_only，并从 TUN address 中移除 IPv6 前缀。

用于修复 IPv6 路由、DNS 或代理节点不完整的网络。代价是 IPv6-only 资源不可达，
且不能把它描述为通用性能优化。

#### Shadowrocket 关闭 IPv6

默认关闭。写入 ipv6=false 与 prefer-ipv6=false。

该预设不承诺代理节点自身的底层连接一定不会使用 IPv6，只控制配置可表达的
Shadowrocket IPv6 行为。

#### UDP/P2P 兼容

默认关闭，提供给 Mihomo 和 sing-box。为 TUN 启用
Endpoint-Independent NAT。

社区使用它改善游戏、语音、P2P 和部分打洞场景。核心文档说明此选项可能轻微
降低性能；sing-box 中它只对 gVisor 栈产生额外效果，其它栈本身已使用
Endpoint-Independent NAT。该预设与 STUN 隐私阻断互斥。

#### Shadowrocket UDP 不支持回退

默认关闭。将 udp-policy-not-supported-behaviour 从 REJECT 改为 DIRECT。

用于兼容不支持 UDP 转发的节点，但会让匹配流量绕过代理，必须明确提示真实出口
地址、运营商路径和本地 DNS 可能暴露。

#### Shadowrocket 受限网络 DNS 回退

默认关闭。将 dns-direct-fallback-proxy 改为 true。

用于直连 DNS 在受限网络中失败的情况。代价是本应直连解析的域名可能改经代理，
因此不作为基础默认值。

### 平台

#### Linux/OpenWrt TUN 加速

默认关闭，提供给 Mihomo 和 sing-box，并自动补齐各自 TUN 与 auto-route 依赖：

- Mihomo 开启 tun.auto-redirect；
- sing-box 开启 TUN auto_redirect；
- Mihomo 显式使用 find-process-mode: strict。

Mihomo 的 strict 是自动模式：只在规则需要进程或 UID 信息时查询，不像 always
那样对所有连接查询，也不像 off 那样禁用相关规则。路由器转发的局域网流量通常
无法对应到路由器本地进程，因此进程或 UID 规则仍可能无法匹配。

该预设只面向 Linux/OpenWrt；其它平台不展示为推荐选项。核心文档说明
auto-redirect 依赖 auto-route，并可能与 routing mark 类配置冲突。

#### sing-box MPTCP 直连

默认关闭，依赖 Linux/OpenWrt TUN 加速。写入 exclude_mptcp=true。

sing-box 无法透明代理 MPTCP；默认拒绝可以避免错误，开启该预设则让 MPTCP 绕过
代理直接连接。必须提示由此产生的策略绕过和出口暴露。

#### Windows VM/多网卡兼容

默认关闭，提供给 Mihomo 和 sing-box。将 strict_route 设为 false。

用于 VirtualBox、虚拟机、多网卡或特殊路由与严格路由冲突的场景。代价是 Windows
多宿主 DNS 防泄漏和不支持流量 fail-closed 能力下降。

## Tailscale

### 模式

Tailscale 通过处理器表达三种状态：

- 没有 Tailscale 处理器：关闭；
- 外部共存处理器：系统中的独立 Tailscale 负责隧道；
- 原生接管处理器：目标客户端自身创建 Tailscale 端点或使用 TAILSCALE policy。

外部共存与原生接管互斥，默认均不添加。界面优先展示原生接管，但不会默认启用。
每个 driver 只提供目标客户端能够实际支持的模式。

### 外部共存

Mihomo 和 sing-box 的外部共存处理器：

- 从代理 TUN 自动路由中排除 100.64.0.0/10；
- 排除 fd7a:115c:a1e0::/48；
- 只为 Tailnet 域名配置 MagicDNS；
- 不创建原生 Tailscale endpoint。

原生接管不得同时保留这些 route exclusion，否则会把应该交给原生端点的流量
绕过代理核心。

### 原生接管

Mihomo 生成一个普通 tailscale proxy：

- 使用默认 state directory；
- 使用默认 hostname；
- ephemeral=false；
- udp=true；
- accept-routes=false；
- 不写 exit-node；
- 不写 auth-key；
- 不写 control-url。

sing-box 生成一个文件局部 Tailscale endpoint：

- 使用默认 state_directory；
- 使用默认 hostname；
- ephemeral=false；
- accept_routes=false；
- 不写 exit_node；
- 不写 auth_key；
- 不写 control_url。

Shadowrocket 只生成目标为 TAILSCALE 的规则。Tailscale 模块的启用和登录留在
Shadowrocket 自身，不增加 Sandrone 表单，也不显示模块启用提醒。

### 登录与控制面

Sandrone 不接收、不保存、不生成 Auth Key，也不提供登录 URL、二维码或其它登录
交互。生成配置省略密钥字段，用户可以通过脚本或其它方式补充。

只使用官方 Tailscale 控制面默认值。即使核心支持 control URL，也不暴露
Headscale 或自定义控制服务器预设。

### MagicDNS

MagicDNS 只处理 Tailnet 名称，不作为普通查询的全局 DNS：

- Mihomo 只将 ts.net 相关名称交给 100.100.100.100，并排除对应 fake IP；
- sing-box 使用固定目标 v1.13.14 官方提供的 MagicDNS-only Tailscale DNS 模式：
  DNS server 关闭默认 resolver，并使用该版本的 `ip_accept_any` DNS 规则写法；
  不使用 v1.14.0 才加入 DNS 规则的 `preferred_by`，也不接受默认 resolver 作为
  普通查询 fallback；
- Shadowrocket 按其 TAILSCALE policy 能力生成规则，不新增全局 DNS 接管。

### 子网路由和 Exit Node

accept routes 默认关闭。用户直接编辑原生处理器开启：

- sing-box 使用 preferred_by 匹配 Tailscale endpoint 动态提供的域名和 allowed
  IP 路由；
- Mihomo 与 Shadowrocket 先提供 100.64.0.0/10 和
  fd7a:115c:a1e0::/48，用户可在处理器中补充自定义 CIDR。

Exit Node 默认不设置。用户自行编辑原生处理器或客户端模块选择 Exit Node。
Tailscale 只成为普通可选路由目标；预设不得把 MATCH 或 FINAL 自动改为
Tailscale。

Tailscale 规则顺序固定为：

1. 用户规则；
2. Tailscale 规则；
3. 私网、地区等通用规则；
4. MATCH 或 FINAL。

## 依赖与冲突

### 自动补齐依赖

- sing-box QUIC 强制回退补齐 sniff；
- Linux/OpenWrt TUN 加速补齐 TUN 和 auto-route；
- sing-box MPTCP 直连补齐 Linux/OpenWrt TUN 加速；
- Tailscale 原生接管补齐目标 driver 所需的 DNS 和规则处理器。

### 自动替换冲突

- Tailscale 外部共存与原生接管互相替换；
- STUN 隐私与 UDP/P2P 兼容互相替换；
- 启用任一 Tailscale 模式会关闭受管 STUN 阻断，避免破坏 Tailscale 打洞；
- 启用 STUN 阻断会关闭受管 Tailscale 模式，并明确提示。

自动处理发生在用户明确添加新预设的同一次操作中。UI 必须列出被关闭的预设。
不得后台修改，不得删除无法明确识别的自定义处理器。

## 现有文件与迁移

- 基础配置变化只影响之后创建的文件；
- 默认 NTP 处理器只加入之后创建的文件；
- tolerance: 50 只影响之后新建的 Mihomo url-test 分组；
- 已有 interrupt_exist_connections 原值保持不变；
- 已有文件不显示强制迁移，不自动重写；
- 用户可以手动添加预设或重新生成基础配置；
- 已添加预设不会随 Sandrone 升级自动刷新。

## 失败处理

- JSON、YAML、INI 或 inline script 无效时阻止预览；
- merge 和 script 失败必须保持输入文件原样，不输出半成品；
- 找不到安全规则锚点时返回包含预设名称和目标 kind 的可操作错误；
- 依赖无法满足时不添加任何部分处理器；
- 冲突替换必须作为单次原子状态变化完成；
- 不支持的客户端不生成近似或伪造配置；
- 预览中的同类错误和警告按现有聚合策略展示，避免逐节点刷屏；
- 不静默 fallback 到 DIRECT、其它代理或其它 Tailscale 模式。

## UI 设计

继续使用现有文件处理器列表和底部添加入口，不新增顶层设置页或创建向导。

处理器选择菜单按四类分组。选择预设后：

- 立即显示该预设的风险说明；
- 显示自动补齐的依赖；
- 显示自动关闭的冲突预设；
- 展开后仍使用现有 merge 或 script 编辑器；
- 保存、预览和 dirty 状态沿用现有文件表单行为。

Tailscale 不增加专属字段、Auth Key 输入、登录按钮或额外弹窗。

## 交付切片

这些改动共享同一预设目录和处理器规划器，但按依赖关系分四个可独立验证的切片：

1. 预设描述、分类、依赖/冲突规划、版本显示和新文件基础默认值；
2. 只修改配置字段的 merge 预设；
3. 有序规则 inline script、默认 NTP、STUN 与 QUIC 预设；
4. Tailscale 模式、MagicDNS、规则顺序和跨预设冲突。

每个切片完成后都保持可构建、可预览；不得先加入 UI 选项再留空实现。四个切片
属于同一产品设计，后续实施计划可以进一步拆成按 driver 验证的任务。

## 验证

### 纯逻辑测试

- 每个 kind 的可见预设能力矩阵；
- 分类、依赖和冲突闭包；
- 重复添加幂等；
- 单次冲突替换；
- 非冲突处理器相对顺序不变；
- 无法识别的用户处理器不被删除；
- 预设副本不会因目录变化被自动迁移。

### Driver 测试

- 每个预设产生预期的 yaml_override、json_override、ini_override 或 inline
  script；
- 新文件基础默认值；
- Mihomo 新 url-test tolerance；
- sing-box interrupt_exist_connections 保留和默认行为；
- Tailscale 三个目标的能力差异；
- 配置和处理器中不存在 Auth Key 或 control URL；
- 规则脚本在安全锚点插入，并在未知结构上失败。

### Service 测试

至少覆盖每个目标一次完整生成：

1. 基础文件；
2. 注入节点；
3. 结构化规则；
4. 按声明顺序执行预设；
5. 验证最终语义和规则顺序。

重点断言 NTP 只匹配 UDP/123、Tailscale 不改写最终规则、原生模式不保留外部
route exclusion、STUN 与 Tailscale 不共存，以及失败时原文不变。

### UI 测试

业务规则优先放入 node 环境纯逻辑测试。只保留一个确实依赖 React 交互的 focused
测试，证明选择新预设时冲突项在一次更新中移除且提示可见；不为每个预设复制
jsdom 用例，不新增大范围 E2E 套件。

### 门禁

先运行相关 driver、processor model 和 service 窄测，再运行：

- Web typecheck；
- Web lint；
- 与改动直接相关的 Web node/UI 测试；
- 仓库根 make check。

## 文档

新增一份 canonical 预设参考，逐项记录：

- 使用动机；
- 支持客户端；
- 默认状态；
- 生成行为；
- 风险；
- 依赖和冲突；
- 主要社区或官方来源。

docs/README.md、Mihomo Fake-IP/Tailscale 参考及故障排查页面只链接到 canonical
说明，不复制完整矩阵。长期文档只描述最终行为，不保留执行清单。

## 明确拒绝或延期

以下社区实践不进入本轮产品：

- Mihomo TCP Keep-Alive 预设：社区取值互相冲突，平台行为不同，Android 还有
  强制默认，统一预设会制造错误确定性；
- Shadowrocket always-real-ip 社区域名列表：来源分歧大，会扩大本地 DNS 暴露；
- Headscale 和自定义 control URL；
- Tailscale Auth Key、登录交互和专属表单；
- 高级 Tailscale hostname、state path、ephemeral、advertise、relay、SSH 和
  MTU 字段；
- 多订阅集合失败策略；
- 新的最终静态语义审计；
- 完整“隐私、游戏、路由器”配置模板；
- 自动迁移旧文件；
- Shadowrocket Tailscale 模块启用警告。

## 主要研究来源

- Mihomo 全局配置：
  https://wiki.metacubex.one/en/config/general/
- Mihomo TUN：
  https://wiki.metacubex.one/en/config/inbound/tun/
- Mihomo Tailscale：
  https://wiki.metacubex.one/en/config/proxies/tailscale/
- Mihomo v1.19.25 进程模式源码：
  https://github.com/MetaCubeX/mihomo/blob/v1.19.25/component/process/find_process_mode.go
- sing-box TUN：
  https://sing-box.sagernet.org/configuration/inbound/tun/
- sing-box Tailscale endpoint：
  https://sing-box.sagernet.org/configuration/endpoint/tailscale/
- sing-box Tailscale DNS：
  https://sing-box.sagernet.org/configuration/dns/server/tailscale/
- Tailscale MagicDNS：
  https://tailscale.com/docs/features/magicdns
- Tailscale DNS 参考：
  https://tailscale.com/docs/reference/dns-in-tailscale
- Shadowrocket 懒人配置参考：
  https://github.com/LOWERTOP/Shadowrocket/blob/main/lazy.conf
- GUI for Cores 社区配置索引：
  https://gui-for-cores.github.io/guide/gfs/community

## 已确认决策摘要

1. 固定目标版本只在 Web 文件预览中显示。
2. 不增加最终静态语义审计。
3. 社区实践做成可组合场景预设。
4. 每个客户端只展示自身支持的能力。
5. Shadowrocket 提供默认关闭的 WebRTC 隐私预设。
6. sing-box 提供默认关闭的 STUN 阻断。
7. Mihomo 使用 UDP 3478/5349 近似阻断 STUN。
8. Mihomo 和 sing-box 提供默认关闭的 QUIC 强制回退。
9. sing-box 提供默认关闭的 IPv4-only。
10. Shadowrocket 提供默认关闭的 IPv6 禁用。
11. 三个客户端的新文件默认加入仅 UDP/123 的 NTP 直连。
12. Mihomo 新 url-test 分组使用 tolerance: 50。
13. 多订阅集合失败策略延期。
14. Mihomo 和 sing-box 提供默认关闭的 Linux/OpenWrt auto-redirect。
15. Mihomo 局域网共享默认开启并使用 RFC1918/ULA 白名单。
16. Shadowrocket 默认 REJECT 不支持的 UDP，并提供 DIRECT 兼容预设。
17. Shadowrocket 默认关闭缺失代理链连接。
18. Mihomo Geo 自动更新默认开启，周期 24 小时。
19. Mihomo 加速预设使用自动的 find-process-mode: strict。
20. Shadowrocket 直连 DNS 失败默认不改经代理，并提供受限网络预设。
21. sing-box 分组支持默认关闭的 interrupt_exist_connections。
22. Tailscale 使用关闭、外部共存、原生接管三态且互斥。
23. Exit Node 默认关闭且不改写最终分流。
24. Tailscale 是文件局部端点，不进入 NodeIR。
25. Sandrone 不接收、保存或生成 Auth Key。
26. accept routes 默认关闭。
27. sing-box 使用 preferred_by；其它客户端使用标准范围和用户 CIDR。
28. MagicDNS 只处理 Tailnet 名称。
29. 只支持官方 Tailscale，不支持 Headscale/control URL。
30. 原生端点保持最小字段，高级字段由用户脚本补充。
31. 规则顺序为用户、Tailscale、通用规则、最终规则。
32. Shadowrocket 不显示 Tailscale 模块启用警告。
33. sing-box UDP/P2P 兼容默认关闭。
34. sing-box MPTCP 直连默认关闭并依赖加速预设。
35. Mihomo 和 sing-box 提供默认关闭的 Windows VM/多网卡兼容。
36. UDP/P2P 兼容同时覆盖 Mihomo 和 sing-box。
37. 不增加 Mihomo TCP Keep-Alive 预设。
38. 预设复用现有处理器并按场景分类。
39. 启用新预设时自动关闭明确识别的冲突预设并显示提示。
40. 现有文件不自动迁移。
41. 采用受管场景预设总体方案；Tailscale 不做表单，使用处理器。
