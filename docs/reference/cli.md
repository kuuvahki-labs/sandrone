# CLI 参考

本页定义当前 `sandrone` 命令行契约。第一次转换可直接阅读
[第一次转换教程](../tutorials/first-conversion.md)；完整客户端文件的编写流程见
[渲染客户端配置](../how-to/render-client-config.md)。

## 命令树

```text
sandrone
├── convert
├── probe
├── validate
├── inspect
├── capability
│   ├── formats
│   └── format <parse|render> <format>
├── doctor
├── file
│   └── render
└── serve
    ├── http
    ├── mcp
    └── all
```

所有命令都接受 `--help`。根命令的 `--version` 输出
`sandrone version <version>`；构建包含 revision 时追加 12 位短 SHA。版本与
revision 的来源和职责见[构建身份](build-info.md)。

## 公共 flag 与环境变量

根命令只有一个业务公共 flag：

| flag | 环境变量 | 缺省值 | 含义 |
| --- | --- | --- | --- |
| `--data-dir <dir>` | `SANDRONE_DATA_DIR` | `./data` | 配置与资源存储目录 |

`data_dir` 是唯一不进入项目设置文件的引导值，解析顺序固定为：

```text
显式 --data-dir > SANDRONE_DATA_DIR > ./data
```

目录内可选的 `settings.json` 保存其余项目设置。文件不存在时使用内建默认值。

`serve` 及其子命令继承以下 flags：

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
`shadowrocket-proxies`、`sing-box-outbounds` 和 `uri-list`。这些是节点片段；
需要完整客户端配置时使用 `file render`。各格式的协议和有损边界见
[格式与能力参考](capabilities.md)。

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

## `probe`

```text
sandrone probe [--format <format>] \
  (--input <path|-> | --input-url <url>)
```

`probe` 输出缩进 JSON `ProbeResult`。输入和远程抓取 flags 与 `convert`
相同。对本地输入，`--format` 缺省为 `uri-list`；远程输入未显式设置
`--format` 时执行自动检测。

| flag | 缺省值 | 契约 |
| --- | --- | --- |
| `--method` | `url-test` | `tcp-connect`、`udp-ntp` 或 `url-test` |
| `--core` | `sing-box` | `url-test` 或 `udp-ntp` 使用的核心名；`url-test` 也支持 `mihomo` |
| `--url` | 空 | `url-test` 的 HTTP 目标 |
| `--ntp-server` | 空 | `udp-ntp` 的 NTP 目标 |
| `--expected-status` | 空 | `url-test` 的状态码或范围，例如 `204`、`200-299` |
| `--timeout` | 服务缺省 | 每节点超时，Go duration |
| `--attempts` | `0` | 每节点尝试次数；`0` 使用服务缺省 |
| `--concurrency` | `0` | 最大并发；`0` 使用服务缺省 |
| `--cache-ttl` | `0` | 缓存秒数；`0` 继承项目设置的 probe cache TTL，两者都为 `0` 时禁用缓存 |
| `--output` | 标准输出 | JSON 输出路径或 `-` |

`tcp-connect` 不使用核心；`udp-ntp` 当前使用 sing-box；`url-test` 支持
sing-box 和 Mihomo。单节点不存活通常记录在结果与 report 中，不等于整条命令失败；
整批无法启动或输入无效时才返回非零退出码。错误层次见
[错误与诊断参考](errors.md)。

## `validate`

`validate` 有三种互斥输入模式：

```text
sandrone validate --format <format> --input <path|->
sandrone validate [--format <format>] --input-url <url>
sandrone validate --file <name-or-spec-path>
```

- 本地节点输入必须显式给出 `--format`。
- 远程节点输入可省略 `--format` 并自动检测；远程 flags 与 `convert` 相同。
- `--file` 接受已存储的文件名，或本地 `.json`、`.yaml`、`.yml`
  `FileSpec` 路径；不得与 `--input-url` 同时使用。
- 命令执行完整验证流程，但不渲染最终文件。
- `--output <path|->` 缺省写标准输出；结果是缩进 JSON `ValidateResult`。

`ValidateResult.ok` 表示契约校验是否通过；命令仅在调用本身返回 error 时退出
`1`，不会根据 JSON 中的任意 warning 自行改变退出码。

## `inspect`、`capability` 与 `doctor`

`inspect` 输出轻量运行时与存储摘要。结果包含格式和 processor 名称、file kind、
probe 方法与运行时可用的 probe backend；不内嵌字段级 capability。`--output`
可将缩进 JSON 写入文件。

