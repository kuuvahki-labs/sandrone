# CLI 参考

本页定义当前 `sandrone` 命令行契约。第一次转换可直接阅读
[第一次转换教程](../tutorials/first-conversion.md)；完整客户端文件的编写流程见
[渲染客户端配置](../how-to/render-client-config.md)。

## 命令树

```text
sandrone
├── convert
├── diagnose
│   ├── input <path|->
│   ├── url <url>
│   ├── subscription <name>
│   └── file <name-or-spec-path>
├── render
│   ├── subscription <name>
│   └── file <name-or-spec.json>
├── inspect
├── capability
│   ├── formats
│   └── format <parse|render> <format>
├── doctor
└── serve
```

所有命令都接受 `--help`。根命令的 `--version` 输出
`sandrone version <version>`；构建包含 revision 时追加 12 位短 SHA。版本与
revision 的来源和职责见[构建身份](build-info.md)。

## 公共 flag 与环境变量

根命令只有一个业务公共 flag：

| flag | 环境变量 | 缺省值 | 含义 |
| --- | --- | --- | --- |
| `--data-dir <dir>` | `SANDRONE_DATA_DIR` | `./data` | 配置与资源存储目录 |

`data_dir` 是不进入项目设置文件的 filesystem 引导值，解析顺序固定为：

```text
显式 --data-dir > SANDRONE_DATA_DIR > ./data
```

目录内可选的 `settings.json` 保存其余项目设置。文件不存在时使用内建默认值。
选择 S3 后 `--data-dir` 不生效。

存储后端只通过环境变量选择，不提供对应 flag：

| 环境变量 | 缺省值 | S3 模式要求 | 含义 |
| --- | --- | --- | --- |
| `SANDRONE_STORAGE_BACKEND` | `filesystem` | 是 | `filesystem` 或 `s3` |
| `SANDRONE_S3_ENDPOINT` | 空 | 必填 | 绝对 HTTP(S) S3 endpoint |
| `SANDRONE_S3_REGION` | 空 | 必填 | 服务 region；R2 使用 `auto` |
| `SANDRONE_S3_BUCKET` | 空 | 必填 | 已存在的私有 bucket |
| `SANDRONE_S3_PREFIX` | `sandrone/` | 否 | 非空安全 namespace |
| `SANDRONE_S3_FORCE_PATH_STYLE` | `false` | 否 | path-style addressing |
| `SANDRONE_S3_ACCESS_KEY_ID` | 空 | 必填 | 显式 access key ID |
| `SANDRONE_S3_SECRET_ACCESS_KEY` | 空 | 必填 | 显式 secret access key |
| `SANDRONE_S3_SESSION_TOKEN` | 空 | 否 | 临时凭据的 session token |

这些字段不会写入 `settings.json` 或备份。Sandrone 不使用 AWS 默认凭据链。
R2 常规配置保持 `SANDRONE_S3_FORCE_PATH_STYLE=false`。

`serve` 接受以下 flags：

| flag | 环境变量 | 内建缺省值 | 含义 |
| --- | --- | --- | --- |
| `--listen <host:port>` | `SANDRONE_LISTEN` | `127.0.0.1:1137` | HTTP 监听地址 |
| `--token <token>` | `SANDRONE_TOKEN` | 空 | HTTP 与 MCP HTTP 的 bearer token |
| `--log-level <level>` | `SANDRONE_LOG_LEVEL` | `info` | `debug`、`info`、`warn` 或 `error` |

启动字段的优先级是：

```text
显式 flag > 环境变量 > <data_dir>/settings.json > 内建默认值
```

环境变量或 flag 只覆盖当前进程，不回写 `settings.json`。token 只从
`--token` 或 `SANDRONE_TOKEN` 读取，既不属于项目设置，也不会进入备份。管理
API 保存了其它被覆盖字段时，当前进程继续使用覆盖值。`warning` 也会被接受为
`warn` 日志级别。统一文件与管理接口见[项目设置接口](http-api/settings.md)。

## 节点格式

需要输入格式的命令接受：

- `uri`：单个分享 URI；
- `uri-list`：逐行分享 URI；
- `base64`：Base64 编码的 URI 列表；
- `mihomo`：带 `proxies` 的 Mihomo YAML/JSON；
- `sing-box`：带 `outbounds` 或 `endpoints` 的 sing-box JSON；
- `json-nodes`：Sandrone 规范化节点 JSON。

节点输出格式为 `json-nodes`、`mihomo-proxies`、
`shadowrocket-proxies`、`sing-box-outbounds` 和 `uri-list`。各格式的协议和
有损边界见[格式与能力参考](capabilities.md)。

