# MCP Agent 能力闭环设计

## 背景

Sandrone 当前 MCP 入口提供转换、文件读取、节点探测、文件校验和能力检查，
并可通过双重配置开关注册 subscription/FileSpec 保存工具。现有入口遵守
entrypoint 只做协议适配、业务编排留在 `internal/service` 的分层，但还不能让
受信任的本地 Agent 完成完整资源管理闭环：

- 已保存资源只能按已知名称读取，不能通过 MCP 发现名称；
- `ProcessorSpec.params` 和 `FileConfig.settings` 使用 `json.RawMessage`，
  自动生成的 MCP JSON Schema 将它们错误描述成字节数组，正常 JSON object
  会在进入 service 前被 SDK 拒绝；
- subscription preview/render/traffic、文件 source 模式、删除等已有 service
  能力没有 MCP 入口；
- processor、typed-file settings 与 script API 缺少机器可读、可细分读取的
  MCP 契约；
- prompts 只是无参数的一句话说明，不能有效指导 Agent 生成复杂定义；
- 超限正文被静默省略，tool annotations 与部分真实网络/覆盖副作用不一致。

本设计面向只由所有者或受信任本地 Agent 使用的 MCP server。Agent 可以直接
创建、覆盖和删除 subscription 与 FileSpec，不要求 server 在写入前实施人工
确认。

## 目标

1. 让 Agent 能发现、读取、验证、创建、覆盖、删除和执行 subscription/FileSpec。
2. 允许 Agent 使用声明式 processors 和现有 sandbox script processor。
3. 保持 tool、resource、prompt 的职责清晰，并避免把巨大动态 schema 重复塞入
   每个 tool。
4. 修复 MCP wire schema，同时保留 service 和 driver 的严格最终校验。
5. 让错误、输出省略和副作用对 Agent 可机器判断。
6. 继续禁止通用代码执行、任意文件系统/进程/网络访问。

## 非目标

- 不暴露通用 shell、JavaScript eval、HTTP fetch 或宿主文件系统 API。
- 不把所有 HTTP 路由机械复制成 MCP tools。
- 第一阶段不暴露 backup restore、runtime settings 修改、share 管理。
- 第一阶段不为大输出隐式创建 share、临时文件或持久化资源。
- 第一阶段不引入 MCP Tasks；probe 继续同步执行并受明确资源上限约束。
- 不在 MCP entrypoint 复制 service 业务编排或 driver 校验。

## 能力分层

### Tools：稳定业务动作

始终注册以下只读或计算型 tools：

| Tool | 用途 |
| --- | --- |
| `sandrone_inspect_capabilities` | 返回 summary，或按 format、processor、file kind 等过滤当前能力。 |
| `sandrone_list_resources` | 按 `subscription` 或 `file` 分页列出摘要和 resource URI。 |
| `sandrone_convert` | 从 inline 或受控 remote 输入解析、处理并渲染节点。 |
| `sandrone_probe_nodes` | 从 `NodeInput` 物化节点并执行受控探测。 |
| `sandrone_preview_subscription` | 比较保存订阅执行 processors 前后的节点。 |
| `sandrone_render_subscription` | 物化保存订阅并渲染到目标格式。 |
| `sandrone_get_subscription_traffic` | 读取 remote subscription 的运行时套餐用量。 |
| `sandrone_get_file` | 支持 `spec`、`source`、`render` mode 和本次调用 `args`。 |
| `sandrone_validate_file` | 对保存或 inline FileSpec 运行完整生成和校验流程。 |

启用 `--allow-management-tools` 时额外注册：

| Tool | 用途 |
| --- | --- |
| `sandrone_put_subscription` | 创建或覆盖 subscription。 |
| `sandrone_delete_subscription` | 删除 subscription。 |
| `sandrone_put_file` | 创建或覆盖 FileSpec。 |
| `sandrone_delete_file` | 删除 FileSpec 及 service 已定义的关联本地 source。 |

移除 `MCP.ReadOnly`、`--readonly` 及相关双重判断。管理 tools 只由一个默认关闭
的 `--allow-management-tools` 控制。开关启用后，server 不额外要求确认。

定义读取继续使用标准 MCP resources，不增加重复的
`sandrone_get_subscription` 或 `sandrone_get_file_spec`：

- `sandrone://subscriptions/{name}`
- `sandrone://files/{name}`
- `sandrone://capabilities`

`sandrone_list_resources` 返回上述 URI，让 Agent 先发现、再读取定义。
`sandrone_get_file` 保留为 tool，因为 source/render 会执行 service flow，而
不是读取静态定义。

### Resources：定义与机器契约

除现有资源外，增加以下 schema resources：

