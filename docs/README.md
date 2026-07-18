# Sandrone 文档

按目标选择入口。贡献开发流程见 [CONTRIBUTING.md](../CONTRIBUTING.md)，Web 专属说明见 [web/README.md](../web/README.md)。

## 第一次使用

- [第一次转换](tutorials/first-conversion.md)：从构建 CLI 到完成一条脱敏节点转换。
- [第一次使用 Web UI](tutorials/first-web-ui.md)：启动服务，在图形界面中组合订阅并生成 Mihomo 配置。

## 完成任务

- [渲染客户端配置](how-to/render-client-config.md)：生成 Mihomo、sing-box 或 Shadowrocket 文件。
- [编写 processor 脚本](how-to/write-processor-script.md)：挂载并验证 nodes-stage 或 file-stage 脚本。
- [排查 Mihomo fake-IP](how-to/troubleshoot-mihomo-fake-ip.md)：按症状定位并验证最小修复。

## 查契约

- [CLI](reference/cli.md)：命令、flags、输出和退出约定。
- [HTTP API](reference/http-api/README.md)：鉴权、通用响应和各资源接口导航。
- [MCP](reference/mcp.md)：transport、tools、resources、prompts 和安全边界。
- [FileSpec](reference/file-spec.md)：文件类型、来源、typed config 和 settings。
- [Processors](reference/processors.md)：nodes/file stages、内建处理器和执行语义。
- [JavaScript 脚本 API](reference/scripting-api.md)：envelope、注入 API、sandbox 和失败边界。
- [格式与能力](reference/capabilities.md)：parser、renderer、协议字段与有损转换。
- [错误与诊断](reference/errors.md)：error、warning、report 和敏感信息边界。
- [Mihomo fake-IP](reference/mihomo-fake-ip.md)：当前默认、适用范围和上游依据。

## 理解系统

- [架构总览](architecture/overview.md)：上下文、层级职责、依赖方向和入口边界。
- [领域模型](architecture/domain-model.md)：`NodeIR`、`Subscription`、`FileSpec` 与诊断对象。
- [节点管线](architecture/node-pipeline.md)：parse、normalize、process、render 和能力报告。
- [文件管线](architecture/file-pipeline.md)：static/typed 构建、driver 编译和 file-stage 处理。
- [节点探测](architecture/probing.md)：运行时观测、后端、缓存和 report 边界。
- [存储架构](architecture/storage.md)：Store、协调、一致性和备份边界。

Go 嵌入式调用入口位于 [`pkg/sandrone`](../pkg/sandrone)。
