# MCP 参考

本页定义 Sandrone 当前 MCP 入口的公开契约：transport、鉴权、tools、
resources、prompts、输出限制与安全边界。启动 flags 见 [CLI 参考](cli.md)，
领域字段继续以 [FileSpec](file-spec.md)、[Processors](processors.md)、
[脚本 API](scripting-api.md)和 [HTTP API](http-api/README.md)为准。

## Transport 与启动

Sandrone 支持两种 MCP transport：

| transport | 启动方式 | 边界 |
| --- | --- | --- |
| `stdio` | `sandrone serve mcp` | 缺省方式；协议使用进程标准输入与标准输出。 |
| `streamable-http` | `sandrone serve mcp --transport streamable-http` | 挂载到 Sandrone HTTP listener，缺省路径为 `/mcp`。 |

`sandrone serve all` 固定使用 `streamable-http`，在一个 listener 上同时提供
HTTP API、可选 Web UI 和 MCP。`--path` 可以修改 MCP 路径，但必须以 `/` 开头。
Streamable HTTP handler 使用 JSON response mode。

stdio 不使用 HTTP bearer token；它依赖启动进程的用户权限、工作目录和数据目录
隔离。Streamable HTTP 复用 HTTP server 的静态 bearer token：

```http
Authorization: Bearer <token>
```

