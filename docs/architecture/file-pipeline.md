# 文件管线

## 目标与所有权

文件管线生成完整、可直接交付的文件。`FileSpec` 是持久化定义，`FileDocument` 是一次请求中的运行时文件；节点 renderer 只生成节点片段，不能替代完整客户端配置编译。

文件分为两条路径，完整数据流只在本页定义。

静态文件：

```text
FileSpec(kind=static) -> read source -> FileDocument -> process(file) -> serve
```

类型化配置文件：

```text
FileSpec(kind=registered typed kind) -> driver lookup -> read/build base -> resolve config.subscriptions -> node render -> driver compile settings -> process(file) -> serve
```

两条路径只共享 source 读取、运行时文档、file-stage processor 和结果报告。typed 路径额外拥有 driver registry、订阅物化和客户端编译边界。

## 公共契约

`FileSpec.kind` 必须显式使用 canonical 值，`static` 也不能省略。缺失、大小写变体、首尾空白或未注册 kind 返回 `invalid_argument`，不会回退为其它类型。

`static` 不允许携带 `config`。typed file 的公共 `FileConfig` 只包含：

- `subscriptions`：需要在编译时物化的已保存订阅名称。
- `settings`：客户端专属 JSON object，由对应 driver 严格解码。

公共 service 不读取 Mihomo、sing-box 或 Shadowrocket 的联合 settings，也不通过字段形状推断目标客户端。未知公共 config 字段会在领域解码时拒绝；unknown settings 字段和类型错误由选中的 driver 拒绝。

省略 typed `config` 或 `settings` 等价于空 settings object。driver 可以为未出现的字段提供默认值；调用方显式提交的空数组保持为空，不能被默认集合覆盖。

字段级 wire 契约见 [`internal/domain/file.go`](../../internal/domain/file.go) 和 [FileSpec 参考](../reference/file-spec.md)。

## Source 读取

`FileSource` 有两类受控来源：

- `inline`：使用定义内的正文。
- `remote`：通过统一 HTTP(S) fetcher 读取，并应用超时、User-Agent、代理与 TTL cache 设置。

保存时，inline 正文作为完整 `FileSpec` 的 `source.content` 留在单个 JSON
record 中；remote source 只保存读取配置，生成时重新获取或命中内部缓存。
source 读取不会让 processor 获得任意宿主路径；remote 内容受 fetcher 的协议、
响应状态和大小边界限制。

`GetFileSource` 返回编译前的 source：

- 对 static 文件，它是读取后的原始文档。
- 对 typed 文件，它是显式 source，或没有显式 source 时的 driver 默认 base。
- 它不物化订阅、不执行 driver compile，也不运行 file-stage processors。

Store key 布局和持久化一致性见[存储架构](storage.md)。

## 最终结果缓存

只有通过名称读取的已保存 FileSpec 可以使用 `file_render` 结果缓存。顶层
`render_cache_ttl_seconds` 省略时继承项目设置的
`cache_defaults.file_render_ttl_seconds`，显式 `0` 关闭，正数覆盖；默认全局值
为 `0`。key 区分完整定义、构建身份、target、请求 args 与 metadata，因此不同
执行输入不会共用结果。

