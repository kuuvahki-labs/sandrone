# AGENTS.md

适用于仓库根及未另设 `AGENTS.md` 的目录；更深层规则优先。开发、验证和
[任务完成标准](CONTRIBUTING.md#任务完成标准)见贡献指南，系统契约从
[文档索引](docs/README.md)按任务查阅。

- 先检查工作树与邻近代码/测试；保留现有改动，沿用已有分层和模式。
- 资料按需读取：涉及契约再查对应 reference，涉及架构再查对应架构页；
  不要求每次通读文档，同一任务中已读且未变化的内容无需重读。
- 业务编排只放 `internal/service`；entrypoint 只做协议适配。
- adapter 不读写 store；processor 不依赖 adapter，也不绕过受控 I/O；service
  和 domain 不依赖 entrypoint framework。
- `FileSpec.kind` 必须是显式 canonical 值；typed `config` 只含
  `subscriptions` 和 `settings`，由对应 driver 严格解码。
- file-stage processor 按声明顺序执行。
- 配置策略归属遵守[文件管线](docs/architecture/file-pipeline.md#配置策略归属)，平台能力与显式策略分别维护。
- Web 改动遵守 [web/AGENTS.md](web/AGENTS.md)。
- adapter 改动以 `NodeIR` 语义为边界判断影响；共享语义、协议值域或客户端共享
  路径变化按[影响矩阵](CONTRIBUTING.md#跨协议与客户端影响矩阵)检查，局部改动
  验证受影响路径及转换边界，不因文件位置无差别扩大范围。
- warning 不等于兼容需求。新增 `NodeIR` 字段、扩大值域或为来源私有值增加映射前，
  按[字段接纳流程](docs/architecture/node-pipeline.md#字段接纳与-warning-处置)
  区分协议语义与实现配置；未知连接关键值应隔离节点，不在 probe 或 renderer 中猜测、
  截断或按前缀兼容。
- 按[验证范围](CONTRIBUTING.md#选择验证范围)先做相关窄测，再完成风险对应的门禁；
  已通过的检查没有新改动、失败或未解决风险时不重复运行。
- 一个事实只在 canonical 文档完整说明；其他位置只链接。
- 删除或重命名时清理本任务涉及的旧实现、专属测试、fixture、示例与文档，并用
  `rg` 检查旧标识；保留仍有效的兼容契约和回归测试。临时材料按
  [文档政策](CONTRIBUTING.md#文档政策)整理。
- 已授权的工作持续执行到相应完成标准；普通实现选择不重复确认，诊断阶段结束
  不应中断已授权的修复。仅在缺少必要信息或授权时询问，并继续不依赖该答复的工作。
- 不提交真实订阅、节点 URI、凭据、私有 fixture、本机路径、运行时数据或
  agent/IDE 状态；安全问题按 [SECURITY.md](SECURITY.md) 私下报告。
