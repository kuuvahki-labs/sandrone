# AGENTS.md

适用于仓库根及未另设 `AGENTS.md` 的目录；更深层规则优先。人类贡献流程见
[CONTRIBUTING.md](CONTRIBUTING.md)，系统契约从 [docs/README.md](docs/README.md)
进入。

- 先检查工作树与邻近代码/测试；保留现有改动，不扩大任务范围。
- 沿用现有分层和模式，做最小完整改动；在最接近契约的位置验证。
- 业务编排只放 `internal/service`；entrypoint 只做协议适配。
- adapter 不读写 store；processor 不依赖 adapter，也不绕过受控 I/O；service
  和 domain 不依赖 entrypoint framework。
- `FileSpec.kind` 必须是显式 canonical 值；typed `config` 只含
  `subscriptions` 和 `settings`，由对应 driver 严格解码。
- file-stage processor 按声明顺序执行。
- Web 改动遵守 [web/AGENTS.md](web/AGENTS.md)。
- 修改任一 adapter 的解析或渲染时，不能只验证当前格式；必须以 canonical
  `NodeIR` 语义为边界，按[贡献指南](CONTRIBUTING.md#选择验证范围)全局检查其它输入、
  输出、校验、能力声明和跨格式测试。
- warning 不等于兼容需求。新增 `NodeIR` 字段、扩大值域或为来源私有值增加映射前，
  必须按[字段接纳流程](docs/architecture/node-pipeline.md#字段接纳与-warning-处置)
  区分协议语义与实现配置；未知连接关键值应隔离节点，不在 probe 或 renderer 中猜测、
  截断或按前缀兼容。
- 协议 canonical 语义变化要检查该协议的全部输入和客户端输出；客户端共享 adapter
  变化要检查该客户端支持的全部协议；domain、service normalization/validation 或
  shared helper 变化要按[影响矩阵](CONTRIBUTING.md#跨协议与客户端影响矩阵)检查全部
  调用者。每项都要有已修改、测试证明无需修改或不适用的结论。
- 先跑相关窄测；交付前运行与范围匹配的门禁，默认全仓门禁为 `make check`。
- 一个事实只在 canonical 文档完整说明；其他位置只链接。
- 删除功能时同步删除专属实现、测试、fixture、示例和文档，并用 `rg` 确认旧标识
  只剩明确兼容点。不要保留功能墓碑或已完成的 plan/spec。
- 不提交真实订阅、节点 URI、凭据、私有 fixture、本机路径、运行时数据或
  agent/IDE 状态；安全问题按 [SECURITY.md](SECURITY.md) 私下报告。