只要配置了 token，该 header 就必须精确匹配。绑定非 loopback 地址时必须提供
token；完整启动与鉴权规则见 [CLI 参考的 serve 章节](cli.md#serve)和
[HTTP API 鉴权](http-api/README.md#鉴权)。

## Tool 注册策略

缺省只注册五个只读 tool。两个管理 tool 只有在配置同时满足以下条件时才出现：

- readonly 为 `false`；
- `allow-management-tools` 为 `true`。

CLI 中需要显式使用
`--readonly=false --allow-management-tools`。管理 tool 是幂等保存操作，但会修改
当前 data dir 中的资源。

MCP SDK 根据当前 Go 输入类型发布 tool input schema。调用方应以 MCP
`tools/list` 返回的 schema 为准；本页只说明输入用途，不复制领域对象的完整字段。

## Tools

### 始终注册

| tool | 输入用途 | 输出用途 |
| --- | --- | --- |
| `sandrone_convert` | `from_format`、`to_format`，以及 inline `content` 或受控 `remote` 输入；可带 parse/render processors、render options 和 metadata。 | `content_type`、渲染 `body` 与 `report`。 |
| `sandrone_get_file` | 已保存的单段 `file` 名称、可选 `target`；`mode` 缺省为 `render`，也可为 `spec`。 | render 模式返回 `content_type`、`body`、`report`；spec 模式返回 `spec` 和对应 `resource_uri`。 |
| `sandrone_probe_nodes` | 一个 `NodeInput`，以及 layer、method、core、目标、timeout、attempts、concurrency 和 cache TTL 等探测参数。 | 节点级 `results` 与汇总 `report`。 |
| `sandrone_validate_file` | 已保存的 `file` 名称或 inline `FileSpec`，以及可选 `target`。 | 生成与校验结果的 `report`，不发布文件正文。 |
| `sandrone_inspect_capabilities` | 无输入。 | 当前 runtime 的 `capabilities` 与 `report`。 |

`sandrone_convert` 的格式与有损边界见[格式与能力参考](capabilities.md)；
processor 声明见 [Processors 参考](processors.md)。文件生成与校验始终经过完整
file flow，不绕过 driver 或 file-stage processor。

### 仅管理模式注册

| tool | 输入用途 | 输出用途 |
| --- | --- | --- |
| `sandrone_put_subscription` | 一个具名 `Subscription` 定义。 | 保存成功返回 `{"ok": true}`。 |
| `sandrone_put_file` | 一个具名 `FileSpec` 定义。 | 保存成功返回 `{"ok": true}`。 |

Subscription 的类型与保存语义见
[HTTP 订阅资源参考](http-api/subscriptions.md)；`FileSpec.kind`、source、
typed config 与 processors 见 [FileSpec 参考](file-spec.md)。

## Resources

所有 MCP resource 都返回 `application/json` 文本。当前有三类 URI：

| URI | 内容 |
| --- | --- |
| `sandrone://capabilities` | 当前 parser、renderer、processor、typed-file driver 与 probe capability summary。 |
| `sandrone://subscriptions/{name}` | 已保存的 `Subscription` 定义。 |
| `sandrone://files/{name}` | 已保存的 `FileSpec` 定义，不是编译后的文件正文。 |

模板中的 `{name}` 必须是非空单段资源名。`.`、`..`、正斜杠和反斜杠都会被
拒绝；对 `/` 做 percent-encoding 也不能绕过检查。resource URI 不映射任意
宿主文件路径或原始 Store key。

读取不存在、无效或无法解码的资源时，失败由 MCP resource error 返回。领域错误
与 warning 的含义见[错误与诊断参考](errors.md)。

## Prompts

当前 prompts 都不接受参数，并返回一条 `user` 文本消息：

| prompt | 用途 |
| --- | --- |
| `convert_nodes` | 指导选择 `sandrone_convert`、`json-nodes` IR 输出或已保存 FileSpec 的 `sandrone_get_file`。 |
| `diagnose_conversion_loss` | 指导比较 parse/render reports、warnings 与 lost fields，解释目标格式限制。 |
| `design_mihomo_file` | 指导设计 Mihomo `FileSpec`，使用 source、file-stage script 和 `api.subscription.produce`。 |
| `design_sing_box_file` | 指导设计 sing-box `FileSpec`，使用 source、file-stage script 和 `api.subscription.produce`。 |
| `explain_report` | 指导按 dependencies、source references、render/probe statistics 和 warnings 解释 report。 |

Prompts 只提供静态工作指引，不读取资源、不执行 tool，也不代表生成的 FileSpec
已经通过 service 校验。脚本可用 API 与 sandbox 以
[脚本 API 参考](scripting-api.md)为准。

## 输出限制

`--max-output-bytes` 缺省为 `1048576`。它当前只限制
`sandrone_convert` 与 `sandrone_get_file` render 模式的 inline `body`：

- `body` 未超过限制时原样返回；
- 超过限制时响应省略 `body` 字段，`content_type` 和 `report` 仍返回；
- server 不会为被省略的 body 隐式创建文件、share 或 resource URI。

该限制不截断 probe results、reports、resource JSON 或 FileSpec。调用方仍应限制
自己保存、展示和转发的 MCP 结果。

Tool 输入校验或 service 调用失败时，由 MCP SDK 返回 tool error；不要依赖人类
可读 message 做程序分支。稳定领域错误与诊断边界见
[错误与诊断参考](errors.md)。

## 安全边界

- `sandrone_convert` 的 remote 输入、文件生成或校验中的远程 source/subscription，
  以及 `sandrone_probe_nodes` 都可以发起受控网络访问；具体 fetch/probe 限制由
  service 执行，tool 不获得通用网络客户端。
- FileSpec 与 processor 始终经过 service。脚本不能访问任意文件系统、子进程或
  网络，只能使用注入的窄 API。
- Resource URI 中的名称以及 tool 接受的 subscription/file 名称只能是单段 ID，
  不能用作宿主路径穿越。
- 管理 tools 缺省不注册；启用后，能连接该 MCP server 的客户端可以覆盖同名
  subscription 或 file。
- MCP 输出和 resource 定义可能包含订阅 URL、节点凭据、脚本、source reference
  或原始 warning 上下文，不能假定已经脱敏。
- 内建 Streamable HTTP listener 不终止 TLS。跨主机使用时应在可信网络或提供
  TLS 的反向代理之后部署，并保护 bearer token。
- stdio 客户端继承启动进程对 data dir 和网络的权限；只应连接可信客户端。