## `convert`

```text
sandrone convert --to <format> \
  (--from <format> --input <path|-> | --input-url <url>)
```

`convert` 解析节点、规范化后直接渲染为目标节点格式。

- 本地输入必须提供 `--from`；`--input` 缺省为 `-`，即标准输入。
- `--input-url` 使用受控远程抓取，并允许省略 `--from` 以自动检测格式。
- 显式给出 `--input`（包括 `--input -`）后不得同时给出 `--input-url`。
- `--to` 必填。
- `--output <path|->` 控制主输出；省略或使用 `-` 时写标准输出。
- `--report-output <path>` 将完整 report 写为缩进 JSON；它必须是文件路径，
  不能为 `-`，也不能与 `--output` 指向同一文件。

远程输入 flags 为：

| flag | 缺省值 | 含义 |
| --- | --- | --- |
| `--input-url <url>` | 空 | 远程输入 URL |
| `--user-agent <value>` | 空 | 抓取请求的 `User-Agent` |
| `--proxy <url>` | 空 | HTTP、HTTPS 或 SOCKS 代理 URL |
| `--remote-timeout <duration>` | 服务缺省 | 远程抓取超时，使用 Go duration，例如 `5s` |

自动检测只用于远程输入；候选包括 `base64`、`uri-list`、`mihomo` 和
`sing-box`。`json-nodes` 不在自动检测候选中。

## `diagnose`

```text
sandrone diagnose input <path|-> [--kind auto|nodes|subscription|file]
sandrone diagnose url <url>
sandrone diagnose subscription <name> [--cache-mode refresh|reuse]
sandrone diagnose file <name-or-spec-path>
```

`diagnose` 是唯一公开诊断入口。它识别并执行节点文档、Sandrone Subscription
或 FileSpec 声明的完整流水线，但不会自动加入 `probe`，也没有 `--live`。只有输入
或一次性 processor 文件显式声明 `probe` processor（或脚本调用 `api.probe`）时
才会测活。

Subscription、FileSpec 和一次性 ProcessorSpec 文件使用 JSON 定义。节点文档
仍按所选节点格式解析。

- `input` 的 `--kind` 缺省为 `auto`；强结构同时命中多个类型时以
  `input_kind_ambiguous` 失败，无法识别时以 `input_kind_unrecognized` 失败。
- `--format` 只覆盖节点格式识别，不会让 Subscription 或 FileSpec 静默退化为节点。
- `--processors <json-path|->` 只用于 nodes 输入；输入和 processors 不能
  同时从标准输入读取。文件内容必须是顶层 `ProcessorSpec[]`。
- `url` 只把远程正文解释为节点文档，并支持 `--user-agent`、`--proxy`、
  `--remote-timeout`、`--format` 和 `--processors`。
- `subscription` 从 Store 按名称读取并执行嵌套来源与已保存 processors；默认
  `--cache-mode refresh` 强制新鲜诊断，显式 `--cache-mode reuse` 可以复用有效的
  subscription-snapshot。
- `file` 对安全名称先查询 Store，本地 `.json` 路径作为 FileSpec 回退，并执行
  完整 typed compile、订阅依赖和 file-stage processors。
- 四个子命令都接受 `--output <path|->`。诊断 JSON 含完整节点、文件正文、report
  与 probe trace，属于敏感数据；新建输出文件权限固定为 `0600`。

结果 `status` 为 `ok`、`partial` 或 `failed`。`ok`/`partial` 退出 `0`；
`failed` 仍先输出结构化 JSON，再退出 `1`。Cobra 参数组合错误在 service 执行前
失败，只写标准错误。每个 processor stage 记录 scope、声明顺序、前后计数、
warnings，以及其内部产生的完整 probe results，即使 `annotate=false`。

## `render`

```text
sandrone render subscription <name> --format <format>
sandrone render file <name-or-spec.json>
```

`render` 执行已声明资源的完整物化流程并直接输出最终正文。三个相关入口的职责是：

- `convert`：将一次性节点输入转换为另一种节点格式；
- `diagnose`：执行输入流水线并输出 stages、warnings、report 和 probe trace 等
  诊断 JSON；
- `render`：执行 Store 中的 Subscription 或 Store/本地的 FileSpec，并输出供
  客户端使用的最终正文。

`render subscription` 只接受 Store 中的 Subscription 名称，`--format` 必填，格式范围
与 `convert --to` 相同。`render file` 对安全名称先查询 Store；资源不存在且参数是已有
本地 `.json` 文件时，将其作为 FileSpec 执行。

两个子命令共同支持：

