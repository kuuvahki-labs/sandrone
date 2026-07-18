# HTTP API：分享链接

## 用途

Share 把一个现有文件或订阅暴露为公开、只读、按请求实时生成的响应。它不是
资源快照，也不是管理员 token：管理接口属于受保护的 `/v1/*` 边界；公开读取
接口始终免 bearer token。

文件 share 每次按当前 [FileSpec](../file-spec.md) 生成内容，订阅 share 每次从
当前订阅生成指定格式。目标资源之后的修改会立即反映到公开响应中。

## 接口

### `GET /v1/shares`

列出所有 share，成功返回 `200`。

### `POST /v1/shares`

为一个已存在的文件或订阅创建 share，成功返回 `201`。省略 `id` 时由服务端
生成；显式 `id` 必须唯一。

### `GET /v1/shares/:id`

返回一个 share 的管理信息，成功返回 `200`。

### `DELETE /v1/shares/:id`

物理删除 share，成功返回 `200`。删除 share 不删除其目标资源。

### `GET /s/:id`

公开读取 share 内容。该接口只允许 `GET`，不要求管理员 bearer token。

## 请求

创建请求示例：

```json
{
  "id": "mobile",
  "name": "mobile",
  "target_kind": "subscription",
  "target_name": "default",
  "target_format": "mihomo-proxies",
  "valid_from": "2026-07-22T00:00:00Z",
  "valid_until": "2026-08-01T00:00:00Z",
  "max_uses": 5,
  "meta": {
    "description": "mobile clients"
  }
}
```

字段契约：

| 字段 | 契约 |
| --- | --- |
| `id` | 可省略；自定义值必须唯一，并且是单个 path segment |
| `name` | 可选显示名，也必须满足单个 path segment 约束 |
| `target_kind` | 必填，只能是 `file` 或 `subscription` |
| `target_name` | 必填，目标必须已存在；HTTP 接口只接受单个 path segment |
| `target_format` | 仅对订阅生效；省略时默认为 `uri-list`，但不锁定公开请求的格式 |
| `content_type` | 可选；非空时覆盖未加密公开响应的推导内容类型 |
| `valid_from` | 可选 RFC 3339 时间，闭区间起点 |
| `valid_until` | 可选 RFC 3339 时间，开区间终点 |
| `age_recipient` | 可选，只接受一个 age X25519 recipient 公钥 |
| `max_uses` | 可省略或为 `0`，表示不限次数；正整数表示成功公开读取的上限 |
| `meta` | 可选字符串 map，随管理响应返回 |

