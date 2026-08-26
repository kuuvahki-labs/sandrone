# 订阅 HTTP API

## 用途

订阅资源保存节点输入及其 nodes-stage 处理链。`remote` 保存受控远程输入，
`local` 保存原文，`collection` 按顺序引用其他输入。保存定义不会预先抓取、
解析或物化节点；需要观察处理结果时使用 preview，需要读取远程服务声明的套餐
用量时使用 traffic。

本页中的接口都属于受保护的 `/v1/*` 管理 API。服务启用 token 鉴权时，请求须
携带 `Authorization: Bearer <token>`。

## 接口

### `GET /v1/subscriptions`

列出订阅摘要。请求没有正文。

成功时返回 `200` 和 `{ "items": [...] }`。每项以 `kind: "subscription"`
标识资源，并可包含 `name`、`display_name`、`type`、`format`、`size`、
`created_at`、`updated_at`、`meta` 或存储解码 `warning`。列表不物化节点，
也不包含 preview 计数或 traffic。

### `POST /v1/subscriptions`

创建或覆盖同名订阅。请求正文是一个订阅定义：

| 字段 | 契约 |
| --- | --- |
| `name` | 必填资源名，必须是单个 path segment。 |
| `display_name` | 可选显示名；保存时去除首尾空白，不参与引用或路由。 |
| `type` | 必填；接受 `remote`、`local` 或 `collection`。 |
| `format` | `remote`/`local` 的输入格式；省略或使用 `auto` 时自动检测，格式能力见[格式与能力参考](../capabilities.md)。 |
| `content` | `local` 的原文字符串。 |
| `remote` | `remote` 的抓取描述；`remote.url` 必填，另可给出 `user_agent`、`proxy`、`timeout_ms`、`cache_ttl_seconds`。 |
| `inputs` | `collection` 的有序输入列表；引用订阅时使用 `ref.kind: "subscription"` 与 `ref.name`。 |
| `processors` | 按声明顺序执行的处理链；字段和失败语义见 [Processors 参考](../processors.md)。 |
| `render_cache_ttl_seconds` | 可选非负整数；省略继承 runtime subscription-render 默认，显式 `0` 关闭，正数覆盖。 |
| `meta` | 可选字符串键值元数据。 |
| `created_at`、`updated_at` | 可选 RFC 3339 时间戳，随定义往返。 |

服务会按 `type` 规范化互斥字段：`remote` 要求 `remote.url` 并清除
`content`/`inputs`；`local` 清除 `remote`/`inputs`；`collection` 清除
`remote`/`content`/`format`。成功时返回 `201`：

```json
{
  "ok": true
}
```

### `GET /v1/subscriptions/{name}`

返回已保存的完整订阅定义，成功状态为 `200`。该读取不会抓取远程内容、解析
节点或执行 processors；本地订阅的 `content` 仍是原文字符串。

### `DELETE /v1/subscriptions/{name}`

物理删除订阅定义，成功时返回 `200`：

```json
{
  "ok": true
}
```

删除不级联：引用它的 collection、文件或 share 不会被自动删除，之后解析这些
引用时才会失败。

### `POST /v1/subscriptions/{name}/preview`

读取已保存定义，比较该订阅自身 nodes-stage processors 执行前后的节点。请求
正文可省略，也可传入字符串参数和刷新标记：

```json
{
  "args": {
    "environment": "test"
  },
  "refresh": true
}
```

查询参数 `arg.<key>=<value>` 也会进入脚本 `input.args`；正文 `args` 中的同名
键覆盖查询参数。脚本参数的完整合并规则见
[JavaScript 脚本 API](../scripting-api.md)。

成功时返回 `200`，包含：

- `subscription_name`、可选 `type` 与 `format`；
- `before_count`、`after_count`；
- `status_counts`，键固定为 `added`、`modified`、`removed`、`unchanged`；
- `nodes` 差异；每项含本次物化链路内稳定的 `runtime_id`、`status`，以及
  适用的 `before`、`after`；
- 有 `after` 的节点可带 `target_names.shadowrocket`，表示真实
  Shadowrocket renderer 归一化后的目标名；空字符串表示该节点会被跳过；
- `warnings`，无 warning 时仍为 `[]`。warning 结构见
  [错误与诊断参考](../errors.md)。

节点完成规范化和校验后会获得仅运行时存在的 `RuntimeID`；preview 只按
`RuntimeID` 关联前后节点，不使用名称、数组位置、连接字段或内容相等作为兜底。保留的
节点即使被改名、重排或修改连接字段也仍按同一节点计算，变化表现为 `modified`；
过滤、排除或去重表现为 `removed`。processor 新建或手工重建且未继承 `RuntimeID` 的
节点表现为 `added`，对应的原节点表现为 `removed`。`RuntimeID` 不属于 NodeIR JSON，
不会出现在 `before`、`after` 或持久化数据中。

远程订阅按保存的抓取设置读取，正数
`cache_ttl_seconds` 可以复用该订阅资源自己的 remote-fetch 缓存，不与其它订阅
共享。preview 不返回 traffic，
也不会把节点或 report 写回订阅。`refresh: true` 跳过本次 remote-fetch 与 probe
缓存读取，并在成功时按当前 TTL 重新填充；preview 本身不使用
subscription-render 结果缓存。