- `--arg <key=value>` 为本次执行提供字符串请求参数，可重复；同名参数以后者为准；
- `--refresh` 绕过已保存订阅的 snapshot、remote-fetch 与 probe 缓存并重新物化；
- `--output <path|->` 控制最终正文输出，省略或使用 `-` 时写标准输出；
- `--report-output <path>` 将完整 report 写为缩进 JSON，不能为 `-`，也不能与
  `--output` 指向同一文件。

## `inspect`、`capability` 与 `doctor`

`inspect` 输出轻量运行时与存储摘要。结果包含格式和 processor 名称、file kind、
probe 方法与运行时可用的 probe backend；不内嵌字段级 capability。`--output`
可将缩进 JSON 写入文件。

`capability formats` 输出所有 parse/render format 的摘要索引；
`capability format <parse|render> <format>` 输出一条字段级详情。两者同样支持
`--output`，精确字段解释见[格式与能力参考](capabilities.md)。

`doctor` 执行两类启动前检查：

- filesystem 模式检查 `--data-dir` 是否可创建、写入并删除临时文件；
- S3 模式在 `_doctor/` 下执行 write/read/stat/list/delete round trip，并
  best-effort 清理临时对象；
- 所有内建输入 parser 与输出 renderer 能否处理各自的最小样本。

结果为缩进 JSON，包含顶层 `ok`、`storage_backend`、`storage_ok`、可选数据目录
状态以及逐格式检查。任一检查失败时，
仍先输出结果，随后命令以 `1` 退出并在标准错误写入 `doctor checks failed`。

## `serve`

`serve` 在一个 HTTP listener 上同时提供 HTTP API、构建时嵌入的 Web UI 与 MCP
streamable HTTP。HTTP 与 MCP 共用静态 bearer token；MCP 没有独立启停开关。
发布二进制提供完整 Web 页面；未先生成 Web 资源的普通 Go 开发构建仍可启动 HTTP
API 和 MCP，但 Web 路径返回 `404`，前端由独立 Vite 开发服务器提供。

除通用的 `--listen`、`--token` 和 `--log-level` 外，`serve` 接受：

| flag | 环境变量 | 内建缺省值 | 含义 |
| --- | --- | --- | --- |
| `--path` | `SANDRONE_MCP_PATH` | `/mcp` | streamable HTTP 路径，必须以 `/` 开头且不能覆盖公开 route |
| `--max-output-bytes` | `SANDRONE_MCP_MAX_OUTPUT_BYTES` | `1048576` | MCP 内联输出上限；不能为负数 |

MCP 的 tool/resource/prompt catalog、管理边界和正文省略规则见
[MCP 参考](mcp.md)。管理 tools 始终注册；`put` 可覆盖同名定义，`delete`
立即生效。
Web 开发与嵌入构建见 [Web UI 快速说明](../../web/README.md)，HTTP endpoint
契约见 [HTTP API 通用约定](http-api/README.md)。
MCP path 不能是 `/`、`/healthz`、`/version`、`/convert`、`/s` 或 `/s/*`，
避免 MCP 绕过共享 bearer token 或与公开分享/Web 路由冲突。

选择 S3 只替换持久化 Store；长驻 `serve` 仍运行定时更新，并保留构建中可用的
probe backend。Vercel serverless profile 的不同能力边界见
[Vercel + Cloudflare R2 部署](../how-to/deploy-vercel-r2.md)。

### 监听与鉴权约束

- 缺省监听 `127.0.0.1:1137`，本机地址允许无 token 启动。
- 监听非 loopback 地址时必须配置 `--token`。
- 只要 token 非空，HTTP 鉴权即启用。
- bearer token 同时保护普通 HTTP API 和 MCP HTTP。
- `serve` 持续运行，入口因 context 取消或 HTTP server 正常关闭而返回时视为
  正常结束；其它启动或运行错误以退出码 `1` 返回。

## 输出文件与退出约定

- `--output` 为空或 `-` 时写标准输出；文件输出会创建父目录并覆盖已有文件。
- `diagnose`、`inspect`、`capability`、`doctor` 产生缩进 JSON，并以换行结尾。
- `convert` 与 `render` 的主输出是目标格式或最终文件原文，不额外包装 JSON。
- `--report-output` 的 report 总是缩进 JSON 文件。主输出先写，report
  写入随后发生；若 report 写入失败，已经写出的主输出不会回滚。
- 成功、`--help` 和 `--version` 返回退出码 `0`。
- 参数、I/O、service 或启动错误返回退出码 `1`。CLI 将 error 文本写到标准错误，
  不输出 HTTP 的 JSON error envelope，也不会在错误时自动打印完整 usage。
