# 错误与诊断参考

Sandrone 有四种不同的失败或诊断载体：

1. `AppError`：操作无法完成时返回的类型化错误；
2. `ValidationIssue`：显式验证结果中的字段或节点问题；
3. `Warning`：操作已完成或部分完成时的结构化诊断；
4. `Report`、`SourceInfo` 与 `SourceRef`：聚合状态、计数和来源追踪。

调用方不应只根据 message 文本判断错误类型，也不应把 warning 当作失败。HTTP
请求和响应的其它通用约定见 [HTTP API 通用约定](http-api/README.md)。

## `AppError`

`AppError` 的字段为：

| 字段 | 含义 |
| --- | --- |
| `code` | 稳定、机器可判定的错误码 |
| `message` | 面向调用方的错误摘要 |
| `source`、`target` | 输入来源或输出目标上下文 |
| `file`、`part` | 文件和文件 part 上下文 |
| `processor` | 失败的 processor |
| `path` | 结构化文件中的目标路径 |
| `cause` | 底层 Go error；不参与 JSON/YAML 序列化 |

`errors.Is`/`errors.As` 可沿 `cause` 解包；嵌入方可用公开 API 的 `IsCode`
按 code 判定。CLI 打印 `AppError.Error()`，因此可能同时包含 code、message 与
底层 cause；HTTP 的公开 envelope 则有更窄的边界，见下一节。

## AppError code

### 请求、解析与渲染

| code | 含义 |
| --- | --- |
| `invalid_argument` | 请求字段、组合、格式名、路径或运行配置无效 |
| `parse_failed` | 已选 parser 无法产生有效节点 |
| `render_failed` | renderer 无法产生目标内容；非空批次没有可渲染节点也使用此码 |
| `node_validation_failed` | 普通执行链中全部节点都未通过语义校验 |
| `not_implemented` | 所需输入类型、fetcher 或 probe 能力未配置或未实现 |

### 文件与 processor

| code | 含义 |
| --- | --- |
| `file_input_not_found` | 必需文件、节点输入或远程输入无法取得 |
| `file_dependency_cycle` | 文件依赖解析遇到环 |
| `file_merge_failed` | file-stage merge 的解码、运算或编码失败 |
| `file_processor_failed` | file-stage processor 的通用失败 |
| `file_validation_failed` | 文件验证失败的保留类型码；当前内建显式验证通常返回 `ValidateResult` 或更具体错误 |
| `processor_unknown` | processor type 未注册到所需 stage |
| `processor_config_invalid` | processor 参数或 stage 选择无效、缺失或有歧义 |
| `node_processor_failed` | node-stage processor 的通用失败 |
| `script_timeout` | 脚本超过处理器配置的执行时限 |
| `script_runtime` | 脚本加载、执行、API 调用或返回值转换失败 |

processor 已返回 `AppError` 时，处理链保留原 code，并补上缺失的
`processor`；只有普通 error 才包装为 `node_processor_failed` 或
`file_processor_failed`。

### 备份

| code | 含义 |
| --- | --- |
| `backup_invalid` | 归档不可读、缺字段、路径不安全或内容无效 |
| `backup_incompatible` | 归档格式或 storage schema 与当前版本不兼容 |
| `backup_too_large` | 归档、文件数或解包后总量超过限制 |
| `backup_restore_failed` | 已通过初步检查，但恢复事务无法完成 |

### Probe 批次错误

| code | 含义 |
| --- | --- |
| `probe_backend_unavailable` | 请求的方法没有已注册 backend |
| `probe_core_unavailable` | 指定或选中的核心 backend 在当前构建不可用 |
| `probe_core_start_failed` | 真实代理核心无法分配资源、创建或启动 |
| `probe_invalid_target` | 整批 URL、payload 或核心目标配置无效 |
| `probe_core_api_failed` | URL 请求失败或响应状态不符合 `expected_status`；通常作为单节点结果码出现 |
| `probe_timeout` | probe 超时；通常作为单节点结果码出现 |
| `probe_tcp_failed` | TCP connect 失败；作为单节点结果码出现 |
| `probe_udp_ntp_failed` | UDP NTP 测试失败；作为单节点结果码出现 |

