# Sandrone 操作参考

HTTP 调用格式为 `scripts/sandrone-api.sh METHOD PATH [BODY_FILE|-]`。JSON 可通过
标准输入 `-` 或权限受限的文件传入。MCP tool 名称可能带客户端 namespace 前缀。

能力不确定时调用 `GET /v1/inspect` 或 `sandrone_inspect`；HTTP 服务版本见
`/version`。写入陌生结构前，从 `GET /v1/schemas` 或 `sandrone://schemas` 进入
对应 schema；格式能力见 `/v1/capabilities/formats` 或
`sandrone://capabilities/formats`。已知资源名时直接读取，未知时再列举；分页时原样
使用返回的 cursor，并保持过滤条件不变。

## 资源操作

| 操作 | HTTP | MCP |
| --- | --- | --- |
| 转换临时输入 | `POST /v1/convert` | `sandrone_convert` |
| 列出 Subscription/FileSpec | `GET /v1/subscriptions` / `GET /v1/files` | `sandrone_list_resources` |
| 读取 Subscription | `GET /v1/subscriptions/{name}` | `sandrone://subscriptions/{name}` |
| 保存 Subscription | `POST /v1/subscriptions` | `sandrone_put_subscription` |
| preview Subscription | `POST /v1/subscriptions/{name}/preview` | `sandrone_preview_subscription` |
| render Subscription | `POST /v1/subscriptions/{name}/render` | `sandrone_render_subscription` |
| 读取 FileSpec | `GET /v1/files/{name}?mode=spec` | `sandrone://files/{name}` |
| 保存 FileSpec | `POST /v1/files` | `sandrone_put_file` |
| render 文件 | `GET /v1/files/{name}?response=json` | `sandrone_get_file` |
| 删除资源 | 对应资源的 `DELETE` 端点 | `sandrone_delete_subscription` / `sandrone_delete_file` |

保存操作会覆盖同名完整定义；编辑时保留无关字段和 processor 顺序。按名称执行
preview/render 只适用于已保存的 Subscription。FileSpec 的 `spec`、`source`、
`render` 模式分别返回定义、基础内容和最终内容。

`POST /v1/convert` 可带 processor 转换临时输入，但不会创建资源；公开的
`GET /convert` 是不带 processor 的便捷接口。具体请求结构查看
[HTTP API 文档](https://github.com/kuuvahki-labs/sandrone/tree/main/docs/reference/http-api)。

## Processor 脚本

先读取对应 stage 的 processor schema；脚本还需读取 script API schema。脚本运行在
注入 Sandrone API 的 ECMAScript 环境中，不是 Node.js；file-stage processor 按声明
顺序执行。写法与登记方式见[脚本指南](https://github.com/kuuvahki-labs/sandrone/blob/main/docs/how-to/write-processor-script.md)，
可复用实现见 [examples/scripts](https://github.com/kuuvahki-labs/sandrone/tree/main/examples/scripts)
和 [examples/script-templates](https://github.com/kuuvahki-labs/sandrone/tree/main/examples/script-templates)。
草稿诊断可使用本地 CLI `sandrone diagnose`；HTTP 和 MCP 不直接提供 diagnose。
