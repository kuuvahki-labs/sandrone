# HTTP API 参考

## 用途

本目录记录 Sandrone HTTP server 的公开契约。接口按资源拆分为以下专题：

- [转换、校验与能力检查](conversion.md)
- [节点探测](probing.md)
- [运行时 schema catalog](schemas.md)
- [订阅资源](subscriptions.md)
- [文件资源](files.md)
- [运行设置、规则集目录与备份](runtime.md)
- [分享资源与公开输出](shares.md)

格式名和 adapter 能力见[格式与能力参考](../capabilities.md)，统一错误码、
validation issue 和 warning 结构见[错误与诊断参考](../errors.md)。

## 通用请求约定

所有路径都相对于 HTTP server 根地址，例如默认本地地址
`http://127.0.0.1:1137`。方法是路由契约的一部分；错误方法可能由 exact route
或资源前缀 handler 分别返回 `405`、`404` 或 endpoint 级 `400`，调用方不能
依赖统一的 method-mismatch status。未注册的受保护路径在通过鉴权后返回
`404 Not Found`。不要给精确路径自行添加尾斜杠；例如 `/version/` 不是版本接口。

本目录的 shell 示例使用以下占位环境变量：

```sh
export SANDRONE_URL="http://127.0.0.1:1137"
export SANDRONE_TOKEN="change-me"
```

资源名和分享 ID 在 URL 中只占一个 path segment。服务会先做 URL path 解码，
再拒绝空值、`.`、`..`、正斜杠和反斜杠；把 `/` 编码成 `%2F` 不能绕过该边界。

JSON 请求体的读取上限为 16 MiB；是否允许空 body 由各接口定义。提供 body
时必须能被解码，无效 JSON 返回 `400`，超过上限返回 `413`。JSON 响应使用
`Content-Type: application/json`；直接文件或分享响应的媒体类型由对应专题
说明。

## 鉴权

配置 `--token` 或 `SANDRONE_TOKEN` 后，管理 API 与 MCP streamable HTTP
使用同一个静态 bearer token：

```http
Authorization: Bearer <token>
```

header 必须与配置值精确匹配。缺失或不匹配时返回 `401`，error code 当前为
`invalid_argument`。未配置 token 的本地监听不要求该 header；绑定非本地地址
时，启动配置校验要求提供 token。

以下路径不使用 bearer token：

- 健康检查与版本查询；
- 公开的一次性转换 `/convert`；
- Web UI 页面和静态资源；
- 持有链接即可读取的公开分享输出。

公开分享的持有者权限、有效期和使用次数边界见[分享接口](shares.md)。

## 响应与失败

成功的管理接口通常返回 JSON object；列表、直接文件和下载类响应以各专题为准。
操作成功但存在信息损失或可恢复问题时，响应可以带 `warnings`，这不等同于请求
失败。进入 Sandrone handler 后产生的鉴权、解码和 service error 使用：

```json
{
  "error": {
    "code": "invalid_argument",
    "message": "..."
  }
}
```

标准库 router 生成的未注册路径、method mismatch，以及部分未知 action 会直接
返回 `text/plain` 的 `404` 或 `405`；客户端应先检查 status 和
`Content-Type`，不能无条件解码 JSON。对于 JSON error，不要根据 `message`
文本或 HTTP status 反推业务错误；完整映射和 warning 字段见
[错误与诊断参考](../errors.md)。

## GET /healthz

### 用途

无需鉴权地确认 HTTP 服务可达；它不检查 store、远程输入或下游客户端状态。

### 请求

没有请求体和查询参数。

### 响应

成功返回 `200`：

```json
{
  "ok": true
}
```

### 最小示例

```sh
curl "$SANDRONE_URL/healthz"
```

## GET /version

### 用途

无需鉴权地读取当前 Sandrone 构建版本。

### 请求

没有请求体和查询参数，路径必须精确为 `/version`。

### 响应

成功返回 `200`：

```json
{
  "name": "sandrone",
  "version": "0.1.0",
  "revision": "0123456789abcdef0123456789abcdef01234567"
}
```

`version` 是当前构建提供的规范版本字符串，`revision` 是完整 Git object ID；
无法取得 VCS metadata 时 `revision` 为空字符串。上例只是示例值，调用方不应
写死。字段来源、短 SHA 展示和容器追溯见[构建身份](../build-info.md)。

### 最小示例

```sh
curl "$SANDRONE_URL/version"
```

## Web、MCP 与分享路径边界

同一 HTTP server 可以同时挂载五类入口：

- `/convert` 是无需 bearer token 的一次性节点转换；
- `/v1/*` 是受保护的管理 HTTP API；
- `/mcp` 及其子路径默认是受保护的 MCP streamable HTTP，路径可由
  `serve --path` 调整；
- `/s/:id` 是无需 bearer token 的公开只读分享输出；
- `/` 及其它不属于上述保留前缀的路径在启用 Web UI 时交给 SPA 和静态资源
  handler。

Web UI fallback 不会接管 `/convert`、`/v1`、MCP 配置路径或 `/s` 保留前缀。
MCP 与 Web 只是同 server 上的独立入口，不是 REST 专题的一部分。
