# 架构总览

Sandrone 用统一领域模型连接订阅格式、节点处理链、完整客户端配置、运行时探测和
服务入口。同一套 service 编排供 Go API、CLI、HTTP、MCP 和 Web UI 调用。

## 主要路径

- [节点管线](node-pipeline.md)：来源解析为 `NodeIR`，处理后输出目标节点片段。
- [文件管线](file-pipeline.md)：按 `FileSpec.kind` 读取静态文件或编译完整配置，再执行文件处理器。
- [节点探测](probing.md)：返回一次运行时观测，区分不可达与所选后端不支持。
- [存储](storage.md)：保存资源定义、项目设置、分享，以及可重建缓存。

持久化定义与运行时结果的关系见[领域模型](domain-model.md)。

## 分层职责

| 层或包 | 职责 |
| --- | --- |
| `cmd/sandrone`、`internal/app` | 进程启动与 service、存储、日志、运行能力装配 |
| `internal/entry/*` | CLI、HTTP、MCP 与 Web 资源的协议适配、鉴权和响应呈现 |
| `pkg/sandrone` | 嵌入式 façade、公开模型别名及依赖注入 |
| `internal/service` | 组合资源读取、parser、processor、renderer、probe 与报告的业务流程 |
| `internal/domain` | 领域值、请求、结果和诊断 |
| `internal/adapter/*` | 外部节点格式与 `NodeIR` 的解析、渲染和能力声明 |
| `internal/filedriver` | 各 typed kind 的 settings 校验和完整文件编译 |
| `internal/processor` | nodes/file registry、声明顺序执行与处理策略 |
| `internal/probe` | backend 注册、选择和探测执行 |
| `internal/fetcher`、`internal/cache` | 受控远程输入与非权威 TTL 缓存 |
| `internal/store` | Store、资源编码与复合读写协调 |
| `internal/inidoc` | 保留格式的 INI 文档与 override 运算 |

## 依赖边界

```text
cmd -> entry/app -> service -> domain
pkg/sandrone -> service -> domain
service -> adapter | filedriver | processor | probe | fetcher | cache | store
adapter | filedriver | processor | probe -> domain
```

业务编排集中在 service，入口负责协议适配。domain 与 service 契约不暴露入口框架类型；
adapter 不读写 Store。processor 和 probe 通过 service 注入的窄接口取得资源、探测
或已渲染 payload，不另建格式映射或绕过受控 I/O。

新增能力放在对应边界：节点格式扩展 adapter，完整配置扩展 typed driver，处理策略
按 nodes/file stage 注册，存储实现 Store。各客户端结构留在 adapter/driver，公共编排
依据注册描述执行。

## 运行与入口

默认部署为单个 Go 二进制，文件系统存储；JavaScript 使用进程内 Goja。可选探测核心
通过 build tag 编译进进程，能力以当前构建的 runtime summary 为准。共享存储的并发
和恢复保证见[存储架构](storage.md)。

生产 Web UI 由 Go 二进制内嵌资源提供，经 HTTP API 使用 service；开发时使用独立
Vite server。CLI、HTTP、MCP 和
嵌入式 API 的调用约定分别见 [CLI](../reference/cli.md)、
[HTTP API](../reference/http-api/README.md)、[MCP](../reference/mcp.md)及
[`pkg/sandrone`](../../pkg/sandrone/sandrone.go)。
