# 文件管线

## 目标与所有权

文件管线生成完整、可直接交付的文件。`FileSpec` 是持久化定义，`FileDocument` 是一次请求中的运行时文件；节点 renderer 只生成节点片段，不能替代完整客户端配置编译。

文件分为两条路径。

静态文件：

```text
FileSpec(kind=static) -> read source -> FileDocument -> process(file) -> serve
```

类型化配置文件：

```text
FileSpec(kind=registered typed kind) -> driver lookup -> read/build base -> optional subscription materialization/node render -> driver compile settings -> process(file) -> serve
```

两条路径只共享 source 读取、运行时文档、file-stage processor 和结果报告。typed 路径额外拥有 driver registry、订阅物化和客户端编译边界。

## 配置策略归属

平台统一用户的配置意图和操作，生成的策略显式保存在 `FileSpec` 的 source、
settings 或 processors 中，可查看、修改、排序和删除。统一入口保留各客户端
的字段语义、正则语法和执行时机差异；能力可由客户端原生实现或显式处理器补齐。

后端负责受控读取、严格解码、节点渲染、目标格式编译和处理链执行，不隐式选择
默认出口等用户策略。`$nodes` 引用展开和目标端内建策略符号的实体化属于格式
编译；协议语义、安全校验和 renderer 边界也继续由后端负责。

固定内容可用模板或 `merge` 表达；动态策略可用合适的显式 processor 或 `script`。
新建默认只在创建时物化，保存或重开不会强制补回用户删除的处理器。
具体预设参数与已有文件行为见[社区配置预设](../reference/community-config-presets.md)。

## 公共契约

`FileSpec.kind` 必须显式使用 canonical 值，`static` 也不能省略。缺失、大小写变体、首尾空白或未注册 kind 返回 `invalid_argument`，不会回退为其它类型。

`static` 不允许携带 `config`。typed file 的公共 `FileConfig` 只包含：

- `subscriptions`：仅供声明节点 renderer 的 typed driver 在编译时物化；
  config-only driver 拒绝非空值。
- `settings`：客户端专属 JSON object，由对应 driver 严格解码。

公共 service 不读取 Mihomo、sing-box 或 Shadowrocket 的联合 settings，也不通过字段形状推断目标客户端。未知公共 config 字段会在领域解码时拒绝；unknown settings 字段和类型错误由选中的 driver 拒绝。

settings 的结构、必填项和编译语义由各 driver 定义；当前内建客户端的要求见
[FileSpec 参考](../reference/file-spec.md)。driver 按保存的定义编译，不隐式补回用户策略。

## Source 读取

`FileSource` 有两类受控来源：

- `inline`：使用定义内的正文。
- `remote`：通过统一 HTTP(S) fetcher 读取，并应用超时、User-Agent、代理与 TTL cache 设置。

source 读取不会让 processor 获得任意宿主路径；remote 内容受 fetcher 的协议、
响应状态和大小边界限制。

`GetFileSource` 返回编译前的 source：

- 对 static 文件，它是读取后的原始文档。
- 对 typed 文件，它是显式 source，或没有显式 source 时的 driver 默认 base。
- 它不物化订阅、不执行 driver compile，也不运行 file-stage processors。

