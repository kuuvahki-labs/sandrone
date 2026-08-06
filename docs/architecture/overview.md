# 架构总览

## 项目上下文

Sandrone 是一套可嵌入、可部署的订阅与节点转换引擎。它用统一领域模型连接外部订阅格式、节点处理链、完整客户端配置、运行时探测和服务入口。

核心目标是让同一份转换语义可以被 Go API、CLI、HTTP API、MCP 和 Web UI 复用，同时保持外部格式、运行时能力和持久化实现彼此隔离。

这里的“轻量”主要指部署和依赖边界：

- 核心转换能力可以作为 Go 包嵌入。
- 默认运行形态是单个 Go 二进制，不要求数据库或额外常驻 runtime。
- JavaScript processor 使用进程内 Goja，不依赖 Node.js。
- 完整客户端配置、脚本和探测是受控扩展，不反向污染节点 IR。
- 存储、远程读取和可选探测核心都通过窄边界装配。

## 主要数据流索引

Sandrone 有三类产物和一类运行时观测：

- 节点产物把来源解析为 `NodeIR`，经过节点阶段处理后渲染为目标节点格式；完整语义见[节点管线](node-pipeline.md)。
- 文件产物按 `FileSpec.kind` 选择静态读取或 typed-file driver 编译，再执行文件阶段处理；完整语义见[文件管线](file-pipeline.md)。
- 探测读取节点或订阅并返回一次性的可达性结果；它不是协议转换的一部分，见[节点探测](probing.md)。
- 订阅、文件定义、项目设置、分享和内部缓存经统一 Store 边界保存，见[存储架构](storage.md)。

这些路径共享[领域模型](domain-model.md)、service 编排、结构化错误和报告，但不会把某一条路径的运行时状态写进另一条路径的稳定模型。

## 分层职责

| 层或包 | 稳定职责 |
| --- | --- |
| `cmd/sandrone` | 进程入口，只启动 CLI |
| `internal/app` | 装配 service、存储、日志和服务型入口的运行配置 |
| `internal/entry/*` | 适配 CLI、HTTP、MCP 和 Web 静态资源，不复制业务编排 |
| `pkg/sandrone` | 对嵌入方提供稳定 façade、请求模型别名和 Store 注入点 |
| `internal/service` | 唯一业务编排层，组合 parser、renderer、processor、probe、fetcher 和 store |
| `internal/domain` | 领域值、请求与结果、错误码、warning、report 和 source trace |
| `internal/adapter/*` | 外部节点格式与 `NodeIR` 之间的解析、渲染和能力声明 |
| `internal/inidoc` | 保留格式的 INI 文档与 override 运算 |
| `internal/processor` | nodes/file 两阶段 registry、顺序执行和内建处理器 |
| `internal/probe` | 探测后端 registry、TCP 后端和可选核心后端 |
| `internal/fetcher`、`internal/cache` | 受控远程输入与独立的非权威 TTL Cache 边界；当前提供 Store-backed 实现 |
| `internal/store` | Store、MetaStore、Coordinator 和文件系统实现 |

`internal/service` 是业务编排边界，不是通用工具包。跨 adapter、processor、probe、fetcher 和 store 的业务顺序只在这一层出现。

## 依赖方向

主要调用方向是：

```text
cmd -> entry/app -> service -> domain
pkg/sandrone -> service -> domain
service -> adapter | processor | probe | fetcher | cache | store
adapter | processor | probe -> domain
```

必须保持以下边界：

- entrypoint 调用 service，不自行解析节点、执行 processor 或直接操作 Store。
- domain 和 service 契约不暴露 Cobra、`net/http` 或 MCP SDK 类型。
- adapter 不读写 Store，也不负责服务入口或探测。
- processor 不 import adapter；网络、资源和探测只能使用 service 注入的窄接口。
- probe 不依赖 entry、adapter 或 processor；需要目标格式 payload 时由 service 使用 renderer 准备。
- Web UI 通过 HTTP API 使用同一 service，不形成独立业务后端。

这些限制让格式支持、处理策略、入口协议和部署方式能够分别演进。

## 入口边界

所有入口都先把协议输入转换为领域请求，再调用相同的 service：

- CLI 负责命令树、参数、标准输入输出和退出码。
- HTTP API 负责路由、鉴权、请求解码、响应编码和下载呈现。
- MCP 负责 tools、resources、prompts 与 transport 的适配，并限制可见资源和输出大小。
- Web UI 由可选静态资源 handler 提供，其数据操作仍经过受保护的 HTTP API。
- `pkg/sandrone.Engine` 是嵌入式调用 façade，不要求调用方 import `internal` 包。

入口层拥有协议级权限和呈现策略；service 拥有业务校验、资源解析和执行顺序。入口不能通过宿主文件路径或任意 Store key 绕过 service 的受控资源模型。

具体调用契约分别见 [CLI 参考](../reference/cli.md)、
[HTTP API 参考](../reference/http-api/README.md)和 [MCP 参考](../reference/mcp.md)。

## 轻量部署原则

默认运行时使用 `afero` 文件系统 Store，可以换成内存文件系统或嵌入方实现的 Store。没有数据库事务或跨进程协调假设；需要共享后端或高可用发布时，协调和备份由部署层补充。

远程订阅、远程文件和远程脚本都经过同一类受控 HTTP(S) fetcher，受超时、大小、代理和缓存策略限制。processor 不获得通用网络客户端或任意文件系统访问。

TCP 探测始终可用；mihomo 和 sing-box 探测能力由 build tag 进入构建图。可选核心与主进程一起编译和运行，不依赖系统中另行安装的可执行文件。

Web UI 可以内嵌或从指定静态目录提供。管理页面、HTTP API 和 MCP 可以共享 listener，但它们仍保留各自的路由和鉴权边界。

## 扩展原则

- 新节点格式通过 adapter 和 capability 声明扩展，不在 `NodeIR` 中复制目标客户端对象。
- 新 typed file kind 通过 driver descriptor、严格 settings 解码和编译器扩展，不在公共编排中增加客户端二选一分支。
- 新处理行为通过明确的 nodes 或 file stage 注册；开放式逻辑使用受限脚本。
- 新入口只适配 service 请求和结果，不重新实现业务流程。
- 新存储后端实现 Store 读写、列举与 metadata 契约；复合一致性继续由 Coordinator 边界管理。

字段与格式的当前支持范围以[格式与能力参考](../reference/capabilities.md)和运行时 capability summary 为准，架构页不维护重复清单。