### `POST /v1/subscriptions/{name}/traffic`

读取远程订阅响应头中的运行时用量。请求正文可省略；需要绕过缓存时传入：

```json
{
  "refresh": true
}
```

`refresh: true` 同时绕过 subscription-traffic 缓存和本次底层 remote-fetch
缓存。成功时返回 `200`：

```json
{
  "subscription_name": "provider",
  "type": "remote",
  "format": "uri-list",
  "traffic": {
    "source_name": "provider",
    "source_url": "https://nodes.example.invalid/subscription",
    "upload_bytes": 1024,
    "download_bytes": 2048,
    "used_bytes": 3072,
    "total_bytes": 10240,
    "remaining_bytes": 7168,
    "plan_name": "Example Plan"
  }
}
```

命中 traffic 缓存时响应可带 `cached: true`。`traffic` 来自
`Subscription-Userinfo`、`Profile-Web-Page-Url`、`Plan-Name` 或
`Profile-Title` 等远程响应头；没有可识别信息时省略。它可能包含
`observed_at`、到期时间、剩余天数、重置日或应用 URL 等可选字段，但不会写回
订阅定义。URL fragment 中的 `noFlow`/`noflow` 会禁用本次用量识别。

traffic 只适用于 `remote`；它不执行订阅的 nodes processors，不返回节点统计
或完整节点。`local` 和 `collection` 请求会失败，collection 也不会聚合来源
订阅的用量。

### `POST /v1/subscriptions/{name}/render`

读取已保存的 Subscription，物化输入并按定义执行 nodes-stage processors，再
渲染成请求指定格式。请求体不会覆盖或写回 Subscription，也不会持久化生成正文
或 report。

请求体为：

| 字段 | 类型 | 契约 |
| --- | --- | --- |
| `format` | string | 必填且非空；目标 renderer 格式。 |
| `args` | object | 可选 string-to-string 参数，传给本次脚本/资源执行。 |
| `refresh` | boolean | 可选；为 `true` 时跳过结果、remote-fetch 和 probe 缓存读取，并在成功后重新填充。 |

查询参数 `arg.<key>=<value>` 也进入本次 args；body `args` 的同名键覆盖查询值。
processor 和脚本参数的完整执行语义分别见
[Processors](../processors.md)与[JavaScript 脚本 API](../scripting-api.md)。

成功返回 `200` 和完整 render 结果：

```json
{
  "content_type": "application/json",
  "body": "[\n  {\n    \"name\": \"example-node\",\n    \"type\": \"ss\",\n    \"server\": \"proxy.example.invalid\",\n    \"port\": 8388,\n    \"password\": \"example-password\",\n    \"cipher\": \"aes-128-gcm\"\n  }\n]",
  "cached": false,
  "report": {
    "kind": "subscription_render",
    "status": "ok",
    "created_at": "2026-01-01T00:00:00Z",
    "dependencies": [
      {
        "kind": "subscription",
        "name": "example"
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

`body` 是目标正文的 JSON string，`content_type` 描述正文格式；外层响应仍是
JSON。`cached` 表示是否直接命中 `subscription_render`；`report` 返回本次
完整 dependencies、source refs、warnings 与 render statistics，字段语义与
敏感边界见[错误与诊断](../errors.md#report)。

名称或 format 无效、订阅不存在、输入无法读取、processor 或 renderer 失败时
返回结构化 service error。render 可能抓取远程订阅、读缓存或执行 processor
的受控副作用；调用不会修改 Subscription、FileSpec 或 share。

```sh
curl -sS "$SANDRONE_URL/v1/subscriptions/example/render?arg.environment=test" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "format": "json-nodes",
    "args": {
      "prefix": "synthetic-"
    }
  }'
```

## 失败与安全边界

- `{name}` 必须非空，URL 解码后只能是一个 path segment；包含 `/`、`\`，
  或名称为 `.`、`..` 都会被拒绝。创建、读取、删除、preview、traffic 与
  render 使用同一约束。
- 远程输入只能经 Sandrone 的受控 fetcher 读取；不要在示例、日志或
  `meta` 中暴露真实订阅 URL、凭据或套餐标识。
- preview 会返回处理前后的节点对象，其中可能含连接凭据；应把响应按敏感数据
  处理。
- 保存或删除任一订阅会使 subscription-traffic、subscription-render 和
  file-render 缓存失效；文件和项目设置变更也会失效相关结果缓存。
- handler 级错误码、状态映射和 warning 字段见[错误与诊断参考](../errors.md)；
  router 与未知 action 的 plain-text `404`/`405` 边界见[通用约定](README.md#响应与失败)。

## 最小示例

保存一个脱敏的本地订阅：

```sh
curl -X POST http://127.0.0.1:1137/v1/subscriptions \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "name": "demo",
    "type": "local",
    "format": "uri-list",
    "content": "ss://YWVzLTEyOC1nY206UkVEQUNURUQ=@node.example.invalid:8388#demo",
    "processors": [
      {
        "type": "rename",
        "stage": "nodes",
        "params": {"mode": "prefix", "value": "test-"}
      }
    ]
  }'
```

预览处理前后差异：

```sh
curl -X POST \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  http://127.0.0.1:1137/v1/subscriptions/demo/preview
```
