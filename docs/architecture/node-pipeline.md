# 节点管线

## 目标与边界

节点管线把不同来源的节点转换为统一 `NodeIR`，在明确的节点阶段应用策略，再生成目标节点片段。parser、processor 和 renderer 各自只拥有一个方向的职责，service 负责顺序、校验和报告汇总。

节点数据流：

```text
source -> parse -> normalize -> process(nodes) -> render -> serve
```

`serve` 表示调用方或入口交付本次结果，不表示生成内容会被持久化。完整 Mihomo、sing-box 或 Shadowrocket 配置属于[文件管线](file-pipeline.md)，不是节点 renderer 的输出职责。

## 1. Source：解析输入来源

节点来源可以由请求正文、`NodeInput` 或保存的 `Subscription` 声明。service 负责把来源解析成内容或直接的节点集合：

- inline nodes 可以直接进入统一模型边界。
- inline 内容按声明格式交给 parser。
- local 输入只读取经过 Store key 校验的受控资源。
- remote 输入通过统一 HTTP(S) fetcher，继承或覆盖超时、User-Agent、代理和缓存设置；
  只有处在已保存资源作用域时才读写持久 remote-fetch cache。
- subscription 引用在使用时物化；collection 可以组合多个输入并记录依赖。

输入解析返回的不只是节点，还包括 `SourceInfo`、resource dependencies、warnings 和订阅级运行时 metadata。它们随 `NodeSet` 传播，由 service 汇总，processor 和 renderer 不自行重新读取来源。

