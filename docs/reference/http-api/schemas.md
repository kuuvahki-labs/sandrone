# Schema HTTP API

## 用途与共同契约

这些路由返回 JSON Schema 或 catalog document，鉴权和错误响应遵循
[HTTP API 约定](README.md)。

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

## 使用写入 schema

按需读取 Subscription、FileSpec 和对应 processor/file-kind schema，获取字段、
枚举、必填项及示例；字段执行语义分别见
[订阅接口](subscriptions.md)、[FileSpec](../file-spec.md)和
[Processors](../processors.md)。

写入 schema 使用 `additionalProperties: false` 描述公开的写入字段集合。
HTTP handler 的未知字段处理不都等于该 schema：例如 Subscription 的时间戳
可随定义往返，未声明的顶层键也不一定被拒绝。不要依赖这种宽容行为构造新请求；
typed `config.settings` 仍由对应 driver 严格校验。

## 失败

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
## 示例

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
