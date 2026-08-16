# 节点管线

## 目标与边界

节点管线把不同来源的节点转换为统一 `NodeIR`，在明确的节点阶段应用策略，再生成目标节点片段。parser、processor 和 renderer 各自只拥有一个方向的职责，service 负责顺序、校验和报告汇总。

完整节点数据流只在本页定义：

```text
source -> parse -> normalize -> process(nodes) -> render -> serve
```

`serve` 表示调用方或入口交付本次结果，不表示生成内容会被持久化。完整 Mihomo、sing-box 或 Shadowrocket 配置属于[文件管线](file-pipeline.md)，不是节点 renderer 的输出职责。

已保存 Subscription 的最终 render 可以按资源三态 TTL 使用
`subscription_render` 内部结果缓存；它仍是可重建加速数据，不会改写
Subscription 或形成可列举产物。直接 render/convert 与 preview 不使用该层。
完整缓存层、key、刷新和失效契约见[存储架构](storage.md#cache)。

## 1. Source：解析输入来源

节点来源可以由请求正文、`NodeInput` 或保存的 `Subscription` 声明。service 负责把来源解析成内容或直接的节点集合：

- inline nodes 可以直接进入统一模型边界。
- inline 内容按声明格式交给 parser。
- local 输入只读取经过 Store key 校验的受控资源。
- remote 输入通过统一 HTTP(S) fetcher，继承或覆盖超时、User-Agent、代理和缓存设置。
- subscription 引用在使用时物化；collection 可以组合多个输入并记录依赖。

输入解析返回的不只是节点，还包括 `SourceInfo`、resource dependencies、warnings 和订阅级运行时 metadata。它们随 `NodeSet` 传播，由 service 汇总，processor 和 renderer 不自行重新读取来源。

远程输入的自动识别是 service 的受控策略。显式格式只调用对应 parser；允许自动识别的输入先判断受支持的完整配置结构，再使用严格订阅格式兜底。adapter 不应各自形成不一致的隐式 fallback 链。

## 2. Parse：外部格式进入 `NodeIR`

parser 只把字节内容转换为 `[]NodeIR` 和可选 `SourceInfo`。外部 schema、别名和字段映射留在 adapter 内，不泄漏为 service 分支。

解析阶段遵守以下约束：

- 先解析结构化值，再把有依据的字段映射到统一语义。
- 可跨目标表达的协议和传输字段进入显式 `NodeIR`。
- 来源私有或未知字段进入带来源前缀的 `Raw`，并在需要时产生 warning。
- 输入顺序在没有显式处理策略时保持稳定。
- 单项输入缺少必要字段时返回结构化错误。
- 容器中的局部失败可以跳过该项并返回可定位 warning；没有任何有效节点时整次解析失败。

parser 不访问 Store、不执行节点探测，也不根据最终客户端策略修改节点。当前格式和协议支持范围由[格式与能力参考](../reference/capabilities.md)和运行时 capability summary 给出，本页不维护第二份清单。

## 3. Normalize 与语义校验

normalize 是 adapter 把外部别名、默认语义和目标结构统一成 `NodeIR` 的边界。service 随后用共享的 node validation 检查协议必填项与结构约束。

### 字段接纳与 warning 处置

warning 是需要分类的诊断，不是新增兼容逻辑或 `NodeIR` 字段的充分理由。来源字段
只有同时满足以下条件，才能提升为新的 canonical 语义或扩大既有字段值域：

1. 有可引用的协议规范、上游主实现或多个独立实现作为证据，并已区分线上协议语义
   与单一客户端的本地配置开关、别名和默认策略。
2. 能给出与来源格式无关的稳定定义，包括值域、默认值、冲突关系以及未知未来值的
   处理方式；字段同名、共享前缀或单一样本可用都不能证明语义等价。
3. 已明确所有输入和输出的行为：哪些 parser 能提升、哪些 renderer 能等价表达、
   哪些目标有损或必须跳过。canonical 字段不要求所有目标都能输出，但不能把目标
   私有结构直接搬进共享 IR。
4. 能在共享 validation、capability catalog 和跨格式测试中验证上述契约，而不是只在
   probe、某个 renderer 或单个 fixture 中形成特判。

字段完成归属和语义判断后，按以下顺序处置：

| 判断 | 处理 | 诊断 |
| --- | --- | --- |
| 来源别名或默认值与既有 IR 语义可证明完全等价 | 规范化到既有字段；无信息损失时可静默消费 | 只有采用了可能影响理解的假设时才产生专用 warning |
| 来源私有或未知的可选扩展，保留原值有明确诊断价值 | 保存在带来源前缀的 `Raw`；不得作为跨目标透传通道 | `parse_unknown_field` |
| 已知 canonical 字段出现未知、冲突或非法值 | 保留该值直到共享 validation；不得猜测、截断或按前缀映射 | `node_validation_dropped`；全批无有效节点时失败 |
| 可选 canonical 字段无法由目标等价表达，且移除不改变连接、安全或路由成立条件 | 保留 IR，目标省略该字段 | `render_lossy_field` |
| 目标缺失会改变认证、TLS identity、transport、协议变体或其它连接关键语义 | 跳过该节点，不做降级输出 | `render_node_skipped` |

“可清空后保留节点”必须能证明清空只移除无操作默认值或非语义 metadata。无法证明
时按连接关键字段处理并隔离节点。warning 的调查流程固定为：先在未运行 processor
的原始 parse 路径复现，再确认字段属于协议还是来源实现，随后选择上表动作，最后
检查所有 parser、renderer、validation、capability、warning、跨格式测试和文档。
probe 只消费已经规范化且验证通过的 `NodeIR`，不承担字段修复。

VMess、VLESS 和 Trojan URI 的 TCP `headerType=http` 规范化为
`transport.type=tcp` 与 `transport.header_type=http`，不能改写成表示 H2 的
`transport.type=http`。空 `vmess.vcn`、`headerType=none`、gRPC `mode=gun`、
AnyTLS `type=tcp` 和默认 TCP `path=/` 作为无语义差异的兼容默认值消费。

URI adapter 会把 WebSocket path 中独占的 `?ed=<正整数>` 约定转换为
`transport.max_early_data`，同时将
`transport.early_data_header_name` 设为 `Sec-WebSocket-Protocol`。VMess AEAD、
VLESS 和 Trojan WebSocket URI 的顶层查询参数 `ed=<正整数>` 与可选非空 `eh`
也映射到这两个字段；缺少 `eh` 时使用相同默认值。非法、非 WebSocket 或与 path
语义冲突的查询参数继续保留到 `raw` 并告警，包含其他 query 参数或无效 `ed` 的
path 保持原样。

canonical VLESS flow 只接受空值和 `xtls-rprx-vision`。其它值不会按前缀或来源
实现的私有约定转换；共享 validation 会隔离该节点并产生
`node_validation_dropped`，因此未知 flow 不会进入目标 renderer 或 probe core。

service 还会统一移除非 TCP/raw transport 上的 VLESS Vision flow，并产生
`node_normalized_incompatible_flow` warning。这个兼容修正与目标 renderer
无关，不允许在 sing-box 或 Mihomo 输出层静默形成不同语义。

校验会在多个可信边界复用：

- parser 输出完成规范化后。
- nodes-stage processors 返回后。
- 调用方直接提交节点给 renderer 时。
- 节点进入 probe 后端前。

局部非法节点被丢弃，并产生 `node_validation_dropped` warning，指出阶段、节点和首个相关字段。只要仍有合法节点，请求可以继续；如果非空输入中的所有节点都非法，则返回 `node_validation_failed`。

校验错误只报告必要的身份和字段上下文，不把 password、UUID、token 或 private key 拼进错误消息。processor 不需要复制每种协议的必填字段规则。

## 4. `process(nodes)`：声明式节点处理

nodes-stage processor 接收节点切片、目标、来源上下文和请求 metadata，返回新的节点结果与 warnings。处理器通过 registry 按 `ProcessorSpec` 构造。

执行规则是稳定契约：

- specs 严格按声明顺序运行，不按处理器类型自动重排。
- 明确写出的 `stage` 决定所属阶段。
- 某类型只注册在一个阶段时，registry 可以推断空 stage。
- 同时注册 nodes/file 的类型必须显式写 stage；`script` 属于这种情况。
- 每一步读取上一步结果，后一步不会看到原始输入的旁路副本。
- processor 失败会终止当前调用，service 不把中间节点作为成功结果交付。

内建处理器覆盖过滤、去重、重命名、排序、常用属性策略和显式探测。JavaScript processor 用于内建策略无法表达的开放式改写，但仍受同步 envelope、超时和注入 API 限制。

processor 的包边界同样重要：

- 不 import adapter，不直接调用特定 renderer。
- 不直接读写 Store 或宿主文件系统。
- 不持有通用 HTTP 客户端。
- 需要探测、订阅、文件或远程脚本时，只能使用 service 注入的窄接口。
- 实现必须把输入视为只读并返回新 output，不依赖跨阶段共享可变状态。

处理器参数错误返回 `processor_config_invalid`，未注册类型返回 `processor_unknown`，执行失败保留处理器身份。脚本的运行时与安全模型见[脚本 API 参考](../reference/scripting-api.md)。

`ParseRequest` 可以在解析后声明节点链，`RenderRequest` 可以在渲染前声明节点链；`ConvertRequest` 保持前者先于后者。它们都处于同一个逻辑 nodes stage，并各自保持声明顺序。

## 5. Render：`NodeIR` 进入目标节点格式

renderer 只接收已经校验和处理的节点，生成目标节点片段。它不读取原订阅、不执行 file-stage processor，也不拼装完整客户端配置。

支持报告的 renderer 同时返回正文与 `RenderReport`。service 把 renderer warnings 与上游解析、校验和 processor warnings 合并为顶层 `Report`。

渲染结果遵守以下兼容边界：

- 目标可表达的稳定字段按 adapter 映射输出。
- 目标无法表达的可选字段产生 lossy warning，节点仍可保留。
- 无法安全降级的认证、TLS、transport 或协议变体会产生节点级跳过 warning。
- 只要还有节点成功输出，renderer 可以返回剩余正文与 warnings。
- 所有节点都无法输出时返回渲染错误，不返回看似成功的空产物。

`Raw` 不会自动复制到任意目标。只有目标 adapter 明确支持并能保持语义时才可以回填，否则仍按能力边界报告。

## 能力与有损报告

adapter capability catalog 描述格式方向、节点类型、字段状态、映射依据和可逆性。运行时 capability summary 从已注册 adapter 汇总，避免手工清单与实际构建不一致。

`RenderReport` 的核心含义是：

- `SuccessCount`：成功输出的节点数。
- `Warnings`：字段损失、raw 未回填或节点跳过的结构化诊断。
- `LostFields`：renderer 汇总的兼容 warning 数量；它可能包含节点级跳过，不能当作严格字段计数。

调用方应检查 warning code、节点身份、字段和 target，而不是仅判断正文非空或 `LostFields` 数值。

新增或改变有损行为时，adapter capability、warning 和测试 fixture 必须同步。兼容性由这三者共同定义，不由静默的“尽量转换”规则定义。

## Report、来源与安全

service 把本次调用的 source refs、dependencies、validation warnings、processor warnings 和 render report 合并后随结果返回。report 不写入 subscription 或 Store，也不修改输入 `NodeIR` 的持久化定义。

warning 的 `NodeContext` 允许 parser 附带原始行或结构化来源值。这有助于诊断，但也可能包含认证信息。部署方在日志、缓存、分享或公开 API 中处理 report 时，不能假定它已统一脱敏。

普通 service 完成日志记录格式、目标、节点数、warning 数和耗时，不记录节点 payload。上游 adapter 和错误构造也应避免把凭据放进错误字符串。

## 与其它管线的关系

- typed file driver 复用节点 renderer 生成片段，但节点片段只是完整文件编译的输入。
- probe 是运行时观测；renderer 不根据探测结果隐式删除节点。
- 只有显式 `probe` processor 或脚本才会按探测结果过滤、排序或写入 `NodeIR.Meta`。
- 订阅 traffic 与 probe result 都不提升为稳定协议字段。

领域对象关系见[领域模型](domain-model.md)，probe 的后端和缓存语义见[节点探测](probing.md)，结构化错误见[错误与诊断参考](../reference/errors.md)。