持久化与[缓存](storage.md#cache)由存储层管理。最终 driver compile 与 file-stage
processor 每次请求都会执行；服务不缓存完整 `FileResult`。

## Static 路径

static 路径解析并校验定义、读取 source，形成 `FileDocument` 后执行 file-stage
processors。它不隐式解析正文中的节点，也不按扩展名选择 typed driver；动态读取
订阅可使用受控脚本 API，完整客户端编译可使用 typed file。

## Typed 路径

### Driver registry

typed-file registry 是 service 私有组合边界。每个 driver descriptor 声明：

- canonical kind。
- 输出 media type、syntax 和默认扩展名。
- 可选默认 base。
- 可选的节点片段 render format；config-only driver 不声明此字段。

driver 还拥有 settings 校验和完整客户端结构编译。service 只根据 `kind` lookup driver，再执行统一编排，不包含按具体客户端名称分支的公共业务逻辑。

注册时会拒绝空 kind、保留的 `static`、缺失必填 descriptor 字段或重复 kind。
运行时还会确认 descriptor 声明的节点 renderer 已注册。Shadowrocket 是
config-only driver，因此不声明 renderer。

### Base

typed file 有显式 source 时，其正文作为 base；`source.type` 为空时使用 driver 的内建 base。base 是客户端配置的输入，不是编译后的历史快照。

显式 inline base 与 driver 内建 base 分别维护，后端不会用内建内容覆盖已保存的 source。

### Subscription materialization

只有 descriptor 声明节点 renderer 时，service 才按 `config.subscriptions` 的
声明顺序读取订阅。每个 subscription 通过与
preview 和直接 subscription render 相同的 canonical subscription execution
完整执行来源解析、normalize、语义校验和 nodes-stage processors，结果节点按订阅
顺序聚合。客户端 target 只传给随后的 node renderer，不改变订阅节点集合。

直接和间接订阅引用、nodes script 使用的文件资源以及脚本动态产生的订阅都进入
file report dependencies。声明式与脚本动态订阅调用共享同一递归栈；缺失订阅、
subscription cycle、全部节点非法或 nodes processor 失败都会终止文件生成。

支持节点渲染的 typed file 不要求订阅一定存在；空 subscriptions 可以由 driver
根据 base 和 settings 生成不含 Sandrone 节点的配置。config-only driver（当前为
Shadowrocket）拒绝非空 subscriptions，直接基于 base 和 settings 编译。

### Node render 与 driver compile

descriptor 声明 renderer 时，service 将聚合节点渲染为目标片段并汇总兼容 warning；
config-only driver 收到空片段。节点能否输出遵守[节点渲染边界](node-pipeline.md#5-rendernodeir-进入目标节点格式)。

driver 接收 base、可选节点片段和 settings，严格解码并校验客户端结构，生成完整
正文。service 不再解析片段的客户端语义：renderer 负责节点表达，driver 负责完整配置。

## `process(file)` 阶段

static source 读取完成或 typed driver 编译完成后，service 才运行 file-stage processor。所有 specs 严格按 `FileSpec.processors` 的声明顺序执行，每一步读取上一步返回的 `FileDocument`。

`merge` 提供以下结构化修改：

- YAML/JSON overlay 处理对象递归合并和整体替换。
- YAML/JSON override 提供有序的数组与强制替换语义。
- INI override 以 section 运算修改文档，并保留未修改文本的格式。
- 语法或类型不匹配返回带 part/path 上下文的 `file_merge_failed`。

需要请求参数、资源组合或项目私有逻辑时，可以使用 file-stage `script`。脚本通过序列化 envelope 修改最终文档，并且只能使用 service 注入的受控 API；它没有通用文件系统、子进程或网络访问。

其它已注册 file processor 属于同一阶段，当前可用集合由 runtime capability summary 给出。脚本接口与示例见[脚本编写指南](../how-to/write-processor-script.md)。

## 依赖与递归读取

file-stage script 可以通过窄接口读取已保存 subscription 的节点或内容产物，也可以读取另一个已保存 file 的最终正文。

service 为同一次文件请求维护调用栈和 memo：

- file-backed script 独立渲染脚本文件资源，不继承当前 processor 或请求参数；
  脚本运行时的 `params.args` 仍与当前请求参数合并。
- `api.subscription.produce` 与 `api.file.content` 的子调用参数只来自各自显式
  `options.args`，不会继承父文件请求参数。
- 动态 subscription/file 引用加入 report dependencies。
- 重复读取可以复用本次请求中的结果。
- 文件循环依赖返回 `file_dependency_cycle`，不继续展开。

typed `config.subscriptions` 是声明式依赖；脚本读取是动态依赖。两者都由 service 解析，processor 不直接访问 MetaStore。

## 失败与发布原子性

文件生成按请求在内存中构造下一份 `FileDocument`。driver compile 或任一 processor 失败时，`GetFile` 和 `ValidateFile` 返回错误，不发布部分 `FileResult`，也不把已经执行的中间正文作为成功响应。

生成的正文和 report 不是权威资源，也不会持久化；失败不会覆盖已保存的
`FileSpec` 或 source。远程读取、probe 与订阅执行快照缓存仍遵守各自策略，不属于
生成产物的提交。

这是“单次生成结果只在完整成功后发布”的边界，不是 Store 事务承诺：

- `PutFile` 会在持久化前校验 kind、driver、renderer 和 settings 结构。
- FileSpec definition 以单个 JSON record 写入 Store。
- 进程崩溃、多进程并发和存储级恢复不由文件管线提供。

持久化与备份边界见[存储架构](storage.md)，节点物化与渲染的详细兼容语义见[节点管线](node-pipeline.md)。
