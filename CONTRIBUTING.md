# 贡献指南

本文件是 Sandrone 人类贡献流程的唯一权威。自动化代理的快速约束见 [AGENTS.md](AGENTS.md)，产品与系统文档从[文档索引](docs/README.md)进入。

参与讨论和提交变更即表示你同意遵守[行为准则](CODE_OF_CONDUCT.md)。

## 准备开发环境

在仓库根目录运行 Go 命令。建议准备：

- Go `1.27.0`，版本以 `go.mod` 为准。
- Node.js `24.17.0` 和 pnpm `11.24.0`，仅 Web 开发需要。
- Docker 与 Docker Compose，用于验证容器运行路径。
- 支持当前 Go 版本的 `golangci-lint`，运行 `make check` 或 `make lint` 时需要。

首次进行 Web 开发时安装依赖：

```sh
nvm install 24.17.0
nvm use 24.17.0
corepack enable
corepack install --global pnpm@11.24.0
pnpm --dir web install --frozen-lockfile
```

运行 `make help` 可以查看维护的 Make targets。`make build` 和 `make build-bin` 会联网生成规则集目录快照；默认质量门 `make check` 使用已有源码完成检查，不依赖该下载步骤。发布构建的 `VERSION`、`REVISION` 和容器 metadata 契约见[构建身份](docs/reference/build-info.md)。

## 贡献流程

1. 从请求和现有上下文确定要解决的问题、范围与完成标准；明确的授权无需重复确认。
2. 检查工作树和邻近代码/测试。契约、架构或命令不明确时，只查对应文档章节；已读且未变化的资料不重复读取。
3. 实现最小完整改动。行为变化或缺陷修复在最接近契约的位置补充必要回归测试；纯文案、样式或机械整理不强制新增测试。
4. 按下文风险范围验证，同步更新拥有变化事实的 canonical 文档。
5. 删除或重命名时清理本任务相关的旧内容并扫描旧标识，保留仍有效的兼容说明和回归测试。
6. 仅在任务包含提交或 PR 时执行对应流程。

不要覆盖或顺手整理无关工作树改动。必要的关联修复可在已授权目标内继续；扩大到新的目标前说明原因并取得授权。

## 任务完成标准

| 任务 | 完成条件 |
| --- | --- |
| 只读审查或方案 | 给出有位置与证据的问题、影响及调整方案；用户要求确认后修改时，在此等待。 |
| 实现或修复 | 完成已授权结果、匹配风险的验证、必要文档与本任务清理；交付实际结果及残余限制。 |
| 本地提交 | 实现完成后检查暂存 diff 与 `git diff --cached --check`，仅提交授权范围，确认 commit 和剩余工作树状态。 |
| PR、发布或部署 | 仅在任务包含该动作时执行，满足相应检查和授权边界，并确认动作结果。 |

已授权的诊断并修复任务应继续到修复完成，不因诊断报告或阶段计划而停工。
检查通过后，无新改动、失败或未解决风险，不重复验证。遇到问题先在授权范围内解决；
缺少必要信息或授权时才询问，并继续独立工作。环境阻碍应报告具体命令、缺口和替代检查，
不得把受阻验证声称为通过，也不为无关失败扩大修复范围。

## 架构边界

[架构总览](docs/architecture/overview.md)说明层级职责和依赖方向；领域对象见[领域模型](docs/architecture/domain-model.md)。完整数据流分别归属[节点管线](docs/architecture/node-pipeline.md)和[文件管线](docs/architecture/file-pipeline.md)，运行时观测与持久化边界见[节点探测](docs/architecture/probing.md)和[存储架构](docs/architecture/storage.md)。

Web 依赖方向和 FileDriver 分工见 [Web 模块约定](web/AGENTS.md)。

## 选择验证范围

所有修改交付前检查 `git diff --check`，其余按风险选择；下列覆盖可组合。

| 改动 | 本地交付验证 |
| --- | --- |
| 纯文档、规则说明、Skill 指令 | 检查相关链接/锚点、命令与规则一致性、示例脱敏；Skill 另做结构校验。不要求 Go/Web 全量检查。 |
| Go 实现 | 相关包或行为窄测后运行 `make check`，包含 Go lint。 |
| Web 局部逻辑或组件 | 相关 Vitest（行为变化时）、`pnpm --dir web typecheck` 与 `pnpm --dir web lint`；纯文案/样式无需新增单元测试。 |
| Web 共享模型、状态、驱动或依赖 | 在上项基础上运行 `pnpm --dir web test:run`；涉及构建或依赖时加 `pnpm --dir web build`。 |
| Web 路由、浏览器集成或响应式布局 | 在相关静态检查基础上运行对应 Playwright 用例：`pnpm --dir web test:e2e` 可带用例路径。合并 Web 变更或发版前运行完整 smoke。 |
| Shell、构建、CI 或部署配置 | 语法/配置校验及受影响命令或构建路径的验证；不因文件后缀是文档就省略可执行示例的必要检查。 |

`make check` 是完整 Go 门禁：格式检查、`go vet`、默认 Go 测试、CLI 构建和 Go lint，
不包含 Web 检查。远端
[CI](.github/workflows/ci.yml) 另有模块校验与独立 Web job，保留其完整检查。
迭代时可单独运行 `make test`、`make vet` 或 `make lint`；完整门禁通过后无需重复运行单项。

