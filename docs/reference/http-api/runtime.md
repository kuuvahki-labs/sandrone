# HTTP API：运行设置、规则集目录与备份

## 用途

本页定义服务级运行默认值、内置规则集目录和 Store 整体备份的 HTTP
契约。这里的接口都位于受保护的 `/v1/*` 管理边界；配置了 HTTP bearer
token 时，请求必须携带 `Authorization: Bearer <token>`。

运行设置是持久化的单例默认值。资源自身提供非空值时仍优先使用资源值。
规则集目录只返回构建时嵌入的元数据。备份下载和恢复处理的是 Store
原始数据，不是单个资源的导入导出。

## 接口

### `GET /v1/settings/runtime`

返回当前归一化后的运行设置。尚未保存设置时返回内建默认值。

### `PUT /v1/settings/runtime`

用 JSON 请求体替换运行设置单例，成功返回 `200`。这是整体替换而不是
JSON merge patch：未提供的字段会回到内建默认值，不会沿用上一次保存的值。

### `GET /v1/rule-set-catalog`

按必填 query 参数 `target` 返回一个目标的内置规则集目录。`target`
只接受 `mihomo`、`sing-box` 或 `shadowrocket`。

### `GET /v1/backup`

下载当前 Store 的完整非 cache 备份。

### `POST /v1/backup/restore`

校验 ZIP 后，用其中的数据替换整个 Store。

## 请求

### 运行设置

`PUT` 请求体为：

```json
{
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
    "subscription_traffic_ttl_seconds": 60
  }
}
```

字段约束：

| 字段 | 契约 |
| --- | --- |
| `remote_defaults.user_agent` | 远程抓取默认 User-Agent；空值使用当前 Sandrone 版本的默认值 |
| `remote_defaults.proxy` | 可为空；非空时必须是带 host 的 `http`、`https` 或 `socks5` URL |
| `remote_defaults.timeout_ms` | 正整数；省略或 `0` 使用内建默认值 |
| `probe_defaults.method` | `tcp_connect`、`udp_ntp` 或 `url_test`；内建默认值为 `url_test` |
| `probe_defaults.core` | `mihomo` 或 `sing-box`；内建默认值为 `sing-box`，`tcp_connect` 不使用 core |
| `probe_defaults.url` | 带 host 的 HTTP(S) URL |
| `probe_defaults.ntp_server` | NTP 服务地址；空值使用内建默认值 |
| `probe_defaults.timeout_ms`、`attempts`、`concurrency` | 正整数；省略或 `0` 使用各自内建默认值 |
| `probe_defaults.cache_ttl_seconds` | 非负秒数；`0` 表示不使用 probe cache |
| `cache_defaults.remote_fetch_ttl_seconds` | 非负秒数；`0` 表示不使用远程抓取 cache |
| `cache_defaults.subscription_traffic_ttl_seconds` | 非负秒数；`0` 表示不缓存订阅用量 |

`cache_defaults` 有意区分“对象缺失”和“对象存在且字段为 `0`”：缺失时恢复整组
内建默认值；显式提交对象则可把两个 TTL 都设为 `0`。

规则集目录请求示例：

```http
GET /v1/rule-set-catalog?target=mihomo
```

该接口没有搜索、格式筛选、分页、预览或刷新参数。

备份恢复的请求正文是原始 ZIP 字节，不是 JSON 或 multipart：

```http
POST /v1/backup/restore
Content-Type: application/zip

<raw ZIP bytes>
```

ZIP 根目录只允许如下结构：

```text
manifest.json
data/<safe-store-key>
```

`manifest.json` 必须恰好包含以下四个字段：

```json
{
  "format": "sandrone-store-backup",
  "storage_schema_version": 1,
  "created_at": "2026-07-21T19:34:56Z",
  "app_version": "0.1.0"
}
```

恢复只接受当前相同的 `storage_schema_version=1`，不自动迁移其它 schema。
`created_at` 必须是 RFC 3339 时间，`app_version` 必须非空。

