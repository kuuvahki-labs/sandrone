# MCP 参考

本页定义 Sandrone 当前 MCP 入口的公开契约：Streamable HTTP、鉴权、tools、
resources、prompts、输出限制与安全边界。启动 flags 见 [CLI 参考](cli.md)，
领域字段继续以 [FileSpec](file-spec.md)、[Processors](processors.md)、
[脚本 API](scripting-api.md)和 [HTTP API](http-api/README.md)为准。

## 协议版本与 Streamable HTTP

Sandrone 只接受并广告 MCP `2026-07-28`，不与旧版本协商。客户端先用
`server/discover` 读取 server identity、能力和唯一支持版本，随后直接发送无状态
请求；不发送 `initialize` 或 `notifications/initialized`。每个请求的 `_meta` 都要
包含：

- `io.modelcontextprotocol/protocolVersion: "2026-07-28"`；
- `io.modelcontextprotocol/clientInfo`；
- `io.modelcontextprotocol/clientCapabilities`。

旧 `initialize` 请求会收到 JSON-RPC `UnsupportedProtocolVersion`（`-32022`），
其 `data.supported` 只有 `2026-07-28`。缺少当前协议所需 metadata 的请求不会回退
到旧初始化流程。

MCP endpoint 使用 JSON response mode 的无状态 Streamable HTTP。每个 POST 都是
完整请求生命周期，server 不签发、读取或要求 `Mcp-Session-Id`；独立 GET 与
DELETE 返回 `405 Method Not Allowed`。请求取消会传递到正在执行的 Sandrone
service 调用。单个请求体上限为官方 Go SDK 的 `4 MiB`，超限返回 HTTP `413`。

`2026-07-28` 请求必须用标准 HTTP header 镜像 JSON-RPC body：

- `Mcp-Protocol-Version` 必须是 `2026-07-28`；
- `Mcp-Method` 必须精确匹配 body 的 method；
- `tools/call`、`prompts/get` 和 `resources/read` 还必须用 `Mcp-Name` 精确匹配
  tool 名、prompt 名或 resource URI。

header 与 body 不一致时返回 JSON-RPC `HeaderMismatch`（`-32020`）和 HTTP
`400`。Sandrone 当前 schema 没有 `x-mcp-header` 参数，因此不定义
`Mcp-Param-*` routing hint。

`sandrone serve mcp` 启动只挂载 MCP 的 HTTP listener；`sandrone serve all` 在
同一个 listener 上同时提供 HTTP API、可选 Web UI 和 MCP。`--path` 可以修改
MCP 路径，但必须以 `/` 开头。handler 复用 HTTP server 的静态 bearer token：

```http
Authorization: Bearer <token>
```

