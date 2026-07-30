# HTTP API：项目设置、规则集目录与备份

## 用途

本页定义统一项目设置、内置规则集目录和 Store 整体备份的管理接口。除
`data_dir` 外，Sandrone 的启动设置、运行默认值、外观语言和订阅行为都属于
同一个项目设置对象，权威文件是可选的 `<data_dir>/settings.json`。

这些接口位于受保护的 `/v1/*` 边界。启用 bearer token 后，请求必须携带：

```http
Authorization: Bearer <token>
```

## 项目设置

### `GET /v1/settings`

返回存储值、当前进程有效值、启动覆盖来源和需要重启的字段：

```text
SettingsEnvelope {
  settings: SettingsView
  effective: SettingsView
  overrides: object<string, "environment" | "flag">
  restart_required: string[]
}
```

其中 `SettingsView` 的完整结构如下：

```json
{
  "schema_version": 1,
  "http": {
    "listen": "127.0.0.1:1137",
    "token_configured": true,
    "token_required": false
  },
  "mcp": {
    "transport": "stdio",
    "path": "/mcp",
    "allow_management_tools": false,
    "max_output_bytes": 1048576
  },
  "webui": {"static_dir": ""},
  "log": {"level": "info"},
  "remote_defaults": {
    "user_agent": "sandrone/0.1.0",
    "timeout_ms": 15000
  },
  "probe_defaults": {
    "method": "url_test",
    "core": "sing-box",
    "url": "http://www.gstatic.com/generate_204",
    "ntp_server": "time.apple.com",
    "timeout_ms": 5000,
    "attempts": 1,
    "concurrency": 10
  },
  "cache_defaults": {
    "remote_fetch_ttl_seconds": 0,
    "subscription_traffic_ttl_seconds": 60,
    "subscription_render_ttl_seconds": 0,
    "file_render_ttl_seconds": 0
  },
  "appearance": {"theme_mode": "dark", "locale": "auto"},
  "subscriptions": {"auto_load_traffic": false}
}
```

`settings` 和 `effective` 都使用上面的完整结构。`settings` 是持久化目标；
`effective` 是当前进程实际使用的值。HTTP token 永不返回原文，只通过
`token_configured` 表示是否已配置。

`overrides` 的 key 是设置路径，值为 `environment` 或 `flag`。被环境变量或
显式 flag 覆盖的启动字段即使写入文件，也不会进入 `restart_required`，因为
下次启动仍会被相同来源覆盖。

### `PUT /v1/settings`

请求体是完整设置对象，不是 merge patch。未知字段会被拒绝，正文上限为
16 MiB。成功返回更新后的同一 envelope。

`http.token` 使用特殊更新语义：

- 缺失或 `null`：保留当前存储 token；
- 非空字符串：替换 token；
- 空字符串：清除 token。

其它字段必须按完整对象提交。典型请求：

```json
{
  "schema_version": 1,
  "http": {
    "listen": "127.0.0.1:1137",
    "token": null,
    "token_required": false
  },
  "mcp": {
    "transport": "stdio",
    "path": "/mcp",
    "allow_management_tools": false,
    "max_output_bytes": 1048576
  },
  "webui": {"static_dir": ""},
  "log": {"level": "info"},
  "remote_defaults": {
    "user_agent": "Sandrone Client",
    "proxy": "socks5://127.0.0.1:1080",
    "timeout_ms": 8000
  },
  "probe_defaults": {
    "method": "url_test",
    "core": "sing-box",
    "url": "https://connectivity.example/generate_204",
    "ntp_server": "time.example",
    "timeout_ms": 5000,
    "attempts": 2,
    "concurrency": 10,
    "cache_ttl_seconds": 300
  },
  "cache_defaults": {
    "remote_fetch_ttl_seconds": 120,
    "subscription_traffic_ttl_seconds": 60,
    "subscription_render_ttl_seconds": 300,
    "file_render_ttl_seconds": 300
  },
  "appearance": {"theme_mode": "system", "locale": "auto"},
  "subscriptions": {"auto_load_traffic": false}
}
```

远程 proxy 只接受带 host 的 `http`、`https` 或 `socks5` URL。probe method
只接受 `tcp_connect`、`udp_ntp`、`url_test`，core 只接受 `mihomo` 或
`sing-box`。TTL 必须非负；远程与 probe 的 timeout、attempts 和 concurrency
归一化后必须为正数。主题接受 `system`、`light`、`dark`，语言接受 `auto`、
`zh-CN`、`en-US`。

远程、probe、cache、appearance 和 subscriptions 组保存后立即生效。HTTP、
MCP、Web UI 静态目录和日志级别属于启动组，保存后列入 `restart_required`，
当前 listener、鉴权边界和其它启动组件不会热切换。

设置文件不存在时使用内建默认值；第一次成功保存才创建文件。文件使用原子替换
并保持 `0600` 权限。`data_dir` 不属于该文件或 API。

最小读取示例：

```sh
curl -fsS \
  -H "Authorization: Bearer ${SANDRONE_TOKEN}" \
  "${SANDRONE_URL}/v1/settings"
```

## 规则集目录

### `GET /v1/rule-set-catalog?target=<target>`

`target` 必须是 `mihomo`、`sing-box` 或 `shadowrocket`。响应为构建时嵌入的
目录，不提供搜索、分页、预览或刷新。目录快照缺失或损坏时返回 `503`。

## 备份与恢复

### `GET /v1/backup`

下载全部非 cache Store 数据的 ZIP。响应使用 `application/zip`、
`Cache-Control: no-store` 和安全的 `Content-Disposition` 文件名。
`settings.json` 会以包含真实 token 的原始 bytes 进入归档，因此备份是敏感明文。

### `POST /v1/backup/restore`

正文是原始 ZIP，不是 JSON 或 multipart。压缩正文上限 32 MiB，解码后总大小
上限 64 MiB，最多 10,000 个数据文件。结构为：

```text
manifest.json
data/<safe-store-key>
```

manifest 必须恰好包含 `format`、`storage_schema_version`、`created_at` 和
`app_version`。恢复只接受当前 `storage_schema_version=1`。

归档会在任何 Store 修改前完整校验。若包含 `settings.json`，还会严格校验设置
schema 和字段；恢复写入时该文件使用原子 `0600` 写入。成功替换后，动态设置
立即刷新，启动设置变化进入 `restart_required`。不含 `settings.json` 的备份
仍然有效，恢复后使用内建默认值。

恢复是单进程协调边界内的全量替换和 best-effort rollback，不提供进程崩溃或
多进程原子性。详细锁与回滚顺序见[存储架构](../../architecture/storage.md#备份与恢复边界)。

下载和恢复示例：

```sh
curl -fsS \
  -H "Authorization: Bearer ${SANDRONE_TOKEN}" \
  -o sandrone-backup.zip \
  "${SANDRONE_URL}/v1/backup"

curl -fsS -X POST \
  -H "Authorization: Bearer ${SANDRONE_TOKEN}" \
  -H "Content-Type: application/zip" \
  --data-binary @sandrone-backup.zip \
  "${SANDRONE_URL}/v1/backup/restore"
```

统一错误 envelope、备份错误码和 HTTP status 映射见[错误与诊断参考](../errors.md)。
