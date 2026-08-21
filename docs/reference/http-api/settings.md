# HTTP API：项目设置、缓存、规则集目录与备份

## 用途

本页定义统一项目设置、持久缓存、内置规则集目录和 Store 整体备份的管理接口。除
`data_dir` 和 bearer token 外，Sandrone 的启动设置、运行默认值、外观语言和
订阅行为都属于同一个项目设置对象，权威文件是可选的
`<data_dir>/settings.json`。bearer token 只从启动 flag 或环境变量读取，不进入
设置 API、设置文件或备份。

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
    "listen": "127.0.0.1:1137"
  },
  "mcp": {
    "path": "/mcp",
    "max_output_bytes": 1048576
  },
  "log": {"level": "info"},
  "remote_defaults": {
    "timeout_ms": 15000
  },
  "probe_defaults": {
    "method": "url_test",
    "core": "sing-box",
    "url": "https://cp.cloudflare.com",
    "ntp_server": "time.apple.com",
    "timeout_ms": 5000,
    "attempts": 1,
    "concurrency": 10
  },
  "cache_defaults": {
    "remote_fetch_ttl_seconds": 0,
    "probe_ttl_seconds": 0,
    "subscription_traffic_ttl_seconds": 60,
    "subscription_render_ttl_seconds": 0,
    "file_render_ttl_seconds": 0
  },
  "appearance": {"theme_mode": "dark", "locale": "auto"},
  "subscriptions": {"auto_load_traffic": false},
  "scheduled_refresh": {
    "enabled": false,
    "schedule": "@every 10m",
    "targets": []
  }
}
```

`settings` 和 `effective` 都使用上面的完整结构。`settings` 是持久化目标；
`effective` 是当前进程实际使用的值。启动 token 不属于这两个对象。
`remote_defaults.user_agent` 可选；省略或保存为空时不固定版本，执行远程请求时
动态使用当前二进制的 `sandrone/<version>`。非空值是显式覆盖，并会跨版本保留。
`mcp` 不包含启停开关：统一服务始终挂载 MCP 并注册全部 tools；这两个字段只
控制 MCP 路径和最终内联正文上限。
`mcp.path` 必须以 `/` 开头，且不能是 `/`、`/healthz`、`/version`、`/convert`、
`/s` 或 `/s/*`，避免覆盖无需 bearer token 的公开 route。

`overrides` 的 key 是设置路径，值为 `environment` 或 `flag`。被环境变量或
显式 flag 覆盖的启动字段即使写入文件，也不会进入 `restart_required`，因为
下次启动仍会被相同来源覆盖。

### `PUT /v1/settings`

请求体是完整设置对象，不是 merge patch。未知字段会被忽略，正文上限为
16 MiB；已知字段仍执行类型、范围和枚举校验。成功返回更新后的同一 envelope。

所有字段必须按完整对象提交。典型请求：

```json
{
  "schema_version": 1,
  "http": {
    "listen": "127.0.0.1:1137"
  },
  "mcp": {
    "path": "/mcp",
    "max_output_bytes": 1048576
  },
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
    "concurrency": 10
  },
  "cache_defaults": {
    "remote_fetch_ttl_seconds": 120,
    "probe_ttl_seconds": 300,
    "subscription_traffic_ttl_seconds": 60,
    "subscription_render_ttl_seconds": 300,
    "file_render_ttl_seconds": 300
  },
  "appearance": {"theme_mode": "system", "locale": "auto"},
  "subscriptions": {"auto_load_traffic": false},
  "scheduled_refresh": {
    "enabled": true,
    "schedule": "@every 10m",
    "targets": [
      {"kind": "subscription", "name": "provider"},
      {"kind": "file", "name": "client.yaml"}
    ]
  }
}
```

远程 proxy 只接受带 host 的 `http`、`https` 或 `socks5` URL。probe method
只接受 `tcp_connect`、`udp_ntp`、`url_test`，core 只接受 `mihomo` 或
`sing-box`。TTL 必须非负；远程与 probe 的 timeout、attempts 和 concurrency
归一化后必须为正数。主题接受 `system`、`light`、`dark`，语言接受 `auto`、
`zh-CN`、`en-US`。

远程、probe、cache、appearance、subscriptions 和 scheduled-refresh 组保存后立即生效。
HTTP listen、MCP 三个字段和日志级别属于启动组，保存后列入 `restart_required`；
当前 listener、MCP 路径与 tool catalog、鉴权边界和其它启动组件不会热切换。

设置文件不存在时使用内建默认值；第一次成功保存才创建文件。文件使用原子替换
并保持 `0600` 权限。`data_dir` 不属于该文件或 API。

最小读取示例：

```sh
curl -fsS \
  -H "Authorization: Bearer ${SANDRONE_TOKEN}" \
  "${SANDRONE_URL}/v1/settings"
```

## 定时更新

`scheduled_refresh.schedule` 接受 robfig/cron v3 的标准五字段表达式和 descriptor，
例如 `0 * * * *`、`@hourly`、`@daily`、`@every 10m`。不接受秒字段；
`@every` 间隔不得小于一分钟。计划始终使用服务器本地时区，因此拒绝
`CRON_TZ=` 和 `TZ=` 前缀。

target 的 `kind` 只能是 `subscription` 或 `file`，`name` 去除首尾空白后必须
非空，同一 `kind`/`name` 不能重复。启用时至少需要一个 target。保存设置时不
检查目标当前是否存在，因此已删除资源仍能保留在设置中；实际触发时该目标失败，
后续目标继续执行。调度与缓存的完整执行边界见[存储架构](../../architecture/storage.md#定时更新)。

### `GET /v1/settings/scheduled-refresh-status`

返回当前进程的内存状态，不重新读取或修改项目设置：

```json
{
  "enabled": true,
  "running": false,
  "next_run_at": "2026-08-11T16:00:00+08:00",
  "last_started_at": "2026-08-11T15:50:00+08:00",
  "last_completed_at": "2026-08-11T15:50:08+08:00",
  "last_success_count": 2,
  "last_failure_count": 0,
  "skipped_count": 0
}
```

`next_run_at`、`last_started_at`、`last_completed_at` 和 `last_skipped_at` 在尚无
对应事件时省略。success/failure 是最近一次已完成运行的目标计数；skipped 是
本进程启动后的累计重叠跳过次数。该接口与其它 `/v1/*` 一样受 bearer token
保护。

## 缓存管理

### `DELETE /v1/cache`

依次清空 `remote_fetch`、`probe`、`subscription_traffic`、
`subscription_render` 和 `file_render` 五个持久缓存层，成功返回
`204 No Content` 和 `Cache-Control: no-store`。空缓存同样成功。

若某层 Store 删除失败，接口返回 `500 cache_operation_failed`；此前层的删除不会
回滚。并发请求可以在清理期间或之后重新写入缓存，因此成功响应不承诺缓存会持续
为空。

该接口不处理请求内 memo，不提供统计、分层清理、条目浏览或单项删除，也不修改
TTL、`refresh=true` 语义或缓存 envelope。服务不会在启动或后台定时调用它。

## 规则集目录

### `GET /v1/rule-set-catalog?target=<target>`

`target` 必须是 `mihomo`、`sing-box` 或 `shadowrocket`。响应为构建时嵌入的
目录，不提供搜索、分页、预览或刷新。目录快照缺失或损坏时返回 `503`。

## 备份与恢复

### `GET /v1/backup`

下载全部非 cache Store 数据的 ZIP。响应使用 `application/zip`、
`Cache-Control: no-store` 和安全的 `Content-Disposition` 文件名。
`settings.json` 会以原始 bytes 进入归档。启动 token 不在其中，但订阅 URL、
节点凭据、脚本和其它 Store 数据仍可能包含敏感内容，因此备份仍是敏感明文。

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
