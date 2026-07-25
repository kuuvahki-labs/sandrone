# 文件 HTTP API

## 用途

文件资源保存一个 `FileSpec`，生成时再读取 source、解析引用的订阅、编译 typed
客户端配置，并按声明顺序执行 file-stage processors。保存与生成是两个独立
动作：创建或更新文件不会立即读取远程 source，也不会产生最终正文。

`FileSpec` 的字段、canonical `kind`、source、typed settings 与默认值以
[FileSpec 参考](../file-spec.md)为准；处理器参数见
[Processors 参考](../processors.md)。本页只定义 HTTP 资源操作和读取模式。

本页中的接口都属于受保护的 `/v1/*` 管理 API。服务启用 token 鉴权时，请求须
携带 `Authorization: Bearer <token>`。

## 接口

### `GET /v1/files`

列出文件摘要。请求没有正文。

成功时返回 `200` 和 `{ "items": [...] }`。每项以 `kind: "file"` 标识资源，
并可包含 `name`、`display_name`、`size`、`processors`、`created_at`、
`updated_at`、`meta` 或存储解码 `warning`。其中：

- `target` 是文件真实的 canonical `FileSpec.kind`；
- `type` 只表示 `source.type`。

调用方必须用 `target` 判断 `static`、`mihomo`、`sing-box` 或
`shadowrocket`，不能从 `type` 推断文件 kind。

### `POST /v1/files`

创建或覆盖同名文件。请求正文是完整 `FileSpec`；`name` 必填且必须是单个
path segment，`kind` 也必须显式使用 canonical 值，包括 `static`。字段详情与
严格校验规则见 [FileSpec 参考](../file-spec.md)。

成功时返回 `201`：

```json
{
  "ok": true
}
```

保存时完整 `FileSpec` 写入一个 JSON record。`inline` 正文继续位于
`source.content`，之后 `mode=spec` 会返回同一完整定义；`remote` 只记录抓取
描述。保存 typed 文件只校验结构，不会在此请求中编译最终客户端配置。

### `GET /v1/files/{name}`

按所选阶段读取文件。请求没有正文，可使用下列查询参数：

| 参数 | 契约 |
| --- | --- |
| `mode=spec` | 返回保存后的 `FileSpec` JSON；不读取 source、不解析订阅、不编译 typed 配置，也不执行 processors。 |
| `mode=source` | 返回编译前的 source 正文。typed 文件未声明 source 时返回 driver 的内建 base；不解析订阅、不编译 typed 配置，也不执行 file-stage processors。 |
| `mode=render` | 运行完整文件管线并返回最终正文。省略 `mode` 时与此相同。 |
| `response=json` | 把 source 或 render 正文放入 JSON envelope；对 `mode=spec` 没有额外作用。值按大小写不敏感匹配。 |
| `arg.<key>=<value>` | 仅为本次 render 提供字符串请求参数；不写入 `FileSpec`。空 key 忽略，同一 key 重复出现时取最后一个值。 |

`mode` 会去除首尾空白并按大小写不敏感匹配；其他值会失败。

render 时，`arg.*` 会随请求上下文传给文件引用的订阅处理链和 file-stage
处理链，并作为脚本 `input.args`。请求值覆盖脚本 `params.args` 中的同名键。
参数只影响本次结果，不影响 `mode=spec`、`mode=source` 或之后的请求；完整
合并规则见 [JavaScript 脚本 API](../scripting-api.md)。

### `DELETE /v1/files/{name}`

物理删除文件定义，成功时返回 `200`：

```json
{
  "ok": true
}
```

删除只移除保存完整 `FileSpec` 的 JSON record。它不会级联删除订阅、其他文件、
share 或 Store 中与该 record 无关的 raw key。

## 响应

`mode=spec` 始终返回保存后的 `FileSpec` JSON。

`mode=source` 默认直接返回 source 字节，使用 source 的媒体类型；媒体类型为空
时使用 `application/octet-stream`，并设置
`X-Content-Type-Options: nosniff`。加上 `response=json` 后返回：

```json
{
  "content_type": "application/yaml",
  "body": "mixed-port: 7891\n"
}
```

`mode=render` 默认直接返回最终字节和最终 `Content-Type`；没有具体媒体类型时
使用 `application/octet-stream`。加上 `response=json` 后返回固定 `200` 的
JSON envelope：

```json
{
  "content_type": "text/plain; charset=utf-8",
  "body": "hello demo",
  "response": {},
  "warnings": []
}
```

`response` 是保留的响应元数据，当前内建文件流程通常返回 `{}`；`warnings`
汇总本次编译、渲染及处理链诊断，无 warning 时仍为 `[]`。直接正文模式不另行
附带 warning envelope。warning 结构见[错误与诊断参考](../errors.md)。

三个 mode 的边界是固定的：

```text
spec   = 保存后的定义
source = 编译前的基础正文
render = 最终生成正文
```

typed render 的完整阶段顺序见[文件管线](../../architecture/file-pipeline.md)。

## 失败与安全边界

- `{name}` 必须非空，URL 解码后只能是一个 path segment；包含 `/`、`\`，
  或名称为 `.`、`..` 都会被拒绝。创建、读取和删除使用同一约束。
- `source.type: "remote"` 只经受控 HTTP(S) fetcher 读取。详细抓取和缓存边界见
  [FileSpec 参考](../file-spec.md)。
- source 可能包含凭据、脚本或未处理配置；`mode=source` 与 `mode=spec`
  响应应按敏感数据处理。
- render 是实时生成操作。引用缺失、远程抓取失败、typed 配置无效或 processor
  失败都会使本次请求失败，不会保存部分生成结果。
- 文件生成结果只随本次读取返回，不创建独立的持久化下载资源。
- handler 级错误码、状态映射和 warning 字段见[错误与诊断参考](../errors.md)；
  router 的 plain-text `404`/`405` 边界见[通用约定](README.md#响应与失败)。

## 最小示例

保存一个接受临时 `name` 参数的静态文件：

```sh
curl -X POST http://127.0.0.1:1137/v1/files \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "name": "greeting.txt",
    "kind": "static",
    "source": {"type": "inline", "content": "hello"},
    "processors": [
      {
        "type": "script",
        "stage": "file",
        "params": {
          "source": {
            "type": "inline",
            "content": "function main(input) { input.file.content += \" \" + input.args.name; return input; }"
          }
        }
      }
    ]
  }'
```

生成 JSON 包装的最终正文：

```sh
curl \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  "http://127.0.0.1:1137/v1/files/greeting.txt?response=json&arg.name=demo"
```
