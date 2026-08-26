# 领域模型

## 模型角色

Sandrone 用少量稳定模型隔离持久化定义、运行时物化结果和外部格式：

```text
Subscription / NodeInput -> NodeSet -> []NodeIR
FileSpec                  -> FileDocument
NodeIR                    -> NodeProbeResult
执行上下文                -> Report / Warning / SourceRef
```

箭头表示“按一次请求解析或构造”，不是持久化所有权。`Subscription` 和 `FileSpec` 是可保存的定义；`NodeSet`、`FileDocument`、`ProbeResult` 和 `Report` 是一次执行中的值。

完整字段以 [`internal/domain`](../../internal/domain/) 和 [`pkg/sandrone`](../../pkg/sandrone/sandrone.go) 的公开别名为准。本页只解释模型之间长期有效的关系。

## `NodeIR`

`NodeIR` 是与外部客户端格式无关的统一节点表达。parser 把分享 URI、订阅或客户端节点结构规范化为它，renderer 再从它生成目标节点片段。

字段按语义分为几类：

- 节点展示与连接端点，例如名称、类型、server 和 port。
- 认证与协议参数，例如 UUID、password、cipher 和 flow。
- 可跨目标表达的 TLS、transport、dialer、multiplex 和协议专属 options。
- 标签与 metadata，用于处理链和调用方上下文。
- `Raw`，保存来源格式中尚未提升为稳定 IR 的字段。
- `SourceFormat`、`Lossy` 和 `Warnings`，保留来源与兼容诊断。

相似名称不会因为字符串相同就合并。字段只有在协议语义一致、且 capability catalog 有依据时才进入显式 IR；目标私有值保留在带来源前缀的 `Raw` key 中。

节点完成规范化和校验后会附带一个不序列化的 `RuntimeID`。它标识本次物化链路中的
一个节点实例，跨 processor 保留，但不跨独立请求承诺稳定，也不属于脚本或公开
`NodeIR` 字段。连接相等性则按需从规范化节点计算 `ConnectionKey`；该 key
覆盖完整连接语义，排除名称、标签、metadata、来源原文和诊断状态，不保存到节点，
也不出现在公开响应中。preview 用前者追踪实例，连接去重和 probe cache 用后者判断
连接语义，二者不再各自维护私有身份算法。

`Raw` 是保留信息的边界，不是任意字段都能跨格式回填的承诺。renderer 仍须根据目标能力决定输出、报告字段损失或跳过节点。

`NodeIR` 不承载以下状态：

- 完整客户端的代理组、路由和 DNS 等平台配置。
- 远程订阅套餐用量。
- 当前探测是否存活、耗时或后端版本。
- Store key、HTTP 请求或入口框架对象。

模型定义见 [`node.go`](../../internal/domain/node.go) 和 [`node_options.go`](../../internal/domain/node_options.go)；格式字段支持范围见[格式与能力参考](../reference/capabilities.md)。

## `NodeInput` 与 `NodeSet`

`NodeInput` 描述“节点从哪里来”，可以携带 inline nodes、待解析内容、受控本地或远程来源，或引用保存的 subscription。它是请求与文件编排的输入声明，不是节点本身。

service 把输入解析成 `NodeSet`。`NodeSet` 把最终 `[]NodeIR` 与以下只读上下文放在一起：

- `Dependencies`：本次物化读取的命名资源。
- `Sources`：parser 或远程输入提供的来源信息。
- `Warnings`：解析、输入跳过和处理阶段诊断。
- `Traffic`：远程订阅响应带来的套餐用量观测。
- `Meta`：订阅或输入附带的调用上下文。

processor、renderer 和 probe backend 最终处理节点值；service 负责携带和汇总 `NodeSet` 的依赖、来源与 warning。

相关定义见 [`file.go`](../../internal/domain/file.go) 和 [`processor.go`](../../internal/domain/processor.go)。

## `Subscription`

`Subscription` 是可命名、可引用的节点来源定义：

- `remote` 保存受控 `RemoteInput` 和可选显式格式。
- `local` 保存本地内容快照和格式。
- `collection` 通过 `NodeInput` 聚合其它来源或 subscription。

subscription 可以声明 nodes-stage processors。service 在 preview、probe、文件编译或分享渲染需要节点时才物化它，并检测 collection 引用循环。

定义本身不保存每次远程读取后的完整节点结果，也不保存本次 report。由响应头解析出的 traffic 只存在于运行时 `NodeSet` 或专用 traffic 结果中，不写入 `NodeIR` 或 `Subscription.Meta`。

subscription 的定义见 [`subscription.go`](../../internal/domain/subscription.go)；物化顺序见[节点管线](node-pipeline.md)。