命中时返回完整 `FileResult` 并标记 `cached: true`。`refresh=true` 跳过最终
结果以及本次内部 remote-fetch/probe 读取，成功后重新填充；`ValidateFile`
始终重新执行文件管线，不使用旧的最终结果。inline spec 和超过 16 MiB 的正文
不写入该层。变更任一 file、subscription 或项目设置会广泛清空相关
结果层，以覆盖 typed config、脚本和间接引用。缓存 key、失效和后端边界的
canonical 说明见[存储架构](storage.md#cache)。

## Static 路径

static 文件适合原样交付或在文件阶段做有限改写。service 的职责依次是：

1. 从请求内 spec 或 MetaStore 解析 `FileSpec`。
2. 校验 kind、禁止的 config 和 source 声明。
3. 读取 inline 或 remote source。
4. 构造带名称、kind、正文、metadata 和 source trace 的 `FileDocument`。
5. 按声明顺序执行 file-stage processors。
6. 在全部步骤成功后构造 `FileResult` 和本次 report。

static 路径不隐式解析正文中的节点，也不会根据文件扩展名选择 typed driver。需要动态消费订阅时，调用方必须显式使用受控脚本 API 或改用 typed file。

## Typed 路径

### Driver registry

typed-file registry 是 service 私有组合边界。每个 driver descriptor 声明：

- canonical kind。
- 输出 media type、syntax 和默认扩展名。
- 可选默认 base。
- 用于节点片段的 render format。

driver 还拥有 settings 校验和完整客户端结构编译。service 只根据 `kind` lookup driver，再执行统一编排，不包含按具体客户端名称分支的公共业务逻辑。

注册时会拒绝空 kind、保留的 `static`、缺失 descriptor 字段或重复 kind。运行时还会确认 descriptor 要求的节点 renderer 已注册。

### Base

typed file 有显式 source 时，其正文作为 base；`source.type` 为空时使用 driver 的内建 base。base 是客户端配置的输入，不是编译后的历史快照。

具体默认模板内容属于 driver 和测试，不进入架构契约。调用方保存的显式 inline base 与后端 driver fallback 也是两个独立边界，不能互相隐式回填。

### Subscription materialization

service 按 `config.subscriptions` 的声明顺序读取订阅。每个 subscription 完整执行自己的来源解析、normalize、语义校验和 nodes-stage processors，结果节点按订阅顺序聚合。

直接和间接订阅引用进入 file report dependencies。缺失订阅、subscription cycle、全部节点非法或 nodes processor 失败都会终止文件生成。

typed file 不要求订阅一定存在；空 subscriptions 可以由 driver 根据 base 和 settings 生成不含 Sandrone 节点的配置。客户端是否允许这种结果仍由 driver 编译规则决定。

### Node render 与 driver compile

service 使用 descriptor 指定的 renderer 把聚合后的 `NodeIR` 生成目标节点片段。该 renderer 继续遵守能力与有损边界：

- 可选字段损失产生结构化 warning。
- 不安全的节点变体可以被跳过。
- 没有任何可渲染节点且 renderer 要求非空结果时返回错误。

renderer warnings 合并进 file report。service 不解析节点片段的客户端语义；driver 接收 base、节点片段和原始 settings，并负责：

- 严格解码自身 settings。
- 解析和校验 base。
- 把节点、分组、规则集、规则或其它受管结构编译到客户端文档。
- 保持显式空集合与默认值的区别。
- 返回完整 YAML、JSON 或 INI 正文。

节点 renderer 与 driver compiler 是正交边界：前者回答“这些节点如何写成目标片段”，后者回答“片段和策略如何组成完整客户端文件”。

## `process(file)` 阶段

static source 读取完成或 typed driver 编译完成后，service 才运行 file-stage processor。所有 specs 严格按 `FileSpec.processors` 的声明顺序执行，每一步读取上一步返回的 `FileDocument`。

结构化修改的首选是 `merge`：

- YAML/JSON overlay 处理对象递归合并和整体替换。
- YAML/JSON override 提供有序的数组与强制替换语义。
- INI override 以 section 运算修改文档，并保留未修改文本的格式。
- 语法或类型不匹配返回带 part/path 上下文的 `file_merge_failed`。

需要请求参数、资源组合或项目私有逻辑时使用 file-stage `script`。脚本通过序列化 envelope 修改最终文档，并且只能使用 service 注入的受控 API；它没有通用文件系统、子进程或网络访问。

`merge` 和 `script` 没有固定的相互优先级。声明为先 merge 后 script 时，脚本看到合并结果；反向声明时，merge 看到脚本结果。多个同类 processor 也遵守同一顺序。

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

生成的正文和 report 不是权威资源；启用结果缓存时只会在完整成功后写入可重建
cache，因此失败不会覆盖已保存的 `FileSpec` 或 source。远程读取等辅助缓存仍
遵守各自缓存策略，不属于生成产物的提交。

这是“单次生成结果只在完整成功后发布”的边界，不是 Store 事务承诺：

- `PutFile` 会在持久化前校验 kind、driver、renderer 和 settings 结构。
- FileSpec definition 以单个 JSON record 写入 Store。
- 进程崩溃、多进程并发和存储级恢复不由文件管线提供。

持久化与备份边界见[存储架构](storage.md)，节点物化与渲染的详细兼容语义见[节点管线](node-pipeline.md)。
