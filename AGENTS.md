# AGENTS.md

适用于仓库根及未另设 `AGENTS.md` 的目录；更深层规则优先。

- 先检查工作树与邻近代码、测试，保留无关改动。
- 开发与验证按[贡献指南](CONTRIBUTING.md)执行；从[文档索引](docs/README.md)
  按任务查阅资料，无需通读全部文档。
- 分层与依赖见[架构总览](docs/architecture/overview.md)；Web 修改另读
  [Web 模块约定](web/AGENTS.md)。
- 转换变更以 `NodeIR` 语义和受影响调用者判断范围，见
  [跨协议与客户端影响矩阵](CONTRIBUTING.md#跨协议与客户端影响矩阵)。
- 文档只保留会影响理解或操作的信息，维护规则见
  [文档政策](CONTRIBUTING.md#文档政策)。
- 不提交真实订阅、凭据、私有运行数据或本机配置；安全报告见
  [SECURITY.md](SECURITY.md)。