`capability formats` 输出所有 parse/render format 的摘要索引；
`capability format <parse|render> <format>` 输出一条字段级详情。两者同样支持
`--output`，精确字段解释见[格式与能力参考](capabilities.md)。

`doctor` 执行两类启动前检查：

- `--data-dir` 是否可创建、写入并删除临时检查文件；
- 所有内建输入 parser 与输出 renderer 能否处理各自的最小样本。

结果为缩进 JSON，包含顶层 `ok`、数据目录状态以及逐格式检查。任一检查失败时，
仍先输出结果，随后命令以 `1` 退出并在标准错误写入 `doctor checks failed`。

## `file render`

```text
sandrone file render <name-or-spec-path>
```

参数可以是存储中的文件名，也可以是本地 `.json`、`.yaml`、`.yml`
`FileSpec`。对于不含路径分隔符的安全名称，CLI 先查询存储；只有存储返回
“不存在”且同名本地规范文件确实存在时，才回退到本地文件。本地路径和
`FileSpec` 字段见 [FileSpec 参考](file-spec.md)。

- `--output <path|->` 写最终文件内容；缺省或 `-` 写标准输出。
- `--report-output <path>` 另写缩进 JSON report，约束与 `convert` 相同。
- `FileSpec.kind` 必须显式使用 canonical 值；CLI 不推断缺失 kind，也不接受
  大小写变体。

## `serve`

`serve` 在一个 HTTP listener 上同时提供 HTTP API、构建时嵌入的 Web UI 与 MCP
streamable HTTP。HTTP 与 MCP 共用静态 bearer token；MCP 没有独立启停开关。
发布二进制提供完整 Web 页面；未先生成 Web 资源的普通 Go 开发构建仍可启动 HTTP
API 和 MCP，但 Web 路径返回 `404`，前端由独立 Vite 开发服务器提供。

除通用的 `--listen`、`--token` 和 `--log-level` 外，`serve` 接受：

| flag | 环境变量 | 内建缺省值 | 含义 |
| --- | --- | --- | --- |
| `--path` | `SANDRONE_MCP_PATH` | `/mcp` | streamable HTTP 路径，必须以 `/` 开头且不能覆盖公开 route |
| `--allow-management-tools` | `SANDRONE_MCP_ALLOW_MANAGEMENT_TOOLS` | `false` | 注册可覆盖或立即删除定义的管理 tools；只应在可信本机 Agent 场景启用 |
| `--max-output-bytes` | `SANDRONE_MCP_MAX_OUTPUT_BYTES` | `1048576` | MCP 内联输出上限；不能为负数 |

MCP 的 tool/resource/prompt catalog、单一管理开关的行为和正文省略规则见
[MCP 参考](mcp.md)。管理 tools 缺省不注册；启用后 `put` 可覆盖同名定义，
`delete` 立即生效。
Web 开发与嵌入构建见 [Web UI 快速说明](../../web/README.md)，HTTP endpoint
契约见 [HTTP API 通用约定](http-api/README.md)。
MCP path 不能是 `/`、`/healthz`、`/version`、`/convert`、`/s` 或 `/s/*`，
避免 MCP 绕过共享 bearer token 或与公开分享/Web 路由冲突。

### 监听与鉴权约束

- 缺省监听 `127.0.0.1:1137`，本机地址允许无 token 启动。
- 监听非 loopback 地址时必须配置 `--token`。
- 只要 token 非空，HTTP 鉴权即启用。
- bearer token 同时保护普通 HTTP API 和 MCP HTTP。
- `serve` 持续运行，入口因 context 取消或 HTTP server 正常关闭而返回时视为
  正常结束；其它启动或运行错误以退出码 `1` 返回。

## 输出文件与退出约定

- `--output` 为空或 `-` 时写标准输出；文件输出会创建父目录并覆盖已有文件。
- `probe`、`validate`、`inspect`、`capability`、`doctor` 产生缩进 JSON，并以换行结尾。
- `convert` 与 `file render` 的主输出是目标格式原文，不额外包装 JSON。
- `--report-output` 的 report 总是缩进 JSON 文件。主输出先写，report
  写入随后发生；若 report 写入失败，已经写出的主输出不会回滚。
- 成功、`--help` 和 `--version` 返回退出码 `0`。
- 参数、I/O、service 或启动错误返回退出码 `1`。CLI 将 error 文本写到标准错误，
  不输出 HTTP 的 JSON error envelope，也不会在错误时自动打印完整 usage。
