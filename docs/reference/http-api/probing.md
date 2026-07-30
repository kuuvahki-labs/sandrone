# 节点探测 HTTP API

## 用途

`POST /v1/probe` 从一个 `NodeInput` 解析节点，并在 Sandrone 的受控 probe
backend 中执行可达性检查。它不保存输入、节点或 report，但可能读取已保存资源、
抓取远程输入、建立 TCP/UDP 连接或启动受控代理核心。

该接口属于受保护的 `/v1/*` 管理 API，继承
[鉴权](README.md#鉴权)、JSON body 上限和
[通用错误响应](README.md#响应与失败)。探测架构、backend 选择与缓存边界见
[节点探测](../../architecture/probing.md)；错误码、单节点失败与 report 语义见
[错误与诊断](../errors.md)。

## POST /v1/probe

### 请求

请求体是：

| 字段 | 类型 | 契约 |
| --- | --- | --- |
| `input` | `NodeInput` | 必填；完整结构可从 [`GET /v1/schemas/subscription`](schemas.md#nodeinput-结构) 的 `inputs.items` 读取。 |
| `method` | string | 可选 canonical method：`tcp_connect`、`udp_ntp`、`url_test`。 |
| `core` | string | 可选：`mihomo` 或 `sing-box`；用于需要代理核心的 probe。 |
| `url` | string | `url_test` 目标；省略时使用项目设置。 |
| `ntp_server` | string | `udp_ntp` 服务器；省略时使用项目设置。 |
| `expected_status` | string | URL 测试的期望状态表达式；backend 不支持时可产生 warning。 |
| `timeout_ms` | integer | 非负；正数限制每次尝试，`0`/省略使用项目设置。 |
| `attempts` | integer | 非负；正数限制每个节点的尝试次数，`0`/省略使用项目设置。 |
| `concurrency` | integer | 非负；正数限制并行节点数，`0`/省略使用项目设置。 |
| `cache_ttl_seconds` | integer | 非负；正数允许复用成功 probe 结果，`0`/省略使用项目设置。 |
| `meta` | object | 可选 string-to-string metadata，进入本次受控执行与 report 上下文。 |

默认项目设置是 `method: "url_test"`、`core: "sing-box"`、
`timeout_ms: 5000`、`attempts: 1`、`concurrency: 10`；管理员可通过项目设置更改
有效值，客户端不应把默认值当作 server capability。`tcp_connect` 不使用核心，
`udp_ntp` 当前使用 sing-box，`url_test` 支持 sing-box 和 Mihomo。可用 backend/core 应通过
[`GET /v1/inspect`](conversion.md#get-v1inspect) 发现。

`NodeInput` 支持 inline nodes、inline content、受控 local/remote 输入与已保存
subscription 引用。remote、local 和引用的解析/依赖语义与 FileSpec 共用受控
I/O 边界；本页不复制其完整语义。

### 响应

整批执行完成时返回 `200`：

```json
{
  "results": [
    {
      "node_name": "example-node",
      "method": "tcp_connect",
      "target": "proxy.example.invalid:8388",
      "backend": "tcp",
      "alive": false,
      "checked_at": "2026-01-01T00:00:00Z",
      "error_code": "probe_tcp_failed",
      "error": "synthetic connection failure"
    }
  ],
  "report": {
    "kind": "probe",
    "status": "ok",
    "created_at": "2026-01-01T00:00:00Z",
    "warnings": [
      {
        "code": "probe_tcp_failed",
        "message": "synthetic connection failure",
        "node": "example-node"
      }
    ],
    "probe": {
      "backend": "tcp",
      "method": "tcp_connect",
      "success_count": 0,
      "failure_count": 1,
      "error_counts": {
        "probe_tcp_failed": 1
      }
    }
  }
}
```

每个 result 可含 `node_id`、`node_name`、`method`、`target`、`core`、
`backend`、`cache_hit`、`alive`、`duration_ms`、`checked_at`、`error_code`
和 `error`。单节点失败写入该项且聚合进完整 `report`，不自动把整批变成 HTTP
error。混合 method/core 时 `report.probe.dimensions` 分组给出各组
`success_count`、`failure_count`、`cache_hit_count` 和 `error_counts`。
`report` 还可以带完整 dependencies、source refs、render diagnostics 与
warnings；字段语义以[错误与诊断的 Report 章节](../errors.md#report)为准。

### 失败与受控网络副作用

- `input` 无法解析、资源缺失、节点全部无效、method/core/target 无效，
  或 backend/core 不可用时返回结构化 service error；不会伪造空的成功 results。
- inline `NodeInput` 的 `content` 是字符串。示例或日志只能使用合成节点，不要
  记录真实凭据、订阅 URL、result target 或底层网络错误。
- `remote` 输入只经 Sandrone 受控 fetcher；probe 只经已注册 backend。接口不
  暴露任意 HTTP client、socket、宿主文件路径或任意核心配置入口。
- `tcp_connect` 会连接节点 endpoint；`udp_ntp` 会经节点检查 NTP 目标；
  `url_test` 会启动/调用所选受控核心并访问 URL。`attempts`、`timeout_ms` 和
  `concurrency` 是调用方必须显式评估的资源边界。
- `cache_ttl_seconds > 0` 时可读写 probe cache；cache 写入失败形成 warning，
  不把已完成探测改成请求失败。

### 合成示例

```sh
curl -sS "$SANDRONE_URL/v1/probe" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "input": {
      "name": "synthetic-nodes",
      "type": "inline",
      "format": "uri-list",
      "content": "ss://aes-128-gcm:example-password@proxy.example.invalid:8388#example-node"
    },
    "method": "tcp_connect",
    "timeout_ms": 1000,
    "attempts": 1,
    "concurrency": 1,
    "cache_ttl_seconds": 0,
    "meta": {
      "caller": "example"
    }
  }'
```