## `FileSpec` 与 `FileDocument`

`FileSpec` 是可持久化的文件定义，描述：

- 稳定的 canonical `kind`。
- inline、remote 或 typed driver 的内建 base。
- typed file 的公共 `config` envelope。
- 按顺序声明的 file-stage processors。
- 名称、时间与 metadata。

`kind` 必须显式提供，包括 `static`。缺失、大小写变体或未注册值不是另一种默认行为。

typed file 的公共 `FileConfig` 只包含：

- `subscriptions`：编译时要物化的命名订阅。
- `settings`：JSON object，由对应 typed-file driver 严格解码。

公共模型不包含 Mihomo、sing-box 或 Shadowrocket 的联合字段，也不根据 settings 形状推断客户端。客户端默认值、严格 schema 和编译语义属于 driver。

`FileDocument` 是一次请求中的运行时文件，携带正文、media type、encoding、metadata、warnings，以及处理器需要时使用的 parts。静态 source 读取和 typed driver 编译都会先形成 `FileDocument`，但生成结果不会写回 `FileSpec`。

模型定义见 [`file.go`](../../internal/domain/file.go)，构造与处理边界见[文件管线](file-pipeline.md)。

## `Warning`、`SourceRef` 与 `Report`

`Warning` 是可定位、可分类的非致命诊断。它可以指向 warning code、节点、字段、来源、目标和节点上下文。局部节点无法解析、语义校验丢弃、渲染有损、缓存失败等情况都可以在仍有可用结果时用 warning 表达。

`SourceRef` 描述本次结果使用的来源或映射依据。运行时远程输入通常记录来源 URL、状态摘要或内容 hash；adapter capability 也可以用它指向协议、schema 或 fixture 依据。

`ResourceRef` 表达命名资源依赖，与 `SourceRef` 不同：

- dependency 回答“这次读取了哪个 subscription 或 file”。
- source ref 回答“内容或字段映射来自哪里”。

`Report` 汇总一次调用的状态、依赖、来源、warnings、render 统计和可选 probe 统计。它随结果返回，不作为可管理资源持久化，也不自动修改 `Subscription`、`FileSpec` 或 `NodeIR`。

warning 上下文允许 parser 携带原始行或结构化原值，其中可能出现凭据。调用方不能假定 report 已脱敏；记录、缓存或公开返回前必须遵守自己的安全边界。

定义见 [`diagnostics.go`](../../internal/domain/diagnostics.go)、[`source.go`](../../internal/domain/source.go)、[`resource.go`](../../internal/domain/resource.go) 和 [`report.go`](../../internal/domain/report.go)。错误与 warning 的 wire 约定见[错误与诊断参考](../reference/errors.md)。

## `ProbeResult`

`ProbeResult` 表达一次运行时观测：

- `Results` 按节点记录 layer、method、target、backend、存活状态、耗时、检查时间和失败诊断。
- `Report.Probe` 汇总成功、失败、缓存命中和按探测维度分组的统计。

探测结果与节点配置的生命周期不同，因此不提升为 `NodeIR` 稳定字段。只有调用方显式使用 probe processor 或脚本时，摘要才可以写入 `NodeIR.Meta`，用于本次后续过滤、排序或展示。

定义见 [`probe.go`](../../internal/domain/probe.go)，后端、缓存和报告边界见[节点探测](probing.md)。

## `DiagnoseResult`

`DiagnoseResult` 是 CLI 与公开 Go façade 的统一诊断结果。它包含输入分类与检测格式、
真实执行顺序的 stages、issues/warnings/dependencies/source refs、最终完整节点或
`FileDocument`，以及失败时不含 cause 的结构化 `AppError`。processor stage 另外
记录 scope、全局执行序号、前后计数和内部产生的完整 `ProbeResult`；processor
仍按各自处理链的声明顺序出现。

状态只有 `ok`、`partial`、`failed`。诊断不会自行加入 probe；trace 只观测输入
本来会执行的 processor/script 调用，不改变正常流水线输出。定义见
[`diagnose.go`](../../internal/domain/diagnose.go)，CLI wire 与退出码见
[CLI 参考](../reference/cli.md#diagnose)。

## 稳定不变量

- 持久化定义不吸收一次请求的派生结果。
- 外部格式只通过 adapter 或 typed-file driver 进入核心模型。
- warning 能解释局部降级；致命错误不伪装成空成功结果。
- report、dependency 和 source trace 由 service 汇总，不由入口层拼接。
- probe 与 traffic 是运行时观测，不是协议字段。
- 字段级 wire 契约以领域源码和公开 API 参考为准，架构页不复制完整 Go struct。
