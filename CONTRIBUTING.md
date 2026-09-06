# 贡献指南

本文件是 Sandrone 人类贡献流程的唯一权威。自动化代理的快速约束见 [AGENTS.md](AGENTS.md)，产品与系统文档从[文档索引](docs/README.md)进入。

参与讨论和提交变更即表示你同意遵守[行为准则](CODE_OF_CONDUCT.md)。

## 准备开发环境

在仓库根目录运行 Go 命令。建议准备：

- Go `1.27.0`，版本以 `go.mod` 为准。
- Node.js `24.17.0` 和 pnpm `11.24.0`，仅 Web 开发需要。
- Docker 与 Docker Compose，用于验证容器运行路径。
- 支持当前 Go 版本的 `golangci-lint`，仅 `make ci` 或 `make lint` 需要。

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

所有变更都必须保持以下约束：

- `internal/service` 是唯一业务编排层；CLI、HTTP、MCP 和 Web 入口调用 service，不复制业务逻辑。
- `internal/adapter/*` 负责外部格式解析与渲染，不直接读写 store。
- processor 不 import adapter，也不绕过受控 API 访问网络或文件系统。
- service 和 domain 不 import entrypoint framework 类型。
- `FileSpec.kind` 显式使用 canonical 值，包括 `static`；typed 公共 `config` 只含 `subscriptions` 和 JSON object `settings`。
- typed-file driver 严格解码自身 settings；file-stage processor 按声明顺序执行。
- Web 依赖方向、模块所有权和 `FileDriver` 边界以 [Web 模块约定](web/AGENTS.md)为准。

## 选择验证范围

所有修改交付前检查 `git diff --check`，其余按风险选择；下列覆盖可组合。

| 改动 | 本地交付验证 |
| --- | --- |
| 纯文档、规则说明、Skill 指令 | 检查相关链接/锚点、命令与规则一致性、示例脱敏；Skill 另做结构校验。不要求 Go/Web 全量检查。 |
| Go 实现 | 相关包或行为窄测后运行 `make check`；提交 Go 变更前补 `make lint`，可用 `make ci` 合并执行。 |
| Web 局部逻辑或组件 | 相关 Vitest（行为变化时）、`pnpm --dir web typecheck` 与 `pnpm --dir web lint`；纯文案/样式无需新增单元测试。 |
| Web 共享模型、状态、驱动或依赖 | 在上项基础上运行 `pnpm --dir web test:run`；涉及构建或依赖时加 `pnpm --dir web build`。 |
| Web 路由、浏览器集成或响应式布局 | 在相关静态检查基础上运行对应 Playwright 用例：`pnpm --dir web test:e2e` 可带用例路径。合并 Web 变更或发版前运行完整 smoke。 |
| Shell、构建、CI 或部署配置 | 语法/配置校验及受影响命令或构建路径的验证；不因文件后缀是文档就省略可执行示例的必要检查。 |

`make check` 是 Go 基线：格式检查、`go vet`、默认 Go 测试和 CLI 构建；
`make ci` 是 `make check` 加 Go lint，两者均不包含 Web 检查。远端
[CI](.github/workflows/ci.yml) 另有模块校验与独立 Web job，保留其完整检查。
已完成 `make check` 后只需补 `make lint`，无需为了目标名称重复运行。

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

