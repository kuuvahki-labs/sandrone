# 转换、校验与能力检查

## 用途

本页说明一次性节点转换、无持久化校验和运行时能力检查。公开转换位于
`/convert`，不使用 bearer token；校验和能力检查位于 `/v1/*`，启用 token
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
`json-nodes`；输出格式为 `mihomo-proxies`、`shadowrocket-proxies`、
`sing-box-outbounds`、`json-nodes` 和 `uri-list`。运行时的权威列表应通过
能力检查读取。

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

## POST /v1/validate

### 用途

真实执行节点输入或文件生成链以返回结构化诊断，但不保存输入、生成内容或
report。

### 请求

请求体支持两种使用模式；调用方不应混用：

| 模式 | 字段 | 契约 |
| --- | --- | --- |
| 文件校验 | `file`, `spec`, `target` | `file` 引用已保存的单 segment 文件名；`spec` 是内联 `FileSpec`。至少提供其一。 |
| 节点校验 | `format`, `content`, `remote`, `target`, `processors` | `content` 与 `remote` 互斥；内联正文需要 `format`，远程输入可省略格式自动检测。 |

文件字段的完整语义见 [FileSpec 参考](../file-spec.md)。同时给出 `file` 与
`spec` 时以内联 `spec` 为准；若 `spec.name` 为空，则用 `file` 补入名称。
文件模式会读取 source、解析 typed 配置及其订阅依赖，并运行
`FileSpec.processors`，但不会保存生成结果。

节点模式在规范化后检查节点语义；`processors` 只作用于通过第一轮检查的节点，
处理后再校验一次。`target` 为校验和 processor 提供目标上下文，不负责选择
parser。若文件字段与节点字段混用，handler 优先进入文件模式；调用方应选择一种
模式，避免依赖该优先级。

### 响应

校验流程完成时返回 `200`：

```json
{
  "ok": false,
  "counts": {
    "input": 1,
    "valid": 0,
    "invalid": 1,
    "error": 1,
    "warning": 0
  },
  "issues": [
    {
      "severity": "error",
      "stage": "normalized",
      "code": "node_validation_invalid",
      "message": "port must be between 1 and 65535",
      "node_index": 0,
      "field": "port"
    }
  ],
  "warnings": []
}
```

`ok: false` 表示发现 validation issue，不是 HTTP error。`counts` 的键固定为
`input`、`valid`、`invalid`、`error`、`warning`；`issues` 可带节点、字段、
stage 和 target 上下文。文件校验成功时当前 counts 为零，`issues` 可为
`null`；`warnings` 仍总是数组。

### 失败与安全边界

- 两种模式都未被选中时返回 `400 invalid_argument`；仅提供 `content` 而没有
  `format` 不会隐式启用节点模式。
- 无法读取或解析输入、文件依赖缺失、依赖成环或 processor 执行失败属于操作
  error，而不是 `ok: false` 的 validation issue。
- 校验会真实读取声明的受控 store 或远程 source，因此可能产生网络访问和缓存
  读取；它只承诺不持久化本次生成结果。

### 最小示例

```sh
curl -sS "$SANDRONE_URL/v1/validate" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "format": "uri-list",
    "content": "ss://aes-128-gcm:example-password@proxy.example.com:8388#example-node"
  }'
```

## GET /v1/inspect

### 用途

读取当前进程实际注册的 parser、renderer、processor、probe backend 和 store
摘要。它适合做运行时能力发现，不应由客户端根据版本号猜测能力。

### 请求

没有请求体。可选 query `target` 会传入 service；当前内建 service 返回全局能力
摘要，不按该值筛选，因此通常应省略。

### 响应

成功返回 `200`：

```json
{
  "capabilities": {
    "parse_formats": [
      "uri",
      "uri-list",
      "base64",
      "mihomo",
      "sing-box",
      "json-nodes"
    ],
    "render_formats": [
      "mihomo-proxies",
      "shadowrocket-proxies",
      "sing-box-outbounds",
      "json-nodes",
      "uri-list"
    ],
    "store_configured": true
  }
}
```

上例只展示稳定的顶层形状；实际 `capabilities` 还包含：

- `capabilities`：带 direction、format 与字段状态的 adapter 声明；
- `node_processors`、`file_processors`；
- `probe_methods`，以及 backend 可报告时的 `probe_backends`；
- 配置 metadata store 时的 `subscriptions`、`files` 数量与
  `store_configured: true`。

精确字段能力和 `supported`、`lossy`、`raw_only` 语义见
[格式与能力参考](../capabilities.md)。响应不包含内部 report。

### 失败与安全边界

该接口只返回已注册能力和资源数量，不返回订阅、文件正文、token 或远程输入
内容。它仍是受保护的 `/v1/*` 管理接口。

### 最小示例

```sh
curl -sS "$SANDRONE_URL/v1/inspect" \
  -H "Authorization: Bearer $SANDRONE_TOKEN"
```