此外，probe engine 未配置时使用 `not_implemented`，输入节点语义全部无效时使用
`node_validation_failed`。批次错误不返回伪造的逐节点结果。

## HTTP handler error envelope

Sandrone handler 产生的鉴权、请求解码和 service error 使用：

```json
{
  "error": {
    "code": "invalid_argument",
    "message": "..."
  }
}
```

对于 `AppError`，HTTP 只暴露顶层 `code` 和 `message`；`source`、`target`、
`file`、`part`、`processor`、`path` 与 `cause` 不进入该 envelope。
非 `AppError` 使用 `internal_error`，message 为该 error 的文本。

HTTP status 与 `AppError.code` 是两个维度，不能互相推导。通用 service 映射为：

| HTTP status | 当前映射 |
| --- | --- |
| `400` | `invalid_argument`、`node_validation_failed`、`file_input_not_found`、`file_dependency_cycle`、`backup_invalid` |
| `404` | 非备份 service error 的 error chain 含 `os.ErrNotExist`；它会覆盖同一分支先选出的 `400` |
| `413` | `backup_too_large` |
| `422` | `backup_incompatible` |
| `500` | `backup_restore_failed` 及没有专门映射的其它 service error |

因此，远程响应失败产生的 `file_input_not_found` 通常是 `400`，而包装了本地
`os.ErrNotExist` 的同码错误是 `404`。`parse_failed`、`render_failed`、
processor、script 和多数 probe 批次错误目前默认映射为 `500`，即使其 message
描述了具体输入。

入口可为协议层错误选择更具体的 status，例如：

- 鉴权失败为 `401`，body code 当前仍为 `invalid_argument`；
- 普通 JSON 请求体超过上限为 `413`，body code 为 `invalid_argument`；
- 特定不可用依赖可由 handler 返回 `503`。

这些 status 选择不改变 handler error envelope。标准库 router 生成的部分
`404`/`405` 不经过该编码，会返回 `text/plain`；各路由允许的方法、响应类型和
endpoint 级限制以 [HTTP API 参考](http-api/README.md)为准。

## 显式验证：issue 不等于 error

`ValidationIssue` 可带 `severity`、`stage`、`code`、`message`、
`node_index`、`node_id`、`node_name`、`node_type`、`field` 和 `target`。
验证流程能够完成时，即使 `ok` 为 `false`，HTTP 仍返回正常验证响应；输入无法
解析、processor 失败等执行错误才返回 `AppError`。