```text
sandrone://schemas/processors
sandrone://schemas/processors/{stage}/{type}
sandrone://schemas/file-kinds/static
sandrone://schemas/file-kinds/mihomo
sandrone://schemas/file-kinds/sing-box
sandrone://schemas/file-kinds/shadowrocket
sandrone://schemas/script-api/v1
```

`sandrone://schemas/processors` 只返回公开 processor 摘要。单个 processor
resource 返回：

- canonical type 和 stage；
- 用途说明；
- 参数 JSON Schema、默认值和 enum；
- 是否执行 probe、读取远程输入或运行脚本；
- 最小示例与常见完整示例；
- 可能产生的稳定错误码与 warning。

只注册但不能通过公开 FileSpec flow 使用的 `inject_nodes` 不得声明为公开可用
processor。未来只有在公开 flow 能提供其所需输入后才能加入。

file-kind resource 返回该 kind 的 typed settings schema、默认值、source 规则
和最小示例。schema resource 是 Agent 构造定义时的权威机器契约，driver 仍是
运行时最终校验者。

`sandrone://schemas/script-api/v1` 返回：

- nodes/file input 与 output envelope schema；
- `api.subscription.produce` 等注入 API 的参数、返回值和错误；
- sandbox、timeout、日志及敏感数据限制；
- inline、受控 store file、受控 remote source 示例；
- API 版本。

### Prompts：组合与编写指导

使用参数化 prompts 替换现有一句话 prompts：

| Prompt | 主要参数与用途 |
| --- | --- |
| `build_subscription` | 接受目标、subscription 类型、输入来源和是否需要 processors，指导生成可保存定义。 |
| `build_file` | 接受 kind、目标描述、引用资源和是否需要 script，指导生成 FileSpec 并调用 validate。 |
| `write_processor_script` | 接受 stage、目标和期望输入输出，引用 script API 与 processor schema。 |
| `diagnose_conversion_loss` | 接受源格式、目标格式和 report JSON，解释字段损失与替代方案。 |
| `explain_report` | 接受 report JSON 和关注维度，按依赖、来源、render/probe 统计和 warning 解释。 |

职责固定为：

```text
Schema resource = 权威机器契约
Prompt          = 编写策略、示例和组合指导
Tool            = 校验并执行
```

Prompt 不复制完整 schema，不成为唯一文档，也不能绕过 tool schema 或 service
校验。Prompt 应从同一 descriptor/registry 数据生成关键说明，减少漂移。

## MCP 专用 wire DTO

MCP entrypoint 不再直接用含 `json.RawMessage` 的 domain struct 生成 tool
schema。它定义只负责协议适配的 wire DTO：

```json
{
  "type": "rename",
  "stage": "nodes",
  "name": "optional-label",
  "params": {
    "mode": "prefix",
    "value": "HK-"
  }
}
```

其中 `params` 是普通 JSON object。入口将各值编码成
`map[string]json.RawMessage` 后调用 service。`FileSpec.config.settings`
同样在 MCP wire 上是普通 JSON object，入口编码成 `json.RawMessage`，再由
对应 driver 严格解码。

Tool 顶层 schema 需要明确：

- format、mode、probe layer/method/core 和 `NodeInput.type` 的 enum；
- `content` 与 `remote` 至少一个且互斥；
- `file` 与 inline `spec` 至少一个且互斥；
- timeout、attempts、concurrency、cache TTL、pagination 的范围；
- processor `params` 是 object；
- settings 是 object。

复杂 processor/file-kind 细节保留在 schema resources，避免每个 tool 重复
巨大的 `oneOf`。Tool schema 接受 object 后，service registry/driver 仍严格
拒绝未知字段。

每个 tool 使用专属 output DTO，避免 `validate_file` 等操作发布不可能出现的
`body`、`spec`、`resource_uri` 字段。`validate_file` 返回明确的
`{"ok": true, "report": ...}`。

## Script 暴露边界

Script 只作为现有 `type: script` processor 暴露，允许：

- nodes 和 file stage；
- inline source；
- 受控 store file source；
- 受控 remote source；
- 现有 sandbox、timeout 和窄注入 API。

不增加 `eval_script`、`run_javascript`、`run_shell` 或通用 fetch/filesystem/
process tool。脚本必须在 Sandrone service flow 内构建并执行，不能从
entrypoint 绕过 processor registry。

即使客户端是受信任本地 Agent，也保留现有 sandbox、超时、受控 source 和窄
API 边界。这些约束保证 Agent 编写错误脚本时不会获得与目标任务无关的宿主能力。

## 数据流

推荐的 Agent 流程是：

```text
inspect capabilities
→ list resources
→ read resource/schema
→ 可选获取 prompt 指导
→ preview/validate
→ put/delete/render/probe
```

