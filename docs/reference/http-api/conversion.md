# 转换与能力检查

## 用途

本页说明一次性节点转换和运行时能力检查。公开的精简转换是
`GET /convert`，不使用 bearer token；完整转换是 `POST /v1/convert`，支持
processor 与完整 report。完整转换和能力检查位于 `/v1/*`，启用 token
时必须遵循[通用鉴权约定](README.md#鉴权)。

输入/输出格式及字段兼容性以[格式与能力参考](../capabilities.md)为准；
`ProcessorSpec` 的 stage、顺序和参数见[Processors 参考](../processors.md)，
其中脚本 processor 的 sandbox 与 API 另见
[Scripting API](../scripting-api.md)。完整错误码、validation issue 和 warning
结构见[错误与诊断参考](../errors.md)。

## GET /convert

### 用途

公开执行一次 `parse -> render`，返回转换后的正文；不创建订阅、文件或 report
资源，也不接受 processor。

### 请求

查询参数为：

| 字段 | 类型 | 契约 |
| --- | --- | --- |
| `content` | string | URL 编码后的内联节点正文；与 `url` 必须且只能提供一个，最大 64 KiB。 |
| `url` | string | URL 编码后的远程 HTTP(S) 订阅地址；与 `content` 必须且只能提供一个。 |
| `from_format` | string | 可选输入格式。内联输入默认 `uri-list`；远程输入默认 `auto`。 |
| `to_format` | string | 必填的输出 renderer 格式。 |
| `response` | string | 大小写不敏感地等于 `json` 时返回 JSON envelope；其它值或省略时直接返回正文。 |

每个参数最多出现一次；未知、重复或空的必填参数返回 `invalid_argument`。接口
不接受 processor、meta、代理、User-Agent、超时或缓存覆盖参数。

远程自动检测会优先识别带 `outbounds`/`endpoints` 的 sing-box 或带 `proxies`
的 Mihomo 文档，再尝试严格的 Base64 与 URI list。显式给出格式后只调用该
parser，不做 fallback。

当前输入格式为 `uri`、`uri-list`、`base64`、`mihomo`、`sing-box` 和
`json-nodes`；输出格式为 `base64`、`mihomo-proxies`、
`shadowrocket-proxies`、`sing-box-outbounds`、`json-nodes` 和 `uri-list`。
运行时的权威列表应通过能力检查读取。

### 响应

默认成功返回 `200`、目标正文及 renderer 的 `Content-Type`。例如
`to_format=mihomo-proxies` 会直接返回 Mihomo YAML。

使用 `response=json` 时返回：

```json
{
  "content_type": "application/json",
  "body": "[\n  {\n    \"name\": \"example-node\",\n    \"type\": \"ss\",\n    \"server\": \"proxy.example.com\",\n    \"port\": 8388,\n    \"password\": \"example-password\",\n    \"cipher\": \"aes-128-gcm\",\n    \"source_format\": \"uri\"\n  }\n]",
  "response": {},
  "warnings": []
}
```

`body` 是目标格式正文的字符串表示，`content_type` 描述该正文，而外层 HTTP
响应仍是 `application/json`。`warnings` 总是数组，聚合 parse 与 render
阶段的本次诊断；raw 模式不返回 warning，调用方需要诊断信息时必须选择 JSON
envelope。HTTP 不返回或持久化完整 report，`response` 当前是空的保留响应元
数据。

成功和失败响应都设置 `Cache-Control: no-store`、`Referrer-Policy:
no-referrer` 和 `X-Content-Type-Options: nosniff`。

### 失败与安全边界

- `content` 与 `url` 同时出现或都未出现、`to_format` 缺失、格式未注册或查询
  参数无效时返回 `invalid_argument`。
- 远程抓取只允许公网可路由的 HTTP(S) 目标。服务会检查 DNS 结果和每次重定向，
  拒绝 loopback、私网、链路本地及其它非公网地址，并禁用代理。
- 远程响应限制为 16 MiB，默认超时为 15 秒。
- 单个节点可被跳过并形成 warning；非空批次最终没有可解析、有效或可渲染节点
  时，操作失败。
- parse 和 render 的具体错误及当前 HTTP status 映射见[错误与诊断参考](../errors.md)。
- 该接口无需 bearer token。公开部署时任何人都可以消耗服务器的抓取和转换资源；
  运维方应在反向代理或网络边界限制访问频率。
- `content` 和 `url` 会出现在浏览器历史、访问日志及中间代理中。不要把不能接受
  这些位置留存的节点凭据或订阅 token 放入公开转换 URL。

### 最小示例

```sh
curl -sS -G "$SANDRONE_URL/convert" \
  --data-urlencode 'content=ss://aes-128-gcm:example-password@proxy.example.com:8388#example-node' \
  --data-urlencode 'to_format=mihomo-proxies'
```

远程订阅并返回 JSON envelope：

```sh
curl -sS -G "$SANDRONE_URL/convert" \
  --data-urlencode 'url=https://subscription.example/nodes?token=example-token' \
  --data-urlencode 'to_format=json-nodes' \
  --data-urlencode 'response=json'
```

示例中的域名、token 与凭据均为虚构值。手工构造浏览器 URL 时必须对 `#`、`+`、
`&`、换行和嵌套远程 URL 做 percent-encoding；优先使用 `URLSearchParams` 或
`curl --data-urlencode`，不要直接拼接。

## POST /v1/convert

### 与公开转换的区别

这个受保护接口执行完整 `parse -> parse processors -> render -> render
processors` 流程。与 `GET /convert` 相比，它：

- 用 JSON body 传递字符串 `content` 或受控 `remote`，不会把节点正文放入 URL；
- 接受 parse/render processor、render options 和 metadata；
- 总是返回字符串 `body` 与完整结构化 `report`；
- 继承 `/v1/*` bearer 鉴权，不是公开转换入口。

processor 的 stage、声明顺序和 config 语义只见
[Processors 参考](../processors.md)；脚本 processor 只见
[JavaScript 脚本 API](../scripting-api.md)。

### 请求

| 字段 | 类型 | 契约 |
| --- | --- | --- |
| `to_format` | string | 必填 renderer 格式。 |
| `from_format` | string | inline `content` 时必填；remote 输入省略时允许自动检测。 |
| `content` | string | inline 节点正文；与 `remote` 互斥。HTTP wire 上始终是字符串，不是 Base64 bytes。 |
| `remote` | object | 与 `content` 互斥；`url` 必填，可选 `user_agent`、`proxy`、非负 `timeout_ms`、`cache_ttl_seconds`。 |
| `parse_processors` | array | 可选 parse 后的 processor 声明列表，按声明顺序执行。 |
| `render_processors` | array | 可选 render flow 的 processor 声明列表，按声明顺序执行。 |
| `options` | object | 可选 render options；当前公开字段是 `format`。未给出时使用 `to_format`。 |
| `meta` | object | 可选 string-to-string metadata，传入本次 parse/report 上下文。 |

`content` 与 `remote` 必须且只能提供一个。格式名应先用
[`GET /v1/inspect`](#get-v1inspect) 或
[schema catalog](schemas.md)发现；字段兼容性见[格式与能力](../capabilities.md)。
convert 属于临时输入；`remote.cache_ttl_seconds` 会被校验但不会创建或读取持久
remote-fetch cache。持久缓存只用于已保存 Subscription/File 的执行作用域。

### 响应

成功返回 `200`：

```json
{
  "content_type": "application/json",
  "body": "[\n  {\n    \"name\": \"skill-example\",\n    \"type\": \"ss\",\n    \"server\": \"proxy.example.invalid\",\n    \"port\": 8388,\n    \"password\": \"example-password\",\n    \"cipher\": \"aes-128-gcm\",\n    \"source_format\": \"uri\"\n  }\n]",
  "report": {
    "kind": "convert",
    "status": "ok",
    "created_at": "2026-01-01T00:00:00Z",
    "lossy": false,
    "source_refs": [
      {
        "kind": "format",
        "name": "uri-list"
      }
    ],
    "warnings": [],
    "render": {
      "success_count": 1,
      "lost_fields": 0
    }
  }
}
```

`body` 始终是目标正文的 JSON string；`content_type` 描述该字符串承载的目标
格式，外层 HTTP response 仍是 JSON。`report` 不做 MCP output-size 省略，包含
本次转换的完整 kind/status/time、refs/dependencies/source refs、lossy、
warnings 与 render statistics；不存在的可选字段按 JSON `omitempty` 省略。
完整字段与敏感信息边界见[错误与诊断](../errors.md#report)。

### 失败与安全边界

- 两种输入同时存在、格式或 processor config 无效时返回结构化 error。没有输入
  时当前由 parse flow 返回 `parse_failed`；调用方应在发送前按 schema 约束验证。
- remote 输入可能执行网络抓取并使用请求指定的受控 User-Agent、proxy、timeout
  与 cache TTL；它仍受 Sandrone fetcher 边界约束，不提供任意网络 API。
- processor 可执行受控 probe、订阅/file 读取或 sandboxed script 能力，具体
  副作用以对应 processor schema 和 canonical 参考为准。
- `body`、report、warnings 和 source refs 可能包含节点凭据、订阅 URL 或原始
  上下文；该接口不构成脱敏边界。

### 合成示例

```sh
curl -sS "$SANDRONE_URL/v1/convert" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "from_format": "uri-list",
    "to_format": "json-nodes",
    "content": "ss://aes-128-gcm:example-password@proxy.example.invalid:8388#example-node",
    "parse_processors": [
      {
        "type": "rename",
        "stage": "nodes",
        "params": {
          "mode": "prefix",
          "value": "skill-"
        }
      }
    ],
    "options": {
      "format": "json-nodes"
    },
    "meta": {
      "caller": "example-skill"
    }
  }'
```

## GET /v1/inspect

### 用途

读取当前进程实际注册的 parser、renderer、processor、file kind、probe backend
和 store 摘要。它只承担轻量运行时发现；字段级格式能力和 schema 使用响应中的
catalog 路径按需读取。

### 请求

没有请求体或查询参数。

### 响应

成功返回 `200`：

```json
{
  "formats": {
    "parse": ["base64", "json-nodes", "mihomo", "sing-box", "uri", "uri-list"],
    "render": ["base64", "json-nodes", "mihomo-proxies", "shadowrocket-proxies", "sing-box-outbounds", "uri-list"]
  },
  "processors": {
    "nodes": ["dedup", "filter", "probe", "quick_settings", "rename", "script", "sort"],
    "file": ["json_patch", "merge", "script", "template", "yaml_patch"]
  },
  "file_kinds": ["static", "mihomo", "sing-box", "shadowrocket"],
  "probe": {
    "methods": ["tcp_connect", "udp_ntp", "url_test"],
    "backends": [{"method": "tcp_connect", "name": "tcp_connect"}]
  },
  "store": {"configured": true, "subscriptions": 0, "files": 0},
  "catalogs": {
    "formats": "/v1/capabilities/formats",
    "schemas": "/v1/schemas",
    "processors": "/v1/schemas/processors",
    "file_kinds": "/v1/schemas/file-kinds"
  }
}
```

`probe.methods` 只列当前实际注册 backend 支持的方法；`probe.backends` 的具体
数量随构建和配置变化。未配置 store 时只返回 `configured: false`，不返回数量。
精确字段能力通过 `catalogs.formats` 的索引和详情读取；`supported`、`lossy`、
`raw_only` 语义见[格式与能力参考](../capabilities.md)。响应不包含字段 catalog
或内部 report。

### 失败与安全边界

该接口只返回已注册名称、backend 摘要和资源数量，不返回订阅、文件正文、token
或远程输入内容。读取 store 数量失败时请求整体失败，不伪装为零。它仍是受保护
的 `/v1/*` 管理接口。

### 最小示例

```sh
curl -sS "$SANDRONE_URL/v1/inspect" \
  -H "Authorization: Bearer $SANDRONE_TOKEN"
```
