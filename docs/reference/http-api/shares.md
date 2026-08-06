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
生成；显式 `id` 已存在时，使用新请求完整覆盖旧 share。

### `GET /v1/shares/:id`

返回一个 share 的管理信息，成功返回 `200`。

### `DELETE /v1/shares/:id`

物理删除 share，成功返回 `200`。删除 share 不删除其目标资源。

### `GET /s/:id`、`GET /s/:id/:filename`

公开读取 share 内容。该接口只允许 `GET`，不要求管理员 bearer token。
`/s/:id` 是兼容形式；Sandrone Web UI 复制的 canonical 形式为
`/s/:id/:filename`，让按 URL 末段命名的客户端也能显示可读文件名。
`:filename` 只负责展示，资源查找始终只使用 `:id`。

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
  "meta": {
    "description": "mobile clients"
  }
}
```

字段契约：

| 字段 | 契约 |
| --- | --- |
| `id` | 可省略；自定义值必须是单个 path segment，已存在时覆盖旧 share |
| `name` | 可选分享名，也是公开文件名的首选来源；必须满足单个 path segment 约束 |
| `target_kind` | 必填，只能是 `file` 或 `subscription` |
| `target_name` | 必填，目标必须已存在；HTTP 接口只接受单个 path segment |
| `target_format` | 仅对订阅生效；省略时默认为 `uri-list`，但不锁定公开请求的格式 |
| `content_type` | 可选；非空时覆盖未加密公开响应的推导内容类型 |
| `valid_from` | 可选 RFC 3339 时间，闭区间起点 |
| `valid_until` | 可选 RFC 3339 时间，开区间终点 |
| `age_recipient` | 可选，只接受一个 age X25519 recipient 公钥 |
| `meta` | 可选字符串 map，随管理响应返回 |

`valid_from` 和 `valid_until` 同时存在时，前者必须早于后者。`id`、`name` 和
`target_name` 的单 segment 约束会拒绝 `/`、
`\`、`.` 和 `..`；调用方不应把层级路径编码进这些字段。

公开请求可带：

```text
GET /s/mobile?format=sing-box-outbounds
GET /s/mobile/mobile.json?format=sing-box-outbounds
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
    "public_filename": "mobile.yaml",
    "format_filenames": {
      "uri-list": "mobile.txt",
      "mihomo-proxies": "mobile.yaml",
      "sing-box-outbounds": "mobile.json",
      "shadowrocket-proxies": "mobile.conf",
      "json-nodes": "mobile.json"
    },
    "created_at": "2026-07-22T12:00:00Z",
    "updated_at": "2026-07-22T12:00:00Z",
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
      "target_format": "mihomo-proxies",
      "public_filename": "mobile.yaml",
      "format_filenames": {
        "uri-list": "mobile.txt",
        "mihomo-proxies": "mobile.yaml",
        "sing-box-outbounds": "mobile.json",
        "shadowrocket-proxies": "mobile.conf",
        "json-nodes": "mobile.json"
      }
    }
  ]
}
```

`public_filename` 是保存的默认格式对应的 canonical 文件名，创建和列表响应都会
返回。
订阅 share 还返回服务端为每个已注册输出格式推导的 `format_filenames`；
文件 share 不返回该 map。它们都是管理响应的派生字段，不写入 share 存储。

管理对象还可能包含 `content_type`、`valid_from`、`valid_until`、
`age_recipient` 等字段；零值时间与多数零值字段会从 JSON 省略。

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
生效的 `format` 使用上表扩展名。文件 share 依次从 share `name`、
`target_name`、share `id` 回退，并保留选中名称的扩展名；列表请求不会为了
推导名称而执行文件处理链。控制字符和路径字符等不安全字符会被替换，边界
空格与点会被清理，过长基名截断为 128 个 Unicode 码点；清理后为空则继续
回退。文件处理器提供的任何 `Content-Disposition` 都会被 Sandrone 最终替换。

canonical 路径把该文件名 percent-encode 为单个 path segment。订阅 share
切换 `format` 时，路径末段和 query 必须同时切换，例如：

```text
/s/mobile/mobile.yaml?format=mihomo-proxies
/s/mobile/mobile.conf?format=shadowrocket-proxies
```

服务端会精确校验解码后的 `:filename` 与实际格式对应的 canonical 文件名。
名称不匹配、空名称、额外 path segment 或编码后的路径分隔符返回
`invalid_argument`。省略 `:filename` 的旧链接继续有效且不重定向。

设置 `age_recipient` 后，正文使用该公钥加密，响应改为
`Content-Type: application/age`；原内容类型通过
`X-Sandrone-Original-Content-Type` 返回。建议文件名在原扩展名后追加且只保留
一个 `.age`。服务端只保存 recipient 公钥，不保存或生成解密私钥。

## 失败与安全边界

公开 URL 本身就是读取凭据。任何持有仍有效 URL 的人都可以发起读取、为订阅
选择任一受支持的 `format`、提交 `arg.*`；
`target_format` 只是默认值，不是格式 allowlist。Share 不授予列表、删除或其它
管理权限。

未启用 age 时，链接持有者直接取得明文。启用 age 时，链接持有者仍能取得
密文，并看到 HTTP 状态、建议文件名和原内容类型；只有对应私钥
持有者能解密正文。age 不加密这些 HTTP 元数据，因此不要把敏感标识放进
share 名称、目标名称或最终文件名。

以下情况对公开接口统一表现为 `404`，不向链接持有者区分原因：

- share 不存在或已物理删除；
- 当前时间早于 `valid_from`；
- 当前时间大于或等于 `valid_until`；
- 当前目标资源已经不存在。

使用未知订阅格式会失败，且不会回退到默认格式。错误 envelope、HTTP status
映射及稳定错误码见[错误与诊断参考](../errors.md)。

## 最小示例

创建一个订阅 share：

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
  'https://public.example/s/mobile/mobile.yaml?format=mihomo-proxies'
```