- adapter：先判断来源字段与 canonical `NodeIR` 语义是否等价，验证受影响的
  parse/render、validation、capability 和 raw/lossy/skip warning 路径。局部修复
  覆盖其转换边界；共享语义变化再按下方矩阵展开。新增字段或扩大值域时，按
  [字段接纳与 warning 处置](docs/architecture/node-pipeline.md#字段接纳与-warning-处置)
  记录来源证据、未知值策略、私有配置边界及所有目标的 supported/lossy/skip 结论。
  warning 本身不是兼容需求，probe 或 renderer 特判不能绕过字段接纳。
- processor、service、store、entrypoint：在最接近公开或层间契约的位置测试；文件流不要只依赖 renderer golden。
- probe：默认门禁覆盖 sing-box；修改 Mihomo backend 时额外运行
  `go test -mod=readonly -tags probe_mihomo ./internal/probe ./internal/service`。
- 用户可见 API、CLI、文件模型或 processor 行为：同时核对 canonical reference 和相关 tutorial/how-to。
- 删除功能：用 `rg` 扫描旧标识，除明确的兼容说明外应为零命中。

### 跨协议与客户端影响矩阵

“全局考虑”要求审查完整影响面，不要求无差别修改所有实现。每个必查项都必须得出
`已修改`、`静态分析或已有测试证明无需修改` 或 `不适用（附原因）` 之一；不能因为当前 fixture、
目标客户端或报错路径只出现一个协议，就省略其它相关项。

矩阵按语义与依赖影响触发，文件路径用于定位，不把注释、局部整理自动升级为全局审查。
结论可按同一影响集合汇总，附调用关系、分支隔离或测试证据，不要求逐文件填写无关项。

最小审查范围按改动性质确定：协议 canonical 语义变化覆盖该协议的全部输入和客户端
输出；客户端共享 adapter 变化覆盖该客户端支持的全部协议；domain、service
normalization/validation 或 shared helper 变化覆盖全部调用者、受影响协议和客户端。

| 修改触点 | 必须审查的代码与契约 |
| --- | --- |
| `internal/domain/node*.go`、协议 option、枚举或 canonical 常量 | 所有 parser/renderer、`nodevalidation`、capability catalog、JSON Nodes、script envelope、`pkg/sandrone` 公共别名，以及节点 clone、preview identity、cache/hash/比较逻辑 |
| `internal/adapter/shared` 的 helper、字段表或 source ref | 用 `rg` 找出全部调用者；检查每个调用协议、输入格式和目标客户端，不能只测新增分支 |
| parser 的来源别名、默认值或 canonical 映射语义变化 | 同协议的其它 parser、受影响 renderer、Raw/unknown warning、validation、capability parse 声明和跨格式转换 |
| renderer 的客户端共享映射变化 | 该客户端支持的全部协议分支、skip/lossy warning、capability render 声明和代表性跨协议测试；若改变 canonical 解释，同时检查其它 renderer |
| `internal/service` 的节点 normalize/validate/输入编排，或 nodes-stage processor/script 节点结构 | 显式/自动/remote/local/ref/inline 输入、processor 前后 validation、直接 render、subscription/file flow、script envelope/schema 和 probe 前校验 |
| capability catalog、warning/error code、report 聚合或上游 revision | 对应 parser/renderer、supported/lossy/raw_only 互斥关系、source ref、阶段与顶层 report、HTTP/CLI/MCP 展示、聚合脱敏和测试计数 |
| probe payload、core backend 或探测前 renderer | sing-box 与 Mihomo 等已注册 core、节点级隔离、raw CLI 路径与保存订阅 processor 链；probe 不得补做 canonical 修复 |
| 删除协议、字段、客户端能力或兼容分支 | parser、renderer、validation、capability、processor/script/API、fixture、文档和旧标识全仓扫描 |

审查时先用 `rg` 确认定义、读写点和 switch/capability 分支，再选择测试。共享 IR 或
共享语义变化至少覆盖一条“一个来源 → `NodeIR` → 两个语义不同的目标”跨格式测试；
客户端共享代码变化至少覆盖两个受影响协议。若实际只支持一个目标或协议，应在交付说明
中明确写出该事实，而不是省略影响分析。

## 文档政策

公开文档只描述当前产品，并按读者意图区分：

- Tutorial 带第一次使用者完成一条有可观察结果的路径，目标约 `120–200` 行。
- How-to 假设读者已有基础，只解决一个任务，目标约 `60–150` 行。
- Reference 可检索、穷举当前契约，通常约 `80–250` 行。
- Architecture 解释边界、关系和数据流，通常约 `100–180` 行，复杂页面不超过约 `250` 行。

入口页同样有预算：根 README 约 `60–100` 行，Docs 索引约 `30–50` 行，本文约 `120–180` 行，AGENTS 约 `50–80` 行。预算用于发现职责膨胀，不应通过空话凑行数。

同一事实只在一个位置完整说明：

- CLI 契约归属 [CLI 参考](docs/reference/cli.md)。
- HTTP 通用约定和资源接口归属 [HTTP API 参考](docs/reference/http-api/README.md)及其专题页。
- MCP transport、tools、resources、prompts 和输出边界归属 [MCP 参考](docs/reference/mcp.md)。
- 文件字段、processor、脚本 API 分别归属 [FileSpec](docs/reference/file-spec.md)、[Processors](docs/reference/processors.md)和[脚本 API](docs/reference/scripting-api.md)。
- 格式能力、错误诊断分别归属[能力参考](docs/reference/capabilities.md)和[错误参考](docs/reference/errors.md)。
- 完整 nodes flow 和 file flow 只在各自架构页出现；README、Docs 索引和架构总览只做摘要与导航。

历史名称只应出现在仍有效且集中的安全、协议、迁移或数据保护说明中。普通 fixture 和示例使用当前规范结构。

实施期间可以维护临时 spec 或 plan；交付时清理本任务产生且已失效的执行材料。仍有效的契约和设计决策先归入 canonical 文档，再删除重复材料；用户明确要求保留的交付文档应保留。长期产品文档不保存 agent 执行日志。删除功能时清理专属旧实现、fixture 和说明，保留仍保护当前行为、兼容或数据安全的回归测试；历史由 Git 保存。

示例只能使用 `example.com`、文档保留地址和占位凭据。不得提交本机路径、私有 fixture、真实订阅、节点 URI、token、cookie、私钥或运行时数据。

## Commit 与 Pull Request

提交信息使用项目现有的 Conventional Commit 前缀，例如 `feat:`、`fix:`、`refactor:`、`docs:` 和 `chore:`；有明确子域时可使用 scope。每个 commit 应只有一个可独立审阅的关注点。

PR 使用[模板](.github/pull_request_template.md)，说明具体变化和实际验证结果。
仅在触发时填写文档、跨格式、兼容性与迁移影响，删除不适用章节；无需为每个无关项写说明。
可见 Web UI 变化附能说明结果的脱敏截图或录屏。

提交时检查暂存范围、diff 卫生和敏感数据；验证按上文选择，不因本地提交或 PR
一律追加无关 Go/Web 检查。涉及语义、删除、文档或专项流程时，确认对应证据与清理已完成。

## 安全报告

安全漏洞请按 [SECURITY.md](SECURITY.md) 使用 GitHub private vulnerability reporting 报告。不要在公开 issue 或 PR 中披露漏洞细节、真实凭据或未脱敏诊断信息。