使用 `make fmt` 自动格式化 Go 文件并排序 import，按标准库、第三方、本项目分组；
`make lint` 会检查同一套 `gci` / `goimports` 规则。不要手动调整 import 排序。

Playwright 默认启动 built SPA，已包含 `pnpm build`；成功的默认 E2E 可计入同一版本的
构建验证。使用外部 server 时需确认其产物来自当前修改，否则不能据此声称构建已验证。
构建并复制 Go 嵌入资源时使用 `make build-webui`。

Makefile 默认使用
`GOFLAGS=-mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls`。
运行窄测试时也应保留这些 flag：

```sh
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/service
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls -run '^TestName$' ./internal/service
make test PKGS=./internal/service TESTFLAGS='-run ^TestName$'
```

按变更类型选择额外覆盖：

- adapter：按下方影响矩阵确定受影响路径，检查 parse/render、validation、capability
  和 warning 是否仍一致。新增 IR 语义的判断见
  [字段接纳与 warning 处置](docs/architecture/node-pipeline.md#字段接纳与-warning-处置)。
- processor、service、store、entrypoint：在最接近公开或层间契约的位置测试；文件流不要只依赖 renderer golden。
- probe：默认门禁覆盖 sing-box；修改 Mihomo backend 时额外运行
  `go test -mod=readonly -tags probe_mihomo ./internal/probe ./internal/service`。
- 用户可见 API、CLI、文件模型或 processor 行为：同时核对 canonical reference 和相关 tutorial/how-to。
- 删除功能：用 `rg` 查找旧标识，清理失效实现与说明，保留仍有效的兼容和回归覆盖。

### 跨协议与客户端影响矩阵

按语义与依赖确定影响面，用调用关系或测试验证判断。局部修复检查受影响的转换
边界；共享语义变化沿调用者展开。交付说明只记录相关结论和未验证的风险。

| 修改触点 | 关注的影响 |
| --- | --- |
| `NodeIR` 字段、协议语义、normalize/validation | 使用该语义的输入与输出、JSON/script 表达、clone、identity 与缓存比较 |
| shared helper 或客户端共享映射 | 全部调用者及受影响协议；其它 renderer 是否解释相同语义 |
| parser 来源映射或 renderer 输出 | 跨格式转换、未知字段、skip/lossy warning、capability 声明 |
| service 或 processor 编排 | 直接转换、保存订阅与文件流程，以及处理前后校验 |
| capability、warning 或 report | 生成与聚合行为，以及受影响的 HTTP/CLI/MCP 展示 |
| probe backend 或 payload | 受影响核心、节点隔离和保存订阅处理链；共享语义修复应在转换边界完成 |
| 删除功能或兼容分支 | 调用者、fixture、公开契约与迁移影响 |

跨格式或跨协议回归选择能区分语义的代表性输入，避免只证明报错 fixture 已通过。
新增字段不要求每个目标都能表达，但应明确相关目标的等价输出、有损或跳过行为。

## 文档政策

每段内容都应能说明：省略后，读者会产生什么具体误解或漏掉什么操作。
没有实际影响就删除，不以行数、篇幅或减字比例决定取舍。

- 教程和操作指南保留完成任务所需输入、命令、前提和预期结果。
- Reference 说明当前接口语义及无法从字段形状推断的行为；可发现 schema 和源码
  清单直接链接，避免维护完整副本。
- Architecture 解释职责、关系、原因与运行后果；当前实现常量和算法留在源码。
- 同一事实在所属文档完整维护，其他页面按需摘要或链接。任务自包含所需的短例子
  可以重复；流程与字段清单不必复制。入口见[文档索引](docs/README.md)。
- 区分当前默认、实现选择和必须遵守的契约。要求应说明触发条件及违反后的影响，
  不将一次修复、某个客户端或现有目录布局扩成永久禁令。

示例使用占位凭据和文档保留地址；说明公共服务的实际用法时可引用其公开地址。
不提交真实订阅、私密配置或运行数据，安全报告按 [SECURITY.md](SECURITY.md)。

删除或重命名时更新本任务相关链接和旧内容。实施材料失效后清理；仍有操作、兼容或
设计价值的内容归入相应文档，用户要求的交付材料保留。不要把执行日志加入产品文档。

## Commit 与 Pull Request

提交信息使用项目现有的 Conventional Commit 前缀，例如 `feat:`、`fix:`、`refactor:`、`docs:` 和 `chore:`；有明确子域时可使用 scope。每个 commit 应只有一个可独立审阅的关注点。

PR 使用[模板](.github/pull_request_template.md)，说明具体变化和实际验证结果。
仅在触发时填写文档、跨格式、兼容性与迁移影响，删除不适用章节；无需为每个无关项写说明。
界面变化难以用文字说明时，附能展示结果的脱敏截图或录屏。

提交时检查暂存范围、diff 卫生和敏感数据；验证按上文选择，不因本地提交或 PR
一律追加无关 Go/Web 检查。涉及语义、删除、文档或专项流程时，确认对应证据与清理已完成。

## 安全报告

安全漏洞请按 [SECURITY.md](SECURITY.md) 使用 GitHub private vulnerability reporting 报告。不要在公开 issue 或 PR 中披露漏洞细节、真实凭据或未脱敏诊断信息。