只要配置了 token，该 header 就必须精确匹配。绑定非 loopback 地址时必须提供
token；完整启动与鉴权规则见 [CLI 参考的 serve 章节](cli.md#serve)和
[HTTP API 鉴权](http-api/README.md#鉴权)。

## 能力与缓存

Sandrone 只广告 `tools`、`resources` 和 `prompts`。三者的 `listChanged` 都为
`false`，resource `subscribe` 为 `false`。不广告 Logging、Roots、Sampling、
Completions、实验能力或 extension，也不打开列表变更流；catalog 在 server 构建
完成后保持固定。

所有 cacheable result 都明确使用 `private`，防止共享缓存跨客户端复用经过鉴权的
Sandrone metadata 或定义：

| result | `ttlMs` | `cacheScope` |
| --- | ---: | --- |
| `server/discover` | `300000` | `private` |
| `tools/list` | `300000` | `private` |
| `prompts/list` | `300000` | `private` |
| `resources/list` | `300000` | `private` |
| `resources/templates/list` | `300000` | `private` |
| `resources/read` | `0` | `private` |

成功响应由 SDK 标记 `resultType: "complete"`，并在 `_meta` 的
`io.modelcontextprotocol/serverInfo` 中携带 Sandrone identity。`resources/read`
使用零 TTL，因为保存的 subscription 和 FileSpec 定义可能随时变化且可能包含
凭据；调用方每次使用前都应重新读取。

## Agent Skill

仓库在 `skills/sandrone` 发布配套的 Agent Skill。它提供推荐调用流程、安全边界
和常见任务配方，但不安装、启动或替代 MCP server，也不复制运行时 schema。

支持 Agent Skills 的客户端可以直接从仓库安装：

```bash
npx skills add kuuvahki-labs/sandrone --skill sandrone --agent codex
```

增加 `--global` 可安装到用户级目录。配置 HTTP 时，Skill 优先使用随附的 HTTP
script，以 `SANDRONE_URL` 指向 Sandrone server，并通过可选
`SANDRONE_TOKEN` 发送 bearer token。对于不能执行 script 的客户端，MCP 仍是
可用执行面；MCP server 的连接地址和 bearer token 继续只配置在 MCP 客户端，
不写入 Skill。HTTP route 与响应见 [HTTP API 参考](http-api/README.md)，这里
的 MCP HTTP 与 tool 契约不因 Skill 选择 HTTP script 而改变。

## Tool 注册与管理边界

缺省注册下列九个常驻 tool。四个管理 tool 只由单一开关
`--allow-management-tools` 控制，缺省关闭；没有第二个 readonly 开关。

管理模式面向可信本机 Agent，不是多租户授权边界。启用后，任何能连接该 MCP
server 的客户端都可以写入当前 data dir：`put` 会立即保存并覆盖同名定义，
`delete` 会立即删除，不提供确认、回收站或预览式写入。调用方应先读取当前
resource，并只在用户明确要求持久化或删除时调用管理 tool。
`sandrone_delete_file` 只删除保存完整 FileSpec 的单个 JSON record；如需备份，
Agent 应先读取 definition。完整删除语义见
[文件 HTTP API 的删除章节](http-api/files.md#delete-v1filesname)。

MCP SDK 发布每个 tool 的封闭 input schema。调用方应以 `tools/list` 返回的
schema 为准；未知字段会被拒绝。与 Go/持久化表示不同，MCP wire 上
`ProcessorSpec.params` 和 `FileSpec.config.settings` 都是 JSON object，而不是
编码后的 JSON 字节。领域结构和严格解码规则分别见
[Processors](processors.md)与 [FileSpec](file-spec.md)。

## Tools

### 始终注册

| tool | 输入用途 | 输出用途 |
| --- | --- | --- |
| `sandrone_list_resources` | 可选 `kind`（`subscription` 或 `file`）、`cursor`、`limit`。 | 当前定义摘要 `items` 与可选 `next_cursor`。 |
| `sandrone_inspect_capabilities` | 无参数返回总览；或同时给出 `kind`（`format`、`processor`、`file_kind`）和 `name`。 | 当前 capability catalog 与 `report`。 |
| `sandrone_convert` | `to_format`，以及 inline `content` + `from_format` 或受控 `remote`；可带 parse/render processors、render options 和 metadata。 | `content_type`、可选 `body` 与 `report`。 |
| `sandrone_probe_nodes` | 一个 `NodeInput`，以及 method、core、目标、timeout、attempts、concurrency 和 cache TTL 等探测参数。method 为 `tcp_connect`、`udp_ntp` 或 `url_test`。 | 节点级 `results` 与汇总 `report`。 |
| `sandrone_preview_subscription` | 已保存的 subscription `name` 与可选字符串 `args`。 | processor 前后的节点、数量与 `report`。 |
| `sandrone_render_subscription` | 已保存的 subscription `name`、目标 `format`、可选字符串 `args` 与 `refresh`。 | `content_type`、可选 `body`、`cached` 与 `report`。 |
| `sandrone_get_subscription_traffic` | 已保存的 remote subscription `name`；可用 `refresh` 强制刷新。 | 流量 metadata 与 `report`。 |
| `sandrone_get_file` | 已保存的 `file` 名称、可选 `target`/字符串 `args`/`refresh`；`mode` 为 `render`、`source` 或 `spec`，省略时默认 `render`。 | render 返回正文、`cached` 与 report；source 返回 `FileDocument`；spec 返回完整定义与 `resource_uri`。 |
| `sandrone_validate_file` | 已保存的 `file` 名称或 inline `spec`，以及可选 `target`/字符串 `args`。 | `ok` 与完整 `report`，不发布最终文件正文。 |

`sandrone_convert` 的格式与有损边界见[格式与能力参考](capabilities.md)；
processor 声明见 [Processors](processors.md)。文件 source、typed config 与完整
file flow 见 [FileSpec](file-spec.md)和[文件管线](../architecture/file-pipeline.md)。
探测语义见[节点探测](../architecture/probing.md)。

`sandrone_list_resources` 的 `limit` 缺省为 `50`，有效范围为 `1..200`。
`next_cursor` 是 server 生成、重启后仍有效但不可由客户端解释或修改的不透明
cursor；后续页必须沿用生成该 cursor 时相同的 `kind`。列表按 kind/name 稳定
排序；资源在翻页期间变化时，后续页反映调用时的当前存储状态。

### 仅管理模式注册

| tool | 输入用途 | 输出用途 |
| --- | --- | --- |
| `sandrone_put_subscription` | 完整具名 `Subscription` 定义。 | `ok` 与定义的 `resource_uri`；同名定义被覆盖。 |
| `sandrone_delete_subscription` | subscription `name`。 | `ok`、`deleted` 与原定义的 `resource_uri`。 |
| `sandrone_put_file` | 完整具名 `FileSpec` 定义。 | `ok` 与定义的 `resource_uri`；同名定义被覆盖。 |
| `sandrone_delete_file` | file `name`。 | 立即删除保存完整 FileSpec 的 JSON record；返回 `ok`、`deleted` 与原定义的 `resource_uri`。 |

Subscription 的类型与保存语义见
[HTTP 订阅资源参考](http-api/subscriptions.md)；`FileSpec.kind`、source、
typed config 与 processors 见 [FileSpec](file-spec.md)。文件删除的精确范围见
[文件 HTTP API](http-api/files.md#delete-v1filesname)。

### Tool annotations

九个常驻 tool 的 `readOnlyHint` 都为 `true`。其中
`sandrone_list_resources` 和 `sandrone_inspect_capabilities` 的
`openWorldHint` 为 `false`；其余七个常驻 tool 可能经受控 fetch、probe、
subscription 或 file flow 访问外部世界，`openWorldHint` 为 `true`。

四个管理 tool 的 annotations 相同：`readOnlyHint: false`、
`destructiveHint: true`、`idempotentHint: true`、`openWorldHint: false`。
这里的幂等表示重复相同调用的最终存储状态一致，不表示调用前会确认，也不表示
可恢复；put 可能覆盖，delete 立即生效。

## Resources 与 schema templates

所有 MCP resource 都返回 `application/json` 文本。固定 resources 为：

| URI | 内容 |
| --- | --- |
| `sandrone://capabilities` | 当前 parser、renderer、公开 processor、typed-file driver 与 probe capability summary。 |
| `sandrone://schemas/processors` | 公开 processor 的 stage、effects、说明和各详情 URI。 |
| `sandrone://schemas/script-api/v1` | versioned script config、envelope、注入方法、来源和 sandbox schema。 |

Resource templates 为：

| URI template | 内容 |
| --- | --- |
| `sandrone://subscriptions/{name}` | 已保存的 `Subscription` 定义。 |
| `sandrone://files/{name}` | 已保存的 `FileSpec` 定义，不是编译后的文件正文。 |
| `sandrone://schemas/processors/{stage}/{type}` | 指定 canonical stage/type 的 `params_schema`、effects、examples 和 error codes。 |
| `sandrone://schemas/file-kinds/{kind}` | 指定 canonical kind 的 `settings_schema`、source rules、defaults 和 examples。 |

模板中的资源名称必须是非空单段名称。`.`、`..`、正斜杠和反斜杠都会被拒绝；
对 `/` 做 percent-encoding 也不能绕过检查。resource URI 不映射任意宿主文件
路径或原始 Store key。Processor stage 只有 `nodes` 和 `file`；file kind 必须
使用 capability catalog 中的 canonical 值。

读取不存在、无效或无法解码的资源时，失败由 MCP resource error 返回。Tool
不会因为写入或正文省略而自动创建额外的 share、文件或隐藏 resource。

## Prompts

五个 prompt 都返回一条 `user` 文本消息；它们会引用当前 capability/schema
catalog，但不会自行读取 resource、执行 tool 或持久化定义。

| prompt | 参数 | 用途 |
| --- | --- | --- |
| `build_subscription` | 必填 `target`、`subscription_type`、`input_source`；可选 `needs_processors` | 使用当前 nodes processor catalog 起草 Subscription，并指导 preview/render。 |
| `build_file` | 必填 `kind`、`target`；可选 `referenced_resources`、`needs_script` | 使用对应 file-kind 与 file-stage schema 起草并验证 FileSpec。 |
| `write_processor_script` | 必填 `stage`、`target`、`expected_input`、`expected_output` | 使用 versioned script API 与对应 processor schema 起草 sandboxed script。 |
| `diagnose_conversion_loss` | 必填 `source_format`、`target_format`、`report_json` | 结合当前 catalog 分析转换损失并建议聚焦复现。 |
| `explain_report` | 必填 `report_json`；可选 `focus` | 按依赖、来源、render/probe statistics 和 warnings 解释 report。 |

`subscription_type`、`kind`、`stage` 和格式参数会按当前 catalog 校验。
`needs_processors` 与 `needs_script` 是布尔文本。两个 report prompt 要求有效 JSON，
并把它作为数据转义；`report_json` 最大为 64 KiB。领域细节仍以本页链接的
canonical reference 与 MCP schema resource 为准。

## Tool 结果、错误与正文省略

每个 tool 的 `outputSchema` 是成功对象与错误对象的 union。成功时
`structuredContent` 是该 tool 的成功对象；失败时 MCP result 标记为 error，
且 `structuredContent` 稳定为：

```json
{
  "error": {
    "code": "invalid_argument",
    "message": "...",
    "field": "spec.config.settings.groups"
  }
}
```

`code` 和 `message` 始终存在；适用时还会给出 `field`、`resource_kind`、
`resource_name`、`source`、`target`、`file`、`part`、`processor` 或 `path`。
调用方应按 `code` 和结构化上下文分支，不要解析人类可读 text。完整领域 code、
warning 与 report 语义见[错误与诊断参考](errors.md)。

`--max-output-bytes` 缺省为 `1048576`。第一阶段限制只适用于以下三类最终
inline `body`：

- `sandrone_convert`；
- `sandrone_render_subscription`；
- `sandrone_get_file` 的 `render` 模式。

正文未超过限制时原样返回。超过限制时不截断，而是省略 `body`，同时返回
`body_omitted: true`、实际 `body_bytes` 与配置的 `max_output_bytes`。
省略正文绝不会隐式创建 share、文件、artifact 或可读取 URI。

该限制不全局覆盖 reports、probe results、preview/traffic 输出、resource JSON、
`sandrone_get_file` 的 source/spec 输出或 capability/schema catalogs；这些结果
仍可能很大，调用方应限制自己保存、展示和转发的 MCP 结果。

## 推荐 Agent 流程

```text
inspect → list → read definition/schema → optional prompt
→ preview/validate → put/delete/render/probe
```

先用 `sandrone_inspect_capabilities` 确认当前能力，再用
`sandrone_list_resources` 发现已存定义；通过标准 MCP resource 读取定义及其
processor、file-kind 或 script schema。只有需要起草或解释时才调用 prompt。
随后用 convert/preview/validate 做无持久化检查。管理开关已启用且用户明确要求
时才 put 或 delete；最后按目标 render、get file 或 probe，并检查 structured
output 中的 report/warnings。

Preview/validate 是推荐检查，不是 put 的服务端前置条件。反过来，
subscription preview/render 只接受已存定义，所以新 subscription 通常要在用户
授权后先 put；FileSpec 可以直接以 inline `spec` 调用 `sandrone_validate_file`。

## 安全边界

- MCP 不是通用代码执行入口。所有 tool 都调用 service 层，不提供任意 Store key、
  宿主路径、网络客户端或 processor 注册面。
- `sandrone_convert` 的 remote 输入、订阅/file flow 中的远程 source，以及
  probe 都只能经受控 fetch/probe 边界执行。
- `script` processor 在 Go 内嵌 ECMAScript sandbox 中运行，不是 Node.js；
  没有任意文件系统、子进程、环境变量或通用网络能力。`permissions` 是保留
  object，当前不会授予这些宿主能力。完整契约见[脚本 API](scripting-api.md)。
- Resource URI 和 tool 名称参数只能使用单段公开名称，不能穿越为宿主路径。
- MCP 输出和 resource 定义可能包含订阅 URL、节点凭据、脚本、source reference
  或原始 warning 上下文，不能假定已经脱敏。
- 内建 Streamable HTTP listener 不终止 TLS。跨主机使用时应在可信网络或提供
  TLS 的反向代理之后部署，并保护 bearer token。