## 响应

`GET /v1/settings/runtime` 直接返回设置对象，而不是额外 envelope。例如内建
默认值形如：

```json
{
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
    "subscription_traffic_ttl_seconds": 60
  }
}
```

值为零或空且带 `omitempty` 的字段可能不出现在 JSON 中；例如默认
`remote_defaults.proxy`、`remote_fetch_ttl_seconds` 和 probe
`cache_ttl_seconds` 会省略。`PUT` 成功返回：

```json
{
  "ok": true
}
```

规则集目录响应为：

```json
{
  "items": [
    {
      "name": "geosite-cn",
      "url": "https://rules.example/geosite-cn.mrs",
      "rule_kind": "domain"
    }
  ]
}
```

Mihomo 和 sing-box 条目的 `rule_kind` 为 `domain` 或 `ip`。
Shadowrocket 条目还带 `reference_type`：`DOMAIN-SET` 对应 `domain`，
`RULE-SET` 对应 `ip` 或 `mixed`。`url` 是条目的稳定身份。

备份下载成功返回 `application/zip`，并设置：

- `Content-Disposition: attachment`，文件名形如
  `sandrone-backup-20260721T193456Z.zip`；
- `Cache-Control: no-store`；
- `X-Content-Type-Options: nosniff`。

恢复成功返回 `200`、`Cache-Control: no-store` 和：

```json
{
  "ok": true
}
```

## 失败与安全边界

备份 ZIP 是**未加密、未签名的明文**。它可能包含订阅和节点凭据、远程
URL、脚本、inline file 正文、运行设置，以及 Store 实现未知的数据。
下载后必须按敏感配置文件保存，并在恢复前通过可信渠道确认来源。

导出包含全部非目录、非 cache Store 文件，包括 Sandrone 当前不认识的安全
key。恢复是全量替换：归档会在写入前完整校验，现有数据（包括 cache）随后被
清除并由归档内容替换；空备份因此会清空 Store。

恢复限制为：

- 最多 32 MiB 压缩请求数据；
- 最多 64 MiB 解码后总数据；
- 最多 10,000 个 `data/` 文件。

归档 member 必须是普通文件；未知根 member、重复项、不安全路径、路径树冲突、
cache key、CRC 错误或不符合精确字段契约的 manifest 都会在任何写入前被拒绝。

删除或写入失败时，服务会尝试恢复旧的非 cache 数据，cache 仍为空。恢复只在
单个 Sandrone 进程的协调边界内避免普通读写交错；它**不提供进程崩溃原子性，
也不提供多进程协调或原子性**。回滚也可能失败并留下部分恢复状态。具体快照、
锁与回滚顺序见[存储架构](../../architecture/storage.md#备份与恢复边界)。

规则集快照缺失或损坏时目录接口返回 `503`，其它功能不受影响。请求字段、
HTTP error envelope、备份错误码和 status 映射见[错误与诊断参考](../errors.md)。

## 最小示例

读取并替换运行设置：

```sh
curl -fsS \
  -H "Authorization: Bearer ${SANDRONE_TOKEN}" \
  http://127.0.0.1:1137/v1/settings/runtime

curl -fsS -X PUT \
  -H "Authorization: Bearer ${SANDRONE_TOKEN}" \
  -H "Content-Type: application/json" \
  --data '{"cache_defaults":{"remote_fetch_ttl_seconds":0,"subscription_traffic_ttl_seconds":0}}' \
  http://127.0.0.1:1137/v1/settings/runtime
```

下载与恢复备份：

```sh
curl -fsS \
  -H "Authorization: Bearer ${SANDRONE_TOKEN}" \
  -o sandrone-backup.zip \
  http://127.0.0.1:1137/v1/backup

curl -fsS -X POST \
  -H "Authorization: Bearer ${SANDRONE_TOKEN}" \
  -H "Content-Type: application/zip" \
  --data-binary @sandrone-backup.zip \
  http://127.0.0.1:1137/v1/backup/restore
```