已保存 Subscription 通过唯一的 subscription execution 边界生成 canonical
`NodeSet`。preview、subscription render、typed file、diagnose、定时更新和脚本中的
订阅子调用复用同一条 normalize、校验和 nodes-stage processor 链；执行同时返回
处理前与处理后结果，并显式汇总声明式引用、脚本文件和脚本动态订阅依赖。
处理前后的结果可通过[订阅执行快照](storage.md#cache)复用，输出格式不改变这条处理链。

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

adapter 与 service 的规范化步骤统一外部别名和等价默认值；共享 validation 检查协议必填项与结构约束。节点实例和连接身份的区别见[领域模型](domain-model.md#nodeir)。

### 字段接纳与 warning 处置

新增 canonical 字段或扩大值域，应先区分协议语义、来源别名和客户端本地策略，
用协议规范或上游实现说明其含义，再确定受影响输入、输出及无法等价表达时的行为。
证据和测试范围按实际语义影响选择，见[影响矩阵](../../CONTRIBUTING.md#跨协议与客户端影响矩阵)。
warning 本身不证明需要新增兼容逻辑；字段同名或共享前缀也不证明语义等价。

根据字段语义选择处理方式：

| 判断 | 处理 | 诊断 |
| --- | --- | --- |
| 来源别名或默认值与既有 IR 语义可证明完全等价 | 规范化到既有字段；无信息损失时可静默消费 | 只有采用了可能影响理解的假设时才产生专用 warning |
| 来源私有或未知的可选扩展，保留原值有明确诊断价值 | 保存在带来源前缀的 `Raw`；不得作为跨目标透传通道 | `parse_unknown_field` |
| 已知 canonical 字段出现未知、冲突或非法值 | 保留该值直到共享 validation；不得猜测、截断或按前缀映射 | `node_validation_dropped`；全批无有效节点时失败 |
| 可选 canonical 字段无法由目标等价表达，且移除不改变连接、安全或路由成立条件 | 保留 IR，目标省略该字段 | `render_lossy_field` |
| 目标缺失会改变认证、TLS identity、transport、协议变体或其它连接关键语义 | 跳过该节点，不做降级输出 | `render_node_skipped` |

不能确认省略是否改变连接、安全或路由语义时，应保留诊断并隔离节点，避免生成
看似可用的错误配置。已确认等价的别名和默认值可以在规范化阶段转换；具体字段和
值域见[格式与能力参考](../reference/capabilities.md)。probe 消费规范化且验证通过的
节点，不承担字段修复。

校验会在多个可信边界复用：

- parser 输出完成规范化后。
- nodes-stage processors 返回后。
- 调用方直接提交节点给 renderer 时。
- 节点进入 probe 后端前。

局部非法节点被丢弃，并产生 `node_validation_dropped` warning，指出阶段、节点和首个相关字段。只要仍有合法节点，请求可以继续；如果非空输入中的所有节点都非法，则返回 `node_validation_failed`。

校验错误只报告必要的身份和字段上下文，不把 password、UUID、token 或 private key 拼进错误消息。processor 不需要复制每种协议的必填字段规则。

## 4. `process(nodes)`：声明式节点处理

nodes-stage processor 接收节点切片、目标、来源上下文和请求 metadata，返回新的节点结果与 warnings。处理器通过 registry 按 `ProcessorSpec` 构造。

已保存 Subscription 的 nodes-stage 结果是客户端无关的 canonical 结果，因此该
执行作用域中的 `target` 为空。目标格式只在后续 renderer 或 file driver 边界生效。
一次性 parse/render/convert 的 processor 仍可接收其调用方明确给出的 target。
因此 probe 所选核心无法表达某节点时会记录 `probe_node_unsupported`；`fail_mode: drop`
把“本次核心无法测活”作为显式过滤策略删除该节点，`keep` 和 `error` 则继续保留，交给
最终客户端 renderer 按自身能力决定输出或跳过。

执行规则是稳定契约：

- specs 严格按声明顺序运行，不按处理器类型自动重排。
- 明确写出的 `stage` 决定所属阶段。
- 某类型只注册在一个阶段时，registry 可以推断空 stage。
- 同时注册 nodes/file 的类型必须显式写 stage；`script` 属于这种情况。
- 每一步读取上一步结果，后一步不会看到原始输入的旁路副本。
- processor 失败会终止当前调用，service 不把中间节点作为成功结果交付；运行时
  完全没有 probe backend 时，内建 probe processor 按
  [probe 降级契约](../reference/processors.md#probe)返回 warning，不视为失败。

处理器可由内建实现或 JavaScript 提供；脚本受同步 envelope、超时和注入 API 限制。

processor 将输入视为只读并返回新 output；资源、远程读取和探测经 service 注入的
窄接口完成，包依赖见[架构总览](overview.md#依赖边界)。参数与错误约定见
[Processor 参考](../reference/processors.md)，脚本边界见[脚本 API](../reference/scripting-api.md)。

`ParseRequest` 可以在解析后声明节点链，`RenderRequest` 可以在渲染前声明节点链；`ConvertRequest` 保持前者先于后者。它们都处于同一个逻辑 nodes stage，并各自保持声明顺序。

## 5. Render：`NodeIR` 进入目标节点格式

renderer 只接收已经校验和处理的节点，生成目标节点片段。它不读取原订阅、不执行 file-stage processor，也不拼装完整客户端配置。

支持报告的 renderer 同时返回正文与 `RenderReport`。service 把 renderer warnings 与上游解析、校验和 processor warnings 合并为顶层 `Report`。

renderer 按[字段处置规则](#字段接纳与-warning-处置)报告损失或跳过节点。
部分节点成功时返回正文及 warnings；所有节点均无法输出时返回错误。
`Raw` 只有在目标 adapter 明确支持并能保持语义时才可回填。

## 能力与有损报告

adapter capability catalog 描述格式方向、节点类型、字段状态、映射依据和可逆性。运行时 capability summary 从已注册 adapter 汇总，避免手工清单与实际构建不一致。

`RenderReport` 的核心含义是：

- `SuccessCount`：成功输出的节点数。
- `Warnings`：字段损失、raw 未回填或节点跳过的结构化诊断。
- `LostFields`：renderer 汇总的兼容 warning 数量；它可能包含节点级跳过，不能当作严格字段计数。

调用方应检查 warning code、节点身份、字段和 target，而不是仅判断正文非空或 `LostFields` 数值。

新增或改变有损行为时，同步受影响的能力声明与诊断，并验证可输出、跳过和损失的边界。

## Report、来源与安全

service 汇总本次 source refs、dependencies、validation、processor 和 renderer 的诊断。
正常日志记录计数和耗时，避免记录节点 payload；report 中的来源上下文可能含凭据，
其处理边界见[错误与诊断参考](../reference/errors.md)。

## 与其它管线的关系

- typed file driver 复用节点 renderer 生成片段，但节点片段只是完整文件编译的输入。
- probe 是运行时观测；renderer 不根据探测结果隐式删除节点。
- 只有显式 `probe` processor 或脚本才会按探测结果过滤、排序或写入 `NodeIR.Meta`。
- 订阅 traffic 与 probe result 都不提升为稳定协议字段。

领域对象关系见[领域模型](domain-model.md)，probe 的后端和缓存语义见[节点探测](probing.md)，结构化错误见[错误与诊断参考](../reference/errors.md)。
