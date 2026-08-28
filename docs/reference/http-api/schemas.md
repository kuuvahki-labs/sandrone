# Schema HTTP API

## 用途与共同契约

本页是 Sandrone HTTP schema 路由、响应和失败的 canonical 参考。所有接口都属于
受保护的 `/v1/*` 管理 API，继承 [HTTP API 鉴权](README.md#鉴权)：配置 token
时必须发送 `Authorization: Bearer <token>`。成功响应是
`Content-Type: application/json` 的 JSON Schema 或 catalog document。

HTTP 与 MCP schema 由同一个 owner-maintained catalog 序列化；二者描述相同的
canonical stage、processor、file kind、Subscription、FileSpec 和脚本 API，
调用方不应维护另一份手写 schema。HTTP 客户端使用本页路由；MCP 客户端使用
[MCP resources 与 schema templates](../mcp.md#resources-与-schema-templates)。

字段的执行语义不在本页重复：

- processor stage、声明顺序、参数与失败边界见
  [Processors 参考](../processors.md)；
- `FileSpec` source、typed `config`、driver 和生成语义见
  [FileSpec 参考](../file-spec.md)；
- script config、envelope、注入 API 与 sandbox 见
  [JavaScript 脚本 API](../scripting-api.md)。

## 路由

| 路由 | 成功响应的用途 |
| --- | --- |
| `GET /v1/schemas` | 列出 schema catalog；每项带说明与对应 `href`。 |
| `GET /v1/schemas/processors` | 列出公开 processor 的 canonical `stage`、`type`、effects、说明和详情 URI。 |
| `GET /v1/schemas/processors/{stage}/{type}` | 返回一个公开 processor 的 `params_schema`、effects、示例和 error codes。 |
| `GET /v1/schemas/file-kinds` | 列出 canonical file kind、media type、syntax、settings 支持状态与详情 `href`。 |
| `GET /v1/schemas/file-kinds/{kind}` | 返回一个 canonical file kind 的 `settings_schema`、source rules、defaults 和示例。 |
| `GET /v1/schemas/script-api/v1` | 返回版本 1 的 script config、envelope、注入方法、来源与 sandbox schema。 |
| `GET /v1/schemas/subscription` | 返回完整、封闭的具名 Subscription 写入 schema。 |
| `GET /v1/schemas/file-spec` | 返回完整、封闭的具名 FileSpec 写入 schema。 |

所有路由都没有请求体和查询参数。processor stage 只有 `nodes`、`file`；
`{type}` 必须来自 processor catalog。file kind 只有 `static`、`mihomo`、
`sing-box`、`shadowrocket`；运行时先读根目录或对应索引，不要从版本号猜测可用项。

## Subscription 写入 schema

`GET /v1/schemas/subscription` 返回 `type: "object"`、
`additionalProperties: false`，必填字段为 `name` 与 `type`。其完整 properties
为：

| 字段 | JSON Schema 形状 |
| --- | --- |
| `name` | `string` |
| `display_name` | `string` |
| `type` | `string` enum：`remote`、`local`、`collection` |
| `format` | `string` enum：`auto`、`uri`、`uri-list`、`base64`、`mihomo`、`sing-box`、`json-nodes` |
| `content` | `string` |
| `remote` | 封闭 object；必填 `url: string`，可选 `user_agent: string`、`proxy: string`、非负整数 `timeout_ms`、`cache_ttl_seconds` |
| `inputs` | `NodeInput` array，形状见下节 |
| `processors` | processor array；每项是封闭 object，必填 `type: string`，可选 `name: string`、`stage` enum `nodes`/`file`、`enabled: boolean`、开放 object `params` |
| `nodes` | 开放 node object array |
| `snapshot_ttl_seconds` | 非负整数；省略继承项目默认，`0` 关闭订阅执行快照缓存 |
| `meta` | string-to-string object |

这个 schema 是 Agent/HTTP 写入面；不接受未列出的顶层键。`type` 对字段组合的
实际规范化与保存行为见[订阅 HTTP API](subscriptions.md)。

### `NodeInput` 结构

每项是 `additionalProperties: false` 的 object，必填 `name` 和 `type`：

| 字段 | JSON Schema 形状 |
| --- | --- |
| `name` | `string` |
| `type` | enum：`inline_nodes`、`inline`、`local`、`remote`、`ref`、`subscription` |
| `ref` | 开放 object |
| `format` | enum：`uri`、`uri-list`、`base64`、`mihomo`、`sing-box`、`json-nodes` |
| `target`、`content`、`path`、`url`、`user_agent`、`proxy` | `string` |
| `nodes` | 开放 node object array |
| `timeout_ms`、`cache_ttl_seconds` | 非负整数 |
| `required` | `boolean` |
| `meta` | string-to-string object |

## 具名 FileSpec 写入 schema

`GET /v1/schemas/file-spec` 返回 `type: "object"`、
`additionalProperties: false`，必填字段为 `name`、`kind`、`source`。完整
properties 为：

| 字段 | JSON Schema 形状 |
| --- | --- |
| `name`、`display_name` | `string` |
| `kind` | enum：`static`、`mihomo`、`sing-box`、`shadowrocket` |
| `source` | 封闭 object；可选 `type` enum `inline`/`remote`、`content: string`、`remote`（与 Subscription schema 相同的封闭 remote object） |
| `config` | 封闭 object；只允许 `subscriptions: string[]` 与开放 object `settings` |
| `processors` | 与 Subscription schema 相同的 processor array |
| `meta` | string-to-string object |

`kind` 必须使用上表的 canonical 值。各 kind 对 `source`、`config.settings` 和
processor stage 的进一步约束由对应 driver 严格解码；权威语义只见
[FileSpec 参考](../file-spec.md)，运行时 shape 通过
`GET /v1/schemas/file-kinds/{kind}` 读取。

## 失败

- token 缺失或不匹配时返回 `401` JSON error，见
  [鉴权](README.md#鉴权)。
- `{stage}` 不是 `nodes`/`file`、公开 `{type}` 不存在、canonical `{kind}`
  不存在或 path 参数无效时返回结构化 JSON error：

  ```json
  {
    "error": {
      "code": "invalid_argument",
      "message": "schema not found"
    }
  }
  ```

- 不存在的 script API 版本、额外 path segment 或未注册路径由 router 返回
  `404`；错误方法返回 `405`。这些 router 响应可能是 `text/plain`，不能假定
  都是 JSON。
- schema 中 `additionalProperties: false` 表示写入对象的未知键不属于公开
  wire contract。提交 Subscription/FileSpec 时应先按返回 schema 验证；
  handler/service 拒绝时的结构化 error 形状与 status 映射见
  [错误与诊断参考](../errors.md#http-handler-error-envelope)。

## 示例

环境变量只使用合成地址和 token：

```sh
export SANDRONE_URL="http://127.0.0.1:1137"
export SANDRONE_TOKEN="example-token"
```

列出 schema 根目录与 processor/file-kind 索引：

```sh
curl -sS "$SANDRONE_URL/v1/schemas" \
  -H "Authorization: Bearer $SANDRONE_TOKEN"

curl -sS "$SANDRONE_URL/v1/schemas/processors" \
  -H "Authorization: Bearer $SANDRONE_TOKEN"

curl -sS "$SANDRONE_URL/v1/schemas/file-kinds" \
  -H "Authorization: Bearer $SANDRONE_TOKEN"
```

读取一个 processor 与一个 file-kind schema：

```sh
curl -sS "$SANDRONE_URL/v1/schemas/processors/nodes/rename" \
  -H "Authorization: Bearer $SANDRONE_TOKEN"

curl -sS "$SANDRONE_URL/v1/schemas/file-kinds/mihomo" \
  -H "Authorization: Bearer $SANDRONE_TOKEN"
```

读取具名写入 schema：

```sh
curl -sS "$SANDRONE_URL/v1/schemas/subscription" \
  -H "Authorization: Bearer $SANDRONE_TOKEN"

curl -sS "$SANDRONE_URL/v1/schemas/file-spec" \
  -H "Authorization: Bearer $SANDRONE_TOKEN"
```