完整 `ValidateResult` wire、counts 和 endpoint 失败边界见
[`POST /v1/validate`](http-api/conversion.md#post-v1validate)。

在普通 parse/render/probe 流程中，部分无效节点会被丢弃并产生
`node_validation_dropped` warning；若非空批次全部无效，则操作返回
`node_validation_failed`。

## `NodeProbeResult.error_code`

单节点 probe 失败写入该节点的 `error_code` 和 `error`。只要 backend 能完成
整批执行，一个或多个节点失败不会把整个 probe 调用变成 `AppError`。

| `error_code` | 含义 |
| --- | --- |
| `probe_node_invalid` | TCP probe 节点缺少 server 或 port |
| `probe_invalid_target` | 核心无法定位该节点或构造目标 |
| `probe_timeout` | 单节点尝试超时 |
| `probe_context_canceled` | context 在该节点完成前取消 |
| `probe_tcp_failed` | TCP connect 失败，且不是超时或取消 |
| `probe_core_api_failed` | URL 请求失败或响应状态不符合 `expected_status` |
| `probe_udp_ntp_failed` | UDP NTP 失败，且不是超时或取消 |

成功节点的 `error_code` 和 `error` 为空。每个失败节点还会在顶层 report 中产生
同 code 的 warning；`report.probe.error_counts` 以及每个 dimension 的
`error_counts` 按 code 汇总。

## Warning

`Warning` 至少包含：

```json
{
  "code": "render_lossy_field",
  "message": "...",
  "node": "node-a",
  "field": "multiplex",
  "source": "sing-box",
  "target": "mihomo-proxies"
}
```

除 `code`、`message` 外，warning 可带 `node`、`node_index`、
`node_context`、`field`、`source`、`target`。`node_index` 是源批次中的
0-based 序号，使用指针表示，因此数值 `0` 不会被省略。

当前内建 warning code 分组如下：

- 解析：`parse_line_skipped`、`parse_line_failed`、
  `parse_proxy_skipped`、`parse_outbound_skipped`、`parse_unknown_field`。
- 渲染：`render_lossy_field`、`render_node_skipped`、
  `uri_profile_unsupported`。
- 节点验证与输入：`node_validation_dropped`、`node_input_not_found`、
  `node_input_unsupported`。
- 快捷设置：`quick_settings_vmess_aead_legacy_unavailable`、
  `quick_settings_snell_reuse_unavailable`。
- 脚本：`script_ext_field`、缺省的 `script_warning`；脚本 `api.warn`
  也可以显式提供自定义 code。
- 订阅用量：`subscription_traffic_parse_failed`。
- Probe：`probe_cache_hit`、`probe_cache_write_failed`、
  `probe_expected_status_unsupported`，以及上一节的单节点 error code。

`probe_expected_status_unsupported` 当前由 `tcp_connect` 在收到
`expected_status` 时产生；它是 warning，不是批次错误。

## Report

成功操作的通用 `Report` 可包含：

| 字段 | 含义 |
| --- | --- |
| `kind` | `convert`、`render`、`file`、`probe`、`validate_nodes` 等操作类型 |
| `status` | 成功 report 缺省为 `ok` |
| `created_at` | UTC 创建时间 |
| `lossy` | 是否检测到有损或 unsupported 诊断 |
| `refs` | 对本次结果有意义的资源引用 |
| `dependencies` | 解析或生成时读取的资源依赖 |
| `source_refs` | 输入格式、远程来源或上游 schema 依据 |
| `warnings` | 跨阶段聚合 warnings |
| `render` | `success_count`、`lost_fields` 与 renderer warnings |
| `probe` | backend、method、core、成功/失败/缓存计数及 dimensions |

renderer warnings 会保留在 `report.render.warnings`，同时合并到顶层
`report.warnings`，便于既按阶段查看又统一处理。若 `refs` 为空而
`dependencies` 非空，report 会复制 dependencies 作为 refs。

`lossy` 在 warning code 包含 `loss`/`unsupported`，或
`render.lost_fields > 0` 时置为 `true`。它是摘要，不代替逐条 warning。

## 来源追踪

`SourceInfo` 包含 parser 的 `format`、`source_refs` 与 warnings。
`SourceRef` 包含：

- 必有语义的 `kind`、`name`；
- 可选的 `url`、`repo`、`revision`、`path`、`lines`、`note`。

来源可能是远程输入、本地存储文件、inline 内容、内建模板、协议规范或目标客户端
schema。`SourceRef` 是可审计来源说明，不代表输入可信，也不承诺其中的 URL 或
路径适合直接公开。

## 敏感诊断边界

诊断对象不会进行通用脱敏，尤其需要注意：

- `node_context.raw` 可包含 Mihomo、sing-box 等原始节点对象及认证字段；
- `node_context.raw_line` 可包含完整分享 URI；
- `parse_line_failed` 等 warning 的 `message` 本身也可能复述原始行；
- `NodeProbeResult.target`、`error` 和 probe warning message 可能暴露目标或
  底层网络错误；
- `SourceRef.url`、`name`、`path` 和 `note` 可能暴露订阅地址、查询参数、本地
  资源名或内容 hash；远程 source trace 只保证去掉 URL fragment，不会通用移除
  userinfo 或 query；
- CLI error 文本和服务日志可能包含 `AppError.cause`，而 HTTP `AppError`
  envelope 不包含 cause。

对外返回、持久化、转发或集中记录 errors、warnings、reports、probe results
之前，调用方必须按自己的凭据、URL、路径和节点信息策略清理这些字段。只删除
`node_context.raw` 并不足够；message、raw line、source trace 和 probe error
也必须一并检查。