Preview 和 validate 是推荐步骤，不是写操作的强制前置条件。Agent 可以直接
put/delete。若需要保留旧定义，Agent 应在覆盖或删除前主动读取 resource；
server 第一阶段不提供自动回滚或强制乐观锁。

## Annotations

- 纯定义读取、inspect、list：`readOnlyHint=true`。
- convert、probe、remote subscription、file render/validate：
  `openWorldHint=true`。
- put：`readOnlyHint=false`、`idempotentHint=true`、
  `destructiveHint=true`，因为同名保存会覆盖旧定义。
- delete：`readOnlyHint=false`、`idempotentHint=true`、
  `destructiveHint=true`。

Annotations 必须描述真实可能副作用，不能因为操作不修改 metadata 就把远程
抓取或 probe 标成 closed-world。

## 结构化错误

Tool error 除人类文本外返回稳定结构：

```json
{
  "error": {
    "code": "processor_config_invalid",
    "message": "rename.params.mode is invalid",
    "field": "parse_processors[0].params.mode",
    "resource_kind": "subscription",
    "resource_name": "demo"
  }
}
```

客户端按 `code` 分支，不解析 message。Entry adapter 保留领域错误码，并在能
可靠确定时补充 MCP wire field path、resource kind/name。Script timeout、
sandbox 拒绝、远程抓取、driver 解码和 processor 校验继续使用现有领域错误。

## 大输出与分页

`--max-output-bytes` 超限时不再静默省略正文，而是返回：

```json
{
  "content_type": "application/yaml",
  "body_omitted": true,
  "body_bytes": 2097152,
  "max_output_bytes": 1048576,
  "report": {}
}
```

第一阶段不自动创建 share、临时文件或 resource URI。

- `sandrone_list_resources` 使用 cursor/limit。
- `sandrone_inspect_capabilities` 支持 summary/filter。
- capabilities、processor、file-kind、script API 拆成细粒度 resources。
- probe 设置明确的节点数、timeout 和 concurrency 上限；结果仍同步返回。
- reports、probe results 和 resource JSON 的统一输出限制若超出第一阶段范围，
  必须至少记录为后续风险，不能声称受 `max-output-bytes` 完整保护。

## 写操作结果

Put 返回：

```json
{
  "ok": true,
  "resource_uri": "sandrone://files/example.yaml"
}
```

Delete 返回：

```json
{
  "ok": true,
  "deleted": true,
  "resource_uri": "sandrone://files/example.yaml"
}
```

Put 不返回旧定义。Delete 在资源不存在时遵循现有 service/store 的稳定错误，
而不是伪装成功。

## 测试与验收

实现必须增加以下 MCP 契约和端到端测试：

1. `tools/list` 中 processor `params` 和 file `settings` 是 object。
2. 带 rename、filter、script processor 的 convert 可以实际调用。
3. typed Mihomo 和 sing-box FileSpec 可以 validate、put、render。
4. subscription 完整闭环：
   list → put → read → preview → render → overwrite → delete。
5. file 完整闭环：
   list → put → read → source/render → validate → overwrite → delete。
6. inline、受控 store file、受控 remote script source 的成功路径。
7. script timeout，以及禁止 filesystem/process/任意 network 的失败路径。
8. 单一 `--allow-management-tools` 的注册行为和旧 `--readonly` 删除。
9. tool annotations 与真实副作用一致。
10. 超限输出明确返回 `body_omitted`、实际大小和限制值。
11. processor、file-kind、script API resources 的 schema 与实际 decoder 一致。
12. stdio 和 Streamable HTTP 各至少一条端到端 smoke test。
13. 所有始终注册 tools 的成功路径和稳定错误结构。
14. list pagination、无资源、资源不存在和非法单段名称边界。

相关窄测先运行 `go test ./internal/entry/mcpapi`，再运行受影响的 service、CLI、
HTTP 契约测试；交付前运行与改动范围匹配的仓库门禁。

## 文档

`docs/reference/mcp.md` 是完整 MCP 公开契约的 canonical 文档。processor、
FileSpec 和 script API 的领域细节继续由各自 canonical 参考负责，MCP 文档只
说明 wire 差异并链接它们。

删除 `--readonly` 时同步更新 CLI 参考、示例和测试，并用 `rg` 确认旧标识只剩
明确兼容点。Prompts、resource descriptions 和其它文档不复制完整 schema。

## 实施顺序

1. 引入 MCP wire DTO 和显式 schema，先修复 RawMessage 阻断。
2. 增加 schema resources，并让 capability summary 只声明公开可用能力。
3. 增加 list、subscription 操作、file source/args 和 delete tools。
4. 简化为单一 management 开关并修正 annotations。
5. 增加结构化错误和显式大输出语义。
6. 参数化 prompts，并从 descriptors 复用说明。
7. 补齐生命周期、script sandbox、transport 和文档测试。