`valid_from` 和 `valid_until` 同时存在时，前者必须早于后者。`max_uses`
不能为负数。`id`、`name` 和 `target_name` 的单 segment 约束会拒绝 `/`、
`\`、`.` 和 `..`；调用方不应把层级路径编码进这些字段。

公开请求可带：

```text
GET /s/mobile?format=sing-box-outbounds
GET /s/mobile?arg.profile=travel
```

对订阅 share，`format` 覆盖保存的 `target_format`，可用值为：

| `format` | 响应内容类型 | 建议文件扩展名 |
| --- | --- | --- |
| `uri-list` | `text/plain` | `.txt` |
| `mihomo-proxies` | `application/yaml` | `.yaml` |
| `sing-box-outbounds` | `application/json` | `.json` |
| `shadowrocket-proxies` | `text/plain; charset=utf-8` | `.conf` |
| `json-nodes` | `application/json` | `.json` |

文件 share 忽略 `format`，始终执行目标文件当前定义的处理链。所有
`arg.<name>=<value>` query 都只进入本次请求的 `args`：文件 share 可在
file-stage processor 中消费它们，订阅 share 可在订阅的 node-stage processor
中消费它们；参数不会写回资源。processor 与脚本字段分别见
[Processor 参考](../processors.md)和[脚本 API](../scripting-api.md)，格式能力与
有损边界见[能力参考](../capabilities.md)。

## 响应

创建成功返回：

```json
{
  "share": {
    "id": "mobile",
    "name": "mobile",
    "target_kind": "subscription",
    "target_name": "default",
    "target_format": "mihomo-proxies",
    "created_at": "2026-07-22T12:00:00Z",
    "updated_at": "2026-07-22T12:00:00Z",
    "max_uses": 5,
    "meta": {
      "description": "mobile clients"
    }
  }
}
```

单项读取使用相同的 `{ "share": ... }` envelope。列表响应为：

```json
{
  "shares": [
    {
      "id": "mobile",
      "target_kind": "subscription",
      "target_name": "default",
      "target_format": "mihomo-proxies"
    }
  ]
}
```

管理对象还可能包含 `content_type`、`valid_from`、`valid_until`、
`last_accessed_at`、`age_recipient`、`use_count` 等字段；零值时间与多数零值
字段会从 JSON 省略。每次成功消费公开内容后，`use_count` 增加，
`last_accessed_at` 和 `updated_at` 更新。

删除成功返回：

```json
{
  "ok": true
}
```

公开响应直接返回生成后的正文，不使用 JSON envelope。Sandrone 设置
`Content-Disposition: inline` 和安全的建议文件名；这是 inline 呈现提示，
不会强制 attachment 下载，也不改变正文。

订阅文件名依次从 share `name`、`target_name`、share `id` 回退，并按实际
生效的 `format` 使用上表扩展名。文件 share 依次从处理后的最终文档名称、
`target_name`、share `id` 回退，并保留最终扩展名。控制字符和路径字符等
不安全字符会被替换，边界空格与点会被清理，过长基名截断为 128 个 Unicode
码点；清理后为空则继续回退。文件处理器提供的任何 `Content-Disposition`
都会被 Sandrone 最终替换。

设置 `age_recipient` 后，正文使用该公钥加密，响应改为
`Content-Type: application/age`；原内容类型通过
`X-Sandrone-Original-Content-Type` 返回。建议文件名在原扩展名后追加且只保留
一个 `.age`。服务端只保存 recipient 公钥，不保存或生成解密私钥。

## 失败与安全边界

公开 URL 本身就是读取凭据。任何持有仍有效 URL 的人都可以发起读取、为订阅
选择任一受支持的 `format`、提交 `arg.*`，并消耗 `max_uses` 配额；
`target_format` 只是默认值，不是格式 allowlist。Share 不授予列表、删除或其它
管理权限。

未启用 age 时，链接持有者直接取得明文。启用 age 时，链接持有者仍能取得
密文、消耗配额，并看到 HTTP 状态、建议文件名和原内容类型；只有对应私钥
持有者能解密正文。age 不加密这些 HTTP 元数据，因此不要把敏感标识放进
share 名称、目标名称或最终文件名。

`max_uses` 通过 Store compare-and-swap 原子消费，并发成功次数不会超过上限。
加密 share 使用相同计数规则。格式无效或生成失败不会消费次数；服务器在写出
响应前完成消费，因此客户端中途断开仍可能已经计数。链接持有者可以主动耗尽
有限配额，`max_uses` 不是身份认证机制。

以下情况对公开接口统一表现为 `404`，不向链接持有者区分原因：

- share 不存在或已物理删除；
- 当前时间早于 `valid_from`；
- 当前时间大于或等于 `valid_until`；
- `use_count` 已达到 `max_uses`；
- 当前目标资源已经不存在。

使用未知订阅格式会失败，且不会回退到默认格式。错误 envelope、HTTP status
映射及稳定错误码见[错误与诊断参考](../errors.md)。

## 最小示例

创建一个不限次数的订阅 share：

```sh
curl -fsS -X POST \
  -H "Authorization: Bearer ${SANDRONE_TOKEN}" \
  -H "Content-Type: application/json" \
  --data '{"id":"mobile","target_kind":"subscription","target_name":"default","target_format":"uri-list"}' \
  http://127.0.0.1:1137/v1/shares
```

持有公开链接的客户端可直接选择格式：

```sh
curl -fsS \
  -o mobile.yaml \
  'https://public.example/s/mobile?format=mihomo-proxies'
```
