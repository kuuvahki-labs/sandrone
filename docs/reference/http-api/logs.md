# 程序日志

## 查看

Web UI 从「设置 → 程序日志」进入，手动刷新当前快照。筛选只作用于已加载
记录，不改变程序日志级别；刷新失败时保留旧记录，供继续检查。程序级别的配置和
重启要求见[项目设置](settings.md)。

## `GET /v1/logs`

接口使用[管理 API 鉴权](README.md#鉴权)，无查询参数，返回当前服务 Runtime 的
全部有界日志快照，并设置 `Cache-Control: no-store`。

```json
{
  "instance_id": "a00b6026-75c3-40d9-826d-b20387045e61",
  "snapshot_time": "2026-09-06T01:02:03.456Z",
  "level": "info",
  "max_entries": 1000,
  "max_bytes": 2097152,
  "max_entry_bytes": 8192,
  "dropped": 0,
  "entries": [
    {
      "id": 1,
      "time": "2026-09-06T01:02:01.123Z",
      "level": "info",
      "message": "starting HTTP API",
      "attrs": { "listen": "127.0.0.1:1137" },
      "truncated": false
    }
  ]
}
```

- `instance_id` 标识当前 Runtime，重新创建后变化；`id` 在实例内递增，排序以
  ID 为准，不依赖系统时钟是否回拨。`entries` 按 ID 降序，空日志为 `[]`。
- `snapshot_time` 是读取快照的时间；`level` 是 logger 建立时生效的日志级别。
- 同时保留最多 1,000 条、2 MiB 序列化记录；先达到任一上限时淘汰最旧记录，
  `dropped` 为累计淘汰条数。总容量不包含响应元数据和临时读取副本。
- 每条序列化记录最多 8 KiB。消息、结构化字段和 logger 上下文均受限；超长文本、
  过多或过深字段可截断或省略，`truncated` 标记这种情况。消息优先于超限字段保留。
- 日志以原文提供，不做脱敏；嵌套字段以 JSON 展示，页面仅按文本渲染。

## 生命周期

日志副本仅在内存中，重启后清空，不进入 Store、备份或持久化缓存。捕获范围从
统一 logger 建立后开始，包括使用该 logger 的 service、HTTP 和 MCP 日志，
继续保留原有终端输出。其他进程、探测子进程的原始 stderr 和业务 warning 报告
不会被额外采集。

成功读取日志接口的访问日志只进入原有终端输出，不写入网页缓存；失败请求仍按
通常规则记录。默认 `info` 级别不会采集普通成功 HTTP 请求的 `debug` 日志。
